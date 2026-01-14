package vaultsandbox

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
	"github.com/vaultsandbox/client-go/internal/delivery"
)

// TTL constants for inbox creation.
const (
	MinTTL = 60 * time.Second     // Minimum TTL: 1 minute
	MaxTTL = 604800 * time.Second // Maximum TTL: 7 days
)

// sseEventTimeout is the timeout for fetching and decrypting an email
// after receiving an SSE notification.
const sseEventTimeout = 30 * time.Second

// syncState tracks the synchronization state for an inbox to enable
// efficient reconnection sync using the /sync endpoint.
type syncState struct {
	seenEmails map[string]struct{} // Set of email IDs already delivered to subscribers
}

// computeEmailsHash computes the hash of seen emails to compare with server's sync hash.
// Algorithm: sort IDs alphabetically, join with comma, SHA256, base64url encode (no padding).
func (s *syncState) computeEmailsHash() string {
	if len(s.seenEmails) == 0 {
		// Empty set has a specific hash
		hash := sha256.Sum256([]byte(""))
		return base64.RawURLEncoding.EncodeToString(hash[:])
	}

	ids := make([]string, 0, len(s.seenEmails))
	for id := range s.seenEmails {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	joined := strings.Join(ids, ",")
	hash := sha256.Sum256([]byte(joined))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// EncryptionPolicy represents the server's encryption policy for inboxes.
type EncryptionPolicy = api.EncryptionPolicy

// Encryption policy constants.
const (
	// EncryptionPolicyAlways requires all inboxes to be encrypted.
	EncryptionPolicyAlways = api.EncryptionPolicyAlways
	// EncryptionPolicyEnabled makes encryption the default, but allows plain inboxes.
	EncryptionPolicyEnabled = api.EncryptionPolicyEnabled
	// EncryptionPolicyDisabled makes plain the default, but allows encrypted inboxes.
	EncryptionPolicyDisabled = api.EncryptionPolicyDisabled
	// EncryptionPolicyNever requires all inboxes to be plain.
	EncryptionPolicyNever = api.EncryptionPolicyNever
)

// ServerInfo contains server configuration.
type ServerInfo struct {
	AllowedDomains   []string
	MaxTTL           time.Duration
	DefaultTTL       time.Duration
	EncryptionPolicy EncryptionPolicy
}

// Client is the main VaultSandbox client for managing inboxes.
type Client struct {
	apiClient     *api.Client
	strategy      delivery.Strategy
	serverInfo    *api.ServerInfo
	inboxes       map[string]*Inbox     // keyed by email address
	inboxesByHash map[string]*Inbox     // keyed by inbox hash for O(1) lookup
	syncStates    map[string]*syncState // keyed by inbox hash for sync optimization
	mu            sync.RWMutex
	closed        bool

	// Subscription manager for email notifications
	subs *subscriptionManager

	strategyCtx    context.Context
	strategyCancel context.CancelFunc

	// Error callback for background sync failures
	onSyncError func(error)
}

// buildAPIClient creates and configures an API client from the given config.
func buildAPIClient(apiKey string, cfg *clientConfig) (*api.Client, error) {
	apiOpts := []api.Option{
		api.WithBaseURL(cfg.baseURL),
	}
	if cfg.timeout > 0 {
		apiOpts = append(apiOpts, api.WithTimeout(cfg.timeout))
	}
	if cfg.retries > 0 {
		apiOpts = append(apiOpts, api.WithRetries(cfg.retries))
	}
	if len(cfg.retryOn) > 0 {
		apiOpts = append(apiOpts, api.WithRetryOn(cfg.retryOn))
	}

	apiClient, err := api.New(apiKey, apiOpts...)
	if err != nil {
		return nil, err
	}

	if cfg.httpClient != nil {
		apiClient.SetHTTPClient(cfg.httpClient)
	}

	return apiClient, nil
}

// createDeliveryStrategy creates a delivery strategy based on the config.
func createDeliveryStrategy(cfg *clientConfig, apiClient *api.Client) delivery.Strategy {
	deliveryCfg := delivery.Config{
		APIClient:                apiClient,
		PollingInitialInterval:   cfg.pollingInitialInterval,
		PollingMaxBackoff:        cfg.pollingMaxBackoff,
		PollingBackoffMultiplier: cfg.pollingBackoffMultiplier,
		PollingJitterFactor:      cfg.pollingJitterFactor,
	}
	switch cfg.deliveryStrategy {
	case StrategyPolling:
		return delivery.NewPollingStrategy(deliveryCfg)
	default:
		return delivery.NewSSEStrategy(deliveryCfg)
	}
}

// New creates a new VaultSandbox client with the given API key.
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	cfg := &clientConfig{
		baseURL:          defaultBaseURL,
		deliveryStrategy: StrategySSE,
		timeout:          defaultWaitTimeout,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	apiClient, err := buildAPIClient(apiKey, cfg)
	if err != nil {
		return nil, err //coverage:ignore
	}

	// Validate API key
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	if err := apiClient.CheckKey(ctx); err != nil {
		return nil, err
	}

	// Fetch server info
	serverInfo, err := apiClient.GetServerInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch server info: %w", err)
	}

	strategy := createDeliveryStrategy(cfg, apiClient)

	strategyCtx, strategyCancel := context.WithCancel(context.Background())

	c := &Client{
		apiClient:      apiClient,
		strategy:       strategy,
		serverInfo:     serverInfo,
		inboxes:        make(map[string]*Inbox),
		inboxesByHash:  make(map[string]*Inbox),
		syncStates:     make(map[string]*syncState),
		subs:           newSubscriptionManager(),
		strategyCtx:    strategyCtx,
		strategyCancel: strategyCancel,
		onSyncError:    cfg.onSyncError,
	}

	// Start the strategy with an event handler
	if err := strategy.Start(strategyCtx, nil, c.handleSSEEvent); err != nil {
		strategyCancel()                                           //coverage:ignore
		return nil, fmt.Errorf("start delivery strategy: %w", err) //coverage:ignore
	}

	// Register reconnect handler to sync emails after SSE reconnection.
	// This catches any emails that arrived during the reconnection window.
	strategy.OnReconnect(c.syncAllInboxes)

	return c, nil
}

// checkClosed returns ErrClientClosed if the client has been closed.
func (c *Client) checkClosed() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return ErrClientClosed
	}
	return nil
}

