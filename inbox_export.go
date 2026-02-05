package vaultsandbox

import (
	"fmt"
	"strings"
	"time"

	"github.com/vaultsandbox/client-go/internal/crypto"
)

// ExportVersion is the current export format version.
const ExportVersion = 1

// ExportedInbox contains all data needed to restore an inbox.
// WARNING: For encrypted inboxes, this contains private key material - handle securely.
//
// The format follows the VaultSandbox specification Section 9.
// For encrypted inboxes, the public key is NOT included as it can be derived
// from the secret key (see spec Section 4.2).
type ExportedInbox struct {
	// Version is the export format version. MUST be 1.
	Version int `json:"version"`
	// EmailAddress is the inbox email address. MUST contain exactly one @.
	EmailAddress string `json:"emailAddress"`
	// ExpiresAt is the inbox expiration timestamp (ISO 8601).
	ExpiresAt time.Time `json:"expiresAt"`
	// InboxHash is the unique inbox identifier. Non-empty.
	InboxHash string `json:"inboxHash"`
	// ServerSigPk is the server's ML-DSA-65 public key (base64url, 1952 bytes decoded).
	// Only set for encrypted inboxes.
	ServerSigPk string `json:"serverSigPk,omitempty"`
	// SecretKey is the ML-KEM-768 secret key (base64url, 2400 bytes decoded).
	// Only set for encrypted inboxes.
	SecretKey string `json:"secretKey,omitempty"`
	// ExportedAt is the export timestamp (ISO 8601). Informational only.
	ExportedAt time.Time `json:"exportedAt"`
	// EmailAuth indicates whether email authentication is enabled for this inbox.
	EmailAuth bool `json:"emailAuth"`
	// Encrypted indicates whether this is an encrypted inbox.
	Encrypted bool `json:"encrypted"`
}

// Validate checks that the exported data is valid per VaultSandbox spec Section 10.
// Validation steps are performed in the order specified.
func (e *ExportedInbox) Validate() error {
	// Step 2: Validate version == 1
	if e.Version != ExportVersion {
		return fmt.Errorf("%w: unsupported version %d, expected %d", ErrInvalidImportData, e.Version, ExportVersion)
	}

	// Step 4: Validate emailAddress is non-empty and contains exactly one @
	if e.EmailAddress == "" {
		return fmt.Errorf("%w: emailAddress is required", ErrInvalidImportData)
	}
	if strings.Count(e.EmailAddress, "@") != 1 {
		return fmt.Errorf("%w: emailAddress must contain exactly one @", ErrInvalidImportData)
	}

	// Step 5: Validate inboxHash is non-empty
	if e.InboxHash == "" {
		return fmt.Errorf("%w: inboxHash is required", ErrInvalidImportData)
	}

	// For encrypted inboxes, validate cryptographic keys
	if e.Encrypted {
		// Step 6: Validate and decode secretKey (2400 bytes)
		if e.SecretKey == "" {
			return fmt.Errorf("%w: secretKey is required for encrypted inbox", ErrInvalidImportData)
		}
		secretKey, err := crypto.FromBase64URL(e.SecretKey)
		if err != nil {
			return fmt.Errorf("%w: invalid secretKey encoding", ErrInvalidImportData)
		}
		if len(secretKey) != crypto.MLKEMSecretKeySize {
			return fmt.Errorf("%w: secretKey size %d, expected %d", ErrInvalidImportData, len(secretKey), crypto.MLKEMSecretKeySize)
		}

		// Step 7: Validate and decode serverSigPk (1952 bytes)
		if e.ServerSigPk == "" {
			return fmt.Errorf("%w: serverSigPk is required for encrypted inbox", ErrInvalidImportData)
		}
		serverSigPk, err := crypto.FromBase64URL(e.ServerSigPk)
		if err != nil {
			return fmt.Errorf("%w: invalid serverSigPk encoding", ErrInvalidImportData)
		}
		if len(serverSigPk) != crypto.MLDSAPublicKeySize {
			return fmt.Errorf("%w: serverSigPk size %d, expected %d", ErrInvalidImportData, len(serverSigPk), crypto.MLDSAPublicKeySize)
		}
	}

	// Step 8: Validate timestamps (Go's time.Time handles ISO 8601 via JSON unmarshaling)
	// ExpiresAt and ExportedAt are validated during JSON unmarshaling.
	// Additional validation: expiresAt should be a valid time
	if e.ExpiresAt.IsZero() {
		return fmt.Errorf("%w: expiresAt is required", ErrInvalidImportData)
	}

	return nil
}

// Export returns exportable inbox data.
// For encrypted inboxes, this includes the private key material.
// The format follows VaultSandbox specification Section 9.
// Note: For encrypted inboxes, the public key is NOT included as it can be derived from the secret key.
func (i *Inbox) Export() *ExportedInbox {
	exported := &ExportedInbox{
		Version:      ExportVersion,
		EmailAddress: i.emailAddress,
		ExpiresAt:    i.expiresAt,
		InboxHash:    i.inboxHash,
		ExportedAt:   time.Now().UTC(),
		EmailAuth:    i.emailAuth,
		Encrypted:    i.encrypted,
	}

	// Only include cryptographic material for encrypted inboxes
	if i.encrypted && i.serverSigPk != nil && i.keypair != nil {
		exported.ServerSigPk = crypto.ToBase64URL(i.serverSigPk)
		exported.SecretKey = crypto.ToBase64URL(i.keypair.SecretKey)
	}

	return exported
}

// newInboxFromExport reconstructs an inbox from exported data.
// For encrypted inboxes, the public key is derived from the secret key per VaultSandbox spec Section 10.2.
func newInboxFromExport(data *ExportedInbox, c *Client) (*Inbox, error) {
	if err := data.Validate(); err != nil {
		return nil, err
	}

	inbox := &Inbox{
		emailAddress: data.EmailAddress,
		expiresAt:    data.ExpiresAt,
		inboxHash:    data.InboxHash,
		client:       c,
		emailAuth:    data.EmailAuth,
		encrypted:    data.Encrypted,
	}

	// For encrypted inboxes, decode keys
	// Validate() already verified these are valid base64 with correct sizes
	if data.Encrypted {
		secretKey, _ := crypto.FromBase64URL(data.SecretKey)
		serverSigPk, _ := crypto.FromBase64URL(data.ServerSigPk)

		// Per spec Section 10.2: Reconstruct keypair by deriving public key from secret key
		keypair, err := crypto.KeypairFromSecretKey(secretKey)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to reconstruct keypair: %v", ErrInvalidImportData, err)
		}

		inbox.serverSigPk = serverSigPk
		inbox.keypair = keypair
	}

	return inbox, nil
}
