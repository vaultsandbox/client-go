package vaultsandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
	"github.com/vaultsandbox/client-go/internal/delivery"
)

// TTL constants for inbox creation.
const (
	MinTTL = 60 * time.Second      // Minimum TTL: 1 minute
	MaxTTL = 604800 * time.Second  // Maximum TTL: 7 days
)

// ServerInfo contains server configuration.
type ServerInfo struct {
	AllowedDomains []string
	MaxTTL         time.Duration
	DefaultTTL     time.Duration
}

// emailEventCallback is an internal callback for email events.
type emailEventCallback func(inbox *Inbox, email *Email)

// Client is the main VaultSandbox client for managing inboxes.
type Client struct {
	apiClient      *api.Client
	strategy       delivery.FullStrategy
	serverInfo     *api.ServerInfo
	inboxes        map[string]*Inbox // keyed by email address
	inboxesByHash  map[string]*Inbox // keyed by inbox hash for O(1) lookup
	mu             sync.RWMutex
	closed         bool

	// Event handling
	eventCallbacks map[string]map[int]emailEventCallback // inboxHash -> id -> callback
	nextCallbackID int
	callbacksMu    sync.RWMutex
	strategyCtx    context.Context
	strategyCancel context.CancelFunc
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
func createDeliveryStrategy(cfg *clientConfig, apiClient *api.Client) delivery.FullStrategy {
	deliveryCfg := delivery.Config{APIClient: apiClient}
	switch cfg.deliveryStrategy {
	case StrategySSE:
		return delivery.NewSSEStrategy(deliveryCfg)
	case StrategyPolling:
		return delivery.NewPollingStrategy(deliveryCfg)
	default:
		return delivery.NewAutoStrategy(deliveryCfg)
	}
}

// New creates a new VaultSandbox client with the given API key.
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	cfg := &clientConfig{
		baseURL:          defaultBaseURL,
		deliveryStrategy: StrategyAuto,
		timeout:          defaultWaitTimeout,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	apiClient, err := buildAPIClient(apiKey, cfg)
	if err != nil {
		return nil, err
	}

	// Validate API key
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	if err := apiClient.CheckKey(ctx); err != nil {
		return nil, wrapError(err)
	}

	// Fetch server info
	serverInfo, err := apiClient.GetServerInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch server info: %w", wrapError(err))
	}

	strategy := createDeliveryStrategy(cfg, apiClient)

	strategyCtx, strategyCancel := context.WithCancel(context.Background())

	c := &Client{
		apiClient:      apiClient,
		strategy:       strategy,
		serverInfo:     serverInfo,
		inboxes:        make(map[string]*Inbox),
		inboxesByHash:  make(map[string]*Inbox),
		eventCallbacks: make(map[string]map[int]emailEventCallback),
		strategyCtx:    strategyCtx,
		strategyCancel: strategyCancel,
	}

	// Start the strategy with an event handler
	if err := strategy.Start(strategyCtx, nil, c.handleSSEEvent); err != nil {
		strategyCancel()
		return nil, fmt.Errorf("start delivery strategy: %w", err)
	}

	return c, nil
}

// CreateInbox creates a new temporary email inbox.
func (c *Client) CreateInbox(ctx context.Context, opts ...InboxOption) (*Inbox, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, ErrClientClosed
	}
	c.mu.RUnlock()

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
	}

	resp, err := c.apiClient.CreateInbox(ctx, req)
	if err != nil {
		return nil, wrapError(err)
	}

	inbox := newInboxFromResult(resp, c)

	c.mu.Lock()
	c.inboxes[inbox.emailAddress] = inbox
	c.inboxesByHash[inbox.inboxHash] = inbox
	c.mu.Unlock()

	// Add to delivery strategy
	c.strategy.AddInbox(delivery.InboxInfo{
		Hash:         inbox.inboxHash,
		EmailAddress: inbox.emailAddress,
	})

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
		return nil, fmt.Errorf("verify inbox: %w", wrapError(err))
	}

	c.mu.Lock()
	c.inboxes[inbox.emailAddress] = inbox
	c.inboxesByHash[inbox.inboxHash] = inbox
	c.mu.Unlock()

	// Add to delivery strategy
	c.strategy.AddInbox(delivery.InboxInfo{
		Hash:         inbox.inboxHash,
		EmailAddress: inbox.emailAddress,
	})

	return inbox, nil
}

// DeleteInbox deletes an inbox by email address.
func (c *Client) DeleteInbox(ctx context.Context, emailAddress string) error {
	c.mu.Lock()
	inbox, exists := c.inboxes[emailAddress]
	if exists {
		delete(c.inboxes, emailAddress)
		delete(c.inboxesByHash, inbox.inboxHash)
		c.strategy.RemoveInbox(inbox.inboxHash)
	}
	c.mu.Unlock()

	return wrapError(c.apiClient.DeleteInboxByEmail(ctx, emailAddress))
}

