package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

// decryptAESGCM decrypts data using AES-256-GCM.
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

// DecryptAES decrypts data using AES-256-GCM (backward compatibility).
// The ciphertext format is: nonce (12 bytes) || ciphertext || tag (16 bytes)
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
// Returns: nonce (12 bytes) || ciphertext || tag (16 bytes)
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
