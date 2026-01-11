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
// WARNING: Contains private key material - handle securely.
//
// The format follows the VaultSandbox specification Section 9.
// The public key is NOT included as it can be derived from the secret key
// (see spec Section 4.2).
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
	ServerSigPk string `json:"serverSigPk"`
	// SecretKey is the ML-KEM-768 secret key (base64url, 2400 bytes decoded).
	SecretKey string `json:"secretKey"`
	// ExportedAt is the export timestamp (ISO 8601). Informational only.
	ExportedAt time.Time `json:"exportedAt"`
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

	// Step 6: Validate and decode secretKey (2400 bytes)
	if e.SecretKey == "" {
		return fmt.Errorf("%w: secretKey is required", ErrInvalidImportData)
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
		return fmt.Errorf("%w: serverSigPk is required", ErrInvalidImportData)
	}
	serverSigPk, err := crypto.FromBase64URL(e.ServerSigPk)
	if err != nil {
		return fmt.Errorf("%w: invalid serverSigPk encoding", ErrInvalidImportData)
	}
	if len(serverSigPk) != crypto.MLDSAPublicKeySize {
		return fmt.Errorf("%w: serverSigPk size %d, expected %d", ErrInvalidImportData, len(serverSigPk), crypto.MLDSAPublicKeySize)
	}

	// Step 8: Validate timestamps (Go's time.Time handles ISO 8601 via JSON unmarshaling)
	// ExpiresAt and ExportedAt are validated during JSON unmarshaling.
	// Additional validation: expiresAt should be a valid time
	if e.ExpiresAt.IsZero() {
		return fmt.Errorf("%w: expiresAt is required", ErrInvalidImportData)
	}

	return nil
}

// Export returns exportable inbox data including private key.
// The format follows VaultSandbox specification Section 9.
// Note: The public key is NOT included as it can be derived from the secret key.
func (i *Inbox) Export() *ExportedInbox {
	return &ExportedInbox{
		Version:      ExportVersion,
		EmailAddress: i.emailAddress,
		ExpiresAt:    i.expiresAt,
		InboxHash:    i.inboxHash,
		ServerSigPk:  crypto.ToBase64URL(i.serverSigPk),
		SecretKey:    crypto.ToBase64URL(i.keypair.SecretKey),
		ExportedAt:   time.Now().UTC(),
	}
}

// newInboxFromExport reconstructs an inbox from exported data.
// Per VaultSandbox spec Section 10.2, the public key is derived from the secret key.
func newInboxFromExport(data *ExportedInbox, c *Client) (*Inbox, error) {
	if err := data.Validate(); err != nil {
		return nil, err
	}

	// Decode keys - Validate() already verified these are valid base64 with correct sizes
	secretKey, _ := crypto.FromBase64URL(data.SecretKey)
	serverSigPk, _ := crypto.FromBase64URL(data.ServerSigPk)

	// Per spec Section 10.2: Reconstruct keypair by deriving public key from secret key
	// publicKey = secretKey[1152:2400]
	// Validate() already verified the secret key size is correct
	keypair, _ := crypto.KeypairFromSecretKey(secretKey)

	return &Inbox{
		emailAddress: data.EmailAddress,
		expiresAt:    data.ExpiresAt,
		inboxHash:    data.InboxHash,
		serverSigPk:  serverSigPk,
		keypair:      keypair,
		client:       c,
	}, nil
}
