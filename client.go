package vaultsandbox

import (
	"context"
	"sync"

	"github.com/vaultsandbox/client-go/internal/api"
	"github.com/vaultsandbox/client-go/internal/delivery"
)

// Client is the main VaultSandbox client for managing inboxes.
type Client struct {
	apiClient *api.Client
	strategy  delivery.Strategy
	inboxes   map[string]*Inbox
	mu        sync.RWMutex
}

// New creates a new VaultSandbox client with the given API key.
func New(apiKey string, opts ...Option) (*Client, error) {
	cfg := &clientConfig{
		baseURL: defaultBaseURL,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	apiClient, err := api.New(apiKey, api.WithBaseURL(cfg.baseURL))
	if err != nil {
		return nil, err
	}

	if cfg.httpClient != nil {
		apiClient.SetHTTPClient(cfg.httpClient)
	}

	c := &Client{
		apiClient: apiClient,
		strategy:  cfg.strategy,
		inboxes:   make(map[string]*Inbox),
	}

	if c.strategy == nil {
		c.strategy = delivery.NewAutoStrategy(apiClient)
	}

	return c, nil
}

// CreateInbox creates a new temporary email inbox.
func (c *Client) CreateInbox(ctx context.Context, opts ...InboxOption) (*Inbox, error) {
	cfg := &inboxConfig{}
	for _, opt := range opts {
		opt(cfg)
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

	return inbox, nil
}

// ImportInbox imports a previously exported inbox.
func (c *Client) ImportInbox(ctx context.Context, data *ExportedInbox) (*Inbox, error) {
	inbox, err := newInboxFromExport(data, c)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.inboxes[inbox.emailAddress] = inbox
	c.mu.Unlock()

	return inbox, nil
}

// DeleteInbox deletes an inbox by email address.
func (c *Client) DeleteInbox(ctx context.Context, emailAddress string) error {
	c.mu.Lock()
	inbox, exists := c.inboxes[emailAddress]
	if exists {
		delete(c.inboxes, emailAddress)
	}
	c.mu.Unlock()

	if !exists {
		return ErrInboxNotFound
	}

	return c.apiClient.DeleteInbox(ctx, inbox.inboxHash)
}

// DeleteAllInboxes deletes all inboxes managed by this client.
func (c *Client) DeleteAllInboxes(ctx context.Context) (int, error) {
	c.mu.Lock()
	inboxes := make([]*Inbox, 0, len(c.inboxes))
	for _, inbox := range c.inboxes {
		inboxes = append(inboxes, inbox)
	}
	c.inboxes = make(map[string]*Inbox)
	c.mu.Unlock()

	var count int
	for _, inbox := range inboxes {
		if err := c.apiClient.DeleteInbox(ctx, inbox.inboxHash); err != nil {
			continue
		}
		count++
	}

	return count, nil
}

// GetInbox returns an inbox by email address.
func (c *Client) GetInbox(emailAddress string) (*Inbox, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	inbox, exists := c.inboxes[emailAddress]
	return inbox, exists
}

// Close closes the client and releases resources.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.inboxes = make(map[string]*Inbox)

	if c.strategy != nil {
		return c.strategy.Close()
	}

	return nil
}
