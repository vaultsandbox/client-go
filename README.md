<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./assets/logo-dark.svg">
  <source media="(prefers-color-scheme: light)" srcset="./assets/logo-light.svg">
  <img alt="VaultSandbox" src="./assets/logo-dark.svg">
</picture>

# VaultSandbox Go Client

[![Go Reference](https://pkg.go.dev/badge/github.com/vaultsandbox/client-go.svg)](https://pkg.go.dev/github.com/vaultsandbox/client-go)
[![CI](https://github.com/vaultsandbox/client-go/actions/workflows/ci.yml/badge.svg)](https://github.com/vaultsandbox/client-go/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.24-brightgreen.svg)](https://golang.org/)

**Production-like email testing. Self-hosted & secure.**

The official Go SDK for [VaultSandbox Gateway](https://github.com/vaultsandbox/gateway) — a secure, receive-only SMTP server for QA/testing environments. This SDK abstracts quantum-safe encryption complexity, making email testing workflows transparent and effortless.

Stop mocking your email stack. If your app sends real emails in production, it must send real emails in testing. VaultSandbox provides isolated inboxes that behave exactly like production without exposing a single byte of customer data.

> **Go 1.24+** required.

## Why VaultSandbox?

| Feature             | Simple Mocks     | Public SaaS  | **VaultSandbox**    |
| :------------------ | :--------------- | :----------- | :------------------ |
| **TLS/SSL**         | Ignored/Disabled | Partial      | **Real ACME certs** |
| **Data Privacy**    | Local only       | Shared cloud | **Private VPC**     |
| **Inbound Mail**    | Outbound only    | Yes          | **Real MX**         |
| **Auth (SPF/DKIM)** | None             | Limited      | **Full Validation** |
| **Crypto**          | Plaintext        | Varies       | **Zero-Knowledge**  |

## Features

- **Quantum-Safe Encryption** — Automatic ML-KEM-768 (Kyber768) key encapsulation + AES-256-GCM encryption
- **Zero Crypto Knowledge Required** — All cryptographic operations are invisible to the user
- **Real-Time Email Delivery** — SSE-based delivery with smart polling fallback
- **Built for CI/CD** — Deterministic tests without sleeps, polling, or flakiness
- **Full Email Access** — Decrypt and access email content, headers, links, and attachments
- **Email Authentication** — Built-in SPF/DKIM/DMARC validation helpers
- **Type-Safe** — Full Go type safety with comprehensive struct definitions

## Installation

```bash
go get github.com/vaultsandbox/client-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "regexp"
    "time"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    // Initialize client with your API key
    client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithBaseURL("https://smtp.vaultsandbox.com"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Create inbox (keypair generated automatically)
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer inbox.Delete(ctx)

    fmt.Printf("Send email to: %s\n", inbox.EmailAddress())

    // Wait for email with timeout
    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(30*time.Second),
        vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Test`)), // Optional filter
    )
    if err != nil {
        log.Fatal(err)
    }

    // Email is already decrypted - just use it!
    fmt.Printf("From: %s\n", email.From)
    fmt.Printf("Subject: %s\n", email.Subject)
    fmt.Printf("Text: %s\n", email.Text)
    fmt.Printf("HTML: %s\n", email.HTML)
}
```

## Usage Examples

### Testing Password Reset Emails

```go
package yourapp_test

import (
    "context"
    "regexp"
    "strings"
    "testing"
    "time"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func TestPasswordResetEmail(t *testing.T) {
    client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(url))
    if err != nil {
        t.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }
    defer inbox.Delete(ctx)

    // Trigger password reset in your app (replace with your own implementation)
    if err := yourApp.RequestPasswordReset(inbox.EmailAddress()); err != nil {
        t.Fatal(err)
    }

    // Wait for and validate the reset email
    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(10*time.Second),
        vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Reset your password`)),
    )
    if err != nil {
        t.Fatal(err)
    }

    // Extract reset link
    var resetLink string
    for _, link := range email.Links {
        if strings.Contains(link, "/reset-password") {
            resetLink = link
            break
        }
    }
    t.Logf("Reset link: %s", resetLink)

    // Validate email authentication
    validation := email.AuthResults.Validate()
    // In a real test, this may not pass if the sender isn't fully configured.
    // A robust check verifies the validation was performed and has the correct shape.
    if validation.SPFPassed {
        t.Log("SPF passed")
    }
    if validation.DKIMPassed {
        t.Log("DKIM passed")
    }
}
```

### Testing Email Authentication (SPF/DKIM/DMARC)

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(5*time.Second))
if err != nil {
    t.Fatal(err)
}

validation := email.AuthResults.Validate()

if !validation.Passed {
    t.Log("Email authentication failed:")
    for _, reason := range validation.Failures {
        t.Logf("  - %s", reason)
    }
}

// Or check individual results. Results can vary based on the sending source.
if email.AuthResults.SPF != nil {
    t.Logf("SPF result: %s", email.AuthResults.SPF.Result)
}
if len(email.AuthResults.DKIM) > 0 {
    t.Logf("DKIM signatures: %d", len(email.AuthResults.DKIM))
}
if email.AuthResults.DMARC != nil {
    t.Logf("DMARC result: %s", email.AuthResults.DMARC.Result)
}
```

### Extracting and Validating Links

```go
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Verify your email`)),
)
if err != nil {
    t.Fatal(err)
}

// All links are automatically extracted
var verifyLink string
for _, link := range email.Links {
    if strings.Contains(link, "/verify") {
        verifyLink = link
        break
    }
}

if verifyLink == "" {
    t.Fatal("verify link not found")
}
if !strings.HasPrefix(verifyLink, "https://") {
    t.Fatal("verify link should use HTTPS")
}

// Test the verification flow
resp, err := http.Get(verifyLink)
if err != nil {
    t.Fatal(err)
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
    t.Fatalf("expected 200, got %d", resp.StatusCode)
}
```

### Working with Email Attachments

Email attachments are automatically decrypted and available as `[]byte` buffers, ready to be processed or saved.

```go
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Documents Attached`)),
)
if err != nil {
    t.Fatal(err)
}

