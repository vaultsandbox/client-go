package api

import (
	"errors"
	"fmt"
)

// Sentinel errors for errors.Is() checks
var (
	ErrUnauthorized      = errors.New("invalid or expired API key")
	ErrInboxNotFound     = errors.New("inbox not found")
	ErrEmailNotFound     = errors.New("email not found")
	ErrInboxAlreadyExists = errors.New("inbox already exists")
	ErrInvalidAPIKey     = errors.New("invalid API key")
)

// APIError represents an HTTP error from the VaultSandbox API.
type APIError struct {
	StatusCode int
	Message    string
	RequestID  string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error %d", e.StatusCode)
}

// Is implements errors.Is for sentinel error matching.
func (e *APIError) Is(target error) bool {
	switch e.StatusCode {
	case 401:
		return target == ErrUnauthorized || target == ErrInvalidAPIKey
	case 404:
		return target == ErrInboxNotFound || target == ErrEmailNotFound
	case 409:
		return target == ErrInboxAlreadyExists
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

// Error is an alias for APIError for backward compatibility.
type Error = APIError
