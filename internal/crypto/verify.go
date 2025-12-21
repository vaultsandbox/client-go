package crypto

import (
	"fmt"

	"github.com/cloudflare/circl/sign/mldsa/mldsa65"
)

// Verify verifies an ML-DSA-65 signature.
func Verify(publicKey, message, signature []byte) error {
	pk := &mldsa65.PublicKey{}
	if err := pk.UnmarshalBinary(publicKey); err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	if !mldsa65.Verify(pk, message, nil, signature) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}
