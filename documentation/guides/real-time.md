---
title: Real-time Monitoring
description: Watch for emails as they arrive using channels
---

VaultSandbox supports real-time email notifications via Server-Sent Events (SSE), enabling instant processing of emails as they arrive. The SDK provides a channel-based API for idiomatic Go patterns.

## Basic Watching

### Watch Single Inbox

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

inbox, err := client.CreateInbox(ctx)
if err != nil {
	log.Fatal(err)
}
defer inbox.Delete(ctx)

fmt.Printf("Watching: %s\n", inbox.EmailAddress())

for email := range inbox.Watch(ctx) {
	fmt.Printf("New email: %s\n", email.Subject)
	fmt.Printf("   From: %s\n", email.From)
	fmt.Printf("   Received: %s\n", email.ReceivedAt)
}
```

### Watch with Processing

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

for email := range inbox.Watch(ctx) {
	fmt.Println("Processing:", email.Subject)

	// Extract links
	if len(email.Links) > 0 {
		fmt.Println("Links found:", email.Links)
	}

	// Check authentication
	auth := email.AuthResults.Validate()
	if !auth.Passed {
		fmt.Println("Authentication failed:", auth.Failures)
	}

	// Mark as processed
	if err := inbox.MarkEmailAsRead(ctx, email.ID); err != nil {
		log.Println("Failed to mark as read:", err)
	}
}
```

## Watching Multiple Inboxes

### Using WatchInboxes

```go
inbox1, _ := client.CreateInbox(ctx)
inbox2, _ := client.CreateInbox(ctx)
inbox3, _ := client.CreateInbox(ctx)

watchCtx, cancel := context.WithCancel(context.Background())
defer cancel()

for event := range client.WatchInboxes(watchCtx, inbox1, inbox2, inbox3) {
	fmt.Printf("Email in %s\n", event.Inbox.EmailAddress())
	fmt.Printf("   Subject: %s\n", event.Email.Subject)
	fmt.Printf("   From: %s\n", event.Email.From)
}
```

### Watching with Handlers

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

for event := range client.WatchInboxes(ctx, inbox1, inbox2) {
	switch {
	case event.Email.From == "alerts@example.com":
		handleAlert(event.Email)
	case strings.Contains(event.Email.Subject, "Invoice"):
		handleInvoice(event.Inbox, event.Email)
	default:
		fmt.Println("Other email:", event.Email.Subject)
	}
}
```

## Stopping Early

### Context Cancellation

Use context cancellation to stop watching at any time:

```go
ctx, cancel := context.WithCancel(context.Background())

go func() {
	for email := range inbox.Watch(ctx) {
		fmt.Println("Email:", email.Subject)

		// Stop after finding welcome email
		if strings.Contains(email.Subject, "Welcome") {
			cancel()
			return
		}
	}
}()
```

### With Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

for email := range inbox.Watch(ctx) {
	processEmail(email)
}
// Channel closes automatically when timeout expires
```

## Select-based Processing

For more control over receiving emails, use select:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

emails := inbox.Watch(ctx)
for {
	select {
	case email, ok := <-emails:
		if !ok {
			fmt.Println("Channel closed")
			return
		}
		processEmail(email)
	case <-ctx.Done():
		fmt.Println("Context cancelled")
		return
	}
}
```

### Select with Multiple Channels

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

emails := inbox.Watch(ctx)
done := make(chan struct{})

for {
	select {
	case email, ok := <-emails:
		if !ok {
			return
		}
		if processAndCheck(email) {
			close(done)
			cancel()
			return
		}
	case <-done:
		return
	case <-time.After(5 * time.Second):
		fmt.Println("No email in last 5 seconds...")
	}
}
```

## Real-World Patterns

### Wait for Specific Email

```go
func waitForSpecificEmail(
	ctx context.Context,
	inbox *vaultsandbox.Inbox,
	predicate func(*vaultsandbox.Email) bool,
	timeout time.Duration,
) (*vaultsandbox.Email, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for email := range inbox.Watch(ctx) {
		if predicate(email) {
			return email, nil
		}
	}

	return nil, fmt.Errorf("timeout waiting for email")
}

// Usage
email, err := waitForSpecificEmail(ctx, inbox, func(e *vaultsandbox.Email) bool {
	return strings.Contains(e.Subject, "Password Reset")
}, 10*time.Second)
```

### Collect Multiple Emails

