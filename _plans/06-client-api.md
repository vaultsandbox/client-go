# 06 - Client API

## Overview

The main `Client` type is the entry point for all VaultSandbox operations. It manages:
- API communication
- Inbox lifecycle
- Delivery strategy
- Thread-safe access

## Client Structure

```go
// client.go
package vaultsandbox

import (
    "context"
    "sync"

    "github.com/vaultsandbox/client-go/internal/api"
    "github.com/vaultsandbox/client-go/internal/crypto"
    "github.com/vaultsandbox/client-go/internal/delivery"
)

// Client is the main entry point for VaultSandbox operations
type Client struct {
    apiClient   *api.Client
    strategy    delivery.Strategy
    serverInfo  *api.ServerInfo
    inboxes     map[string]*Inbox // key: email address
    mu          sync.RWMutex
    closed      bool
}

type clientConfig struct {
    baseURL    string
    httpClient *http.Client
    timeout    time.Duration
    strategy   DeliveryStrategy
    maxRetries int
    retryDelay time.Duration
}

func defaultClientConfig() clientConfig {
    return clientConfig{
        baseURL:    api.DefaultBaseURL,
        timeout:    api.DefaultTimeout,
        strategy:   StrategyAuto,
        maxRetries: api.DefaultMaxRetries,
        retryDelay: api.DefaultRetryDelay,
    }
}
```

## Constructor

```go
// New creates a new VaultSandbox client
func New(apiKey string, opts ...Option) (*Client, error) {
    if apiKey == "" {
        return nil, ErrMissingAPIKey
    }

    cfg := defaultClientConfig()
    for _, opt := range opts {
        opt(&cfg)
    }

    apiClient := api.NewClient(api.Config{
        BaseURL:    cfg.baseURL,
        APIKey:     apiKey,
        HTTPClient: cfg.httpClient,
        Timeout:    cfg.timeout,
        MaxRetries: cfg.maxRetries,
        RetryDelay: cfg.retryDelay,
    })

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

    // Initialize delivery strategy
    var strategy delivery.Strategy
    deliveryCfg := delivery.Config{APIClient: apiClient}

    switch cfg.strategy {
    case StrategySSE:
        strategy = delivery.NewSSEStrategy(deliveryCfg)
    case StrategyPolling:
        strategy = delivery.NewPollingStrategy(deliveryCfg)
    default:
        strategy = delivery.NewAutoStrategy(deliveryCfg)
    }

    return &Client{
        apiClient:  apiClient,
        strategy:   strategy,
        serverInfo: serverInfo,
        inboxes:    make(map[string]*Inbox),
    }, nil
}
```

## Options

```go
// options.go
package vaultsandbox

import (
    "net/http"
    "time"
)

// Option configures the Client
type Option func(*clientConfig)

// WithBaseURL sets a custom API base URL
func WithBaseURL(url string) Option {
    return func(c *clientConfig) {
        c.baseURL = url
    }
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) Option {
    return func(c *clientConfig) {
        c.httpClient = client
    }
}

// WithTimeout sets the default timeout for operations
func WithTimeout(timeout time.Duration) Option {
    return func(c *clientConfig) {
        c.timeout = timeout
    }
}

// WithStrategy sets the delivery strategy
func WithStrategy(strategy DeliveryStrategy) Option {
    return func(c *clientConfig) {
        c.strategy = strategy
    }
}

// WithRetries sets the maximum number of retries
func WithRetries(count int) Option {
    return func(c *clientConfig) {
        c.maxRetries = count
    }
}

// WithRetryDelay sets the initial retry delay
func WithRetryDelay(delay time.Duration) Option {
    return func(c *clientConfig) {
        c.retryDelay = delay
    }
}
```

## Inbox Management

### CreateInbox

