---
title: Inboxes
description: Understanding VaultSandbox inboxes and how to work with them
---

Inboxes are the core concept in VaultSandbox. Each inbox is an isolated, encrypted email destination with its own unique address and encryption keys.

## What is an Inbox?

An inbox is a temporary, encrypted email destination that:

- Has a **unique email address** (e.g., `a1b2c3d4@mail.example.com`)
- Uses **client-side encryption** (ML-KEM-768 keypair)
- **Expires automatically** after a configurable time-to-live (TTL)
- Is **isolated** from other inboxes
- Stores emails **in memory** on the gateway

## Creating Inboxes

### Basic Creation

```go
package main

import (
	"context"
	"fmt"
	"log"

	vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
	client, err := vaultsandbox.New(apiKey)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(inbox.EmailAddress()) // "a1b2c3d4@mail.example.com"
	fmt.Println(inbox.InboxHash())    // "Rr02MLnP7F0pRVC6QdcpSIeyklqu3PDkYglvsfN7Oss"
	fmt.Println(inbox.ExpiresAt())    // time.Time
}
```

### With Options

```go
inbox, err := client.CreateInbox(ctx,
	vaultsandbox.WithTTL(time.Hour),                           // 1 hour (default: 1 hour)
	vaultsandbox.WithEmailAddress("test@mail.example.com"),    // Request specific address
)
```

**Note**: Requesting a specific email address may fail if it's already in use. The server will return an error.

## Client Options

When creating a client with `vaultsandbox.New()`, you can configure various options:

```go
client, err := vaultsandbox.New(apiKey,
	vaultsandbox.WithBaseURL("https://custom-api.example.com"),
	vaultsandbox.WithTimeout(30*time.Second),
	vaultsandbox.WithRetries(3),
)
```

### Available Client Options

| Option | Description |
|--------|-------------|
| `WithBaseURL(url string)` | Set a custom API base URL (default: `https://api.vaultsandbox.com`) |
| `WithHTTPClient(client *http.Client)` | Use a custom HTTP client for requests |
| `WithDeliveryStrategy(strategy DeliveryStrategy)` | Set how new emails are delivered (see below) |
| `WithTimeout(timeout time.Duration)` | Set default request timeout (default: 60s) |
| `WithRetries(count int)` | Set number of retry attempts for failed requests |
| `WithRetryOn(statusCodes []int)` | Set HTTP status codes that trigger retries (default: 408, 429, 500, 502, 503, 504) |

### Delivery Strategies

The `DeliveryStrategy` type controls how the client receives new email notifications:

```go
const (
	// StrategyAuto tries SSE first, falls back to polling (default)
	StrategyAuto DeliveryStrategy = "auto"

	// StrategySSE uses Server-Sent Events for real-time push notifications
	StrategySSE DeliveryStrategy = "sse"

	// StrategyPolling uses periodic API calls with exponential backoff
	StrategyPolling DeliveryStrategy = "polling"
)
```

```go
// Force polling for environments where SSE is blocked
client, err := vaultsandbox.New(apiKey,
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
)

// Force SSE for lowest latency
client, err := vaultsandbox.New(apiKey,
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),
)
```

## Inbox Properties

### EmailAddress()

**Returns**: `string`

The full email address for this inbox.

```go
fmt.Println(inbox.EmailAddress())
// "a1b2c3d4@mail.example.com"
```

Send emails to this address to have them appear in the inbox.

### InboxHash()

**Returns**: `string`

A unique cryptographic hash identifier for the inbox. This is used internally for encryption and identification purposes.

```go
fmt.Println(inbox.InboxHash())
// "Rr02MLnP7F0pRVC6QdcpSIeyklqu3PDkYglvsfN7Oss"
```

**Note**: This is not the same as the local part of the email address. The email address local part (e.g., `a1b2c3d4` in `a1b2c3d4@mail.example.com`) is different from the `InboxHash()`.

### ExpiresAt()

**Returns**: `time.Time`

When the inbox will automatically expire and be deleted.

```go
fmt.Println(inbox.ExpiresAt())
// 2024-01-16 12:00:00 +0000 UTC

// Check if inbox is expiring soon
hoursUntilExpiry := time.Until(inbox.ExpiresAt()).Hours()
fmt.Printf("Expires in %.1f hours\n", hoursUntilExpiry)
```

