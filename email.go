package vaultsandbox

import (
	"context"
	"time"

	"github.com/vaultsandbox/client-go/authresults"
)

// Email represents a decrypted email.
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
	Headers     map[string]string
	Attachments []Attachment
	Links       []string
	AuthResults *authresults.AuthResults
	IsRead      bool

	inbox *Inbox
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

// GetRaw fetches the raw email content.
func (e *Email) GetRaw(ctx context.Context) (string, error) {
	return e.inbox.client.apiClient.GetEmailRaw(ctx, e.inbox.inboxHash, e.ID)
}

// MarkAsRead marks the email as read.
func (e *Email) MarkAsRead(ctx context.Context) error {
	if err := e.inbox.client.apiClient.MarkEmailAsRead(ctx, e.inbox.inboxHash, e.ID); err != nil {
		return err
	}
	e.IsRead = true
	return nil
}

// Delete deletes the email.
func (e *Email) Delete(ctx context.Context) error {
	return e.inbox.client.apiClient.DeleteEmail(ctx, e.inbox.inboxHash, e.ID)
}
