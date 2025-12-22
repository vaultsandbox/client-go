# Codebase Quality & Idiomatic Go Analysis

## Overview
This document outlines the findings from a code quality review of the `client-go` library. The primary focus was to identify non-idiomatic Go patterns, particularly those stemming from a Node.js influence, and to assess the robustness of concurrency and error handling.

## Key Findings

### 1. Node.js "Artifacts" & Code Smells
The codebase exhibits several patterns that are common in JavaScript/Node.js but foreign or suboptimal in Go.

*   **Callback Management (Integer IDs):**
    *   **Observation:** `client.go` uses an integer-based ID system (`nextCallbackID`) for registering and unregistering callbacks (e.g., `registerEmailCallback`). This mimics JavaScript's `setTimeout` / `clearTimeout` pattern.
    *   **Issue:** It leaks implementation details (state management of IDs) to the user and requires them to manage integers.
    *   **Recommendation:** Return a `Subscription` interface with an `Unsubscribe()` method, or a simple closure `func()` that cancels the subscription. This encapsulates the state.

*   **Unbounded Concurrency:**
    *   **Observation:** `handleSSEEvent` spawns a new goroutine for every single event callback (`go cb(inbox, email)`).
    *   **Issue:** While Go routines are lightweight, this lacks backpressure. In a high-load scenario (even if not expected), this could lead to resource exhaustion. It mirrors the "fire and forget" nature of the Node.js event loop but without the implicit serialization.
    *   **Recommendation:** For low-volume testing tools, this is acceptable, but a Worker Pool pattern or a buffered channel would be more idiomatic and safer.

*   **Static Event Strategy:**
    *   **Observation:** `SSEStrategy.AddInbox` adds an inbox to a local map but does not trigger a reconnection. The server is not notified of the new inbox until the connection breaks and reconnects naturally.
    *   **Issue:** This leads to a state where the local client thinks it's monitoring an inbox, but the server stream is unaware of it.
    *   **Recommendation:** Use a control channel (e.g., `chan struct{}`) to signal the connection loop to gracefully restart with the updated parameters immediately upon modification.

### 2. Context Propagation
Proper usage of `context.Context` is critical in Go for cancellation and timeouts.

*   **Broken Context Chains:**
    *   **Observation:** The `decryptEmail` method in `inbox.go` creates a fresh `context.Background()` internally.
    *   **Issue:** This breaks the cancellation chain. If a user calls `inbox.GetEmails(ctx)` with a timeout, the internal network request to fetch the full email body (triggered inside `decryptEmail`) effectively ignores that timeout.
    *   **Recommendation:** The `ctx` argument must be passed down through every layer that performs I/O, including private helper methods like `decryptEmail`.

### 3. API Client & Error Handling

*   **Manual Retry Logic:**
    *   **Observation:** The `doWithRetry` method manually handles `io.Seeker` to reset request bodies between retries.
    *   **Recommendation:** Use the `GetBody` field in `http.Request` (available since Go 1.8+), which is the standard way to handle request body replayability.

*   **Error Wrapping:**
    *   **Observation:** The code relies on a generic `wrapError` helper function.
    *   **Recommendation:** While useful, idiomatic Go prefers explicit wrapping at the call site using `fmt.Errorf("doing operation x: %w", err)`. This creates a human-readable chain of events (breadcrumbs) rather than just transforming the error type.

## Summary of Recommendations
To align this library with Go best practices:

1.  **Refactor Callbacks:** Transition from ID-based callbacks to `Subscription` interfaces.
2.  **Fix Contexts:** Audit all methods to ensure `context.Context` is threaded through to all I/O operations.
3.  **Dynamic SSE:** Implement a mechanism to restart the SSE stream dynamically when inboxes are added or removed.
4.  **Modernize HTTP Client:** Leverage standard library features for body rewinding and retries.
