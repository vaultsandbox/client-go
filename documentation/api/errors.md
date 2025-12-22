---
title: Error Handling
description: Complete guide to error handling and retry behavior in VaultSandbox Client for Go
---

The VaultSandbox Client SDK provides comprehensive error handling with automatic retries for transient failures and specific error types for different failure scenarios.

## Error Design

The SDK uses Go's idiomatic error handling with two patterns:

1. **Sentinel errors** - For simple `errors.Is()` checks
2. **Error types** - For detailed error information via `errors.As()`

All SDK error types implement the `VaultSandboxError` interface, allowing type assertion to identify SDK-specific errors.

```go
// Sentinel errors for errors.Is() checks
var (
    ErrMissingAPIKey      error
    ErrClientClosed       error
    ErrUnauthorized       error
    ErrInboxNotFound      error
    ErrEmailNotFound      error
    ErrInboxAlreadyExists error
    ErrInvalidImportData  error
    ErrDecryptionFailed   error
    ErrSignatureInvalid   error
    ErrSSEConnection      error
    ErrInboxExpired       error
    ErrRateLimited        error
)

// Error types for errors.As() checks
type APIError struct { ... }
type NetworkError struct { ... }
type TimeoutError struct { ... }
type DecryptionError struct { ... }
type SignatureVerificationError struct { ... }
type SSEError struct { ... }
type StrategyError struct { ... }
type ValidationError struct { ... }
```

## Automatic Retries

The SDK automatically retries failed HTTP requests for transient errors. This helps mitigate temporary network issues or server-side problems.

### Default Retry Behavior

By default, requests are retried for these HTTP status codes:

- `408` - Request Timeout
- `429` - Too Many Requests (Rate Limiting)
- `500` - Internal Server Error
- `502` - Bad Gateway
- `503` - Service Unavailable
- `504` - Gateway Timeout

### Configuration

Configure retry behavior when creating the client:

```go
package main

import (
    "os"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    client, err := vaultsandbox.NewClient(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithRetries(5),                                  // Default: 3
        vaultsandbox.WithRetryOn([]int{408, 429, 500, 502, 503, 504}), // Default status codes
    )
    if err != nil {
        panic(err)
    }
    defer client.Close()
}
```

### Retry Strategy

The SDK uses **exponential backoff** for retries:

- 1st retry: `1s`
- 2nd retry: `2s`
- 3rd retry: `4s`
- And so on...

#### Example

```go
// With default settings (3 retries, 1s base delay):
// Retry schedule:
//   1st attempt: immediate
//   2nd attempt: after 1s
//   3rd attempt: after 2s
//   4th attempt: after 4s
//   Total time: up to 7 seconds + request time
```

## Sentinel Errors

Sentinel errors allow simple equality checks using `errors.Is()`.

### ErrMissingAPIKey

Returned when no API key is provided to the client.

```go
client, err := vaultsandbox.NewClient("")
if errors.Is(err, vaultsandbox.ErrMissingAPIKey) {
    log.Fatal("API key is required")
}
```

---

### ErrClientClosed

Returned when operations are attempted on a closed client.

```go
client.Close()

_, err := client.CreateInbox(ctx)
if errors.Is(err, vaultsandbox.ErrClientClosed) {
    log.Println("Client has been closed")
}
```

---

### ErrUnauthorized

Returned when the API key is invalid or expired.

```go
inbox, err := client.CreateInbox(ctx)
if errors.Is(err, vaultsandbox.ErrUnauthorized) {
    log.Fatal("Invalid or expired API key")
}
```

---

### ErrInboxNotFound

Returned when an inbox does not exist or has expired.

```go
emails, err := inbox.ListEmails(ctx)
if errors.Is(err, vaultsandbox.ErrInboxNotFound) {
    log.Println("Inbox no longer exists - it may have expired or been deleted")
}
```

---

### ErrEmailNotFound

Returned when an email does not exist.

