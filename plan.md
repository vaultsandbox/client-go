# Codebase Quality Review and Improvement Plan for client-go

This document outlines the findings from a codebase quality review and the proposed plan for addressing identified issues.

## Summary of Initial Findings (from codebase_investigator)

The `client-go` library generally exhibits **high quality**, a good structure, and largely follows **idiomatic Go patterns**. The initial concern that it might be a direct port from Node.js appears unfounded.

**Key Strengths:**
*   **API Design:** Clean, uses functional options, `context.Context` correctly, and a clear object hierarchy.
*   **Error Handling:** Exemplary error handling strategy using sentinel errors, rich custom types, and a marker interface.
*   **Robust Internals:** Sophisticated internal polling and SSE strategies with adaptive backoff, jitter, and sync-based optimizations.

**Primary Areas for Improvement & Code Smells:**

1.  **Major Architectural Inconsistency (AutoStrategy):**
    *   **Description:** The `AutoStrategy` (the default) exhibits inconsistent behavior. While it uses SSE for real-time events (`OnNewEmail`), the blocking `WaitForEmail` function *always* falls back to creating a new, temporary polling strategy.
    *   **Impact:** This bypasses any active SSE connection for `WaitForEmail`, leading to higher latency and unexpected behavior for users who expect `WaitForEmail` to leverage the real-time capabilities of SSE.
    *   **Location:** `internal/delivery/auto.go`, `internal/delivery/sse.go`, `inbox.go`

2.  **Code Smell (Type Safety - `interface{}` overuse):**
    *   **Description:** The library's waiting logic (in `internal/delivery` and `inbox.go`) heavily relies on `interface{}` to achieve generic functionality.
    *   **Impact:** Sacrifices type safety, making the code more error-prone and harder to reason about.
    *   **Recommendation:** Refactor to use Go 1.18+ generics (the library already requires Go 1.21+).
    *   **Location:** `internal/delivery/strategy.go`, `inbox.go`

3.  **Code Smell (Memory Leak - `unregisterEmailCallback`):**
    *   **Description:** The `unregisterEmailCallback` function in `client.go` sets a callback to `nil` in a slice instead of actively removing it.
    *   **Impact:** The underlying slice will grow indefinitely as subscriptions are created and destroyed, leading to a memory leak over time with prolonged usage.
    *   **Location:** `client.go`

## Proposed Action Plan

My plan to address these findings is as follows, in order of priority:

1.  **Refactor `SSEStrategy` to handle `WaitForEmail` directly:**
    *   **Goal:** Eliminate the architectural inconsistency by allowing `SSEStrategy` to leverage its active SSE connection for `WaitForEmail` calls.
    *   **Steps:**
        *   Add a subscription mechanism to `SSEStrategy` (`waiters` map, `waiterMu` mutex).
        *   Modify the SSE event handler (`connect` function) to notify relevant `waiters` when an email arrives for a subscribed inbox.
        *   Implement `WaitForEmail` and `WaitForEmailWithSync` methods directly within `SSEStrategy`. These methods will:
            *   First attempt to fetch existing emails.
            *   If not found, subscribe to new email events for the inbox.
            *   Wait on a channel for a notification from the SSE handler.
            *   Re-fetch and match the email upon notification or timeout.
        *   Update `AutoStrategy` to use `SSEStrategy`'s `WaitForEmail` methods when SSE is the active strategy.

2.  **Implement Generics for Waiting Logic:**
    *   **Goal:** Improve type safety and remove `interface{}` overuse in waiting logic.
    *   **Steps:**
        *   Identify the `interface{}` parameters in `EmailFetcher`, `EmailMatcher`, and related waiting functions within `internal/delivery` and `inbox.go`.
        *   Introduce type parameters to these interfaces and functions (e.g., `EmailFetcher[T]`, `EmailMatcher[T]`).
        *   Update all calling sites and implementations to use the new generic types.

3.  **Fix Memory Leak in `unregisterEmailCallback`:**
    *   **Goal:** Ensure that removed callbacks are properly deleted from the slice.
    *   **Steps:**
        *   Modify `unregisterEmailCallback` in `client.go` to remove the element from the slice rather than setting it to `nil`.

I will proceed with **Step 1: Refactor `SSEStrategy` to handle `WaitForEmail` directly** first, as it addresses the most significant architectural inconsistency.