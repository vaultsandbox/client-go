# 04 - Data Types

## Overview

This document defines all public data types exposed by the SDK.

## Email Types

### Email

```go
// Email represents a decrypted email message
type Email struct {
    // Unique identifier
    ID string

    // Sender address (from encryptedMetadata)
    From string

    // Recipient addresses (from encryptedMetadata)
    To []string

    // Email subject (from encryptedMetadata)
    Subject string

    // Plain text body (from encryptedParsed)
    Text string

    // HTML body (from encryptedParsed)
    HTML string

    // When the email was received by the server
    ReceivedAt time.Time

    // Email headers (from encryptedParsed)
    Headers map[string]string

    // File attachments (from encryptedParsed)
    Attachments []Attachment

    // URLs extracted from email body (from encryptedParsed)
    Links []string

    // Authentication results (from encryptedParsed)
    AuthResults *authresults.AuthResults

    // Whether the email has been marked as read
    IsRead bool

    // Reference to parent inbox for operations
    inbox *Inbox
}

// GetRaw retrieves the raw email source (encrypted then decrypted)
func (e *Email) GetRaw(ctx context.Context) (string, error) {
    // Fetch from API, decrypt, return
}

// MarkAsRead marks this email as read
func (e *Email) MarkAsRead(ctx context.Context) error {
    // PATCH /api/inboxes/{email}/emails/{id}/read
}

// Delete removes this email from the inbox
func (e *Email) Delete(ctx context.Context) error {
    // DELETE /api/inboxes/{email}/emails/{id}
}
```

### Attachment

```go
// Attachment represents an email attachment
type Attachment struct {
    // Original filename
    Filename string

    // MIME type (e.g., "application/pdf")
    ContentType string

    // Size in bytes
    Size int

    // Content-ID for inline attachments
    ContentID string

    // Disposition: "attachment" or "inline"
    ContentDisposition string

    // Raw attachment bytes (decoded from base64)
    Content []byte

    // Optional SHA-256 checksum
    Checksum string
}
```

### Decrypted JSON Structures

```go
// emailMetadata is the decrypted encryptedMetadata
type emailMetadata struct {
    From       string    `json:"from"`
    To         []string  `json:"to"`
    Subject    string    `json:"subject"`
    ReceivedAt time.Time `json:"receivedAt"`
}

// emailContent is the decrypted encryptedParsed
type emailContent struct {
    Text        string                   `json:"text"`
    HTML        string                   `json:"html"`
    Headers     map[string]string        `json:"headers"`
    Attachments []attachmentJSON         `json:"attachments"`
    Links       []string                 `json:"links"`
    AuthResults *authresults.AuthResults `json:"authResults"`
    Metadata    map[string]any           `json:"metadata"`
}

type attachmentJSON struct {
    Filename           string `json:"filename"`
    ContentType        string `json:"contentType"`
    Size               int    `json:"size"`
    ContentID          string `json:"contentId"`
    ContentDisposition string `json:"contentDisposition"`
    Content            string `json:"content"` // base64 encoded
    Checksum           string `json:"checksum"`
}
```

## Inbox Types

### Inbox

```go
// Inbox represents a temporary email inbox
type Inbox struct {
    emailAddress string
    expiresAt    time.Time
    inboxHash    string
    serverSigPk  []byte
    keypair      *crypto.Keypair
    client       *Client
}

// EmailAddress returns the inbox email address
func (i *Inbox) EmailAddress() string

// ExpiresAt returns when this inbox expires
func (i *Inbox) ExpiresAt() time.Time

// InboxHash returns the SHA-256 hash of the public key
func (i *Inbox) InboxHash() string

// IsExpired checks if the inbox has expired
func (i *Inbox) IsExpired() bool

// GetEmails retrieves all emails in the inbox
func (i *Inbox) GetEmails(ctx context.Context) ([]*Email, error)

// GetEmail retrieves a specific email by ID
func (i *Inbox) GetEmail(ctx context.Context, emailID string) (*Email, error)

// WaitForEmail waits for an email matching the filters
func (i *Inbox) WaitForEmail(ctx context.Context, opts ...WaitOption) (*Email, error)

// WaitForEmailCount waits until N emails are in the inbox
func (i *Inbox) WaitForEmailCount(ctx context.Context, count int, opts ...WaitOption) ([]*Email, error)

// Delete removes this inbox from the server
func (i *Inbox) Delete(ctx context.Context) error

// Export returns exportable inbox data including private key
func (i *Inbox) Export() *ExportedInbox
```

### ExportedInbox

