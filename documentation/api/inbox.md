---
title: Inbox API
description: Complete API reference for the Inbox type and InboxMonitor
---

The `Inbox` type represents a single email inbox in VaultSandbox. It provides methods for managing emails, waiting for new messages, and monitoring in real-time.

## Properties

### EmailAddress

```go
func (i *Inbox) EmailAddress() string
```

Returns the email address for this inbox. Use this address to send test emails.

#### Example

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Send email to: %s\n", inbox.EmailAddress())

// Use in your application
err = sendWelcomeEmail(inbox.EmailAddress())
```

---

### InboxHash

```go
func (i *Inbox) InboxHash() string
```

Returns the unique identifier (SHA-256 hash of the public key) for this inbox. Used internally for API operations.

#### Example

```go
fmt.Printf("Inbox ID: %s\n", inbox.InboxHash())
```

---

### ExpiresAt

```go
func (i *Inbox) ExpiresAt() time.Time
```

Returns the time when this inbox will expire and be automatically deleted.

#### Example

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Inbox expires at: %s\n", inbox.ExpiresAt().Format(time.RFC3339))

timeUntilExpiry := time.Until(inbox.ExpiresAt())
fmt.Printf("Time remaining: %v\n", timeUntilExpiry.Round(time.Second))
```

---

### IsExpired

```go
func (i *Inbox) IsExpired() bool
```

Returns whether the inbox has expired.

#### Example

```go
if inbox.IsExpired() {
    fmt.Println("Inbox has expired")
}
```

## Methods

### GetEmails

Lists all emails in the inbox. Emails are automatically decrypted.

```go
func (i *Inbox) GetEmails(ctx context.Context) ([]*Email, error)
```

#### Returns

`[]*Email` - Slice of decrypted email objects, sorted by received time (newest first)

#### Example

```go
emails, err := inbox.GetEmails(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Inbox has %d emails\n", len(emails))

for _, email := range emails {
    fmt.Printf("- %s from %s\n", email.Subject, email.From)
}
```

---

### GetEmail

Retrieves a specific email by ID.

```go
func (i *Inbox) GetEmail(ctx context.Context, emailID string) (*Email, error)
```

#### Parameters

- `emailID`: The unique identifier for the email

#### Returns

`*Email` - The decrypted email object

#### Example

```go
emails, err := inbox.GetEmails(ctx)
if err != nil {
    log.Fatal(err)
}

firstEmail, err := inbox.GetEmail(ctx, emails[0].ID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Subject: %s\n", firstEmail.Subject)
fmt.Printf("Body: %s\n", firstEmail.Text)
```

#### Errors

- `ErrEmailNotFound` - Email does not exist

---

### WaitForEmail

Waits for an email matching specified criteria. This is the recommended way to handle email arrival in tests.

```go
func (i *Inbox) WaitForEmail(ctx context.Context, opts ...WaitOption) (*Email, error)
```

#### Options

| Option | Description |
| ------ | ----------- |
| `WithWaitTimeout(d time.Duration)` | Maximum time to wait (default: 60s) |
| `WithPollInterval(d time.Duration)` | Polling interval (default: 2s) |
| `WithSubject(s string)` | Filter by exact subject match |
| `WithSubjectRegex(r *regexp.Regexp)` | Filter by subject pattern |
| `WithFrom(s string)` | Filter by exact sender address |
| `WithFromRegex(r *regexp.Regexp)` | Filter by sender pattern |
| `WithPredicate(fn func(*Email) bool)` | Custom filter function |

#### Returns

`*Email` - The first email matching the criteria

#### Examples

```go
// Wait for any email
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
)

// Wait for email with specific subject
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Password Reset`)),
)

// Wait for email from specific sender
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithFrom("noreply@example.com"),
)

// Wait with custom predicate
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(15*time.Second),
    vaultsandbox.WithPredicate(func(e *vaultsandbox.Email) bool {
        for _, to := range e.To {
            if to == "user@example.com" {
                return true
            }
        }
        return false
    }),
)

// Combine multiple filters
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Welcome`)),
    vaultsandbox.WithFromRegex(regexp.MustCompile(`noreply@`)),
    vaultsandbox.WithPredicate(func(e *vaultsandbox.Email) bool {
        return len(e.Links) > 0
    }),
)
```

#### Errors

- `context.DeadlineExceeded` - No matching email received within timeout period

---

### WaitForEmailCount

Waits until the inbox has at least the specified number of emails. More efficient than using arbitrary sleeps when testing multiple emails.

```go
func (i *Inbox) WaitForEmailCount(ctx context.Context, count int, opts ...WaitOption) ([]*Email, error)
```

#### Parameters

- `count`: Minimum number of emails to wait for

#### Options

