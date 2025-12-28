package crypto

const (
	// HKDFContext is the context string used in HKDF key derivation
	// for domain separation.
	HKDFContext = "vaultsandbox:email:v1"

	// MLKEMPublicKeySize is the size of an ML-KEM-768 public key in bytes.
	MLKEMPublicKeySize = 1184
	// MLKEMSecretKeySize is the size of an ML-KEM-768 secret key in bytes.
	MLKEMSecretKeySize = 2400
	// MLKEMCiphertextSize is the size of an ML-KEM-768 ciphertext in bytes.
	MLKEMCiphertextSize = 1088
	// MLKEMSharedKeySize is the size of the shared secret from ML-KEM-768 in bytes.
	MLKEMSharedKeySize = 32

	// MLDSAPublicKeySize is the size of an ML-DSA-65 public key in bytes.
	MLDSAPublicKeySize = 1952
	// MLDSASignatureSize is the size of an ML-DSA-65 signature in bytes.
	MLDSASignatureSize = 3309

	// AESKeySize is the size of an AES-256 key in bytes.
	AESKeySize = 32
	// AESNonceSize is the size of an AES-GCM nonce in bytes.
	AESNonceSize = 12
	// AESTagSize is the size of an AES-GCM authentication tag in bytes.
	AESTagSize = 16

	// PublicKeyOffset is the byte offset where the public key is embedded
	// within an ML-KEM-768 secret key.
	PublicKeyOffset = 1152
)

// AlgsCiphersuite is the canonical string representation of the algorithm suite.
var AlgsCiphersuite = "ML-KEM-768:ML-DSA-65:AES-256-GCM:HKDF-SHA-512"
