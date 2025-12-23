# Plan: Tweak Event Dispatching Logic

**Goal:** Improve reliability and idiomatic Go compliance in the event dispatching mechanism within `client.go`.

## Context
Currently, the `notifyWatchers` function uses a non-blocking send with a `default` case. If the channel buffer (16) is full, the event is silently dropped. While unlikely in low-volume testing scenarios, this pattern is non-idiomatic and introduces theoretical flakiness.

## Proposed Change
Replace the non-blocking send with a **context-aware blocking send**. This ensures that the system waits for the consumer (test code) to be ready or for the operation to be canceled, rather than discarding data.

### 1. Update `notifyWatchers` Signature and Logic
**File:** `client.go`

Change the function signature to accept `context.Context` and use it in the `select` statement.

**From:**
```go
func (c *Client) notifyWatchers(inboxHash string, email *Email) {
    // ...
    for _, ch := range watchersCopy {
        select {
        case ch <- email:
        default:
            // Non-blocking: drop if channel is full
        }
    }
}
```

**To:**
```go
func (c *Client) notifyWatchers(ctx context.Context, inboxHash string, email *Email) {
    // ...
    for _, ch := range watchersCopy {
        select {
        case ch <- email:
        case <-ctx.Done():
            return
        }
    }
}
```

### 2. Update Callers
The `notifyWatchers` function is called in two places that need updating:

1.  **`syncInbox`**:
    *   Pass the existing `ctx` from `syncInbox(ctx context.Context, ...)` to `notifyWatchers`.

2.  **`handleSSEEvent`**:
    *   Pass the `ctx` (which is already available and includes a timeout in the existing code) to `notifyWatchers`.

## Benefits
*   **Determinism:** Eliminates the possibility of silent data loss.
*   **Idiomatic Go:** Follows standard patterns for channel communication, respecting context cancellation/timeouts.
*   **Safety:** Prevents goroutine leaks or hangs by tying the blocking operation to the request context.
