# Documentation Plan: Channel-Based Watch API (Commit 2bc9793)

## Summary of Code Changes

The commit `2bc97938a85b29f5dec37ed6d1ce46c45e68b144` ("channel refactor") replaces the callback-based real-time email monitoring API with a more idiomatic Go channel-based API.

### Removed API

| Type/Function | Description |
|---------------|-------------|
| `InboxMonitor` | Struct for monitoring multiple inboxes |
| `InboxMonitor.OnEmail()` | Register callback for new emails |
| `InboxMonitor.Unsubscribe()` | Stop monitoring |
| `Subscription` | Interface with `Unsubscribe()` method |
| `Inbox.OnNewEmail()` | Subscribe to single inbox with callback |
| `Client.MonitorInboxes()` | Create multi-inbox monitor |
| `InboxEmailCallback` | Callback type `func(email *Email)` |

### Added API

| Type/Function | Signature | Description |
|---------------|-----------|-------------|
| `InboxEvent` | `struct { Inbox *Inbox; Email *Email }` | Event struct for multi-inbox watching |
| `Inbox.Watch()` | `func (i *Inbox) Watch(ctx context.Context) <-chan *Email` | Watch single inbox via channel |
| `Client.WatchInboxes()` | `func (c *Client) WatchInboxes(ctx context.Context, inboxes ...*Inbox) <-chan *InboxEvent` | Watch multiple inboxes via channel |

### Key Behavioral Changes

1. **Lifecycle Control**: Context cancellation replaces explicit `Unsubscribe()` calls
2. **Channel Semantics**: Receive-only channels replace callback functions
3. **Non-blocking Sends**: Internal `notifyWatchers` uses non-blocking sends (drops if channel full)
4. **Buffer Size**: Channels are created with buffer size 16
5. **Cleanup**: Channels close automatically when context is cancelled

---

## Files Requiring Updates

### 1. README.md

**Section**: Real-time Monitoring Example (lines ~404-455)

**Current (Callback-based)**:
```go
subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
    fmt.Printf("New email received: %q\n", email.Subject)
})
// ...
subscription.Unsubscribe()
```

**New (Channel-based)**:
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

for email := range inbox.Watch(ctx) {
    fmt.Printf("New email received: %q\n", email.Subject)
}
```

**Additional Changes**:
- Update `InboxMonitor` API reference section (lines ~488-524) to document `WatchInboxes`
- Update method list in API Reference section

---

### 2. documentation/guides/real-time.md

**Status**: Entire file needs comprehensive rewrite

**Current Structure** (callback-based):
- Basic Subscription (`inbox.OnNewEmail`)
- Monitoring Multiple Inboxes (`client.MonitorInboxes`)
- Unsubscribing (subscription management)
- Real-World Patterns (callback-based)
- Testing with Real-Time Monitoring
- Error Handling (callback-based)
- Advanced Patterns (rate-limiting, worker pools using callbacks)

**New Structure** (channel-based):

#### 2.1 Basic Watching

```go
// Watch single inbox
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

for email := range inbox.Watch(ctx) {
    fmt.Printf("New email: %s\n", email.Subject)
}
```

#### 2.2 Watching Multiple Inboxes

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

for event := range client.WatchInboxes(ctx, inbox1, inbox2) {
    fmt.Printf("Email in %s: %s\n", event.Inbox.EmailAddress(), event.Email.Subject)
}
```

#### 2.3 Stopping Early (context cancellation)

```go
ctx, cancel := context.WithCancel(context.Background())

go func() {
    for email := range inbox.Watch(ctx) {
        if strings.Contains(email.Subject, "Welcome") {
            cancel() // Stop watching after finding welcome email
            return
        }
    }
}()
```

#### 2.4 Select-based Processing

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

emails := inbox.Watch(ctx)
for {
    select {
    case email, ok := <-emails:
        if !ok {
            return // Channel closed
        }
        processEmail(email)
    case <-ctx.Done():
        return
    }
}
```

#### 2.5 Worker Pool Pattern (channel-based)

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

emails := inbox.Watch(ctx)

// Fan-out to workers
var wg sync.WaitGroup
for i := 0; i < 3; i++ {
    wg.Add(1)
    go func(workerID int) {
        defer wg.Done()
        for email := range emails {
            fmt.Printf("Worker %d processing: %s\n", workerID, email.Subject)
            processEmail(email)
        }
    }(i)
}
wg.Wait()
```

