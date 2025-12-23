---
title: Client Configuration
description: Configure the VaultSandbox client for your environment
---

This page covers all configuration options for the VaultSandbox Go client.

## Basic Configuration

### Creating a Client

```go
import "github.com/vaultsandbox/client-go"

client, err := vaultsandbox.New("your-api-key",
	vaultsandbox.WithBaseURL("https://mail.example.com"),
)
if err != nil {
	log.Fatal(err)
}
defer client.Close()
```

## Configuration Options

### Required Parameters

#### apiKey

**Type**: `string`

**Description**: API key for authentication. Passed as the first argument to `New()`.

**Example**:

```go
client, err := vaultsandbox.New("vs_1234567890abcdef...")
```

**Best practices**:

- Store in environment variables
- Never commit to version control
- Rotate periodically

### Client Options

Options are passed as variadic arguments to `New()`.

#### WithBaseURL

**Signature**: `WithBaseURL(url string) Option`

**Default**: `https://api.vaultsandbox.com`

**Description**: Base URL of your VaultSandbox Gateway

**Examples**:

```go
vaultsandbox.WithBaseURL("https://mail.example.com")
vaultsandbox.WithBaseURL("http://localhost:3000") // Local development
```

**Requirements**:

- Must include protocol (`https://` or `http://`)
- Should not include trailing slash
- Must be accessible from your application

#### WithDeliveryStrategy

**Signature**: `WithDeliveryStrategy(strategy DeliveryStrategy) Option`

**Default**: `StrategyAuto`

**Description**: Email delivery strategy

**Options**:

- `StrategyAuto` - Automatically choose best strategy (tries SSE first, falls back to polling)
- `StrategySSE` - Server-Sent Events for real-time delivery
- `StrategyPolling` - Poll for new emails at intervals

**Examples**:

```go
vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto)    // Recommended
vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE)     // Force SSE
vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling) // Force polling
```

**When to use each**:

- `StrategyAuto`: Most use cases (recommended)
- `StrategySSE`: When you need real-time, low-latency delivery
- `StrategyPolling`: When SSE is blocked by firewall/proxy

#### WithTimeout

**Signature**: `WithTimeout(timeout time.Duration) Option`

**Default**: `60 * time.Second`

**Description**: Default timeout for wait operations (`WaitForEmail`, `WaitForEmailCount`) and API key validation during client creation. This timeout is also passed to the internal HTTP client for individual requests.

**Note**: This controls the maximum time for wait operations, not per-HTTP-request timeouts. For fine-grained HTTP timeout control, use `WithHTTPClient` with a custom `http.Client`.

**Examples**:

```go
vaultsandbox.WithTimeout(30 * time.Second)
vaultsandbox.WithTimeout(2 * time.Minute)
```

#### WithRetries

**Signature**: `WithRetries(count int) Option`

**Default**: `3`

**Description**: Maximum retry attempts for failed HTTP requests

**Examples**:

```go
vaultsandbox.WithRetries(3) // Default
vaultsandbox.WithRetries(5) // More resilient
vaultsandbox.WithRetries(0) // No retries
```

#### WithRetryOn

**Signature**: `WithRetryOn(statusCodes []int) Option`

**Default**: `[]int{408, 429, 500, 502, 503, 504}`

**Description**: HTTP status codes that trigger a retry

**Example**:

```go
vaultsandbox.WithRetryOn([]int{408, 429, 500, 502, 503, 504}) // Default
vaultsandbox.WithRetryOn([]int{500, 502, 503})                // Only server errors
vaultsandbox.WithRetryOn([]int{})                             // Never retry
```

#### WithHTTPClient

**Signature**: `WithHTTPClient(client *http.Client) Option`

**Default**: `http.DefaultClient` with timeout

**Description**: Custom HTTP client for advanced networking needs

**Example**:

```go
httpClient := &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true,
	},
}

client, err := vaultsandbox.New(apiKey,
	vaultsandbox.WithHTTPClient(httpClient),
)
```

#### WithPollingInitialInterval

**Signature**: `WithPollingInitialInterval(interval time.Duration) Option`

**Default**: `2 * time.Second`

**Description**: Initial polling interval when using polling strategy. This is the interval used when emails are actively being received.

**Example**:

