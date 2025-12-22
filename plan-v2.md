# Codebase Quality Improvement Plan v2

This document outlines the revised plan for addressing identified issues in client-go.

## Summary of Issues

1. **Architectural Inconsistency**: `WaitForEmail` always uses polling, bypassing active SSE connections
2. **Memory Leak**: `unregisterEmailCallback` sets callbacks to `nil` instead of removing them
3. **Type Safety**: Overuse of `interface{}` where generics would be appropriate

## Phase 1: Fix WaitForEmail to Leverage SSE

### Problem

In `auto.go:119-133`, `WaitForEmail` always creates a new `PollingStrategy`:

```go
func (a *AutoStrategy) WaitForEmailWithSync(...) (interface{}, error) {
    polling := NewPollingStrategy(a.cfg)  // Always polls, ignores active SSE
    return polling.WaitForEmailWithSync(...)
}
```

### Key Insight

Both SSE and Polling strategies call the same `handler` (`handleSSEEvent` in client.go:139). The callback registration lives at the `Client` level, not the strategy level. This means `OnNewEmail` callbacks already work regardless of which strategy is active.

### Solution

Implement callback-based waiting in `inbox.go` using existing infrastructure:

```go
func (i *Inbox) WaitForEmail(ctx context.Context, opts ...WaitOption) (*Email, error) {
    cfg := &waitConfig{
        timeout:      defaultWaitTimeout,
        pollInterval: defaultPollInterval,
    }
    for _, opt := range opts {
        opt(cfg)
    }

    ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
    defer cancel()

    resultCh := make(chan *Email, 1)

    // 1. Subscribe FIRST to avoid race condition
    sub := i.OnNewEmail(func(email *Email) {
        if cfg.Matches(email) {
            select {
            case resultCh <- email:
            default: // already found
            }
        }
    })
    defer sub.Unsubscribe()

    // 2. Check existing emails (handles already-arrived case)
    emails, err := i.GetEmails(ctx)
    if err != nil {
        return nil, err
    }
    for _, e := range emails {
        if cfg.Matches(e) {
            return e, nil
        }
    }

    // 3. Wait for callback or timeout
    select {
    case email := <-resultCh:
        return email, nil
    case <-ctx.Done():
        return nil, ErrEmailNotFound
    }
}
```

### Files to Modify

- `inbox.go`: Rewrite `WaitForEmail` and `WaitForEmailCount` to use callback-based approach
- `internal/delivery/auto.go`: Remove `WaitForEmail*` methods (no longer needed at strategy level)
- `internal/delivery/strategy.go`: Remove `WaitForEmail*` from `FullStrategy` interface (optional cleanup)

### Benefits

- Uses existing callback infrastructure (no new mutexes or data structures)
- Works with SSE (instant notification) and polling (when handler fires)
- Avoids race condition by subscribing before fetching
- Simpler implementation (~20 lines vs multi-file refactor)

---

## Phase 2: Fix Memory Leak in Callback Registration

### Problem

In `client.go:445-452`:

```go
func (c *Client) unregisterEmailCallback(inboxHash string, index int) {
    // ...
    callbacks[index] = nil  // Slice never shrinks - memory leak
}
```

The slice grows indefinitely as subscriptions are created and destroyed.

### Solution

Replace slice-based storage with map-based storage using incrementing IDs:

#### Changes to `client.go`

```go
// Change the type definition
type Client struct {
    // ...
    eventCallbacks   map[string]map[int]emailEventCallback // inboxHash -> id -> callback
    nextCallbackID   int
    callbacksMu      sync.RWMutex
    // ...
}

// Update initialization in New()
c := &Client{
    // ...
    eventCallbacks: make(map[string]map[int]emailEventCallback),
    // ...
}

// Update registerEmailCallback
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

// Update unregisterEmailCallback
func (c *Client) unregisterEmailCallback(inboxHash string, id int) {
    c.callbacksMu.Lock()
    defer c.callbacksMu.Unlock()

    if callbacks, ok := c.eventCallbacks[inboxHash]; ok {
        delete(callbacks, id)
        // Clean up empty maps
        if len(callbacks) == 0 {
            delete(c.eventCallbacks, inboxHash)
        }
    }
}

// Update handleSSEEvent callback iteration
func (c *Client) handleSSEEvent(event *api.SSEEvent) error {
    // ...
    c.callbacksMu.RLock()
    callbacks := c.eventCallbacks[event.InboxID]
    // Copy to slice for iteration
    callbacksCopy := make([]emailEventCallback, 0, len(callbacks))
    for _, cb := range callbacks {
        callbacksCopy = append(callbacksCopy, cb)
    }
    c.callbacksMu.RUnlock()
    // ...
}
```

### Files to Modify

- `client.go`: Update callback storage from `map[string][]callback` to `map[string]map[int]callback`

### Benefits

- Proper cleanup when callbacks unsubscribe
- Stable IDs that don't change when other callbacks unsubscribe
- No memory accumulation over time

---

## Phase 3: Implement Generics for Type Safety

### Problem

The waiting logic uses `interface{}` extensively:

- `inbox.go:125-137`: `emailFetcher()` returns `[]interface{}`
- `internal/delivery/strategy.go`: `EmailFetcher`, `EmailMatcher` use `interface{}`

### Solution

Introduce type parameters to replace `interface{}` usage.

#### Changes to `internal/delivery/strategy.go`

```go
// Generic types for fetcher and matcher
type EmailFetcher[T any] func(ctx context.Context) ([]T, error)
type EmailMatcher[T any] func(T) bool

// Generic wait functions
type WaitStrategy[T any] interface {
    WaitForEmail(ctx context.Context, inboxHash string, fetcher EmailFetcher[T], matcher EmailMatcher[T], pollInterval time.Duration) (T, error)
    WaitForEmailCount(ctx context.Context, inboxHash string, fetcher EmailFetcher[T], matcher EmailMatcher[T], count int, pollInterval time.Duration) ([]T, error)
}
```

#### Changes to `inbox.go`

```go
func (i *Inbox) emailFetcher() func(ctx context.Context) ([]*Email, error) {
    return func(ctx context.Context) ([]*Email, error) {
        return i.GetEmails(ctx)
    }
}

func emailMatcher(cfg *waitConfig) func(*Email) bool {
    return func(e *Email) bool {
        return cfg.Matches(e)
    }
}
```

### Files to Modify

- `internal/delivery/strategy.go`: Add generic type parameters
- `internal/delivery/polling.go`: Update to use generics
- `inbox.go`: Remove type conversions

### Note

If Phase 1 is implemented (callback-based waiting), the strategy-level wait methods may be removed entirely, making this phase partially or fully unnecessary. Evaluate after Phase 1 completion.

---

## Implementation Order

1. **Phase 2** (Memory Leak) - Quick fix, low risk, immediate benefit
2. **Phase 1** (WaitForEmail) - Core architectural fix
3. **Phase 3** (Generics) - Cleanup, may be reduced in scope after Phase 1

## Testing Strategy

- Add unit tests for callback registration/unregistration to verify no memory leak
- Add integration tests for `WaitForEmail` with both SSE and polling strategies
- Verify existing tests pass after each phase
