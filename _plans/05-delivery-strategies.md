# 05 - Delivery Strategies

## Overview

Delivery strategies determine how the client receives new emails:
- **SSE (Server-Sent Events)**: Real-time push notifications
- **Polling**: Periodic API calls with exponential backoff
- **Auto**: Tries SSE first, falls back to polling

## Strategy Interface

```go
// internal/delivery/strategy.go
package delivery

import (
    "context"

    "github.com/vaultsandbox/client-go/internal/api"
)

// InboxInfo contains the information needed to monitor an inbox
type InboxInfo struct {
    Hash         string // SHA-256 hash of public key (used for SSE)
    EmailAddress string // Email address (used for polling API calls)
}

// Strategy defines the interface for email delivery mechanisms
type Strategy interface {
    // Start begins listening for emails on the given inboxes
    Start(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error

    // Stop gracefully shuts down the strategy
    Stop() error

    // AddInbox adds an inbox to monitor (for SSE, updates connection)
    AddInbox(inbox InboxInfo) error

    // RemoveInbox removes an inbox from monitoring
    RemoveInbox(inboxHash string) error

    // Name returns the strategy name for logging/debugging
    Name() string
}

// EventHandler is called when a new email arrives
type EventHandler func(event *api.SSEEvent) error

// Config holds common strategy configuration
type Config struct {
    APIClient *api.Client
}
```

## SSE Strategy

### Implementation

```go
// internal/delivery/sse.go
package delivery

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "strings"
    "sync"
    "time"

    "github.com/vaultsandbox/client-go/internal/api"
)

const (
    SSEReconnectInterval   = 5 * time.Second
    SSEMaxReconnectAttempts = 10
    SSEBackoffMultiplier   = 2
)

type SSEStrategy struct {
    apiClient     *api.Client
    inboxHashes   map[string]struct{} // Only need hashes for SSE endpoint
    handler       EventHandler
    cancel        context.CancelFunc
    mu            sync.RWMutex
    reconnectWait time.Duration
    attempts      int
}

func NewSSEStrategy(cfg Config) *SSEStrategy {
    return &SSEStrategy{
        apiClient:     cfg.APIClient,
        inboxHashes:   make(map[string]struct{}),
        reconnectWait: SSEReconnectInterval,
    }
}

func (s *SSEStrategy) Name() string {
    return "sse"
}

func (s *SSEStrategy) Start(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error {
    s.mu.Lock()
    for _, inbox := range inboxes {
        s.inboxHashes[inbox.Hash] = struct{}{}
    }
    s.handler = handler
    s.mu.Unlock()

    ctx, s.cancel = context.WithCancel(ctx)
    go s.connectLoop(ctx)
    return nil
}

func (s *SSEStrategy) Stop() error {
    if s.cancel != nil {
        s.cancel()
    }
    return nil
}

func (s *SSEStrategy) AddInbox(inbox InboxInfo) error {
    s.mu.Lock()
    s.inboxHashes[inbox.Hash] = struct{}{}
    s.mu.Unlock()
    // Trigger reconnection with new inbox set
    return nil
}

func (s *SSEStrategy) RemoveInbox(inboxHash string) error {
    s.mu.Lock()
    delete(s.inboxHashes, inboxHash)
    s.mu.Unlock()
    return nil
}

func (s *SSEStrategy) connectLoop(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        err := s.connect(ctx)
        if err == nil {
            // Clean disconnect
            return
        }

        // Handle reconnection with backoff
        s.attempts++
        if s.attempts >= SSEMaxReconnectAttempts {
            // Max attempts reached, give up
            return
        }

        wait := s.reconnectWait * time.Duration(1<<(s.attempts-1))
        select {
        case <-ctx.Done():
            return
        case <-time.After(wait):
        }
    }
}

func (s *SSEStrategy) connect(ctx context.Context) error {
    s.mu.RLock()
    hashes := make([]string, 0, len(s.inboxHashes))
    for h := range s.inboxHashes {
        hashes = append(hashes, h)
    }
    s.mu.RUnlock()

    if len(hashes) == 0 {
        return nil
    }

    resp, err := s.apiClient.OpenEventStream(ctx, hashes)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    // Reset attempts on successful connection
    s.attempts = 0

    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()

        // Skip empty lines and comments
        if line == "" || strings.HasPrefix(line, ":") {
            continue
        }

        // Parse SSE data line
        if strings.HasPrefix(line, "data: ") {
            data := strings.TrimPrefix(line, "data: ")

            var event api.SSEEvent
            if err := json.Unmarshal([]byte(data), &event); err != nil {
                continue // Skip malformed events
            }

            if s.handler != nil {
                s.handler(&event)
            }
        }
    }

    return scanner.Err()
}
```