// Access attachments slice
fmt.Printf("Found %d attachments\n", len(email.Attachments))

// Iterate through attachments
for _, attachment := range email.Attachments {
    fmt.Printf("Filename: %s\n", attachment.Filename)
    fmt.Printf("Content-Type: %s\n", attachment.ContentType)
    fmt.Printf("Size: %d bytes\n", attachment.Size)

    if attachment.Content == nil {
        continue
    }

    // Decode text-based attachments
    if strings.Contains(attachment.ContentType, "text") {
        textContent := string(attachment.Content)
        fmt.Printf("Content: %s\n", textContent)
    }

    // Parse JSON attachments
    if strings.Contains(attachment.ContentType, "json") {
        var data map[string]any
        if err := json.Unmarshal(attachment.Content, &data); err == nil {
            fmt.Printf("Parsed data: %v\n", data)
        }
    }

    // Save binary files to disk
    if strings.Contains(attachment.ContentType, "pdf") ||
        strings.Contains(attachment.ContentType, "image") {
        if err := os.WriteFile("./downloads/"+attachment.Filename, attachment.Content, 0644); err != nil {
            t.Fatal(err)
        }
        fmt.Printf("Saved %s\n", attachment.Filename)
    }
}

// Find and verify specific attachment in tests
var pdfAttachment *vaultsandbox.Attachment
for i := range email.Attachments {
    if email.Attachments[i].Filename == "invoice.pdf" {
        pdfAttachment = &email.Attachments[i]
        break
    }
}

if pdfAttachment == nil {
    t.Fatal("invoice.pdf not found")
}
if pdfAttachment.ContentType != "application/pdf" {
    t.Fatalf("expected application/pdf, got %s", pdfAttachment.ContentType)
}
if pdfAttachment.Size == 0 {
    t.Fatal("attachment size should be greater than 0")
}

// Verify attachment content exists and has expected size
if pdfAttachment.Content != nil {
    if len(pdfAttachment.Content) != int(pdfAttachment.Size) {
        t.Fatalf("content length %d != size %d", len(pdfAttachment.Content), pdfAttachment.Size)
    }
}
```

### Testing with Go's testing Package

```go
package email_test

