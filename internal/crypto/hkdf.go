package crypto

import (
	"crypto/sha512"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// DeriveKey derives a key using HKDF-SHA-512.
func DeriveKey(secret, salt, info []byte, length int) ([]byte, error) {
	if len(salt) == 0 {
		salt = make([]byte, sha512.Size)
	}

	reader := hkdf.New(sha512.New, secret, salt, info)
	key := make([]byte, length)

	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	return key, nil
}
