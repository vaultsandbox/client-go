# 07 - Error Handling

## Overview

The SDK uses a hierarchical error system that allows callers to:
- Check for specific error types using `errors.Is()` and `errors.As()`
- Get detailed error information when needed
- Handle errors consistently across the codebase

## Error Hierarchy

```
VaultSandboxError (interface)
├── APIError              (HTTP errors from server)
├── NetworkError          (connection failures)
├── TimeoutError          (operation timeouts)
├── InboxNotFoundError    (404 for inbox)
├── EmailNotFoundError    (404 for email)
├── InboxAlreadyExistsError (import conflict)
├── InvalidImportDataError  (validation failure)
├── DecryptionError       (crypto failure)
├── SignatureVerificationError (tampering detected)
├── SSEError              (SSE connection issues)
└── StrategyError         (strategy configuration)
```

## Implementation

### errors.go

```go
package vaultsandbox

import (
    "errors"
    "fmt"
)

// Sentinel errors for errors.Is() checks
var (
    ErrMissingAPIKey       = errors.New("API key is required")
    ErrClientClosed        = errors.New("client has been closed")
    ErrUnauthorized        = errors.New("invalid or expired API key")
    ErrInboxNotFound       = errors.New("inbox not found")
    ErrEmailNotFound       = errors.New("email not found")
    ErrInboxAlreadyExists  = errors.New("inbox already exists")
    ErrInvalidImportData   = errors.New("invalid import data")
    ErrDecryptionFailed    = errors.New("decryption failed")
    ErrSignatureInvalid    = errors.New("signature verification failed")
    ErrSSEConnection       = errors.New("SSE connection error")
    ErrInvalidSecretKeySize = errors.New("invalid secret key size")
)

// VaultSandboxError is implemented by all SDK errors
type VaultSandboxError interface {
    error
    VaultSandboxError() // marker method
}
```

### API Errors

```go
// APIError represents an HTTP error from the VaultSandbox API
type APIError struct {
    StatusCode int
    Message    string
    RequestID  string // if returned by server
}

func (e *APIError) Error() string {
    if e.Message != "" {
        return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
    }
    return fmt.Sprintf("API error %d", e.StatusCode)
}

func (e *APIError) VaultSandboxError() {}

// Is implements errors.Is for sentinel error matching
func (e *APIError) Is(target error) bool {
    switch e.StatusCode {
    case 401:
        return target == ErrUnauthorized
    case 404:
        // Could be inbox or email, caller should use specific errors
        return target == ErrInboxNotFound || target == ErrEmailNotFound
    case 409:
        return target == ErrInboxAlreadyExists
    }
    return false
}
```

### Network Errors

```go
// NetworkError represents a network-level failure
type NetworkError struct {
    Err     error
    URL     string
    Attempt int
}

func (e *NetworkError) Error() string {
    return fmt.Sprintf("network error: %v", e.Err)
}

func (e *NetworkError) Unwrap() error {
    return e.Err
}

func (e *NetworkError) VaultSandboxError() {}
```

### Timeout Errors

```go
// TimeoutError represents an operation that exceeded its deadline
type TimeoutError struct {
    Operation string
    Timeout   time.Duration
}

func (e *TimeoutError) Error() string {
    return fmt.Sprintf("%s timed out after %v", e.Operation, e.Timeout)
}

func (e *TimeoutError) VaultSandboxError() {}
```

### Crypto Errors

```go
// DecryptionError represents a failure to decrypt email content
type DecryptionError struct {
    Stage   string // "kem", "hkdf", "aes"
    Message string
    Err     error
}

func (e *DecryptionError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("decryption failed at %s: %v", e.Stage, e.Err)
    }
    return fmt.Sprintf("decryption failed at %s: %s", e.Stage, e.Message)
}

func (e *DecryptionError) Unwrap() error {
    return e.Err
}

func (e *DecryptionError) Is(target error) bool {
    return target == ErrDecryptionFailed
}

func (e *DecryptionError) VaultSandboxError() {}

// SignatureVerificationError indicates potential tampering
type SignatureVerificationError struct {
    Message string
}

func (e *SignatureVerificationError) Error() string {
    return fmt.Sprintf("signature verification failed: %s", e.Message)
}

func (e *SignatureVerificationError) Is(target error) bool {
    return target == ErrSignatureInvalid
}

func (e *SignatureVerificationError) VaultSandboxError() {}
```

### SSE Errors

