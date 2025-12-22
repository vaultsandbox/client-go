---
title: Email API
description: Complete API reference for the Email type and related types
---

The `Email` type represents a decrypted email message in VaultSandbox. All emails are automatically decrypted when retrieved, so you can access content, headers, links, and attachments directly.

## Fields

### ID

```go
ID string
```

Unique identifier for this email. Use this to reference the email in API calls.

#### Example

```go
emails, err := inbox.GetEmails(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Email ID: %s\n", emails[0].ID)

// Get specific email
email, err := inbox.GetEmail(ctx, emails[0].ID)
if err != nil {
    log.Fatal(err)
}
```

---

### From

```go
From string
```

The sender's email address.

#### Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}
fmt.Printf("From: %s\n", email.From)

if email.From != "noreply@example.com" {
    t.Errorf("expected from noreply@example.com, got %s", email.From)
}
```

---

### To

```go
To []string
```

Slice of recipient email addresses.

#### Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}
fmt.Printf("To: %s\n", strings.Join(email.To, ", "))

// Check if specific recipient is included
found := false
for _, addr := range email.To {
    if addr == inbox.EmailAddress() {
        found = true
        break
    }
}
if !found {
    t.Error("inbox address not in recipients")
}
```

---

### Subject

```go
Subject string
```

The email subject line.

#### Example

```go
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Welcome`)),
)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Subject: %s\n", email.Subject)
if !strings.Contains(email.Subject, "Welcome") {
    t.Error("expected subject to contain Welcome")
}
```

---

### Text

```go
Text string
```

Plain text content of the email. May be empty if the email only has HTML content.

#### Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}

if email.Text != "" {
    fmt.Println("Plain text version:")
    fmt.Println(email.Text)

    // Validate content
    if !strings.Contains(email.Text, "Thank you for signing up") {
        t.Error("expected text to contain signup message")
    }
}
```

---

### HTML

```go
HTML string
```

HTML content of the email. May be empty if the email only has plain text.

#### Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}

if email.HTML != "" {
    fmt.Println("HTML version present")

    // Validate HTML structure
    if !strings.Contains(email.HTML, "<a href=") {
        t.Error("expected HTML to contain links")
    }
    if !strings.Contains(email.HTML, "</html>") {
        t.Error("expected valid HTML closing tag")
    }
}
```

---

### ReceivedAt

```go
ReceivedAt time.Time
```

The date and time when the email was received by VaultSandbox.

#### Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Received at: %s\n", email.ReceivedAt.Format(time.RFC3339))

// Check if email was received recently
ageInSeconds := time.Since(email.ReceivedAt).Seconds()
if ageInSeconds > 60 {
    t.Error("email is older than 60 seconds")
}
```

---

### IsRead

```go
IsRead bool
```

Whether this email has been marked as read.

#### Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Read status: %v\n", email.IsRead)

// Mark as read using inbox method
if err := inbox.MarkEmailAsRead(ctx, email.ID); err != nil {
    log.Fatal(err)
}

// Verify status changed
updated, err := inbox.GetEmail(ctx, email.ID)
if err != nil {
    log.Fatal(err)
}
if !updated.IsRead {
    t.Error("expected email to be marked as read")
}
```

---

### Links

```go
Links []string
```

All URLs automatically extracted from the email content (both text and HTML).

#### Example

```go
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Password Reset`)),
)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Found %d links:\n", len(email.Links))
for _, link := range email.Links {
    fmt.Printf("  - %s\n", link)
}

// Find specific link
var resetLink string
for _, link := range email.Links {
    if strings.Contains(link, "/reset-password") {
        resetLink = link
        break
    }
}
if resetLink == "" {
    t.Fatal("reset link not found")
}
if !strings.HasPrefix(resetLink, "https://") {
    t.Error("expected HTTPS link")
}

// Extract query parameters
u, err := url.Parse(resetLink)
if err != nil {
    t.Fatal(err)
}
token := u.Query().Get("token")
if token == "" {
    t.Error("expected token in URL")
}
```

---

### Headers

```go
Headers map[string]string
```

Email headers as a key-value map. Only string values are included.

#### Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}

fmt.Println("Headers:")
fmt.Printf("  Content-Type: %s\n", email.Headers["content-type"])
fmt.Printf("  Message-ID: %s\n", email.Headers["message-id"])

