package api

import (
	"time"

	"github.com/vaultsandbox/client-go/internal/crypto"
)

// ServerInfo represents the /api/server-info response containing server
// configuration and capabilities.
type ServerInfo struct {
	// ServerSigPk is the server's Ed25519 public key (base64url) for signature verification.
	ServerSigPk string `json:"serverSigPk"`
	// Algs describes the cryptographic algorithms used by this server.
	Algs crypto.AlgorithmSuite `json:"algs"`
	// Context is the HKDF context string used in key derivation.
	Context string `json:"context"`
	// MaxTTL is the maximum allowed inbox time-to-live in seconds.
	MaxTTL int `json:"maxTtl"`
	// DefaultTTL is the default inbox time-to-live in seconds if not specified.
	DefaultTTL int `json:"defaultTtl"`
	// SSEConsole indicates if the server supports server-sent events for real-time updates.
	SSEConsole bool `json:"sseConsole"`
	// AllowedDomains lists email domains that can be used for inbox creation.
	AllowedDomains []string `json:"allowedDomains"`
}

// CreateInboxRequest represents the POST /api/inboxes request body.
type CreateInboxRequest struct {
	// ClientKemPk is the client's ML-KEM-768 public key (base64url) for encryption.
	ClientKemPk string `json:"clientKemPk"`
	// TTL is the inbox time-to-live in seconds. Optional; uses server default if omitted.
	TTL int `json:"ttl,omitempty"`
	// EmailAddress is the desired email address. Optional; server generates one if omitted.
	EmailAddress string `json:"emailAddress,omitempty"`
}

// CreateInboxResponse represents the POST /api/inboxes response.
type CreateInboxResponse struct {
	// EmailAddress is the created inbox's email address.
	EmailAddress string `json:"emailAddress"`
	// ExpiresAt is when the inbox will be automatically deleted.
	ExpiresAt time.Time `json:"expiresAt"`
	// InboxHash is the unique hash identifier for the inbox.
	InboxHash string `json:"inboxHash"`
	// ServerSigPk is the server's signing public key (base64url) for verification.
	ServerSigPk string `json:"serverSigPk"`
}

// SyncStatus represents the /api/inboxes/{email}/sync response used to check
// for new emails without fetching full content.
type SyncStatus struct {
	// EmailCount is the number of emails in the inbox.
	EmailCount int `json:"emailCount"`
	// EmailsHash is a hash of the email list; changes indicate new/deleted emails.
	EmailsHash string `json:"emailsHash"`
}

// RawEmail represents an encrypted email from the API before decryption.
type RawEmail struct {
	// ID is the unique email identifier.
	ID string `json:"id"`
	// InboxID is the inbox this email belongs to.
	InboxID string `json:"inboxId"`
	// ReceivedAt is when the email was received by the server.
	ReceivedAt time.Time `json:"receivedAt"`
	// IsRead indicates whether the email has been marked as read.
	IsRead bool `json:"isRead"`
	// EncryptedMetadata contains the encrypted email headers (from, to, subject).
	EncryptedMetadata *crypto.EncryptedPayload `json:"encryptedMetadata"`
	// EncryptedParsed contains the encrypted email body and attachments.
	// Only present when fetching full email details.
	EncryptedParsed *crypto.EncryptedPayload `json:"encryptedParsed,omitempty"`
}

// RawEmailSource represents the raw RFC 5322 email source in encrypted form.
type RawEmailSource struct {
	// ID is the email identifier.
	ID string `json:"id"`
	// EncryptedRaw contains the encrypted raw email source.
	EncryptedRaw *crypto.EncryptedPayload `json:"encryptedRaw"`
}

// SSEEvent represents a server-sent event payload for real-time email notifications.
type SSEEvent struct {
	// InboxID is the inbox that received the email.
	InboxID string `json:"inboxId"`
	// EmailID is the unique identifier of the new email.
	EmailID string `json:"emailId"`
	// EncryptedMetadata contains the encrypted email headers for preview.
	EncryptedMetadata *crypto.EncryptedPayload `json:"encryptedMetadata"`
}

// Legacy types for backward compatibility with existing endpoints.go code.
// These mirror the exported types but are used internally.

type createInboxAPIRequest struct {
	ClientKemPk  string `json:"clientKemPk"`
	TTL          int    `json:"ttl,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
}

type createInboxAPIResponse struct {
	EmailAddress string    `json:"emailAddress"`
	ExpiresAt    time.Time `json:"expiresAt"`
	InboxHash    string    `json:"inboxHash"`
	ServerSigPk  string    `json:"serverSigPk"`
}

type getEmailsAPIResponse struct {
	Emails []emailAPIResponse `json:"emails"`
}

// emailAPIResponse represents an email in the legacy API format with separate
// encapsulatedKey, ciphertext, and signature fields (pre-EncryptedPayload format).
type emailAPIResponse struct {
	ID              string    `json:"id"`
	EncapsulatedKey string    `json:"encapsulatedKey"` // KEM ciphertext (base64url)
	Ciphertext      string    `json:"ciphertext"`      // AES-GCM encrypted content (base64url)
	Signature       string    `json:"signature"`       // Ed25519 signature (base64url)
	ReceivedAt      time.Time `json:"receivedAt"`
	IsRead          bool      `json:"isRead"`
}