// registerInbox adds an inbox to the client's tracking maps and delivery strategy.
func (c *Client) registerInbox(inbox *Inbox) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrClientClosed
	}
	c.inboxes[inbox.emailAddress] = inbox
	c.inboxesByHash[inbox.inboxHash] = inbox
	c.syncStates[inbox.inboxHash] = &syncState{
		seenEmails: make(map[string]struct{}),
	}
	c.strategy.AddInbox(delivery.InboxInfo{
		Hash:         inbox.inboxHash,
		EmailAddress: inbox.emailAddress,
	})
	return nil
}

// CreateInbox creates a new temporary email inbox.
func (c *Client) CreateInbox(ctx context.Context, opts ...InboxOption) (*Inbox, error) {
	if err := c.checkClosed(); err != nil {
		return nil, err
	}

	cfg := &inboxConfig{
		ttl: time.Hour, // Default 1 hour
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Validate TTL against limits
	if cfg.ttl > 0 {
		if cfg.ttl < MinTTL {
			return nil, fmt.Errorf("TTL %v is below minimum %v", cfg.ttl, MinTTL)
		}
		serverMaxTTL := time.Duration(c.serverInfo.MaxTTL) * time.Second
		if cfg.ttl > serverMaxTTL {
			return nil, fmt.Errorf("TTL %v exceeds server maximum %v", cfg.ttl, serverMaxTTL)
		}
	}

	req := &api.CreateInboxParams{
		TTL:          cfg.ttl,
		EmailAddress: cfg.emailAddress,
		EmailAuth:    cfg.emailAuth,
		Encryption:   string(cfg.encryption),
	}

	resp, err := c.apiClient.CreateInbox(ctx, req)
	if err != nil {
		return nil, err
	}

	inbox := newInboxFromResult(resp, c)

	if err := c.registerInbox(inbox); err != nil {
		return nil, err //coverage:ignore
	}

	return inbox, nil
}

// ImportInbox imports a previously exported inbox.
func (c *Client) ImportInbox(ctx context.Context, data *ExportedInbox) (*Inbox, error) {
	if data == nil {
		return nil, fmt.Errorf("exported inbox data cannot be nil")
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClientClosed
	}

	// Check for duplicate
	if _, exists := c.inboxes[data.EmailAddress]; exists {
		c.mu.Unlock()
		return nil, ErrInboxAlreadyExists
	}
	c.mu.Unlock()

	inbox, err := newInboxFromExport(data, c)
	if err != nil {
		return nil, err
	}

	// Verify inbox still exists on server
	_, err = c.apiClient.GetInboxSync(ctx, inbox.emailAddress)
	if err != nil {
		return nil, fmt.Errorf("verify inbox: %w", err)
	}

	if err := c.registerInbox(inbox); err != nil {
		return nil, err //coverage:ignore
	}

	return inbox, nil
}

// DeleteInbox deletes an inbox by email address.
func (c *Client) DeleteInbox(ctx context.Context, emailAddress string) error {
	c.mu.Lock()
	inbox, exists := c.inboxes[emailAddress]
	if exists {
		delete(c.inboxes, emailAddress)
		delete(c.inboxesByHash, inbox.inboxHash)
		delete(c.syncStates, inbox.inboxHash)
		c.strategy.RemoveInbox(inbox.inboxHash)
	}
	c.mu.Unlock()

	return c.apiClient.DeleteInboxByEmail(ctx, emailAddress)
}

// DeleteAllInboxes deletes all inboxes managed by this client.
func (c *Client) DeleteAllInboxes(ctx context.Context) (int, error) {
	c.mu.Lock()
	for email, inbox := range c.inboxes {
		c.strategy.RemoveInbox(inbox.inboxHash)
		delete(c.inboxes, email)
		delete(c.inboxesByHash, inbox.inboxHash)
		delete(c.syncStates, inbox.inboxHash)
	}
	c.mu.Unlock()

	count, err := c.apiClient.DeleteAllInboxes(ctx)
	return count, err
}

// GetInbox returns an inbox by email address.
func (c *Client) GetInbox(emailAddress string) (*Inbox, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	inbox, exists := c.inboxes[emailAddress]
	return inbox, exists
}

// Inboxes returns all inboxes managed by this client.
func (c *Client) Inboxes() []*Inbox {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*Inbox, 0, len(c.inboxes))
	for _, inbox := range c.inboxes {
		result = append(result, inbox)
	}
	return result
}