### SSE Event Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    SSE Connection                           │
│  GET /api/events?inboxes=hash1,hash2                       │
│  Accept: text/event-stream                                  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Event Stream                              │
│  data: {"inboxId":"hash1","emailId":"uuid",...}            │
│  data: {"inboxId":"hash2","emailId":"uuid",...}            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Event Handler                             │
│  - Parse JSON event                                         │
│  - Find matching inbox by hash                              │
│  - Decrypt email metadata                                   │
│  - Notify waiting goroutines                                │
└─────────────────────────────────────────────────────────────┘
```

## Polling Strategy

### Implementation

```go
// internal/delivery/polling.go
package delivery

import (
    "context"
    "math/rand"
    "sync"
    "time"

    "github.com/vaultsandbox/client-go/internal/api"
)

const (
    PollingInitialInterval = 2 * time.Second
    PollingMaxBackoff      = 30 * time.Second
    PollingBackoffMultiplier = 1.5
    PollingJitterFactor    = 0.3
)

type PollingStrategy struct {
    apiClient *api.Client
    inboxes   map[string]*polledInbox // keyed by hash
    handler   EventHandler
    cancel    context.CancelFunc
    mu        sync.RWMutex
}

type polledInbox struct {
    hash         string
    emailAddress string // Required for polling API endpoints
    lastHash     string
    seenEmails   map[string]struct{}
    interval     time.Duration
}

func NewPollingStrategy(cfg Config) *PollingStrategy {
    return &PollingStrategy{
        apiClient: cfg.APIClient,
        inboxes:   make(map[string]*polledInbox),
    }
}

func (p *PollingStrategy) Name() string {
    return "polling"
}

func (p *PollingStrategy) Start(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error {
    p.mu.Lock()
    p.handler = handler
    for _, inbox := range inboxes {
        p.inboxes[inbox.Hash] = &polledInbox{
            hash:         inbox.Hash,
            emailAddress: inbox.EmailAddress,
            seenEmails:   make(map[string]struct{}),
            interval:     PollingInitialInterval,
        }
    }
    p.mu.Unlock()

    ctx, p.cancel = context.WithCancel(ctx)
    go p.pollLoop(ctx)
    return nil
}

func (p *PollingStrategy) Stop() error {
    if p.cancel != nil {
        p.cancel()
    }
    return nil
}

func (p *PollingStrategy) AddInbox(inbox InboxInfo) error {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.inboxes[inbox.Hash] = &polledInbox{
        hash:         inbox.Hash,
        emailAddress: inbox.EmailAddress,
        seenEmails:   make(map[string]struct{}),
        interval:     PollingInitialInterval,
    }
    return nil
}

func (p *PollingStrategy) RemoveInbox(inboxHash string) error {
    p.mu.Lock()
    defer p.mu.Unlock()
    delete(p.inboxes, inboxHash)
    return nil
}

func (p *PollingStrategy) pollLoop(ctx context.Context) {
    ticker := time.NewTicker(PollingInitialInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            p.pollAll(ctx)
        }
    }
}

func (p *PollingStrategy) pollAll(ctx context.Context) {
    p.mu.RLock()
    inboxList := make([]*polledInbox, 0, len(p.inboxes))
    for _, inbox := range p.inboxes {
        inboxList = append(inboxList, inbox)
    }
    p.mu.RUnlock()

    for _, inbox := range inboxList {
        p.pollInbox(ctx, inbox)
    }
}

