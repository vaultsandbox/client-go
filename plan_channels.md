# Plan: Refactor Callbacks to Channels

## Goal
Replace the JavaScript-style callback/subscription system with idiomatic Go channels and context-based cancellation.

## Current State

### Files involved:
- `monitor.go` - `Subscription` interface, `InboxMonitor` with callback pattern
- `inbox_wait.go` - `OnNewEmail()` callback, `WaitForEmail()`, `WaitForEmailCount()`
- `client.go` - `registerEmailCallback()`, `unregisterEmailCallback()`, callback maps

### Current flow:
```
User calls inbox.OnNewEmail(callback)
  → registers callback in map[string]map[int]callback
  → returns Subscription with Unsubscribe()
  → user must call Unsubscribe() to clean up
```

### Problems:
1. Callback registration with manual `Unsubscribe()` is error-prone
2. `WaitForEmail` bridges callbacks back to channels (unnecessary indirection)
3. Complex internal state: `eventCallbacks map[string]map[int]emailEventCallback`
4. Spawns goroutine per callback invocation (`go cb(inbox, email)`)

## Target State

### New API:
```go
// Single inbox watching
emails := inbox.Watch(ctx)
for email := range emails {
    // handle email
}

// Multi-inbox watching
emails := client.WatchInboxes(ctx, inbox1, inbox2)
for event := range emails {
    fmt.Printf("Email in %s: %s\n", event.Inbox.EmailAddress(), event.Email.Subject)
}

// WaitForEmail becomes thin wrapper
email, err := inbox.WaitForEmail(ctx, WithSubject("Welcome"))
```

### Benefits:
- Cancellation via `context.Context` (automatic cleanup)
- Composable with `select` statements
- No manual unsubscribe
- Simpler internal state

## Implementation Steps

### Step 1: Add channel-based `Watch` method to `Inbox`

Create new method in `inbox_wait.go`:

```go
// Watch returns a channel that receives emails as they arrive.
// The channel closes when the context is cancelled.
func (i *Inbox) Watch(ctx context.Context) <-chan *Email
```

Internal implementation:
- Create buffered channel
- Register with client's internal event system
- Spawn goroutine that forwards events to channel
- Close channel when context done

### Step 2: Add `WatchInboxes` to `Client`

```go
// InboxEvent represents an email arriving in a specific inbox.
type InboxEvent struct {
    Inbox *Inbox
    Email *Email
}

// WatchInboxes returns a channel that receives events from multiple inboxes.
func (c *Client) WatchInboxes(ctx context.Context, inboxes ...*Inbox) <-chan *InboxEvent
```

### Step 3: Simplify `WaitForEmail` and `WaitForEmailCount`

Rewrite to use `Watch()` internally:

```go
func (i *Inbox) WaitForEmail(ctx context.Context, opts ...WaitOption) (*Email, error) {
    cfg := buildConfig(opts)
    ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
    defer cancel()

    // Check existing emails first
    emails, err := i.GetEmails(ctx)
    if err != nil {
        return nil, err
    }
    for _, e := range emails {
        if cfg.Matches(e) {
            return e, nil
        }
    }

    // Watch for new emails
    for email := range i.Watch(ctx) {
        if cfg.Matches(email) {
            return email, nil
        }
    }
    return nil, ctx.Err()
}
```

### Step 4: Refactor internal event distribution in `Client`

Replace callback maps with channel fan-out:

```go
type Client struct {
    // Remove these:
    // eventCallbacks map[string]map[int]emailEventCallback
    // nextCallbackID int
    // callbacksMu    sync.RWMutex

    // Add:
    watchers   map[string][]chan<- *Email  // inboxHash -> channels
    watchersMu sync.RWMutex
}

func (c *Client) addWatcher(inboxHash string, ch chan<- *Email) func() {
    // Returns cleanup function
}

func (c *Client) notifyWatchers(inbox *Inbox, email *Email) {
    // Non-blocking send to all watchers
}
```

### Step 5: Deprecate and remove old API

1. Mark as deprecated (optional, since new library):
   - `Subscription` interface
   - `OnNewEmail()` method
   - `InboxMonitor` type

2. Delete:
   - `monitor.go` entirely
   - `inboxEmailSubscription` in `inbox_wait.go`
   - Callback-related fields/methods in `client.go`

### Step 6: Update tests

- Update `inbox_wait_test.go` to use new API
- Delete `monitor_test.go` or rewrite for `WatchInboxes`
- Add new tests for `Watch()` cancellation behavior

## Migration Summary

| Old API | New API |
|---------|---------|
| `inbox.OnNewEmail(cb); defer sub.Unsubscribe()` | `for email := range inbox.Watch(ctx)` |
| `client.MonitorInboxes(inboxes).OnEmail(cb)` | `for event := range client.WatchInboxes(ctx, inboxes...)` |
| `inbox.WaitForEmail(ctx, opts...)` | (unchanged, but simpler internally) |

## Files to Modify

1. `inbox_wait.go` - Add `Watch()`, simplify `WaitForEmail`/`WaitForEmailCount`
2. `client.go` - Replace callback maps with watcher channels, add `WatchInboxes()`
3. `monitor.go` - Delete entirely
4. `monitor_test.go` - Delete or rewrite
5. `inbox_wait_test.go` - Update tests
