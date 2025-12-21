package crypto

import (
	"fmt"

	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
)

// EncryptedPayload represents the encrypted data structure from the server.
type EncryptedPayload struct {
	V           int            `json:"v"`
	Algs        AlgorithmSuite `json:"algs"`
	CtKem       string         `json:"ct_kem"`
	Nonce       string         `json:"nonce"`
	AAD         string         `json:"aad"`
	Ciphertext  string         `json:"ciphertext"`
	Sig         string         `json:"sig"`
	ServerSigPk string         `json:"server_sig_pk"`
}

// AlgorithmSuite represents the cryptographic algorithm suite.
type AlgorithmSuite struct {
	KEM  string `json:"kem"`
	Sig  string `json:"sig"`
	AEAD string `json:"aead"`
	KDF  string `json:"kdf"`
}

// VerifySignature verifies the ML-DSA-65 signature on the encrypted payload.
// CRITICAL: This MUST be called BEFORE any decryption attempt.
func VerifySignature(payload *EncryptedPayload) error {
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