```go
email, err := inbox.GetEmail(ctx, "non-existent-id")
if errors.Is(err, vaultsandbox.ErrEmailNotFound) {
    log.Println("Email not found - it may have been deleted")
}
```

---

### ErrInboxAlreadyExists

Returned when attempting to import an inbox that already exists.

```go
inbox, err := client.ImportInbox(ctx, exportedData)
if errors.Is(err, vaultsandbox.ErrInboxAlreadyExists) {
    log.Println("Inbox already imported in this client")
}
```

---

### ErrInvalidImportData

Returned when imported inbox data is invalid.

```go
inbox, err := client.ImportInbox(ctx, corruptedData)
if errors.Is(err, vaultsandbox.ErrInvalidImportData) {
    log.Println("Invalid import data - the exported data may be corrupted")
}
```

---

### ErrDecryptionFailed

Returned when email decryption fails.

```go
emails, err := inbox.ListEmails(ctx)
if errors.Is(err, vaultsandbox.ErrDecryptionFailed) {
    log.Println("Failed to decrypt email - this is a critical error")
}
```

---

### ErrSignatureInvalid

Returned when signature verification fails. This is a **critical security error**.

```go
inbox, err := client.CreateInbox(ctx)
if errors.Is(err, vaultsandbox.ErrSignatureInvalid) {
    log.Fatal("CRITICAL: Signature verification failed - possible MITM attack")
}
```

---

### ErrSSEConnection

Returned when the SSE connection fails.

```go
err := inbox.Subscribe(ctx, handler)
if errors.Is(err, vaultsandbox.ErrSSEConnection) {
    log.Println("SSE connection failed - consider using polling strategy")
}
```

---

### ErrInboxExpired

Returned when an inbox has exceeded its TTL.

```go
emails, err := inbox.ListEmails(ctx)
if errors.Is(err, vaultsandbox.ErrInboxExpired) {
    log.Println("Inbox has expired")
}
```

---

### ErrRateLimited

Returned when the API rate limit is exceeded.

```go
inbox, err := client.CreateInbox(ctx)
if errors.Is(err, vaultsandbox.ErrRateLimited) {
    log.Println("Rate limit exceeded - wait before retrying")
}
```

## Error Types

Error types provide detailed information about failures. Use `errors.As()` to extract them.

### APIError

Represents an HTTP error from the VaultSandbox API.

```go
type APIError struct {
    StatusCode int
    Message    string
    RequestID  string
}
```

#### Fields

- `StatusCode`: HTTP status code from the API
- `Message`: Error message from the server
- `RequestID`: Request ID for debugging (if returned by server)

#### Example

```go
import (
    "errors"
    "log"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

inbox, err := client.CreateInbox(ctx)
if err != nil {
    var apiErr *vaultsandbox.APIError
    if errors.As(err, &apiErr) {
        log.Printf("API Error (%d): %s", apiErr.StatusCode, apiErr.Message)

        switch apiErr.StatusCode {
        case 401:
            log.Println("Invalid API key")
        case 403:
            log.Println("Permission denied")
        case 429:
            log.Println("Rate limit exceeded")
        }

        if apiErr.RequestID != "" {
            log.Printf("Request ID: %s", apiErr.RequestID)
        }
    }
}
```

---

### NetworkError

Represents a network-level failure (e.g., cannot connect to server).

```go
type NetworkError struct {
    Err     error
    URL     string
    Attempt int
}
```

#### Fields

- `Err`: The underlying error (implements `Unwrap()`)
- `URL`: The URL that failed
- `Attempt`: The attempt number when the error occurred