### IsExpired()

**Returns**: `bool`

Checks if the inbox has expired.

```go
if inbox.IsExpired() {
	fmt.Println("Inbox has expired")
}
```

### GetSyncStatus()

**Returns**: `(*SyncStatus, error)`

Retrieves the synchronization status of the inbox, including the email count and a hash for efficient change detection.

```go
status, err := inbox.GetSyncStatus(ctx)
if err != nil {
	log.Fatal(err)
}

fmt.Printf("Email count: %d\n", status.EmailCount)
fmt.Printf("Emails hash: %s\n", status.EmailsHash)
```

The `SyncStatus` struct contains:

```go
type SyncStatus struct {
	EmailCount int    // Number of emails in the inbox
	EmailsHash string // Hash of the email list for change detection
}
```

## Inbox Lifecycle

```
┌─────────────────────────────────────────────────────────┐
│                  Inbox Lifecycle                        │
└─────────────────────────────────────────────────────────┘

1. Creation
   client.CreateInbox(ctx) → *Inbox
   ↓
   - Keypair generated client-side
   - Public key sent to server
   - Unique email address assigned
   - TTL timer starts

2. Active
   ↓
   - Receive emails
   - List/read emails
   - Wait for emails
   - Monitor for new emails

3. Expiration (TTL reached) or Manual Deletion
   ↓
   inbox.Delete(ctx) or TTL expires
   - All emails deleted
   - Inbox address freed
   - Keypair destroyed
```

## Working with Inboxes

### Listing Emails

```go
emails, err := inbox.GetEmails(ctx)
if err != nil {
	log.Fatal(err)
}

fmt.Printf("%d emails in inbox\n", len(emails))
for _, email := range emails {
	fmt.Printf("%s: %s\n", email.From, email.Subject)
}
```

### Getting a Specific Email

```go
email, err := inbox.GetEmail(ctx, "email-id-123")
if err != nil {
	log.Fatal(err)
}

fmt.Println(email.Subject)
fmt.Println(email.Text)
```

### Email Struct Fields

The `Email` struct contains the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `string` | Unique email identifier |
| `From` | `string` | Sender email address |
| `To` | `[]string` | Recipient email addresses |
| `Subject` | `string` | Email subject line |
| `Text` | `string` | Plain text body |
| `HTML` | `string` | HTML body |
| `ReceivedAt` | `time.Time` | When the email was received |
| `Headers` | `map[string]string` | Email headers |
| `Attachments` | `[]Attachment` | File attachments |
| `Links` | `[]string` | Links extracted from the email body |
| `AuthResults` | `*authresults.AuthResults` | Email authentication results (SPF, DKIM, DMARC) |
| `IsRead` | `bool` | Whether the email has been marked as read |

### Attachment Struct

The `Attachment` struct contains the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `Filename` | `string` | Original filename of the attachment |
| `ContentType` | `string` | MIME type (e.g., `application/pdf`, `image/png`) |
| `Size` | `int` | Size in bytes |
| `ContentID` | `string` | Content-ID for inline attachments |
| `ContentDisposition` | `string` | Disposition type (`attachment` or `inline`) |
| `Content` | `[]byte` | Raw attachment content |
| `Checksum` | `string` | Checksum for integrity verification |

```go
// Working with attachments
for _, att := range email.Attachments {
	fmt.Printf("Filename: %s\n", att.Filename)
	fmt.Printf("Type: %s\n", att.ContentType)
	fmt.Printf("Size: %d bytes\n", att.Size)

	// Save attachment to file
	os.WriteFile(att.Filename, att.Content, 0644)
}
```

### AuthResults Struct

The `AuthResults` struct contains email authentication check results:

```go
type AuthResults struct {
	SPF        *SPFResult        // SPF check result
	DKIM       []DKIMResult      // DKIM signature results (may have multiple)
	DMARC      *DMARCResult      // DMARC policy result
	ReverseDNS *ReverseDNSResult // Reverse DNS check result
}

type SPFResult struct {
	Status string // pass, fail, softfail, neutral, none, temperror, permerror
	Domain string
	IP     string
	Info   string
}

type DKIMResult struct {
	Status   string // pass, fail, none
	Domain   string
	Selector string
	Info     string
}

type DMARCResult struct {
	Status  string // pass, fail, none
	Policy  string // none, quarantine, reject
	Aligned bool
	Domain  string
	Info    string
}

type ReverseDNSResult struct {
	Status   string // pass, fail, none
	IP       string
	Hostname string
	Info     string
}
```