import (
    "context"
    "testing"
    "time"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

var (
    client *vaultsandbox.Client
    ctx    = context.Background()
)

func TestMain(m *testing.M) {
    var err error
    client, err = vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(url))
    if err != nil {
        panic(err)
    }
    defer client.Close()
    m.Run()
}

func TestWelcomeEmail(t *testing.T) {
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }
    defer inbox.Delete(ctx)

    if err := sendWelcomeEmail(inbox.EmailAddress()); err != nil {
        t.Fatal(err)
    }

    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(5*time.Second),
        vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Welcome`)),
    )
    if err != nil {
        t.Fatal(err)
    }

    if email.From != "noreply@example.com" {
        t.Errorf("expected from noreply@example.com, got %s", email.From)
    }
    if !strings.Contains(email.Text, "Thank you for signing up") {
        t.Error("expected welcome message in email body")
    }
}
```

### Waiting for Multiple Emails

When testing scenarios that send multiple emails, use `WaitForEmailCount()` instead of arbitrary timeouts for faster and more reliable tests:

```go
func TestMultipleNotificationEmails(t *testing.T) {
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }
    defer inbox.Delete(ctx)

    // Send multiple emails
    if err := sendNotifications(inbox.EmailAddress(), 3); err != nil {
        t.Fatal(err)
    }

    // Wait for all 3 emails to arrive
    emails, err := inbox.WaitForEmailCount(ctx, 3,
        vaultsandbox.WithWaitTimeout(30*time.Second),
    )
    if err != nil {
        t.Fatal(err)
    }

    // Verify all emails
    if len(emails) != 3 {
        t.Fatalf("expected 3 emails, got %d", len(emails))
    }
    if !strings.Contains(emails[0].Subject, "Notification") {
        t.Error("expected notification subject")
    }
}
```

### Real-time Monitoring

For scenarios where you need to process emails as they arrive, use `WatchFunc()`. Context cancellation controls the lifecycle.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer inbox.Delete(ctx)

    fmt.Printf("Watching for emails at: %s\n", inbox.EmailAddress())

    // Create cancellable context for watching
    watchCtx, cancel := context.WithCancel(ctx)

    // Handle interrupt signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigChan
        cancel() // Stop watching on interrupt
    }()

    // Watch for new emails
    inbox.WatchFunc(watchCtx, func(email *vaultsandbox.Email) {
        fmt.Printf("New email received: %q\n", email.Subject)
        // Process the email here...
    })

    fmt.Println("Stopped monitoring")
}
```

## API Reference

### Client

The main client for interacting with the VaultSandbox Gateway.

#### Constructor

```go
func New(apiKey string, opts ...Option) (*Client, error)
```

**Options:**

- `WithBaseURL(url string)` — Gateway URL (default: `https://api.vaultsandbox.com`)
- `WithHTTPClient(client *http.Client)` — Custom HTTP client
- `WithDeliveryStrategy(strategy DeliveryStrategy)` — Delivery strategy: `StrategyAuto`, `StrategySSE`, or `StrategyPolling` (default: `StrategyAuto`)
- `WithTimeout(timeout time.Duration)` — Operation timeout
- `WithRetries(count int)` — Max retry attempts for HTTP requests (default: 3)
- `WithRetryOn(statusCodes []int)` — HTTP status codes that trigger a retry (default: 408, 429, 500, 502, 503, 504)
- `WithPollingInitialInterval(interval time.Duration)` — Initial polling interval (default: 2s)
- `WithPollingMaxBackoff(maxBackoff time.Duration)` — Maximum polling backoff interval (default: 30s)
- `WithPollingBackoffMultiplier(multiplier float64)` — Backoff multiplier (default: 1.5)
- `WithPollingJitterFactor(factor float64)` — Jitter factor for polling intervals (default: 0.3)
- `WithSSEConnectionTimeout(timeout time.Duration)` — SSE connection timeout for auto mode fallback (default: 5s)

#### Methods