```go
const (
    MinTTL = 60 * time.Second       // Minimum TTL: 1 minute
    MaxTTL = 604800 * time.Second   // Maximum TTL: 7 days
)

// CreateInbox creates a new temporary email inbox
func (c *Client) CreateInbox(ctx context.Context, opts ...InboxOption) (*Inbox, error) {
    c.mu.Lock()
    if c.closed {
        c.mu.Unlock()
        return nil, ErrClientClosed
    }
    c.mu.Unlock()

    cfg := defaultInboxConfig()
    for _, opt := range opts {
        opt(&cfg)
    }

    // Validate TTL against server limits
    if cfg.TTL < MinTTL {
        return nil, fmt.Errorf("TTL %v is below minimum %v", cfg.TTL, MinTTL)
    }
    serverMaxTTL := time.Duration(c.serverInfo.MaxTTL) * time.Second
    if cfg.TTL > serverMaxTTL {
        return nil, fmt.Errorf("TTL %v exceeds server maximum %v", cfg.TTL, serverMaxTTL)
    }

    // Generate ML-KEM-768 keypair
    keypair, err := crypto.GenerateKeypair()
    if err != nil {
        return nil, fmt.Errorf("generate keypair: %w", err)
    }

    // Create inbox via API
    req := api.CreateInboxRequest{
        ClientKemPk:  keypair.PublicKeyB64,
        TTL:          int(cfg.TTL.Seconds()),
        EmailAddress: cfg.EmailAddress,
    }

    resp, err := c.apiClient.CreateInbox(ctx, req)
    if err != nil {
        return nil, err
    }

    // Decode server signing public key
    serverSigPk, err := crypto.FromBase64URL(resp.ServerSigPk)
    if err != nil {
        return nil, fmt.Errorf("decode server public key: %w", err)
    }

    inbox := &Inbox{
        emailAddress: resp.EmailAddress,
        expiresAt:    resp.ExpiresAt,
        inboxHash:    resp.InboxHash,
        serverSigPk:  serverSigPk,
        keypair:      keypair,
        client:       c,
    }

    // Register inbox
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

type inboxConfig struct {
    TTL          time.Duration
    EmailAddress string
}

func defaultInboxConfig() inboxConfig {
    return inboxConfig{
        TTL: time.Hour, // Default 1 hour
    }
}

// InboxOption configures inbox creation
type InboxOption func(*inboxConfig)

func WithTTL(ttl time.Duration) InboxOption {
    return func(c *inboxConfig) {
        c.TTL = ttl
    }
}

func WithEmailAddress(email string) InboxOption {
    return func(c *inboxConfig) {
        c.EmailAddress = email
    }
}
```

### ImportInbox

```go
// ImportInbox restores an inbox from exported data
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

    // Validate import data
    if err := data.Validate(); err != nil {
        return nil, err
    }

    // Decode secret key and reconstruct keypair
    secretKey, err := crypto.FromBase64URL(data.SecretKeyB64)
    if err != nil {
        return nil, fmt.Errorf("decode secret key: %w", err)
    }

    keypair, err := crypto.KeypairFromSecretKey(secretKey)
    if err != nil {
        return nil, err
    }

    // Decode server signing public key
    serverSigPk, err := crypto.FromBase64URL(data.ServerSigPk)
    if err != nil {
        return nil, fmt.Errorf("decode server public key: %w", err)
    }

    inbox := &Inbox{
        emailAddress: data.EmailAddress,
        expiresAt:    data.ExpiresAt,
        inboxHash:    data.InboxHash,
        serverSigPk:  serverSigPk,
        keypair:      keypair,
        client:       c,
    }

    // Verify inbox still exists on server (optional)
    _, err = c.apiClient.GetInboxSync(ctx, inbox.emailAddress)
    if err != nil {
        return nil, fmt.Errorf("verify inbox: %w", err)
    }

    c.mu.Lock()
    c.inboxes[inbox.emailAddress] = inbox
    c.mu.Unlock()

    c.strategy.AddInbox(delivery.InboxInfo{
        Hash:         inbox.inboxHash,
        EmailAddress: inbox.emailAddress,
    })

    return inbox, nil
}
```

### DeleteInbox / DeleteAllInboxes