#### Example

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
    var netErr *vaultsandbox.NetworkError
    if errors.As(err, &netErr) {
        log.Printf("Network error on attempt %d: %v", netErr.Attempt, netErr.Err)
        log.Printf("Failed URL: %s", netErr.URL)
        log.Println("Check your internet connection and server URL")
    }
}
```

---

### TimeoutError

Represents an operation that exceeded its deadline. Commonly returned by `WaitForEmail()` and `WaitForEmailCount()`.

```go
type TimeoutError struct {
    Operation string
    Timeout   time.Duration
}
```

#### Fields

- `Operation`: Description of the operation that timed out
- `Timeout`: The timeout duration that was exceeded

#### Example

```go
import (
    "errors"
    "log"
    "regexp"
    "time"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(5*time.Second),
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Welcome`)),
)
if err != nil {
    var timeoutErr *vaultsandbox.TimeoutError
    if errors.As(err, &timeoutErr) {
        log.Printf("Timed out after %v waiting for email", timeoutErr.Timeout)

        // Check what emails did arrive
        emails, _ := inbox.ListEmails(ctx)
        log.Printf("Found %d emails:", len(emails))
        for _, e := range emails {
            log.Printf("  - %s", e.Subject)
        }
    }
}
```

---

### DecryptionError

Represents a failure to decrypt email content.

```go
type DecryptionError struct {
    Stage   string // "kem", "hkdf", "aes"
    Message string
    Err     error
}
```

#### Fields

- `Stage`: The cryptographic stage where decryption failed
- `Message`: Description of the failure
- `Err`: The underlying error (implements `Unwrap()`)

#### Example

```go
emails, err := inbox.ListEmails(ctx)
if err != nil {
    var decErr *vaultsandbox.DecryptionError
    if errors.As(err, &decErr) {
        log.Printf("Decryption failed at %s stage: %s", decErr.Stage, decErr.Message)
        log.Println("This is a critical error - please report it")

        // Log for investigation
        log.Printf("Inbox: %s", inbox.EmailAddress())
        log.Printf("Time: %s", time.Now().Format(time.RFC3339))
    }
}
```

#### Handling

Decryption errors should **always** be logged and investigated as they may indicate:

- Data corruption
- SDK bug
- MITM attack (rare)
- Server-side encryption issue

---

### SignatureVerificationError

Indicates potential tampering. This is a **critical security error** that may indicate a man-in-the-middle (MITM) attack.

```go
type SignatureVerificationError struct {
    Message string
}
```

#### Example

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
    var sigErr *vaultsandbox.SignatureVerificationError
    if errors.As(err, &sigErr) {
        log.Printf("CRITICAL: Signature verification failed: %s", sigErr.Message)
        log.Println("This may indicate a MITM attack")

        // Alert security team
        alertSecurityTeam(sigErr.Message, client.BaseURL())

        // Do not continue
        os.Exit(1)
    }
}
```

#### Handling

Signature verification errors should **never** be ignored:

1. **Log immediately** with full context
2. **Alert security/operations team**
3. **Stop processing** - do not continue with the operation
4. **Investigate** - check for network issues, proxy problems, or actual attacks

---

### SSEError

Represents an SSE connection failure.

```go
type SSEError struct {
    Err      error
    Attempts int
}
```

#### Fields

- `Err`: The underlying error (implements `Unwrap()`)
- `Attempts`: Number of connection attempts made

#### Example

```go
client, _ := vaultsandbox.NewClient(
    os.Getenv("VAULTSANDBOX_API_KEY"),
    vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),
)

inbox, _ := client.CreateInbox(ctx)

err := inbox.Subscribe(ctx, func(email *vaultsandbox.Email) {
    log.Printf("New email: %s", email.Subject)
})

var sseErr *vaultsandbox.SSEError
if errors.As(err, &sseErr) {
    log.Printf("SSE connection failed after %d attempts: %v", sseErr.Attempts, sseErr.Err)
    log.Println("Falling back to polling strategy")

    // Recreate client with polling
    pollingClient, _ := vaultsandbox.NewClient(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
    )
    defer pollingClient.Close()
}
```

---

### StrategyError

Indicates a delivery strategy failure.

```go
type StrategyError struct {
    Message string
    Err     error
}
```

#### Fields

- `Message`: Description of the strategy failure
- `Err`: The underlying error (implements `Unwrap()`)

#### Example

```go
err := inbox.Subscribe(ctx, handler)

