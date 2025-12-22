---
title: Inbox Import/Export
description: Learn how to export and import inboxes for test reproducibility and cross-environment sharing
---

VaultSandbox allows you to export and import inboxes, including their encryption keys and metadata. This enables advanced workflows like test reproducibility, manual testing, cross-environment sharing, and debugging.

## Overview

When you export an inbox, you get an `ExportedInbox` struct containing:

- Email address
- Inbox identifier (hash)
- Expiration time
- **Public encryption key** (base64-encoded)
- **Secret encryption key** (sensitive!)
- **Server public signing key**
- Export timestamp

This exported data can be imported into another client instance, allowing you to access the same inbox from different environments or at different times.

## Security Warning

**Exported inbox data contains private encryption keys.** Anyone with this data can:

- Read all emails in the inbox
- Impersonate the inbox to receive new emails
- Decrypt all future emails sent to the inbox

**Never:**

- Commit exported data to version control
- Share exported data over insecure channels
- Store exported data in plaintext in production

**Always:**

- Treat exported data as sensitive credentials
- Encrypt exported files at rest
- Use secure channels for sharing
- Rotate/delete inboxes after use

## Use Cases

### 1. Test Reproducibility

Export an inbox at the end of a test run to reproduce issues later:

```go
package mytest

import (
    "context"
    "encoding/json"
    "os"
    "testing"
    "time"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func TestEmailFlow(t *testing.T) {
    ctx := context.Background()

    client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
    if err != nil {
        t.Fatal(err)
    }
    defer client.Close()

    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }

    // Ensure cleanup, but export on failure
    defer func() {
        if t.Failed() {
            // Export on test failure
            exportData := inbox.Export()
            jsonData, _ := json.MarshalIndent(exportData, "", "  ")
            filename := fmt.Sprintf("./debug/inbox-%d.json", time.Now().Unix())
            os.WriteFile(filename, jsonData, 0600)
            t.Logf("Inbox exported to %s", filename)
        }
        inbox.Delete(ctx)
    }()

    // Send welcome email to inbox
    sendWelcomeEmail(inbox.EmailAddress())

    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Welcome`)),
        vaultsandbox.WithWaitTimeout(10*time.Second),
    )
    if err != nil {
        t.Fatal(err)
    }

    if !strings.Contains(email.Subject, "Welcome") {
        t.Errorf("expected subject to contain 'Welcome', got %s", email.Subject)
    }
}
```

### 2. Manual Testing

Export an inbox from automated tests for manual verification:

```go
package main

import (
    "context"
    "fmt"
    "os"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    ctx := context.Background()

    client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
    if err != nil {
        panic(err)
    }
    defer client.Close()

    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        panic(err)
    }

    // Export for manual testing
    if err := client.ExportInboxToFile(inbox, "./manual-test-inbox.json"); err != nil {
        panic(err)
    }

    fmt.Printf("Manual test inbox: %s\n", inbox.EmailAddress())
    fmt.Println("Exported to: ./manual-test-inbox.json")

    // Continue with automated tests...
}
```

Then manually inspect:

```bash
# Use the exported inbox in a manual test script
go run scripts/check-inbox/main.go ./manual-test-inbox.json
```

### 3. Cross-Environment Sharing

Export an inbox from one environment and import it in another:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func exportInDev() {
    ctx := context.Background()

    devClient, err := vaultsandbox.New(
        os.Getenv("DEV_API_KEY"),
        vaultsandbox.WithBaseURL("https://dev.vaultsandbox.com"),
    )
    if err != nil {
        panic(err)
    }
    defer devClient.Close()

    inbox, err := devClient.CreateInbox(ctx)
    if err != nil {
        panic(err)
    }

    exportData := inbox.Export()

    // Save to shared location
    jsonData, _ := json.MarshalIndent(exportData, "", "  ")
    os.WriteFile("./shared/staging-inbox.json", jsonData, 0600)

    fmt.Printf("Exported: %s\n", inbox.EmailAddress())
}

func importInStaging() {
    ctx := context.Background()

    // Note: Must use same server URL!
    stagingClient, err := vaultsandbox.New(
        os.Getenv("STAGING_API_KEY"),
        vaultsandbox.WithBaseURL("https://dev.vaultsandbox.com"),
    )
    if err != nil {
        panic(err)
    }
    defer stagingClient.Close()

    inbox, err := stagingClient.ImportInboxFromFile(ctx, "./shared/staging-inbox.json")
    if err != nil {
        panic(err)
    }

    fmt.Printf("Imported inbox: %s\n", inbox.EmailAddress())
}
```

### 4. Debugging Production Issues