#### 2.6 Testing Patterns

```go
func TestRealTimeEmailProcessing(t *testing.T) {
    client, _ := vaultsandbox.New(apiKey)
    defer client.Close()

    ctx := context.Background()
    inbox, _ := client.CreateInbox(ctx)
    defer inbox.Delete(ctx)

    watchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    var received []*vaultsandbox.Email
    done := make(chan struct{})

    go func() {
        for email := range inbox.Watch(watchCtx) {
            received = append(received, email)
            if len(received) >= 2 {
                cancel()
                close(done)
                return
            }
        }
    }()

    sendEmail(inbox.EmailAddress(), "Test 1")
    sendEmail(inbox.EmailAddress(), "Test 2")

    <-done
    if len(received) != 2 {
        t.Errorf("expected 2 emails, got %d", len(received))
    }
}
```

#### 2.7 Comparison Table Update

| Aspect | `Inbox.Watch()` | `Client.WatchInboxes()` |
|--------|-----------------|-------------------------|
| **Scope** | Single inbox | Multiple inboxes |
| **Return Type** | `<-chan *Email` | `<-chan *InboxEvent` |
| **Inbox Info** | Implicit (from receiver) | `event.Inbox` field |
| **Lifecycle** | Context cancellation | Context cancellation |
| **Cleanup** | Automatic on ctx cancel | Automatic on ctx cancel |

---

### 3. documentation/api/client.md

**Section**: MonitorInboxes (lines 263-303)

**Action**: Replace with `WatchInboxes` documentation

**New Content**:

```markdown
### WatchInboxes

Returns a channel that receives events from multiple inboxes. The channel closes when the context is cancelled.

\`\`\`go
func (c *Client) WatchInboxes(ctx context.Context, inboxes ...*Inbox) <-chan *InboxEvent
\`\`\`

#### Parameters

- `ctx`: Context for cancellation - when cancelled, the channel closes and all watchers are cleaned up
- `inboxes`: Variadic list of inbox instances to watch

#### Returns

- `<-chan *InboxEvent` - Receive-only channel of inbox events

#### InboxEvent Type

\`\`\`go
type InboxEvent struct {
    Inbox *Inbox  // The inbox that received the email
    Email *Email  // The received email
}
\`\`\`

#### Example

\`\`\`go
inbox1, _ := client.CreateInbox(ctx)
inbox2, _ := client.CreateInbox(ctx)

watchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
defer cancel()

for event := range client.WatchInboxes(watchCtx, inbox1, inbox2) {
    fmt.Printf("New email in %s: %s\n", event.Inbox.EmailAddress(), event.Email.Subject)
}
\`\`\`

#### Behavior

- Returns immediately closed channel if no inboxes provided
- Channel has buffer size of 16
- Non-blocking sends: if channel buffer is full, events may be dropped
- All internal goroutines and watchers are cleaned up when context is cancelled
```

**Also update**: Import/export example (line 512-515) that references `OnNewEmail`

---

### 4. documentation/api/inbox.md

**Section 1**: OnNewEmail (lines 289-364)

**Action**: Replace with `Watch` documentation

**New Content**:

```markdown
### Watch

Returns a channel that receives emails as they arrive. The channel closes when the context is cancelled.

\`\`\`go
func (i *Inbox) Watch(ctx context.Context) <-chan *Email
\`\`\`

#### Parameters

- `ctx`: Context for cancellation - when cancelled, the channel closes

#### Returns

- `<-chan *Email` - Receive-only channel of emails

#### Example

\`\`\`go
inbox, err := client.CreateInbox(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Watching: %s\n", inbox.EmailAddress())

watchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
defer cancel()

for email := range inbox.Watch(watchCtx) {
    fmt.Printf("New email: %q\n", email.Subject)
    fmt.Printf("From: %s\n", email.From)
}
```

#### Behavior

- Channel has buffer size of 16
- Non-blocking sends: if buffer is full, events may be dropped
- Channel closes automatically when context is cancelled
- Watcher is automatically unregistered on context cancellation

