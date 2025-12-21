package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

const (
	// AES-256-GCM nonce size
	nonceSize = 12
	// AES-256-GCM tag size
	tagSize = 16
)

// DecryptAES decrypts data using AES-256-GCM.
// The ciphertext format is: nonce (12 bytes) || ciphertext || tag (16 bytes)
func DecryptAES(key, ciphertext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key size: expected 32, got %d", len(key))
	}

	if len(ciphertext) < nonceSize+tagSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := ciphertext[:nonceSize]
	ciphertextWithTag := ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertextWithTag, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// EncryptAES encrypts data using AES-256-GCM.
// Returns: nonce (12 bytes) || ciphertext || tag (16 bytes)
func EncryptAES(key, plaintext, nonce []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key size: expected 32, got %d", len(key))
	}

	if len(nonce) != nonceSize {
		return nil, fmt.Errorf("invalid nonce size: expected %d, got %d", nonceSize, len(nonce))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}