```go
vaultsandbox.WithPollingInitialInterval(1 * time.Second)  // More responsive
vaultsandbox.WithPollingInitialInterval(5 * time.Second)  // Less aggressive
```

#### WithPollingMaxBackoff

**Signature**: `WithPollingMaxBackoff(maxBackoff time.Duration) Option`

**Default**: `30 * time.Second`

**Description**: Maximum polling backoff interval. When no new emails arrive, the polling interval increases exponentially up to this maximum.

**Example**:

```go
vaultsandbox.WithPollingMaxBackoff(60 * time.Second)  // Longer backoff
vaultsandbox.WithPollingMaxBackoff(10 * time.Second)  // Shorter backoff
```

#### WithPollingBackoffMultiplier

**Signature**: `WithPollingBackoffMultiplier(multiplier float64) Option`

**Default**: `1.5`

**Description**: Backoff multiplier for polling. After each poll with no changes, the interval is multiplied by this factor until it reaches the maximum backoff.

**Example**:

```go
vaultsandbox.WithPollingBackoffMultiplier(2.0)  // Faster backoff
vaultsandbox.WithPollingBackoffMultiplier(1.2)  // Slower backoff
```

#### WithPollingJitterFactor

**Signature**: `WithPollingJitterFactor(factor float64) Option`

**Default**: `0.3` (30%)

**Description**: Jitter factor for polling intervals. Random jitter up to this fraction of the interval is added to prevent synchronized polling across multiple clients (thundering herd problem).

**Example**:

```go
vaultsandbox.WithPollingJitterFactor(0.5)  // Up to 50% jitter
vaultsandbox.WithPollingJitterFactor(0.1)  // Up to 10% jitter
```

#### WithSSEConnectionTimeout

**Signature**: `WithSSEConnectionTimeout(timeout time.Duration) Option`

**Default**: `5 * time.Second`

**Description**: Timeout for SSE connection establishment when using `StrategyAuto`. If the SSE connection is not established within this timeout, the client automatically falls back to polling.

**Example**:

```go
vaultsandbox.WithSSEConnectionTimeout(10 * time.Second)  // More time for SSE
vaultsandbox.WithSSEConnectionTimeout(2 * time.Second)   // Quick fallback
```

## Inbox Options

Options passed to `CreateInbox()`.

#### WithTTL

**Signature**: `WithTTL(ttl time.Duration) InboxOption`

**Default**: `1 * time.Hour`

**Description**: Time-to-live for the inbox

**Constraints**:

- Minimum: 1 minute (`MinTTL`)
- Maximum: 7 days (`MaxTTL`) or server-configured limit

**Examples**:

```go
inbox, err := client.CreateInbox(ctx,
	vaultsandbox.WithTTL(30 * time.Minute),
)

inbox, err := client.CreateInbox(ctx,
	vaultsandbox.WithTTL(24 * time.Hour),
)
```

#### WithEmailAddress

**Signature**: `WithEmailAddress(email string) InboxOption`

**Default**: Auto-generated

**Description**: Request a specific email address

**Example**:

```go
inbox, err := client.CreateInbox(ctx,
	vaultsandbox.WithEmailAddress("test-user@mail.example.com"),
)
```

## Wait Options

Options passed to `WaitForEmail()` and `WaitForEmailCount()`.

#### WithWaitTimeout

**Signature**: `WithWaitTimeout(timeout time.Duration) WaitOption`

**Default**: `60 * time.Second`

**Description**: Maximum time to wait for email

**Example**:

```go
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithWaitTimeout(30 * time.Second),
)
```

#### WithPollInterval

**Signature**: `WithPollInterval(interval time.Duration) WaitOption`

**Default**: `2 * time.Second`

**Description**: Polling interval when using polling strategy

**Examples**:

```go
vaultsandbox.WithPollInterval(2 * time.Second)   // Default
vaultsandbox.WithPollInterval(5 * time.Second)   // Less aggressive
vaultsandbox.WithPollInterval(500 * time.Millisecond) // More responsive
```

#### WithSubject

**Signature**: `WithSubject(subject string) WaitOption`

**Description**: Filter emails by exact subject match

**Example**:

```go
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithSubject("Password Reset"),
)
```

#### WithSubjectRegex

