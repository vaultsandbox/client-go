package crypto

import (
	"encoding/base64"
)

// Base64 encoding functions for cryptographic data.
//
// This package provides two encoding variants:
//   - URL-safe (base64url): Uses - and _ instead of + and /, no padding.
//     Preferred for URLs, JSON, and cryptographic keys/ciphertexts.
//   - Standard (base64): Uses + and /, with = padding.
//     Used for attachment content and general binary data.
//
// The VaultSandbox protocol uses URL-safe base64 without padding for all
// cryptographic values (keys, nonces, ciphertexts, signatures).

// ToBase64URL encodes bytes to URL-safe base64 without padding (RFC 4648 §5).
// This is the preferred encoding for cryptographic values in the protocol.
func ToBase64URL(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// FromBase64URL decodes URL-safe base64 without padding (RFC 4648 §5).
// Returns an error if the input contains invalid characters.
func FromBase64URL(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// EncodeBase64 is an alias for [ToBase64URL] for backward compatibility.
// Deprecated: Use [ToBase64URL] instead.
func EncodeBase64(data []byte) string {
	return ToBase64URL(data)
}

// DecodeBase64 decodes base64 data with automatic format detection.
// It tries multiple encodings in order:
//  1. URL-safe without padding (base64url, RFC 4648 §5)
//  2. URL-safe with padding
//  3. Standard without padding
//  4. Standard with padding (RFC 4648 §4)
//
// This lenient decoding is useful when the exact encoding format is unknown.
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

// ToBase64 encodes bytes to standard base64 with padding (RFC 4648 §4).
// Use this for attachment content and non-URL contexts where the standard
// alphabet (+ and /) is acceptable.
func ToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// FromBase64 decodes standard base64 with padding (RFC 4648 §4).
// Use this for attachment content. For cryptographic values from the API,
// use [FromBase64URL] instead.
func FromBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
