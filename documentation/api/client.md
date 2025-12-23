---
title: Client API
description: Complete API reference for the VaultSandbox Client
---

The `Client` is the main entry point for interacting with the VaultSandbox Gateway. It handles authentication, inbox creation, and provides utility methods for managing inboxes.

## Constants

TTL (time-to-live) constants for inbox creation:

```go
const (
    MinTTL = 60 * time.Second      // Minimum TTL: 1 minute
    MaxTTL = 604800 * time.Second  // Maximum TTL: 7 days
)
```

## Constructor

```go
func New(apiKey string, opts ...Option) (*Client, error)
```

Creates a new VaultSandbox client instance.

### Options

Configuration options for the client using the functional options pattern.

```go
// Available options
WithBaseURL(url string) Option
WithHTTPClient(client *http.Client) Option
WithDeliveryStrategy(strategy DeliveryStrategy) Option
WithTimeout(timeout time.Duration) Option
WithRetries(count int) Option
WithRetryOn(statusCodes []int) Option
```

#### Option Functions

| Option                | Type                | Default                            | Description                                |
| --------------------- | ------------------- | ---------------------------------- | ------------------------------------------ |
| `WithBaseURL`         | `string`            | `https://api.vaultsandbox.com`     | Gateway URL                                |
| `WithHTTPClient`      | `*http.Client`      | Default client                     | Custom HTTP client                         |
| `WithDeliveryStrategy`| `DeliveryStrategy`  | `StrategyAuto`                     | Email delivery strategy                    |
| `WithTimeout`         | `time.Duration`     | `60s`                              | Request timeout                            |
| `WithRetries`         | `int`               | `3`                                | Maximum retry attempts for HTTP requests   |
| `WithRetryOn`         | `[]int`             | `[408, 429, 500, 502, 503, 504]`   | HTTP status codes that trigger a retry     |

#### Polling Configuration Options

For advanced control over polling behavior, use `WithPollingConfig` or individual polling options:

```go
// Polling configuration struct
type PollingConfig struct {
    InitialInterval      time.Duration // Starting polling interval (default: 2s)
    MaxBackoff           time.Duration // Maximum polling interval (default: 30s)
    BackoffMultiplier    float64       // Interval multiplier after no changes (default: 1.5)
    JitterFactor         float64       // Randomness factor to prevent synchronized polling (default: 0.3)
    SSEConnectionTimeout time.Duration // Timeout for SSE before falling back to polling (default: 5s)
}

// Available polling options
WithPollingConfig(cfg PollingConfig) Option
WithPollingInitialInterval(interval time.Duration) Option
WithPollingMaxBackoff(maxBackoff time.Duration) Option
WithPollingBackoffMultiplier(multiplier float64) Option
WithPollingJitterFactor(factor float64) Option
WithSSEConnectionTimeout(timeout time.Duration) Option
```

| Option                       | Type            | Default | Description                                              |
| ---------------------------- | --------------- | ------- | -------------------------------------------------------- |
| `WithPollingConfig`          | `PollingConfig` | -       | Set all polling options at once                          |
| `WithPollingInitialInterval` | `time.Duration` | `2s`    | Starting polling interval                                |
| `WithPollingMaxBackoff`      | `time.Duration` | `30s`   | Maximum polling interval after backoff                   |
| `WithPollingBackoffMultiplier` | `float64`     | `1.5`   | Multiplier for interval after each poll with no changes  |
| `WithPollingJitterFactor`    | `float64`       | `0.3`   | Random jitter factor (30%) to prevent synchronized polling |
| `WithSSEConnectionTimeout`   | `time.Duration` | `5s`    | Timeout for SSE connection in auto mode before fallback  |

#### Delivery Strategies

```go
const (
    StrategyAuto    DeliveryStrategy = "auto"    // SSE with fallback to polling
    StrategySSE     DeliveryStrategy = "sse"     // Server-Sent Events only
    StrategyPolling DeliveryStrategy = "polling" // Periodic polling only
)
```

#### Example

```go
package main

import (
    "os"
    "time"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    client, err := vaultsandbox.New(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithBaseURL("https://api.vaultsandbox.com"),
        vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto),
        vaultsandbox.WithRetries(5),
        vaultsandbox.WithTimeout(30*time.Second),
    )
    if err != nil {
        panic(err)
    }
    defer client.Close()
}
```

## Methods

### CreateInbox

Creates a new email inbox with automatic key generation and encryption setup.

```go
func (c *Client) CreateInbox(ctx context.Context, opts ...InboxOption) (*Inbox, error)
```

#### Parameters

- `ctx`: Context for cancellation and timeouts
- `opts` (optional): Configuration options for the inbox

