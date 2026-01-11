package crypto

import (
	"crypto/subtle"
	"fmt"

	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
)

// Expected algorithm identifiers per VaultSandbox spec Section 3.
const (
	ExpectedKEM  = "ML-KEM-768"
	ExpectedSig  = "ML-DSA-65"
	ExpectedAEAD = "AES-256-GCM"
	ExpectedKDF  = "HKDF-SHA-512"
)

// ProtocolVersion is the expected protocol version per VaultSandbox spec.
const ProtocolVersion = 1

// EncryptedPayload represents the encrypted data structure from the server.
type EncryptedPayload struct {
	// V is the protocol version number.
	V int `json:"v"`
	// Algs specifies the cryptographic algorithm suite used.
	Algs AlgorithmSuite `json:"algs"`
	// CtKem is the ML-KEM-768 ciphertext (base64url-encoded).
	CtKem string `json:"ct_kem"`
	// Nonce is the AES-GCM nonce/IV (base64url-encoded).
	Nonce string `json:"nonce"`
	// AAD is the additional authenticated data (base64url-encoded).
	AAD string `json:"aad"`
	// Ciphertext is the AES-GCM encrypted content (base64url-encoded).
	Ciphertext string `json:"ciphertext"`
	// Sig is the ML-DSA-65 signature over the transcript (base64url-encoded).
	Sig string `json:"sig"`
	// ServerSigPk is the server's ML-DSA-65 public key (base64url-encoded).
	ServerSigPk string `json:"server_sig_pk"`
}

// AlgorithmSuite represents the cryptographic algorithm suite.
type AlgorithmSuite struct {
	// KEM is the key encapsulation mechanism (e.g., "ML-KEM-768").
	KEM string `json:"kem"`
	// Sig is the signature algorithm (e.g., "ML-DSA-65").
	Sig string `json:"sig"`
	// AEAD is the authenticated encryption algorithm (e.g., "AES-256-GCM").
	AEAD string `json:"aead"`
	// KDF is the key derivation function (e.g., "HKDF-SHA-512").
	KDF string `json:"kdf"`
}

// ValidatePayload validates the encrypted payload structure per VaultSandbox spec Section 8.
// This performs steps 2-4 of the decryption process:
//   - Validate version == 1
//   - Validate all algorithm fields match expected values
//   - Validate decoded binary field sizes
func ValidatePayload(payload *EncryptedPayload) error {
	// Step 2: Validate version
	if payload.V != ProtocolVersion {
		return fmt.Errorf("%w: got version %d, expected %d", ErrInvalidPayload, payload.V, ProtocolVersion)
	}

	// Step 3: Validate algorithms
	if payload.Algs.KEM != ExpectedKEM {
		return fmt.Errorf("%w: unsupported KEM %q", ErrInvalidAlgorithm, payload.Algs.KEM)
	}
	if payload.Algs.Sig != ExpectedSig {
		return fmt.Errorf("%w: unsupported signature algorithm %q", ErrInvalidAlgorithm, payload.Algs.Sig)
	}
	if payload.Algs.AEAD != ExpectedAEAD {
		return fmt.Errorf("%w: unsupported AEAD %q", ErrInvalidAlgorithm, payload.Algs.AEAD)
	}
	if payload.Algs.KDF != ExpectedKDF {
		return fmt.Errorf("%w: unsupported KDF %q", ErrInvalidAlgorithm, payload.Algs.KDF)
	}

	// Step 4: Validate sizes after decoding
	ctKem, err := FromBase64URL(payload.CtKem)
	if err != nil {
		return fmt.Errorf("%w: invalid ct_kem encoding", ErrInvalidPayload)
	}
	if len(ctKem) != MLKEMCiphertextSize {
		return fmt.Errorf("%w: ct_kem size %d, expected %d", ErrInvalidSize, len(ctKem), MLKEMCiphertextSize)
	}

	nonce, err := FromBase64URL(payload.Nonce)
	if err != nil {
		return fmt.Errorf("%w: invalid nonce encoding", ErrInvalidPayload)
	}
	if len(nonce) != AESNonceSize {
		return fmt.Errorf("%w: nonce size %d, expected %d", ErrInvalidSize, len(nonce), AESNonceSize)
	}

	sig, err := FromBase64URL(payload.Sig)
	if err != nil {
		return fmt.Errorf("%w: invalid sig encoding", ErrInvalidPayload)
	}
	if len(sig) != MLDSASignatureSize {
		return fmt.Errorf("%w: signature size %d, expected %d", ErrInvalidSize, len(sig), MLDSASignatureSize)
	}

	serverSigPk, err := FromBase64URL(payload.ServerSigPk)
	if err != nil {
		return fmt.Errorf("%w: invalid server_sig_pk encoding", ErrInvalidPayload)
	}
	if len(serverSigPk) != MLDSAPublicKeySize {
		return fmt.Errorf("%w: server_sig_pk size %d, expected %d", ErrInvalidSize, len(serverSigPk), MLDSAPublicKeySize)
	}

	return nil
}

