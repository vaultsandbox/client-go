package vaultsandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/vaultsandbox/client-go/authresults"
	"github.com/vaultsandbox/client-go/internal/api"
	"github.com/vaultsandbox/client-go/internal/crypto"
)

// Inbox represents a temporary email inbox.
type Inbox struct {
	emailAddress string
	expiresAt    time.Time
	inboxHash    string
	serverSigPk  []byte
	keypair      *crypto.Keypair
	client       *Client
}

// SyncStatus represents the synchronization status of an inbox.
type SyncStatus struct {
	// EmailCount is the number of emails in the inbox.
	EmailCount int
	// EmailsHash is a hash of the email list for efficient change detection.
	EmailsHash string
}

// ExportedInbox contains all data needed to restore an inbox.
// WARNING: Contains private key material - handle securely.
type ExportedInbox struct {
	EmailAddress string    `json:"emailAddress"`
	ExpiresAt    time.Time `json:"expiresAt"`
	InboxHash    string    `json:"inboxHash"`
	ServerSigPk  string    `json:"serverSigPk"`
	PublicKeyB64 string    `json:"publicKeyB64"`
	SecretKeyB64 string    `json:"secretKeyB64"`
	ExportedAt   time.Time `json:"exportedAt"`
}

// Validate checks that the exported data is valid.
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

// EmailAddress returns the inbox email address.
func (i *Inbox) EmailAddress() string {
	return i.emailAddress
}

// ExpiresAt returns when the inbox expires.
func (i *Inbox) ExpiresAt() time.Time {
	return i.expiresAt
}

// InboxHash returns the SHA-256 hash of the public key.
func (i *Inbox) InboxHash() string {
	return i.inboxHash
}

// IsExpired checks if the inbox has expired.
func (i *Inbox) IsExpired() bool {
	return time.Now().After(i.expiresAt)
}

// GetSyncStatus retrieves the synchronization status of the inbox.
// This includes the number of emails and a hash of the email list,
// which can be used to efficiently check for changes.
func (i *Inbox) GetSyncStatus(ctx context.Context) (*SyncStatus, error) {
	status, err := i.client.apiClient.GetInboxSync(ctx, i.emailAddress)
	if err != nil {
		return nil, wrapError(err)
	}
	return &SyncStatus{
		EmailCount: status.EmailCount,
		EmailsHash: status.EmailsHash,
	}, nil
}

// GetEmails fetches all emails in the inbox.
func (i *Inbox) GetEmails(ctx context.Context) ([]*Email, error) {
	resp, err := i.client.apiClient.GetEmails(ctx, i.emailAddress)
	if err != nil {
		return nil, wrapError(err)
	}

	emails := make([]*Email, 0, len(resp.Emails))
	for _, e := range resp.Emails {
		email, err := i.decryptEmail(e)
		if err != nil {
			return nil, err
		}
		emails = append(emails, email)
	}

	return emails, nil
}

// GetEmail fetches a specific email by ID.
func (i *Inbox) GetEmail(ctx context.Context, emailID string) (*Email, error) {
	resp, err := i.client.apiClient.GetEmail(ctx, i.emailAddress, emailID)
	if err != nil {
		return nil, wrapError(err)
	}

	return i.decryptEmail(resp)
}

