package api

import (
	"time"

	"github.com/vaultsandbox/client-go/internal/crypto"
)

// EncryptionPolicy represents the server's encryption policy for inboxes.
type EncryptionPolicy string

const (
	// EncryptionPolicyAlways requires all inboxes to be encrypted.
	// No per-inbox override is allowed.
	EncryptionPolicyAlways EncryptionPolicy = "always"
	// EncryptionPolicyEnabled makes encryption the default, but allows
	// per-inbox override to request plain inboxes.
	EncryptionPolicyEnabled EncryptionPolicy = "enabled"
	// EncryptionPolicyDisabled makes plain the default, but allows
	// per-inbox override to request encrypted inboxes.
	EncryptionPolicyDisabled EncryptionPolicy = "disabled"
	// EncryptionPolicyNever requires all inboxes to be plain.
	// No per-inbox override is allowed.
	EncryptionPolicyNever EncryptionPolicy = "never"
)

// CanOverride returns true if the policy allows per-inbox encryption override.
func (p EncryptionPolicy) CanOverride() bool {
	return p == EncryptionPolicyEnabled || p == EncryptionPolicyDisabled
}

// DefaultEncrypted returns true if encryption is the default for this policy.
func (p EncryptionPolicy) DefaultEncrypted() bool {
	return p == EncryptionPolicyAlways || p == EncryptionPolicyEnabled
}

// PersistencePolicy represents the server's persistence policy for inboxes.
type PersistencePolicy string

const (
	// PersistencePolicyAlways requires all inboxes to be persistent.
	// No per-inbox override is allowed.
	PersistencePolicyAlways PersistencePolicy = "always"
	// PersistencePolicyEnabled makes persistence the default, but allows
	// per-inbox override to request ephemeral inboxes.
	PersistencePolicyEnabled PersistencePolicy = "enabled"
	// PersistencePolicyDisabled makes ephemeral the default, but allows
	// per-inbox override to request persistent inboxes.
	PersistencePolicyDisabled PersistencePolicy = "disabled"
	// PersistencePolicyNever requires all inboxes to be ephemeral.
	// No per-inbox override is allowed.
	PersistencePolicyNever PersistencePolicy = "never"
)

// CanOverride returns true if the policy allows per-inbox persistence override.
func (p PersistencePolicy) CanOverride() bool {
	return p == PersistencePolicyEnabled || p == PersistencePolicyDisabled
}

// DefaultPersistent returns true if persistence is the default for this policy.
func (p PersistencePolicy) DefaultPersistent() bool {
	return p == PersistencePolicyAlways || p == PersistencePolicyEnabled
}

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
	// EncryptionPolicy specifies the server's encryption policy for inboxes.
	EncryptionPolicy EncryptionPolicy `json:"encryptionPolicy"`
	// PersistencePolicy specifies the server's persistence policy for inboxes.
	PersistencePolicy PersistencePolicy `json:"persistencePolicy"`
	// PersistentGlobalWebhooks indicates whether global webhooks fire for persistent inboxes.
	PersistentGlobalWebhooks bool `json:"persistentGlobalWebhooks"`
	// SpamAnalysisEnabled indicates whether spam analysis (Rspamd) is enabled on the server.
	SpamAnalysisEnabled bool `json:"spamAnalysisEnabled"`
	// ChaosEnabled indicates whether chaos engineering features are enabled on the server.
	ChaosEnabled bool `json:"chaosEnabled"`
}

// SyncStatus represents the /api/inboxes/{email}/sync response used to check
// for new emails without fetching full content.
type SyncStatus struct {
	// EmailCount is the number of emails in the inbox.
	EmailCount int `json:"emailCount"`
	// EmailsHash is a hash of the email list; changes indicate new/deleted emails.
	EmailsHash string `json:"emailsHash"`
}

