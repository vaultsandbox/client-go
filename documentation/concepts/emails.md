---
title: Email Objects
description: Understanding email objects and their properties in VaultSandbox
---

Email objects in VaultSandbox represent decrypted emails with all their content, headers, and metadata.

## Email Structure

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WaitOptions{Timeout: 30 * time.Second})
if err != nil {
    log.Fatal(err)
}

fmt.Println(email.ID)          // "email_abc123"
fmt.Println(email.From)        // "sender@example.com"
fmt.Println(email.To)          // ["recipient@mail.example.com"]
fmt.Println(email.Subject)     // "Welcome to our service"
fmt.Println(email.Text)        // Plain text content
fmt.Println(email.HTML)        // HTML content
fmt.Println(email.ReceivedAt)  // time.Time
fmt.Println(email.IsRead)      // false
fmt.Println(email.Links)       // ["https://example.com/verify"]
fmt.Println(email.Attachments) // []Attachment
fmt.Println(email.AuthResults) // SPF/DKIM/DMARC results
```

## Core Properties

### ID

**Type**: `string`

Unique identifier for the email.

```go
emailID := email.ID
// Later...
sameEmail, err := inbox.GetEmail(ctx, emailID)
```

### From

**Type**: `string`

Sender's email address (from the `From` header).

```go
fmt.Println(email.From) // "noreply@example.com"

// Use in assertions
if email.From != "support@example.com" {
    t.Error("unexpected sender")
}
```

### To

**Type**: `[]string`

Slice of recipient email addresses.

```go
fmt.Println(email.To) // ["user@mail.example.com"]

// Multiple recipients
fmt.Println(email.To) // ["user1@mail.example.com", "user2@mail.example.com"]

// Check if sent to specific address
if !slices.Contains(email.To, inbox.EmailAddress()) {
    t.Error("inbox address not in recipients")
}
```

### Subject

**Type**: `string`

Email subject line.

```go
fmt.Println(email.Subject) // "Password Reset Request"

// Use in filtering
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WaitOptions{
    Subject: regexp.MustCompile(`Password Reset`),
})
```

### Text

**Type**: `string`

Plain text content of the email. Empty string if email is HTML-only.

```go
fmt.Println(email.Text)
// "Hello,\n\nClick here to reset your password:\nhttps://..."

// May be empty if email is HTML-only
if email.Text != "" {
    if !strings.Contains(email.Text, "reset your password") {
        t.Error("expected reset text")
    }
}
```

### HTML

**Type**: `string`

HTML content of the email. Empty string if email is plain text only.

```go
fmt.Println(email.HTML)
// "<html><body><p>Hello,</p><a href='https://...'>Reset Password</a></body></html>"

// May be empty if email is plain text only
if email.HTML != "" {
    if !strings.Contains(email.HTML, "<a href") {
        t.Error("expected link in HTML")
    }
}
```

### ReceivedAt

**Type**: `time.Time`

When the email was received by the gateway.

```go
fmt.Println(email.ReceivedAt) // 2024-01-15 12:00:00 +0000 UTC

// Check if email arrived recently
age := time.Since(email.ReceivedAt)
if age > time.Minute {
    t.Error("email took too long to arrive")
}
```

### IsRead

**Type**: `bool`

Whether the email has been marked as read.

```go
fmt.Println(email.IsRead) // false

if err := email.MarkAsRead(ctx); err != nil {
    log.Fatal(err)
}

fmt.Println(email.IsRead) // true
```

### Links

**Type**: `[]string`

All URLs extracted from the email (text and HTML).

```go
fmt.Println(email.Links)
// [
//   "https://example.com/verify?token=abc123",
//   "https://example.com/unsubscribe",
//   "https://example.com/privacy"
// ]

// Find specific link
var verifyLink string
for _, link := range email.Links {
    if strings.Contains(link, "/verify") {
        verifyLink = link
        break
    }
}
if verifyLink == "" {
    t.Fatal("verify link not found")
}