```go
// Available inbox options
WithTTL(ttl time.Duration) InboxOption
WithEmailAddress(email string) InboxOption
```

| Option             | Type            | Description                                                                    |
| ------------------ | --------------- | ------------------------------------------------------------------------------ |
| `WithTTL`          | `time.Duration` | Time-to-live for the inbox (min: 60s, max: 7 days, default: 1 hour)            |
| `WithEmailAddress` | `string`        | Request a specific email address (e.g., `test@inbox.vaultsandbox.com`)         |

#### Returns

- `*Inbox` - The created inbox instance
- `error` - Any error that occurred

#### Example

```go
ctx := context.Background()

// Create inbox with default settings
inbox, err := client.CreateInbox(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Println(inbox.EmailAddress())

// Create inbox with custom TTL (1 hour)
inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(time.Hour))

// Request specific email address
inbox, err := client.CreateInbox(ctx,
    vaultsandbox.WithEmailAddress("mytest@inbox.vaultsandbox.com"),
)
```

#### Errors

- `ErrUnauthorized` - Invalid API key
- `ErrInboxAlreadyExists` - Requested email address is already in use
- `*NetworkError` - Network connection failure
- `*APIError` - API-level error (invalid request, permission denied)

---

### DeleteAllInboxes

Deletes all inboxes associated with the current API key. Useful for cleanup in test environments.

```go
func (c *Client) DeleteAllInboxes(ctx context.Context) (int, error)
```

#### Returns

- `int` - Number of inboxes deleted
- `error` - Any error that occurred

#### Example

```go
deleted, err := client.DeleteAllInboxes(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Deleted %d inboxes\n", deleted)
```

#### Best Practice

Use this in test cleanup to avoid orphaned inboxes:

```go
func TestMain(m *testing.M) {
    // Setup
    client, _ := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))

    code := m.Run()

    // Cleanup
    deleted, _ := client.DeleteAllInboxes(context.Background())
    if deleted > 0 {
        log.Printf("Cleaned up %d orphaned inboxes\n", deleted)
    }
    client.Close()

    os.Exit(code)
}
```

---

### ServerInfo

Returns information about the VaultSandbox Gateway server. This information is fetched once during client initialization.

```go
func (c *Client) ServerInfo() *ServerInfo
```

#### Returns

`*ServerInfo` - Server information struct

```go
type ServerInfo struct {
    AllowedDomains []string
    MaxTTL         time.Duration
    DefaultTTL     time.Duration
}
```

| Field            | Type            | Description                               |
| ---------------- | --------------- | ----------------------------------------- |
| `AllowedDomains` | `[]string`      | List of domains allowed for inbox creation|
| `MaxTTL`         | `time.Duration` | Maximum time-to-live for inboxes          |
| `DefaultTTL`     | `time.Duration` | Default time-to-live for inboxes          |

#### Example

```go
info := client.ServerInfo()
fmt.Printf("Max TTL: %v, Default TTL: %v\n", info.MaxTTL, info.DefaultTTL)
fmt.Printf("Allowed domains: %v\n", info.AllowedDomains)
```

---

### CheckKey

Validates the API key with the server.

```go
func (c *Client) CheckKey(ctx context.Context) error
```

#### Returns

- `error` - `nil` if the API key is valid, otherwise an error

#### Example

```go
if err := client.CheckKey(ctx); err != nil {
    log.Fatal("Invalid API key:", err)
}
```

#### Usage

Useful for verifying configuration before running tests:

```go
func TestMain(m *testing.M) {
    client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
    if err != nil {
        log.Fatal(err)
    }

    if err := client.CheckKey(context.Background()); err != nil {
        log.Fatal("VaultSandbox API key is invalid:", err)
    }

    os.Exit(m.Run())
}
```

---

### WatchInboxes

Returns a channel that receives events from multiple inboxes. The channel closes when the context is cancelled.

```go
func (c *Client) WatchInboxes(ctx context.Context, inboxes ...*Inbox) <-chan *InboxEvent
```

#### Parameters

- `ctx`: Context for cancellation - when cancelled, the channel closes and all watchers are cleaned up
- `inboxes`: Variadic list of inbox instances to watch

#### Returns

- `<-chan *InboxEvent` - Receive-only channel of inbox events

#### InboxEvent Type

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

watchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
defer cancel()

