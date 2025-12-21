# VaultSandbox Go Client SDK

A Go client SDK for [VaultSandbox](https://vaultsandbox.com), a secure receive-only SMTP server for QA/testing environments. This SDK enables creating temporary email inboxes with quantum-safe encryption.

## Features

- Create temporary email inboxes for testing
- Quantum-safe encryption using ML-KEM-768 (Kyber768) and ML-DSA-65 (Dilithium3)
- Real-time email delivery via SSE (Server-Sent Events) or polling
- Email filtering by subject, sender, or custom predicates
- Export/import inboxes for persistence across test runs
- Authentication results validation (SPF, DKIM, DMARC)

## Installation

```bash
go get github.com/vaultsandbox/client-go
```

## Requirements

- Go 1.21 or later

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    // Create a client with your API key
    client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Create a temporary inbox
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer inbox.Delete(ctx)

    fmt.Printf("Send emails to: %s\n", inbox.EmailAddress())

    // Wait for an email
    email, err := inbox.WaitForEmail(ctx)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Subject: %s\n", email.Subject)
    fmt.Printf("From: %s\n", email.From)
    fmt.Printf("Body: %s\n", email.Text)
}
```

## Configuration

### Client Options

```go
client, err := vaultsandbox.New(apiKey,
    vaultsandbox.WithBaseURL("https://custom-api.example.com"),
    vaultsandbox.WithHTTPClient(customHTTPClient),
    vaultsandbox.WithTimeout(30 * time.Second),
    vaultsandbox.WithRetries(3),
)
```

### Inbox Options

```go
inbox, err := client.CreateInbox(ctx,
    vaultsandbox.WithTTL(10 * time.Minute),
    vaultsandbox.WithEmailAddress("custom-prefix"),  // Request specific prefix
)
```

### Wait Options

```go
// Wait for email with specific subject
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithSubject("Welcome to Our Service"),
    vaultsandbox.WithWaitTimeout(2 * time.Minute),
)

// Wait for email matching regex
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)verification`)),
    vaultsandbox.WithFromRegex(regexp.MustCompile(`@example\.com$`)),
)

// Wait for email matching custom predicate
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithPredicate(func(e *vaultsandbox.Email) bool {
        return len(e.Attachments) > 0
    }),
)
```

## Examples

See the [examples](./examples) directory for complete examples:

- **[basic](./examples/basic)** - Create inbox, fetch emails
- **[waitforemail](./examples/waitforemail)** - Wait for specific emails with filtering
- **[export](./examples/export)** - Export/import inbox for persistence

## Development

### Building

```bash
# Build the library
go build ./...

# Build examples
go build ./examples/...
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test ./internal/crypto/...
```

### Test Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View coverage in terminal
go tool cover -func=coverage.out

# View coverage in browser
go tool cover -html=coverage.out

# Generate coverage with race detector
go test -race -coverprofile=coverage.out ./...
```

# Test script

## Unit tests only
./scripts/test.sh

## Include integration tests (loads .env automatically)
./scripts/test.sh --integration

## With coverage
./scripts/test.sh --coverage

## All options
./scripts/test.sh --integration --coverage -v

Note on Integration Tests

The integration tests in integration/integration_test.go already auto-load .env via godotenv:

func TestMain(m *testing.M) {
    _ = godotenv.Load("../.env")  // Loads .env from project root
    // ...
}


### Linting

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run

# Run with auto-fix
golangci-lint run --fix
```

### Formatting

```bash
# Format all code
go fmt ./...

# Check formatting (useful in CI)
gofmt -l .
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VAULTSANDBOX_API_KEY` | Your VaultSandbox API key | Required |
| `VAULTSANDBOX_URL` | API base URL | `https://api.vaultsandbox.com` |

For local development, copy `.env.example` to `.env` and fill in your values:

```bash
cp .env.example .env
```

## Project Structure

```
.
├── client.go              # Main client implementation
├── inbox.go               # Inbox operations
├── email.go               # Email types and operations
├── options.go             # Configuration options
├── errors.go              # Error types
├── doc.go                 # Package documentation
├── examples/              # Example applications
│   ├── basic/
│   ├── waitforemail/
│   └── export/
├── internal/
│   ├── api/               # HTTP API client
│   │   ├── client.go
│   │   ├── endpoints.go
│   │   ├── retry.go
│   │   └── types.go
│   ├── crypto/            # Cryptographic operations
│   │   ├── constants.go   # Key sizes and constants
│   │   ├── errors.go      # Crypto error types
│   │   ├── base64.go      # Base64 URL encoding
│   │   ├── keypair.go     # ML-KEM-768 keypair
│   │   ├── verify.go      # ML-DSA-65 signature verification
│   │   ├── aes.go         # AES-256-GCM encryption
│   │   └── decrypt.go     # Full decryption pipeline
│   └── delivery/          # Email delivery strategies
│       ├── strategy.go
│       ├── sse.go
│       ├── polling.go
│       └── auto.go
└── authresults/           # Email authentication validation
    ├── authresults.go
    └── validate.go
```

## Cryptographic Details

This SDK implements post-quantum cryptography for secure email encryption:

| Algorithm | Purpose | Standard |
|-----------|---------|----------|
| ML-KEM-768 (Kyber768) | Key Encapsulation | FIPS 203 |
| ML-DSA-65 (Dilithium3) | Digital Signatures | FIPS 204 |
| AES-256-GCM | Symmetric Encryption | FIPS 197 |
| HKDF-SHA-512 | Key Derivation | RFC 5869 |

### Decryption Flow

1. **Signature Verification** - Verify ML-DSA-65 signature on the encrypted payload
2. **KEM Decapsulation** - Extract shared secret using ML-KEM-768
3. **Key Derivation** - Derive AES key using HKDF-SHA-512
4. **Decryption** - Decrypt email content using AES-256-GCM

## API Reference

### Client

| Method | Description |
|--------|-------------|
| `New(apiKey, ...Option)` | Create a new client |
| `CreateInbox(ctx, ...InboxOption)` | Create a temporary inbox |
| `ImportInbox(ctx, *ExportedInbox)` | Import a previously exported inbox |
| `GetInbox(emailAddress)` | Get an inbox by email address |
| `DeleteInbox(ctx, emailAddress)` | Delete an inbox |
| `DeleteAllInboxes(ctx)` | Delete all managed inboxes |
| `Close()` | Close the client |

### Inbox

| Method | Description |
|--------|-------------|
| `EmailAddress()` | Get the inbox email address |
| `ExpiresAt()` | Get the inbox expiration time |
| `GetEmails(ctx)` | Fetch all emails |
| `GetEmail(ctx, emailID)` | Fetch a specific email |
| `WaitForEmail(ctx, ...WaitOption)` | Wait for an email matching criteria |
| `Export()` | Export inbox for persistence |
| `Delete(ctx)` | Delete the inbox |

### Email

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `string` | Unique email ID |
| `From` | `string` | Sender address |
| `To` | `[]string` | Recipient addresses |
| `Subject` | `string` | Email subject |
| `Text` | `string` | Plain text body |
| `HTML` | `string` | HTML body |
| `Headers` | `map[string]string` | Email headers |
| `Attachments` | `[]Attachment` | File attachments |
| `Links` | `[]string` | Extracted links |
| `AuthResults` | `*AuthResults` | Authentication results |
| `ReceivedAt` | `time.Time` | Receive timestamp |

## License

[Add your license here]
