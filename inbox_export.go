package vaultsandbox

import (
	"fmt"
	"time"

	"github.com/vaultsandbox/client-go/internal/crypto"
)

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

func newInboxFromExport(data *ExportedInbox, c *Client) (*Inbox, error) {
	if err := data.Validate(); err != nil {
		return nil, err
	}

	secretKey, err := crypto.FromBase64URL(data.SecretKeyB64)
	if err != nil {
		return nil, fmt.Errorf("invalid secret key: %w", err)
	}
	publicKey, err := crypto.FromBase64URL(data.PublicKeyB64)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}
	serverSigPk, err := crypto.FromBase64URL(data.ServerSigPk)
	if err != nil {
		return nil, fmt.Errorf("invalid server signature key: %w", err)
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