**Validation Methods:**

```go
// Quick check if all primary auth checks passed
if email.AuthResults.IsPassing() {
	fmt.Println("Email authentication passed")
}

// Detailed validation with failure messages
validation := email.AuthResults.Validate()
fmt.Printf("Overall: %v\n", validation.Passed)
fmt.Printf("SPF: %v\n", validation.SPFPassed)
fmt.Printf("DKIM: %v\n", validation.DKIMPassed)
fmt.Printf("DMARC: %v\n", validation.DMARCPassed)
fmt.Printf("Reverse DNS: %v\n", validation.ReverseDNSPassed)

if len(validation.Failures) > 0 {
	fmt.Println("Failures:")
	for _, f := range validation.Failures {
		fmt.Printf("  - %s\n", f)
	}
}
```

```go
// Accessing email fields
fmt.Printf("From: %s\n", email.From)
fmt.Printf("Subject: %s\n", email.Subject)
fmt.Printf("Received: %s\n", email.ReceivedAt)
fmt.Printf("Is Read: %v\n", email.IsRead)
fmt.Printf("Attachments: %d\n", len(email.Attachments))

// Check authentication results
if email.AuthResults != nil {
	if email.AuthResults.SPF != nil {
		fmt.Printf("SPF: %s\n", email.AuthResults.SPF.Status)
	}
	for _, dkim := range email.AuthResults.DKIM {
		fmt.Printf("DKIM: %s (domain: %s)\n", dkim.Status, dkim.Domain)
	}
	if email.AuthResults.DMARC != nil {
		fmt.Printf("DMARC: %s\n", email.AuthResults.DMARC.Status)
	}

	// Use convenience method to check if all auth checks passed
	if email.AuthResults.IsPassing() {
		fmt.Println("All authentication checks passed")
	}
}
```

**Standalone Validation Functions:**

The `authresults` package also provides standalone validation functions for granular control:

```go
import "github.com/vaultsandbox/client-go/authresults"

// Validate all authentication results at once
err := authresults.Validate(email.AuthResults)
if err != nil {
	var valErr *authresults.ValidationError
	if errors.As(err, &valErr) {
		fmt.Println("Validation failures:", valErr.Errors)
	}
}

// Validate individual checks
if err := authresults.ValidateSPF(email.AuthResults); err != nil {
	fmt.Println("SPF failed:", err)
}

if err := authresults.ValidateDKIM(email.AuthResults); err != nil {
	fmt.Println("DKIM failed:", err)
}

if err := authresults.ValidateDMARC(email.AuthResults); err != nil {
	fmt.Println("DMARC failed:", err)
}

if err := authresults.ValidateReverseDNS(email.AuthResults); err != nil {
	fmt.Println("Reverse DNS failed:", err)
}
```

**Available Validation Functions:**

| Function | Description |
|----------|-------------|
| `Validate(results)` | Validates all authentication results, returns `ValidationError` with all failures |
| `ValidateSPF(results)` | Validates only SPF, returns `ErrSPFFailed` on failure |
| `ValidateDKIM(results)` | Validates DKIM (passes if at least one signature passes), returns `ErrDKIMFailed` on failure |
| `ValidateDMARC(results)` | Validates only DMARC, returns `ErrDMARCFailed` on failure |
| `ValidateReverseDNS(results)` | Validates only reverse DNS, returns `ErrReverseDNSFailed` on failure |

**Sentinel Errors in authresults package:**

| Error | Description |
|-------|-------------|
| `ErrSPFFailed` | SPF check failed |
| `ErrDKIMFailed` | DKIM check failed |
| `ErrDMARCFailed` | DMARC check failed |
| `ErrReverseDNSFailed` | Reverse DNS check failed |
| `ErrNoAuthResults` | No authentication results available |

### Waiting for Emails

