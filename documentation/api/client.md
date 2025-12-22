---
title: Client API
description: Complete API reference for the VaultSandbox Client
---

The `Client` is the main entry point for interacting with the VaultSandbox Gateway. It handles authentication, inbox creation, and provides utility methods for managing inboxes.

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

### MonitorInboxes

Monitors multiple inboxes simultaneously and invokes callbacks when new emails arrive.

```go
func (c *Client) MonitorInboxes(inboxes []*Inbox) (*InboxMonitor, error)
```

#### Parameters

- `inboxes`: Slice of inbox instances to monitor

#### Returns

- `*InboxMonitor` - Monitor for managing email callbacks
- `error` - Any error that occurred

#### Example

```go
inbox1, _ := client.CreateInbox(ctx)
inbox2, _ := client.CreateInbox(ctx)

monitor, err := client.MonitorInboxes([]*vaultsandbox.Inbox{inbox1, inbox2})
if err != nil {
    log.Fatal(err)
}

sub := monitor.OnEmail(func(inbox *vaultsandbox.Inbox, email *vaultsandbox.Email) {
    fmt.Printf("New email in %s: %s\n", inbox.EmailAddress(), email.Subject)
})

// Later, stop monitoring
sub.Unsubscribe()

// Or stop all monitoring
monitor.Unsubscribe()
```

See [InboxMonitor API](/api/inbox-go#inboxmonitor) for more details.

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

// Subscribe to new emails
sub := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
    fmt.Printf("New email: %s\n", email.Subject)
})
defer sub.Unsubscribe()
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
