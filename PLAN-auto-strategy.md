# Plan: Extract AutoStrategy

## Problem

The "auto mode" fallback logic is scattered across `Client` (client.go:240-301) rather than encapsulated in a proper strategy implementation:

- `autoMode`, `autoChecked`, `sseTimeout` fields on Client (lines 42-50)
- `checkAutoModeConnection()` method with type assertion to `*delivery.SSEStrategy` (line 252)
- `fallbackToPolling()` method that manually swaps strategies (lines 272-301)

This breaks the strategy abstraction: Client shouldn't know about concrete strategy types.

## Solution

Create `internal/delivery/auto.go` implementing the `Strategy` interface that internally manages SSE-to-polling fallback.

## Files to Modify

| File | Changes |
|------|---------|
| `internal/delivery/auto.go` | **NEW** - AutoStrategy implementation |
| `client.go` | Remove autoMode fields and methods, simplify createDeliveryStrategy |

## AutoStrategy Design

```go
// internal/delivery/auto.go
type AutoStrategy struct {
    cfg           Config
    sseStrategy   *SSEStrategy
    pollStrategy  *PollingStrategy
    active        Strategy          // Currently active strategy
    handler       EventHandler
    ctx           context.Context
    cancel        context.CancelFunc
    mu            sync.RWMutex
    switched      bool              // True after fallback occurred
    sseTimeout    time.Duration
}

func NewAutoStrategy(cfg Config) *AutoStrategy
func (a *AutoStrategy) Start(ctx, inboxes, handler) error
func (a *AutoStrategy) Stop() error
func (a *AutoStrategy) AddInbox(inbox) error
func (a *AutoStrategy) RemoveInbox(hash) error
func (a *AutoStrategy) Name() string  // Returns "auto:sse" or "auto:polling"
func (a *AutoStrategy) OnReconnect(fn)
```

## Implementation Steps

### Step 1: Create `internal/delivery/auto.go`