**Signature**: `WithSubjectRegex(pattern *regexp.Regexp) WaitOption`

**Description**: Filter emails by subject regex

**Example**:

```go
pattern := regexp.MustCompile(`(?i)password.*reset`)
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithSubjectRegex(pattern),
)
```

#### WithFrom

**Signature**: `WithFrom(from string) WaitOption`

**Description**: Filter emails by exact sender match

**Example**:

```go
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithFrom("noreply@example.com"),
)
```

#### WithFromRegex

**Signature**: `WithFromRegex(pattern *regexp.Regexp) WaitOption`

**Description**: Filter emails by sender regex

**Example**:

```go
pattern := regexp.MustCompile(`@example\.com$`)
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithFromRegex(pattern),
)
```

#### WithPredicate

**Signature**: `WithPredicate(fn func(*Email) bool) WaitOption`

**Description**: Filter emails by custom predicate function

**Example**:

```go
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithPredicate(func(e *vaultsandbox.Email) bool {
		return len(e.Attachments) > 0
	}),
)
```

## Configuration Examples

### Production Configuration

```go
client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
	vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto),
	vaultsandbox.WithRetries(5),
	vaultsandbox.WithTimeout(60 * time.Second),
)
```

### CI/CD Configuration

```go
client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
	vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto),
	vaultsandbox.WithRetries(3),
	vaultsandbox.WithTimeout(30 * time.Second),
)
```

### Development Configuration

```go
client, err := vaultsandbox.New("dev-api-key",
	vaultsandbox.WithBaseURL("http://localhost:3000"),
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
	vaultsandbox.WithRetries(1),
)
```

### High-Reliability Configuration

```go
client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
	vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	vaultsandbox.WithRetries(10),
	vaultsandbox.WithRetryOn([]int{408, 429, 500, 502, 503, 504}),
	vaultsandbox.WithTimeout(2 * time.Minute),
)
```

### Custom Polling Configuration

```go
client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
	vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
	vaultsandbox.WithPollingInitialInterval(1 * time.Second),  // Faster initial polling
	vaultsandbox.WithPollingMaxBackoff(15 * time.Second),      // Lower max backoff
	vaultsandbox.WithPollingBackoffMultiplier(1.2),            // Slower backoff growth
	vaultsandbox.WithPollingJitterFactor(0.2),                 // Less jitter
)
```

### Tuned Auto Mode Configuration

```go
client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
	vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto),
	vaultsandbox.WithSSEConnectionTimeout(10 * time.Second),   // More time for SSE
	vaultsandbox.WithPollingInitialInterval(2 * time.Second),  // Fallback polling config
	vaultsandbox.WithPollingMaxBackoff(30 * time.Second),
)
```

## Environment Variables

Store configuration in environment variables:

### `.env` File

```bash
VAULTSANDBOX_URL=https://mail.example.com
VAULTSANDBOX_API_KEY=vs_1234567890abcdef...
```

### Usage

```go
import (
	"os"

	"github.com/joho/godotenv"
	"github.com/vaultsandbox/client-go"
)

func main() {
	// Load .env file
	godotenv.Load()

	client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
		vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
}
```

## Client Methods

### CreateInbox()

Create a new temporary email inbox:

```go
inbox, err := client.CreateInbox(ctx,
	vaultsandbox.WithTTL(30 * time.Minute),
	vaultsandbox.WithEmailAddress("test@mail.example.com"),
)
```

**Signature**: `CreateInbox(ctx context.Context, opts ...InboxOption) (*Inbox, error)`

**Returns**: A new `*Inbox` that can receive emails.

### ImportInbox()

Import a previously exported inbox:

```go
inbox, err := client.ImportInbox(ctx, exportedData)
```

**Signature**: `ImportInbox(ctx context.Context, data *ExportedInbox) (*Inbox, error)`

**Returns**: The restored `*Inbox`.

**Errors**: Returns `ErrInboxAlreadyExists` if the inbox is already managed by this client.

### DeleteInbox()

Delete an inbox by email address:

```go
err := client.DeleteInbox(ctx, "test@mail.example.com")
```

**Signature**: `DeleteInbox(ctx context.Context, emailAddress string) error`

### DeleteAllInboxes()

Delete all inboxes managed by this client:

