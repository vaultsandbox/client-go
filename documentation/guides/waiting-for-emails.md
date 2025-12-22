---
title: Waiting for Emails
description: Efficiently wait for and filter emails in your tests
---

VaultSandbox provides powerful methods for waiting for emails with filtering and timeout support.

## Basic Waiting

### Wait for Any Email

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

email, err := inbox.WaitForEmail(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Println("Received:", email.Subject)
```

### With Explicit Timeout Option

```go
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(30*time.Second),
)
if err != nil {
    log.Fatal(err)
}
```

## Filtering Options

### Filter by Subject

```go
// Exact match
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithSubject("Password Reset"),
)

// Regex match
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)reset`)), // Case-insensitive
)
```

### Filter by Sender

```go
// Exact match
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithFrom("noreply@example.com"),
)

// Regex match
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithFromRegex(regexp.MustCompile(`@example\.com$`)), // Any @example.com address
)
```

### Multiple Filters

```go
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)welcome`)),
    vaultsandbox.WithFrom("support@example.com"),
)
```

### Custom Predicate

```go
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithPredicate(func(email *vaultsandbox.Email) bool {
        // Custom logic
        hasRecipient := false
        for _, to := range email.To {
            if to == "specific@example.com" {
                hasRecipient = true
                break
            }
        }
        return hasRecipient &&
            len(email.Links) > 0 &&
            strings.Contains(email.Subject, "Verify")
    }),
)
```

## Waiting for Multiple Emails

### Wait for Specific Count

```go
// Trigger multiple emails
sendNotifications(inbox.EmailAddress(), 3)

// Wait for all 3 to arrive
emails, err := inbox.WaitForEmailCount(ctx, 3,
    vaultsandbox.WithWaitTimeout(30*time.Second),
    vaultsandbox.WithPollInterval(1*time.Second), // Check every second
)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Received %d emails\n", len(emails))
```

### Process as They Arrive

```go
func waitForEmails(ctx context.Context, inbox *vaultsandbox.Inbox, count int) ([]*vaultsandbox.Email, error) {
    emails := make([]*vaultsandbox.Email, 0, count)

    for i := 0; i < count; i++ {
        email, err := inbox.WaitForEmail(ctx,
            vaultsandbox.WithWaitTimeout(30*time.Second),
        )
        if err != nil {
            return emails, err
        }
        emails = append(emails, email)
        fmt.Printf("Received %d/%d: %s\n", i+1, count, email.Subject)
    }

    return emails, nil
}

// Usage
emails, err := waitForEmails(ctx, inbox, 3)
```

## Timeout Handling

### With Error Handling

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

email, err := inbox.WaitForEmail(ctx)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        fmt.Println("No email received within 5 seconds")
    } else {
        log.Fatal(err)
    }
}

if email != nil {
    fmt.Println("Email received:", email.Subject)
}
```

### With Fallback

```go
func waitForEmailWithFallback(ctx context.Context, inbox *vaultsandbox.Inbox, opts ...vaultsandbox.WaitOption) (*vaultsandbox.Email, error) {
    email, err := inbox.WaitForEmail(ctx, opts...)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            fmt.Println("Timeout, checking if email arrived anyway")
            emails, listErr := inbox.GetEmails(ctx)
            if listErr != nil {
                return nil, listErr
            }
            if len(emails) > 0 {
                return emails[len(emails)-1], nil // Return latest
            }
        }
        return nil, err
    }
    return email, nil
}
```

### Retry Pattern

```go
func waitWithRetry(ctx context.Context, inbox *vaultsandbox.Inbox, maxRetries int, opts ...vaultsandbox.WaitOption) (*vaultsandbox.Email, error) {
    var lastErr error
    for i := 0; i < maxRetries; i++ {
        email, err := inbox.WaitForEmail(ctx, opts...)
        if err == nil {
            return email, nil
        }
        lastErr = err
        if errors.Is(err, context.DeadlineExceeded) && i < maxRetries-1 {
            fmt.Printf("Attempt %d failed, retrying...\n", i+1)
            continue
        }
        break
    }
    return nil, lastErr
}
```