```go
// Wait for any email
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithWaitTimeout(30*time.Second),
)

// Wait for specific email
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithWaitTimeout(30*time.Second),
	vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Password Reset`)),
	vaultsandbox.WithFrom("noreply@example.com"),
)
```

**Available Wait Options:**

| Option | Description |
|--------|-------------|
| `WithWaitTimeout(duration)` | Maximum time to wait for an email |
| `WithSubject(string)` | Filter by exact subject match |
| `WithSubjectRegex(regexp)` | Filter by subject regex pattern |
| `WithFrom(string)` | Filter by exact sender match |
| `WithFromRegex(regexp)` | Filter by sender regex pattern |
| `WithPredicate(func(*Email) bool)` | Filter by custom predicate function |
| `WithPollInterval(duration)` | Set the polling interval (default: 2s) |

```go
// Using exact subject match
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithSubject("Welcome to Our Service"),
)

// Using sender regex
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithFromRegex(regexp.MustCompile(`.*@example\.com`)),
)

// Using custom predicate
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithPredicate(func(e *vaultsandbox.Email) bool {
		return len(e.Attachments) > 0
	}),
)

// Combining multiple filters
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithWaitTimeout(60*time.Second),
	vaultsandbox.WithFrom("notifications@example.com"),
	vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Order #\d+`)),
	vaultsandbox.WithPollInterval(1*time.Second),
)
```

### Waiting for Multiple Emails

```go
// Wait until the inbox has at least 3 emails
emails, err := inbox.WaitForEmailCount(ctx, 3,
	vaultsandbox.WithWaitTimeout(60*time.Second),
)
if err != nil {
	log.Fatal(err)
}

fmt.Printf("Received %d emails\n", len(emails))
```

### Getting Raw Email Content

```go
// Get raw email (original MIME content)
rawContent, err := inbox.GetRawEmail(ctx, email.ID)
if err != nil {
	log.Fatal(err)
}
fmt.Println(rawContent)
```

### Marking Emails as Read

```go
err := inbox.MarkEmailAsRead(ctx, email.ID)
if err != nil {
	log.Fatal(err)
}
```

### Deleting Emails

```go
// Get the email first, then delete
email, err := inbox.GetEmail(ctx, "email-id-123")
if err != nil {
	log.Fatal(err)
}
err = inbox.DeleteEmail(ctx, email.ID)
```

### Deleting Inbox

```go
// Delete inbox and all its emails
err := inbox.Delete(ctx)
```

## Inbox Isolation

Each inbox is completely isolated:

```go
inbox1, _ := client.CreateInbox(ctx)
inbox2, _ := client.CreateInbox(ctx)

// inbox1 cannot access inbox2's emails
// inbox2 cannot access inbox1's emails

// Each has its own:
// - Email address
// - Encryption keys
// - Email storage
// - Expiration time
```

## Time-to-Live (TTL)

Inboxes automatically expire after their TTL:

### Default TTL

```go
// Uses default TTL (1 hour)
inbox, err := client.CreateInbox(ctx)
```

### Custom TTL

```go
// Expire after 1 hour
inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(time.Hour))

// Expire after 10 minutes (useful for quick tests)
inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))

// Expire after 7 days (maximum allowed)
inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(7*24*time.Hour))
```

### TTL Constraints

```go
const (
	MinTTL = 60 * time.Second      // Minimum TTL: 1 minute
	MaxTTL = 604800 * time.Second  // Maximum TTL: 7 days
)
```

### Checking Expiration

```go
minutesLeft := time.Until(inbox.ExpiresAt()).Minutes()

if minutesLeft < 5 {
	fmt.Println("Inbox expiring soon!")
}
```

## Import and Export

Inboxes can be exported and imported for:

- Test reproducibility
- Sharing between environments
- Backup and restore

### Export

```go
exportData := inbox.Export()

// Save to file
jsonData, _ := json.MarshalIndent(exportData, "", "  ")
os.WriteFile("inbox.json", jsonData, 0600)

// Or use the convenience method
err := client.ExportInboxToFile(inbox, "inbox.json")
```

### Import

```go
// From ExportedInbox struct
jsonData, _ := os.ReadFile("inbox.json")
var exportData vaultsandbox.ExportedInbox
json.Unmarshal(jsonData, &exportData)
inbox, err := client.ImportInbox(ctx, &exportData)

// Or use the convenience method
inbox, err := client.ImportInboxFromFile(ctx, "inbox.json")