| Option | Description |
| ------ | ----------- |
| `WithWaitTimeout(d time.Duration)` | Maximum time to wait (default: 60s) |
| `WithPollInterval(d time.Duration)` | Polling interval (default: 2s) |
| `WithSubject(s string)` | Filter by exact subject match |
| `WithSubjectRegex(r *regexp.Regexp)` | Filter by subject pattern |
| `WithFrom(s string)` | Filter by exact sender address |
| `WithFromRegex(r *regexp.Regexp)` | Filter by sender pattern |
| `WithPredicate(fn func(*Email) bool)` | Custom filter function |

#### Returns

`[]*Email` - All matching emails in the inbox once count is reached

#### Example

```go
// Trigger multiple emails
err := sendMultipleNotifications(inbox.EmailAddress(), 3)
if err != nil {
    log.Fatal(err)
}

// Wait for all 3 to arrive
emails, err := inbox.WaitForEmailCount(ctx, 3,
    vaultsandbox.WithWaitTimeout(30*time.Second),
)
if err != nil {
    log.Fatal(err)
}

// Now process all emails
if len(emails) != 3 {
    log.Fatalf("expected 3 emails, got %d", len(emails))
}
```

#### Errors

- `context.DeadlineExceeded` - Required count not reached within timeout

---

### OnNewEmail

Subscribes to new emails in real-time. Receives a callback for each new email that arrives.

```go
func (i *Inbox) OnNewEmail(callback InboxEmailCallback) Subscription
```

#### Parameters

- `callback`: Function called when a new email arrives

```go
type InboxEmailCallback func(email *Email)
```

#### Returns

`Subscription` - Subscription interface with `Unsubscribe()` method

```go
type Subscription interface {
    Unsubscribe()
}
```

#### Example

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Monitoring: %s\n", inbox.EmailAddress())

// Subscribe to new emails
subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
    fmt.Printf("New email: %q\n", email.Subject)
    fmt.Printf("From: %s\n", email.From)

    // Process email...
})

// Later, stop monitoring
subscription.Unsubscribe()
```

#### Best Practice

Always unsubscribe when done to avoid goroutine leaks:

```go
var subscription vaultsandbox.Subscription

func TestEmailFlow(t *testing.T) {
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }

    subscription = inbox.OnNewEmail(func(email *vaultsandbox.Email) {
        // Handle email
    })

    t.Cleanup(func() {
        if subscription != nil {
            subscription.Unsubscribe()
        }
        inbox.Delete(ctx)
    })

    // Test logic...
}
```

---

### GetSyncStatus

Gets the current synchronization status of the inbox with the server.

```go
func (i *Inbox) GetSyncStatus(ctx context.Context) (*SyncStatus, error)
```

#### Returns

`*SyncStatus` - Sync status information

```go
type SyncStatus struct {
    EmailCount int    // Number of emails in the inbox
    EmailsHash string // Hash of the email list for change detection
}
```

#### Example

```go
status, err := inbox.GetSyncStatus(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Email count: %d\n", status.EmailCount)
fmt.Printf("Emails hash: %s\n", status.EmailsHash)
```

---

### Delete

Deletes this inbox and all its emails.

```go
func (i *Inbox) Delete(ctx context.Context) error
```

#### Example

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
    log.Fatal(err)
}

// Use inbox...

// Clean up
err = inbox.Delete(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Println("Inbox deleted")
```

#### Best Practice

Always delete inboxes after tests using `t.Cleanup`:

```go
func TestEmailFlow(t *testing.T) {
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }

    t.Cleanup(func() {
        inbox.Delete(context.Background())
    })

    // Test logic...
}
```

---

### Export

Exports inbox data and encryption keys for backup or sharing.

```go
func (i *Inbox) Export() *ExportedInbox
```

#### Returns

`*ExportedInbox` - Serializable inbox data including sensitive keys

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

#### Validate

Validates that the exported data is valid before import.

```go
func (e *ExportedInbox) Validate() error
```

Returns `ErrInvalidImportData` if the email address is empty or the secret key is missing/invalid.

#### Example

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
    log.Fatal(err)
}
data := inbox.Export()

// Save for later
jsonData, err := json.MarshalIndent(data, "", "  ")
if err != nil {
    log.Fatal(err)
}
err = os.WriteFile("inbox-backup.json", jsonData, 0600)
if err != nil {
    log.Fatal(err)
}
```

#### Security Warning

Exported data contains private encryption keys. Store securely with restrictive file permissions (0600)!

## InboxMonitor

The `InboxMonitor` type allows you to monitor multiple inboxes simultaneously.

### Creating a Monitor

```go
inbox1, _ := client.CreateInbox(ctx)
inbox2, _ := client.CreateInbox(ctx)