for event := range client.WatchInboxes(watchCtx, inbox1, inbox2) {
    fmt.Printf("New email in %s: %s\n", event.Inbox.EmailAddress(), event.Email.Subject)
}
```

#### Behavior

- Returns immediately closed channel if no inboxes provided
- Channel has buffer size of 16
- Non-blocking sends: if channel buffer is full, events may be dropped
- All internal goroutines and watchers are cleaned up when context is cancelled

See [Real-time Monitoring Guide](/client-go/guides/real-time/) for more details.

---

### GetInbox

Retrieves an inbox by its email address from the client's managed inboxes.

```go
func (c *Client) GetInbox(emailAddress string) (*Inbox, bool)
```

#### Parameters

- `emailAddress`: The email address of the inbox to retrieve

#### Returns

- `*Inbox` - The inbox instance if found
- `bool` - `true` if the inbox was found, `false` otherwise

#### Example

```go
inbox, ok := client.GetInbox("test@inbox.vaultsandbox.com")
if !ok {
    log.Fatal("Inbox not found")
}
fmt.Println(inbox.EmailAddress())
```

---

### Inboxes

Returns all inboxes currently managed by the client.

```go
func (c *Client) Inboxes() []*Inbox
```

#### Returns

`[]*Inbox` - Slice of all managed inbox instances

#### Example

```go
for _, inbox := range client.Inboxes() {
    fmt.Printf("Inbox: %s (expires: %v)\n", inbox.EmailAddress(), inbox.ExpiresAt())
}
```

---

### DeleteInbox

Deletes a specific inbox by its email address.

```go
func (c *Client) DeleteInbox(ctx context.Context, emailAddress string) error
```

#### Parameters

- `ctx`: Context for cancellation and timeouts
- `emailAddress`: The email address of the inbox to delete

#### Returns

- `error` - Any error that occurred

#### Example

```go
err := client.DeleteInbox(ctx, "test@inbox.vaultsandbox.com")
if err != nil {
    log.Fatal(err)
}
```

---

### ExportInboxToFile

Exports an inbox to a JSON file on disk. The exported data includes sensitive key material and should be treated as confidential.

```go
func (c *Client) ExportInboxToFile(inbox *Inbox, filePath string) error
```

#### Parameters

- `inbox`: Inbox instance to export
- `filePath`: Path where the JSON file will be written

#### Returns

- `error` - Any error that occurred

#### Example

```go
inbox, _ := client.CreateInbox(ctx)

// Export to file
err := client.ExportInboxToFile(inbox, "./backup/inbox.json")
if err != nil {
    log.Fatal(err)
}

fmt.Println("Inbox exported to ./backup/inbox.json")
```

#### Security Warning

Exported data contains private encryption keys. Store securely and never commit to version control.

---

### ImportInbox

Imports a previously exported inbox, restoring all data and encryption keys.

```go
func (c *Client) ImportInbox(ctx context.Context, data *ExportedInbox) (*Inbox, error)
```

#### Parameters

- `ctx`: Context for cancellation and timeouts
- `data`: Previously exported inbox data

#### Returns

- `*Inbox` - The imported inbox instance
- `error` - Any error that occurred

#### ExportedInbox Type

```go
type ExportedInbox struct {
    EmailAddress string    `json:"emailAddress"`
    ExpiresAt    time.Time `json:"expiresAt"`
    InboxHash    string    `json:"inboxHash"`
    ServerSigPk  string    `json:"serverSigPk"`
    PublicKeyB64 string    `json:"publicKeyB64"`
    SecretKeyB64 string    `json:"secretKeyB64"`
    ExportedAt   time.Time `json:"exportedAt"`
}
```

#### Example

```go
// Load exported data
data, _ := os.ReadFile("./backup/inbox.json")

var exportedData vaultsandbox.ExportedInbox
json.Unmarshal(data, &exportedData)

inbox, err := client.ImportInbox(ctx, &exportedData)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Imported inbox: %s\n", inbox.EmailAddress())

// Use inbox normally
emails, _ := inbox.GetEmails(ctx)
```

#### Errors

- `ErrInboxAlreadyExists` - Inbox is already imported in this client
- `ErrInvalidImportData` - Import data is invalid or corrupted
- `*APIError` - Server rejected the import (inbox may not exist)

---

### ImportInboxFromFile

Imports an inbox from a JSON file.

```go
func (c *Client) ImportInboxFromFile(ctx context.Context, filePath string) (*Inbox, error)
```

#### Parameters

- `ctx`: Context for cancellation and timeouts
- `filePath`: Path to the exported inbox JSON file

#### Returns

- `*Inbox` - The imported inbox instance
- `error` - Any error that occurred

#### Example

```go
// Import from file
inbox, err := client.ImportInboxFromFile(ctx, "./backup/inbox.json")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Imported inbox: %s\n", inbox.EmailAddress())

// Watch for new emails
watchCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
defer cancel()

