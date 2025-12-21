package vaultsandbox

import "errors"

// ErrMissingAPIKey is returned when no API key is provided.
var ErrMissingAPIKey = errors.New("API key is required")

// ErrClientClosed is returned when operations are attempted on a closed client.
var ErrClientClosed = errors.New("client is closed")

// ErrInboxAlreadyExists is returned when trying to import an inbox that already exists.
var ErrInboxAlreadyExists = errors.New("inbox already exists")

// ErrInboxNotFound is returned when an inbox is not found.
var ErrInboxNotFound = errors.New("inbox not found")

// ErrEmailNotFound is returned when an email is not found.
var ErrEmailNotFound = errors.New("email not found")

// ErrTimeout is returned when an operation times out.
var ErrTimeout = errors.New("operation timed out")

// ErrInboxExpired is returned when an inbox has expired.
var ErrInboxExpired = errors.New("inbox has expired")

// ErrDecryptionFailed is returned when email decryption fails.
var ErrDecryptionFailed = errors.New("email decryption failed")

// ErrSignatureInvalid is returned when signature verification fails.
var ErrSignatureInvalid = errors.New("signature verification failed")

// ErrAPIKeyInvalid is returned when the API key is invalid.
var ErrAPIKeyInvalid = errors.New("invalid API key")

// ErrRateLimited is returned when the API rate limit is exceeded.
var ErrRateLimited = errors.New("rate limit exceeded")

// ErrInvalidImportData is returned when imported inbox data is invalid.
var ErrInvalidImportData = errors.New("invalid import data")

// APIError represents an error from the API.
type APIError struct {
	StatusCode int
	Message    string
	RequestID  string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "API error"
}
