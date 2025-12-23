---
title: Managing Inboxes
description: Common operations for creating, using, and deleting inboxes
---

This guide covers common inbox management operations with practical examples.

## Client Configuration

### Basic Client Creation

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
	client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Email address: %s\n", inbox.EmailAddress())
}
```

### Client Options

```go
import (
	"net/http"
	"time"
)

// Custom base URL (for testing or self-hosted)
client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL("https://custom.api.com"))

// Custom HTTP client
httpClient := &http.Client{Timeout: 30 * time.Second}
client, err := vaultsandbox.New(apiKey, vaultsandbox.WithHTTPClient(httpClient))

// Custom timeout for operations
client, err := vaultsandbox.New(apiKey, vaultsandbox.WithTimeout(2*time.Minute))

// Configure retries
client, err := vaultsandbox.New(apiKey,
	vaultsandbox.WithRetries(3),
	vaultsandbox.WithRetryOn([]int{408, 429, 500, 502, 503, 504}),
)

// Delivery strategy for real-time emails
client, err := vaultsandbox.New(apiKey,
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),      // Server-Sent Events
	// or vaultsandbox.StrategyPolling for polling
	// or vaultsandbox.StrategyAuto (default) to try SSE, fall back to polling
)
```

### Validate API Key

```go
// Check if the API key is valid
if err := client.CheckKey(ctx); err != nil {
	log.Fatal("Invalid API key:", err)
}
```

### Get Server Configuration

```go
info := client.ServerInfo()
fmt.Println("Allowed domains:", info.AllowedDomains)
fmt.Println("Max TTL:", info.MaxTTL)
fmt.Println("Default TTL:", info.DefaultTTL)
```

## Creating Inboxes

### Basic Creation

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Printf("Email address: %s\n", inbox.EmailAddress())
```

### With Custom TTL

```go
import "time"

// Expire after 1 hour (good for CI/CD)
inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(time.Hour))

// Expire after 10 minutes (quick tests)
inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(10*time.Minute))

// Expire after 7 days (long-running tests)
inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(7*24*time.Hour))
```

### Requesting Specific Address

```go
import "errors"

inbox, err := client.CreateInbox(ctx,
	vaultsandbox.WithEmailAddress("test@mail.example.com"),
)
if errors.Is(err, vaultsandbox.ErrInboxAlreadyExists) {
	fmt.Println("Address already in use, using random address")
	inbox, err = client.CreateInbox(ctx)
}
if err != nil {
	log.Fatal(err)
}
fmt.Println("Got address:", inbox.EmailAddress())
```

## Listing Emails

### List All Emails

```go
emails, err := inbox.GetEmails(ctx)
if err != nil {
	log.Fatal(err)
}

fmt.Printf("Inbox contains %d emails\n", len(emails))
for _, email := range emails {
	fmt.Printf("- %s: %s\n", email.From, email.Subject)
}
```

### Filtering Emails

```go
emails, err := inbox.GetEmails(ctx)
if err != nil {
	log.Fatal(err)
}

// Filter by sender
var fromSupport []*vaultsandbox.Email
for _, e := range emails {
	if e.From == "support@example.com" {
		fromSupport = append(fromSupport, e)
	}
}

// Filter by subject
var passwordResets []*vaultsandbox.Email
for _, e := range emails {
	if strings.Contains(strings.ToLower(e.Subject), "reset") {
		passwordResets = append(passwordResets, e)
	}
}

// Filter by date
oneHourAgo := time.Now().Add(-time.Hour)
var recentEmails []*vaultsandbox.Email
for _, e := range emails {
	if e.ReceivedAt.After(oneHourAgo) {
		recentEmails = append(recentEmails, e)
	}
}
```

### Sorting Emails

```go
import "sort"

emails, err := inbox.GetEmails(ctx)
if err != nil {
	log.Fatal(err)
}

// Sort by date (newest first)
sort.Slice(emails, func(i, j int) bool {
	return emails[i].ReceivedAt.After(emails[j].ReceivedAt)
})

// Sort by sender
sort.Slice(emails, func(i, j int) bool {
	return emails[i].From < emails[j].From
})
```