```go
func collectEmails(
	ctx context.Context,
	inbox *vaultsandbox.Inbox,
	count int,
	timeout time.Duration,
) ([]*vaultsandbox.Email, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	emails := make([]*vaultsandbox.Email, 0, count)

	for email := range inbox.Watch(ctx) {
		emails = append(emails, email)
		fmt.Printf("Received %d/%d\n", len(emails), count)

		if len(emails) >= count {
			return emails, nil
		}
	}

	return nil, fmt.Errorf("timeout: only received %d/%d", len(emails), count)
}

// Usage
emails, err := collectEmails(ctx, inbox, 3, 20*time.Second)
```

### Process Email Pipeline

```go
func processEmailPipeline(ctx context.Context, inbox *vaultsandbox.Inbox) {
	for email := range inbox.Watch(ctx) {
		fmt.Println("Processing:", email.Subject)

		// Step 1: Validate
		auth := email.AuthResults.Validate()
		if !auth.Passed {
			fmt.Println("Failed auth:", auth.Failures)
			continue
		}

		// Step 2: Extract data
		links := email.Links
		attachments := email.Attachments

		// Step 3: Store/process
		if err := storeEmail(ctx, email); err != nil {
			fmt.Println("Error storing:", err)
			continue
		}

		// Step 4: Notify
		if err := notifyProcessed(ctx, email.ID); err != nil {
			fmt.Println("Error notifying:", err)
			continue
		}

		// Step 5: Cleanup
		if err := inbox.DeleteEmail(ctx, email.ID); err != nil {
			fmt.Println("Error deleting:", err)
			continue
		}

		fmt.Println("Processed:", email.Subject)
		_ = links       // use as needed
		_ = attachments // use as needed
	}
}

// Usage
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go processEmailPipeline(ctx, inbox)
```

## Testing with Real-Time Watching

### Integration Test

```go
func TestRealTimeEmailProcessing(t *testing.T) {
	client, _ := vaultsandbox.New(apiKey)
	defer client.Close()

	ctx := context.Background()
	inbox, _ := client.CreateInbox(ctx)
	defer inbox.Delete(ctx)

	watchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var received []*vaultsandbox.Email
	done := make(chan struct{})

	go func() {
		for email := range inbox.Watch(watchCtx) {
			received = append(received, email)
			if len(received) >= 2 {
				cancel()
				close(done)
				return
			}
		}
	}()

	// Send test emails
	sendEmail(inbox.EmailAddress(), "Test 1")
	sendEmail(inbox.EmailAddress(), "Test 2")

	<-done

	if len(received) != 2 {
		t.Errorf("expected 2 emails, got %d", len(received))
	}
	if received[0].Subject != "Test 1" {
		t.Errorf("expected 'Test 1', got %s", received[0].Subject)
	}
	if received[1].Subject != "Test 2" {
		t.Errorf("expected 'Test 2', got %s", received[1].Subject)
	}
}
```

### Async Processing Test

```go
func TestProcessesEmailsAsynchronously(t *testing.T) {
	client, _ := vaultsandbox.New(apiKey)
	defer client.Close()

	ctx := context.Background()
	inbox, _ := client.CreateInbox(ctx)
	defer inbox.Delete(ctx)

	watchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var mu sync.Mutex
	var processed []string
	done := make(chan struct{})

	go func() {
		for email := range inbox.Watch(watchCtx) {
			processEmail(email)
			mu.Lock()
			processed = append(processed, email.ID)
			mu.Unlock()
			close(done)
			cancel()
			return
		}
	}()

	sendEmail(inbox.EmailAddress(), "Test")

	<-done

	mu.Lock()
	defer mu.Unlock()
	if len(processed) != 1 {
		t.Errorf("expected 1 processed, got %d", len(processed))
	}
}
```

## Error Handling

### Handle Processing Errors

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

for email := range inbox.Watch(ctx) {
	if err := processEmail(email); err != nil {
		log.Println("Error processing email:", err)
		// Continue processing - don't stop watching
	}
}
```

### Graceful Shutdown

```go
func gracefulShutdown(cancel context.CancelFunc, client *vaultsandbox.Client) {
	fmt.Println("Shutting down...")

	// Cancel context to stop all watchers
	cancel()

	// Wait for pending operations
	time.Sleep(1 * time.Second)

	// Close client
	client.Close()

	fmt.Println("Shutdown complete")
}

// Usage
ctx, cancel := context.WithCancel(context.Background())

go func() {
	for email := range inbox.Watch(ctx) {
		processEmail(email)
	}
}()

