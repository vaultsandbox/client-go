package crypto

import (
	"encoding/base64"
)

// ToBase64URL encodes bytes to URL-safe base64 without padding.
func ToBase64URL(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// FromBase64URL decodes URL-safe base64 (handles missing padding).
func FromBase64URL(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// EncodeBase64 is an alias for ToBase64URL for backward compatibility.
func EncodeBase64(data []byte) string {
	return ToBase64URL(data)
}

// DecodeBase64 decodes base64url (with or without padding) to bytes.
// This version is more lenient and tries multiple encodings.
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

// ToBase64 encodes bytes to standard base64 with padding.
// Use this for attachment content and non-URL contexts.
func ToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// FromBase64 decodes standard base64 (with padding) to bytes.
func FromBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