Export a problematic inbox from production for local debugging:

```go
package main

import (
    "context"
    "fmt"
    "os"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func debugProductionIssue() {
    ctx := context.Background()

    // Local development: Import and investigate
    localClient, err := vaultsandbox.New(
        os.Getenv("LOCAL_API_KEY"),
        vaultsandbox.WithBaseURL("https://smtp.vaultsandbox.com"), // Same server as production
    )
    if err != nil {
        panic(err)
    }
    defer localClient.Close()

    inbox, err := localClient.ImportInboxFromFile(ctx, "./production-issue-123.json")
    if err != nil {
        panic(err)
    }

    // Check emails
    emails, err := inbox.GetEmails(ctx)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Found %d emails\n", len(emails))

    for _, email := range emails {
        fmt.Println("\n---")
        fmt.Printf("Subject: %s\n", email.Subject)
        fmt.Printf("From: %s\n", email.From)
        fmt.Printf("Received: %s\n", email.ReceivedAt.Format(time.RFC3339))
        fmt.Printf("Links: %d\n", len(email.Links))
        fmt.Printf("Attachments: %d\n", len(email.Attachments))
    }
}
```

## Export Methods

### Export to Struct

```go
func (i *Inbox) Export() *ExportedInbox
```

Returns an `ExportedInbox` struct with the inbox data:

```go
inbox, _ := client.CreateInbox(ctx)
data := inbox.Export()

fmt.Printf("Email: %s\n", data.EmailAddress)
fmt.Printf("Expires: %s\n", data.ExpiresAt.Format(time.RFC3339))
fmt.Printf("Exported: %s\n", data.ExportedAt.Format(time.RFC3339))

// Save to file manually
jsonData, _ := json.MarshalIndent(data, "", "  ")
os.WriteFile("inbox.json", jsonData, 0600)
```

The `ExportedInbox` struct contains:

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

### Export to File

```go
func (c *Client) ExportInboxToFile(inbox *Inbox, filePath string) error
```

Directly writes the inbox data to a JSON file with secure permissions (0600):

```go
inbox, _ := client.CreateInbox(ctx)

// Export to file
err := client.ExportInboxToFile(inbox, "./backups/inbox.json")
if err != nil {
    log.Fatal(err)
}
```

## Import Methods

### Import from Struct

```go
func (c *Client) ImportInbox(ctx context.Context, data *ExportedInbox) (*Inbox, error)
```

Imports inbox data from an `ExportedInbox` struct:

```go
// Read and parse JSON file
jsonData, _ := os.ReadFile("./backup.json")

var exportedData vaultsandbox.ExportedInbox
json.Unmarshal(jsonData, &exportedData)

inbox, err := client.ImportInbox(ctx, &exportedData)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Imported: %s\n", inbox.EmailAddress())

// Use inbox normally
emails, _ := inbox.GetEmails(ctx)
```

### Import from File

```go
func (c *Client) ImportInboxFromFile(ctx context.Context, filePath string) (*Inbox, error)
```

Directly imports an inbox from a JSON file:

```go
inbox, err := client.ImportInboxFromFile(ctx, "./backups/inbox.json")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Imported: %s\n", inbox.EmailAddress())

// Monitor for new emails
subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
    fmt.Printf("New email: %s\n", email.Subject)
})
defer subscription.Unsubscribe()
```

## Import Validation

The SDK validates imported data and returns errors for invalid imports:

```go
import (
    "errors"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

inbox, err := client.ImportInbox(ctx, data)
if err != nil {
    if errors.Is(err, vaultsandbox.ErrInvalidImportData) {
        fmt.Println("Invalid import data:", err)
        // Possible causes:
        // - Missing required fields
        // - Invalid encryption keys
        // - Corrupted JSON
    } else if errors.Is(err, vaultsandbox.ErrInboxAlreadyExists) {
        fmt.Println("Inbox already imported in this client")
        // The inbox is already available in this client instance
    } else if errors.Is(err, vaultsandbox.ErrInboxNotFound) {
        fmt.Println("Inbox no longer exists on server")
        // The inbox may have expired or been deleted
    } else {
        fmt.Println("Import failed:", err)
    }
}
```

## Complete Examples

### Manual Testing Workflow

```go
// cmd/export-test-inbox/main.go
package main

import (
    "context"
    "fmt"
    "os"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    ctx := context.Background()

    client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
    if err != nil {
        panic(err)
    }
    defer client.Close()

    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Created test inbox: %s\n", inbox.EmailAddress())
    fmt.Printf("Expires at: %s\n", inbox.ExpiresAt().Format(time.RFC3339))

    // Export for manual use
    if err := client.ExportInboxToFile(inbox, "./tmp/test-inbox.json"); err != nil {
        panic(err)
    }
    fmt.Println("Exported to: ./tmp/test-inbox.json")

    fmt.Println("\nSend test emails to this address, then run:")
    fmt.Println("  go run cmd/check-test-inbox/main.go")
}
```

