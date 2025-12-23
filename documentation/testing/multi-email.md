---
title: Testing Multi-Email Scenarios
description: Learn how to test scenarios involving multiple emails with VaultSandbox
---

Many real-world email flows involve sending multiple emails in sequence or in parallel. VaultSandbox provides efficient methods for testing these scenarios without using arbitrary timeouts.

## Waiting for Multiple Emails

The `WaitForEmailCount()` method is the recommended way to test scenarios that send multiple emails. It's more efficient and reliable than using arbitrary timeouts.

### Basic Example

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/vaultsandbox/client-go"
)

func main() {
	ctx := context.Background()

	client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
		vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer inbox.Delete(ctx)

	// Send multiple emails
	if err := sendNotifications(inbox.EmailAddress(), 3); err != nil {
		log.Fatal(err)
	}

	// Wait for all 3 emails to arrive (polls every 2s by default)
	emails, err := inbox.WaitForEmailCount(ctx, 3,
		vaultsandbox.WithWaitTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Verify all emails arrived
	if len(emails) != 3 {
		log.Fatalf("expected 3 emails, got %d", len(emails))
	}
}
```

## Testing Email Sequences

Test workflows that send emails in a specific sequence:

```go
package myapp_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go"
)

func TestWelcomeEmailSequence(t *testing.T) {
	ctx := context.Background()

	client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(baseURL))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("failed to create inbox: %v", err)
	}
	defer inbox.Delete(ctx)

	t.Run("should send complete onboarding sequence", func(t *testing.T) {
		// Trigger user registration
		if err := registerUser(inbox.EmailAddress()); err != nil {
			t.Fatalf("failed to register user: %v", err)
		}

		// Wait for all 4 onboarding emails
		emails, err := inbox.WaitForEmailCount(ctx, 4,
			vaultsandbox.WithWaitTimeout(30*time.Second),
		)
		if err != nil {
			t.Fatalf("failed to receive emails: %v", err)
		}

		// Verify sequence order and content
		expectedSubjects := []string{"Welcome", "Getting Started", "Tips and Tricks", "We're Here to Help"}
		for i, expected := range expectedSubjects {
			if !strings.Contains(emails[i].Subject, expected) {
				t.Errorf("email %d: expected subject containing %q, got %q", i, expected, emails[i].Subject)
			}
		}

		// Verify timing between emails
		timeDiff := emails[1].ReceivedAt.Sub(emails[0].ReceivedAt)

		// Emails should be spaced at least 1 second apart
		if timeDiff < time.Second {
			t.Errorf("expected at least 1s between emails, got %v", timeDiff)
		}
	})
}
```

## Testing Batch Notifications

Test scenarios where multiple similar emails are sent at once:

```go
func TestBatchOrderConfirmations(t *testing.T) {
	ctx := context.Background()

	client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(baseURL))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("failed to create inbox: %v", err)
	}
	defer inbox.Delete(ctx)

	orderIDs := []string{"ORD-001", "ORD-002", "ORD-003"}

	// Place multiple orders
	for _, orderID := range orderIDs {
		if err := placeOrder(inbox.EmailAddress(), orderID); err != nil {
			t.Fatalf("failed to place order %s: %v", orderID, err)
		}
	}

	// Wait for all confirmations
	emails, err := inbox.WaitForEmailCount(ctx, 3,
		vaultsandbox.WithWaitTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to receive confirmations: %v", err)
	}

	// Verify each order has a confirmation
	for _, orderID := range orderIDs {
		found := false
		for _, email := range emails {
			if strings.Contains(email.Text, orderID) {
				if !strings.Contains(email.Subject, "Order Confirmation") {
					t.Errorf("order %s email missing 'Order Confirmation' in subject", orderID)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no confirmation email found for order %s", orderID)
		}
	}
}
```

## Testing Email Timing

Validate that emails arrive within expected time windows:

```go
func TestEmailIntervals(t *testing.T) {
	ctx := context.Background()

	client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(baseURL))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("failed to create inbox: %v", err)
	}
	defer inbox.Delete(ctx)

	startTime := time.Now()

	// Trigger time-based email sequence
	if err := startTrialPeriod(inbox.EmailAddress()); err != nil {
		t.Fatalf("failed to start trial: %v", err)
	}

	// Wait for initial email immediately
	welcome, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Welcome to your trial`)),
	)
	if err != nil {
		t.Fatalf("failed to receive welcome email: %v", err)
	}

	if elapsed := time.Since(startTime); elapsed > 5*time.Second {
		t.Errorf("welcome email took too long: %v", elapsed)
	}

	// Wait for reminder email (should come after delay)
	emails, err := inbox.WaitForEmailCount(ctx, 2,
		vaultsandbox.WithWaitTimeout(60*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to receive reminder: %v", err)
	}

	// Find the reminder email
	var reminder *vaultsandbox.Email
	for _, email := range emails {
		if strings.Contains(email.Subject, "Trial Reminder") {
			reminder = email
			break
		}
	}

	if reminder == nil {
		t.Fatal("reminder email not found")
	}

	timeBetween := reminder.ReceivedAt.Sub(welcome.ReceivedAt)

	// Reminder should come at least 30 seconds after welcome
	if timeBetween < 30*time.Second {
		t.Errorf("reminder came too soon: %v after welcome", timeBetween)
	}
}
```

