# 01 - Project Structure

## Module Setup

```bash
go mod init github.com/vaultsandbox/client-go
```

## Directory Layout

```
client-go/
├── go.mod
├── go.sum
├── client.go              # Main VaultSandboxClient
├── inbox.go               # Inbox type and methods
├── email.go               # Email type and methods
├── options.go             # Functional options
├── errors.go              # Error types
├── doc.go                 # Package documentation
│
├── internal/
│   ├── api/
│   │   ├── client.go      # HTTP API client
│   │   ├── endpoints.go   # API endpoint definitions
│   │   ├── retry.go       # Retry logic
│   │   └── types.go       # API request/response types
│   │
│   ├── crypto/
│   │   ├── keypair.go     # ML-KEM-768 keypair generation
│   │   ├── decrypt.go     # Decryption pipeline
│   │   ├── verify.go      # ML-DSA-65 signature verification
│   │   ├── hkdf.go        # HKDF-SHA-512 key derivation
│   │   ├── aes.go         # AES-256-GCM operations
│   │   └── base64.go      # Base64url encoding/decoding
│   │
│   └── delivery/
│       ├── strategy.go    # Strategy interface
│       ├── sse.go         # SSE implementation
│       ├── polling.go     # Polling implementation
│       └── auto.go        # Auto strategy selection
│
├── authresults/
│   ├── authresults.go     # SPF/DKIM/DMARC types
│   └── validate.go        # Validation helpers
│
└── examples/
    ├── basic/
    │   └── main.go
    ├── waitforemail/
    │   └── main.go
    └── export/
        └── main.go
```

## Public API Surface

### Package `vaultsandbox`

**Types:**
- `Client` - Main client for managing inboxes
- `Inbox` - Represents a temporary email inbox
- `Email` - Represents a decrypted email
- `Attachment` - Email attachment data
- `ExportedInbox` - Serializable inbox data

**Functions:**
- `New(apiKey string, opts ...Option) (*Client, error)`
- `WithBaseURL(url string) Option`
- `WithHTTPClient(client *http.Client) Option`
- `WithStrategy(strategy DeliveryStrategy) Option`
- `WithTimeout(timeout time.Duration) Option`

**Interfaces:**
- `DeliveryStrategy` - Interface for SSE/polling implementations

### Package `authresults`

**Types:**
- `AuthResults` - Container for all auth results
- `SPFResult` - SPF check result
- `DKIMResult` - DKIM check result
- `DMARCResult` - DMARC check result
- `ReverseDNSResult` - Reverse DNS result

**Functions:**
- `Validate(results *AuthResults) error`

## File Contents Overview

### `client.go`
```go
package vaultsandbox

type Client struct {
    apiClient *api.Client
    strategy  DeliveryStrategy
    inboxes   map[string]*Inbox
    mu        sync.RWMutex
}

func New(apiKey string, opts ...Option) (*Client, error)
func (c *Client) CreateInbox(ctx context.Context, opts ...InboxOption) (*Inbox, error)
func (c *Client) ImportInbox(ctx context.Context, data *ExportedInbox) (*Inbox, error)
func (c *Client) DeleteInbox(ctx context.Context, emailAddress string) error
func (c *Client) DeleteAllInboxes(ctx context.Context) (int, error)
func (c *Client) GetInbox(emailAddress string) (*Inbox, bool)
func (c *Client) Close() error
```

### `inbox.go`
```go
type Inbox struct {
    emailAddress string
    expiresAt    time.Time
    inboxHash    string
    serverSigPk  []byte
    keypair      *crypto.Keypair
    client       *Client
}

func (i *Inbox) EmailAddress() string
func (i *Inbox) ExpiresAt() time.Time
func (i *Inbox) GetEmails(ctx context.Context) ([]*Email, error)
func (i *Inbox) GetEmail(ctx context.Context, emailID string) (*Email, error)
func (i *Inbox) WaitForEmail(ctx context.Context, opts ...WaitOption) (*Email, error)
func (i *Inbox) WaitForEmailCount(ctx context.Context, count int, opts ...WaitOption) ([]*Email, error)
func (i *Inbox) Delete(ctx context.Context) error
func (i *Inbox) Export() *ExportedInbox
```

### `email.go`
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

type Attachment struct {
    Filename           string
    ContentType        string
    Size               int
    ContentID          string
    ContentDisposition string
    Content            []byte
    Checksum           string
}

func (e *Email) GetRaw(ctx context.Context) (string, error)
func (e *Email) MarkAsRead(ctx context.Context) error
func (e *Email) Delete(ctx context.Context) error
```

### `options.go`
```go
type Option func(*clientConfig)
type InboxOption func(*inboxConfig)
type WaitOption func(*waitConfig)

// Client options
func WithBaseURL(url string) Option
func WithHTTPClient(client *http.Client) Option
func WithStrategy(strategy DeliveryStrategy) Option
func WithTimeout(timeout time.Duration) Option
func WithRetries(count int) Option

// Inbox options
func WithTTL(ttl time.Duration) InboxOption
func WithEmailAddress(email string) InboxOption

// Wait options
func WithSubject(subject string) WaitOption
func WithSubjectRegex(pattern *regexp.Regexp) WaitOption
func WithFrom(from string) WaitOption
func WithFromRegex(pattern *regexp.Regexp) WaitOption
func WithPredicate(fn func(*Email) bool) WaitOption
func WithWaitTimeout(timeout time.Duration) WaitOption
func WithPollInterval(interval time.Duration) WaitOption
```

## Build Tags

None required - use standard Go build.

## Versioning

Follow semantic versioning. Initial release: `v0.1.0`
