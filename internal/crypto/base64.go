package crypto

import (
	"encoding/base64"
)

// EncodeBase64 encodes bytes to base64url without padding.
func EncodeBase64(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// DecodeBase64 decodes base64url (with or without padding) to bytes.
func DecodeBase64(s string) ([]byte, error) {
	// Try without padding first
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err == nil {
		return data, nil
	}

	// Try with padding
	data, err = base64.URLEncoding.DecodeString(s)
	if err == nil {
		return data, nil
	}

	// Try standard base64 without padding
	data, err = base64.RawStdEncoding.DecodeString(s)
	if err == nil {
		return data, nil
	}

	// Try standard base64 with padding
	return base64.StdEncoding.DecodeString(s)
}