## Processing Emails as They Arrive

For scenarios where you need to process emails immediately as they arrive, use `Watch()`. This returns a channel that receives emails as they arrive:

```go
func TestRealTimeNotifications(t *testing.T) {
	ctx := context.Background()

	client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(baseURL))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("failed to create inbox: %v", err)
	}
	defer inbox.Delete(ctx)

	// Trigger multiple notifications
	if err := sendMultipleNotifications(inbox.EmailAddress(), 5); err != nil {
		t.Fatalf("failed to send notifications: %v", err)
	}

	// Create a context with timeout for watching
	watchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Watch for emails as they arrive
	var receivedSubjects []string
	for email := range inbox.Watch(watchCtx) {
		t.Logf("Received: %s", email.Subject)
		receivedSubjects = append(receivedSubjects, email.Subject)

		// Stop after receiving 5 emails
		if len(receivedSubjects) >= 5 {
			cancel()
		}
	}

	// Verify all were processed
	if len(receivedSubjects) != 5 {
		t.Errorf("expected 5 processed emails, got %d", len(receivedSubjects))
	}

	for _, subject := range receivedSubjects {
		if !strings.Contains(subject, "Notification") {
			t.Errorf("unexpected subject: %s", subject)
		}
	}
}
```

## Testing Parallel Email Flows

Test scenarios where different email types are triggered simultaneously:

```go
func TestConcurrentEmailTypes(t *testing.T) {
	ctx := context.Background()

	client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(baseURL))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("failed to create inbox: %v", err)
	}
	defer inbox.Delete(ctx)

	// Trigger different email flows simultaneously
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		sendWelcomeEmail(inbox.EmailAddress())
	}()
	go func() {
		defer wg.Done()
		sendOrderConfirmation(inbox.EmailAddress(), "ORD-123")
	}()
	go func() {
		defer wg.Done()
		sendNewsletterSubscription(inbox.EmailAddress())
	}()

	wg.Wait()

	// Wait for all 3 emails
	emails, err := inbox.WaitForEmailCount(ctx, 3,
		vaultsandbox.WithWaitTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to receive emails: %v", err)
	}

	// Verify all email types arrived
	var welcome, order, newsletter *vaultsandbox.Email
	for _, email := range emails {
		switch {
		case strings.Contains(email.Subject, "Welcome"):
			welcome = email
		case strings.Contains(email.Subject, "Order"):
			order = email
		case strings.Contains(email.Subject, "Newsletter"):
			newsletter = email
		}
	}

	if welcome == nil {
		t.Error("welcome email not received")
	}
	if order == nil {
		t.Error("order email not received")
	}
	if newsletter == nil {
		t.Error("newsletter email not received")
	}
}
```

## Filtering and Validating Multiple Emails

Use Go's standard library to validate email collections:

```go
func TestBulkNotificationValidation(t *testing.T) {
	ctx := context.Background()

	client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(baseURL))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("failed to create inbox: %v", err)
	}
	defer inbox.Delete(ctx)

	if err := sendBulkNotifications(inbox.EmailAddress(), 10); err != nil {
		t.Fatalf("failed to send notifications: %v", err)
	}

	emails, err := inbox.WaitForEmailCount(ctx, 10,
		vaultsandbox.WithWaitTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to receive emails: %v", err)
	}

	// All should be from the same sender
	senders := make(map[string]struct{})
	for _, email := range emails {
		senders[email.From] = struct{}{}
	}
	if len(senders) != 1 {
		t.Errorf("expected 1 unique sender, got %d", len(senders))
	}
	for sender := range senders {
		if sender != "notifications@example.com" {
			t.Errorf("unexpected sender: %s", sender)
		}
	}

	// All should have valid authentication
	for i, email := range emails {
		if email.AuthResults == nil {
			t.Errorf("email %d missing auth results", i)
		}
	}

	// All should have links
	emailsWithLinks := 0
	for _, email := range emails {
		if len(email.Links) > 0 {
			emailsWithLinks++
		}
	}
	if emailsWithLinks != 10 {
		t.Errorf("expected 10 emails with links, got %d", emailsWithLinks)
	}

	// Check that all emails are unique
	subjects := make(map[string]struct{})
	for _, email := range emails {
		subjects[email.Subject] = struct{}{}
	}
	if len(subjects) != 10 {
		t.Errorf("expected 10 unique subjects, got %d", len(subjects))
	}
}
```

