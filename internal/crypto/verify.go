package crypto

import (
	"bytes"
	"fmt"

	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
)

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

// VerifySignature verifies the ML-DSA-65 signature on the encrypted payload.
// CRITICAL: This MUST be called BEFORE any decryption attempt.
//
// The pinnedServerPk parameter is the server's public key that was captured
// at inbox creation or import time. The payload's embedded server key must
// match this pinned key exactly, or ErrServerKeyMismatch is returned.
// This prevents attackers from injecting payloads signed with their own keys.
func VerifySignature(payload *EncryptedPayload, pinnedServerPk []byte) error {
	// Decode all components
	ctKem, err := FromBase64URL(payload.CtKem)
	if err != nil {
		return fmt.Errorf("decode ct_kem: %w", err)
	}

	nonce, err := FromBase64URL(payload.Nonce)
	if err != nil {
		return fmt.Errorf("decode nonce: %w", err)
	}

	aad, err := FromBase64URL(payload.AAD)
	if err != nil {
		return fmt.Errorf("decode aad: %w", err)
	}

	ciphertext, err := FromBase64URL(payload.Ciphertext)
	if err != nil {
		return fmt.Errorf("decode ciphertext: %w", err)
	}

	serverSigPk, err := FromBase64URL(payload.ServerSigPk)
	if err != nil {
		return fmt.Errorf("decode server_sig_pk: %w", err)
	}

	// Verify the payload's server key matches the pinned key from inbox creation.
	// This is critical: without this check, an attacker could inject payloads
	// signed with their own key and bypass authenticity verification.
	if !bytes.Equal(serverSigPk, pinnedServerPk) {
		return ErrServerKeyMismatch
	}

	sig, err := FromBase64URL(payload.Sig)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	// Build transcript exactly as server does
	transcript := buildTranscript(payload.V, payload.Algs, ctKem, nonce, aad, ciphertext, serverSigPk)

	// Unmarshal public key
	var pubKey mldsa65.PublicKey
	if err := pubKey.UnmarshalBinary(serverSigPk); err != nil {
		return fmt.Errorf("unmarshal public key: %w", err)
	}

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