```go
// cmd/check-test-inbox/main.go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    ctx := context.Background()

    client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Import the test inbox
    inbox, err := client.ImportInboxFromFile(ctx, "./tmp/test-inbox.json")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Monitoring: %s\n\n", inbox.EmailAddress())

    // Show existing emails
    emails, err := inbox.GetEmails(ctx)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Found %d existing emails:\n\n", len(emails))

    for i, email := range emails {
        fmt.Printf("%d. \"%s\" from %s\n", i+1, email.Subject, email.From)
        fmt.Printf("   Received: %s\n", email.ReceivedAt.Format(time.RFC1123))
        fmt.Printf("   Links: %d\n\n", len(email.Links))
    }

    // Monitor for new emails
    fmt.Println("Waiting for new emails (Ctrl+C to exit)...\n")

    subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
        fmt.Println("New email received!")
        fmt.Printf("   Subject: %s\n", email.Subject)
        fmt.Printf("   From: %s\n", email.From)
        fmt.Printf("   Received: %s\n\n", email.ReceivedAt.Format(time.RFC1123))
    })
    defer subscription.Unsubscribe()

    // Wait for interrupt signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
}
```

### Test Debugging Workflow

```go
// testutil/inbox_helper.go
package testutil

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "testing"
    "time"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

// TestInbox wraps an inbox with automatic export on test failure.
type TestInbox struct {
    *vaultsandbox.Inbox
    t         *testing.T
    client    *vaultsandbox.Client
    exportDir string
}

// NewTestInbox creates a new inbox that exports on test failure.
func NewTestInbox(t *testing.T, client *vaultsandbox.Client, exportDir string) *TestInbox {
    ctx := context.Background()

    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }

    ti := &TestInbox{
        Inbox:     inbox,
        t:         t,
        client:    client,
        exportDir: exportDir,
    }

    t.Cleanup(func() {
        if t.Failed() {
            ti.exportForDebugging()
        }
        inbox.Delete(context.Background())
    })

    return ti
}

func (ti *TestInbox) exportForDebugging() {
    os.MkdirAll(ti.exportDir, 0755)

    filename := fmt.Sprintf("inbox-%s-%d.json",
        ti.t.Name(),
        time.Now().Unix(),
    )
    filepath := filepath.Join(ti.exportDir, filename)

    if err := ti.client.ExportInboxToFile(ti.Inbox, filepath); err != nil {
        ti.t.Logf("Failed to export inbox: %v", err)
        return
    }

    ti.t.Logf("Exported failed test inbox to: %s", filepath)
}
```

Usage in tests:

```go
func TestEmailWorkflow(t *testing.T) {
    client, _ := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
    defer client.Close()

    inbox := testutil.NewTestInbox(t, client, "./debug")

    // Test code here...
    // If test fails, inbox is automatically exported to ./debug/
}
```

### Cross-Environment Sync

```go
// cmd/sync-inbox-to-staging/main.go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    ctx := context.Background()

    // Export from development
    devClient, err := vaultsandbox.New(
        os.Getenv("DEV_API_KEY"),
        vaultsandbox.WithBaseURL("https://dev.vaultsandbox.com"),
    )
    if err != nil {
        panic(err)
    }
    defer devClient.Close()

    devInbox, err := devClient.CreateInbox(ctx)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Created dev inbox: %s\n", devInbox.EmailAddress())

    // Export
    exportData := devInbox.Export()
    exportPath := "./tmp/staging-sync.json"

    jsonData, _ := json.MarshalIndent(exportData, "", "  ")
    os.WriteFile(exportPath, jsonData, 0600)

    fmt.Printf("Exported to: %s\n", exportPath)
    fmt.Println("\nRun in staging environment:")
    fmt.Println("  go run cmd/import-from-dev/main.go")

    // Keep inbox alive
    fmt.Println("\nInbox will remain active for manual testing...")
    fmt.Println("Press Ctrl+C to exit")
    select {}
}
```

