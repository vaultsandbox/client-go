# Codebase Investigation: SSE & "Deaf Strategy" Hypothesis

## Summary
During the code review, a potential race condition/bug was identified in the `SSEStrategy` implementation within `internal/delivery/sse.go`. While your tests pass, the code analysis suggests that new inboxes created *after* the initial client connection might not be monitored immediately unless the backend has specific undocumented behavior.

## The Hypothesis: "Deaf SSE"
1.  **Client Initialization:** `client.New(key)` creates the client and immediately calls `strategy.Start()`.
2.  **Empty Connection:** `Start()` launches a goroutine that calls `apiClient.OpenEventStream(ctx, hashes)`. At this point, `hashes` is empty `[]string{}`.
3.  **Active Connection:** The HTTP connection to the server is established with this empty list.
4.  **Inbox Creation:** The user calls `client.CreateInbox()`. This calls `strategy.AddInbox(hash)`.
5.  **The Gap:** `AddInbox` adds the hash to a local map, but **does not** signal the active `connectLoop` to restart. The existing HTTP connection continues running, listening to the original "empty list."
6.  **The Risk:** The client believes it is watching the new inbox, but the server is not sending events for it (because it wasn't in the initial list).

## Why Your Tests Might Be Passing (False Positives)
If your tests pass despite this logic, one of the following must be true:

### 1. Backend "Wildcard" Behavior
*   **Question:** Does `OpenEventStream` with an empty list `[]` imply "subscribe to ALL inboxes for this API Key"?
*   **Check:** Verify backend logic or API documentation. If this is true, the code is fine.

### 2. Race Condition Luck
*   **Scenario:** `WaitForEmail` performs an initial check: `emails, err := i.GetEmails(ctx)`.
*   **Luck:** If the email arrives at the server *before* your test calls `WaitForEmail`, this initial check finds it, and the broken SSE subscription is never actually needed.
*   **Check:** Add a `time.Sleep(5 * time.Second)` *inside* your test, right *before* you send the email. This ensures the email arrives *after* `WaitForEmail` has started waiting. If the test fails/timeouts, the SSE subscription is indeed broken.

### 3. Silent Fallback
*   **Scenario:** The SSE connection fails silently or times out during `New()`, causing `AutoStrategy` to fall back to `PollingStrategy`.
*   **Logic:** `PollingStrategy` *does* check its map on every tick, so it correctly picks up new inboxes.
*   **Check:** Enable debug logging or inspect `client.strategy.Name()` during a test. If it says `auto:polling`, you aren't testing SSE.

## Action Items
1.  **Test the Backend:** Manually call the `/events` endpoint with no inbox IDs. Send an email to a new inbox. See if an event appears in the stream.
2.  **Stress Test:** Write a test that establishes the client, waits 5 seconds (to ensure SSE is connected), creates an inbox, and *then* triggers an email.
3.  **Verify Strategy:** Print `client.strategy.Name()` in your tests to confirm you are actually using SSE.

## Code References
*   `internal/delivery/sse.go`: `AddInbox` and `connect` methods.
*   `internal/delivery/auto.go`: Fallback logic.
*   `inbox.go`: `WaitForEmail` logic.