## Testing with Multiple Inboxes

Test scenarios involving multiple recipients:

```go
func TestMultipleRecipients(t *testing.T) {
	ctx := context.Background()

	client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(baseURL))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	// Create multiple inboxes
	inbox1, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("failed to create inbox1: %v", err)
	}
	defer inbox1.Delete(ctx)

	inbox2, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("failed to create inbox2: %v", err)
	}
	defer inbox2.Delete(ctx)

	inbox3, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("failed to create inbox3: %v", err)
	}
	defer inbox3.Delete(ctx)

	// Send announcement to all
	addresses := []string{
		inbox1.EmailAddress(),
		inbox2.EmailAddress(),
		inbox3.EmailAddress(),
	}
	if err := sendAnnouncement(addresses); err != nil {
		t.Fatalf("failed to send announcement: %v", err)
	}

	// Wait for emails in all inboxes concurrently
	announcementRegex := regexp.MustCompile(`Announcement`)

	var wg sync.WaitGroup
	errors := make(chan error, 3)

	for _, inbox := range []*vaultsandbox.Inbox{inbox1, inbox2, inbox3} {
		wg.Add(1)
		go func(ib *vaultsandbox.Inbox) {
			defer wg.Done()
			_, err := ib.WaitForEmail(ctx,
				vaultsandbox.WithWaitTimeout(10*time.Second),
				vaultsandbox.WithSubjectRegex(announcementRegex),
			)
			if err != nil {
				errors <- fmt.Errorf("inbox %s: %w", ib.EmailAddress(), err)
			}
		}(inbox)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}

	// Verify all received the same content
	emails1, _ := inbox1.GetEmails(ctx)
	emails2, _ := inbox2.GetEmails(ctx)
	emails3, _ := inbox3.GetEmails(ctx)

	if emails1[0].Subject != emails2[0].Subject || emails2[0].Subject != emails3[0].Subject {
		t.Error("subjects do not match across inboxes")
	}
	if emails1[0].Text != emails2[0].Text {
		t.Error("content does not match across inboxes")
	}
}
```

## Monitoring Multiple Inboxes

Use `WatchInboxes()` to watch multiple inboxes simultaneously. This returns a channel of `InboxEvent` structs containing both the inbox and email:

```go
func TestMultipleInboxMonitor(t *testing.T) {
	ctx := context.Background()

	client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(baseURL))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	inbox1, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("failed to create inbox1: %v", err)
	}
	defer inbox1.Delete(ctx)

	inbox2, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("failed to create inbox2: %v", err)
	}
	defer inbox2.Delete(ctx)

	// Send emails to both inboxes
	if err := sendEmail(inbox1.EmailAddress(), "Test 1"); err != nil {
		t.Fatalf("failed to send to inbox1: %v", err)
	}
	if err := sendEmail(inbox2.EmailAddress(), "Test 2"); err != nil {
		t.Fatalf("failed to send to inbox2: %v", err)
	}

	// Create a context with timeout for watching
	watchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Watch both inboxes simultaneously
	events := client.WatchInboxes(watchCtx, inbox1, inbox2)

	type receivedEmail struct {
		inboxAddress string
		subject      string
	}
	var receivedEmails []receivedEmail

	for event := range events {
		receivedEmails = append(receivedEmails, receivedEmail{
			inboxAddress: event.Inbox.EmailAddress(),
			subject:      event.Email.Subject,
		})

		// Stop after receiving 2 emails
		if len(receivedEmails) >= 2 {
			cancel()
		}
	}

	if len(receivedEmails) != 2 {
		t.Errorf("expected 2 received emails, got %d", len(receivedEmails))
	}

	// Verify we received emails from both inboxes
	inboxAddresses := make(map[string]bool)
	for _, email := range receivedEmails {
		inboxAddresses[email.inboxAddress] = true
	}
	if !inboxAddresses[inbox1.EmailAddress()] {
		t.Error("did not receive email from inbox1")
	}
	if !inboxAddresses[inbox2.EmailAddress()] {
		t.Error("did not receive email from inbox2")
	}
}
```

## Best Practices

### Use WaitForEmailCount() for Known Quantities