## Polling Configuration

### Custom Poll Interval

```go
// Poll every 500ms (more responsive)
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithPollInterval(500*time.Millisecond),
)

// Poll every 5 seconds (less frequent)
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(60*time.Second),
    vaultsandbox.WithPollInterval(5*time.Second),
)
```

### Efficient Polling

```go
// For quick tests - poll frequently
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(5*time.Second),
    vaultsandbox.WithPollInterval(200*time.Millisecond), // Check 5 times per second
)

// For slow email services - poll less frequently
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(2*time.Minute),
    vaultsandbox.WithPollInterval(10*time.Second), // Check every 10 seconds
)
```

## Real-World Examples

### Password Reset Flow

```go
func TestPasswordResetEmail(t *testing.T) {
    ctx := context.Background()
    inbox, _ := client.CreateInbox(ctx)
    defer inbox.Delete(ctx)

    // Trigger reset
    app.RequestPasswordReset(inbox.EmailAddress())

    // Wait for reset email
    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(10*time.Second),
        vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)reset`)),
        vaultsandbox.WithFrom("security@example.com"),
    )
    require.NoError(t, err)

    // Validate content
    assert.Contains(t, email.Subject, "Password Reset")
    assert.Greater(t, len(email.Links), 0)

    // Extract and test link
    var resetLink string
    for _, link := range email.Links {
        if strings.Contains(link, "/reset") {
            resetLink = link
            break
        }
    }
    assert.NotEmpty(t, resetLink)
}
```

### Welcome Email with Verification

```go
func TestWelcomeEmailWithVerification(t *testing.T) {
    ctx := context.Background()
    inbox, _ := client.CreateInbox(ctx)
    defer inbox.Delete(ctx)

    // Sign up
    app.Signup(SignupRequest{
        Email: inbox.EmailAddress(),
        Name:  "Test User",
    })

    // Wait for welcome email
    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(10*time.Second),
        vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)welcome`)),
        vaultsandbox.WithPredicate(func(e *vaultsandbox.Email) bool {
            for _, link := range e.Links {
                if strings.Contains(link, "/verify") {
                    return true
                }
            }
            return false
        }),
    )
    require.NoError(t, err)

    // Extract verification link
    var verifyLink string
    for _, link := range email.Links {
        if strings.Contains(link, "/verify") {
            verifyLink = link
            break
        }
    }

    // Test verification
    resp, err := http.Get(verifyLink)
    require.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

### Multi-Step Email Flow

```go
func TestOrderConfirmationAndShipping(t *testing.T) {
    ctx := context.Background()
    inbox, _ := client.CreateInbox(ctx)
    defer inbox.Delete(ctx)

    // Place order
    orderId := app.PlaceOrder(OrderRequest{
        Email: inbox.EmailAddress(),
        Items: []string{"widget"},
    })

    // Wait for confirmation
    confirmation, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(10*time.Second),
        vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)order.*confirmed`)),
    )
    require.NoError(t, err)

    assert.Contains(t, confirmation.Subject, "Order Confirmed")
    assert.Contains(t, confirmation.Text, "Order #")

    // Simulate shipping
    app.ShipOrder(orderId)

    // Wait for shipping notification
    shipping, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(10*time.Second),
        vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)shipped`)),
    )
    require.NoError(t, err)

    assert.Contains(t, shipping.Subject, "Shipped")
    assert.Contains(t, shipping.Text, "tracking")
}
```

### Email with Attachments

```go
func TestInvoiceWithPDFAttachment(t *testing.T) {
    ctx := context.Background()
    inbox, _ := client.CreateInbox(ctx)
    defer inbox.Delete(ctx)

    app.SendInvoice(inbox.EmailAddress())

    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(10*time.Second),
        vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)invoice`)),
        vaultsandbox.WithPredicate(func(e *vaultsandbox.Email) bool {
            return len(e.Attachments) > 0
        }),
    )
    require.NoError(t, err)

    // Find PDF attachment
    var pdf *vaultsandbox.Attachment
    for i := range email.Attachments {
        if email.Attachments[i].ContentType == "application/pdf" {
            pdf = &email.Attachments[i]
            break
        }
    }

    require.NotNil(t, pdf)
    assert.Regexp(t, regexp.MustCompile(`(?i)invoice.*\.pdf`), pdf.Filename)
    assert.Greater(t, pdf.Size, 0)
}
```