// WaitForEmail waits for an email matching the given criteria.
// It uses the client's callback infrastructure to receive instant notifications
// when SSE is active, or receives events when the polling handler fires.
func (i *Inbox) WaitForEmail(ctx context.Context, opts ...WaitOption) (*Email, error) {
	cfg := &waitConfig{
		timeout:      defaultWaitTimeout,
		pollInterval: defaultPollInterval,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	resultCh := make(chan *Email, 1)

	// 1. Subscribe FIRST to avoid race condition
	sub := i.OnNewEmail(func(email *Email) {
		if cfg.Matches(email) {
			select {
			case resultCh <- email:
			default: // already found
			}
		}
	})
	defer sub.Unsubscribe()

	// 2. Check existing emails (handles already-arrived case)
	emails, err := i.GetEmails(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range emails {
		if cfg.Matches(e) {
			return e, nil
		}
	}

	// 3. Wait for callback or timeout
	select {
	case email := <-resultCh:
		return email, nil
	case <-ctx.Done():
		return nil, ErrEmailNotFound
	}
}

// WaitForEmailCount waits until at least count matching emails are found.
// It uses the client's callback infrastructure to receive instant notifications
// when SSE is active, or receives events when the polling handler fires.
func (i *Inbox) WaitForEmailCount(ctx context.Context, count int, opts ...WaitOption) ([]*Email, error) {
	cfg := &waitConfig{
		timeout:      defaultWaitTimeout,
		pollInterval: defaultPollInterval,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	// Use a mutex to protect concurrent access to the results slice
	var mu sync.Mutex
	var results []*Email
	doneCh := make(chan struct{})

	// 1. Subscribe FIRST to avoid race condition
	sub := i.OnNewEmail(func(email *Email) {
		if cfg.Matches(email) {
			mu.Lock()
			// Check if we already have this email (by ID)
			for _, e := range results {
				if e.ID == email.ID {
					mu.Unlock()
					return
				}
			}
			results = append(results, email)
			if len(results) >= count {
				select {
				case doneCh <- struct{}{}:
				default:
				}
			}
			mu.Unlock()
		}
	})
	defer sub.Unsubscribe()

	// 2. Check existing emails (handles already-arrived case)
	emails, err := i.GetEmails(ctx)
	if err != nil {
		return nil, err
	}
	mu.Lock()
	for _, e := range emails {
		if cfg.Matches(e) {
			results = append(results, e)
		}
	}
	if len(results) >= count {
		matched := results[:count]
		mu.Unlock()
		return matched, nil
	}
	mu.Unlock()

	// 3. Wait for callbacks or timeout
	for {
		select {
		case <-doneCh:
			mu.Lock()
			if len(results) >= count {
				matched := results[:count]
				mu.Unlock()
				return matched, nil
			}
			mu.Unlock()
		case <-ctx.Done():
			return nil, ErrEmailNotFound
		}
	}
}

// Delete deletes the inbox.
func (i *Inbox) Delete(ctx context.Context) error {
	return i.client.DeleteInbox(ctx, i.emailAddress)
}

// GetRawEmail fetches the raw email content for a specific email.
func (i *Inbox) GetRawEmail(ctx context.Context, emailID string) (string, error) {
	raw, err := i.client.apiClient.GetEmailRaw(ctx, i.emailAddress, emailID)
	if err != nil {
		return "", wrapError(err)
	}
	return raw, nil
}

// MarkEmailAsRead marks a specific email as read.
func (i *Inbox) MarkEmailAsRead(ctx context.Context, emailID string) error {
	return wrapError(i.client.apiClient.MarkEmailAsRead(ctx, i.emailAddress, emailID))
}

// DeleteEmail deletes a specific email.
func (i *Inbox) DeleteEmail(ctx context.Context, emailID string) error {
	return wrapError(i.client.apiClient.DeleteEmail(ctx, i.emailAddress, emailID))
}

// Export returns exportable inbox data including private key.
func (i *Inbox) Export() *ExportedInbox {
	return &ExportedInbox{
		EmailAddress: i.emailAddress,
		ExpiresAt:    i.expiresAt,
		InboxHash:    i.inboxHash,
		ServerSigPk:  crypto.ToBase64URL(i.serverSigPk),
		PublicKeyB64: crypto.ToBase64URL(i.keypair.PublicKey),
		SecretKeyB64: crypto.ToBase64URL(i.keypair.SecretKey),
		ExportedAt:   time.Now(),
	}
}

// InboxEmailCallback is called when a new email arrives in the inbox.
type InboxEmailCallback func(email *Email)

// OnNewEmail subscribes to new email notifications for this inbox.
// The callback is invoked whenever a new email arrives.
// Returns a Subscription that can be used to unsubscribe.
//
// This method uses the client's delivery strategy (SSE, polling, or auto)
// for real-time email notifications. With SSE enabled, emails are delivered
// instantly as push notifications.
//
// Example:
//
//	subscription := inbox.OnNewEmail(func(email *Email) {
//	    fmt.Printf("New email: %s\n", email.Subject)
//	})
//	defer subscription.Unsubscribe()
func (i *Inbox) OnNewEmail(callback InboxEmailCallback) Subscription {
	// Register callback with the client's event system
	index := i.client.registerEmailCallback(i.inboxHash, func(inbox *Inbox, email *Email) {
		callback(email)
	})

	return &inboxEmailSubscription{
		inbox:         i,
		callbackIndex: index,
	}
}

// inboxEmailSubscription implements Subscription for single inbox monitoring.
type inboxEmailSubscription struct {
	inbox         *Inbox
	callbackIndex int
}

func (s *inboxEmailSubscription) Unsubscribe() {
	if s.inbox != nil && s.inbox.client != nil {
		s.inbox.client.unregisterEmailCallback(s.inbox.inboxHash, s.callbackIndex)
	}
}

func newInboxFromResult(resp *api.CreateInboxResult, c *Client) *Inbox {
	return &Inbox{
		emailAddress: resp.EmailAddress,
		expiresAt:    resp.ExpiresAt,
		inboxHash:    resp.InboxHash,
		serverSigPk:  resp.ServerSigPk,
		keypair:      resp.Keypair,
		client:       c,
	}
}

func newInboxFromExport(data *ExportedInbox, c *Client) (*Inbox, error) {
	if err := data.Validate(); err != nil {
		return nil, err
	}

	secretKey, err := crypto.FromBase64URL(data.SecretKeyB64)
	if err != nil {
		return nil, ErrInvalidImportData
	}

	publicKey, err := crypto.FromBase64URL(data.PublicKeyB64)
	if err != nil {
		return nil, ErrInvalidImportData
	}

	serverSigPk, err := crypto.FromBase64URL(data.ServerSigPk)
	if err != nil {
		return nil, ErrInvalidImportData
	}

	keypair, err := crypto.NewKeypairFromBytes(secretKey, publicKey)
	if err != nil {
		return nil, err
	}

	return &Inbox{
		emailAddress: data.EmailAddress,
		expiresAt:    data.ExpiresAt,
		inboxHash:    data.InboxHash,
		serverSigPk:  serverSigPk,
		keypair:      keypair,
		client:       c,
	}, nil
}

func (i *Inbox) decryptEmail(raw *api.RawEmail) (*Email, error) {
	return i.decryptEmailWithContext(context.Background(), raw)
}

func (i *Inbox) decryptEmailWithContext(ctx context.Context, raw *api.RawEmail) (*Email, error) {
	if raw.EncryptedMetadata == nil {
		return nil, fmt.Errorf("email has no encrypted metadata")
	}

	// Fetch full email if we don't have parsed content
	emailData := raw
	if raw.EncryptedParsed == nil {
		fullEmail, err := i.client.apiClient.GetEmail(ctx, i.emailAddress, raw.ID)
		if err != nil {
			return nil, fmt.Errorf("fetch full email: %w", wrapError(err))
		}
		emailData = fullEmail
	}

	// Verify and decrypt metadata
	metadataPlaintext, err := i.verifyAndDecrypt(emailData.EncryptedMetadata)
	if err != nil {
		return nil, err
	}

	metadata, err := parseMetadata(metadataPlaintext)
	if err != nil {
		return nil, err
	}

	// Build decrypted email from metadata
	decrypted := buildDecryptedEmail(emailData, metadata)

	// Decrypt and apply parsed content if available
	if emailData.EncryptedParsed != nil {
		if err := i.applyParsedContent(emailData.EncryptedParsed, decrypted); err != nil {
			return nil, err
		}
	}

	return i.convertDecryptedEmail(decrypted), nil
}

// applyParsedContent decrypts parsed content and applies it to the decrypted email.
func (i *Inbox) applyParsedContent(encrypted *crypto.EncryptedPayload, decrypted *crypto.DecryptedEmail) error {
	parsedPlaintext, err := i.verifyAndDecrypt(encrypted)
	if err != nil {
		return err
	}

	parsed, headers, err := parseParsedContent(parsedPlaintext)
	if err != nil {
		return err
	}

	decrypted.Text = parsed.Text
	decrypted.HTML = parsed.HTML
	decrypted.Attachments = parsed.Attachments
	decrypted.Links = parsed.Links
	decrypted.AuthResults = parsed.AuthResults
	decrypted.Headers = headers

	return nil
}

func (i *Inbox) convertDecryptedEmail(d *crypto.DecryptedEmail) *Email {
	attachments := make([]Attachment, len(d.Attachments))
	for j, a := range d.Attachments {
		attachments[j] = Attachment{
			Filename:           a.Filename,
			ContentType:        a.ContentType,
			Size:               a.Size,
			ContentID:          a.ContentID,
			ContentDisposition: a.ContentDisposition,
			Content:            a.Content,
			Checksum:           a.Checksum,
		}
	}

	email := &Email{
		ID:          d.ID,
		From:        d.From,
		To:          d.To,
		Subject:     d.Subject,
		Text:        d.Text,
		HTML:        d.HTML,
		ReceivedAt:  d.ReceivedAt,
		Headers:     d.Headers,
		Attachments: attachments,
		Links:       d.Links,
		IsRead:      d.IsRead,
	}

	// Unmarshal AuthResults if present
	if len(d.AuthResults) > 0 {
		var ar authresults.AuthResults
		if err := json.Unmarshal(d.AuthResults, &ar); err == nil {
			email.AuthResults = &ar
		}
	}

	return email
}

// verifyAndDecrypt verifies the signature and decrypts an encrypted payload.
// It returns the decrypted plaintext or an error if verification/decryption fails.
func (i *Inbox) verifyAndDecrypt(payload *crypto.EncryptedPayload) ([]byte, error) {
	if err := crypto.VerifySignature(payload, i.serverSigPk); err != nil {
		return nil, wrapCryptoError(err)
	}
	return crypto.Decrypt(payload, i.keypair)
}

// parseMetadata unmarshals decrypted metadata JSON into a DecryptedMetadata struct.
func parseMetadata(plaintext []byte) (*crypto.DecryptedMetadata, error) {
	var metadata crypto.DecryptedMetadata
	if err := json.Unmarshal(plaintext, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted metadata: %w", err)
	}
	return &metadata, nil
}

// parseParsedContent unmarshals decrypted parsed content JSON and converts headers.
// Headers are converted from interface{} to string map, preserving only string values.
func parseParsedContent(plaintext []byte) (*crypto.DecryptedParsed, map[string]string, error) {
	var parsed crypto.DecryptedParsed
	if err := json.Unmarshal(plaintext, &parsed); err != nil {
		return nil, nil, fmt.Errorf("failed to parse decrypted parsed content: %w", err)
	}

	// Convert headers from interface{} to string map.
	// The server may send headers with non-string values, but for type safety
	// we only preserve string-typed values.
	var headers map[string]string
	if len(parsed.Headers) > 0 {
		headers = make(map[string]string)
		for k, v := range parsed.Headers {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
	}

	return &parsed, headers, nil
}

// buildDecryptedEmail constructs a DecryptedEmail from raw email data and metadata.
// It handles receivedAt fallback logic when metadata timestamp is missing or invalid.
func buildDecryptedEmail(emailData *api.RawEmail, metadata *crypto.DecryptedMetadata) *crypto.DecryptedEmail {
	decrypted := &crypto.DecryptedEmail{
		ID:      emailData.ID,
		From:    metadata.From,
		To:      []string{metadata.To},
		Subject: metadata.Subject,
		IsRead:  emailData.IsRead,
	}

	// Parse receivedAt from metadata, fallback to API timestamp
	if metadata.ReceivedAt != "" {
		if t, err := time.Parse(time.RFC3339, metadata.ReceivedAt); err == nil {
			decrypted.ReceivedAt = t
		}
	}
	if decrypted.ReceivedAt.IsZero() {
		decrypted.ReceivedAt = emailData.ReceivedAt
	}

	return decrypted
}

// wrapCryptoError converts internal crypto errors to public sentinel errors
// so that errors.Is() checks work correctly.
func wrapCryptoError(err error) error {
	if err == nil {
		return nil
	}

	// Map internal crypto errors to public sentinel errors
	if errors.Is(err, crypto.ErrServerKeyMismatch) {
		return &SignatureVerificationError{Message: err.Error(), IsKeyMismatch: true}
	}
	if errors.Is(err, crypto.ErrSignatureVerificationFailed) {
		return &SignatureVerificationError{Message: err.Error(), IsKeyMismatch: false}
	}

	return err
}