// VerifySignature verifies the ML-DSA-65 signature on the encrypted payload.
// CRITICAL: This MUST be called BEFORE any decryption attempt per spec Section 8.2.
//
// The pinnedServerPk parameter is the server's public key that was captured
// at inbox creation or import time. The payload's embedded server key must
// match this pinned key exactly, or ErrServerKeyMismatch is returned.
// This prevents attackers from injecting payloads signed with their own keys.
//
// Per spec Section 11.3, constant-time comparison is used for server key verification.
func VerifySignature(payload *EncryptedPayload, pinnedServerPk []byte) error {
	// First validate the payload structure
	if err := ValidatePayload(payload); err != nil {
		return err
	}

	// Decode all components (already validated by ValidatePayload)
	ctKem, _ := FromBase64URL(payload.CtKem)
	nonce, _ := FromBase64URL(payload.Nonce)
	aad, err := FromBase64URL(payload.AAD)
	if err != nil {
		return fmt.Errorf("decode aad: %w", err)
	}
	ciphertext, err := FromBase64URL(payload.Ciphertext)
	if err != nil {
		return fmt.Errorf("decode ciphertext: %w", err)
	}
	serverSigPk, _ := FromBase64URL(payload.ServerSigPk)
	sig, _ := FromBase64URL(payload.Sig)

	// Step 5: Verify the payload's server key matches the pinned key from inbox creation.
	// Per spec Section 11.3: MUST use constant-time comparison.
	// This is critical: without this check, an attacker could inject payloads
	// signed with their own key and bypass authenticity verification.
	if len(serverSigPk) != len(pinnedServerPk) || subtle.ConstantTimeCompare(serverSigPk, pinnedServerPk) != 1 {
		return ErrServerKeyMismatch
	}

	// Step 6: Build transcript and verify signature
	transcript := buildTranscript(payload.V, payload.Algs, ctKem, nonce, aad, ciphertext, serverSigPk)

	// Unmarshal public key (size already validated by ValidatePayload)
	var pubKey mldsa65.PublicKey
	_ = pubKey.UnmarshalBinary(serverSigPk)

	// Verify signature
	if !mldsa65.Verify(&pubKey, transcript, nil, sig) {
		return ErrSignatureVerificationFailed
	}

	return nil
}

// buildTranscript constructs the signature transcript.
func buildTranscript(version int, algs AlgorithmSuite, ctKem, nonce, aad, ciphertext, serverSigPk []byte) []byte {
	// version (1 byte)
	transcript := []byte{byte(version)}

	// algs ciphersuite string
	algsCiphersuite := fmt.Sprintf("%s:%s:%s:%s", algs.KEM, algs.Sig, algs.AEAD, algs.KDF)
	transcript = append(transcript, []byte(algsCiphersuite)...)

	// context string
	transcript = append(transcript, []byte(HKDFContext)...)

	// raw bytes
	transcript = append(transcript, ctKem...)
	transcript = append(transcript, nonce...)
	transcript = append(transcript, aad...)
	transcript = append(transcript, ciphertext...)
	transcript = append(transcript, serverSigPk...)

	return transcript
}

// VerifySignatureSafe verifies the signature without returning an error.
// Returns true if the signature is valid and the server key matches, false otherwise.
func VerifySignatureSafe(payload *EncryptedPayload, pinnedServerPk []byte) bool {
	err := VerifySignature(payload, pinnedServerPk)
	return err == nil
}

// ValidateServerPublicKey validates that a server public key has the correct format and size.
// Takes a base64url-encoded server public key string.
// Returns true if valid, false otherwise.
func ValidateServerPublicKey(serverPublicKey string) bool {
	publicKey, err := FromBase64URL(serverPublicKey)
	if err != nil {
		return false
	}
	return len(publicKey) == MLDSAPublicKeySize
}

// Verify verifies an ML-DSA-65 signature (low-level function).
func Verify(publicKey, message, signature []byte) error {
	pk := &mldsa65.PublicKey{}
	if err := pk.UnmarshalBinary(publicKey); err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	if !mldsa65.Verify(pk, message, nil, signature) {
		return ErrSignatureVerificationFailed
	}

	return nil
}
