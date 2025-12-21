package crypto

import (
	"fmt"

	"github.com/cloudflare/circl/kem/mlkem/mlkem768"
)

// SharedSecretSize is the size of the shared secret in bytes.
const SharedSecretSize = 32

// CiphertextSize is the size of the ciphertext in bytes.
const CiphertextSize = 1088

// Keypair holds ML-KEM-768 key pair.
type Keypair struct {
	privateKey *mlkem768.PrivateKey
	publicKey  *mlkem768.PublicKey
}

// GenerateKeypair generates a new ML-KEM-768 key pair.
func GenerateKeypair() (*Keypair, error) {
	pub, priv, err := mlkem768.GenerateKeyPair(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ML-KEM-768 keypair: %w", err)
	}

	return &Keypair{
		privateKey: priv,
		publicKey:  pub,
	}, nil
}

// NewKeypairFromBytes creates a keypair from raw bytes.
func NewKeypairFromBytes(privateKeyBytes, publicKeyBytes []byte) (*Keypair, error) {
	priv := &mlkem768.PrivateKey{}
	if err := priv.Unpack(privateKeyBytes); err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	pub := &mlkem768.PublicKey{}
	if err := pub.Unpack(publicKeyBytes); err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return &Keypair{
		privateKey: priv,
		publicKey:  pub,
	}, nil
}

// PrivateKey returns the private key bytes.
func (k *Keypair) PrivateKey() []byte {
	data, _ := k.privateKey.MarshalBinary()
	return data
}

// PublicKey returns the public key bytes.
func (k *Keypair) PublicKey() []byte {
	data, _ := k.publicKey.MarshalBinary()
	return data
}

// Decapsulate decapsulates a shared secret from the encapsulated key.
func (k *Keypair) Decapsulate(encapsulatedKey []byte) ([]byte, error) {
	if len(encapsulatedKey) != CiphertextSize {
		return nil, fmt.Errorf("invalid ciphertext size: expected %d, got %d", CiphertextSize, len(encapsulatedKey))
	}

	sharedSecret := make([]byte, SharedSecretSize)
	k.privateKey.DecapsulateTo(sharedSecret, encapsulatedKey)
	return sharedSecret, nil
}