- `CreateInbox(ctx, opts ...InboxOption) (*Inbox, error)` — Creates a new inbox
- `ImportInbox(ctx, data *ExportedInbox) (*Inbox, error)` — Imports an inbox from exported data
- `DeleteInbox(ctx, emailAddress string) error` — Deletes a specific inbox
- `DeleteAllInboxes(ctx) (int, error)` — Deletes all inboxes for this API key
- `GetInbox(emailAddress string) (*Inbox, bool)` — Gets an inbox by email address
- `Inboxes() []*Inbox` — Gets all managed inboxes
- `ServerInfo() *ServerInfo` — Gets server information
- `CheckKey(ctx) error` — Validates API key
- `WatchInboxes(ctx, inboxes ...*Inbox) <-chan *InboxEvent` — Returns a channel that receives events from multiple inboxes; use select on ctx.Done() to detect cancellation
- `WatchInboxesFunc(ctx, fn func(*InboxEvent), inboxes ...*Inbox)` — Calls fn for each event until context is cancelled (convenience wrapper)
- `ExportInboxToFile(inbox *Inbox, filePath string) error` — Exports an inbox to a JSON file
- `ImportInboxFromFile(ctx, filePath string) (*Inbox, error)` — Imports an inbox from a JSON file
- `Close() error` — Closes the client, terminates any active SSE or polling connections, and cleans up resources

**Inbox Import/Export:** For advanced use cases like test reproducibility or sharing inboxes between environments, you can export an inbox (including its encryption keys) to a JSON file and import it later. This allows you to persist inboxes across test runs or share them with other tools.

### InboxEvent

Event struct returned when watching multiple inboxes via `Client.WatchInboxes()`.

```go
type InboxEvent struct {
    Inbox *Inbox  // The inbox that received the email
    Email *Email  // The received email
}
```

#### Example

```go
inbox1, _ := client.CreateInbox(ctx)
inbox2, _ := client.CreateInbox(ctx)

fmt.Printf("Watching inboxes: %s, %s\n", inbox1.EmailAddress(), inbox2.EmailAddress())

watchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
defer cancel()

client.WatchInboxesFunc(watchCtx, func(event *vaultsandbox.InboxEvent) {
    fmt.Printf("New email in %s: %s\n", event.Inbox.EmailAddress(), event.Email.Subject)
    // Further processing...
}, inbox1, inbox2)
```

### Inbox

Represents a single email inbox.

#### Properties

- `EmailAddress() string` — The inbox email address
- `InboxHash() string` — Unique inbox identifier (SHA-256 hash of public key)
- `ExpiresAt() time.Time` — When the inbox expires
- `IsExpired() bool` — Whether the inbox has expired

#### Methods

- `GetEmails(ctx) ([]*Email, error)` — Lists all emails (decrypted)
- `GetEmail(ctx, emailID string) (*Email, error)` — Gets a specific email
- `WaitForEmail(ctx, opts ...WaitOption) (*Email, error)` — Waits for an email matching criteria
- `WaitForEmailCount(ctx, count int, opts ...WaitOption) ([]*Email, error)` — Waits until the inbox has at least the specified number of emails
- `Watch(ctx) <-chan *Email` — Returns a channel that receives emails as they arrive; use select on ctx.Done() to detect cancellation
- `WatchFunc(ctx, fn func(*Email))` — Calls fn for each email until context is cancelled (convenience wrapper)
- `GetSyncStatus(ctx) (*SyncStatus, error)` — Gets inbox sync status
- `GetRawEmail(ctx, emailID string) (string, error)` — Gets the raw, decrypted source of a specific email
- `MarkEmailAsRead(ctx, emailID string) error` — Marks email as read
- `DeleteEmail(ctx, emailID string) error` — Deletes an email
- `Delete(ctx) error` — Deletes this inbox
- `Export() *ExportedInbox` — Exports inbox data and key material for backup/sharing (treat output as sensitive)

### Email

Represents a decrypted email.

#### Fields

```go
type Email struct {
    ID          string                    // Email ID
    From        string                    // Sender address
    To          []string                  // Recipient addresses
    Subject     string                    // Email subject
    Text        string                    // Plain text content
    HTML        string                    // HTML content
    ReceivedAt  time.Time                 // When the email was received
    IsRead      bool                      // Read status
    Links       []string                  // Extracted URLs from email
    Headers     map[string]string         // Email headers
    Attachments []Attachment              // Email attachments
    AuthResults *authresults.AuthResults  // Email authentication results
}
```