// Inbox restored with all encryption keys
```

**Security Warning**: Exported data contains private keys. Treat as sensitive.

## Monitoring for New Emails

### Single Inbox Monitoring

```go
subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
	fmt.Printf("New email: %s\n", email.Subject)
})
defer subscription.Unsubscribe()
```

### Multiple Inbox Monitoring

```go
monitor, err := client.MonitorInboxes([]*vaultsandbox.Inbox{inbox1, inbox2})
if err != nil {
	log.Fatal(err)
}
defer monitor.Unsubscribe()

monitor.OnEmail(func(inbox *vaultsandbox.Inbox, email *vaultsandbox.Email) {
	fmt.Printf("New email in %s: %s\n", inbox.EmailAddress(), email.Subject)
})
```

## Best Practices

### CI/CD Pipelines

**Short TTL for fast cleanup**:

```go
inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(time.Hour))
```

**Always clean up**:

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
	log.Fatal(err)
}
defer inbox.Delete(context.Background())

// Run tests
```

### Testing with Go

**Test setup and teardown**:

```go
func TestPasswordReset(t *testing.T) {
	client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(context.Background())

	// Trigger password reset
	triggerPasswordReset(inbox.EmailAddress())

	// Wait for email
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Assertions
	if email.Subject != "Password Reset" {
		t.Errorf("expected subject 'Password Reset', got %q", email.Subject)
	}
}
```

### Manual Testing

**Longer TTL for convenience**:

```go
inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(24*time.Hour))
```

**Export for reuse**:

```go
// Export after creating
err := client.ExportInboxToFile(inbox, "test-inbox.json")

// Reuse in later sessions
inbox, err := client.ImportInboxFromFile(ctx, "test-inbox.json")
```

### Production Monitoring

**Monitor expiration**:

```go
go func() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		minutesLeft := time.Until(inbox.ExpiresAt()).Minutes()
		if minutesLeft < 10 {
			log.Printf("Inbox %s expiring in %.0f minutes",
				inbox.EmailAddress(), minutesLeft)
		}
	}
}()
```

## Common Patterns

### Dedicated Test Inbox

```go
var testInbox *vaultsandbox.Inbox
var testClient *vaultsandbox.Client

func TestMain(m *testing.M) {
	var err error
	testClient, err = vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	testInbox, err = testClient.CreateInbox(context.Background(),
		vaultsandbox.WithTTL(2*time.Hour),
	)
	if err != nil {
		log.Fatal(err)
	}

	code := m.Run()

	testInbox.Delete(context.Background())
	testClient.Close()

	os.Exit(code)
}

func TestPasswordReset(t *testing.T) {
	ctx := context.Background()
	triggerPasswordReset(testInbox.EmailAddress())

	email, err := testInbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	// ...
}
```

### Multiple Inboxes

```go
user1Inbox, _ := client.CreateInbox(ctx)
user2Inbox, _ := client.CreateInbox(ctx)
adminInbox, _ := client.CreateInbox(ctx)

// Each inbox receives emails independently
sendWelcomeEmail(user1Inbox.EmailAddress())
sendWelcomeEmail(user2Inbox.EmailAddress())
sendAdminReport(adminInbox.EmailAddress())
```

### Inbox Pool

```go
type InboxPool struct {
	client *vaultsandbox.Client
	pool   chan *vaultsandbox.Inbox
	size   int
}

func NewInboxPool(client *vaultsandbox.Client, size int) *InboxPool {
	return &InboxPool{
		client: client,
		pool:   make(chan *vaultsandbox.Inbox, size),
		size:   size,
	}
}

func (p *InboxPool) Initialize(ctx context.Context) error {
	for i := 0; i < p.size; i++ {
		inbox, err := p.client.CreateInbox(ctx)
		if err != nil {
			return err
		}
		p.pool <- inbox
	}
	return nil
}

func (p *InboxPool) Get() *vaultsandbox.Inbox {
	return <-p.pool
}

func (p *InboxPool) Return(inbox *vaultsandbox.Inbox) {
	p.pool <- inbox
}

func (p *InboxPool) Cleanup(ctx context.Context) {
	close(p.pool)
	for inbox := range p.pool {
		inbox.Delete(ctx)
	}
}
```

## Error Handling

### Sentinel Errors

Use `errors.Is()` to check for specific error conditions:

```go
import "errors"

emails, err := inbox.GetEmails(ctx)
if err != nil {
	switch {
	case errors.Is(err, vaultsandbox.ErrInboxNotFound):
		log.Println("Inbox not found or expired")
	case errors.Is(err, vaultsandbox.ErrUnauthorized):
		log.Println("Invalid API key")
	case errors.Is(err, vaultsandbox.ErrRateLimited):
		log.Println("Rate limit exceeded, retry later")
	default:
		log.Printf("Unexpected error: %v", err)
	}
}
```

### Available Sentinel Errors

| Error | Description |
|-------|-------------|
| `ErrMissingAPIKey` | No API key was provided to `New()` |
| `ErrClientClosed` | Operation attempted on a closed client |
| `ErrUnauthorized` | API key is invalid or expired (HTTP 401) |
| `ErrInboxNotFound` | Inbox does not exist or has expired (HTTP 404) |
| `ErrEmailNotFound` | Email does not exist (HTTP 404) |
| `ErrInboxAlreadyExists` | Inbox with requested address already exists (HTTP 409) |
| `ErrInvalidImportData` | Imported inbox data is malformed or invalid |
| `ErrDecryptionFailed` | Email decryption failed |
| `ErrSignatureInvalid` | Signature verification failed (potential tampering) |
| `ErrServerKeyMismatch` | Server public key mismatch (potential key substitution attack) |
| `ErrSSEConnection` | Server-Sent Events connection failed |
| `ErrInvalidSecretKeySize` | Secret key has invalid size |
| `ErrInboxExpired` | Inbox TTL has been exceeded |
| `ErrRateLimited` | API rate limit exceeded (HTTP 429) |

### Error Types

The SDK provides structured error types for detailed error handling:

```go
// APIError - HTTP errors from the VaultSandbox API
var apiErr *vaultsandbox.APIError
if errors.As(err, &apiErr) {
	fmt.Printf("Status: %d\n", apiErr.StatusCode)
	fmt.Printf("Message: %s\n", apiErr.Message)
	fmt.Printf("Request ID: %s\n", apiErr.RequestID)
}

// NetworkError - Network-level failures
var netErr *vaultsandbox.NetworkError
if errors.As(err, &netErr) {
	fmt.Printf("URL: %s\n", netErr.URL)
	fmt.Printf("Attempt: %d\n", netErr.Attempt)
	fmt.Printf("Underlying: %v\n", netErr.Err)
}

// TimeoutError - Operation exceeded deadline
var timeoutErr *vaultsandbox.TimeoutError
if errors.As(err, &timeoutErr) {
	fmt.Printf("Operation: %s\n", timeoutErr.Operation)
	fmt.Printf("Timeout: %v\n", timeoutErr.Timeout)
}

// DecryptionError - Email decryption failure
var decryptErr *vaultsandbox.DecryptionError
if errors.As(err, &decryptErr) {
	fmt.Printf("Stage: %s\n", decryptErr.Stage) // "kem", "hkdf", or "aes"
	fmt.Printf("Message: %s\n", decryptErr.Message)
}

// SignatureVerificationError - Signature verification failure
var sigErr *vaultsandbox.SignatureVerificationError
if errors.As(err, &sigErr) {
	fmt.Printf("Message: %s\n", sigErr.Message)
	if sigErr.IsKeyMismatch {
		fmt.Println("Server key mismatch - potential key substitution attack")
	}
}

// SSEError - Server-Sent Events connection failure
var sseErr *vaultsandbox.SSEError
if errors.As(err, &sseErr) {
	fmt.Printf("Attempts: %d\n", sseErr.Attempts)
	fmt.Printf("Underlying: %v\n", sseErr.Err)
}

// ValidationError - Multiple validation failures
var valErr *vaultsandbox.ValidationError
if errors.As(err, &valErr) {
	fmt.Printf("Errors: %v\n", valErr.Errors)
}

// StrategyError - Delivery strategy failure
var stratErr *vaultsandbox.StrategyError
if errors.As(err, &stratErr) {
	fmt.Printf("Message: %s\n", stratErr.Message)
	fmt.Printf("Underlying: %v\n", stratErr.Err)
}
```

### Error Type Definitions

