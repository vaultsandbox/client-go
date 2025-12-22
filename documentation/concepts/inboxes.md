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

### Deleting Emails

```go
// Delete via inbox
err := inbox.client.apiClient.DeleteEmail(ctx, inbox.InboxHash(), "email-id-123")

// Or via email object
err := email.Delete(ctx)
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

### Deleting All Inboxes

```go
count, err := client.DeleteAllInboxes(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Printf("Deleted %d inboxes\n", count)
```

## Next Steps

- **[Email Objects](/client-go/concepts/emails/)** - Learn about email structure
- **[Managing Inboxes](/client-go/guides/managing-inboxes/)** - Common inbox operations
- **[Import/Export](/client-go/advanced/import-export/)** - Advanced inbox persistence
- **[API Reference: Inbox](/client-go/api/inbox/)** - Complete API documentation
