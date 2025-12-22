---
title: Real-time Monitoring
description: Subscribe to emails as they arrive using Server-Sent Events
---

VaultSandbox supports real-time email notifications via Server-Sent Events (SSE), enabling instant processing of emails as they arrive.

## Basic Subscription

### Subscribe to Single Inbox

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
	log.Fatal(err)
}

fmt.Printf("Monitoring: %s\n", inbox.EmailAddress())

subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
	fmt.Printf("New email: %s\n", email.Subject)
	fmt.Printf("   From: %s\n", email.From)
	fmt.Printf("   Received: %s\n", email.ReceivedAt)
})

// Later, stop monitoring
// subscription.Unsubscribe()
```

### Subscribe with Processing

```go
subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
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
	if err := email.MarkAsRead(ctx); err != nil {
		log.Println("Failed to mark as read:", err)
	}
})
```

## Monitoring Multiple Inboxes

### Using InboxMonitor

```go
inbox1, _ := client.CreateInbox(ctx)
inbox2, _ := client.CreateInbox(ctx)
inbox3, _ := client.CreateInbox(ctx)

monitor, err := client.MonitorInboxes([]*vaultsandbox.Inbox{inbox1, inbox2, inbox3})
if err != nil {
	log.Fatal(err)
}

monitor.OnEmail(func(inbox *vaultsandbox.Inbox, email *vaultsandbox.Email) {
	fmt.Printf("Email in %s\n", inbox.EmailAddress())
	fmt.Printf("   Subject: %s\n", email.Subject)
	fmt.Printf("   From: %s\n", email.From)
})

// Later, stop monitoring all
// monitor.Unsubscribe()
```

### Monitoring with Handlers

```go
monitor, _ := client.MonitorInboxes([]*vaultsandbox.Inbox{inbox1, inbox2})

monitor.OnEmail(func(inbox *vaultsandbox.Inbox, email *vaultsandbox.Email) {
	switch {
	case email.From == "alerts@example.com":
		handleAlert(email)
	case strings.Contains(email.Subject, "Invoice"):
		handleInvoice(inbox, email)
	default:
		fmt.Println("Other email:", email.Subject)
	}
})
```

## Unsubscribing

### Unsubscribe from Single Inbox

```go
subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
	fmt.Println("Email:", email.Subject)
})

// Unsubscribe when done
subscription.Unsubscribe()
```

### Conditional Unsubscribe

```go
var subscription vaultsandbox.Subscription

subscription = inbox.OnNewEmail(func(email *vaultsandbox.Email) {
	fmt.Println("Email:", email.Subject)

	// Unsubscribe after first welcome email
	if strings.Contains(email.Subject, "Welcome") {
		subscription.Unsubscribe()
	}
})
```

### Unsubscribe from Monitor

```go
monitor, _ := client.MonitorInboxes([]*vaultsandbox.Inbox{inbox1, inbox2})

monitor.OnEmail(func(inbox *vaultsandbox.Inbox, email *vaultsandbox.Email) {
	fmt.Println("Email:", email.Subject)
})

// Unsubscribe from all inboxes
monitor.Unsubscribe()
```

### Selective Callback Unsubscribe

```go
monitor, _ := client.MonitorInboxes([]*vaultsandbox.Inbox{inbox1, inbox2})

// Register multiple callbacks
sub1 := monitor.OnEmail(func(inbox *vaultsandbox.Inbox, email *vaultsandbox.Email) {
	fmt.Println("Handler 1:", email.Subject)
})

sub2 := monitor.OnEmail(func(inbox *vaultsandbox.Inbox, email *vaultsandbox.Email) {
	fmt.Println("Handler 2:", email.Subject)
})

// Unsubscribe only the first callback
sub1.Unsubscribe()
// Handler 2 continues receiving emails
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

	resultCh := make(chan *vaultsandbox.Email, 1)

	var subscription vaultsandbox.Subscription
	subscription = inbox.OnNewEmail(func(email *vaultsandbox.Email) {
		if predicate(email) {
			subscription.Unsubscribe()
			select {
			case resultCh <- email:
			default:
			}
		}
	})

	select {
	case email := <-resultCh:
		return email, nil
	case <-ctx.Done():
		subscription.Unsubscribe()
		return nil, fmt.Errorf("timeout waiting for email")
	}
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

	var mu sync.Mutex
	emails := make([]*vaultsandbox.Email, 0, count)
	done := make(chan struct{})

	var subscription vaultsandbox.Subscription
	subscription = inbox.OnNewEmail(func(email *vaultsandbox.Email) {
		mu.Lock()
		defer mu.Unlock()

		emails = append(emails, email)
		fmt.Printf("Received %d/%d\n", len(emails), count)

		if len(emails) >= count {
			subscription.Unsubscribe()
			close(done)
		}
	})

	select {
	case <-done:
		return emails, nil
	case <-ctx.Done():
		subscription.Unsubscribe()
		mu.Lock()
		defer mu.Unlock()
		return nil, fmt.Errorf("timeout: only received %d/%d", len(emails), count)
	}
}