```go
// DeleteInbox removes a specific inbox
func (c *Client) DeleteInbox(ctx context.Context, emailAddress string) error {
    c.mu.Lock()
    inbox, exists := c.inboxes[emailAddress]
    if exists {
        delete(c.inboxes, emailAddress)
        c.strategy.RemoveInbox(inbox.inboxHash)
    }
    c.mu.Unlock()

    return c.apiClient.DeleteInbox(ctx, emailAddress)
}

// DeleteAllInboxes removes all inboxes for this API key
func (c *Client) DeleteAllInboxes(ctx context.Context) (int, error) {
    c.mu.Lock()
    for email, inbox := range c.inboxes {
        c.strategy.RemoveInbox(inbox.inboxHash)
        delete(c.inboxes, email)
    }
    c.mu.Unlock()

    return c.apiClient.DeleteAllInboxes(ctx)
}
```

### GetInbox

```go
// GetInbox returns an inbox by email address if it's managed by this client
func (c *Client) GetInbox(emailAddress string) (*Inbox, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    inbox, ok := c.inboxes[emailAddress]
    return inbox, ok
}

// Inboxes returns all inboxes managed by this client
func (c *Client) Inboxes() []*Inbox {
    c.mu.RLock()
    defer c.mu.RUnlock()

    result := make([]*Inbox, 0, len(c.inboxes))
    for _, inbox := range c.inboxes {
        result = append(result, inbox)
    }
    return result
}
```

### Close

```go
// Close shuts down the client and releases resources
func (c *Client) Close() error {
    c.mu.Lock()
    defer c.mu.Unlock()

    if c.closed {
        return nil
    }

    c.closed = true

    // Stop delivery strategy
    if err := c.strategy.Stop(); err != nil {
        return err
    }

    // Clear inboxes (keys are not deleted from server)
    c.inboxes = make(map[string]*Inbox)

    return nil
}
```

## Server Info

```go
// ServerInfo returns the server configuration
func (c *Client) ServerInfo() *ServerInfo {
    return &ServerInfo{
        AllowedDomains: c.serverInfo.AllowedDomains,
        MaxTTL:         time.Duration(c.serverInfo.MaxTTL) * time.Second,
        DefaultTTL:     time.Duration(c.serverInfo.DefaultTTL) * time.Second,
    }
}

// ServerInfo contains server configuration
type ServerInfo struct {
    AllowedDomains []string
    MaxTTL         time.Duration
    DefaultTTL     time.Duration
}
```

## Usage Examples

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/vaultsandbox/client-go"
)

func main() {
    client, err := vaultsandbox.New("your-api-key")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Create inbox
    inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(2*time.Hour))
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Inbox created: %s\n", inbox.EmailAddress())

    // Wait for email
    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithSubject("Welcome"),
        vaultsandbox.WithWaitTimeout(60*time.Second),
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Received: %s\n", email.Subject)
    fmt.Printf("From: %s\n", email.From)
    fmt.Printf("Body: %s\n", email.Text)

    // Clean up
    if err := inbox.Delete(ctx); err != nil {
        log.Fatal(err)
    }
}
```

### Export/Import

```go
// Export
exported := inbox.Export()
data, _ := json.Marshal(exported)
os.WriteFile("inbox.json", data, 0600)

// Import later
data, _ = os.ReadFile("inbox.json")
var imported vaultsandbox.ExportedInbox
json.Unmarshal(data, &imported)

inbox, err := client.ImportInbox(ctx, &imported)
```

### Custom HTTP Client

```go
httpClient := &http.Client{
    Timeout: 60 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        10,
        IdleConnTimeout:     30 * time.Second,
        DisableCompression:  true,
    },
}

client, err := vaultsandbox.New("api-key",
    vaultsandbox.WithHTTPClient(httpClient),
    vaultsandbox.WithStrategy(vaultsandbox.StrategyPolling),
)
```

### Multiple Inboxes

```go
// Create multiple inboxes
inbox1, _ := client.CreateInbox(ctx)
inbox2, _ := client.CreateInbox(ctx)
inbox3, _ := client.CreateInbox(ctx)

// Wait for emails concurrently
var wg sync.WaitGroup
for _, inbox := range []*vaultsandbox.Inbox{inbox1, inbox2, inbox3} {
    wg.Add(1)
    go func(i *vaultsandbox.Inbox) {
        defer wg.Done()
        email, err := i.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(time.Minute))
        if err != nil {
            return
        }
        fmt.Printf("%s received: %s\n", i.EmailAddress(), email.Subject)
    }(inbox)
}
wg.Wait()
```
