---
title: Managing Inboxes
description: Common operations for creating, using, and deleting inboxes
---

This guide covers common inbox management operations with practical examples.

## Creating Inboxes

### Basic Creation

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
if err := email.Delete(ctx); err != nil {
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
	if err := email.Delete(ctx); err != nil {
		log.Printf("Failed to delete email %s: %v", email.ID, err)
	}
}

// Or delete concurrently with errgroup
import "golang.org/x/sync/errgroup"

g, ctx := errgroup.WithContext(ctx)
for _, email := range emails {
	email := email // capture loop variable
	g.Go(func() error {
		return email.Delete(ctx)
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
		if err := email.Delete(ctx); err != nil {
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
