# Codebase Quality Assessment & Improvement Plan

This document outlines findings from a quality review of the `client-go` SDK, focusing on Go idiomaticity, performance risks, and remnants of the Node.js implementation.

## 1. High-Priority Findings (Critical Debt)

### 1.1 Brittle Error Handling
**Location:** `errors.go`
**Finding:** The `APIError.Is` method uses `strings.Contains` on server-returned messages to distinguish between error types (e.g., checking if a 404 message contains "inbox" vs "email").
**Risk:** 
- Extremely brittle: Any change in the server's error message breaks the SDK's error logic.
- Non-localized: Fails if the server returns non-English messages.
**Solution:** 
- Map 404s to specific sentinel errors within the `internal/api` layer based on context (which endpoint was called).
- Ideally, implement stable error codes in the API response.

### 1.2 Unbounded Concurrency (Node.js Callback Pattern)
**Location:** `client.go` (`handleSSEEvent`), `monitor.go` (`emitEmail`)
**Finding:** The SDK uses a callback-based event model (`OnEmail`, `OnNewEmail`). For every incoming event, a new goroutine is spawned for every registered callback without any limits or backpressure mechanism.
**Risk:** 
- Resource exhaustion: High-volume SSE streams can cause a goroutine explosion, potentially leading to OOM (Out of Memory).
- Lack of backpressure: The SDK cannot signal to the consumer to slow down.
**Solution:** 
- Implement an idiomatic Go channel-based API (e.g., `Events() <-chan *Email`).
- Use a worker pool or a limited buffer for internal event dispatching.

---

## 2. Code Smells & "Node-isms"

### 2.1 API Naming Consistency
**Location:** `internal/api/endpoints.go`
**Finding:** Many methods are suffixed with `New` (e.g., `CreateInboxNew`, `GetEmailNew`), suggesting an incomplete transition from an older API version.
**Solution:** Rename internal methods to standard Go naming and remove the legacy versions if unused.

### 2.2 Functional Interface vs. Structural Integrity
**Finding:** `InboxMonitor` tries to replicate a JavaScript `EventEmitter`.
**Solution:** Provide a more native Go experience by leveraging `context.Context` for subscription lifetimes rather than just manual `Unsubscribe()` methods where appropriate.

---

## 3. Current Strengths (Maintain These)

- **Project Layout:** Use of the `internal/` directory correctly encapsulates implementation details (`crypto`, `api`, `delivery`).
- **Configuration Pattern:** The Functional Options pattern (`WithBaseURL`, `WithTimeout`) is correctly implemented and idiomatic.
- **Sync/Async Bridge:** The `WaitForEmail` methods are well-designed "helper" methods that make the async event system easy to use in synchronous Go code.

---

## 4. Proposed Roadmap

### Phase 1: Robust Error Handling
1. Modify `internal/api` to return structured errors or specific status codes.
2. Update `errors.go` to remove string-based matching.
3. Ensure `errors.As` and `errors.Is` work across the package boundary.

### Phase 2: Idiomatic Concurrency
1. Add `Events(ctx context.Context) <-chan *Email` to the `Inbox` and `InboxMonitor` structs.
2. Refactor internal event dispatchers to use a fixed worker pool or buffered channels to prevent goroutine leaks.

### Phase 3: Cleanup
1. Refactor `internal/api/endpoints.go` to remove `New` suffixes.
2. Standardize internal naming conventions (e.g., consistent use of `Create...` vs `New...`).