monitor, err := client.MonitorInboxes([]*vaultsandbox.Inbox{inbox1, inbox2})
if err != nil {
    log.Fatal(err)
}
```

### Events

#### OnEmail

Registers a callback for new emails across all monitored inboxes.

```go
func (m *InboxMonitor) OnEmail(callback EmailCallback) Subscription
```

##### Parameters

- `callback`: Function called when a new email arrives

```go
type EmailCallback func(inbox *Inbox, email *Email)
```

##### Example

```go
subscription := monitor.OnEmail(func(inbox *vaultsandbox.Inbox, email *vaultsandbox.Email) {
    fmt.Printf("Email received in %s\n", inbox.EmailAddress())
    fmt.Printf("Subject: %s\n", email.Subject)
})
```

### Methods

#### Unsubscribe

Stops monitoring all inboxes and cleans up resources.

```go
func (m *InboxMonitor) Unsubscribe()
```

##### Example

```go
monitor, err := client.MonitorInboxes([]*vaultsandbox.Inbox{inbox1, inbox2})
if err != nil {
    log.Fatal(err)
}

// Use monitor...

// Stop monitoring
monitor.Unsubscribe()
```

### Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/vaultsandbox/client-go"
)

func monitorMultipleInboxes() error {
    ctx := context.Background()

    client, err := vaultsandbox.New(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
    )
    if err != nil {
        return err
    }
    defer client.Close()

    // Create multiple inboxes
    inbox1, err := client.CreateInbox(ctx)
    if err != nil {
        return err
    }
    inbox2, err := client.CreateInbox(ctx)
    if err != nil {
        return err
    }

    fmt.Printf("Inbox 1: %s\n", inbox1.EmailAddress())
    fmt.Printf("Inbox 2: %s\n", inbox2.EmailAddress())

    // Monitor both inboxes
    monitor, err := client.MonitorInboxes([]*vaultsandbox.Inbox{inbox1, inbox2})
    if err != nil {
        return err
    }

    monitor.OnEmail(func(inbox *vaultsandbox.Inbox, email *vaultsandbox.Email) {
        fmt.Printf("\nNew email in %s:\n", inbox.EmailAddress())
        fmt.Printf("  Subject: %s\n", email.Subject)
        fmt.Printf("  From: %s\n", email.From)
    })

    // Wait for emails to arrive...
    time.Sleep(60 * time.Second)

    // Clean up
    monitor.Unsubscribe()
    inbox1.Delete(ctx)
    inbox2.Delete(ctx)

    return nil
}

func main() {
    if err := monitorMultipleInboxes(); err != nil {
        log.Fatal(err)
    }
}
```

## Complete Inbox Example

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "regexp"
    "time"

    "github.com/vaultsandbox/client-go"
)

func completeInboxExample() error {
    ctx := context.Background()

    client, err := vaultsandbox.New(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
    )
    if err != nil {
        return err
    }
    defer client.Close()

    // Create inbox
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        return err
    }
    fmt.Printf("Created: %s\n", inbox.EmailAddress())
    fmt.Printf("Expires: %s\n", inbox.ExpiresAt().Format(time.RFC3339))

    // Subscribe to new emails
    subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
        fmt.Printf("Received: %s\n", email.Subject)
    })

    // Trigger test email
    err = sendTestEmail(inbox.EmailAddress())
    if err != nil {
        return err
    }

    // Wait for specific email
    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(10*time.Second),
        vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Test`)),
    )
    if err != nil {
        return err
    }

    fmt.Printf("Found email: %s\n", email.Subject)
    fmt.Printf("Body: %s\n", email.Text)

    // Mark as read
    err = email.MarkAsRead(ctx)
    if err != nil {
        return err
    }

    // Get all emails
    allEmails, err := inbox.GetEmails(ctx)
    if err != nil {
        return err
    }
    fmt.Printf("Total emails: %d\n", len(allEmails))

    // Export inbox
    exportData := inbox.Export()
    jsonData, err := json.Marshal(exportData)
    if err != nil {
        return err
    }
    err = os.WriteFile("inbox.json", jsonData, 0600)
    if err != nil {
        return err
    }

    // Clean up
    subscription.Unsubscribe()
    err = inbox.Delete(ctx)
    if err != nil {
        return err
    }

    return nil
}

func main() {
    if err := completeInboxExample(); err != nil {
        log.Fatal(err)
    }
}
```

## Next Steps

- [Email API Reference](/client-go/api/email) - Work with email objects
- [Client API Reference](/client-go/api/client) - Learn about client methods
- [Waiting for Emails Guide](/client-go/guides/waiting-for-emails) - Best practices
- [Real-time Monitoring Guide](/client-go/guides/real-time) - Advanced monitoring patterns
