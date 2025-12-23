# Plan: Simplify Error Handling

## Goal
Reduce error type complexity while maintaining useful sentinel errors for `errors.Is()` checks.

## Current State

### File: `errors.go`

**Sentinel errors (15 total):**
- `ErrMissingAPIKey`, `ErrClientClosed`, `ErrUnauthorized`
- `ErrInboxNotFound`, `ErrEmailNotFound`, `ErrInboxAlreadyExists`
- `ErrInvalidImportData`, `ErrDecryptionFailed`, `ErrSignatureInvalid`
- `ErrServerKeyMismatch`, `ErrSSEConnection`, `ErrInvalidSecretKeySize`
- `ErrInboxExpired`, `ErrRateLimited`

**Custom error types (8 total):**
1. `VaultSandboxError` interface (marker interface)
2. `ResourceType` enum
3. `APIError` - HTTP errors with status code, message, request ID
4. `NetworkError` - network failures with URL and attempt count
5. `TimeoutError` - operation timeouts
6. `DecryptionError` - decryption failures with stage info
7. `SignatureVerificationError` - signature/key mismatch
8. `SSEError` - SSE connection failures with attempt count
9. `ValidationError` - multiple validation errors
10. `StrategyError` - delivery strategy errors

### Problems:
1. `VaultSandboxError` marker interface adds no value (callers don't use it)
2. `TimeoutError` duplicates `context.DeadlineExceeded`
3. `ValidationError` is unused in the codebase
4. `StrategyError` is only used once for a simple message
5. `NetworkError` and `SSEError` could just wrap with `%w`
6. Too many types for a testing library

## Target State

Keep only what callers actually need to match on:

### Keep (simplified):
```go
// Sentinel errors
var (
    ErrMissingAPIKey      = errors.New("API key is required")
    ErrClientClosed       = errors.New("client has been closed")
    ErrUnauthorized       = errors.New("invalid or expired API key")
    ErrInboxNotFound      = errors.New("inbox not found")
    ErrEmailNotFound      = errors.New("email not found")
    ErrInboxAlreadyExists = errors.New("inbox already exists")
    ErrDecryptionFailed   = errors.New("decryption failed")
    ErrSignatureInvalid   = errors.New("signature verification failed")
    ErrRateLimited        = errors.New("rate limit exceeded")
)

// APIError - keep this, callers may want status codes
type APIError struct {
    StatusCode int
    Message    string
    RequestID  string
}
```

### Remove:
1. `VaultSandboxError` interface - marker interfaces aren't idiomatic Go
2. `ResourceType` enum - fold into `APIError.Is()` logic or remove
3. `NetworkError` - use `fmt.Errorf("network error: %w", err)`
4. `TimeoutError` - use `context.DeadlineExceeded` directly
5. `DecryptionError` - use `fmt.Errorf("decryption failed at %s: %w", stage, ErrDecryptionFailed)`
6. `SignatureVerificationError` - use `fmt.Errorf(...): %w", ErrSignatureInvalid)`
7. `SSEError` - use `fmt.Errorf("SSE connection failed: %w", err)`
8. `ValidationError` - unused, delete
9. `StrategyError` - use plain `fmt.Errorf`
10. `ErrInvalidImportData` - unused or fold into validation
11. `ErrServerKeyMismatch` - fold into `ErrSignatureInvalid`
12. `ErrSSEConnection` - not needed if SSEError removed
13. `ErrInvalidSecretKeySize` - fold into decryption error
14. `ErrInboxExpired` - check if used, likely removable

## Implementation Steps

### Step 1: Audit usage of each error type

Search codebase for each error type/sentinel to confirm what's actually used:
- `grep -r "ErrInboxExpired"` etc.
- Check if callers do `errors.Is()` or `errors.As()` on each type

### Step 2: Simplify `errors.go`

New file structure:

```go
package vaultsandbox

import (
    "errors"
    "fmt"
)

// Sentinel errors for errors.Is() checks
var (
    ErrMissingAPIKey      = errors.New("API key is required")
    ErrClientClosed       = errors.New("client has been closed")
    ErrUnauthorized       = errors.New("invalid or expired API key")
    ErrInboxNotFound      = errors.New("inbox not found")
    ErrEmailNotFound      = errors.New("email not found")
    ErrInboxAlreadyExists = errors.New("inbox already exists")
    ErrDecryptionFailed   = errors.New("decryption failed")
    ErrSignatureInvalid   = errors.New("signature verification failed")
    ErrRateLimited        = errors.New("rate limit exceeded")
)

// APIError represents an HTTP error from the VaultSandbox API.
type APIError struct {
    StatusCode int
    Message    string
    RequestID  string
}

func (e *APIError) Error() string {
    if e.RequestID != "" {
        return fmt.Sprintf("API error %d: %s (request_id: %s)", e.StatusCode, e.Message, e.RequestID)
    }
    if e.Message != "" {
        return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
    }
    return fmt.Sprintf("API error %d", e.StatusCode)
}

func (e *APIError) Is(target error) bool {
    switch e.StatusCode {
    case 401:
        return target == ErrUnauthorized
    case 404:
        return target == ErrInboxNotFound || target == ErrEmailNotFound
    case 409:
        return target == ErrInboxAlreadyExists
    case 429:
        return target == ErrRateLimited
    }
    return false
}

// wrapError converts internal API errors to public errors.
func wrapError(err error) error {
    // ... simplified version
}
```

### Step 3: Update error creation sites

Find all places that create custom error types and replace with simple wrapping:

```go
// Before
return &DecryptionError{Stage: "aes", Err: err}

// After
return fmt.Errorf("decryption failed at aes: %w", ErrDecryptionFailed)
```

```go
// Before
return &NetworkError{Err: err, URL: url}

// After
return fmt.Errorf("network error on %s: %w", url, err)
```

### Step 4: Update internal/api error types

Check if `internal/api/errors.go` has similar duplication and simplify there too.

### Step 5: Update tests

- Remove tests for deleted error types
- Ensure `errors.Is()` checks still work with new structure
- Update any `errors.As()` calls that matched on removed types

## Files to Modify

1. `errors.go` - Major simplification
2. `internal/api/errors.go` - Review and simplify if needed
3. `internal/crypto/*.go` - Update decryption error creation
4. `internal/delivery/*.go` - Update SSE/strategy error creation
5. `*_test.go` - Update error assertions

## Final Error Count

| Category | Before | After |
|----------|--------|-------|
| Sentinel errors | 15 | 9 |
| Custom types | 10 | 1 (`APIError`) |
| Interfaces | 1 | 0 |

~70% reduction in error-related code.
