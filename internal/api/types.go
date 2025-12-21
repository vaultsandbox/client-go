package api

import (
	"time"

	"github.com/vaultsandbox/client-go/internal/crypto"
)

// ServerInfo represents the /api/server-info response.
type ServerInfo struct {
	ServerSigPk    string                `json:"serverSigPk"`
	Algs           crypto.AlgorithmSuite `json:"algs"`
	Context        string                `json:"context"`
	MaxTTL         int                   `json:"maxTtl"`
	DefaultTTL     int                   `json:"defaultTtl"`
	SSEConsole     bool                  `json:"sseConsole"`
	AllowedDomains []string              `json:"allowedDomains"`
}

// CreateInboxRequest represents the POST /api/inboxes request.
type CreateInboxRequest struct {
	ClientKemPk  string `json:"clientKemPk"`
	TTL          int    `json:"ttl,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
}

// CreateInboxResponse represents the POST /api/inboxes response.
type CreateInboxResponse struct {
	EmailAddress string    `json:"emailAddress"`
	ExpiresAt    time.Time `json:"expiresAt"`
	InboxHash    string    `json:"inboxHash"`
	ServerSigPk  string    `json:"serverSigPk"`
}

// SyncStatus represents the /api/inboxes/{email}/sync response.
type SyncStatus struct {
	EmailCount int    `json:"emailCount"`
	EmailsHash string `json:"emailsHash"`
}

// RawEmail represents an encrypted email from the API.
type RawEmail struct {
	ID                string                   `json:"id"`
	InboxID           string                   `json:"inboxId"`
	ReceivedAt        time.Time                `json:"receivedAt"`
	IsRead            bool                     `json:"isRead"`
	EncryptedMetadata *crypto.EncryptedPayload `json:"encryptedMetadata"`
	EncryptedParsed   *crypto.EncryptedPayload `json:"encryptedParsed,omitempty"`
}

// RawEmailSource represents the raw email source response.
type RawEmailSource struct {
	ID           string                   `json:"id"`
	EncryptedRaw *crypto.EncryptedPayload `json:"encryptedRaw"`
}

// SSEEvent represents an SSE event payload.
type SSEEvent struct {
	InboxID           string                   `json:"inboxId"`
	EmailID           string                   `json:"emailId"`
	EncryptedMetadata *crypto.EncryptedPayload `json:"encryptedMetadata"`
}

// Legacy types for backward compatibility with existing endpoints.go code

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

type emailAPIResponse struct {
	ID              string    `json:"id"`
	EncapsulatedKey string    `json:"encapsulatedKey"`
	Ciphertext      string    `json:"ciphertext"`
	Signature       string    `json:"signature"`
	ReceivedAt      time.Time `json:"receivedAt"`
	IsRead          bool      `json:"isRead"`
}
