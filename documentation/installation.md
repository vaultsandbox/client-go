---
title: Client Installation
description: Install and set up the VaultSandbox Go client SDK
---

The `github.com/vaultsandbox/client-go` SDK provides a developer-friendly interface for integrating email testing into your Go applications and test suites.

## Requirements

- **Go**: 1.24 or higher
- **Platform**: Any platform supported by Go
- **VaultSandbox Gateway**: Running instance with API access

## Installation

### Go Modules

```bash
go get github.com/vaultsandbox/client-go
```

This adds the dependency to your `go.mod` file automatically.

### Manual Addition

Add to your `go.mod`:

```
require github.com/vaultsandbox/client-go v1.0.0
```

Then run:

```bash
go mod tidy
```

## Quick Start

### Basic Usage

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vaultsandbox/client-go"
)

func main() {
	client, err := vaultsandbox.New("your-api-key",
		vaultsandbox.WithBaseURL("https://mail.example.com"),
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
	fmt.Printf("Send email to: %s\n", inbox.EmailAddress())

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Received:", email.Subject)

	if err := inbox.Delete(ctx); err != nil {
		log.Fatal(err)
	}
}
```

## Verifying Installation

Create a test file `cmd/verify/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/vaultsandbox/client-go"
)

func main() {
	client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
		vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	serverInfo := client.ServerInfo()
	fmt.Println("Connected to VaultSandbox")
	fmt.Printf("Allowed domains: %v\n", serverInfo.AllowedDomains)

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		log.Fatalf("Failed to create inbox: %v", err)
	}
	fmt.Printf("Created inbox: %s\n", inbox.EmailAddress())

	if err := inbox.Delete(ctx); err != nil {
		log.Fatalf("Failed to delete inbox: %v", err)
	}
	fmt.Println("Cleanup successful")

	fmt.Println("\nInstallation verified!")
}
```

Run it:

```bash
export VAULTSANDBOX_URL=https://mail.example.com
export VAULTSANDBOX_API_KEY=your-api-key
go run cmd/verify/main.go
```

## Testing Integration

### With `go test`

```go
package myapp_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go"
)

var testClient *vaultsandbox.Client

func TestMain(m *testing.M) {
	var err error
	testClient, err = vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
		vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	)
	if err != nil {
		panic(err)
	}

	code := m.Run()

	testClient.Close()
	os.Exit(code)
}

func TestEmailWorkflow(t *testing.T) {
	ctx := context.Background()

	inbox, err := testClient.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("Failed to create inbox: %v", err)
	}
	defer inbox.Delete(ctx)

	// Trigger your application to send an email to inbox.EmailAddress()

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to receive email: %v", err)
	}

	if email.Subject == "" {
		t.Error("Expected non-empty subject")
	}
}
```

### With Testify

```go
package myapp_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vaultsandbox/client-go"
)

func TestPasswordReset(t *testing.T) {
	client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
		vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	require.NoError(t, err)
	defer inbox.Delete(ctx)

	// Trigger password reset for inbox.EmailAddress()

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithSubject("Password Reset"),
		vaultsandbox.WithWaitTimeout(30*time.Second),
	)
	require.NoError(t, err)

	assert.Contains(t, email.Text, "reset your password")
	assert.NotEmpty(t, email.Links)
}
```

## Project Structure

Recommended project structure for using the SDK:

```
myproject/
├── go.mod
├── go.sum
├── main.go
├── internal/
│   └── email/
│       └── client.go      # VaultSandbox client wrapper
└── tests/
    └── integration/
        └── email_test.go  # Integration tests
```

### Client Wrapper Example

`internal/email/client.go`:

```go
package email

import (
	"context"
	"os"
	"sync"

	"github.com/vaultsandbox/client-go"
)

var (
	client *vaultsandbox.Client
	once   sync.Once
)

// GetClient returns a shared VaultSandbox client instance.
func GetClient() (*vaultsandbox.Client, error) {
	var err error
	once.Do(func() {
		client, err = vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
			vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
		)
	})
	return client, err
}

// CreateTestInbox creates an inbox for testing purposes.
func CreateTestInbox(ctx context.Context) (*vaultsandbox.Inbox, error) {
	c, err := GetClient()
	if err != nil {
		return nil, err
	}
	return c.CreateInbox(ctx)
}
```

## Dependencies

The SDK has minimal dependencies:

```
github.com/cloudflare/circl  # Post-quantum cryptography (ML-KEM-768)
golang.org/x/crypto          # Additional crypto primitives
```

Optional development dependency:

```
github.com/joho/godotenv     # Load .env files
```

## Build Tags

The SDK works with all standard Go build configurations. No special build tags are required.

```bash
# Standard build
go build ./...

# With race detector
go build -race ./...

# Cross-compilation
GOOS=linux GOARCH=amd64 go build ./...
```

## Next Steps

- **[Configuration](/client-go/configuration/)** - Configure the client for your environment
- **[Core Concepts](/client-go/concepts/inboxes/)** - Understand inboxes, emails, and authentication
- **[Guides](/client-go/guides/managing-inboxes/)** - Learn common usage patterns
- **[Testing Patterns](/client-go/testing/password-reset/)** - Integrate with your test suite
