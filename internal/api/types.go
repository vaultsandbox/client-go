package api

import "time"

// API request/response types

type createInboxAPIRequest struct {
	PublicKey    string `json:"public_key"`
	TTL          int    `json:"ttl,omitempty"`
	EmailAddress string `json:"email_address,omitempty"`
}

type createInboxAPIResponse struct {
	EmailAddress             string    `json:"email_address"`
	ExpiresAt                time.Time `json:"expires_at"`
	InboxHash                string    `json:"inbox_hash"`
	ServerSignaturePublicKey string    `json:"server_signature_public_key"`
}

type getEmailsAPIResponse struct {
	Emails []emailAPIResponse `json:"emails"`
}

type emailAPIResponse struct {
	ID              string    `json:"id"`
	EncapsulatedKey string    `json:"encapsulated_key"`
	Ciphertext      string    `json:"ciphertext"`
	Signature       string    `json:"signature"`
	ReceivedAt      time.Time `json:"received_at"`
	IsRead          bool      `json:"is_read"`
}
