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

// DecryptedMetadata represents the decrypted email metadata (from list endpoint).
type DecryptedMetadata struct {
	From       string `json:"from"`
	To         string `json:"to"` // Single recipient in metadata
	Subject    string `json:"subject"`
	ReceivedAt string `json:"receivedAt"`
}

// DecryptedParsed represents the decrypted parsed email content.
type DecryptedParsed struct {
	Text        string                 `json:"text"`
	HTML        string                 `json:"html"`
	Headers     map[string]interface{} `json:"headers"`
	Attachments []DecryptedAttachment  `json:"attachments"`
	Links       []string               `json:"links"`
	AuthResults json.RawMessage        `json:"authResults"`
}

// DecryptedEmail represents a fully decrypted email (combined metadata + parsed).
type DecryptedEmail struct {
	ID          string
	From        string
	To          []string
	Subject     string
	Text        string
	HTML        string
	ReceivedAt  time.Time
	Headers     map[string]string
	Attachments []DecryptedAttachment
	Links       []string
	AuthResults json.RawMessage
	IsRead      bool
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

// Decrypt decrypts an encrypted payload using the provided keypair.
// IMPORTANT: VerifySignature must be called before this function.
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

// deriveKey performs HKDF-SHA-512 key derivation.
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

// DeriveKey derives a key using HKDF-SHA-512 (backward compatibility).
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

// EncryptedEmail represents an encrypted email for decryption (backward compatibility).
type EncryptedEmail struct {
	ID              string
	EncapsulatedKey []byte
	Ciphertext      []byte
	Signature       []byte
	ReceivedAt      time.Time
	IsRead          bool
}

// DecryptEmail decrypts an encrypted email (backward compatibility).
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