// Usage
emails, err := collectEmails(ctx, inbox, 3, 20*time.Second)
```

### Process Email Pipeline

```go
func processEmailPipeline(ctx context.Context, inbox *vaultsandbox.Inbox) vaultsandbox.Subscription {
	return inbox.OnNewEmail(func(email *vaultsandbox.Email) {
		fmt.Println("Processing:", email.Subject)

		// Step 1: Validate
		auth := email.AuthResults.Validate()
		if !auth.Passed {
			fmt.Println("Failed auth:", auth.Failures)
			return
		}

		// Step 2: Extract data
		links := email.Links
		attachments := email.Attachments

		// Step 3: Store/process
		if err := storeEmail(ctx, email); err != nil {
			fmt.Println("Error storing:", err)
			return
		}

		// Step 4: Notify
		if err := notifyProcessed(ctx, email.ID); err != nil {
			fmt.Println("Error notifying:", err)
			return
		}

		// Step 5: Cleanup
		if err := email.Delete(ctx); err != nil {
			fmt.Println("Error deleting:", err)
			return
		}

		fmt.Println("Processed:", email.Subject)
		_ = links       // use as needed
		_ = attachments // use as needed
	})
}

// Usage
subscription := processEmailPipeline(ctx, inbox)
defer subscription.Unsubscribe()
```

## Testing with Real-Time Monitoring

### Integration Test

```go
func TestRealTimeEmailProcessing(t *testing.T) {
	client, _ := vaultsandbox.New(apiKey)
	defer client.Close()

	ctx := context.Background()
	inbox, _ := client.CreateInbox(ctx)
	defer inbox.Delete(ctx)

	var mu sync.Mutex
	var received []*vaultsandbox.Email

	subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
		mu.Lock()
		received = append(received, email)
		mu.Unlock()
	})
	defer subscription.Unsubscribe()

	// Send test emails
	sendEmail(inbox.EmailAddress(), "Test 1")
	sendEmail(inbox.EmailAddress(), "Test 2")

	// Wait for emails to arrive
	time.Sleep(5 * time.Second)

	mu.Lock()
	defer mu.Unlock()

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

	var mu sync.Mutex
	var processed []string

	subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
		processEmail(email)
		mu.Lock()
		processed = append(processed, email.ID)
		mu.Unlock()
	})
	defer subscription.Unsubscribe()

	sendEmail(inbox.EmailAddress(), "Test")

	// Wait for processing
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(processed)
		mu.Unlock()
		if count > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(processed) != 1 {
		t.Errorf("expected 1 processed, got %d", len(processed))
	}
}
```

## Error Handling

### Handle Subscription Errors

```go
subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
	if err := processEmail(email); err != nil {
		log.Println("Error processing email:", err)
		// Don't panic - keeps subscription active
	}
})
```

### Graceful Shutdown

```go
func gracefulShutdown(subscriptions []vaultsandbox.Subscription, client *vaultsandbox.Client) {
	fmt.Println("Shutting down...")

	// Unsubscribe from all
	for _, sub := range subscriptions {
		sub.Unsubscribe()
	}

	// Wait for pending operations
	time.Sleep(1 * time.Second)

	// Close client
	client.Close()

	fmt.Println("Shutdown complete")
}

// Usage
subs := []vaultsandbox.Subscription{subscription1, subscription2}
gracefulShutdown(subs, client)
```

### Using Context for Cancellation

```go
func monitorWithContext(ctx context.Context, inbox *vaultsandbox.Inbox) {
	subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
		fmt.Println("Email:", email.Subject)
	})

	// Wait for context cancellation
	<-ctx.Done()
	subscription.Unsubscribe()
	fmt.Println("Monitoring stopped")
}

// Usage
ctx, cancel := context.WithCancel(context.Background())
go monitorWithContext(ctx, inbox)

// Later, stop monitoring
cancel()
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
type rateLimitedProcessor struct {
	queue   chan *vaultsandbox.Email
	process func(*vaultsandbox.Email)
}