// Check for custom headers
if customHeader, ok := email.Headers["x-custom-header"]; ok {
    fmt.Printf("Custom header: %s\n", customHeader)
}
```

---

### Attachments

```go
Attachments []Attachment
```

Slice of email attachments, automatically decrypted and ready to use.

```go
type Attachment struct {
    Filename           string
    ContentType        string
    Size               int
    ContentID          string
    ContentDisposition string
    Checksum           string
    Content            []byte
}
```

#### Example

```go
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(10*time.Second),
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Invoice`)),
)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Attachments: %d\n", len(email.Attachments))

for _, att := range email.Attachments {
    fmt.Printf("  - %s (%d bytes)\n", att.Filename, att.Size)
    fmt.Printf("    Type: %s\n", att.ContentType)
}

// Find PDF attachment
for _, att := range email.Attachments {
    if att.ContentType == "application/pdf" && len(att.Content) > 0 {
        if err := os.WriteFile("./downloads/"+att.Filename, att.Content, 0644); err != nil {
            log.Fatal(err)
        }
        fmt.Printf("Saved %s\n", att.Filename)
        break
    }
}

// Process text attachment
for _, att := range email.Attachments {
    if strings.Contains(att.ContentType, "text") && len(att.Content) > 0 {
        text := string(att.Content)
        fmt.Printf("Text content: %s\n", text)
        break
    }
}

// Parse JSON attachment
for _, att := range email.Attachments {
    if strings.Contains(att.ContentType, "json") && len(att.Content) > 0 {
        var data map[string]interface{}
        if err := json.Unmarshal(att.Content, &data); err != nil {
            log.Fatal(err)
        }
        fmt.Printf("JSON data: %v\n", data)
        break
    }
}
```

See the [Attachments Guide](/client-go/guides/attachments) for more examples.

---

### AuthResults

```go
AuthResults *authresults.AuthResults
```

Email authentication results including SPF, DKIM, and DMARC validation.

```go
type AuthResults struct {
    SPF        *SPFResult
    DKIM       []DKIMResult
    DMARC      *DMARCResult
    ReverseDNS *ReverseDNSResult
}
```

#### Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}

// Validate all authentication
validation := email.AuthResults.Validate()
fmt.Printf("Authentication passed: %v\n", validation.Passed)

if !validation.Passed {
    fmt.Println("Failures:")
    for _, failure := range validation.Failures {
        fmt.Printf("  - %s\n", failure)
    }
}

// Check individual results
if email.AuthResults.SPF != nil {
    fmt.Printf("SPF Status: %s\n", email.AuthResults.SPF.Status)
}

if len(email.AuthResults.DKIM) > 0 {
    fmt.Printf("DKIM Status: %s\n", email.AuthResults.DKIM[0].Status)
}

if email.AuthResults.DMARC != nil {
    fmt.Printf("DMARC Status: %s\n", email.AuthResults.DMARC.Status)
}
```

See the [Authentication Guide](/client-go/guides/authentication) for more details.

---

## Operations on Emails

`Email` is a pure data struct with no methods. Use `Inbox` methods to perform operations on emails:

- `inbox.GetRawEmail(ctx, emailID)` — Gets raw email source (RFC 5322 format)
- `inbox.MarkEmailAsRead(ctx, emailID)` — Marks email as read
- `inbox.DeleteEmail(ctx, emailID)` — Deletes an email

### Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}

// Mark as read
if err := inbox.MarkEmailAsRead(ctx, email.ID); err != nil {
    log.Fatal(err)
}
fmt.Println("Marked as read")

// Get raw source
raw, err := inbox.GetRawEmail(ctx, email.ID)
if err != nil {
    log.Fatal(err)
}
fmt.Println("Raw MIME source:")
fmt.Println(raw)

// Delete email
if err := inbox.DeleteEmail(ctx, email.ID); err != nil {
    log.Fatal(err)
}
fmt.Println("Email deleted")
```

See [Inbox API Reference](/client-go/api/inbox) for full method documentation.

---

## AuthResults

The `AuthResults` type provides email authentication validation.

### Fields

#### SPF

```go
SPF *SPFResult
```

SPF (Sender Policy Framework) validation result.

```go
type SPFResult struct {
    Status string // "pass", "fail", "softfail", "neutral", "none", "temperror", "permerror"
    Domain string
    IP     string
    Info   string
}
```

#### DKIM

```go
DKIM []DKIMResult
```

DKIM (DomainKeys Identified Mail) validation results. May have multiple signatures.

```go
type DKIMResult struct {
    Status   string // "pass", "fail", "none"
    Domain   string
    Selector string
    Info     string
}
```

#### DMARC

```go
DMARC *DMARCResult
```

DMARC (Domain-based Message Authentication) validation result.

```go
type DMARCResult struct {
    Status  string // "pass", "fail", "none"
    Policy  string // "none", "quarantine", "reject"
    Aligned bool
    Domain  string
    Info    string
}
```

#### ReverseDNS

```go
ReverseDNS *ReverseDNSResult
```

Reverse DNS lookup result.

```go
type ReverseDNSResult struct {
    Status   string // "pass", "fail", "none"
    IP       string
    Hostname string
    Info     string
}
```

### Methods

#### Validate

Validates all authentication results and returns a summary.

```go
func (a *AuthResults) Validate() AuthValidation
```

##### Returns

```go
type AuthValidation struct {
    Passed           bool
    SPFPassed        bool
    DKIMPassed       bool
    DMARCPassed      bool
    ReverseDNSPassed bool
    Failures         []string
}
```

##### Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}

validation := email.AuthResults.Validate()

fmt.Printf("Overall: %s\n", passOrFail(validation.Passed))
fmt.Printf("SPF: %s\n", passOrFail(validation.SPFPassed))
fmt.Printf("DKIM: %s\n", passOrFail(validation.DKIMPassed))
fmt.Printf("DMARC: %s\n", passOrFail(validation.DMARCPassed))

if !validation.Passed {
    fmt.Println("\nFailures:")
    for _, failure := range validation.Failures {
        fmt.Printf("  - %s\n", failure)
    }
}

func passOrFail(passed bool) string {
    if passed {
        return "PASS"
    }
    return "FAIL"
}
```

#### IsPassing

Quick check if all primary authentication checks passed.

```go
func (a *AuthResults) IsPassing() bool
```

##### Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}

if email.AuthResults.IsPassing() {
    fmt.Println("All authentication checks passed")
} else {
    fmt.Println("Some authentication checks failed")
}
```

---

## Complete Example

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "regexp"
    "strings"
    "time"

    "github.com/vaultsandbox/client-go"
)

func main() {
    ctx := context.Background()

    client, err := vaultsandbox.New(
        os.Getenv("VAULTSANDBOX_API_KEY"),
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
    fmt.Printf("Created inbox: %s\n", inbox.EmailAddress())

    // Trigger test email
    sendTestEmail(inbox.EmailAddress())

    // Wait for email
    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(10*time.Second),
        vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Test`)),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Basic info
    fmt.Println("\n=== Email Details ===")
    fmt.Printf("ID: %s\n", email.ID)
    fmt.Printf("From: %s\n", email.From)
    fmt.Printf("To: %s\n", strings.Join(email.To, ", "))
    fmt.Printf("Subject: %s\n", email.Subject)
    fmt.Printf("Received: %s\n", email.ReceivedAt.Format(time.RFC3339))
    fmt.Printf("Read: %v\n", email.IsRead)

    // Content
    fmt.Println("\n=== Content ===")
    if email.Text != "" {
        fmt.Println("Plain text:")
        if len(email.Text) > 200 {
            fmt.Printf("%s...\n", email.Text[:200])
        } else {
            fmt.Println(email.Text)
        }
    }
    if email.HTML != "" {
        fmt.Println("HTML version present")
    }

    // Links
    fmt.Println("\n=== Links ===")
    fmt.Printf("Found %d links:\n", len(email.Links))
    for _, link := range email.Links {
        fmt.Printf("  - %s\n", link)
    }

    // Attachments
    fmt.Println("\n=== Attachments ===")
    fmt.Printf("Found %d attachments:\n", len(email.Attachments))
    for _, att := range email.Attachments {
        fmt.Printf("  - %s (%s, %d bytes)\n", att.Filename, att.ContentType, att.Size)

        // Save attachment
        if len(att.Content) > 0 {
            path := "./downloads/" + att.Filename
            if err := os.WriteFile(path, att.Content, 0644); err != nil {
                log.Printf("Failed to save %s: %v\n", att.Filename, err)
            } else {
                fmt.Printf("    Saved to %s\n", path)
            }
        }
    }

    // Authentication
    fmt.Println("\n=== Authentication ===")
    validation := email.AuthResults.Validate()
    fmt.Printf("Overall: %s\n", boolToPassFail(validation.Passed))
    fmt.Printf("SPF: %v\n", validation.SPFPassed)
    fmt.Printf("DKIM: %v\n", validation.DKIMPassed)
    fmt.Printf("DMARC: %v\n", validation.DMARCPassed)

    if !validation.Passed {
        fmt.Printf("Failures: %v\n", validation.Failures)
    }

    // Mark as read
    if err := inbox.MarkEmailAsRead(ctx, email.ID); err != nil {
        log.Fatal(err)
    }
    fmt.Println("\nMarked as read")

    // Get raw source
    raw, err := inbox.GetRawEmail(ctx, email.ID)
    if err != nil {
        log.Fatal(err)
    }
    filename := fmt.Sprintf("email-%s.eml", email.ID)
    if err := os.WriteFile(filename, []byte(raw), 0644); err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Saved raw source to %s\n", filename)

    // Clean up
    if err := inbox.Delete(ctx); err != nil {
        log.Fatal(err)
    }
}

func boolToPassFail(b bool) string {
    if b {
        return "PASS"
    }
    return "FAIL"
}

func sendTestEmail(address string) {
    // Your email sending logic here
}
```

## Next Steps

- [Inbox API Reference](/client-go/api/inbox) - Learn about inbox methods
- [Attachments Guide](/client-go/guides/attachments) - Working with attachments
- [Authentication Guide](/client-go/guides/authentication) - Email authentication testing
- [Waiting for Emails](/client-go/guides/waiting-for-emails) - Best practices for email waiting