## Getting Specific Emails

### By ID

```go
emailID := "email_abc123"
email, err := inbox.GetEmail(ctx, emailID)
if err != nil {
	log.Fatal(err)
}

fmt.Println(email.Subject)
```

### With Error Handling

```go
import "errors"

email, err := inbox.GetEmail(ctx, emailID)
if errors.Is(err, vaultsandbox.ErrEmailNotFound) {
	fmt.Println("Email not found")
} else if err != nil {
	log.Fatal(err)
} else {
	fmt.Println("Found:", email.Subject)
}
```

## Deleting Emails

### Delete Single Email

```go
// By ID via inbox
email, err := inbox.GetEmail(ctx, "email_abc123")
if err != nil {
	log.Fatal(err)
}
if err := inbox.DeleteEmail(ctx, email.ID); err != nil {
	log.Fatal(err)
}
```

### Delete Multiple Emails

```go
emails, err := inbox.GetEmails(ctx)
if err != nil {
	log.Fatal(err)
}

// Delete all emails sequentially
for _, email := range emails {
	if err := inbox.DeleteEmail(ctx, email.ID); err != nil {
		log.Printf("Failed to delete email %s: %v", email.ID, err)
	}
}

// Or delete concurrently with errgroup
import "golang.org/x/sync/errgroup"

g, ctx := errgroup.WithContext(ctx)
for _, email := range emails {
	email := email // capture loop variable
	g.Go(func() error {
		return inbox.DeleteEmail(ctx, email.ID)
	})
}
if err := g.Wait(); err != nil {
	log.Fatal(err)
}
```

### Delete by Criteria

```go
emails, err := inbox.GetEmails(ctx)
if err != nil {
	log.Fatal(err)
}

// Delete old emails (older than 24 hours)
cutoff := time.Now().Add(-24 * time.Hour)
for _, email := range emails {
	if email.ReceivedAt.Before(cutoff) {
		if err := inbox.DeleteEmail(ctx, email.ID); err != nil {
			log.Printf("Failed to delete email %s: %v", email.ID, err)
		}
	}
}
```

## Deleting Inboxes

### Delete Single Inbox

```go
if err := inbox.Delete(ctx); err != nil {
	log.Fatal(err)
}
// Inbox and all emails are now deleted
```

### Delete All Inboxes

```go
// Delete all inboxes for this API key
count, err := client.DeleteAllInboxes(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Printf("Deleted %d inboxes\n", count)
```

### Safe Deletion with Cleanup

```go
func withInbox(ctx context.Context, client *vaultsandbox.Client, fn func(*vaultsandbox.Inbox) error) error {
	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		return err
	}
	defer inbox.Delete(ctx)

	return fn(inbox)
}

// Usage
err := withInbox(ctx, client, func(inbox *vaultsandbox.Inbox) error {
	sendTestEmail(inbox.EmailAddress())
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		return err
	}
	if !strings.Contains(email.Subject, "Test") {
		return fmt.Errorf("unexpected subject: %s", email.Subject)
	}
	return nil
})
```

## Checking Inbox Status

### Check if Inbox Exists

```go
import "errors"

_, err := inbox.GetEmails(ctx)
if errors.Is(err, vaultsandbox.ErrInboxNotFound) {
	fmt.Println("Inbox expired or deleted")
} else if err != nil {
	log.Fatal(err)
} else {
	fmt.Println("Inbox exists")
}
```

### Check Expiration

```go
expiresIn := time.Until(inbox.ExpiresAt())

if expiresIn < 5*time.Minute {
	fmt.Println("Inbox expiring soon!")
	fmt.Printf("Time left: %d minutes\n", int(expiresIn.Minutes()))
}

// Or use the convenience method
if inbox.IsExpired() {
	fmt.Println("Inbox has expired")
}
```

### Get Sync Status

```go
syncStatus, err := inbox.GetSyncStatus(ctx)
if err != nil {
	log.Fatal(err)
}

fmt.Println("Email count:", syncStatus.EmailCount)
fmt.Println("Emails hash:", syncStatus.EmailsHash)
```