func newRateLimitedProcessor(process func(*vaultsandbox.Email)) *rateLimitedProcessor {
	p := &rateLimitedProcessor{
		queue:   make(chan *vaultsandbox.Email, 100),
		process: process,
	}
	go p.run()
	return p
}

func (p *rateLimitedProcessor) run() {
	for email := range p.queue {
		p.process(email)
		time.Sleep(1 * time.Second) // Rate limit: 1 per second
	}
}

func (p *rateLimitedProcessor) enqueue(email *vaultsandbox.Email) {
	p.queue <- email
}

func (p *rateLimitedProcessor) close() {
	close(p.queue)
}

// Usage
processor := newRateLimitedProcessor(func(email *vaultsandbox.Email) {
	fmt.Println("Processing:", email.Subject)
})
defer processor.close()

subscription := inbox.OnNewEmail(processor.enqueue)
defer subscription.Unsubscribe()
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

subscription := inbox.OnNewEmail(func(email *vaultsandbox.Email) {
	switch getPriority(email) {
	case "high":
		processImmediately(email)
	case "medium":
		queueForProcessing(email)
	default:
		logAndDiscard(email)
	}
})
```

### Worker Pool Processing

```go
type workerPool struct {
	jobs    chan *vaultsandbox.Email
	workers int
	wg      sync.WaitGroup
}

func newWorkerPool(workers int, process func(*vaultsandbox.Email)) *workerPool {
	p := &workerPool{
		jobs:    make(chan *vaultsandbox.Email, 100),
		workers: workers,
	}

	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go func(id int) {
			defer p.wg.Done()
			for email := range p.jobs {
				fmt.Printf("Worker %d processing: %s\n", id, email.Subject)
				process(email)
			}
		}(i)
	}

	return p
}

func (p *workerPool) submit(email *vaultsandbox.Email) {
	p.jobs <- email
}

func (p *workerPool) shutdown() {
	close(p.jobs)
	p.wg.Wait()
}

// Usage
pool := newWorkerPool(3, processEmail)
defer pool.shutdown()

monitor, _ := client.MonitorInboxes([]*vaultsandbox.Inbox{inbox1, inbox2, inbox3})
monitor.OnEmail(func(inbox *vaultsandbox.Inbox, email *vaultsandbox.Email) {
	pool.submit(email)
})
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

	var subscription vaultsandbox.Subscription
	defer func() {
		if subscription != nil {
			subscription.Unsubscribe()
		}
	}()

	subscription = inbox.OnNewEmail(func(email *vaultsandbox.Email) {
		fmt.Println("Email:", email.Subject)
	})

	// Test code...
}
```

### Cleanup with Monitor

```go
func TestMultiInboxMonitoring(t *testing.T) {
	client, _ := vaultsandbox.New(apiKey)
	defer client.Close()

	ctx := context.Background()

	inbox1, _ := client.CreateInbox(ctx)
	inbox2, _ := client.CreateInbox(ctx)
	defer inbox1.Delete(ctx)
	defer inbox2.Delete(ctx)

	monitor, _ := client.MonitorInboxes([]*vaultsandbox.Inbox{inbox1, inbox2})
	defer monitor.Unsubscribe()

	monitor.OnEmail(func(inbox *vaultsandbox.Inbox, email *vaultsandbox.Email) {
		fmt.Println("Email:", email.Subject)
	})

	// Test code...
}
```

## Single vs Multiple Inbox Monitoring

| Aspect | Single Inbox (`OnNewEmail`) | Multiple Inboxes (`MonitorInboxes`) |
|--------|---------------------------|-------------------------------------|
| **Method** | `Inbox.OnNewEmail()` | `Client.MonitorInboxes()` |
| **Callback Type** | `func(email *Email)` | `func(inbox *Inbox, email *Email)` |
| **Inbox Info** | Implicit (from receiver) | Passed to callback |
| **Strategy** | Uses client's strategy (SSE/polling/auto) | Uses client's strategy (SSE/polling/auto) |
| **Multiple Callbacks** | Create separate subscriptions | Register via `OnEmail()` |
| **Selective Unsubscribe** | N/A (one per inbox) | Yes, per-callback |
| **Complexity** | Simple, lightweight | More powerful, flexible |

## Next Steps

- **[Waiting for Emails](/client-go/guides/waiting-for-emails/)** - Alternative polling-based approach
- **[Managing Inboxes](/client-go/guides/managing-inboxes/)** - Inbox operations
- **[Configuration](/client-go/configuration/)** - Configure SSE behavior