var stratErr *vaultsandbox.StrategyError
if errors.As(err, &stratErr) {
    log.Printf("Strategy error: %s", stratErr.Message)
    if stratErr.Err != nil {
        log.Printf("Underlying error: %v", stratErr.Err)
    }
}
```

---

### ValidationError

Contains multiple validation failures.

```go
type ValidationError struct {
    Errors []string
}
```

#### Example

```go
inbox, err := client.ImportInbox(ctx, invalidData)

var valErr *vaultsandbox.ValidationError
if errors.As(err, &valErr) {
    log.Println("Validation failed:")
    for _, e := range valErr.Errors {
        log.Printf("  - %s", e)
    }
}
```

## Error Handling Patterns

### Basic Error Handling

```go
package main

import (
    "context"
    "errors"
    "log"
    "os"
    "time"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    ctx := context.Background()

    client, err := vaultsandbox.NewClient(os.Getenv("VAULTSANDBOX_API_KEY"))
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        log.Fatalf("Failed to create inbox: %v", err)
    }
    log.Printf("Send email to: %s", inbox.EmailAddress())

    email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
    if err != nil {
        var timeoutErr *vaultsandbox.TimeoutError
        var apiErr *vaultsandbox.APIError
        var netErr *vaultsandbox.NetworkError

        switch {
        case errors.As(err, &timeoutErr):
            log.Printf("Timed out waiting for email after %v", timeoutErr.Timeout)
        case errors.As(err, &apiErr):
            log.Printf("API Error (%d): %s", apiErr.StatusCode, apiErr.Message)
        case errors.As(err, &netErr):
            log.Printf("Network error: %v", netErr.Err)
        default:
            log.Printf("Unexpected error: %v", err)
        }
        os.Exit(1)
    }

    log.Printf("Email received: %s", email.Subject)

    if err := inbox.Delete(ctx); err != nil {
        log.Printf("Failed to delete inbox: %v", err)
    }
}
```

### Retry with Custom Logic

```go
func waitForEmailWithRetry(ctx context.Context, inbox *vaultsandbox.Inbox, opts []vaultsandbox.WaitOption, maxAttempts int) (*vaultsandbox.Email, error) {
    var lastErr error

    for attempt := 1; attempt <= maxAttempts; attempt++ {
        email, err := inbox.WaitForEmail(ctx, opts...)
        if err == nil {
            return email, nil
        }

        lastErr = err

        var timeoutErr *vaultsandbox.TimeoutError
        if errors.As(err, &timeoutErr) {
            log.Printf("Attempt %d/%d timed out", attempt, maxAttempts)

            if attempt < maxAttempts {
                log.Println("Retrying...")
                time.Sleep(2 * time.Second)
                continue
            }
        } else {
            // Non-timeout error, don't retry
            return nil, err
        }
    }

    return nil, lastErr
}

