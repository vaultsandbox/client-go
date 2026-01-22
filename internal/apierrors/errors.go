// Package apierrors provides shared error types for the VaultSandbox client.
package apierrors

import (
	"errors"
	"fmt"
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

	// ErrRateLimited is returned when the API rate limit is exceeded.
	ErrRateLimited = errors.New("rate limit exceeded")

	// ErrWebhookNotFound is returned when a webhook is not found.
	ErrWebhookNotFound = errors.New("webhook not found")

	// ErrChaosDisabled is returned when chaos is disabled globally on the server.
	ErrChaosDisabled = errors.New("chaos is disabled on this server")
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
	// ResourceWebhook indicates the error relates to a webhook.
	ResourceWebhook ResourceType = "webhook"
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

// Is implements errors.Is for sentinel error matching.
func (e *APIError) Is(target error) bool {
	switch e.StatusCode {
	case 401:
		return target == ErrUnauthorized
	case 404:
		switch e.ResourceType {
		case ResourceInbox:
			return target == ErrInboxNotFound
		case ResourceEmail:
			return target == ErrEmailNotFound
		case ResourceWebhook:
			return target == ErrWebhookNotFound
		default:
			return target == ErrInboxNotFound || target == ErrEmailNotFound || target == ErrWebhookNotFound
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
	Err error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error: %v", e.Err)
}

// Unwrap returns the underlying error.
func (e *NetworkError) Unwrap() error {
	return e.Err
}

// SignatureVerificationError indicates signature verification failed,
// including server key mismatch (potential MITM attack).
type SignatureVerificationError struct {
	Message       string
	IsKeyMismatch bool
}

func (e *SignatureVerificationError) Error() string {
	if e.IsKeyMismatch {
		return fmt.Sprintf("server key mismatch: %s", e.Message)
	}
	return fmt.Sprintf("signature verification failed: %s", e.Message)
}

// Is implements errors.Is for sentinel error matching.
// All signature verification failures match ErrSignatureInvalid.
func (e *SignatureVerificationError) Is(target error) bool {
	return target == ErrSignatureInvalid
}
