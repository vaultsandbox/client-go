package vaultsandbox

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

// Sentinel errors for errors.Is() checks
var (
	// ErrMissingAPIKey is returned when no API key is provided.
	ErrMissingAPIKey = errors.New("API key is required")

	// ErrClientClosed is returned when operations are attempted on a closed client.
	ErrClientClosed = errors.New("client has been closed")

	// ErrUnauthorized is returned when the API key is invalid or expired.
	ErrUnauthorized = errors.New("invalid or expired API key")

	// ErrInboxNotFound is returned when an inbox is not found.
	ErrInboxNotFound = errors.New("inbox not found")

	// ErrEmailNotFound is returned when an email is not found.
	ErrEmailNotFound = errors.New("email not found")

	// ErrInboxAlreadyExists is returned when trying to import an inbox that already exists.
	ErrInboxAlreadyExists = errors.New("inbox already exists")

	// ErrInvalidImportData is returned when imported inbox data is invalid.
	ErrInvalidImportData = errors.New("invalid import data")

	// ErrDecryptionFailed is returned when email decryption fails.
	ErrDecryptionFailed = errors.New("decryption failed")

	// ErrSignatureInvalid is returned when signature verification fails.
	ErrSignatureInvalid = errors.New("signature verification failed")

	// ErrSSEConnection is returned when SSE connection fails.
	ErrSSEConnection = errors.New("SSE connection error")

	// ErrInvalidSecretKeySize is returned when the secret key size is invalid.
	ErrInvalidSecretKeySize = errors.New("invalid secret key size")

	// ErrInboxExpired is returned when an inbox has expired.
	ErrInboxExpired = errors.New("inbox has expired")

	// ErrRateLimited is returned when the API rate limit is exceeded.
	ErrRateLimited = errors.New("rate limit exceeded")
)

// VaultSandboxError is implemented by all SDK errors.
type VaultSandboxError interface {
	error
	VaultSandboxError() // marker method
}

// APIError represents an HTTP error from the VaultSandbox API.
type APIError struct {
	StatusCode int
	Message    string
	RequestID  string // if returned by server
}

func (e *APIError) Error() string {
	if e.RequestID != "" {
		if e.Message != "" {
			return fmt.Sprintf("API error %d: %s (request_id: %s)", e.StatusCode, e.Message, e.RequestID)
		}
		return fmt.Sprintf("API error %d (request_id: %s)", e.StatusCode, e.RequestID)
	}
	if e.Message != "" {
		return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error %d", e.StatusCode)
}

// VaultSandboxError implements the VaultSandboxError interface.
func (e *APIError) VaultSandboxError() {}

// Is implements errors.Is for sentinel error matching.
func (e *APIError) Is(target error) bool {
	switch e.StatusCode {
	case 401:
		return target == ErrUnauthorized
	case 404:
		// Check message content to distinguish inbox vs email errors
		// Matches Node SDK behavior
		msgLower := strings.ToLower(e.Message)
		hasInbox := strings.Contains(msgLower, "inbox")
		hasEmail := strings.Contains(msgLower, "email")

		if target == ErrInboxNotFound {
			// Match if message contains "inbox" OR no specific keyword (backward compat)
			return hasInbox || (!hasInbox && !hasEmail)
		}
		if target == ErrEmailNotFound {
			// Match if message contains "email" OR no specific keyword (backward compat)
			return hasEmail || (!hasInbox && !hasEmail)
		}
		return false
	case 409:
		return target == ErrInboxAlreadyExists
	case 429:
		return target == ErrRateLimited
	}
	return false
}

// NetworkError represents a network-level failure.
type NetworkError struct {
	Err     error
	URL     string
	Attempt int
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error: %v", e.Err)
}

// Unwrap returns the underlying error.
func (e *NetworkError) Unwrap() error {
	return e.Err
}

// VaultSandboxError implements the VaultSandboxError interface.
func (e *NetworkError) VaultSandboxError() {}

// TimeoutError represents an operation that exceeded its deadline.
type TimeoutError struct {
	Operation string
	Timeout   time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("%s timed out after %v", e.Operation, e.Timeout)
}

// VaultSandboxError implements the VaultSandboxError interface.
func (e *TimeoutError) VaultSandboxError() {}

// DecryptionError represents a failure to decrypt email content.
type DecryptionError struct {
	Stage   string // "kem", "hkdf", "aes"
	Message string
	Err     error
}

func (e *DecryptionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("decryption failed at %s: %v", e.Stage, e.Err)
	}
	return fmt.Sprintf("decryption failed at %s: %s", e.Stage, e.Message)
}

// Unwrap returns the underlying error.
func (e *DecryptionError) Unwrap() error {
	return e.Err
}

// Is implements errors.Is for sentinel error matching.
func (e *DecryptionError) Is(target error) bool {
	return target == ErrDecryptionFailed
}

// VaultSandboxError implements the VaultSandboxError interface.
func (e *DecryptionError) VaultSandboxError() {}

// SignatureVerificationError indicates potential tampering.
type SignatureVerificationError struct {
	Message string
}

func (e *SignatureVerificationError) Error() string {
	return fmt.Sprintf("signature verification failed: %s", e.Message)
}

// Is implements errors.Is for sentinel error matching.
func (e *SignatureVerificationError) Is(target error) bool {
	return target == ErrSignatureInvalid
}

// VaultSandboxError implements the VaultSandboxError interface.
func (e *SignatureVerificationError) VaultSandboxError() {}

// SSEError represents an SSE connection failure.
type SSEError struct {
	Err      error
	Attempts int
}

func (e *SSEError) Error() string {
	return fmt.Sprintf("SSE connection failed after %d attempts: %v", e.Attempts, e.Err)
}

// Unwrap returns the underlying error.
func (e *SSEError) Unwrap() error {
	return e.Err
}

// Is implements errors.Is for sentinel error matching.
func (e *SSEError) Is(target error) bool {
	return target == ErrSSEConnection
}

// VaultSandboxError implements the VaultSandboxError interface.
func (e *SSEError) VaultSandboxError() {}

// ValidationError contains multiple validation failures.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed: %v", e.Errors)
}

// VaultSandboxError implements the VaultSandboxError interface.
func (e *ValidationError) VaultSandboxError() {}

// StrategyError indicates a delivery strategy failure.
type StrategyError struct {
	Message string
	Err     error
}

func (e *StrategyError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("strategy error: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("strategy error: %s", e.Message)
}

// Unwrap returns the underlying error.
func (e *StrategyError) Unwrap() error {
	return e.Err
}

// VaultSandboxError implements the VaultSandboxError interface.
func (e *StrategyError) VaultSandboxError() {}

// wrapError converts internal API errors to public errors.
// This ensures that errors.Is() checks work with public sentinel errors.
func wrapError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		return &APIError{
			StatusCode: apiErr.StatusCode,
			Message:    apiErr.Message,
			RequestID:  apiErr.RequestID,
		}
	}

	var netErr *api.NetworkError
	if errors.As(err, &netErr) {
		return &NetworkError{
			Err:     netErr.Err,
			URL:     netErr.URL,
			Attempt: netErr.Attempt,
		}
	}

	return err
}