for email := range inbox.Watch(watchCtx) {
    fmt.Printf("New email: %s\n", email.Subject)
}
```

#### Use Cases

- Test reproducibility across runs
- Sharing inboxes between environments
- Manual testing workflows
- Debugging production issues

---

### Close

Closes the client, terminates any active SSE or polling connections, and cleans up resources.

```go
func (c *Client) Close() error
```

#### Returns

- `error` - Any error that occurred during cleanup

#### Example

```go
client, _ := vaultsandbox.New(apiKey)

defer client.Close()

inbox, _ := client.CreateInbox(ctx)
// Use inbox...
```

#### Best Practice

Always close the client when done, especially in long-running processes:

```go
var client *vaultsandbox.Client

func TestMain(m *testing.M) {
    var err error
    client, err = vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
    if err != nil {
        log.Fatal(err)
    }

    code := m.Run()

    client.Close()
    os.Exit(code)
}
```

## Errors

The client package exports sentinel errors for use with `errors.Is()` checks, as well as error types for detailed error handling.

### Sentinel Errors

```go
var (
    ErrMissingAPIKey      error // No API key provided
    ErrClientClosed       error // Operations attempted on a closed client
    ErrUnauthorized       error // Invalid or expired API key
    ErrInboxNotFound      error // Inbox not found
    ErrEmailNotFound      error // Email not found
    ErrInboxAlreadyExists error // Inbox already exists (import conflict)
    ErrInvalidImportData  error // Invalid or corrupted import data
    ErrDecryptionFailed   error // Email decryption failed
    ErrSignatureInvalid   error // Signature verification failed
    ErrRateLimited        error // API rate limit exceeded
)
```

#### Example

```go
inbox, err := client.CreateInbox(ctx)
if errors.Is(err, vaultsandbox.ErrUnauthorized) {
    log.Fatal("Invalid API key")
}
if errors.Is(err, vaultsandbox.ErrRateLimited) {
    log.Println("Rate limited, retrying...")
}
```

### Error Types

#### APIError

Represents an HTTP error from the VaultSandbox API.

```go
type APIError struct {
    StatusCode int    // HTTP status code
    Message    string // Error message from server
    RequestID  string // Request ID for support
}
```

#### NetworkError

Represents a network-level failure.

```go
type NetworkError struct {
    Err error // Underlying network error
}
```

#### SignatureVerificationError

Indicates signature verification failed, including potential server key mismatch (MITM detection).

```go
type SignatureVerificationError struct {
    Message string
}
```

### ResourceType

Used to identify which resource type an error relates to:

```go
type ResourceType string

const (
    ResourceUnknown ResourceType = ""      // Resource type not specified
    ResourceInbox   ResourceType = "inbox" // Error relates to an inbox
    ResourceEmail   ResourceType = "email" // Error relates to an email
)
```

#### Example: Type Assertions

```go
email, err := inbox.GetEmail(ctx, emailID)
if err != nil {
    var apiErr *vaultsandbox.APIError
    if errors.As(err, &apiErr) {
        log.Printf("API error %d: %s (request: %s)",
            apiErr.StatusCode, apiErr.Message, apiErr.RequestID)
    }

    var netErr *vaultsandbox.NetworkError
    if errors.As(err, &netErr) {
        log.Printf("Network error: %v", netErr.Err)
    }
}
```

## Complete Example

Here's a complete example showing typical client usage:

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
    ctx := context.Background()

    // Create client
    client, err := vaultsandbox.New(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
        vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto),
        vaultsandbox.WithRetries(5),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Verify API key
    if err := client.CheckKey(ctx); err != nil {
        log.Fatal("Invalid API key:", err)
    }

    // Get server info
    info := client.ServerInfo()
    fmt.Printf("Connected to VaultSandbox (default TTL: %v)\n", info.DefaultTTL)

    // Create inbox
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Created inbox: %s\n", inbox.EmailAddress())

    // Export for later use
    if err := client.ExportInboxToFile(inbox, "./inbox-backup.json"); err != nil {
        log.Fatal(err)
    }

    // Wait for email
    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(30*time.Second),
        vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Test`)),
    )
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Received: %s\n", email.Subject)

    // Clean up
    if err := inbox.Delete(ctx); err != nil {
        log.Fatal(err)
    }

    // Delete any other orphaned inboxes
    deleted, err := client.DeleteAllInboxes(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Cleaned up %d total inboxes\n", deleted)
}
```

## Next Steps

- [Inbox API Reference](/api/inbox-go) - Learn about inbox methods
- [Email API Reference](/api/email-go) - Work with email objects
- [Error Handling](/api/errors-go) - Handle errors gracefully
- [Import/Export Guide](/advanced/import-export) - Advanced import/export usage