// RawEmail represents an email from the API, either encrypted or plain.
// Use IsEncrypted() to determine the format:
//   - Encrypted: EncryptedMetadata and EncryptedParsed are set
//   - Plain: Metadata and Parsed are set (Base64-encoded JSON)
type RawEmail struct {
	// ID is the unique email identifier.
	ID string `json:"id"`
	// InboxID is the inbox this email belongs to.
	InboxID string `json:"inboxId"`
	// ReceivedAt is when the email was received by the server.
	ReceivedAt time.Time `json:"receivedAt"`
	// IsRead indicates whether the email has been marked as read.
	IsRead bool `json:"isRead"`

	// Encrypted format fields
	// EncryptedMetadata contains the encrypted email headers (from, to, subject).
	EncryptedMetadata *crypto.EncryptedPayload `json:"encryptedMetadata,omitempty"`
	// EncryptedParsed contains the encrypted email body and attachments.
	// Only present when fetching full email details.
	EncryptedParsed *crypto.EncryptedPayload `json:"encryptedParsed,omitempty"`

	// Plain format fields
	// Metadata contains the Base64-encoded JSON email headers (from, to, subject).
	Metadata string `json:"metadata,omitempty"`
	// Parsed contains the Base64-encoded JSON email body and attachments.
	// Only present when fetching full email details.
	Parsed string `json:"parsed,omitempty"`
}

// IsEncrypted returns true if the email is in encrypted format.
func (r *RawEmail) IsEncrypted() bool {
	return r.EncryptedMetadata != nil
}

// RawEmailSource represents the raw RFC 5322 email source, either encrypted or plain.
// Use IsEncrypted() to determine the format.
type RawEmailSource struct {
	// ID is the email identifier.
	ID string `json:"id"`
	// EncryptedRaw contains the encrypted raw email source (encrypted inboxes).
	EncryptedRaw *crypto.EncryptedPayload `json:"encryptedRaw,omitempty"`
	// Raw contains the Base64-encoded raw email source (plain inboxes).
	Raw string `json:"raw,omitempty"`
}

// IsEncrypted returns true if the raw email source is in encrypted format.
func (r *RawEmailSource) IsEncrypted() bool {
	return r.EncryptedRaw != nil
}

// SSEEvent represents a server-sent event payload for real-time email notifications.
// Use IsEncrypted() to determine the format.
type SSEEvent struct {
	// InboxID is the inbox that received the email.
	InboxID string `json:"inboxId"`
	// EmailID is the unique identifier of the new email.
	EmailID string `json:"emailId"`
	// EncryptedMetadata contains the encrypted email headers for preview (encrypted inboxes).
	EncryptedMetadata *crypto.EncryptedPayload `json:"encryptedMetadata,omitempty"`
	// Metadata contains the Base64-encoded JSON email headers for preview (plain inboxes).
	Metadata string `json:"metadata,omitempty"`
}

// IsEncrypted returns true if the SSE event is for an encrypted inbox.
func (e *SSEEvent) IsEncrypted() bool {
	return e.EncryptedMetadata != nil
}

type createInboxAPIRequest struct {
	ClientKemPk  string `json:"clientKemPk,omitempty"` // Required when creating encrypted inbox
	TTL          int    `json:"ttl,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
	EmailAuth    *bool  `json:"emailAuth,omitempty"`
	Encryption   string `json:"encryption,omitempty"`   // "encrypted" or "plain", omit for server default
	Persistence  string `json:"persistence,omitempty"` // "persistent" or "ephemeral", omit for server default
	SpamAnalysis *bool  `json:"spamAnalysis,omitempty"`
}

type createInboxAPIResponse struct {
	EmailAddress string    `json:"emailAddress"`
	ExpiresAt    time.Time `json:"expiresAt"`
	InboxHash    string    `json:"inboxHash"`
	ServerSigPk  string    `json:"serverSigPk,omitempty"` // Only present when Encrypted=true
	EmailAuth    bool      `json:"emailAuth"`
	Encrypted    bool      `json:"encrypted"`  // Actual encryption state of the inbox
	Persistent   bool      `json:"persistent"` // Actual persistence state of the inbox
	SpamAnalysis *bool     `json:"spamAnalysis,omitempty"`
}

