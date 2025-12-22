# Refactoring Plan: Idiomatic Go Patterns

## Phase 1: Fix Context Propagation

**Goal:** Ensure user-provided contexts flow through all I/O operations.

### Changes

**inbox.go**

1. Remove the broken `decryptEmail` wrapper (line 375-376):
   ```go
   // DELETE THIS
   func (i *Inbox) decryptEmail(raw *api.RawEmail) (*Email, error) {
       return i.decryptEmailWithContext(context.Background(), raw)
   }
   ```

2. Rename `decryptEmailWithContext` → `decryptEmail` and update signature:
   ```go
   func (i *Inbox) decryptEmail(ctx context.Context, raw *api.RawEmail) (*Email, error)
   ```

3. Update all callers to pass context:
   - `GetEmails` (line 105): already has ctx, pass it through
   - `GetEmail` (line 122): already has ctx, pass it through

**client.go**

4. Update `handleSSEEvent` (line 388-431):
   - Accept `ctx context.Context` as parameter
   - Pass ctx to `inbox.GetEmail`

5. Update `handleEmailEvent` in SSE callback registration to pass context from the event loop.

**internal/delivery/sse.go**

6. The event handler signature should accept context:
   ```go
   type EventHandler func(ctx context.Context, event *api.SSEEvent) error
   ```

7. Pass the connection context to the handler in `connect()` (line 232).

### Files Affected
- `inbox.go`
- `client.go`
- `internal/delivery/sse.go`

---

## Phase 2: Subscription-Based Callbacks

**Goal:** Replace integer ID returns with `func()` closures for cleaner unsubscription.

### Changes

**client.go**

1. Change `registerEmailCallback` to return a cleanup function:
   ```go
   func (c *Client) registerEmailCallback(inboxHash string, callback emailEventCallback) func() {
       c.callbacksMu.Lock()
       defer c.callbacksMu.Unlock()

       if c.eventCallbacks[inboxHash] == nil {
           c.eventCallbacks[inboxHash] = make(map[int]emailEventCallback)
       }

       id := c.nextCallbackID
       c.nextCallbackID++
       c.eventCallbacks[inboxHash][id] = callback

       // Return closure that captures id and inboxHash
       return func() {
           c.unregisterEmailCallback(inboxHash, id)
       }
   }
   ```

2. Keep `unregisterEmailCallback` as internal helper (unexported).

**inbox.go**

3. Simplify `inboxEmailSubscription`:
   ```go
   type inboxEmailSubscription struct {
       unsubscribe func()
       once        sync.Once
   }

   func (s *inboxEmailSubscription) Unsubscribe() {
       s.once.Do(s.unsubscribe)
   }
   ```

4. Update `OnNewEmail`:
   ```go
   func (i *Inbox) OnNewEmail(callback InboxEmailCallback) Subscription {
       unsub := i.client.registerEmailCallback(i.inboxHash, func(inbox *Inbox, email *Email) {
           callback(email)
       })

       return &inboxEmailSubscription{unsubscribe: unsub}
   }
   ```

**monitor.go**

5. Change `callbackIndices` from `map[string]int` to `map[string]func()`:
   ```go
   type InboxMonitor struct {
       // ...
       unsubscribers map[string]func()
   }
   ```

6. Update `startMonitoring` and `Stop` accordingly.

### Files Affected
- `client.go`
- `inbox.go`
- `monitor.go`

---

## Execution Order

Phases are independent and can be done in parallel, but the recommended sequence:

1. **Phase 1** first - it's a correctness bug
2. **Phase 2** second - API cleanup, lower risk

Each phase should include tests verifying the fix before moving to the next.
