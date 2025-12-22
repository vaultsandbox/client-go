package api

import (
	"errors"
	"fmt"
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

// ResourceType indicates which type of resource an error relates to.
type ResourceType string

const (
	// ResourceUnknown indicates the resource type is not specified.
	ResourceUnknown ResourceType = ""
	// ResourceInbox indicates the error relates to an inbox.
	ResourceInbox ResourceType = "inbox"
	// ResourceEmail indicates the error relates to an email.
	ResourceEmail ResourceType = "email"
)

// APIError represents an HTTP error from the VaultSandbox API.
type APIError struct {
	StatusCode   int
	Message      string
	RequestID    string
	ResourceType ResourceType
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
		// Use ResourceType for precise error matching
		switch e.ResourceType {
		case ResourceInbox:
			return target == ErrInboxNotFound
		case ResourceEmail:
			return target == ErrEmailNotFound
		default:
			// Fallback: match both for unknown resource type
			return target == ErrInboxNotFound || target == ErrEmailNotFound
		}
	case 409:
		return target == ErrInboxAlreadyExists
	case 429:
		return target == ErrRateLimited
	}
	return false
}

// WithResourceType returns a copy of the error with the resource type set.
// If the error is not an *APIError, it is returned unchanged.
func WithResourceType(err error, rt ResourceType) error {
	if err == nil {
		return nil
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return &APIError{
			StatusCode:   apiErr.StatusCode,
			Message:      apiErr.Message,
			RequestID:    apiErr.RequestID,
			ResourceType: rt,
		}
	}
	return err
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

