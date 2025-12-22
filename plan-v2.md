# Codebase Quality Assessment & Improvement Plan (v2)

This document outlines findings from a quality review of the `client-go` SDK, focusing on Go idiomaticity and code hygiene.

---

## Phase 1: Robust Error Handling

**Location:** `errors.go`, `internal/api`

**Problem:** The `APIError.Is` method uses `strings.Contains` on server-returned messages to distinguish between error types (e.g., checking if a 404 message contains "inbox" vs "email").

**Risks:**
- Brittle: Any change in the server's error message breaks the SDK's error logic
- Non-localized: Fails if the server returns non-English messages

**Solution:**
1. Modify `internal/api` to return context-aware errors based on which endpoint was called (e.g., a 404 from `/inbox/:id/emails/:emailId` → `ErrEmailNotFound`)
2. Update `errors.go` to remove string-based matching
3. Define clear sentinel errors: `ErrInboxNotFound`, `ErrEmailNotFound`, etc.
4. Ensure `errors.As` and `errors.Is` work correctly across the package boundary

**Files to modify:**
- `internal/api/endpoints.go` - return typed errors based on endpoint context
- `internal/api/errors.go` - define internal error types
- `errors.go` - update public error handling

---

## Phase 2: Internal API Cleanup

**Location:** `internal/api/endpoints.go`

**Problem:** Methods are suffixed with `New` (e.g., `CreateInboxNew`, `GetEmailNew`), suggesting an incomplete transition from an older API version.

**Solution:**
1. Rename internal methods to standard Go naming:
   - `CreateInboxNew` → `CreateInbox`
   - `GetEmailsNew` → `GetEmails`
   - `GetEmailNew` → `GetEmail`
   - `GetEmailRawNew` → `GetEmailRaw`
   - `MarkEmailAsReadNew` → `MarkEmailAsRead`
   - `DeleteEmailNew` → `DeleteEmail`

2. Update all internal usages of these methods

3. Remove any legacy/unused versions if they exist

**Files to modify:**
- `internal/api/endpoints.go` - rename methods
- Any files in `internal/` that call these methods
- No documentation changes needed (public API names are already correct)

---

## Deferred: Idiomatic Concurrency

The callback-based event model (`OnEmail`, `OnNewEmail`) was identified as a potential concern for high-volume scenarios. However, given the expected low-volume usage patterns, this refactoring is **not prioritized**.

The current implementation is functional and can be revisited if:
- Processing requirements exceed ~100 emails/second
- Goroutine leaks are observed in long-running monitors
- Users report resource exhaustion issues

---

## Current Strengths (Preserve)

- **Project Layout:** Proper use of `internal/` for encapsulation
- **Functional Options:** Idiomatic `WithBaseURL`, `WithTimeout` pattern
- **Sync/Async Bridge:** Well-designed `WaitForEmail` helpers
- **Documentation:** Comprehensive README and docs already use correct public API names