// Test link
resp, err := http.Get(verifyLink)
if err != nil {
    t.Fatal(err)
}
defer resp.Body.Close()
if resp.StatusCode != http.StatusOK {
    t.Errorf("expected 200, got %d", resp.StatusCode)
}
```

### Attachments

**Type**: `[]Attachment`

Slice of email attachments.

```go
fmt.Println(len(email.Attachments)) // 2

for _, att := range email.Attachments {
    fmt.Println(att.Filename)    // "invoice.pdf"
    fmt.Println(att.ContentType) // "application/pdf"
    fmt.Println(att.Size)        // 15234 bytes
    fmt.Println(len(att.Content)) // byte slice length
}
```

See [Working with Attachments](/client-go/guides/attachments/) for details.

### AuthResults

**Type**: `*authresults.AuthResults`

Email authentication results (SPF, DKIM, DMARC, reverse DNS).

```go
auth := email.AuthResults

if auth.SPF != nil {
    fmt.Println(auth.SPF.Status) // "pass"
}
fmt.Println(len(auth.DKIM)) // 1
if auth.DMARC != nil {
    fmt.Println(auth.DMARC.Status) // "pass"
}

// Validate all checks
validation := auth.Validate()
if !validation.Passed {
    fmt.Println("Authentication failed:", validation.Failures)
}
```

See [Authentication Results](/client-go/concepts/auth-results/) for details.

### Headers

**Type**: `map[string]string`

All email headers as a key-value map.

```go
fmt.Println(email.Headers)
// map[
//   "from": "noreply@example.com",
//   "to": "user@mail.example.com",
//   "subject": "Welcome",
//   "message-id": "<abc123@example.com>",
//   "date": "Mon, 15 Jan 2024 12:00:00 +0000",
//   "content-type": "text/html; charset=utf-8",
//   ...
// ]

// Access specific headers
messageID := email.Headers["message-id"]
contentType := email.Headers["content-type"]
```

## Email Methods

### MarkAsRead

Mark the email as read.

```go
func (e *Email) MarkAsRead(ctx context.Context) error
```

```go
if err := email.MarkAsRead(ctx); err != nil {
    log.Fatal(err)
}

fmt.Println(email.IsRead) // true
```

### Delete

Delete the email from the inbox.

```go
func (e *Email) Delete(ctx context.Context) error
```

```go
if err := email.Delete(ctx); err != nil {
    log.Fatal(err)
}

// Email is now deleted
_, err := inbox.GetEmail(ctx, email.ID)
if errors.Is(err, vaultsandbox.ErrEmailNotFound) {
    fmt.Println("Email deleted")
}
```

### GetRaw

Get the raw email source (decrypted MIME).

```go
func (e *Email) GetRaw(ctx context.Context) (string, error)
```

```go
raw, err := email.GetRaw(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Println(raw)
// "From: sender@example.com\r\nTo: recipient@example.com\r\n..."
```

## Attachment Structure

```go
type Attachment struct {
    Filename           string // The attachment's filename
    ContentType        string // MIME type (e.g., "application/pdf")
    Size               int    // Size in bytes
    ContentID          string // Content-ID for inline attachments
    ContentDisposition string // "inline" or "attachment"
    Content            []byte // Raw attachment data
    Checksum           string // SHA-256 hash for integrity verification
}
```

## Common Patterns

### Content Validation

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WaitOptions{
    Subject: regexp.MustCompile(`Welcome`),
    Timeout: 10 * time.Second,
})
if err != nil {
    t.Fatal(err)
}

// Validate sender
if email.From != "noreply@example.com" {
    t.Errorf("unexpected sender: %s", email.From)
}

// Validate content
if !strings.Contains(email.Text, "Thank you for signing up") {
    t.Error("missing welcome text")
}
if !strings.Contains(email.HTML, "<h1>Welcome</h1>") {
    t.Error("missing welcome heading")
}

// Validate links
var verifyLink string
for _, link := range email.Links {
    if strings.Contains(link, "/verify") {
        verifyLink = link
        break
    }
}
if verifyLink == "" {
    t.Fatal("verify link not found")
}
if !strings.HasPrefix(verifyLink, "https://") {
    t.Error("verify link should use HTTPS")
}
```

### Link Extraction and Testing

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WaitOptions{
    Subject: regexp.MustCompile(`Reset`),
})
if err != nil {
    t.Fatal(err)
}