```go
count, err := client.DeleteAllInboxes(ctx)
fmt.Printf("Deleted %d inboxes\n", count)
```

**Signature**: `DeleteAllInboxes(ctx context.Context) (int, error)`

**Returns**: The number of inboxes deleted.

### GetInbox()

Get an inbox by email address:

```go
inbox, exists := client.GetInbox("test@mail.example.com")
if exists {
	// Use inbox
}
```

**Signature**: `GetInbox(emailAddress string) (*Inbox, bool)`

**Returns**: The inbox and `true` if found, otherwise `nil` and `false`.

### Inboxes()

Get all inboxes managed by this client:

```go
inboxes := client.Inboxes()
for _, inbox := range inboxes {
	fmt.Println(inbox.EmailAddress())
}
```

**Signature**: `Inboxes() []*Inbox`

### ServerInfo()

Get server configuration:

```go
info := client.ServerInfo()
fmt.Printf("Max TTL: %v\n", info.MaxTTL)
fmt.Printf("Allowed domains: %v\n", info.AllowedDomains)
```

**Signature**: `ServerInfo() *ServerInfo`

**Returns**: `*ServerInfo` containing:

- `AllowedDomains []string` - Email domains the server accepts
- `MaxTTL time.Duration` - Maximum inbox TTL allowed
- `DefaultTTL time.Duration` - Default inbox TTL

### CheckKey()

Validate the API key:

```go
err := client.CheckKey(ctx)
if err != nil {
	log.Fatal("Invalid API key")
}
```

**Signature**: `CheckKey(ctx context.Context) error`

**Note**: The API key is automatically validated when creating a client with `New()`.

### ExportInboxToFile()

Export an inbox to a JSON file:

```go
err := client.ExportInboxToFile(inbox, "/path/to/inbox.json")
```

**Signature**: `ExportInboxToFile(inbox *Inbox, filePath string) error`

**Warning**: The exported file contains private key material. Handle securely.

### ImportInboxFromFile()

Import an inbox from a JSON file:

```go
inbox, err := client.ImportInboxFromFile(ctx, "/path/to/inbox.json")
```

**Signature**: `ImportInboxFromFile(ctx context.Context, filePath string) (*Inbox, error)`

### WatchInboxes()

Watch multiple inboxes for new emails via a channel:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

for event := range client.WatchInboxes(ctx, inbox1, inbox2) {
	fmt.Printf("New email in %s: %s\n", event.Inbox.EmailAddress(), event.Email.Subject)
}
```

**Signature**: `WatchInboxes(ctx context.Context, inboxes ...*Inbox) <-chan *InboxEvent`

**Returns**: A receive-only channel of `*InboxEvent` that closes when context is cancelled.

### Close()

Close the client and clean up resources:

```go
err := client.Close()
```

**What it does**:

- Terminates all active SSE connections
- Stops all polling operations
- Cleans up resources

**When to use**:

- After test suite completes
- Before process exit
- When client is no longer needed

**Example**:

```go
client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(url))
if err != nil {
	log.Fatal(err)
}
defer client.Close()