### Get Inbox Hash

```go
// InboxHash returns a unique identifier derived from the inbox's public key
hash := inbox.InboxHash()
fmt.Println("Inbox hash:", hash)
```

## Waiting for Emails

### Basic Wait

```go
// Wait for any email with default timeout (60 seconds)
email, err := inbox.WaitForEmail(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Println("Received:", email.Subject)
```

### Wait with Timeout

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(30*time.Second))
if err != nil {
	log.Fatal(err)
}
```

### Wait with Filters

```go
import "regexp"

// Wait for email from specific sender
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithFrom("noreply@example.com"))

// Wait for email with specific subject
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithSubject("Password Reset"))

// Wait for email matching subject regex
pattern := regexp.MustCompile(`(?i)verification|confirm`)
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithSubjectRegex(pattern))

// Wait for email matching sender regex
senderPattern := regexp.MustCompile(`@example\.(com|org)$`)
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithFromRegex(senderPattern))

// Wait with custom predicate
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithPredicate(func(e *vaultsandbox.Email) bool {
	return len(e.Attachments) > 0 // Wait for email with attachments
}))

// Combine multiple filters
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithFrom("support@example.com"),
	vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)ticket`)),
	vaultsandbox.WithWaitTimeout(2*time.Minute),
)
```

### Wait for Multiple Emails

```go
// Wait until inbox has at least 3 emails
emails, err := inbox.WaitForEmailCount(ctx, 3, vaultsandbox.WithWaitTimeout(2*time.Minute))
if err != nil {
	log.Fatal(err)
}
fmt.Printf("Received %d emails\n", len(emails))

// With filters - wait for 2 emails from specific sender
emails, err := inbox.WaitForEmailCount(ctx, 2,
	vaultsandbox.WithFrom("notifications@example.com"),
	vaultsandbox.WithWaitTimeout(time.Minute),
)
```

## Working with Email Content

### Get Raw Email

```go
// Fetch the original raw email content (RFC 5322 format)
rawContent, err := inbox.GetRawEmail(ctx, email.ID)
if err != nil {
	log.Fatal(err)
}
fmt.Println("Raw email:", rawContent)
```

### Mark Email as Read

```go
if err := inbox.MarkEmailAsRead(ctx, email.ID); err != nil {
	log.Fatal(err)
}
fmt.Println("Email marked as read")
```

### Access Email Fields

```go
email, err := inbox.GetEmail(ctx, emailID)
if err != nil {
	log.Fatal(err)
}

fmt.Println("ID:", email.ID)
fmt.Println("From:", email.From)
fmt.Println("To:", email.To)
fmt.Println("Subject:", email.Subject)
fmt.Println("Text body:", email.Text)
fmt.Println("HTML body:", email.HTML)
fmt.Println("Received:", email.ReceivedAt)
fmt.Println("Is read:", email.IsRead)
fmt.Println("Links found:", email.Links)

// Access headers
for key, value := range email.Headers {
	fmt.Printf("Header %s: %s\n", key, value)
}

// Access attachments
for _, att := range email.Attachments {
	fmt.Printf("Attachment: %s (%s, %d bytes)\n", att.Filename, att.ContentType, att.Size)
	// att.Content contains the raw bytes
}

// Access authentication results (SPF, DKIM, DMARC)
if email.AuthResults != nil {
	fmt.Println("SPF:", email.AuthResults.SPF)
	fmt.Println("DKIM:", email.AuthResults.DKIM)
	fmt.Println("DMARC:", email.AuthResults.DMARC)
}
```

## Real-time Email Monitoring

### Subscribe to Single Inbox

```go
// Subscribe to new emails on a single inbox
subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
	fmt.Printf("New email: %s from %s\n", email.Subject, email.From)
})
defer subscription.Unsubscribe()

// Keep running to receive notifications
time.Sleep(10 * time.Minute)
```

### Monitor Multiple Inboxes