```go
type APIError struct {
	StatusCode int
	Message    string
	RequestID  string
}

type NetworkError struct {
	Err     error
	URL     string
	Attempt int
}

type TimeoutError struct {
	Operation string
	Timeout   time.Duration
}

type DecryptionError struct {
	Stage   string // "kem", "hkdf", "aes"
	Message string
	Err     error
}

type SignatureVerificationError struct {
	Message       string
	IsKeyMismatch bool // true if caused by server key mismatch
}

type SSEError struct {
	Err      error
	Attempts int
}

type ValidationError struct {
	Errors []string
}

type StrategyError struct {
	Message string
	Err     error
}
```

### VaultSandboxError Interface

All SDK errors implement the `VaultSandboxError` interface:

```go
type VaultSandboxError interface {
	error
	VaultSandboxError() // marker method
}

// Check if error is from this SDK
var sdkErr vaultsandbox.VaultSandboxError
if errors.As(err, &sdkErr) {
	fmt.Println("Error from VaultSandbox SDK")
}
```

## Troubleshooting

### Inbox Not Receiving Emails

**Check**:

1. Email is sent to correct address
2. Inbox hasn't expired
3. DNS/MX records configured correctly
4. SMTP connection successful

```go
// Verify inbox still exists
_, err := inbox.GetEmails(ctx) // Will error if inbox expired
if errors.Is(err, vaultsandbox.ErrInboxNotFound) {
	log.Println("Inbox has expired or was deleted")
}
```

### Inbox Already Exists Error

When requesting a specific email address:

```go
inbox, err := client.CreateInbox(ctx,
	vaultsandbox.WithEmailAddress("test@mail.example.com"),
)
if errors.Is(err, vaultsandbox.ErrInboxAlreadyExists) {
	// Address already in use, generate random instead
	inbox, err = client.CreateInbox(ctx)
}
```

### Inbox Expired

```go
emails, err := inbox.GetEmails(ctx)
if errors.Is(err, vaultsandbox.ErrInboxNotFound) {
	log.Println("Inbox has expired")
	// Create new inbox
	newInbox, err := client.CreateInbox(ctx)
	if err != nil {
		log.Fatal(err)
	}
	// Continue with newInbox
}
```

### Context Cancellation

All inbox operations accept a context for cancellation and timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

emails, err := inbox.GetEmails(ctx)
if errors.Is(err, context.DeadlineExceeded) {
	log.Println("Operation timed out")
}
```

## Client Management

### Getting All Inboxes

```go
// Get all inboxes managed by this client
inboxes := client.Inboxes()
for _, inbox := range inboxes {
	fmt.Printf("Inbox: %s (expires: %s)\n",
		inbox.EmailAddress(), inbox.ExpiresAt())
}
```

### Getting a Specific Inbox

```go
inbox, exists := client.GetInbox("test@mail.example.com")
if !exists {
	log.Println("Inbox not found in client")
}
```

### Deleting a Specific Inbox

```go
err := client.DeleteInbox(ctx, "test@mail.example.com")
if err != nil {
	log.Fatal(err)
}
```

### Deleting All Inboxes

```go
count, err := client.DeleteAllInboxes(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Printf("Deleted %d inboxes\n", count)
```

### Validating API Key

```go
err := client.CheckKey(ctx)
if err != nil {
	log.Fatal("API key is invalid:", err)
}
fmt.Println("API key is valid")
```

### Getting Server Information

```go
info := client.ServerInfo()
fmt.Printf("Allowed domains: %v\n", info.AllowedDomains)
fmt.Printf("Max TTL: %v\n", info.MaxTTL)
fmt.Printf("Default TTL: %v\n", info.DefaultTTL)
```

The `ServerInfo` struct contains:

```go
type ServerInfo struct {
	AllowedDomains []string      // Email domains available for inbox creation
	MaxTTL         time.Duration // Maximum allowed TTL for inboxes
	DefaultTTL     time.Duration // Default TTL when not specified
}
```

## Next Steps

- **[Email Objects](/client-go/concepts/emails/)** - Learn about email structure
- **[Managing Inboxes](/client-go/guides/managing-inboxes/)** - Common inbox operations
- **[Import/Export](/client-go/advanced/import-export/)** - Advanced inbox persistence
- **[API Reference: Inbox](/client-go/api/inbox/)** - Complete API documentation
