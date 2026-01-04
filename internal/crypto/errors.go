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

	// ErrServerKeyMismatch is returned when the payload's server public key
	// does not match the pinned server key from inbox creation.
	ErrServerKeyMismatch = errors.New("server public key mismatch: payload key differs from pinned key")

	// ErrDecryptionFailed is returned when decryption fails.
	ErrDecryptionFailed = errors.New("decryption failed")

	// ErrInvalidKeySize is returned when the AES key size is invalid.
	ErrInvalidKeySize = errors.New("invalid key size")

	// ErrInvalidNonceSize is returned when the nonce size is invalid.
	ErrInvalidNonceSize = errors.New("invalid nonce size")

	// ErrInvalidPayload is returned when the encrypted payload structure is invalid.
	// This includes malformed JSON, missing required fields, or invalid encoding.
	ErrInvalidPayload = errors.New("invalid payload")

	// ErrInvalidAlgorithm is returned when an unrecognized or unsupported
	// algorithm is specified in the payload.
	ErrInvalidAlgorithm = errors.New("invalid algorithm")

	// ErrInvalidSize is returned when a decoded field has an incorrect size.
	ErrInvalidSize = errors.New("invalid size")
)
