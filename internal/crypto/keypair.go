package crypto

import (
	"github.com/cloudflare/circl/kem/mlkem/mlkem768"
)

// Keypair represents an ML-KEM-768 keypair.
type Keypair struct {
	PublicKey    []byte
	SecretKey    []byte
	PublicKeyB64 string
}

// GenerateKeypair creates a new ML-KEM-768 keypair.
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

// KeypairFromSecretKey reconstructs a keypair from the secret key.
// The public key is embedded in the secret key at offset 1152.
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

// NewKeypairFromBytes creates a keypair from raw bytes.
// This is provided for backward compatibility.
func NewKeypairFromBytes(privateKeyBytes, publicKeyBytes []byte) (*Keypair, error) {
	if len(privateKeyBytes) != MLKEMSecretKeySize {
		return nil, ErrInvalidSecretKeySize
	}
	if len(publicKeyBytes) != MLKEMPublicKeySize {
		return nil, ErrInvalidPublicKeySize
	}

	// Validate that keys can be parsed
	priv := &mlkem768.PrivateKey{}
	if err := priv.Unpack(privateKeyBytes); err != nil {
		return nil, err
	}

	pub := &mlkem768.PublicKey{}
	if err := pub.Unpack(publicKeyBytes); err != nil {
		return nil, err
	}

	return &Keypair{
		PublicKey:    publicKeyBytes,
		SecretKey:    privateKeyBytes,
		PublicKeyB64: ToBase64URL(publicKeyBytes),
	}, nil
}

// Decapsulate decapsulates a shared secret from the encapsulated key.
func (k *Keypair) Decapsulate(encapsulatedKey []byte) ([]byte, error) {
	if len(encapsulatedKey) != MLKEMCiphertextSize {
		return nil, ErrInvalidCiphertextSize
	}

	var privKey mlkem768.PrivateKey
	if err := privKey.Unpack(k.SecretKey); err != nil {
		return nil, err
	}

	sharedSecret := make([]byte, MLKEMSharedKeySize)
	privKey.DecapsulateTo(sharedSecret, encapsulatedKey)

	return sharedSecret, nil
}
