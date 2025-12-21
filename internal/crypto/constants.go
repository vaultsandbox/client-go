package crypto

const (
	HKDFContext = "vaultsandbox:email:v1"

	// ML-KEM-768 key sizes
	MLKEMPublicKeySize  = 1184
	MLKEMSecretKeySize  = 2400
	MLKEMCiphertextSize = 1088
	MLKEMSharedKeySize  = 32

	// ML-DSA-65 key sizes
	MLDSAPublicKeySize = 1952
	MLDSASignatureSize = 3309

	// AES-256-GCM sizes
	AESKeySize   = 32
	AESNonceSize = 12
	AESTagSize   = 16

	// Offset for extracting public key from secret key
	PublicKeyOffset = 1152
)

var AlgsCiphersuite = "ML-KEM-768:ML-DSA-65:AES-256-GCM:HKDF-SHA-512"
