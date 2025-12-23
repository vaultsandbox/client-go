# Channel Close Race in Watch/WatchInboxes

## Explanation
The watch APIs (`Inbox.Watch`, `Client.WatchInboxes`) close their output channel after calling `unsubscribe()`. The `subscriptionManager.notify` method copies the subscription list under a lock, releases the lock, and then invokes callbacks. This means a callback can be in-flight after `unsubscribe()` removes it. If the cancel goroutine closes the channel while that callback is about to send, the send will panic with `send on closed channel`.

This can happen even when the user intentionally cancels. The race is about timing, not intent.

Affected paths:
- `Inbox.Watch` close path
- `Client.WatchInboxes` close path
- `subscriptionManager.notify` (copies subs, then calls callbacks after unlock)

## Possible Fixes

### 1) Do not close the channel
**Idea:** Remove `close(ch)` and let the consumer stop on context cancellation.

**Pros:**
- Smallest change, no new synchronization.
- Removes panic risk immediately.

**Cons:**
- Consumers waiting on channel close will block unless they also watch the context.
- Slightly less ergonomic for `for range ch` usage unless documented.

### 2) Add a done channel and guard sends
**Idea:** Create `done` channel and have the callback `select` on `done` before sending. Close `done` before `unsubscribe()`.

**Pros:**
- Keeps channel close semantics.
- Prevents sends after cancel without complex sync.

**Cons:**
- Still need to ensure no send happens after channel close; requires ordering and careful select patterns.
- Slightly more code in each watch method.

### 3) Track in-flight callbacks (WaitGroup)
**Idea:** In `subscriptionManager.notify`, increment a counter before invoking callbacks and decrement after. `unsubscribe()` waits for in-flight callbacks to finish before returning, and the watch close happens after that wait.

**Pros:**
- Preserves current API and channel close semantics.
- Strong guarantee: no callback runs after unsubscribe completes.

**Cons:**
- More complex synchronization in `subscriptionManager`.
- Potential for a slow callback to block unsubscribe/close.

### 4) Non-blocking send + channel never closed
**Idea:** Keep current non-blocking send, remove channel close, and document that consumers should use context to stop.

**Pros:**
- Simplest safe behavior.
- Existing drop-on-full behavior stays intact.

**Cons:**
- Same ergonomic tradeoff as option 1.

## Recommendation
For minimal change with highest safety, choose either option 1 or 4 and document that users should select on context. If preserving channel-close semantics is important, option 3 provides the strongest guarantee but requires more changes.
