# Plan: Fix Watcher Concurrency Hazards

## Problem

The watcher system has a race condition and awkward ownership:

**Race condition in `notifyWatchers()` (client.go:606-620):**
```go
watchersCopy := make([]chan<- *Email, len(watchers))
copy(watchersCopy, watchers)
c.watchersMu.RUnlock()  // Lock released

for _, ch := range watchersCopy {
    select {
    case ch <- email:  // PANIC if channel closed between copy and send
    default:
    }
}
```

**Cleanup in `Watch()` (inbox_wait.go:23-27):**
```go
go func() {
    <-ctx.Done()
    cleanup()   // Removes from map
    close(ch)   // Closes channel - can race with notifyWatchers send
}()
```

Between releasing the lock and sending, the channel can be closed by the cleanup goroutine, causing a panic.

## Solution

Replace exposed channels with a callback-based subscription API that manages lifecycle internally.

## Files to Modify

| File | Changes |
|------|---------|
| `subscription.go` | **NEW** - Subscription manager |
| `client.go` | Replace watchers map with subscription manager |
| `inbox_wait.go` | Update `Watch()` to use new API |

## Subscription Manager Design

```go
// subscription.go
type subscription struct {
    id        string
    inboxHash string
    callback  func(*Email)
    active    atomic.Bool
}

type subscriptionManager struct {
    mu     sync.RWMutex
    subs   map[string]map[string]*subscription // inboxHash -> subID -> subscription
    nextID atomic.Uint64
}
```

## Implementation Steps

### Step 1: Create `subscription.go`

```go
package vaultsandbox

import (
    "context"
    "strconv"
    "sync"
    "sync/atomic"
)

// subscription represents an active email subscription.
type subscription struct {
    id        string
    inboxHash string
    callback  func(*Email)
    active    atomic.Bool
}

// subscriptionManager handles email subscriptions with safe lifecycle management.
// It ensures callbacks are never invoked after unsubscription completes.
type subscriptionManager struct {
    mu     sync.RWMutex
    subs   map[string]map[string]*subscription // inboxHash -> subID -> subscription
    nextID atomic.Uint64
}

// newSubscriptionManager creates a new subscription manager.
func newSubscriptionManager() *subscriptionManager {
    return &subscriptionManager{
        subs: make(map[string]map[string]*subscription),
    }
}

// subscribe registers a callback for emails arriving at the given inbox.
// The callback will be invoked synchronously when emails arrive.
// Returns an unsubscribe function that must be called to clean up.
func (m *subscriptionManager) subscribe(inboxHash string, callback func(*Email)) func() {
    id := strconv.FormatUint(m.nextID.Add(1), 10)

    sub := &subscription{
        id:        id,
        inboxHash: inboxHash,
        callback:  callback,
    }
    sub.active.Store(true)

    m.mu.Lock()
    if m.subs[inboxHash] == nil {
        m.subs[inboxHash] = make(map[string]*subscription)
    }
    m.subs[inboxHash][id] = sub
    m.mu.Unlock()

    return func() {
        m.unsubscribe(inboxHash, id)
    }
}

// subscribeWithContext registers a callback that auto-unsubscribes when ctx is done.
func (m *subscriptionManager) subscribeWithContext(ctx context.Context, inboxHash string, callback func(*Email)) {
    unsubscribe := m.subscribe(inboxHash, callback)

    go func() {
        <-ctx.Done()
        unsubscribe()
    }()
}

// unsubscribe removes a subscription. Safe to call multiple times.
func (m *subscriptionManager) unsubscribe(inboxHash, subID string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if inboxSubs, ok := m.subs[inboxHash]; ok {
        if sub, ok := inboxSubs[subID]; ok {
            sub.active.Store(false) // Mark inactive before removing
            delete(inboxSubs, subID)
            if len(inboxSubs) == 0 {
                delete(m.subs, inboxHash)
            }
        }
    }
}

// notify calls all registered callbacks for the given inbox.
// Callbacks are invoked synchronously under read lock - they should be fast.
func (m *subscriptionManager) notify(inboxHash string, email *Email) {
    m.mu.RLock()
    inboxSubs := m.subs[inboxHash]
    if len(inboxSubs) == 0 {
        m.mu.RUnlock()
        return
    }

    // Copy subscriptions to avoid holding lock during callbacks
    // But check active flag before invoking
    subs := make([]*subscription, 0, len(inboxSubs))
    for _, sub := range inboxSubs {
        subs = append(subs, sub)
    }
    m.mu.RUnlock()

    for _, sub := range subs {
        if sub.active.Load() {
            sub.callback(email)
        }
    }
}

// clear removes all subscriptions. Called during Client.Close().
func (m *subscriptionManager) clear() {
    m.mu.Lock()
    defer m.mu.Unlock()

    for _, inboxSubs := range m.subs {
        for _, sub := range inboxSubs {
            sub.active.Store(false)
        }
    }
    m.subs = make(map[string]map[string]*subscription)
}
```