```go
// cmd/import-from-dev/main.go
package main

import (
    "context"
    "fmt"
    "os"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    ctx := context.Background()

    stagingClient, err := vaultsandbox.New(
        os.Getenv("STAGING_API_KEY"),
        vaultsandbox.WithBaseURL("https://dev.vaultsandbox.com"), // Same server!
    )
    if err != nil {
        panic(err)
    }
    defer stagingClient.Close()

    inbox, err := stagingClient.ImportInboxFromFile(ctx, "./tmp/staging-sync.json")
    if err != nil {
        panic(err)
    }

    fmt.Printf("Imported inbox: %s\n", inbox.EmailAddress())
    fmt.Println("Checking for emails...\n")

    emails, _ := inbox.GetEmails(ctx)
    for _, email := range emails {
        fmt.Printf("- %s (%s)\n", email.Subject, email.From)
    }
}
```

## Best Practices

### 1. Secure Storage

Never store exported data in plaintext. Use encryption:

```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/sha256"
    "encoding/json"
    "io"
)

func exportInboxSecurely(inbox *vaultsandbox.Inbox, password string) ([]byte, error) {
    data := inbox.Export()
    jsonData, err := json.Marshal(data)
    if err != nil {
        return nil, err
    }

    // Derive key from password
    key := sha256.Sum256([]byte(password))

    block, err := aes.NewCipher(key[:])
    if err != nil {
        return nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }

    return gcm.Seal(nonce, nonce, jsonData, nil), nil
}

func importInboxSecurely(encryptedData []byte, password string, client *vaultsandbox.Client, ctx context.Context) (*vaultsandbox.Inbox, error) {
    // Derive key from password
    key := sha256.Sum256([]byte(password))

    block, err := aes.NewCipher(key[:])
    if err != nil {
        return nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    nonceSize := gcm.NonceSize()
    nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]

    jsonData, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil, err
    }

    var data vaultsandbox.ExportedInbox
    if err := json.Unmarshal(jsonData, &data); err != nil {
        return nil, err
    }

    return client.ImportInbox(ctx, &data)
}
```

### 2. Server URL Matching

Imported inboxes must be used with the same server:

```go
// Export from server A
clientA, _ := vaultsandbox.New(
    "key-a",
    vaultsandbox.WithBaseURL("https://server-a.vaultsandbox.com"),
)
inbox, _ := clientA.CreateInbox(ctx)
data := inbox.Export()

// Import must use same server
clientB, _ := vaultsandbox.New(
    "key-b", // Different API key is OK
    vaultsandbox.WithBaseURL("https://server-a.vaultsandbox.com"), // Same server
)

inbox, err := clientB.ImportInbox(ctx, data) // Works

// Wrong server will fail
clientC, _ := vaultsandbox.New(
    "key-c",
    vaultsandbox.WithBaseURL("https://server-c.vaultsandbox.com"), // Different server
)

inbox, err = clientC.ImportInbox(ctx, data) // Returns error - inbox not found
```

### 3. Clean Up Exported Inboxes

Delete inboxes when done to avoid quota issues:

```go
func debugWithImportedInbox(ctx context.Context, client *vaultsandbox.Client, filepath string) error {
    inbox, err := client.ImportInboxFromFile(ctx, filepath)
    if err != nil {
        return err
    }
    defer inbox.Delete(ctx) // Clean up when done

    // Debug...
    emails, err := inbox.GetEmails(ctx)
    if err != nil {
        return err
    }

    fmt.Printf("Found %d emails\n", len(emails))
    return nil
}
```

### 4. Version Exported Data

Include metadata in exports for tracking:

```go
type VersionedExport struct {
    Version     string                       `json:"version"`
    ExportedAt  time.Time                    `json:"exportedAt"`
    ExportedBy  string                       `json:"exportedBy"`
    Environment string                       `json:"environment"`
    Inbox       *vaultsandbox.ExportedInbox  `json:"inbox"`
}

func exportWithMetadata(inbox *vaultsandbox.Inbox) *VersionedExport {
    return &VersionedExport{
        Version:     "1.0",
        ExportedAt:  time.Now(),
        ExportedBy:  os.Getenv("USER"),
        Environment: os.Getenv("GO_ENV"),
        Inbox:       inbox.Export(),
    }
}

func importWithMetadata(ctx context.Context, client *vaultsandbox.Client, data *VersionedExport) (*vaultsandbox.Inbox, error) {
    fmt.Printf("Import from: %s\n", data.ExportedBy)
    fmt.Printf("Exported at: %s\n", data.ExportedAt.Format(time.RFC3339))
    fmt.Printf("Environment: %s\n", data.Environment)

    return client.ImportInbox(ctx, data.Inbox)
}
```

## Next Steps

- [Delivery Strategies](/client-go/advanced/strategies) - SSE vs Polling
- [Error Handling](/client-go/api/errors) - Handle import errors
- [Client API](/client-go/api/client) - Client import/export methods