// ServerInfo returns the server configuration.
func (c *Client) ServerInfo() *ServerInfo {
	return &ServerInfo{
		AllowedDomains:   c.serverInfo.AllowedDomains,
		MaxTTL:           time.Duration(c.serverInfo.MaxTTL) * time.Second,
		DefaultTTL:       time.Duration(c.serverInfo.DefaultTTL) * time.Second,
		EncryptionPolicy: c.serverInfo.EncryptionPolicy,
	}
}

// CheckKey validates the API key.
// Returns nil if the key is valid, otherwise returns an error.
func (c *Client) CheckKey(ctx context.Context) error {
	if err := c.checkClosed(); err != nil {
		return err
	}
	return c.apiClient.CheckKey(ctx)
}

// ExportInboxToFile exports an inbox to a JSON file with secure permissions (0600).
func (c *Client) ExportInboxToFile(inbox *Inbox, filePath string) error {
	if inbox == nil {
		return fmt.Errorf("inbox is nil")
	}

	data := inbox.Export()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal inbox data: %w", err) //coverage:ignore
	}

	if err := os.WriteFile(filePath, jsonData, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// ImportInboxFromFile imports an inbox from a JSON file.
// Returns the imported inbox or an error if the file cannot be read or parsed.
func (c *Client) ImportInboxFromFile(ctx context.Context, filePath string) (*Inbox, error) {
	if err := c.checkClosed(); err != nil {
		return nil, err
	}

	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var data ExportedInbox
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("parse inbox data: %w", err)
	}

	return c.ImportInbox(ctx, &data)
}