// DeleteAllInboxes deletes all inboxes managed by this client.
func (c *Client) DeleteAllInboxes(ctx context.Context) (int, error) {
	c.mu.Lock()
	for email, inbox := range c.inboxes {
		c.strategy.RemoveInbox(inbox.inboxHash)
		delete(c.inboxes, email)
		delete(c.inboxesByHash, inbox.inboxHash)
	}
	c.mu.Unlock()

	count, err := c.apiClient.DeleteAllInboxes(ctx)
	return count, wrapError(err)
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
		AllowedDomains: c.serverInfo.AllowedDomains,
		MaxTTL:         time.Duration(c.serverInfo.MaxTTL) * time.Second,
		DefaultTTL:     time.Duration(c.serverInfo.DefaultTTL) * time.Second,
	}
}

// CheckKey validates the API key.
// Returns nil if the key is valid, otherwise returns an error.
func (c *Client) CheckKey(ctx context.Context) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrClientClosed
	}
	c.mu.RUnlock()

	return wrapError(c.apiClient.CheckKey(ctx))
}

// ExportInboxToFile exports an inbox to a JSON file.
// The inbox can be specified by its email address or by passing an *Inbox directly.
func (c *Client) ExportInboxToFile(inbox *Inbox, filePath string) error {
	if inbox == nil {
		return fmt.Errorf("inbox is nil")
	}

	data := inbox.Export()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal inbox data: %w", err)
	}

	if err := os.WriteFile(filePath, jsonData, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// ImportInboxFromFile imports an inbox from a JSON file.
// Returns the imported inbox or an error if the file cannot be read or parsed.
func (c *Client) ImportInboxFromFile(ctx context.Context, filePath string) (*Inbox, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, ErrClientClosed
	}
	c.mu.RUnlock()

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

// MonitorInboxes creates a monitor that watches multiple inboxes for new emails.
// The returned InboxMonitor can be used to register callbacks for email events.
//
// Example:
//
//	monitor := client.MonitorInboxes([]*Inbox{inbox1, inbox2})
//	monitor.OnEmail(func(inbox *Inbox, email *Email) {
//	    fmt.Printf("New email in %s: %s\n", inbox.EmailAddress(), email.Subject)
//	})
//	defer monitor.Unsubscribe()
func (c *Client) MonitorInboxes(inboxes []*Inbox) (*InboxMonitor, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, ErrClientClosed
	}
	if c.strategy == nil {
		c.mu.RUnlock()
		return nil, &StrategyError{Message: "no delivery strategy available"}
	}
	c.mu.RUnlock()

	if len(inboxes) == 0 {
		return nil, fmt.Errorf("at least one inbox is required")
	}

	return newInboxMonitor(c, inboxes), nil
}

// handleSSEEvent processes incoming SSE events from the delivery strategy.
func (c *Client) handleSSEEvent(ctx context.Context, event *api.SSEEvent) error {
	if event == nil {
		return nil
	}

	c.callbacksMu.RLock()
	callbacks := c.eventCallbacks[event.InboxID]
	if len(callbacks) == 0 {
		c.callbacksMu.RUnlock()
		return nil
	}
	// Copy callbacks to slice for iteration outside lock
	callbacksCopy := make([]emailEventCallback, 0, len(callbacks))
	for _, cb := range callbacks {
		callbacksCopy = append(callbacksCopy, cb)
	}
	c.callbacksMu.RUnlock()

	// Find the inbox using O(1) lookup
	c.mu.RLock()
	inbox := c.inboxesByHash[event.InboxID]
	c.mu.RUnlock()

	if inbox == nil {
		return nil
	}

	// Fetch and decrypt the email using the provided context
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	email, err := inbox.GetEmail(ctx, event.EmailID)
	if err != nil {
		return err
	}

	// Low volume expected; spawning per-email is fine.
	for _, cb := range callbacksCopy {
		go cb(inbox, email)
	}

	return nil
}

// registerEmailCallback registers a callback for email events on a specific inbox.
// Returns an ID that can be used to unregister the callback.
func (c *Client) registerEmailCallback(inboxHash string, callback emailEventCallback) int {
	c.callbacksMu.Lock()
	defer c.callbacksMu.Unlock()

	if c.eventCallbacks[inboxHash] == nil {
		c.eventCallbacks[inboxHash] = make(map[int]emailEventCallback)
	}

	id := c.nextCallbackID
	c.nextCallbackID++
	c.eventCallbacks[inboxHash][id] = callback
	return id
}

// unregisterEmailCallback removes a callback by its ID.
func (c *Client) unregisterEmailCallback(inboxHash string, id int) {
	c.callbacksMu.Lock()
	defer c.callbacksMu.Unlock()

	if callbacks, ok := c.eventCallbacks[inboxHash]; ok {
		delete(callbacks, id)
		// Clean up empty maps to avoid memory accumulation
		if len(callbacks) == 0 {
			delete(c.eventCallbacks, inboxHash)
		}
	}
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
		if err := c.strategy.Close(); err != nil {
			return err
		}
	}

	// Clear inboxes and callbacks
	c.inboxes = make(map[string]*Inbox)
	c.inboxesByHash = make(map[string]*Inbox)
	c.callbacksMu.Lock()
	c.eventCallbacks = make(map[string]map[int]emailEventCallback)
	c.callbacksMu.Unlock()

	return nil
}