#### Best Practice

Use context for lifecycle management:

\`\`\`go
func TestEmailFlow(t *testing.T) {
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }
    defer inbox.Delete(ctx)

    watchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    for email := range inbox.Watch(watchCtx) {
        // Process email
        if foundDesiredEmail(email) {
            cancel() // Stop watching early
            break
        }
    }
}
\`\`\`
```

**Section 2**: InboxMonitor (lines 504-639)

**Action**: Remove entire section (InboxMonitor no longer exists)

---

### 5. documentation/guides/waiting-for-emails.md

**Status**: Minor update needed

The guide focuses on `WaitForEmail` and `WaitForEmailCount` which still use channels internally but the public API remains the same. However, the "Process as They Arrive" section (lines 122-142) could benefit from mentioning `Watch()` as an alternative.

**Add note**:
```markdown
> **Tip**: For real-time processing without specifying a count, consider using `inbox.Watch(ctx)`
> which returns a channel. See [Real-time Monitoring](/client-go/guides/real-time/).
```

---

## Implementation Checklist

- [ ] **README.md**
  - [ ] Update Real-time Monitoring example (lines ~404-455)
  - [ ] Update API Reference section - replace `MonitorInboxes` with `WatchInboxes`
  - [ ] Update API Reference section - replace `OnNewEmail` with `Watch`
  - [ ] Add `InboxEvent` type documentation

- [ ] **documentation/guides/real-time.md**
  - [ ] Replace Basic Subscription section with `Watch()` examples
  - [ ] Replace Multiple Inboxes section with `WatchInboxes()` examples
  - [ ] Remove Unsubscribing section (replace with context cancellation)
  - [ ] Update Real-World Patterns with channel-based examples
  - [ ] Update Testing section with channel-based patterns
  - [ ] Update Error Handling section
  - [ ] Update Advanced Patterns (worker pool, rate limiting) with channel idioms
  - [ ] Update comparison table

- [ ] **documentation/api/client.md**
  - [ ] Replace `MonitorInboxes` section with `WatchInboxes`
  - [ ] Document `InboxEvent` type
  - [ ] Update Complete Example if it uses old API
  - [ ] Update ImportInboxFromFile example (line 512-515)

- [ ] **documentation/api/inbox.md**
  - [ ] Replace `OnNewEmail` section with `Watch`
  - [ ] Remove entire `InboxMonitor` section
  - [ ] Update Complete Inbox Example
  - [ ] Remove `Subscription` type reference

- [ ] **documentation/guides/waiting-for-emails.md**
  - [ ] Add tip about `Watch()` in "Process as They Arrive" section

---

## Migration Guide (for users)

### Before (Callback-based)

```go
// Single inbox
subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
    fmt.Printf("New email: %s\n", email.Subject)
})
defer subscription.Unsubscribe()

// Multiple inboxes
monitor, _ := client.MonitorInboxes([]*vaultsandbox.Inbox{inbox1, inbox2})
monitor.OnEmail(func(inbox *vaultsandbox.Inbox, email *vaultsandbox.Email) {
    fmt.Printf("Email in %s: %s\n", inbox.EmailAddress(), email.Subject)
})
defer monitor.Unsubscribe()
```

### After (Channel-based)

```go
// Single inbox
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

for email := range inbox.Watch(ctx) {
    fmt.Printf("New email: %s\n", email.Subject)
}

// Multiple inboxes
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

for event := range client.WatchInboxes(ctx, inbox1, inbox2) {
    fmt.Printf("Email in %s: %s\n", event.Inbox.EmailAddress(), event.Email.Subject)
}
```

### Key Differences

1. **Lifecycle**: Use `context.WithCancel()` or `context.WithTimeout()` instead of `Unsubscribe()`
2. **Iteration**: Use `for email := range channel` instead of callbacks
3. **Early Exit**: Call `cancel()` to stop watching instead of `subscription.Unsubscribe()`
4. **Multi-inbox**: Use variadic `WatchInboxes(ctx, inbox1, inbox2)` instead of slice `MonitorInboxes([]*Inbox{...})`
5. **Event Type**: `InboxEvent` struct replaces callback parameters for multi-inbox watching
