package crypto

import (
	"encoding/json"
	"fmt"
	"time"
)

// EncryptedEmail represents an encrypted email for decryption.
type EncryptedEmail struct {
	ID              string
	EncapsulatedKey []byte
	Ciphertext      []byte
	Signature       []byte
	ReceivedAt      time.Time
	IsRead          bool
}

// DecryptedEmail represents the decrypted email payload.
type DecryptedEmail struct {
	ID          string                `json:"id"`
	From        string                `json:"from"`
	To          []string              `json:"to"`
	Subject     string                `json:"subject"`
	Text        string                `json:"text"`
	HTML        string                `json:"html"`
	ReceivedAt  time.Time             `json:"received_at"`
	Headers     map[string]string     `json:"headers"`
	Attachments []DecryptedAttachment `json:"attachments"`
	Links       []string              `json:"links"`
	AuthResults json.RawMessage       `json:"auth_results"`
	IsRead      bool                  `json:"is_read"`
}

// DecryptedAttachment represents a decrypted attachment.
type DecryptedAttachment struct {
	Filename           string `json:"filename"`
	ContentType        string `json:"content_type"`
	Size               int    `json:"size"`
	ContentID          string `json:"content_id"`
	ContentDisposition string `json:"content_disposition"`
	Content            []byte `json:"content"`
	Checksum           string `json:"checksum"`
}

// DecryptEmail decrypts an encrypted email.
func DecryptEmail(encrypted *EncryptedEmail, keypair *Keypair, serverSigPk []byte) (*DecryptedEmail, error) {
	// Verify signature first
	signedData := append(encrypted.EncapsulatedKey, encrypted.Ciphertext...)
	if err := Verify(serverSigPk, signedData, encrypted.Signature); err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	// Decapsulate shared secret
	sharedSecret, err := keypair.Decapsulate(encrypted.EncapsulatedKey)
	if err != nil {
		return nil, fmt.Errorf("decapsulation failed: %w", err)
	}

	// Derive AES key using HKDF
	aesKey, err := DeriveKey(sharedSecret, nil, []byte("vaultsandbox-email-encryption"), 32)
	if err != nil {
		return nil, fmt.Errorf("key derivation failed: %w", err)
	}

	// Decrypt ciphertext
	plaintext, err := DecryptAES(aesKey, encrypted.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	// Parse JSON payload
	var email DecryptedEmail
	if err := json.Unmarshal(plaintext, &email); err != nil {
		return nil, fmt.Errorf("failed to parse email: %w", err)
	}

	email.ID = encrypted.ID
	email.ReceivedAt = encrypted.ReceivedAt
	email.IsRead = encrypted.IsRead

	return &email, nil
}