```go
// ExportedInbox contains all data needed to restore an inbox
// WARNING: Contains private key material - handle securely
type ExportedInbox struct {
    EmailAddress string    `json:"emailAddress"`
    ExpiresAt    time.Time `json:"expiresAt"`
    InboxHash    string    `json:"inboxHash"`
    ServerSigPk  string    `json:"serverSigPk"`
    PublicKeyB64 string    `json:"publicKeyB64"`
    SecretKeyB64 string    `json:"secretKeyB64"`
    ExportedAt   time.Time `json:"exportedAt"`
}

// Validate checks that the exported data is valid
func (e *ExportedInbox) Validate() error {
    if e.EmailAddress == "" {
        return ErrInvalidImportData
    }
    if e.SecretKeyB64 == "" {
        return ErrInvalidImportData
    }
    // Validate key sizes after decoding
    secretKey, err := crypto.FromBase64URL(e.SecretKeyB64)
    if err != nil || len(secretKey) != crypto.MLKEMSecretKeySize {
        return ErrInvalidImportData
    }
    return nil
}
```

## Authentication Result Types

Located in `authresults` subpackage:

```go
package authresults

// AuthResults contains all email authentication check results
type AuthResults struct {
    SPF        *SPFResult        `json:"spf,omitempty"`
    DKIM       []DKIMResult      `json:"dkim,omitempty"`
    DMARC      *DMARCResult      `json:"dmarc,omitempty"`
    ReverseDNS *ReverseDNSResult `json:"reverseDns,omitempty"`
}

// SPFResult represents an SPF check result
type SPFResult struct {
    Status string `json:"status"` // pass, fail, softfail, neutral, none, temperror, permerror
    Domain string `json:"domain,omitempty"`
    IP     string `json:"ip,omitempty"`
    Info   string `json:"info,omitempty"`
}

// DKIMResult represents a DKIM check result
type DKIMResult struct {
    Status   string `json:"status"` // pass, fail, none
    Domain   string `json:"domain,omitempty"`
    Selector string `json:"selector,omitempty"`
    Info     string `json:"info,omitempty"`
}

// DMARCResult represents a DMARC check result
type DMARCResult struct {
    Status  string `json:"status"` // pass, fail, none
    Policy  string `json:"policy,omitempty"` // none, quarantine, reject
    Aligned bool   `json:"aligned,omitempty"`
    Domain  string `json:"domain,omitempty"`
    Info    string `json:"info,omitempty"`
}

// ReverseDNSResult represents a reverse DNS check result
type ReverseDNSResult struct {
    Status   string `json:"status"` // pass, fail, none
    IP       string `json:"ip,omitempty"`
    Hostname string `json:"hostname,omitempty"`
    Info     string `json:"info,omitempty"`
}
```

### Validation Helper

```go
// Validate checks that all authentication results pass
func Validate(results *AuthResults) error {
    if results == nil {
        return ErrNoAuthResults
    }

    var errs []string

    // SPF must pass
    if results.SPF == nil || results.SPF.Status != "pass" {
        errs = append(errs, "SPF did not pass")
    }

    // At least one DKIM must pass
    dkimPassed := false
    for _, dkim := range results.DKIM {
        if dkim.Status == "pass" {
            dkimPassed = true
            break
        }
    }
    if !dkimPassed {
        errs = append(errs, "no DKIM signature passed")
    }

    // DMARC must pass
    if results.DMARC == nil || results.DMARC.Status != "pass" {
        errs = append(errs, "DMARC did not pass")
    }

    // ReverseDNS must pass if present
    if results.ReverseDNS != nil && results.ReverseDNS.Status != "pass" {
        errs = append(errs, "reverse DNS did not pass")
    }

    if len(errs) > 0 {
        return &ValidationError{Errors: errs}
    }

    return nil
}
```

## Configuration Types

### WaitOptions

```go
// WaitConfig holds options for WaitForEmail
type WaitConfig struct {
    Subject       string         // Exact subject match
    SubjectRegex  *regexp.Regexp // Regex subject match
    From          string         // Exact from match
    FromRegex     *regexp.Regexp // Regex from match
    Predicate     func(*Email) bool // Custom filter
    Timeout       time.Duration  // Max wait time (default: 30s)
    PollInterval  time.Duration  // Poll interval for polling strategy
}

// WaitOption configures WaitForEmail behavior
type WaitOption func(*WaitConfig)

func WithSubject(subject string) WaitOption {
    return func(c *WaitConfig) { c.Subject = subject }
}

func WithSubjectRegex(pattern *regexp.Regexp) WaitOption {
    return func(c *WaitConfig) { c.SubjectRegex = pattern }
}

func WithFrom(from string) WaitOption {
    return func(c *WaitConfig) { c.From = from }
}

func WithFromRegex(pattern *regexp.Regexp) WaitOption {
    return func(c *WaitConfig) { c.FromRegex = pattern }
}

func WithPredicate(fn func(*Email) bool) WaitOption {
    return func(c *WaitConfig) { c.Predicate = fn }
}

func WithWaitTimeout(timeout time.Duration) WaitOption {
    return func(c *WaitConfig) { c.Timeout = timeout }
}

func WithPollInterval(interval time.Duration) WaitOption {
    return func(c *WaitConfig) { c.PollInterval = interval }
}
```