### Step 2: Update `client.go`

**Replace watcher fields in Client struct (lines 39-40):**
```go
// DELETE:
watchers   map[string][]chan<- *Email
watchersMu sync.RWMutex

// ADD:
subs *subscriptionManager
```

**Update `New()` initialization (around line 160):**
```go
// DELETE:
watchers: make(map[string][]chan<- *Email),

// ADD:
subs: newSubscriptionManager(),
```

**Replace `addWatcher` (lines 574-582):**
```go
// DELETE entire function, replaced by subs.subscribe()
```

**Replace `removeWatcher` (lines 585-600):**
```go
// DELETE entire function, replaced by subs.unsubscribe()
```

**Replace `notifyWatchers` (lines 603-622):**
```go
// DELETE entire function

// Replace calls to notifyWatchers with:
c.subs.notify(inboxHash, email)
```

**Update `Close()` (around line 684):**
```go
// DELETE:
c.watchersMu.Lock()
c.watchers = make(map[string][]chan<- *Email)
c.watchersMu.Unlock()

// ADD:
c.subs.clear()
```

### Step 3: Update `inbox_wait.go`

**Update `Watch()` (lines 18-30):**
```go
func (i *Inbox) Watch(ctx context.Context) <-chan *Email {
    ch := make(chan *Email, 16)

    // Subscribe with callback that sends to channel
    unsubscribe := i.client.subs.subscribe(i.inboxHash, func(email *Email) {
        select {
        case ch <- email:
        default:
            // Buffer full, drop (same behavior as before)
        }
    })

    // Cleanup goroutine
    go func() {
        <-ctx.Done()
        unsubscribe()  // Callback won't fire after this returns
        close(ch)      // Safe - no more sends possible
    }()

    return ch
}
```

### Step 4: Update `WatchInboxes()` in `client.go` (lines 483-534)

```go
func (c *Client) WatchInboxes(ctx context.Context, inboxes ...*Inbox) <-chan *InboxEvent {
    ch := make(chan *InboxEvent, 16)

    if len(inboxes) == 0 {
        close(ch)
        return ch
    }

    // Track unsubscribe functions
    var unsubscribes []func()
    var mu sync.Mutex

    for _, inbox := range inboxes {
        inbox := inbox
        unsub := c.subs.subscribe(inbox.inboxHash, func(email *Email) {
            select {
            case ch <- &InboxEvent{Inbox: inbox, Email: email}:
            default:
            }
        })
        mu.Lock()
        unsubscribes = append(unsubscribes, unsub)
        mu.Unlock()
    }

    // Cleanup goroutine
    go func() {
        <-ctx.Done()
        mu.Lock()
        for _, unsub := range unsubscribes {
            unsub()
        }
        mu.Unlock()
        close(ch)
    }()

    return ch
}
```

## Key Safety Guarantees

1. **No send-on-closed-channel**: The `active` atomic flag is set to false before removing from map. Callbacks check this flag before invoking.

2. **Clear ownership**: `subscriptionManager` owns all subscription lifecycle.

3. **Idempotent cleanup**: `unsubscribe()` is safe to call multiple times.

4. **No blocking under lock**: Callbacks are copied out before invoking, lock released.

## Testing

Add `subscription_test.go`:
```go
func TestSubscriptionConcurrency(t *testing.T)
func TestCallbackNotInvokedAfterUnsubscribe(t *testing.T)
func TestContextCancellation(t *testing.T)
func TestClearStopsAllCallbacks(t *testing.T)
```
