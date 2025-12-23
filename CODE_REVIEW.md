# Codebase Quality Review

## Summary
The codebase is functional and well-structured but exhibits several "porting smells" — patterns that are common in other languages (JavaScript/Node.js, Java) but are not idiomatic in Go. Refactoring these will make the library feel more "Go-native," simplify concurrency, and reduce the chance of bugs.

## Key Findings

### 1. JavaScript-Style Event Callbacks
**Location:** `client.go`, `inbox.go`

**Issue:**
The library uses a callback-based event system that mimics Node.js event emitters or RxJS subscriptions.
- `OnNewEmail` takes a function and returns a `Subscription` object.
- The user is responsible for calling `Unsubscribe()`.
- Internally, `Client` manages a complex `map[string]map[int]callback` with explicit locking.

**Why it's not Idiomatic Go:**
- Go uses **Channels** for signaling and event streams.
- Channels compose naturally with `select`, `range`, and `context.Context` for cancellation.
- Manual `Unsubscribe` is prone to memory leaks if the user forgets it (or if a panic occurs before `defer`).

**Current Code:**
```go
// inbox_wait.go
sub := i.OnNewEmail(func(email *Email) {
    if cfg.Matches(email) {
        select {
        case resultCh <- email:
        default:
        }
    }
})
defer sub.Unsubscribe()
```

**Idiomatic Go Alternative:**
```go
// Using a channel that closes when context is done
events := i.Watch(ctx)
for email := range events {
    // handle email
}
```

### 2. Concurrency & "Callback-to-Channel" Bridge
**Location:** `inbox_wait.go`

**Issue:**
The `WaitForEmail` function artificially bridges the callback world to the Go channel world. It creates a channel, subscribes a callback that pushes to that channel, and then waits on the channel. This adds unnecessary layers and locking overhead.

### 3. Heavy "Service" Objects in Delivery
**Location:** `internal/delivery/`

**Issue:**
The `Strategy` interface and its implementations (`SSEStrategy`, `PollingStrategy`) are designed as heavy service objects with `Start`, `Stop`, `AddInbox`, and `RemoveInbox`. While this works, it requires managing complex internal state and mutexes.

**Idiomatic Go:**
Prefer small, composable functions or workers that accept a channel of updates.

### 4. Error Handling
**Location:** `errors.go`

**Issue:**
The error handling is robust but perhaps over-engineered for a low-throughput testing library. There is a lot of wrapping and custom error types. Go 1.13+ error wrapping (`%w`) is often sufficient without needing a deep hierarchy of custom types unless callers specifically need to match against them.

## Recommendations

1. **Refactor `OnNewEmail` to return a Channel:**
   Replace the callback system with a method that returns a read-only channel `<-chan *Email`. Pass a `context.Context` to handle cancellation/unsubscription automatically.

2. **Simplify `WaitForEmail`:**
   Once `OnNewEmail` returns a channel, `WaitForEmail` becomes a simple `select` statement on that channel.

3. **Remove `Subscription` interface:**
   Let `context.CancelFunc` or simply closing the channel handle cleanup.