// InboxEvent represents an email arriving in a specific inbox.
type InboxEvent struct {
	Inbox *Inbox
	Email *Email
}

// WatchInboxes returns a channel that receives events from multiple inboxes.
// The channel is not closed when the context is cancelled; use a select
// on ctx.Done() to detect cancellation.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
//	defer cancel()
//
//	ch := client.WatchInboxes(ctx, inbox1, inbox2)
//	for {
//	    select {
//	    case <-ctx.Done():
//	        return
//	    case event := <-ch:
//	        fmt.Printf("Email in %s: %s\n", event.Inbox.EmailAddress(), event.Email.Subject)
//	    }
//	}
func (c *Client) WatchInboxes(ctx context.Context, inboxes ...*Inbox) <-chan *InboxEvent {
	ch := make(chan *InboxEvent, 16)

	if len(inboxes) == 0 {
		close(ch)
		return ch
	}

	// Track unsubscribe functions
	unsubscribes := make([]func(), 0, len(inboxes))

	for _, inbox := range inboxes {
		inbox := inbox
		unsub := c.subs.subscribe(inbox.inboxHash, func(email *Email) {
			// Spawn goroutine to guarantee delivery without blocking event source
			go func(e *Email) { ch <- &InboxEvent{Inbox: inbox, Email: e} }(email)
		})
		unsubscribes = append(unsubscribes, unsub)
	}

	// Cleanup goroutine: unsubscribe when context is cancelled.
	// We intentionally do not close(ch) to avoid a race where an
	// in-flight callback tries to send after close.
	go func() {
		<-ctx.Done()
		for _, unsub := range unsubscribes {
			unsub()
		}
	}()

	return ch
}

// WatchInboxesFunc calls fn for each event from multiple inboxes until context is cancelled.
// This is a convenience wrapper around WatchInboxes for simpler use cases.
//
// Example:
//
//	client.WatchInboxesFunc(ctx, func(event *vaultsandbox.InboxEvent) {
//	    fmt.Printf("Email in %s: %s\n", event.Inbox.EmailAddress(), event.Email.Subject)
//	}, inbox1, inbox2)
func (c *Client) WatchInboxesFunc(ctx context.Context, fn func(*InboxEvent), inboxes ...*Inbox) {
	events := c.WatchInboxes(ctx, inboxes...)
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-events:
			if event != nil {
				fn(event)
			}
		}
	}
}

// syncAllInboxes fetches emails for all tracked inboxes and notifies watchers.
// This is called after SSE reconnection to catch any emails that arrived
// during the reconnection window.
func (c *Client) syncAllInboxes(ctx context.Context) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return
	}
	// Copy inbox list to avoid holding lock during API calls
	inboxes := make([]*Inbox, 0, len(c.inboxes))
	for _, inbox := range c.inboxes {
		inboxes = append(inboxes, inbox)
	}
	c.mu.RUnlock()

	// Sync each inbox
	for _, inbox := range inboxes {
		c.syncInbox(ctx, inbox)
	}
}

