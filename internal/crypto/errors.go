package crypto

import "errors"

var (
	// ErrInvalidSecretKeySize is returned when the secret key size is invalid.
	ErrInvalidSecretKeySize = errors.New("invalid secret key size")

	// ErrInvalidPublicKeySize is returned when the public key size is invalid.
	ErrInvalidPublicKeySize = errors.New("invalid public key size")

	// ErrInvalidCiphertextSize is returned when the ciphertext size is invalid.
	ErrInvalidCiphertextSize = errors.New("invalid ciphertext size")

	// ErrSignatureVerificationFailed is returned when signature verification fails.
	ErrSignatureVerificationFailed = errors.New("signature verification failed")

	// ErrDecryptionFailed is returned when decryption fails.
	ErrDecryptionFailed = errors.New("decryption failed")

	// ErrInvalidKeySize is returned when the AES key size is invalid.
	ErrInvalidKeySize = errors.New("invalid key size")

	// ErrInvalidNonceSize is returned when the nonce size is invalid.
	ErrInvalidNonceSize = errors.New("invalid nonce size")
)
