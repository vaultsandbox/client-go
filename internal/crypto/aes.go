package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

// decryptAESGCM decrypts data using AES-256-GCM with authenticated additional data.
//
// Parameters:
//   - key: 256-bit (32-byte) AES key
//   - nonce: 96-bit (12-byte) initialization vector (must be unique per key)
//   - aad: additional authenticated data (integrity-protected but not encrypted)
//   - ciphertext: encrypted data with appended 128-bit authentication tag
//
// Returns the decrypted plaintext or [ErrDecryptionFailed] if authentication fails.
func decryptAESGCM(key, nonce, aad, ciphertext []byte) ([]byte, error) {
	if len(key) != AESKeySize {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrInvalidKeySize, len(key), AESKeySize)
	}

	if len(nonce) != AESNonceSize {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrInvalidNonceSize, len(nonce), AESNonceSize)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// DecryptAES decrypts data using AES-256-GCM without additional authenticated data.
//
// The ciphertext format is: nonce (12 bytes) || encrypted data || tag (16 bytes)
//
// Parameters:
//   - key: 256-bit (32-byte) AES key
//   - ciphertext: combined nonce, encrypted data, and authentication tag
//
// This function extracts the nonce from the ciphertext prefix and uses no AAD.
// It is provided for backward compatibility with the legacy encryption format.
func DecryptAES(key, ciphertext []byte) ([]byte, error) {
	if len(key) != AESKeySize {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrInvalidKeySize, len(key), AESKeySize)
	}

	if len(ciphertext) < AESNonceSize+AESTagSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:AESNonceSize]
	ciphertextWithTag := ciphertext[AESNonceSize:]

	return decryptAESGCM(key, nonce, nil, ciphertextWithTag)
}

// EncryptAES encrypts data using AES-256-GCM.
//
// Returns: nonce (12 bytes) || ciphertext || tag (16 bytes)
//
// Parameters:
//   - key: 256-bit (32-byte) AES key
//   - plaintext: data to encrypt
//   - nonce: 96-bit (12-byte) initialization vector
//
// Security: The nonce MUST be unique for each encryption with the same key.
// Nonce reuse completely breaks the security of AES-GCM, allowing attackers
// to recover the authentication key and forge messages. Use crypto/rand to
// generate random nonces, or use a deterministic counter if synchronization
// is guaranteed.
func EncryptAES(key, plaintext, nonce []byte) ([]byte, error) {
	if len(key) != AESKeySize {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrInvalidKeySize, len(key), AESKeySize)
	}

	if len(nonce) != AESNonceSize {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrInvalidNonceSize, len(nonce), AESNonceSize)
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