## Advanced Patterns

### Wait for First Matching Email

```go
func waitForFirstMatch(ctx context.Context, inbox *vaultsandbox.Inbox, matchers []func(*vaultsandbox.Email) bool, timeout time.Duration) (*vaultsandbox.Email, error) {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil, fmt.Errorf("no matching email found: %w", ctx.Err())
        case <-ticker.C:
            emails, err := inbox.GetEmails(ctx)
            if err != nil {
                return nil, err
            }

            for _, matcher := range matchers {
                for _, email := range emails {
                    if matcher(email) {
                        return email, nil
                    }
                }
            }
        }
    }
}

// Usage
email, err := waitForFirstMatch(ctx, inbox, []func(*vaultsandbox.Email) bool{
    func(e *vaultsandbox.Email) bool { return strings.Contains(e.Subject, "Welcome") },
    func(e *vaultsandbox.Email) bool { return strings.Contains(e.Subject, "Verify") },
    func(e *vaultsandbox.Email) bool { return e.From == "support@example.com" },
}, 30*time.Second)
```

### Wait with Progress Callback

```go
type WaitProgress struct {
    Attempts int
    Elapsed  time.Duration
    TimedOut bool
}

func waitWithProgress(ctx context.Context, inbox *vaultsandbox.Inbox, timeout time.Duration, onProgress func(WaitProgress)) (*vaultsandbox.Email, error) {
    startTime := time.Now()
    attempts := 0

    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            onProgress(WaitProgress{
                Attempts: attempts,
                Elapsed:  time.Since(startTime),
                TimedOut: true,
            })
            return nil, ctx.Err()
        case <-ticker.C:
            attempts++
            onProgress(WaitProgress{
                Attempts: attempts,
                Elapsed:  time.Since(startTime),
                TimedOut: false,
            })

            emails, err := inbox.GetEmails(ctx)
            if err != nil {
                return nil, err
            }
            if len(emails) > 0 {
                return emails[0], nil
            }
        }
    }
}

// Usage
email, err := waitWithProgress(ctx, inbox, 10*time.Second, func(p WaitProgress) {
    fmt.Printf("Attempt %d, %v elapsed\n", p.Attempts, p.Elapsed)
})
```

### Conditional Waiting

```go
func waitConditionally(ctx context.Context, inbox *vaultsandbox.Inbox, subjectPattern *regexp.Regexp) (*vaultsandbox.Email, error) {
    // First check if email already exists
    existing, err := inbox.GetEmails(ctx)
    if err != nil {
        return nil, err
    }

    for _, email := range existing {
        if subjectPattern.MatchString(email.Subject) {
            fmt.Println("Email already present")
            return email, nil
        }
    }

    // Wait for new email
    fmt.Println("Waiting for email...")
    return inbox.WaitForEmail(ctx,
        vaultsandbox.WithSubjectRegex(subjectPattern),
    )
}
```

## Testing Patterns

### Flake-Free Tests

```go
// Good: Use WaitForEmail, not sleep
func TestReceivesEmail(t *testing.T) {
    ctx := context.Background()
    inbox, _ := client.CreateInbox(ctx)
    defer inbox.Delete(ctx)

    sendEmail(inbox.EmailAddress())

    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(10*time.Second),
    )
    require.NoError(t, err)
    assert.NotNil(t, email)
}

// Bad: Arbitrary sleep causes flakiness
func TestReceivesEmailBad(t *testing.T) {
    ctx := context.Background()
    inbox, _ := client.CreateInbox(ctx)
    defer inbox.Delete(ctx)

    sendEmail(inbox.EmailAddress())
    time.Sleep(5 * time.Second) // May not be enough, or wastes time

    emails, _ := inbox.GetEmails(ctx)
    assert.Equal(t, 1, len(emails))
}
```