```go
// Create a monitor for multiple inboxes
monitor, err := client.MonitorInboxes([]*vaultsandbox.Inbox{inbox1, inbox2, inbox3})
if err != nil {
	log.Fatal(err)
}
defer monitor.Unsubscribe()

// Register callback for new emails
monitor.OnEmail(func(inbox *vaultsandbox.Inbox, email *vaultsandbox.Email) {
	fmt.Printf("New email in %s: %s\n", inbox.EmailAddress(), email.Subject)
})

// Keep running to receive notifications
select {}
```

## Export and Import Inboxes

### Export Inbox

```go
// Export inbox data (includes private key - handle securely!)
exported := inbox.Export()
fmt.Println("Exported at:", exported.ExportedAt)
fmt.Println("Email address:", exported.EmailAddress)
fmt.Println("Expires at:", exported.ExpiresAt)

// Export to file
if err := client.ExportInboxToFile(inbox, "/path/to/inbox.json"); err != nil {
	log.Fatal(err)
}
```

### Import Inbox

```go
// Import from ExportedInbox struct
imported, err := client.ImportInbox(ctx, exported)
if err != nil {
	log.Fatal(err)
}

// Import from file
imported, err := client.ImportInboxFromFile(ctx, "/path/to/inbox.json")
if err != nil {
	log.Fatal(err)
}

// Use imported inbox
emails, err := imported.GetEmails(ctx)
```

## Client Inbox Management

### Get Inbox by Address

```go
// Look up an inbox managed by this client
inbox, exists := client.GetInbox("test@mail.example.com")
if exists {
	fmt.Println("Found inbox:", inbox.EmailAddress())
}
```

### List All Client Inboxes

```go
// Get all inboxes currently managed by this client
inboxes := client.Inboxes()
for _, inbox := range inboxes {
	fmt.Printf("- %s (expires: %s)\n", inbox.EmailAddress(), inbox.ExpiresAt())
}
```

### Delete Inbox by Address

```go
// Delete a specific inbox by email address
if err := client.DeleteInbox(ctx, "test@mail.example.com"); err != nil {
	log.Fatal(err)
}
```

## Bulk Operations

### Create Multiple Inboxes

```go
import "golang.org/x/sync/errgroup"

const numInboxes = 3
inboxes := make([]*vaultsandbox.Inbox, numInboxes)

g, ctx := errgroup.WithContext(ctx)
for i := 0; i < numInboxes; i++ {
	i := i
	g.Go(func() error {
		inbox, err := client.CreateInbox(ctx)
		if err != nil {
			return err
		}
		inboxes[i] = inbox
		return nil
	})
}
if err := g.Wait(); err != nil {
	log.Fatal(err)
}

fmt.Printf("Created %d inboxes\n", len(inboxes))
for _, inbox := range inboxes {
	fmt.Printf("- %s\n", inbox.EmailAddress())
}
```

### Clean Up Multiple Inboxes

```go
// Delete individually
for _, inbox := range inboxes {
	if err := inbox.Delete(ctx); err != nil {
		log.Printf("Failed to delete inbox: %v", err)
	}
}

// Or use convenience method to delete all
count, err := client.DeleteAllInboxes(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Printf("Deleted %d inboxes\n", count)
```

## Testing Patterns

### Test Setup/Teardown

```go
package myapp_test

import (
	"context"
	"os"
	"testing"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
)

var (
	client *vaultsandbox.Client
)

func TestMain(m *testing.M) {
	var err error
	client, err = vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
	if err != nil {
		panic(err)
	}
	defer client.Close()

	os.Exit(m.Run())
}

func TestReceivesEmail(t *testing.T) {
	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	sendEmail(inbox.EmailAddress())

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if email == nil {
		t.Fatal("expected email")
	}
}
```

### Shared Inbox Pattern

```go
package myapp_test

import (
	"context"
	"os"
	"testing"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
)

var (
	client *vaultsandbox.Client
	inbox  *vaultsandbox.Inbox
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	var err error
	client, err = vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
	if err != nil {
		panic(err)
	}

	inbox, err = client.CreateInbox(ctx, vaultsandbox.WithTTL(2*time.Hour))
	if err != nil {
		panic(err)
	}

	code := m.Run()

	inbox.Delete(ctx)
	client.Close()

	os.Exit(code)
}

func TestOne(t *testing.T) {
	// Use shared inbox
}

func TestTwo(t *testing.T) {
	// Use shared inbox
}
```

