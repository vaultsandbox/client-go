// Package crypto provides cryptographic primitives for the VaultSandbox protocol.
// It implements post-quantum key encapsulation, authenticated encryption, and
// digital signatures using modern, standardized algorithms.
//
// # Algorithm Suite
//
// The package uses the following cryptographic algorithms:
//
//   - ML-KEM-768 (NIST FIPS 203): Post-quantum key encapsulation mechanism
//     for establishing shared secrets. Provides 192-bit classical and quantum
//     security levels.
//
//   - ML-DSA-65 (NIST FIPS 204): Post-quantum digital signature algorithm
//     for authenticating encrypted payloads. Provides 192-bit security.
//
//   - AES-256-GCM: Authenticated encryption with associated data (AEAD)
//     for encrypting email content. Provides confidentiality and integrity.
//
//   - HKDF-SHA-512 (RFC 5869): Key derivation function for deriving AES keys
//     from KEM shared secrets with domain separation.
//
// # Security Model
//
// The encryption scheme provides:
//
//   - Confidentiality: Only the holder of the private key can decrypt emails.
//   - Authenticity: Signatures prove emails originated from the VaultSandbox server.
//   - Integrity: Tampering with encrypted content causes decryption to fail.
//   - Forward secrecy: Each email uses a fresh KEM encapsulation.
//
// # Critical Security Notes
//
// Signature verification MUST be performed BEFORE decryption. Decrypting
// unauthenticated ciphertext may expose the system to chosen-ciphertext attacks.
// Always use [VerifySignature] before [Decrypt]:
//
//	if err := crypto.VerifySignature(payload); err != nil {
//	    return nil, fmt.Errorf("signature verification failed: %w", err)
//	}
//	plaintext, err := crypto.Decrypt(payload, keypair)
//
// AES-GCM nonces MUST be unique for each encryption with the same key. Nonce
// reuse completely breaks the security of AES-GCM, allowing attackers to
// recover the authentication key and forge messages.
//
// # Key Management
//
// Use [GenerateKeypair] to create a new ML-KEM-768 keypair. The secret key
// contains an embedded copy of the public key at offset 1152, which can be
// extracted using [KeypairFromSecretKey] or [DerivePublicKeyFromSecret].
//
// Keep secret keys secure. They should never be logged, transmitted in
// plaintext, or stored in version control.
//
// # Base64 Encoding
//
// The package provides base64 encoding functions for cryptographic data:
//
//   - [ToBase64URL]/[FromBase64URL]: URL-safe base64 without padding (RFC 4648 §5).
//     Used for all protocol values (keys, nonces, ciphertexts, signatures).
//
//   - [ToBase64]/[FromBase64]: Standard base64 with padding (RFC 4648 §4).
//     Used for attachment content.
package crypto