### Fast Tests

```go
// Good: Short timeout for fast-sending systems
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(2*time.Second),
)

// Bad: Unnecessarily long timeout slows tests
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(60*time.Second),
)
```

### Parallel Email Tests

```go
func TestMultipleUsersReceiveEmails(t *testing.T) {
    ctx := context.Background()
    inbox1, _ := client.CreateInbox(ctx)
    inbox2, _ := client.CreateInbox(ctx)
    defer inbox1.Delete(ctx)
    defer inbox2.Delete(ctx)

    // Send emails concurrently
    var wg sync.WaitGroup
    wg.Add(2)
    go func() {
        defer wg.Done()
        sendWelcome(inbox1.EmailAddress())
    }()
    go func() {
        defer wg.Done()
        sendWelcome(inbox2.EmailAddress())
    }()
    wg.Wait()

    // Wait in parallel
    var email1, email2 *vaultsandbox.Email
    var err1, err2 error

    wg.Add(2)
    go func() {
        defer wg.Done()
        email1, err1 = inbox1.WaitForEmail(ctx,
            vaultsandbox.WithWaitTimeout(10*time.Second),
        )
    }()
    go func() {
        defer wg.Done()
        email2, err2 = inbox2.WaitForEmail(ctx,
            vaultsandbox.WithWaitTimeout(10*time.Second),
        )
    }()
    wg.Wait()

    require.NoError(t, err1)
    require.NoError(t, err2)
    assert.Contains(t, email1.Subject, "Welcome")
    assert.Contains(t, email2.Subject, "Welcome")
}
```

### Using errgroup for Cleaner Parallel Tests

```go
func TestMultipleUsersWithErrgroup(t *testing.T) {
    ctx := context.Background()
    inbox1, _ := client.CreateInbox(ctx)
    inbox2, _ := client.CreateInbox(ctx)
    defer inbox1.Delete(ctx)
    defer inbox2.Delete(ctx)

    // Send emails
    sendWelcome(inbox1.EmailAddress())
    sendWelcome(inbox2.EmailAddress())

    // Wait in parallel with errgroup
    g, ctx := errgroup.WithContext(ctx)

    var email1, email2 *vaultsandbox.Email

    g.Go(func() error {
        var err error
        email1, err = inbox1.WaitForEmail(ctx,
            vaultsandbox.WithWaitTimeout(10*time.Second),
        )
        return err
    })

    g.Go(func() error {
        var err error
        email2, err = inbox2.WaitForEmail(ctx,
            vaultsandbox.WithWaitTimeout(10*time.Second),
        )
        return err
    })

    require.NoError(t, g.Wait())
    assert.Contains(t, email1.Subject, "Welcome")
    assert.Contains(t, email2.Subject, "Welcome")
}
```

## Troubleshooting

### Email Not Arriving

```go
// Add debug logging
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`(?i)test`)),
)
if err != nil {
    fmt.Println("Timeout! Checking inbox manually:")
    emails, listErr := inbox.GetEmails(ctx)
    if listErr != nil {
        log.Fatal(listErr)
    }
    fmt.Printf("Found %d emails:\n", len(emails))
    for _, e := range emails {
        fmt.Printf("  - %s\n", e.Subject)
    }
    log.Fatal(err)
}

fmt.Println("Received:", email.Subject)
```

### Filter Not Matching

```go
// Log filter mismatches
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithPredicate(func(e *vaultsandbox.Email) bool {
        matches := strings.Contains(e.Subject, "Test")
        if !matches {
            fmt.Printf("Subject %q doesn't match\n", e.Subject)
        }
        return matches
    }),
)
```

## Next Steps

- **[Managing Inboxes](/client-go/guides/managing-inboxes/)** - Learn inbox operations
- **[Real-time Monitoring](/client-go/guides/real-time/)** - Subscribe to emails as they arrive
- **[Testing Patterns](/client-go/testing/password-reset/)** - Real-world testing examples
- **[API Reference: Inbox](/client-go/api/inbox/)** - Complete API documentation