### Inbox Pool Pattern

```go
package myapp

import (
	"context"
	"errors"
	"sync"

	vaultsandbox "github.com/vaultsandbox/client-go"
)

type InboxPool struct {
	client    *vaultsandbox.Client
	size      int
	available chan *vaultsandbox.Inbox
	inUse     map[*vaultsandbox.Inbox]struct{}
	mu        sync.Mutex
}

func NewInboxPool(client *vaultsandbox.Client, size int) *InboxPool {
	return &InboxPool{
		client:    client,
		size:      size,
		available: make(chan *vaultsandbox.Inbox, size),
		inUse:     make(map[*vaultsandbox.Inbox]struct{}),
	}
}

func (p *InboxPool) Initialize(ctx context.Context) error {
	for i := 0; i < p.size; i++ {
		inbox, err := p.client.CreateInbox(ctx)
		if err != nil {
			return err
		}
		p.available <- inbox
	}
	return nil
}

func (p *InboxPool) Acquire(ctx context.Context) (*vaultsandbox.Inbox, error) {
	select {
	case inbox := <-p.available:
		p.mu.Lock()
		p.inUse[inbox] = struct{}{}
		p.mu.Unlock()
		return inbox, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *InboxPool) Release(inbox *vaultsandbox.Inbox) {
	p.mu.Lock()
	delete(p.inUse, inbox)
	p.mu.Unlock()
	p.available <- inbox
}

func (p *InboxPool) Cleanup(ctx context.Context) error {
	close(p.available)

	var errs []error
	for inbox := range p.available {
		if err := inbox.Delete(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	p.mu.Lock()
	for inbox := range p.inUse {
		if err := inbox.Delete(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	p.mu.Unlock()

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Usage
func Example() {
	ctx := context.Background()
	pool := NewInboxPool(client, 5)
	if err := pool.Initialize(ctx); err != nil {
		panic(err)
	}
	defer pool.Cleanup(ctx)

	inbox, err := pool.Acquire(ctx)
	if err != nil {
		panic(err)
	}
	// Use inbox
	pool.Release(inbox)
}
```

## Error Handling

### Sentinel Errors

The SDK provides sentinel errors for common error conditions:

```go
import "errors"

// Check for specific error types using errors.Is()
if errors.Is(err, vaultsandbox.ErrMissingAPIKey) {
	// API key was not provided
}
if errors.Is(err, vaultsandbox.ErrClientClosed) {
	// Operation attempted on a closed client
}
if errors.Is(err, vaultsandbox.ErrUnauthorized) {
	// Invalid or expired API key
}
if errors.Is(err, vaultsandbox.ErrInboxNotFound) {
	// Inbox does not exist (expired or deleted)
}
if errors.Is(err, vaultsandbox.ErrEmailNotFound) {
	// Email does not exist
}
if errors.Is(err, vaultsandbox.ErrInboxAlreadyExists) {
	// Tried to create/import inbox that already exists
}
if errors.Is(err, vaultsandbox.ErrInvalidImportData) {
	// Import data is malformed or invalid
}
if errors.Is(err, vaultsandbox.ErrDecryptionFailed) {
	// Failed to decrypt email content
}
if errors.Is(err, vaultsandbox.ErrSignatureInvalid) {
	// Email signature verification failed
}
if errors.Is(err, vaultsandbox.ErrRateLimited) {
	// API rate limit exceeded
}
```

### Error Types

