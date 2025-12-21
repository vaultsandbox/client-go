# 02 - Crypto Layer

## Overview

The crypto layer handles all post-quantum cryptographic operations:
- ML-KEM-768 (Kyber768) for key encapsulation
- ML-DSA-65 (Dilithium3) for signature verification
- AES-256-GCM for symmetric encryption
- HKDF-SHA-512 for key derivation

## Dependencies

```go
import (
    "github.com/cloudflare/circl/kem/mlkem/mlkem768"
    "github.com/cloudflare/circl/sign/mldsa/mldsa65"
    "golang.org/x/crypto/hkdf"
)
```

## Constants

```go
// internal/crypto/constants.go
package crypto

const (
    HKDFContext = "vaultsandbox:email:v1"

    // ML-KEM-768 key sizes
    MLKEMPublicKeySize  = 1184
    MLKEMSecretKeySize  = 2400
    MLKEMCiphertextSize = 1088
    MLKEMSharedKeySize  = 32

    // ML-DSA-65 key sizes
    MLDSAPublicKeySize  = 1952
    MLDSASignatureSize  = 3309

    // AES-256-GCM sizes
    AESKeySize   = 32
    AESNonceSize = 12
    AESTagSize   = 16

    // Offset for extracting public key from secret key
    PublicKeyOffset = 1152
)

var AlgsCiphersuite = "ML-KEM-768:ML-DSA-65:AES-256-GCM:HKDF-SHA-512"
```

## File: `internal/crypto/base64.go`

```go
package crypto

import (
    "encoding/base64"
)

// ToBase64URL encodes bytes to URL-safe base64 without padding
func ToBase64URL(data []byte) string {
    return base64.RawURLEncoding.EncodeToString(data)
}

// FromBase64URL decodes URL-safe base64 (handles missing padding)
func FromBase64URL(s string) ([]byte, error) {
    return base64.RawURLEncoding.DecodeString(s)
}
```

## File: `internal/crypto/keypair.go`

```go
package crypto

import (
    "github.com/cloudflare/circl/kem/mlkem/mlkem768"
)

// Keypair represents an ML-KEM-768 keypair
type Keypair struct {
    PublicKey    []byte
    SecretKey    []byte
    PublicKeyB64 string
}

// GenerateKeypair creates a new ML-KEM-768 keypair
func GenerateKeypair() (*Keypair, error) {
    pub, priv, err := mlkem768.GenerateKeyPair(nil)
    if err != nil {
        return nil, err
    }

    pubBytes, err := pub.MarshalBinary()
    if err != nil {
        return nil, err
    }

    privBytes, err := priv.MarshalBinary()
    if err != nil {
        return nil, err
    }

    return &Keypair{
        PublicKey:    pubBytes,
        SecretKey:    privBytes,
        PublicKeyB64: ToBase64URL(pubBytes),
    }, nil
}

// KeypairFromSecretKey reconstructs a keypair from the secret key
// The public key is embedded in the secret key at offset 1152
func KeypairFromSecretKey(secretKey []byte) (*Keypair, error) {
    if len(secretKey) != MLKEMSecretKeySize {
        return nil, ErrInvalidSecretKeySize
    }

    publicKey := secretKey[PublicKeyOffset : PublicKeyOffset+MLKEMPublicKeySize]

    return &Keypair{
        PublicKey:    publicKey,
        SecretKey:    secretKey,
        PublicKeyB64: ToBase64URL(publicKey),
    }, nil
}
```

## File: `internal/crypto/verify.go`

```go
package crypto

import (
    "fmt"

    "github.com/cloudflare/circl/sign/mldsa/mldsa65"
)

// EncryptedPayload represents the encrypted data structure from the server
type EncryptedPayload struct {
    V           int               `json:"v"`
    Algs        AlgorithmSuite    `json:"algs"`
    CtKem       string            `json:"ct_kem"`
    Nonce       string            `json:"nonce"`
    AAD         string            `json:"aad"`
    Ciphertext  string            `json:"ciphertext"`
    Sig         string            `json:"sig"`
    ServerSigPk string            `json:"server_sig_pk"`
}

type AlgorithmSuite struct {
    KEM  string `json:"kem"`
    Sig  string `json:"sig"`
    AEAD string `json:"aead"`
    KDF  string `json:"kdf"`
}

// VerifySignature verifies the ML-DSA-65 signature on the encrypted payload
// CRITICAL: This MUST be called BEFORE any decryption attempt
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

// buildTranscript constructs the signature transcript
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
```

## File: `internal/crypto/decrypt.go`

```go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/sha256"
    "crypto/sha512"
    "encoding/binary"
    "fmt"
    "io"

    "github.com/cloudflare/circl/kem/mlkem/mlkem768"
    "golang.org/x/crypto/hkdf"
)

// Decrypt decrypts an encrypted payload using the provided keypair
// IMPORTANT: VerifySignature must be called before this function
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
    if err := privKey.UnmarshalBinary(keypair.SecretKey); err != nil {
        return nil, fmt.Errorf("unmarshal private key: %w", err)
    }

    sharedSecret, err := mlkem768.Decapsulate(&privKey, ctKem)
    if err != nil {
        return nil, fmt.Errorf("decapsulate: %w", err)
    }

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

// deriveKey performs HKDF-SHA-512 key derivation
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
```

## File: `internal/crypto/aes.go`

```go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "fmt"
)

// decryptAESGCM decrypts data using AES-256-GCM
func decryptAESGCM(key, nonce, aad, ciphertext []byte) ([]byte, error) {
    if len(key) != AESKeySize {
        return nil, fmt.Errorf("invalid key size: got %d, want %d", len(key), AESKeySize)
    }

    if len(nonce) != AESNonceSize {
        return nil, fmt.Errorf("invalid nonce size: got %d, want %d", len(nonce), AESNonceSize)
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
```

## Decryption Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    Encrypted Payload                        │
│  {v, algs, ct_kem, nonce, aad, ciphertext, sig, server_pk} │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              1. VERIFY SIGNATURE (CRITICAL)                 │
│  - Build transcript from payload fields                     │
│  - Verify ML-DSA-65 signature with server public key        │
│  - ABORT if verification fails                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  2. KEM DECAPSULATION                       │
│  - mlkem768.Decapsulate(ct_kem, secretKey)                 │
│  - Returns 32-byte shared secret                            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              3. KEY DERIVATION (HKDF-SHA-512)               │
│  - salt = SHA-256(ct_kem)                                   │
│  - info = context || len(aad) || aad                        │
│  - aesKey = HKDF-Expand(sharedSecret, salt, info, 32)       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│               4. AES-256-GCM DECRYPTION                     │
│  - plaintext = AES-GCM-Decrypt(aesKey, nonce, aad, ct)     │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    Decrypted Plaintext (JSON)
```

## Testing Considerations

1. **Test vectors**: Create known test vectors for each crypto operation
2. **Error cases**: Test invalid signatures, corrupted ciphertext, wrong keys
3. **Edge cases**: Empty AAD, maximum size payloads
4. **Interoperability**: Verify against reference implementations