// Use client
inbox, err := client.CreateInbox(ctx)
// ...
```

## Inbox Methods

### EmailAddress()

Get the inbox email address:

```go
email := inbox.EmailAddress()
```

**Signature**: `EmailAddress() string`

### ExpiresAt()

Get when the inbox expires:

```go
expiry := inbox.ExpiresAt()
fmt.Printf("Expires at: %v\n", expiry)
```

**Signature**: `ExpiresAt() time.Time`

### InboxHash()

Get the SHA-256 hash of the public key:

```go
hash := inbox.InboxHash()
```

**Signature**: `InboxHash() string`

### IsExpired()

Check if the inbox has expired:

```go
if inbox.IsExpired() {
	// Create a new inbox
}
```

**Signature**: `IsExpired() bool`

### GetSyncStatus()

Get the synchronization status of the inbox:

```go
status, err := inbox.GetSyncStatus(ctx)
fmt.Printf("Email count: %d\n", status.EmailCount)
fmt.Printf("Emails hash: %s\n", status.EmailsHash)
```

**Signature**: `GetSyncStatus(ctx context.Context) (*SyncStatus, error)`

**Returns**: `*SyncStatus` containing:

- `EmailCount int` - Number of emails in the inbox
- `EmailsHash string` - Hash of the email list for change detection

### GetEmails()

Fetch all emails in the inbox:

```go
emails, err := inbox.GetEmails(ctx)
for _, email := range emails {
	fmt.Printf("Subject: %s\n", email.Subject)
}
```

**Signature**: `GetEmails(ctx context.Context) ([]*Email, error)`

### GetEmail()

Fetch a specific email by ID:

```go
email, err := inbox.GetEmail(ctx, "email-id-123")
```

**Signature**: `GetEmail(ctx context.Context, emailID string) (*Email, error)`

### WaitForEmail()

Wait for an email matching the given criteria:

```go
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithSubject("Welcome"),
	vaultsandbox.WithWaitTimeout(30 * time.Second),
)
```

**Signature**: `WaitForEmail(ctx context.Context, opts ...WaitOption) (*Email, error)`

### WaitForEmailCount()

Wait until the inbox has at least N emails matching criteria:

```go
emails, err := inbox.WaitForEmailCount(ctx, 3,
	vaultsandbox.WithFrom("noreply@example.com"),
)
```

**Signature**: `WaitForEmailCount(ctx context.Context, count int, opts ...WaitOption) ([]*Email, error)`

### Delete()

Delete the inbox:

```go
err := inbox.Delete(ctx)
```

**Signature**: `Delete(ctx context.Context) error`

### Export()

Export inbox data including private key:

```go
data := inbox.Export()
// data can be serialized to JSON and stored
```

**Signature**: `Export() *ExportedInbox`

**Warning**: The returned data contains private key material. Handle securely.

### Watch()

Watch for new email notifications via a channel:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

for email := range inbox.Watch(ctx) {
	fmt.Printf("New email: %s\n", email.Subject)
}
```

**Signature**: `Watch(ctx context.Context) <-chan *Email`

**Returns**: A receive-only channel of `*Email` that closes when context is cancelled.

## Email Operations

`Email` is a pure data struct with no methods. Use `Inbox` methods to perform operations on emails.

### GetRawEmail()

Fetch the raw email content (RFC 5322 format):

```go
raw, err := inbox.GetRawEmail(ctx, email.ID)
fmt.Println(raw)
```

**Signature**: `GetRawEmail(ctx context.Context, emailID string) (string, error)`

### MarkEmailAsRead()

Mark an email as read:

```go
err := inbox.MarkEmailAsRead(ctx, email.ID)
```

**Signature**: `MarkEmailAsRead(ctx context.Context, emailID string) error`

### DeleteEmail()

Delete an email:

```go
err := inbox.DeleteEmail(ctx, email.ID)
```

**Signature**: `DeleteEmail(ctx context.Context, emailID string) error`

## Types

### Email

Represents a decrypted email:

```go
type Email struct {
	ID          string
	From        string
	To          []string
	Subject     string
	Text        string
	HTML        string
	ReceivedAt  time.Time
	Headers     map[string]string
	Attachments []Attachment
	Links       []string
	AuthResults *authresults.AuthResults
	IsRead      bool
}
```

### Attachment

Represents an email attachment:

```go
type Attachment struct {
	Filename           string
	ContentType        string
	Size               int
	ContentID          string
	ContentDisposition string
	Content            []byte
	Checksum           string
}
```

### ServerInfo

Contains server configuration:

```go
type ServerInfo struct {
	AllowedDomains []string
	MaxTTL         time.Duration
	DefaultTTL     time.Duration
}
```

### SyncStatus

Represents the synchronization status of an inbox:

```go
type SyncStatus struct {
	EmailCount int
	EmailsHash string
}
```

### ExportedInbox

Contains all data needed to restore an inbox:

```go
type ExportedInbox struct {
	EmailAddress string
	ExpiresAt    time.Time
	InboxHash    string
	ServerSigPk  string
	PublicKeyB64 string
	SecretKeyB64 string
	ExportedAt   time.Time
}
```

**Warning**: Contains private key material. Handle securely.

### InboxEvent

Event struct returned when watching multiple inboxes:

```go
type InboxEvent struct {
	Inbox *Inbox  // The inbox that received the email
	Email *Email  // The received email
}
```