// Extract reset link
var resetLink string
for _, link := range email.Links {
    if strings.Contains(link, "reset-password") || strings.Contains(link, "token=") {
        resetLink = link
        break
    }
}
if resetLink == "" {
    t.Fatal("reset link not found")
}

// Extract token from link
u, err := url.Parse(resetLink)
if err != nil {
    t.Fatal(err)
}
token := u.Query().Get("token")

if token == "" {
    t.Fatal("token not found in link")
}
if len(token) <= 20 {
    t.Error("token seems too short")
}

// Test the link
resp, err := http.Get(resetLink)
if err != nil {
    t.Fatal(err)
}
defer resp.Body.Close()
if resp.StatusCode != http.StatusOK {
    t.Errorf("expected 200, got %d", resp.StatusCode)
}
```

### Multi-Part Emails

```go
// Email with both text and HTML
if email.Text != "" && email.HTML != "" {
    // Validate both versions have key content
    if !strings.Contains(email.Text, "Welcome") {
        t.Error("missing welcome in text")
    }
    if !strings.Contains(email.HTML, "<h1>Welcome</h1>") {
        t.Error("missing welcome in HTML")
    }
}

// HTML-only email
if email.HTML != "" && email.Text == "" {
    fmt.Println("HTML-only email")
    if !strings.Contains(email.HTML, "<!DOCTYPE html>") {
        t.Error("missing doctype")
    }
}

// Plain text only
if email.Text != "" && email.HTML == "" {
    fmt.Println("Plain text email")
}
```

### Time-Based Assertions

```go
startTime := time.Now()

// Trigger email
sendWelcomeEmail(inbox.EmailAddress())

// Wait and receive
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WaitOptions{Timeout: 10 * time.Second})
if err != nil {
    t.Fatal(err)
}

// Verify it arrived quickly
deliveryTime := email.ReceivedAt.Sub(startTime)
if deliveryTime > 5*time.Second {
    t.Errorf("email took too long: %v", deliveryTime)
}
```

### Email Metadata Analysis

```go
fmt.Println("Email details:")
fmt.Printf("- From: %s\n", email.From)
fmt.Printf("- Subject: %s\n", email.Subject)
fmt.Printf("- Received: %s\n", email.ReceivedAt.Format(time.RFC3339))
fmt.Printf("- Size: %d chars\n", len(email.Text))
fmt.Printf("- Links: %d\n", len(email.Links))
fmt.Printf("- Attachments: %d\n", len(email.Attachments))

// Check email authentication
if email.AuthResults != nil {
    auth := email.AuthResults.Validate()
    fmt.Printf("- Auth passed: %v\n", auth.Passed)
    if !auth.Passed {
        fmt.Printf("- Auth failures: %v\n", auth.Failures)
    }
}
```

## Testing Examples

### Standard Test

```go
func TestWelcomeEmail(t *testing.T) {
    ctx := context.Background()

    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }
    defer inbox.Delete(ctx)

    // Trigger email
    registerUser(inbox.EmailAddress())

    email, err := inbox.WaitForEmail(ctx, vaultsandbox.WaitOptions{
        Subject: regexp.MustCompile(`Welcome`),
        Timeout: 10 * time.Second,
    })
    if err != nil {
        t.Fatal(err)
    }

    if email.From != "noreply@example.com" {
        t.Errorf("unexpected sender: %s", email.From)
    }
    if !strings.Contains(email.Subject, "Welcome") {
        t.Errorf("unexpected subject: %s", email.Subject)
    }
    if !strings.Contains(email.Text, "Thank you for signing up") {
        t.Error("missing welcome text")
    }

    var verifyLink string
    for _, link := range email.Links {
        if strings.Contains(link, "/verify") {
            verifyLink = link
            break
        }
    }
    if verifyLink == "" {
        t.Error("verify link not found")
    }
}

