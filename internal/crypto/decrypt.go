package crypto

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/cloudflare/circl/kem/mlkem/mlkem768"
	"golang.org/x/crypto/hkdf"
)

// DecryptedMetadata represents the decrypted email metadata returned from the
// list endpoint. This provides a lightweight preview without fetching the full
// email body.
type DecryptedMetadata struct {
	// From is the sender's email address.
	From string `json:"from"`
	// To is the primary recipient. Note: only one recipient is included in
	// metadata; use DecryptedParsed.Headers for full recipient list.
	To string `json:"to"`
	// Subject is the email subject line.
	Subject string `json:"subject"`
	// ReceivedAt is the timestamp when the email was received (ISO 8601 format).
	ReceivedAt string `json:"receivedAt"`
}

// DecryptedParsed represents the decrypted parsed email content including
// the body, headers, and attachments.
type DecryptedParsed struct {
	// Text is the plain text body of the email.
	Text string `json:"text"`
	// HTML is the HTML body of the email, if present.
	HTML string `json:"html"`
	// Headers contains the full email headers as key-value pairs.
	Headers map[string]interface{} `json:"headers"`
	// Attachments contains the email attachments with their content.
	Attachments []DecryptedAttachment `json:"attachments"`
	// Links contains URLs extracted from the email body.
	Links []string `json:"links"`
	// AuthResults contains email authentication results (SPF, DKIM, DMARC).
	AuthResults json.RawMessage `json:"authResults"`
}

// DecryptedEmail represents a fully decrypted email combining metadata and
// parsed content. This is the primary type returned to users after decryption.
type DecryptedEmail struct {
	// ID is the unique email identifier.
	ID string
	// From is the sender's email address.
	From string
	// To contains all recipient email addresses.
	To []string
	// Subject is the email subject line.
	Subject string
	// Text is the plain text body.
	Text string
	// HTML is the HTML body, if present.
	HTML string
	// ReceivedAt is when the email was received.
	ReceivedAt time.Time
	// Headers contains email headers as string key-value pairs.
	Headers map[string]string
	// Attachments contains the email attachments.
	Attachments []DecryptedAttachment
	// Links contains URLs extracted from the email body.
	Links []string
	// AuthResults contains email authentication results (SPF, DKIM, DMARC).
	AuthResults json.RawMessage
	// IsRead indicates whether the email has been marked as read.
	IsRead bool
}

// DecryptedAttachment represents a decrypted email attachment.
type DecryptedAttachment struct {
	// Filename is the attachment's filename.
	Filename string `json:"filename"`
	// ContentType is the MIME type (e.g., "application/pdf").
	ContentType string `json:"contentType"`
	// Size is the attachment size in bytes.
	Size int `json:"size"`
	// ContentID is the Content-ID for inline attachments (e.g., embedded images).
	ContentID string `json:"contentId,omitempty"`
	// ContentDisposition indicates "inline" or "attachment".
	ContentDisposition string `json:"contentDisposition,omitempty"`
	// Content is the raw attachment data (automatically base64-decoded).
	Content Base64Bytes `json:"content,omitempty"`
	// Checksum is the SHA-256 hash of the content for integrity verification.
	Checksum string `json:"checksum,omitempty"`
}

// Base64Bytes handles JSON unmarshaling of base64-encoded content.
// The server may send attachment content as a base64-encoded string,
// which this type automatically decodes to []byte.
type Base64Bytes []byte

// UnmarshalJSON implements json.Unmarshaler for Base64Bytes.
// It handles both raw bytes and base64-encoded strings.
func (b *Base64Bytes) UnmarshalJSON(data []byte) error {
	// Handle null
	if string(data) == "null" {
		*b = nil
		return nil
	}

	// If it's a quoted string, it's base64-encoded
	if len(data) >= 2 && data[0] == '"' && data[len(data)-1] == '"' {
		// Remove quotes
		encoded := string(data[1 : len(data)-1])
		if encoded == "" {
			*b = nil
			return nil
		}
		// Try standard base64 first (for attachment content)
		decoded, err := FromBase64(encoded)
		if err != nil {
			// Fall back to URL-safe base64
			decoded, err = FromBase64URL(encoded)
			if err != nil {
				return err
			}
		}
		*b = decoded
		return nil
	}

	// Otherwise, treat as raw JSON bytes (shouldn't happen for attachments)
	*b = data
	return nil
}