## Strategy Selection Guide

### Auto (Recommended)

**Use when**: You want optimal performance with automatic fallback

**Behavior**:

1. Tries SSE first
2. Falls back to polling if SSE fails
3. Automatically reconnects on errors

**Pros**:

- Best of both worlds
- No manual configuration needed
- Resilient to network issues

**Cons**:

- Slightly more complex internally

### SSE (Server-Sent Events)

**Use when**: You need real-time, low-latency delivery

**Behavior**:

- Persistent connection to server
- Push-based email notification
- Instant delivery

**Pros**:

- Real-time delivery (no polling delay)
- Efficient (no repeated HTTP requests)
- Deterministic tests

**Cons**:

- Requires persistent connection
- May be blocked by some proxies/firewalls
- More complex error handling

### Polling

**Use when**: SSE is blocked or unreliable

**Behavior**:

- Periodic HTTP requests for new emails
- Pull-based email retrieval
- Configurable interval

**Pros**:

- Works in all network environments
- No persistent connection required
- Simple and predictable

**Cons**:

- Delay based on polling interval
- More HTTP requests
- Less efficient than SSE

## Best Practices

### Security

**Do**:

```go
// Use environment variables
client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
	vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
)
```

**Don't**:

```go
// Hard-code credentials
client, err := vaultsandbox.New("vs_1234567890...", // Never do this
	vaultsandbox.WithBaseURL("https://mail.example.com"),
)
```

### Resource Management

**Do**:

```go
client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(url))
if err != nil {
	log.Fatal(err)
}
defer client.Close() // Always clean up

runTests()
```

**Don't**:

```go
client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(url))
runTests()
// Forgot to close, resources leak
```

### Error Handling

**Error Types**:

- `APIError` - HTTP errors from the API with `StatusCode`, `Message`, `RequestID`, and `ResourceType` fields
- `NetworkError` - Network-level failures with `Err` (underlying error), `URL`, and `Attempt` fields
- `SignatureVerificationError` - Signature verification failures with `Message` and `IsKeyMismatch` fields

**Sentinel Errors** (use with `errors.Is()`):

- `ErrMissingAPIKey` - No API key provided to `New()`
- `ErrClientClosed` - Client has been closed
- `ErrUnauthorized` - Invalid or expired API key
- `ErrInboxNotFound` - Inbox does not exist or has expired
- `ErrEmailNotFound` - Email does not exist
- `ErrInboxAlreadyExists` - Inbox already exists (when importing duplicate)
- `ErrInvalidImportData` - Invalid inbox import data
- `ErrDecryptionFailed` - Email decryption failed
- `ErrSignatureInvalid` - Signature verification failed
- `ErrRateLimited` - API rate limit exceeded

**Timeouts**: Use `context.DeadlineExceeded` for timeout handling

**Do**:

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
	var apiErr *vaultsandbox.APIError
	var netErr *vaultsandbox.NetworkError

	switch {
	case errors.As(err, &apiErr):
		log.Printf("API error %d: %s (request: %s)", apiErr.StatusCode, apiErr.Message, apiErr.RequestID)
	case errors.As(err, &netErr):
		log.Printf("Network error on %s (attempt %d): %v", netErr.URL, netErr.Attempt, netErr.Err)
	case errors.Is(err, vaultsandbox.ErrUnauthorized):
		log.Printf("Invalid API key")
	case errors.Is(err, vaultsandbox.ErrRateLimited):
		log.Printf("Rate limited, retry later")
	default:
		log.Printf("Unexpected error: %v", err)
	}
	return
}
```

### Context Usage

**Do**:

```go
// Use context for cancellation and timeouts
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

email, err := inbox.WaitForEmail(ctx)
```

**Don't**:

```go
// Don't use context.Background() for long operations without timeout
email, err := inbox.WaitForEmail(context.Background()) // May hang forever
```

## Next Steps

- **[Core Concepts: Inboxes](/client-go/concepts/inboxes/)** - Learn about inboxes
- **[Managing Inboxes](/client-go/guides/managing-inboxes/)** - Common inbox operations
- **[Testing Patterns](/client-go/testing/password-reset/)** - Integrate with your tests
- **[API Reference: Client](/client-go/api/client/)** - Full API documentation