func TestUnsubscribeLink(t *testing.T) {
    ctx := context.Background()

    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }
    defer inbox.Delete(ctx)

    registerUser(inbox.EmailAddress())

    email, err := inbox.WaitForEmail(ctx, vaultsandbox.WaitOptions{Timeout: 10 * time.Second})
    if err != nil {
        t.Fatal(err)
    }

    var unsubLink string
    for _, link := range email.Links {
        if strings.Contains(link, "/unsubscribe") || strings.Contains(link, "list-unsubscribe") {
            unsubLink = link
            break
        }
    }
    if unsubLink == "" {
        t.Error("unsubscribe link not found")
    }
}
```

### Table-Driven Test

```go
func TestPasswordResetFlow(t *testing.T) {
    ctx := context.Background()

    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }
    defer inbox.Delete(ctx)

    requestPasswordReset(inbox.EmailAddress())

    email, err := inbox.WaitForEmail(ctx, vaultsandbox.WaitOptions{
        Subject: regexp.MustCompile(`(?i)reset`),
        Timeout: 10 * time.Second,
    })
    if err != nil {
        t.Fatal(err)
    }

    t.Run("sender", func(t *testing.T) {
        if email.From != "security@example.com" {
            t.Errorf("expected security@example.com, got %s", email.From)
        }
    })

    t.Run("reset link", func(t *testing.T) {
        if len(email.Links) == 0 {
            t.Fatal("no links found")
        }

        resetLink := email.Links[0]
        if !strings.HasPrefix(resetLink, "https://") {
            t.Error("reset link should use HTTPS")
        }
        if !strings.Contains(resetLink, "token=") {
            t.Error("reset link should contain token")
        }

        // Verify token format
        u, err := url.Parse(resetLink)
        if err != nil {
            t.Fatal(err)
        }
        token := u.Query().Get("token")
        if len(token) != 64 {
            t.Errorf("expected token length 64, got %d", len(token))
        }
    })
}
```

### Using testify/assert

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestWelcomeEmailWithTestify(t *testing.T) {
    ctx := context.Background()

    inbox, err := client.CreateInbox(ctx)
    require.NoError(t, err)
    defer inbox.Delete(ctx)

    registerUser(inbox.EmailAddress())

    email, err := inbox.WaitForEmail(ctx, vaultsandbox.WaitOptions{
        Subject: regexp.MustCompile(`Welcome`),
        Timeout: 10 * time.Second,
    })
    require.NoError(t, err)

    assert.Equal(t, "noreply@example.com", email.From)
    assert.Contains(t, email.Subject, "Welcome")
    assert.Contains(t, email.Text, "Thank you for signing up")
    assert.NotEmpty(t, email.Links)
}
```

## Troubleshooting

### Email Content is Empty

```go
if email.Text == "" && email.HTML == "" {
    fmt.Println("Email has no content")
    fmt.Printf("Headers: %v\n", email.Headers)

    raw, err := email.GetRaw(ctx)
    if err == nil {
        fmt.Printf("Raw: %s\n", raw)
    }
}
```

### Links Not Extracted

```go
if len(email.Links) == 0 {
    fmt.Println("No links found")
    fmt.Printf("Text: %s\n", email.Text)
    fmt.Printf("HTML: %s\n", email.HTML)

    // Manually extract
    urlRegex := regexp.MustCompile(`https?://[^\s]+`)
    textLinks := urlRegex.FindAllString(email.Text, -1)
    fmt.Printf("Manual extraction: %v\n", textLinks)
}
```

### Decryption Errors

```go
email, err := inbox.GetEmail(ctx, emailID)
if err != nil {
    var decryptErr *vaultsandbox.DecryptionError
    if errors.As(err, &decryptErr) {
        fmt.Println("Failed to decrypt email")
        fmt.Println("This may indicate:")
        fmt.Println("- Wrong private key")
        fmt.Println("- Corrupted data")
        fmt.Println("- Server issue")
    }
}
```

## Next Steps

- **[Authentication Results](/client-go/concepts/auth-results/)** - Email authentication details
- **[Working with Attachments](/client-go/guides/attachments/)** - Handle email attachments
- **[Email Authentication](/client-go/guides/authentication/)** - Test SPF/DKIM/DMARC
- **[API Reference: Email](/client-go/api/email/)** - Complete API documentation
