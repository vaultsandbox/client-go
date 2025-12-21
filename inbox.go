package vaultsandbox

import (
	"context"
	"time"

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

// GetEmails fetches all emails in the inbox.
func (i *Inbox) GetEmails(ctx context.Context) ([]*Email, error) {
	resp, err := i.client.apiClient.GetEmails(ctx, i.emailAddress)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	return i.decryptEmail(resp)
}

// WaitForEmail waits for an email matching the given criteria.
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

	// Create fetcher that returns emails as interface{}
	fetcher := func(ctx context.Context) ([]interface{}, error) {
		emails, err := i.GetEmails(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]interface{}, len(emails))
		for j, e := range emails {
			result[j] = e
		}
		return result, nil
	}

	// Create matcher that handles interface{} and delegates to waitConfig
	matcher := func(email interface{}) bool {
		e, ok := email.(*Email)
		if !ok {
			return false
		}
		return cfg.Matches(e)
	}

	result, err := i.client.strategy.WaitForEmail(ctx, i.inboxHash, fetcher, matcher, cfg.pollInterval)
	if err != nil {
		return nil, err
	}

	email, ok := result.(*Email)
	if !ok {
		return nil, ErrEmailNotFound
	}

	return email, nil
}

// WaitForEmailCount waits until the inbox has at least count emails.
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

	// Create fetcher that returns emails as interface{}
	fetcher := func(ctx context.Context) ([]interface{}, error) {
		emails, err := i.GetEmails(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]interface{}, len(emails))
		for j, e := range emails {
			result[j] = e
		}
		return result, nil
	}

	// Create matcher that handles interface{} and delegates to waitConfig
	matcher := func(email interface{}) bool {
		e, ok := email.(*Email)
		if !ok {
			return false
		}
		return cfg.Matches(e)
	}

	results, err := i.client.strategy.WaitForEmailCount(ctx, i.inboxHash, fetcher, matcher, count, cfg.pollInterval)
	if err != nil {
		return nil, err
	}

	emails := make([]*Email, len(results))
	for j, r := range results {
		email, ok := r.(*Email)
		if !ok {
			return nil, ErrEmailNotFound
		}
		emails[j] = email
	}

	return emails, nil
}

// Delete deletes the inbox.
func (i *Inbox) Delete(ctx context.Context) error {
	return i.client.DeleteInbox(ctx, i.emailAddress)
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

func newInboxFromLegacyResponse(resp *api.LegacyCreateInboxResponse, c *Client) *Inbox {
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

func (i *Inbox) decryptEmail(encrypted *api.EncryptedEmail) (*Email, error) {
	// Convert api.EncryptedEmail to crypto.EncryptedEmail
	cryptoEncrypted := &crypto.EncryptedEmail{
		ID:              encrypted.ID,
		EncapsulatedKey: encrypted.EncapsulatedKey,
		Ciphertext:      encrypted.Ciphertext,
		Signature:       encrypted.Signature,
		ReceivedAt:      encrypted.ReceivedAt,
		IsRead:          encrypted.IsRead,
	}

	decrypted, err := crypto.DecryptEmail(cryptoEncrypted, i.keypair, i.serverSigPk)
	if err != nil {
		return nil, err
	}

	// Convert crypto.DecryptedEmail to Email
	return i.convertDecryptedEmail(decrypted), nil
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

	return &Email{
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
		inbox:       i,
	}
}