### InboxOptions

```go
// InboxConfig holds options for CreateInbox
type InboxConfig struct {
    TTL          time.Duration // Inbox lifetime (default: 1 hour)
    EmailAddress string        // Desired email or domain
}

// InboxOption configures inbox creation
type InboxOption func(*InboxConfig)

func WithTTL(ttl time.Duration) InboxOption {
    return func(c *InboxConfig) { c.TTL = ttl }
}

func WithEmailAddress(email string) InboxOption {
    return func(c *InboxConfig) { c.EmailAddress = email }
}
```

## Email Processing Pipeline

**Important:** The `GET /api/inboxes/{email}/emails` endpoint returns only encrypted metadata (from, to, subject, date). To retrieve full email content (text, HTML, attachments), the client must fetch each email individually using `GET /api/inboxes/{email}/emails/{id}`.

The SDK handles this transparently:
- `GetEmails()` returns emails with metadata only (fast, for listing)
- `GetEmail(id)` returns the full email with content (fetches individually)

```go
// processEmail decrypts and constructs an Email from raw API data
// If raw.EncryptedParsed is nil (list endpoint), only metadata fields are populated.
// Call GetEmail(id) to fetch full content including body and attachments.
func (i *Inbox) processEmail(ctx context.Context, raw *api.RawEmail) (*Email, error) {
    // 1. Verify and decrypt metadata (always present)
    if err := crypto.VerifySignature(raw.EncryptedMetadata); err != nil {
        return nil, err
    }
    metadataJSON, err := crypto.Decrypt(raw.EncryptedMetadata, i.keypair)
    if err != nil {
        return nil, err
    }

    var metadata emailMetadata
    if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
        return nil, err
    }

    email := &Email{
        ID:         raw.ID,
        From:       metadata.From,
        To:         metadata.To,
        Subject:    metadata.Subject,
        ReceivedAt: metadata.ReceivedAt,
        IsRead:     raw.IsRead,
        inbox:      i,
    }

    // 2. Decrypt parsed content if available (only from GetEmail, not from list)
    // The list endpoint (GET /emails) does NOT include encryptedParsed.
    // The single email endpoint (GET /emails/{id}) includes encryptedParsed.
    if raw.EncryptedParsed != nil {
        if err := crypto.VerifySignature(raw.EncryptedParsed); err != nil {
            return nil, err
        }
        contentJSON, err := crypto.Decrypt(raw.EncryptedParsed, i.keypair)
        if err != nil {
            return nil, err
        }

        var content emailContent
        if err := json.Unmarshal(contentJSON, &content); err != nil {
            return nil, err
        }

        email.Text = content.Text
        email.HTML = content.HTML
        email.Headers = content.Headers
        email.Links = content.Links
        email.AuthResults = content.AuthResults

        // Decode attachments from base64
        for _, att := range content.Attachments {
            decoded, err := base64.StdEncoding.DecodeString(att.Content)
            if err != nil {
                return nil, fmt.Errorf("decode attachment: %w", err)
            }
            email.Attachments = append(email.Attachments, Attachment{
                Filename:           att.Filename,
                ContentType:        att.ContentType,
                Size:               att.Size,
                ContentID:          att.ContentID,
                ContentDisposition: att.ContentDisposition,
                Content:            decoded,
                Checksum:           att.Checksum,
            })
        }
    }

    return email, nil
}

// GetEmails retrieves all emails in the inbox (metadata only for efficiency)
func (i *Inbox) GetEmails(ctx context.Context) ([]*Email, error) {
    rawEmails, err := i.client.apiClient.GetEmails(ctx, i.emailAddress)
    if err != nil {
        return nil, err
    }

    emails := make([]*Email, 0, len(rawEmails))
    for _, raw := range rawEmails {
        // Note: raw.EncryptedParsed will be nil from list endpoint
        email, err := i.processEmail(ctx, &raw)
        if err != nil {
            return nil, err
        }
        emails = append(emails, email)
    }
    return emails, nil
}

// GetEmail retrieves a specific email with full content (body, attachments)
func (i *Inbox) GetEmail(ctx context.Context, emailID string) (*Email, error) {
    raw, err := i.client.apiClient.GetEmail(ctx, i.emailAddress, emailID)
    if err != nil {
        return nil, err
    }
    // Note: raw.EncryptedParsed will be populated from single email endpoint
    return i.processEmail(ctx, raw)
}
```