func (p *PollingStrategy) pollInbox(ctx context.Context, inbox *polledInbox) {
    // Check sync status first
    sync, err := p.apiClient.GetInboxSync(ctx, inbox.emailAddress)
    if err != nil {
        return
    }

    // No changes since last poll
    if sync.EmailsHash == inbox.lastHash {
        // Increase backoff
        inbox.interval = min(
            time.Duration(float64(inbox.interval)*PollingBackoffMultiplier),
            PollingMaxBackoff,
        )
        return
    }

    // Changes detected - fetch emails
    inbox.lastHash = sync.EmailsHash
    inbox.interval = PollingInitialInterval // Reset backoff

    emails, err := p.apiClient.GetEmails(ctx, inbox.emailAddress)
    if err != nil {
        return
    }

    // Find new emails
    for _, email := range emails {
        if _, seen := inbox.seenEmails[email.ID]; !seen {
            inbox.seenEmails[email.ID] = struct{}{}

            if p.handler != nil {
                p.handler(&api.SSEEvent{
                    InboxID:           inbox.hash,
                    EmailID:           email.ID,
                    EncryptedMetadata: email.EncryptedMetadata,
                })
            }
        }
    }
}

func (p *PollingStrategy) getWaitDuration(inbox *polledInbox) time.Duration {
    // Add jitter to prevent thundering herd
    jitter := time.Duration(rand.Float64() * PollingJitterFactor * float64(inbox.interval))
    return inbox.interval + jitter
}
```

### Polling Flow

```
┌─────────────────────────────────────────────────────────────┐
│                     Start Polling                           │
│  interval = 2s, lastHash = ""                               │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              GET /api/inboxes/{email}/sync                  │
│              Response: { emailsHash, emailCount }           │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              │                               │
       Hash Changed                    Hash Same
              │                               │
              ▼                               ▼
┌─────────────────────────┐     ┌─────────────────────────────┐
│  Fetch All Emails       │     │  Increase Backoff           │
│  Reset interval to 2s   │     │  interval *= 1.5            │
│  Find new email IDs     │     │  (max 30s)                  │
│  Call handler for each  │     │                             │
└─────────────────────────┘     └─────────────────────────────┘
              │                               │
              └───────────────┬───────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              Wait (interval + jitter)                       │
│              Jitter = random 0-30% of interval              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    (Loop back to sync)
```

## Auto Strategy

```go
// internal/delivery/auto.go
package delivery

import (
    "context"
    "time"
)

const (
    AutoSSETimeout = 5 * time.Second
)

type AutoStrategy struct {
    cfg       Config
    current   Strategy
    handler   EventHandler
}

func NewAutoStrategy(cfg Config) *AutoStrategy {
    return &AutoStrategy{
        cfg: cfg,
    }
}

func (a *AutoStrategy) Name() string {
    if a.current != nil {
        return "auto:" + a.current.Name()
    }
    return "auto"
}