// Later, on shutdown signal
gracefulShutdown(cancel, client)
```

## SSE vs Polling

### When to Use SSE

Use SSE (real-time) when:

- You need instant notification of new emails
- Processing emails as they arrive
- Building real-time dashboards
- Minimizing latency is critical

```go
client, err := vaultsandbox.New(apiKey,
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),
)
```

### When to Use Polling

Use polling when:

- SSE is blocked by firewall/proxy
- Running in environments that don't support persistent connections
- Batch processing is acceptable

```go
client, err := vaultsandbox.New(apiKey,
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
)
```

### Auto Strategy (Recommended)

```go
client, err := vaultsandbox.New(apiKey,
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto),
)
```

The auto strategy attempts SSE first and automatically falls back to polling if SSE is unavailable.

## Advanced Patterns

### Rate-Limited Processing

```go
func rateLimitedWatch(ctx context.Context, inbox *vaultsandbox.Inbox, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	emails := inbox.Watch(ctx)
	for {
		select {
		case email, ok := <-emails:
			if !ok {
				return
			}
			processEmail(email)
			<-ticker.C // Wait for next tick
		case <-ctx.Done():
			return
		}
	}
}

// Usage: Process at most 1 email per second
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go rateLimitedWatch(ctx, inbox, time.Second)
```

### Priority Processing

```go
func getPriority(email *vaultsandbox.Email) string {
	if strings.Contains(email.Subject, "URGENT") {
		return "high"
	}
	if email.From == "alerts@example.com" {
		return "high"
	}
	if len(email.Attachments) > 0 {
		return "medium"
	}
	return "low"
}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

for email := range inbox.Watch(ctx) {
	switch getPriority(email) {
	case "high":
		processImmediately(email)
	case "medium":
		queueForProcessing(email)
	default:
		logAndDiscard(email)
	}
}
```

### Worker Pool Processing

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

emails := inbox.Watch(ctx)

// Fan-out to workers
var wg sync.WaitGroup
for i := 0; i < 3; i++ {
	wg.Add(1)
	go func(workerID int) {
		defer wg.Done()
		for email := range emails {
			fmt.Printf("Worker %d processing: %s\n", workerID, email.Subject)
			processEmail(email)
		}
	}(i)
}
wg.Wait()
```

### Multi-Inbox Worker Pool

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

events := client.WatchInboxes(ctx, inbox1, inbox2, inbox3)

var wg sync.WaitGroup
for i := 0; i < 5; i++ {
	wg.Add(1)
	go func(workerID int) {
		defer wg.Done()
		for event := range events {
			fmt.Printf("Worker %d processing email from %s: %s\n",
				workerID, event.Inbox.EmailAddress(), event.Email.Subject)
			processEmail(event.Email)
		}
	}(i)
}
wg.Wait()
```

## Cleanup

### Proper Cleanup in Tests

```go
func TestEmailMonitoring(t *testing.T) {
	client, err := vaultsandbox.New(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Delete(ctx)

	watchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for email := range inbox.Watch(watchCtx) {
		fmt.Println("Email:", email.Subject)
		break // Process first email only
	}

	// Channel automatically closes when context is cancelled
}
```

### Cleanup with Multi-Inbox Watching

```go
func TestMultiInboxMonitoring(t *testing.T) {
	client, _ := vaultsandbox.New(apiKey)
	defer client.Close()

	ctx := context.Background()

	inbox1, _ := client.CreateInbox(ctx)
	inbox2, _ := client.CreateInbox(ctx)
	defer inbox1.Delete(ctx)
	defer inbox2.Delete(ctx)

	watchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for event := range client.WatchInboxes(watchCtx, inbox1, inbox2) {
		fmt.Println("Email:", event.Email.Subject)
		break // Process first email only
	}

	// Channel automatically closes when context is cancelled
}
```

## Comparison: Watch vs WatchInboxes

| Aspect | `Inbox.Watch()` | `Client.WatchInboxes()` |
|--------|-----------------|-------------------------|
| **Scope** | Single inbox | Multiple inboxes |
| **Return Type** | `<-chan *Email` | `<-chan *InboxEvent` |
| **Inbox Info** | Implicit (from receiver) | `event.Inbox` field |
| **Lifecycle** | Context cancellation | Context cancellation |
| **Cleanup** | Automatic on ctx cancel | Automatic on ctx cancel |

## Channel Behavior

- **Buffer Size**: Channels are created with a buffer size of 16
- **Non-blocking Sends**: If the channel buffer is full, events may be dropped
- **Automatic Cleanup**: Channels close automatically when the context is cancelled
- **Empty Input**: `WatchInboxes` returns an immediately closed channel if no inboxes are provided

## Next Steps

- **[Waiting for Emails](/client-go/guides/waiting-for-emails/)** - Alternative polling-based approach
- **[Managing Inboxes](/client-go/guides/managing-inboxes/)** - Inbox operations
- **[Configuration](/client-go/configuration/)** - Configure SSE behavior