```go
// SSEError represents an SSE connection failure
type SSEError struct {
    Err      error
    Attempts int
}

func (e *SSEError) Error() string {
    return fmt.Sprintf("SSE connection failed after %d attempts: %v", e.Attempts, e.Err)
}

func (e *SSEError) Unwrap() error {
    return e.Err
}

func (e *SSEError) Is(target error) bool {
    return target == ErrSSEConnection
}

func (e *SSEError) VaultSandboxError() {}
```

### Validation Errors

```go
// ValidationError contains multiple validation failures
type ValidationError struct {
    Errors []string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed: %v", e.Errors)
}

func (e *ValidationError) VaultSandboxError() {}
```

## Error Handling Patterns

### Basic Error Checking

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
    if errors.Is(err, vaultsandbox.ErrUnauthorized) {
        // Re-authenticate or prompt for new API key
    }
    return err
}
```

### Extracting Error Details

```go
email, err := inbox.WaitForEmail(ctx)
if err != nil {
    var timeoutErr *vaultsandbox.TimeoutError
    if errors.As(err, &timeoutErr) {
        log.Printf("Operation timed out after %v", timeoutErr.Timeout)
    }

    var apiErr *vaultsandbox.APIError
    if errors.As(err, &apiErr) {
        log.Printf("API returned %d: %s", apiErr.StatusCode, apiErr.Message)
    }

    return err
}
```

### Handling Crypto Errors

```go
email, err := inbox.GetEmail(ctx, emailID)
if err != nil {
    // Signature errors are security-critical
    if errors.Is(err, vaultsandbox.ErrSignatureInvalid) {
        log.Error("SECURITY: Possible tampering detected!")
        // Alert security team, don't retry
        return err
    }

    // Decryption errors might be transient
    if errors.Is(err, vaultsandbox.ErrDecryptionFailed) {
        var decErr *vaultsandbox.DecryptionError
        if errors.As(err, &decErr) {
            log.Printf("Decryption failed at stage: %s", decErr.Stage)
        }
    }

    return err
}
```

### Retry Logic

```go
func fetchWithRetry(ctx context.Context, inbox *vaultsandbox.Inbox) (*vaultsandbox.Email, error) {
    var lastErr error

    for attempt := 0; attempt < 3; attempt++ {
        email, err := inbox.GetEmail(ctx, emailID)
        if err == nil {
            return email, nil
        }

        // Don't retry security errors
        if errors.Is(err, vaultsandbox.ErrSignatureInvalid) {
            return nil, err
        }

        // Don't retry 4xx errors (except 429)
        var apiErr *vaultsandbox.APIError
        if errors.As(err, &apiErr) {
            if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
                if apiErr.StatusCode != 429 {
                    return nil, err
                }
            }
        }

        lastErr = err
        time.Sleep(time.Second * time.Duration(1<<attempt))
    }

    return nil, lastErr
}
```

## Critical Error Handling

### SignatureVerificationError

This error is **security-critical** and should:
1. Be logged immediately with full context
2. Never be silently ignored or retried
3. Potentially trigger security alerts

```go
func (i *Inbox) processEmail(raw *api.RawEmail) (*Email, error) {
    if err := crypto.VerifySignature(raw.EncryptedMetadata); err != nil {
        // Log full details for investigation
        log.WithFields(log.Fields{
            "inbox":    i.emailAddress,
            "emailId":  raw.ID,
            "error":    err,
        }).Error("SECURITY: Signature verification failed")

        return nil, &SignatureVerificationError{
            Message: err.Error(),
        }
    }
    // ... proceed with decryption
}
```

### DecryptionError

May indicate:
- Corrupted data in transit
- Key mismatch
- Server-side issue

```go
if errors.Is(err, vaultsandbox.ErrDecryptionFailed) {
    // Log for debugging
    log.WithError(err).Warn("Decryption failed")

    // Could try fetching again, but don't loop forever
    // The SDK already has retry logic built in
}
```

## Error Wrapping Guidelines

1. **Always wrap with context**: Use `fmt.Errorf("context: %w", err)`
2. **Preserve original errors**: Use `%w` for wrapping
3. **Add actionable information**: Include IDs, paths, etc.

```go
func (i *Inbox) GetEmail(ctx context.Context, emailID string) (*Email, error) {
    raw, err := i.client.apiClient.GetEmail(ctx, i.emailAddress, emailID)
    if err != nil {
        return nil, fmt.Errorf("get email %s from inbox %s: %w",
            emailID, i.emailAddress, err)
    }
    // ...
}
```