func (a *AutoStrategy) Start(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error {
    a.handler = handler

    // Try SSE first with timeout
    sseCtx, cancel := context.WithTimeout(ctx, AutoSSETimeout)
    defer cancel()

    sse := NewSSEStrategy(a.cfg)
    err := sse.Start(sseCtx, inboxes, handler)

    if err == nil {
        a.current = sse
        return nil
    }

    // Fall back to polling
    polling := NewPollingStrategy(a.cfg)
    err = polling.Start(ctx, inboxes, handler)
    if err != nil {
        return err
    }

    a.current = polling
    return nil
}

func (a *AutoStrategy) Stop() error {
    if a.current != nil {
        return a.current.Stop()
    }
    return nil
}

func (a *AutoStrategy) AddInbox(inbox InboxInfo) error {
    if a.current != nil {
        return a.current.AddInbox(inbox)
    }
    return nil
}

func (a *AutoStrategy) RemoveInbox(inboxHash string) error {
    if a.current != nil {
        return a.current.RemoveInbox(inboxHash)
    }
    return nil
}
```

## Strategy Configuration

```go
// Public API for strategy selection
package vaultsandbox

type DeliveryStrategy string

const (
    StrategyAuto    DeliveryStrategy = "auto"
    StrategySSE     DeliveryStrategy = "sse"
    StrategyPolling DeliveryStrategy = "polling"
)

func WithStrategy(strategy DeliveryStrategy) Option {
    return func(c *clientConfig) {
        c.strategy = strategy
    }
}
```

## WaitForEmail Integration

```go
func (i *Inbox) WaitForEmail(ctx context.Context, opts ...WaitOption) (*Email, error) {
    cfg := defaultWaitConfig()
    for _, opt := range opts {
        opt(&cfg)
    }

    // Create timeout context
    ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
    defer cancel()

    // Channel for receiving matching emails
    emailCh := make(chan *Email, 1)

    // Register handler with strategy
    handler := func(event *api.SSEEvent) error {
        if event.InboxID != i.inboxHash {
            return nil
        }

        // Decrypt and check filters
        email, err := i.processEmailEvent(ctx, event)
        if err != nil {
            return nil // Skip this email
        }

        if matchesFilters(email, &cfg) {
            select {
            case emailCh <- email:
            default:
            }
        }
        return nil
    }

    // Start strategy if not running
    inboxInfo := delivery.InboxInfo{
        Hash:         i.inboxHash,
        EmailAddress: i.emailAddress,
    }
    i.client.strategy.Start(ctx, []delivery.InboxInfo{inboxInfo}, handler)

    // Also check existing emails
    go func() {
        emails, err := i.GetEmails(ctx)
        if err != nil {
            return
        }
        for _, email := range emails {
            if matchesFilters(email, &cfg) {
                select {
                case emailCh <- email:
                    return
                default:
                }
            }
        }
    }()

    select {
    case email := <-emailCh:
        return email, nil
    case <-ctx.Done():
        return nil, &TimeoutError{Timeout: cfg.Timeout}
    }
}

// WaitForEmailCount waits until at least N emails are in the inbox
func (i *Inbox) WaitForEmailCount(ctx context.Context, count int, opts ...WaitOption) ([]*Email, error) {
    cfg := defaultWaitConfig()
    for _, opt := range opts {
        opt(&cfg)
    }

    // Create timeout context
    ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
    defer cancel()

    // Channel for signaling new emails
    newEmailCh := make(chan struct{}, 1)

    // Register handler to detect new emails
    handler := func(event *api.SSEEvent) error {
        if event.InboxID != i.inboxHash {
            return nil
        }
        select {
        case newEmailCh <- struct{}{}:
        default:
        }
        return nil
    }

    // Start strategy
    inboxInfo := delivery.InboxInfo{
        Hash:         i.inboxHash,
        EmailAddress: i.emailAddress,
    }
    i.client.strategy.Start(ctx, []delivery.InboxInfo{inboxInfo}, handler)

    // Poll until we have enough emails
    ticker := time.NewTicker(cfg.PollInterval)
    defer ticker.Stop()

    for {
        emails, err := i.GetEmails(ctx)
        if err != nil {
            return nil, err
        }

        // Filter emails if predicates are set
        var matched []*Email
        for _, email := range emails {
            if matchesFilters(email, &cfg) {
                matched = append(matched, email)
            }
        }

        if len(matched) >= count {
            return matched[:count], nil
        }

        // Wait for new email or timeout
        select {
        case <-newEmailCh:
            // New email arrived, recheck immediately
        case <-ticker.C:
            // Periodic recheck
        case <-ctx.Done():
            return nil, &TimeoutError{Timeout: cfg.Timeout}
        }
    }
}

func matchesFilters(email *Email, cfg *WaitConfig) bool {
    if cfg.Subject != "" && email.Subject != cfg.Subject {
        return false
    }
    if cfg.SubjectRegex != nil && !cfg.SubjectRegex.MatchString(email.Subject) {
        return false
    }
    if cfg.From != "" && email.From != cfg.From {
        return false
    }
    if cfg.FromRegex != nil && !cfg.FromRegex.MatchString(email.From) {
        return false
    }
    if cfg.Predicate != nil && !cfg.Predicate(email) {
        return false
    }
    return true
}
```
