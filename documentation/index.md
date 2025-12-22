---
title: Go Client
description: Overview of the VaultSandbox Go SDK for email testing
---

The official Go SDK for VaultSandbox Gateway. It handles quantum-safe encryption automatically, letting you focus on testing email workflows.

## Key Capabilities

- **Automatic Encryption**: ML-KEM-768 key encapsulation + AES-256-GCM encryption handled transparently
- **Real-Time Delivery**: SSE-based email delivery with smart polling fallback
- **Email Authentication**: Built-in SPF/DKIM/DMARC validation helpers
- **Full Email Access**: Decrypted content, headers, links, and attachments
- **Idiomatic Go**: Context-based cancellation, functional options, and proper error handling

## Requirements

- Go 1.24+
- VaultSandbox Gateway server
- Valid API key

## Gateway Server

The SDK connects to a VaultSandbox Gateway - a receive-only SMTP server you self-host. It handles email reception, authentication validation, and encryption. You can run one with Docker in minutes.

See [Gateway Overview](/gateway/) or jump to [Quick Start](/getting-started/quickstart/) to deploy one.

## Quick Example

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
		vaultsandbox.WithBaseURL("https://gateway.example.com"),
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

	// Send email to inbox.EmailAddress() from your application...

	// Wait for email
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Subject:", email.Subject)
	fmt.Println("Text:", email.Text)

	// Cleanup
	if err := inbox.Delete(ctx); err != nil {
		log.Fatal(err)
	}
}
```

## Links

- [GitHub Repository](https://github.com/vaultsandbox/client-go)
- [Go Package](https://pkg.go.dev/github.com/vaultsandbox/client-go)

## Next Steps

- [Installation](/client-go/installation/) - Install the SDK
- [Configuration](/client-go/configuration/) - Client options and setup
- [Core Concepts](/client-go/concepts/inboxes/) - Inboxes, emails, and authentication
- [API Reference](/client-go/api/client/) - Full API documentation