// Decrypt decrypts an encrypted payload using the provided keypair.
//
// The decryption process:
//  1. ML-KEM-768 decapsulation to recover the shared secret
//  2. HKDF-SHA-512 key derivation using the shared secret, AAD, and KEM ciphertext
//  3. AES-256-GCM decryption of the ciphertext
//
// Security: This function does NOT verify signatures. Callers MUST call
// [VerifySignature] before decryption to ensure authenticity and integrity.
// Decrypting without verification may expose the system to chosen-ciphertext attacks.
func Decrypt(payload *EncryptedPayload, keypair *Keypair) ([]byte, error) {
	// Decode components
	ctKem, err := FromBase64URL(payload.CtKem)
	if err != nil {
		return nil, fmt.Errorf("decode ct_kem: %w", err)
	}

	nonce, err := FromBase64URL(payload.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}

	aad, err := FromBase64URL(payload.AAD)
	if err != nil {
		return nil, fmt.Errorf("decode aad: %w", err)
	}

	ciphertext, err := FromBase64URL(payload.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	// 1. KEM Decapsulation
	var privKey mlkem768.PrivateKey
	if err := privKey.Unpack(keypair.SecretKey); err != nil {
		return nil, fmt.Errorf("unmarshal private key: %w", err)
	}

	sharedSecret := make([]byte, MLKEMSharedKeySize)
	privKey.DecapsulateTo(sharedSecret, ctKem)

	// 2. Key Derivation (HKDF-SHA-512)
	aesKey, err := deriveKey(sharedSecret, aad, ctKem)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	// 3. AES-256-GCM Decryption
	plaintext, err := decryptAESGCM(aesKey, nonce, aad, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// deriveKey performs HKDF-SHA-512 key derivation for the encryption scheme.
//
// The key derivation uses:
//   - IKM (input key material): the KEM shared secret
//   - Salt: SHA-256 hash of the KEM ciphertext
//   - Info: context string || AAD length (4 bytes BE) || AAD
//
// This produces a 256-bit key suitable for AES-256-GCM.
func deriveKey(sharedSecret, aad, ctKem []byte) ([]byte, error) {
	// Salt is SHA-256 hash of KEM ciphertext
	saltHash := sha256.Sum256(ctKem)
	salt := saltHash[:]

	// Info construction: context || aad_length (4 bytes BE) || aad
	contextBytes := []byte(HKDFContext)
	aadLength := make([]byte, 4)
	binary.BigEndian.PutUint32(aadLength, uint32(len(aad)))

	info := make([]byte, 0, len(contextBytes)+4+len(aad))
	info = append(info, contextBytes...)
	info = append(info, aadLength...)
	info = append(info, aad...)

	// HKDF with SHA-512
	reader := hkdf.New(sha512.New, sharedSecret, salt, info)
	key := make([]byte, AESKeySize)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, err
	}

	return key, nil
}

// DeriveKey derives a key using HKDF-SHA-512.
//
// Parameters:
//   - secret: the input key material (e.g., shared secret from KEM)
//   - salt: optional salt value; if empty, a zero-filled salt is used
//   - info: context/application-specific info for domain separation
//   - length: desired output key length in bytes
func DeriveKey(secret, salt, info []byte, length int) ([]byte, error) {
	if len(salt) == 0 {
		salt = make([]byte, sha512.Size)
	}

	reader := hkdf.New(sha512.New, secret, salt, info)
	key := make([]byte, length)

	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	return key, nil
}
