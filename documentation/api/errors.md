---
title: Error Handling
description: Complete guide to error handling and retry behavior in VaultSandbox Client for Go
---

The VaultSandbox Client SDK provides comprehensive error handling with automatic retries for transient failures and specific error types for different failure scenarios.

## Error Design

The SDK uses Go's idiomatic error handling with two patterns:

1. **Sentinel errors** - For simple `errors.Is()` checks
2. **Error types** - For detailed error information via `errors.As()`

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
    ErrRateLimited        error
)

// Resource type for distinguishing 404 errors
type ResourceType string
const (
    ResourceUnknown ResourceType
    ResourceInbox   ResourceType
    ResourceEmail   ResourceType
)

// Error types for errors.As() checks
type APIError struct { ... }
type NetworkError struct { ... }
type SignatureVerificationError struct { ... }
```

The `authresults` package also provides errors for email authentication validation:

```go
import "github.com/vaultsandbox/client-go/authresults"

// Sentinel errors for authentication validation
var (
    authresults.ErrSPFFailed        error
    authresults.ErrDKIMFailed       error
    authresults.ErrDMARCFailed      error
    authresults.ErrReverseDNSFailed error
    authresults.ErrNoAuthResults    error
)

// Error type for multiple validation failures
type authresults.ValidationError struct { ... }
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
    client, err := vaultsandbox.New(
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
client, err := vaultsandbox.New("")
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
emails, err := inbox.GetEmails(ctx)
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
emails, err := inbox.GetEmails(ctx)
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

### ResourceType

Indicates which type of resource an error relates to. Used by `APIError` to distinguish between inbox and email errors for 404 responses.

```go
type ResourceType string

const (
    ResourceUnknown ResourceType = ""      // Resource type not specified
    ResourceInbox   ResourceType = "inbox" // Error relates to an inbox
    ResourceEmail   ResourceType = "email" // Error relates to an email
)
```

This enables precise error matching with `errors.Is()`:

```go
_, err := inbox.GetEmail(ctx, "non-existent-id")

var apiErr *vaultsandbox.APIError
if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
    switch apiErr.ResourceType {
    case vaultsandbox.ResourceEmail:
        log.Println("Email not found")
    case vaultsandbox.ResourceInbox:
        log.Println("Inbox not found")
    }
}
```

---

### APIError

Represents an HTTP error from the VaultSandbox API.

```go
type APIError struct {
    StatusCode   int
    Message      string
    RequestID    string
    ResourceType ResourceType
}
```

#### Fields

- `StatusCode`: HTTP status code from the API
- `Message`: Error message from the server
- `RequestID`: Request ID for debugging (if returned by server)
- `ResourceType`: The type of resource the error relates to (`ResourceInbox`, `ResourceEmail`, or `ResourceUnknown`)

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

### SignatureVerificationError

Indicates potential tampering. This is a **critical security error** that may indicate a man-in-the-middle (MITM) attack or a key substitution attack. This error always matches `ErrSignatureInvalid` when using `errors.Is()`.

```go
type SignatureVerificationError struct {
    Message       string
    IsKeyMismatch bool // true if caused by server key mismatch
}
```

#### Fields

- `Message`: Description of the signature verification failure
- `IsKeyMismatch`: `true` if the error was caused by a server key mismatch (potential key substitution attack), `false` for other signature failures

#### Example

```go
emails, err := inbox.GetEmails(ctx)
if err != nil {
    // Simple check using sentinel
    if errors.Is(err, vaultsandbox.ErrSignatureInvalid) {
        log.Println("CRITICAL: Signature verification failed!")

        // For detailed information, use errors.As()
        var sigErr *vaultsandbox.SignatureVerificationError
        if errors.As(err, &sigErr) {
            log.Printf("Details: %s", sigErr.Message)

            if sigErr.IsKeyMismatch {
                log.Println("Server key mismatch - potential key substitution attack")
            } else {
                log.Println("Invalid signature - potential MITM attack or data tampering")
            }
        }

        // Alert security team
        alertSecurityTeam(err)

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

## Authentication Results Errors

The `authresults` package provides errors for email authentication validation (SPF, DKIM, DMARC, Reverse DNS).

### Sentinel Errors

```go
import "github.com/vaultsandbox/client-go/authresults"

var (
    authresults.ErrSPFFailed        // SPF check failed
    authresults.ErrDKIMFailed       // DKIM check failed
    authresults.ErrDMARCFailed      // DMARC check failed
    authresults.ErrReverseDNSFailed // Reverse DNS check failed
    authresults.ErrNoAuthResults    // No authentication results available
)
```

### ErrSPFFailed

Returned when the SPF (Sender Policy Framework) check did not pass.

```go
err := authresults.ValidateSPF(email.AuthResults)
if errors.Is(err, authresults.ErrSPFFailed) {
    log.Println("SPF check failed - sender may not be authorized")
}
```

---

### ErrDKIMFailed

Returned when no DKIM (DomainKeys Identified Mail) signature passed verification.

```go
err := authresults.ValidateDKIM(email.AuthResults)
if errors.Is(err, authresults.ErrDKIMFailed) {
    log.Println("DKIM verification failed - email may have been modified")
}
```

---

### ErrDMARCFailed

Returned when the DMARC (Domain-based Message Authentication) check did not pass.

```go
err := authresults.ValidateDMARC(email.AuthResults)
if errors.Is(err, authresults.ErrDMARCFailed) {
    log.Println("DMARC check failed - email may not be from claimed domain")
}
```

---

### ErrReverseDNSFailed

Returned when the reverse DNS check did not pass.

```go
err := authresults.ValidateReverseDNS(email.AuthResults)
if errors.Is(err, authresults.ErrReverseDNSFailed) {
    log.Println("Reverse DNS check failed")
}
```

---

### ErrNoAuthResults

Returned when no authentication results are available for validation.

```go
err := authresults.ValidateSPF(email.AuthResults)
if errors.Is(err, authresults.ErrNoAuthResults) {
    log.Println("No authentication results available")
}
```

---

### ValidationError (authresults)

Contains multiple authentication validation failures. This is distinct from the main package's `ValidationError`.

```go
type ValidationError struct {
    Errors []string
}
```

#### Example

```go
err := authresults.Validate(email.AuthResults)

var valErr *authresults.ValidationError
if errors.As(err, &valErr) {
    log.Println("Authentication validation failed:")
    for _, e := range valErr.Errors {
        log.Printf("  - %s", e)
    }
}
```

### Full Validation Example

```go
import (
    "errors"
    "log"

    vaultsandbox "github.com/vaultsandbox/client-go"
    "github.com/vaultsandbox/client-go/authresults"
)

func validateEmailAuthenticity(email *vaultsandbox.Email) error {
    // Quick check using the convenience method
    if email.AuthResults.IsPassing() {
        log.Println("All authentication checks passed")
        return nil
    }

    // Detailed validation
    validation := email.AuthResults.Validate()
    if !validation.Passed {
        log.Println("Authentication failed:")
        for _, failure := range validation.Failures {
            log.Printf("  - %s", failure)
        }
    }

    // Or use individual validators for specific checks
    if err := authresults.ValidateSPF(email.AuthResults); err != nil {
        return fmt.Errorf("SPF validation failed: %w", err)
    }

    if err := authresults.ValidateDKIM(email.AuthResults); err != nil {
        return fmt.Errorf("DKIM validation failed: %w", err)
    }

    if err := authresults.ValidateDMARC(email.AuthResults); err != nil {
        return fmt.Errorf("DMARC validation failed: %w", err)
    }

    return nil
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

    client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
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
        var apiErr *vaultsandbox.APIError
        var netErr *vaultsandbox.NetworkError

        switch {
        case errors.Is(err, context.DeadlineExceeded):
            log.Println("Timed out waiting for email")
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

        if errors.Is(err, context.DeadlineExceeded) {
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

    if errors.Is(err, context.DeadlineExceeded) {
        log.Println("No new emails, checking existing...")
        // Fall back to listing existing emails
        return inbox.GetEmails(ctx)
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

    client, err := vaultsandbox.New(testAPIKey)
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

### 1. Always Handle Timeouts

Timeouts are common in email testing. Always handle them explicitly using `context.DeadlineExceeded`:

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        // List what emails did arrive
        emails, _ := inbox.GetEmails(ctx)
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
    if errors.Is(err, vaultsandbox.ErrSignatureInvalid) ||
       errors.Is(err, vaultsandbox.ErrDecryptionFailed) {
        // Critical security/integrity error
        logger.Critical(map[string]any{
            "error":     err.Error(),
            "timestamp": time.Now().Format(time.RFC3339),
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
var netErr *vaultsandbox.NetworkError
var apiErr *vaultsandbox.APIError

switch {
case errors.Is(err, context.DeadlineExceeded):
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
client, err := vaultsandbox.New(apiKey)
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

- [Authentication Results](#authentication-results-errors) - Email authentication validation
- [CI/CD Integration](/client-go/testing/cicd) - Error handling in CI
- [Client API](/client-go/api/client) - Client configuration
