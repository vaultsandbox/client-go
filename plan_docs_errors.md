# Plan: Update Documentation for Simplified Error Types

## Summary

The error handling was simplified per `plan_errors.md`. This document tracks the documentation updates needed to reflect those changes.

## What Changed in Code

### Removed Sentinel Errors
- `ErrServerKeyMismatch` - folded into `ErrSignatureInvalid`
- `ErrSSEConnection` - removed (SSEError type removed)
- `ErrInvalidSecretKeySize` - internal only, removed from public API
- `ErrInboxExpired` - removed (unused)

### Removed Error Types
- `VaultSandboxError` interface - marker interface removed
- `TimeoutError` - use `context.DeadlineExceeded` instead
- `DecryptionError` - use `ErrDecryptionFailed` sentinel
- `SSEError` - removed
- `ValidationError` - removed (authresults package has its own)
- `StrategyError` - removed

### Kept (Simplified)
- 10 sentinel errors: `ErrMissingAPIKey`, `ErrClientClosed`, `ErrUnauthorized`, `ErrInboxNotFound`, `ErrEmailNotFound`, `ErrInboxAlreadyExists`, `ErrInvalidImportData`, `ErrDecryptionFailed`, `ErrSignatureInvalid`, `ErrRateLimited`
- 3 error types: `APIError`, `NetworkError`, `SignatureVerificationError`
- `SignatureVerificationError.Is()` now always returns `ErrSignatureInvalid` (regardless of `IsKeyMismatch`)

---

## Files to Update

### 1. README.md

**Lines 669-694: Error Types section**

Current:
```markdown
- **`ErrSSEConnection`** — Server-Sent Events connection error
- **`ErrInboxExpired`** — Inbox has expired
...
- **`TimeoutError`** — Operation timeouts with `Operation` and `Timeout` fields
- **`DecryptionError`** — Decryption failures with `Stage` and `Message` fields
- **`SSEError`** — SSE connection failures with `Attempts` field
```

Replace with:
```markdown
**Error Structs:**

- **`APIError`** — HTTP API errors with `StatusCode`, `Message`, `RequestID`, and `ResourceType` fields
- **`NetworkError`** — Network-level failures with `Err`, `URL`, and `Attempt` fields
- **`SignatureVerificationError`** — Signature/key mismatch failures with `Message` and `IsKeyMismatch` fields
```

**Lines 730-745: Example code**

Replace `TimeoutError` example with `context.DeadlineExceeded`:
```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(5*time.Second))
if err != nil {
    var apiErr *vaultsandbox.APIError

    switch {
    case errors.Is(err, context.DeadlineExceeded):
        fmt.Println("Timed out waiting for email")
    case errors.As(err, &apiErr):
        fmt.Printf("API Error (%d): %s\n", apiErr.StatusCode, apiErr.Message)
    case errors.Is(err, vaultsandbox.ErrSignatureInvalid):
        fmt.Println("CRITICAL: Signature verification failed!")
    default:
        fmt.Printf("Unexpected error: %v\n", err)
    }
    return
}
```

---

### 2. documentation/api/errors.md

This is the main errors documentation file. Major rewrites needed:

**Remove sections:**
- `### ErrServerKeyMismatch` (lines ~258-277)
- `### ErrSSEConnection` (lines ~278-290)
- `### ErrInvalidSecretKeySize` (lines ~291-303)
- `### ErrInboxExpired` (lines ~304-316)
- `### TimeoutError` (lines ~452-500)
- `### DecryptionError` (lines ~501-546)
- `### SSEError` (lines ~598-643)
- `### StrategyError` (lines ~644-675)
- `### ValidationError` (lines ~676-700)
- `### VaultSandboxError Interface` section

**Update sections:**
- `### SignatureVerificationError` - note that it always matches `ErrSignatureInvalid`
- Update all example code that uses removed types
- Update the error type listing at the top

---

### 3. documentation/guides/managing-inboxes.md

**Lines ~924-992: Error handling examples**

Remove/update:
```go
// Remove these:
if errors.Is(err, vaultsandbox.ErrSSEConnection) { ... }
if errors.Is(err, vaultsandbox.ErrInboxExpired) { ... }
var timeoutErr *vaultsandbox.TimeoutError
var decryptErr *vaultsandbox.DecryptionError
var sseErr *vaultsandbox.SSEError
var valErr *vaultsandbox.ValidationError
var stratErr *vaultsandbox.StrategyError
```

Replace with simplified examples using:
- `context.DeadlineExceeded` for timeouts
- `ErrSignatureInvalid` for all signature errors
- `ErrDecryptionFailed` sentinel instead of `DecryptionError` type

---

### 4. documentation/concepts/inboxes.md

**Lines ~907-1038: Error reference table and examples**

Update error table to remove:
- `ErrServerKeyMismatch`
- `ErrSSEConnection`
- `ErrInvalidSecretKeySize`
- `ErrInboxExpired`

Remove error type definitions for:
- `TimeoutError`
- `DecryptionError`
- `SSEError`
- `ValidationError`
- `StrategyError`

Remove `VaultSandboxError` interface section.

---

### 5. documentation/concepts/emails.md

**Line ~695:**
```go
var decryptErr *vaultsandbox.DecryptionError
```

Replace with:
```go
if errors.Is(err, vaultsandbox.ErrDecryptionFailed) {
    // Handle decryption failure
}
```

---

### 6. documentation/advanced/strategies.md

**Lines ~566-577, ~713:**
```go
if errors.Is(err, vaultsandbox.ErrSSEConnection) { ... }
var sseErr *vaultsandbox.SSEError
case errors.Is(err, vaultsandbox.ErrInboxExpired):
```

Remove `ErrSSEConnection` checks. For SSE failures, just check the underlying error or use a generic network error check.

Remove `ErrInboxExpired` check.

---

### 7. documentation/configuration.md

**Lines ~1001-1022:**

Remove from error type list:
- `TimeoutError`
- `DecryptionError`
- `SSEError`
- `ValidationError`
- `StrategyError`
- `ErrInboxExpired`
- `ErrSSEConnection`
- `ErrInvalidSecretKeySize`

Update example code accordingly.

---

## Recommended Approach

1. **Start with README.md** - it's the most visible
2. **Update documentation/api/errors.md** - comprehensive error reference
3. **Update guides and concepts** - user-facing documentation
4. **Update configuration.md** - reference material

## New Error Handling Patterns to Document

### Timeout Handling
```go
// Old (removed):
var timeoutErr *vaultsandbox.TimeoutError
if errors.As(err, &timeoutErr) { ... }

// New (idiomatic Go):
if errors.Is(err, context.DeadlineExceeded) { ... }
```

### Signature Errors
```go
// Old:
if errors.Is(err, vaultsandbox.ErrServerKeyMismatch) { ... }
if errors.Is(err, vaultsandbox.ErrSignatureInvalid) { ... }

// New (both cases match ErrSignatureInvalid):
if errors.Is(err, vaultsandbox.ErrSignatureInvalid) {
    // Could be signature failure OR key mismatch
    var sigErr *vaultsandbox.SignatureVerificationError
    if errors.As(err, &sigErr) && sigErr.IsKeyMismatch {
        // Specific handling for key mismatch (potential MITM)
    }
}
```

### Decryption Errors
```go
// Old:
var decErr *vaultsandbox.DecryptionError
if errors.As(err, &decErr) { ... }

// New:
if errors.Is(err, vaultsandbox.ErrDecryptionFailed) { ... }
```
