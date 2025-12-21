package api

import (
	"errors"
	"fmt"
	"strings"
)

// Common API errors that can be checked with errors.Is.
var (
	// ErrUnauthorized indicates the API key is invalid or expired.
	ErrUnauthorized = errors.New("invalid or expired API key")
	// ErrInboxNotFound indicates the requested inbox does not exist.
	ErrInboxNotFound = errors.New("inbox not found")
	// ErrEmailNotFound indicates the requested email does not exist.
	ErrEmailNotFound = errors.New("email not found")
	// ErrInboxAlreadyExists indicates an inbox with that address already exists.
	ErrInboxAlreadyExists = errors.New("inbox already exists")
	// ErrInvalidAPIKey indicates the API key format is invalid.
	ErrInvalidAPIKey = errors.New("invalid API key")
	// ErrRateLimited indicates the rate limit has been exceeded.
	ErrRateLimited = errors.New("rate limit exceeded")
)

// APIError represents an HTTP error from the VaultSandbox API.
type APIError struct {
	StatusCode int
	Message    string
	RequestID  string
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
		return target == ErrUnauthorized || target == ErrInvalidAPIKey
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

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// VaultSandboxError implements the VaultSandboxError interface.
func (e *NetworkError) VaultSandboxError() {}

// Error is an alias for APIError for backward compatibility.
type Error = APIError