// Usage
email, err := waitForEmailWithRetry(ctx, inbox, []vaultsandbox.WaitOption{
    vaultsandbox.WithWaitTimeout(10 * time.Second),
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Welcome`)),
}, 3)
if err != nil {
    log.Printf("Failed after retries: %v", err)
}
```

### Graceful Degradation

```go
func getEmailsWithFallback(ctx context.Context, inbox *vaultsandbox.Inbox) ([]*vaultsandbox.Email, error) {
    // Try to wait for new email
    email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(5*time.Second))
    if err == nil {
        return []*vaultsandbox.Email{email}, nil
    }

    var timeoutErr *vaultsandbox.TimeoutError
    if errors.As(err, &timeoutErr) {
        log.Println("No new emails, checking existing...")
        // Fall back to listing existing emails
        return inbox.ListEmails(ctx)
    }

    return nil, err
}
```

### Test Cleanup with Error Handling

```go
package mypackage_test

import (
    "context"
    "errors"
    "testing"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func TestEmailReceived(t *testing.T) {
    ctx := context.Background()

    client, err := vaultsandbox.NewClient(testAPIKey)
    if err != nil {
        t.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatalf("Failed to create inbox: %v", err)
    }

    // Always clean up, even if test fails
    defer func() {
        if err := inbox.Delete(ctx); err != nil {
            if errors.Is(err, vaultsandbox.ErrInboxNotFound) {
                // Inbox already deleted, that's fine
                t.Log("Inbox already deleted")
            } else {
                // Log but don't fail the test
                t.Logf("Failed to delete inbox: %v", err)
            }
        }
    }()

    sendTestEmail(inbox.EmailAddress())

    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(10*time.Second),
        vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Test`)),
    )
    if err != nil {
        t.Fatalf("Failed to receive email: %v", err)
    }

    if !strings.Contains(email.Subject, "Test") {
        t.Errorf("Expected subject to contain 'Test', got %q", email.Subject)
    }
}
```

## Best Practices

### 1. Always Handle TimeoutError

Timeouts are common in email testing. Always handle them explicitly:

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    var timeoutErr *vaultsandbox.TimeoutError
    if errors.As(err, &timeoutErr) {
        // List what emails did arrive
        emails, _ := inbox.ListEmails(ctx)
        log.Printf("Expected email not found. Received %d emails:", len(emails))
        for _, e := range emails {
            log.Printf("  - %q from %s", e.Subject, e.From)
        }
    }
    return err
}
```

### 2. Log Critical Errors

Always log signature verification and decryption errors:

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
    if errors.Is(err, vaultsandbox.ErrSignatureInvalid) || errors.Is(err, vaultsandbox.ErrDecryptionFailed) {
        // Critical security/integrity error
        logger.Critical(map[string]any{
            "error":     err.Error(),
            "timestamp": time.Now().Format(time.RFC3339),
            "serverURL": client.BaseURL(),
        })

        // Alert operations team
        alertOps(err)

        // Exit immediately
        os.Exit(1)
    }
}
```

### 3. Use errors.Is() for Sentinel Errors

Use `errors.Is()` for simple error checks:

```go
// Good: Use errors.Is() for sentinel errors
if errors.Is(err, vaultsandbox.ErrInboxNotFound) {
    // Handle not found
}

// Avoid: Direct comparison doesn't work with wrapped errors
if err == vaultsandbox.ErrInboxNotFound {
    // May not match if error is wrapped
}
```

### 4. Use errors.As() for Error Types

Use `errors.As()` to extract detailed error information:

```go
// Good: Use errors.As() for error types
var apiErr *vaultsandbox.APIError
if errors.As(err, &apiErr) {
    log.Printf("Status: %d, Message: %s", apiErr.StatusCode, apiErr.Message)
}

// Handle specific to general
var timeoutErr *vaultsandbox.TimeoutError
var netErr *vaultsandbox.NetworkError
var apiErr *vaultsandbox.APIError

switch {
case errors.As(err, &timeoutErr):
    // Handle timeout
case errors.As(err, &netErr):
    // Handle network error
case errors.As(err, &apiErr):
    // Handle API error
default:
    // Handle unknown error
}
```

### 5. Clean Up Resources

Always clean up, even when errors occur:

```go
client, err := vaultsandbox.NewClient(apiKey)
if err != nil {
    return err
}
defer client.Close()

inbox, err := client.CreateInbox(ctx)
if err != nil {
    return err
}
defer func() {
    if delErr := inbox.Delete(ctx); delErr != nil {
        log.Printf("Warning: failed to delete inbox: %v", delErr)
    }
}()

// Use inbox...
```

### 6. Use Context for Cancellation

Pass context for proper cancellation and timeout handling:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        log.Println("Operation cancelled due to context timeout")
    }
    return err
}
```

## Next Steps

- [CI/CD Integration](/client-go/testing/cicd) - Error handling in CI
- [Client API](/client-go/api/client) - Client configuration
