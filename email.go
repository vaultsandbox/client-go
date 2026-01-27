package vaultsandbox

import (
	"time"

	"github.com/vaultsandbox/client-go/authresults"
	"github.com/vaultsandbox/client-go/spamanalysis"
)

// Email represents a decrypted email.
// Email is a pure data struct with no methods that require API calls.
// Use Inbox methods to perform operations on emails:
//   - inbox.GetRawEmail(ctx, emailID) — Gets raw email source
//   - inbox.MarkEmailAsRead(ctx, emailID) — Marks email as read
//   - inbox.DeleteEmail(ctx, emailID) — Deletes an email
type Email struct {
	ID          string
	From        string
	To          []string
	Subject     string
	Text        string
	HTML        string
	ReceivedAt  time.Time
	// Headers contains email headers as string key-value pairs.
	// Non-string header values from the server are omitted during parsing.
	Headers      map[string]string
	Attachments  []Attachment
	Links        []string
	AuthResults  *authresults.AuthResults
	SpamAnalysis *spamanalysis.SpamAnalysis
	IsRead       bool

	// AuthResultsError contains any error that occurred parsing auth results.
	// This is set instead of AuthResults if parsing failed.
	AuthResultsError error `json:"-"`

	// SpamAnalysisError contains any error that occurred parsing spam analysis.
	// This is set instead of SpamAnalysis if parsing failed.
	SpamAnalysisError error `json:"-"`
}

// Attachment represents an email attachment.
type Attachment struct {
	Filename           string
	ContentType        string
	Size               int
	ContentID          string
	ContentDisposition string
	Content            []byte
	Checksum           string
}

// EmailMetadata represents email metadata without full content.
// Use this for efficient email list displays when you don't need body/attachments.
type EmailMetadata struct {
	ID         string
	From       string
	Subject    string
	ReceivedAt time.Time
	IsRead     bool
}