```go
import "errors"

// APIError - HTTP errors from the API
var apiErr *vaultsandbox.APIError
if errors.As(err, &apiErr) {
	fmt.Printf("Status: %d\n", apiErr.StatusCode)
	fmt.Printf("Message: %s\n", apiErr.Message)
	fmt.Printf("Request ID: %s\n", apiErr.RequestID)
}

// NetworkError - Network-level failures
var netErr *vaultsandbox.NetworkError
if errors.As(err, &netErr) {
	fmt.Printf("URL: %s\n", netErr.URL)
	fmt.Printf("Attempt: %d\n", netErr.Attempt)
	fmt.Printf("Cause: %v\n", netErr.Err)
}

// SignatureVerificationError - Signature check failed
var sigErr *vaultsandbox.SignatureVerificationError
if errors.As(err, &sigErr) {
	fmt.Printf("Message: %s\n", sigErr.Message)
	if sigErr.IsKeyMismatch {
		fmt.Println("Server key mismatch detected")
	}
}

// Timeouts - use context.DeadlineExceeded
if errors.Is(err, context.DeadlineExceeded) {
	fmt.Println("Operation timed out")
}
```

### Handling Expired Inboxes

```go
import "errors"

emails, err := inbox.GetEmails(ctx)
if errors.Is(err, vaultsandbox.ErrInboxNotFound) {
	fmt.Println("Inbox expired, creating new one")
	inbox, err = client.CreateInbox(ctx)
	if err != nil {
		log.Fatal(err)
	}
} else if err != nil {
	log.Fatal(err)
}
```

### Handling Creation Errors

```go
import "errors"

inbox, err := client.CreateInbox(ctx)
if err != nil {
	var apiErr *vaultsandbox.APIError
	var netErr *vaultsandbox.NetworkError

	switch {
	case errors.As(err, &apiErr):
		fmt.Printf("API error: %d %s\n", apiErr.StatusCode, apiErr.Message)
	case errors.As(err, &netErr):
		fmt.Printf("Network error: %v\n", netErr.Err)
	default:
		log.Fatal(err)
	}
}
```

### Comprehensive Error Handling

```go
func handleError(err error) {
	if err == nil {
		return
	}

	switch {
	case errors.Is(err, vaultsandbox.ErrUnauthorized):
		log.Fatal("Invalid API key - please check your credentials")

	case errors.Is(err, vaultsandbox.ErrRateLimited):
		log.Println("Rate limited - waiting before retry")
		time.Sleep(time.Minute)

	case errors.Is(err, vaultsandbox.ErrInboxNotFound):
		log.Println("Inbox not found - it may have expired")

	case errors.Is(err, vaultsandbox.ErrDecryptionFailed):
		log.Println("Decryption failed - email may be corrupted")

	default:
		var apiErr *vaultsandbox.APIError
		var netErr *vaultsandbox.NetworkError

		switch {
		case errors.As(err, &apiErr):
			log.Printf("API error %d: %s", apiErr.StatusCode, apiErr.Message)
		case errors.As(err, &netErr):
			log.Printf("Network error: %v", netErr.Err)
		default:
			log.Printf("Unexpected error: %v", err)
		}
	}
}
```

## Best Practices

### Always Clean Up

```go
// Good: Cleanup with defer
inbox, err := client.CreateInbox(ctx)
if err != nil {
	log.Fatal(err)
}
defer inbox.Delete(ctx)
// Use inbox

// Bad: No cleanup
inbox, err := client.CreateInbox(ctx)
if err != nil {
	log.Fatal(err)
}
// Use inbox
// Inbox never deleted
```

### Use Appropriate TTL

```go
// Good: Short TTL for CI/CD
inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(time.Hour))

// Bad: Long TTL wastes resources
inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(7*24*time.Hour)) // 7 days for quick test
```

### Handle Cleanup Errors

```go
func safeDelete(ctx context.Context, inbox *vaultsandbox.Inbox) {
	if err := inbox.Delete(ctx); err != nil {
		// Inbox may have already expired
		if !errors.Is(err, vaultsandbox.ErrInboxNotFound) {
			log.Printf("Error deleting inbox: %v", err)
		}
	}
}
```

## Next Steps

- **[Waiting for Emails](/client-go/guides/waiting-for-emails/)** - Learn about email waiting strategies
- **[Real-time Monitoring](/client-go/guides/real-time/)** - Subscribe to new emails
- **[API Reference: Inbox](/client-go/api/inbox/)** - Complete inbox API documentation
- **[Core Concepts: Inboxes](/client-go/concepts/inboxes/)** - Deep dive into inbox concepts