`Email` is a pure data struct with no methods. Use `Inbox` methods to perform operations on emails:

- `inbox.GetRawEmail(ctx, emailID)` — Gets raw email source
- `inbox.MarkEmailAsRead(ctx, emailID)` — Marks email as read
- `inbox.DeleteEmail(ctx, emailID)` — Deletes an email

### Attachment

Represents an email attachment.

```go
type Attachment struct {
    Filename           string  // Attachment filename
    ContentType        string  // MIME content type
    Size               int     // Size in bytes
    Content            []byte  // Decrypted content
    ContentID          string  // Content-ID for inline attachments
    ContentDisposition string  // Content disposition
    Checksum           string  // Content checksum
}
```

### AuthResults

Returned by `email.AuthResults`, this struct contains email authentication results (SPF, DKIM, DMARC) and a validation helper.

#### Fields

- `SPF *SPFResult` — SPF result
- `DKIM []DKIMResult` — All DKIM results
- `DMARC *DMARCResult` — DMARC result
- `ReverseDNS *ReverseDNSResult` — Reverse DNS result

#### Methods

- `Validate() AuthValidation` — Validates all authentication results and returns a summary with `Passed`, per-check booleans (`SPFPassed`, `DKIMPassed`, `DMARCPassed`, `ReverseDNSPassed`), and a list of `Failures`
- `IsPassing() bool` — Convenience method (equivalent to `Validate().Passed`)

### InboxOption

Options for creating an inbox with `client.CreateInbox()`.

- `WithTTL(ttl time.Duration)` — Time-to-live for the inbox (default: server-defined, min: 1 minute, max: 7 days)
- `WithEmailAddress(email string)` — A specific email address to request. If unavailable, the server will generate one

### WaitOption

Options for waiting for emails with `inbox.WaitForEmail()`.

- `WithWaitTimeout(timeout time.Duration)` — Maximum time to wait (default: 60 seconds)
- `WithSubject(subject string)` — Filter emails by exact subject match
- `WithSubjectRegex(pattern *regexp.Regexp)` — Filter emails by subject regex
- `WithFrom(from string)` — Filter emails by exact sender address
- `WithFromRegex(pattern *regexp.Regexp)` — Filter emails by sender regex
- `WithPredicate(fn func(*Email) bool)` — Custom filter function

**Example:**

```go
// Wait for email with specific subject
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Password Reset`)),
)

// Wait with custom predicate
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(15*time.Second),
    vaultsandbox.WithPredicate(func(email *vaultsandbox.Email) bool {
        for _, to := range email.To {
            if to == "user@example.com" {
                return true
            }
        }
        return false
    }),
)
```

## Error Handling

The SDK is designed to be resilient and provide clear feedback when issues occur. It includes automatic retries for transient network and server errors, and returns specific, checkable errors for different failure scenarios.

All custom errors can be checked using Go's `errors.Is()` function.

### Automatic Retries

By default, the client automatically retries failed HTTP requests that result in one of the following status codes: `408`, `429`, `500`, `502`, `503`, `504`. This helps mitigate transient network or server-side issues.

The retry behavior can be configured via client options:

- `WithRetries(count int)` — The maximum number of retry attempts (default: 3)
- `WithRetryOn(statusCodes []int)` — HTTP status codes that should trigger a retry

### Error Types

The following sentinel errors may be returned:

- **`ErrMissingAPIKey`** — No API key was provided
- **`ErrClientClosed`** — Operations attempted on a closed client
- **`ErrUnauthorized`** — Invalid or expired API key (HTTP 401)
- **`ErrInboxNotFound`** — Inbox does not exist (HTTP 404)
- **`ErrEmailNotFound`** — Email does not exist (HTTP 404)
- **`ErrInboxAlreadyExists`** — Attempting to import an inbox that already exists
- **`ErrInvalidImportData`** — Imported inbox data fails validation
- **`ErrDecryptionFailed`** — Client fails to decrypt an email
- **`ErrSignatureInvalid`** — Cryptographic signature verification failed (potential MITM)
- **`ErrRateLimited`** — API rate limit exceeded (HTTP 429)

**Error Structs:**

- **`APIError`** — HTTP API errors with `StatusCode`, `Message`, `RequestID`, and `ResourceType` fields
- **`NetworkError`** — Network-level failures with `Err`, `URL`, and `Attempt` fields
- **`SignatureVerificationError`** — Signature/key mismatch failures with `Message` and `IsKeyMismatch` fields

### Example

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"
    "os"
    "time"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer inbox.Delete(ctx)

    fmt.Printf("Send email to: %s\n", inbox.EmailAddress())

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

    fmt.Printf("Email received: %s\n", email.Subject)
}
```