// syncInbox fetches emails for a single inbox and notifies subscribers for new emails.
// It uses the sync endpoint to check for changes before fetching, and only fetches
// full email data for emails that haven't been seen before. It also handles deletions
// by removing IDs from seenEmails that are no longer on the server.
func (c *Client) syncInbox(ctx context.Context, inbox *Inbox) {
	// Get sync state for this inbox and compute current hash
	c.mu.RLock()
	state := c.syncStates[inbox.inboxHash]
	var localHash string
	if state != nil {
		localHash = state.computeEmailsHash()
	}
	c.mu.RUnlock()

	if state == nil {
		// Inbox was deleted or not registered, skip
		return
	}

	// Check sync status first (lightweight call)
	status, err := inbox.GetSyncStatus(ctx)
	if err != nil {
		if c.onSyncError != nil {
			c.onSyncError(err)
		}
		return
	}

	// If hash unchanged, no changes - skip fetching
	if status.EmailsHash == localHash {
		return
	}

	// Hash changed - fetch metadata only to find changes
	metadata, err := inbox.GetEmailsMetadataOnly(ctx)
	if err != nil {
		if c.onSyncError != nil {
			c.onSyncError(err)
		}
		return
	}

	// Build set of server email IDs
	serverIDs := make(map[string]struct{}, len(metadata))
	for _, m := range metadata {
		serverIDs[m.ID] = struct{}{}
	}

	// Find new and deleted email IDs
	c.mu.Lock()
	state = c.syncStates[inbox.inboxHash]
	if state == nil {
		c.mu.Unlock() //coverage:ignore
		return        //coverage:ignore
	}

	// Find new emails (on server but not in seenEmails)
	var newEmailIDs []string
	for id := range serverIDs {
		if _, seen := state.seenEmails[id]; !seen {
			newEmailIDs = append(newEmailIDs, id)
		}
	}

	// Find and remove deleted emails (in seenEmails but not on server)
	for id := range state.seenEmails {
		if _, exists := serverIDs[id]; !exists {
			delete(state.seenEmails, id)
		}
	}
	c.mu.Unlock()

	// Fetch full email data only for new emails
	for _, emailID := range newEmailIDs {
		email, err := inbox.GetEmail(ctx, emailID)
		if err != nil {
			if c.onSyncError != nil {
				c.onSyncError(err)
			}
			continue
		}

		// Mark as seen and notify
		c.mu.Lock()
		state = c.syncStates[inbox.inboxHash]
		if state == nil {
			c.mu.Unlock() //coverage:ignore
			return        //coverage:ignore
		}
		state.seenEmails[email.ID] = struct{}{}
		c.mu.Unlock()

		c.subs.notify(inbox.inboxHash, email)
	}
}

// handleSSEEvent processes incoming SSE events from the delivery strategy.
func (c *Client) handleSSEEvent(ctx context.Context, event *api.SSEEvent) error {
	if event == nil {
		return nil
	}

	// Find the inbox using O(1) lookup
	c.mu.RLock()
	inbox := c.inboxesByHash[event.InboxID]
	state := c.syncStates[event.InboxID]
	c.mu.RUnlock()

	if inbox == nil {
		return nil
	}

	// Fetch and decrypt the email
	ctx, cancel := context.WithTimeout(ctx, sseEventTimeout)
	defer cancel()

	email, err := inbox.GetEmail(ctx, event.EmailID)
	if err != nil {
		return err
	}

	// Mark email as seen to avoid duplicate notifications on reconnection sync
	if state != nil {
		c.mu.Lock()
		// Re-check state exists (inbox could have been deleted)
		if state = c.syncStates[event.InboxID]; state != nil {
			state.seenEmails[email.ID] = struct{}{}
		}
		c.mu.Unlock()
	}

	// Notify all subscribers
	c.subs.notify(inbox.inboxHash, email)

	return nil
}

// Close closes the client and releases resources.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	// Cancel strategy context
	if c.strategyCancel != nil {
		c.strategyCancel()
	}

	// Stop delivery strategy
	if c.strategy != nil {
		if err := c.strategy.Stop(); err != nil {
			return err //coverage:ignore
		}
	}

	// Clear inboxes and subscriptions
	c.inboxes = make(map[string]*Inbox)
	c.inboxesByHash = make(map[string]*Inbox)
	c.subs.clear()

	return nil
}
