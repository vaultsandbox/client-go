package vaultsandbox

import (
	"context"
	"fmt"
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

// Client is the main VaultSandbox client for managing inboxes.
type Client struct {
	apiClient  *api.Client
	strategy   delivery.FullStrategy
	serverInfo *api.ServerInfo
	inboxes    map[string]*Inbox
	mu         sync.RWMutex
	closed     bool
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

	// Build API client options
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

	// Create delivery strategy based on configuration
	deliveryCfg := delivery.Config{APIClient: apiClient}
	var strategy delivery.FullStrategy
	switch cfg.deliveryStrategy {
	case StrategySSE:
		strategy = delivery.NewSSEStrategy(deliveryCfg)
	case StrategyPolling:
		strategy = delivery.NewPollingStrategy(deliveryCfg)
	default:
		strategy = delivery.NewAutoStrategy(deliveryCfg)
	}

	c := &Client{
		apiClient:  apiClient,
		strategy:   strategy,
		serverInfo: serverInfo,
		inboxes:    make(map[string]*Inbox),
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

	req := &api.LegacyCreateInboxRequest{
		TTL:          cfg.ttl,
		EmailAddress: cfg.emailAddress,
	}

	resp, err := c.apiClient.CreateInbox(ctx, req)
	if err != nil {
		return nil, err
	}

	inbox := newInboxFromLegacyResponse(resp, c)

	c.mu.Lock()
	c.inboxes[inbox.emailAddress] = inbox
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

	c.mu.Lock()
	c.inboxes[inbox.emailAddress] = inbox
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
	}
	c.mu.Unlock()

	return c.apiClient.DeleteAllInboxes(ctx)
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

// Close closes the client and releases resources.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	// Stop delivery strategy
	if c.strategy != nil {
		if err := c.strategy.Close(); err != nil {
			return err
		}
	}

	// Clear inboxes (keys are not deleted from server)
	c.inboxes = make(map[string]*Inbox)

	return nil
}