## Channel vs Callback API

The SDK offers two styles for watching emails:

**Callback-based (recommended for most cases):**

```go
// Simple and concise - handles context cancellation internally
inbox.WatchFunc(ctx, func(email *vaultsandbox.Email) {
    fmt.Printf("New email: %s\n", email.Subject)
})

// Multi-inbox variant
client.WatchInboxesFunc(ctx, func(event *vaultsandbox.InboxEvent) {
    fmt.Printf("Email in %s: %s\n", event.Inbox.EmailAddress(), event.Email.Subject)
}, inbox1, inbox2)
```

**Channel-based (for advanced use cases):**

```go
// Use when you need to combine with other channels or custom select logic
emails := inbox.Watch(ctx)
for {
    select {
    case <-ctx.Done():
        return
    case email := <-emails:
        fmt.Printf("New email: %s\n", email.Subject)
    case <-otherChan:
        // Handle other events concurrently
    }
}
```

## Requirements

- Go 1.24 or later
- VaultSandbox Gateway server
- Valid API key

## Testing

```bash
# Run all tests with coverage and integration (requires .env)
./scripts/test.sh

# Skip integration tests (unit tests only)
./scripts/test.sh --skip-integration

# Skip coverage collection
./scripts/test.sh --skip-coverage

# Verbose output
./scripts/test.sh -v

# Unit tests only, no coverage
./scripts/test.sh --skip-integration --skip-coverage
```

## Building

```bash
# Build the library
go build ./...

# Run linter
golangci-lint run
```

## Architecture

The SDK is built on several layers:

1. **Crypto Layer** — Handles ML-KEM-768 keypair generation, AES-256-GCM encryption/decryption, and ML-DSA-65 signature verification
2. **HTTP Layer** — REST API client with automatic retry and error handling
3. **Domain Layer** — Email, Inbox, and Client types with intuitive APIs
4. **Strategy Layer** — SSE and polling strategies for email delivery

All cryptographic operations are performed transparently — developers never need to handle keys, encryption, or signatures directly.

## Security

- **Cryptography:** ML-KEM-768 (Kyber768) for key encapsulation + AES-256-GCM for payload encryption, with HKDF-SHA-512 key derivation
- **Signatures:** ML-DSA-65 (Dilithium3) signatures are verified **before** any decryption using the gateway-provided transcript context
- **Threat model:** Protects confidentiality/integrity of gateway responses and detects tampering/MITM. Skipping signature verification defeats these guarantees
- **Key handling:** Inbox keypairs stay in memory only; exported inbox data contains secrets and must be treated as sensitive
- **Validation:** Signature verification failures return `ErrSignatureInvalid`; decryption issues return `ErrDecryptionFailed`. Always surface these in logs/alerts for investigation

## Related

- [VaultSandbox Gateway](https://github.com/vaultsandbox/gateway) — The self-hosted SMTP server this SDK connects to
- [VaultSandbox Documentation](https://vaultsandbox.dev) — Full documentation and guides

## Support

- [Documentation](https://vaultsandbox.dev/client-go/)
- [Issue Tracker](https://github.com/vaultsandbox/client-go/issues)
- [Discussions](https://github.com/vaultsandbox/gateway/discussions)
- [Website](https://www.vaultsandbox.com)

## Contributing

Contributions are welcome! Please read our [contributing guidelines](CONTRIBUTING.md) before submitting PRs.

## License

MIT — see [LICENSE](LICENSE) for details.