When you know exactly how many emails to expect, always use `WaitForEmailCount()`:

```go
// Good: Efficient and reliable
emails, err := inbox.WaitForEmailCount(ctx, 3,
	vaultsandbox.WithWaitTimeout(30*time.Second),
)

// Avoid: Arbitrary timeout with polling
time.Sleep(10 * time.Second)
emails, err := inbox.GetEmails(ctx)
```

### Set Appropriate Timeouts

Calculate timeouts based on expected email count and delivery speed:

```go
// For fast local testing
emails, err := inbox.WaitForEmailCount(ctx, 5,
	vaultsandbox.WithWaitTimeout(10*time.Second), // 2s per email
)

// For CI/CD or production gateways
emails, err := inbox.WaitForEmailCount(ctx, 5,
	vaultsandbox.WithWaitTimeout(30*time.Second), // 6s per email
)

// For very large batches
emails, err := inbox.WaitForEmailCount(ctx, 100,
	vaultsandbox.WithWaitTimeout(2*time.Minute), // 1.2s per email
)
```

### Verify Email Ordering When Important

If order matters, explicitly check timestamps:

```go
emails, err := inbox.GetEmails(ctx)
if err != nil {
	t.Fatal(err)
}

// Sort by received time
sort.Slice(emails, func(i, j int) bool {
	return emails[i].ReceivedAt.Before(emails[j].ReceivedAt)
})

// Verify first email came before second
if !emails[0].ReceivedAt.Before(emails[1].ReceivedAt) {
	t.Error("emails not in expected order")
}
```

### Clean Up Multiple Inboxes

Use goroutines to clean up multiple inboxes efficiently:

```go
func cleanupInboxes(ctx context.Context, inboxes []*vaultsandbox.Inbox) {
	var wg sync.WaitGroup
	for _, inbox := range inboxes {
		wg.Add(1)
		go func(ib *vaultsandbox.Inbox) {
			defer wg.Done()
			ib.Delete(ctx)
		}(inbox)
	}
	wg.Wait()
}

// In test cleanup
defer cleanupInboxes(ctx, []*vaultsandbox.Inbox{inbox1, inbox2, inbox3})
```

### Use Table-Driven Tests

Make it clear what email scenario you're testing:

```go
func TestOrderConfirmations(t *testing.T) {
	testCases := []struct {
		name       string
		orderCount int
		timeout    time.Duration
	}{
		{"single order", 1, 10 * time.Second},
		{"batch of 3 orders", 3, 30 * time.Second},
		{"large batch of 10 orders", 10, 60 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test implementation
		})
	}
}
```

## Performance Considerations

### Polling Interval

Adjust the polling interval based on expected email volume:

```go
// Default: 2 second polling
emails, err := inbox.WaitForEmailCount(ctx, 10,
	vaultsandbox.WithWaitTimeout(30*time.Second),
)

// For time-sensitive tests
emails, err := inbox.WaitForEmailCount(ctx, 10,
	vaultsandbox.WithWaitTimeout(30*time.Second),
)

// For large batches
emails, err := inbox.WaitForEmailCount(ctx, 100,
	vaultsandbox.WithWaitTimeout(2*time.Minute),
)
```

### Batch Operations

Fetch all emails once rather than making multiple API calls:

```go
// Good: Single API call
emails, err := inbox.GetEmails(ctx)
if err != nil {
	t.Fatal(err)
}

var welcome, confirmation *vaultsandbox.Email
for _, email := range emails {
	if strings.Contains(email.Subject, "Welcome") {
		welcome = email
	}
	if strings.Contains(email.Subject, "Confirmation") {
		confirmation = email
	}
}

// Avoid: Multiple API calls
email1, _ := inbox.GetEmail(ctx, id1)
email2, _ := inbox.GetEmail(ctx, id2)
email3, _ := inbox.GetEmail(ctx, id3)
```

### Use Context for Cancellation

Leverage Go's context for proper timeout and cancellation handling:

```go
func TestWithTimeout(t *testing.T) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := vaultsandbox.New(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(context.Background()) // Use background for cleanup

	// All operations respect the parent context timeout
	emails, err := inbox.WaitForEmailCount(ctx, 5,
		vaultsandbox.WithWaitTimeout(30*time.Second),
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			t.Fatal("test timed out waiting for emails")
		}
		t.Fatal(err)
	}
}
```

## Next Steps

- [CI/CD Integration](/client-go/testing/cicd) - Run multi-email tests in CI
- [Real-time Monitoring](/client-go/guides/real-time) - Process emails as they arrive
- [Managing Inboxes](/client-go/guides/managing-inboxes) - Learn more about inbox operations