```go
package delivery

import (
    "context"
    "sync"
    "time"
)

// AutoStrategy implements automatic fallback from SSE to polling.
// It starts with SSE and falls back to polling if SSE doesn't connect
// within the configured timeout.
type AutoStrategy struct {
    cfg         Config
    sse         *SSEStrategy
    polling     *PollingStrategy
    active      Strategy
    handler     EventHandler
    onReconnect func(ctx context.Context)
    ctx         context.Context
    cancel      context.CancelFunc
    mu          sync.RWMutex
    switched    bool
    timeout     time.Duration
}

// NewAutoStrategy creates a new auto strategy that starts with SSE
// and falls back to polling if SSE doesn't connect in time.
func NewAutoStrategy(cfg Config) *AutoStrategy {
    timeout := cfg.SSEConnectionTimeout
    if timeout == 0 {
        timeout = DefaultSSEConnectionTimeout
    }
    return &AutoStrategy{
        cfg:     cfg,
        timeout: timeout,
    }
}

func (a *AutoStrategy) Name() string {
    a.mu.RLock()
    defer a.mu.RUnlock()
    if a.switched {
        return "auto:polling"
    }
    return "auto:sse"
}

func (a *AutoStrategy) Start(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error {
    a.mu.Lock()
    a.handler = handler
    a.ctx, a.cancel = context.WithCancel(ctx)

    // Create both strategies
    a.sse = NewSSEStrategy(a.cfg)
    a.polling = NewPollingStrategy(a.cfg)
    a.active = a.sse
    a.mu.Unlock()

    // Start SSE
    if err := a.sse.Start(a.ctx, inboxes, handler); err != nil {
        return err
    }

    // Register reconnect handler on SSE
    if a.onReconnect != nil {
        a.sse.OnReconnect(a.onReconnect)
    }

    // Monitor SSE connection in background
    go a.monitorConnection(inboxes)

    return nil
}

func (a *AutoStrategy) monitorConnection(inboxes []InboxInfo) {
    select {
    case <-a.sse.Connected():
        // SSE connected successfully, keep using it
        return
    case <-time.After(a.timeout):
        // SSE didn't connect in time, fall back to polling
        a.fallbackToPolling(inboxes)
    case <-a.ctx.Done():
        // Context canceled, don't switch
        return
    }
}

func (a *AutoStrategy) fallbackToPolling(inboxes []InboxInfo) {
    a.mu.Lock()
    defer a.mu.Unlock()

    if a.switched {
        return // Already switched
    }

    // Stop SSE
    a.sse.Stop()

    // Collect current inboxes from SSE (may have changed since Start)
    a.sse.mu.RLock()
    currentInboxes := make([]InboxInfo, 0, len(a.sse.inboxHashes))
    for hash := range a.sse.inboxHashes {
        currentInboxes = append(currentInboxes, InboxInfo{Hash: hash})
    }
    a.sse.mu.RUnlock()

    // Use provided inboxes if SSE has none
    if len(currentInboxes) == 0 {
        currentInboxes = inboxes
    }

    // Start polling
    if err := a.polling.Start(a.ctx, currentInboxes, a.handler); err != nil {
        return // Keep SSE on polling start failure
    }

    // Register reconnect handler on polling
    if a.onReconnect != nil {
        a.polling.OnReconnect(a.onReconnect)
    }

    a.active = a.polling
    a.switched = true
}

func (a *AutoStrategy) Stop() error {
    a.mu.Lock()
    defer a.mu.Unlock()

    if a.cancel != nil {
        a.cancel()
    }

    if a.active != nil {
        return a.active.Stop()
    }
    return nil
}

func (a *AutoStrategy) AddInbox(inbox InboxInfo) error {
    a.mu.RLock()
    active := a.active
    a.mu.RUnlock()

    if active != nil {
        return active.AddInbox(inbox)
    }
    return nil
}

func (a *AutoStrategy) RemoveInbox(inboxHash string) error {
    a.mu.RLock()
    active := a.active
    a.mu.RUnlock()

    if active != nil {
        return active.RemoveInbox(inboxHash)
    }
    return nil
}

func (a *AutoStrategy) OnReconnect(fn func(ctx context.Context)) {
    a.mu.Lock()
    a.onReconnect = fn
    active := a.active
    a.mu.Unlock()

    if active != nil {
        active.OnReconnect(fn)
    }
}
```

### Step 2: Update `client.go`

**Remove these fields from Client struct (lines 42-50):**
```go
// DELETE:
autoMode        bool
autoChecked     bool
sseTimeout      time.Duration
```

**Remove these methods:**
- `checkAutoModeConnection()` (lines 240-270)
- `fallbackToPolling()` (lines 272-301)

**Update `createDeliveryStrategy()` (lines 79-99):**
```go
func createDeliveryStrategy(strategy DeliveryStrategy, cfg delivery.Config) delivery.Strategy {
    switch strategy {
    case StrategySSE:
        return delivery.NewSSEStrategy(cfg)
    case StrategyPolling:
        return delivery.NewPollingStrategy(cfg)
    case StrategyAuto:
        return delivery.NewAutoStrategy(cfg)  // Changed from NewSSEStrategy
    default:
        return delivery.NewAutoStrategy(cfg)
    }
}
```

**Update `New()` function:**
- Remove `autoMode: cfg.deliveryStrategy == StrategyAuto` (line 164)
- Remove `sseTimeout` field initialization

**Update `CreateInbox()`:**
- Remove the auto-check trigger (lines 232-235):
```go
// DELETE:
if c.autoMode && isFirstInbox && !c.autoChecked {
    c.checkAutoModeConnection()
}
```

## Testing

Add `internal/delivery/auto_test.go`:
- Test SSE success path (no fallback)
- Test SSE timeout triggers polling fallback
- Test inbox add/remove delegated correctly after fallback
- Test `Name()` reflects current active strategy
