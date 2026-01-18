package vaultsandbox

import (
	"github.com/vaultsandbox/client-go/internal/apierrors"
)

// Sentinel errors for errors.Is() checks - re-exported from internal package
var (
	// ErrMissingAPIKey is returned when no API key is provided.
	ErrMissingAPIKey = apierrors.ErrMissingAPIKey

	// ErrClientClosed is returned when operations are attempted on a closed client.
	ErrClientClosed = apierrors.ErrClientClosed

	// ErrUnauthorized is returned when the API key is invalid or expired.
	ErrUnauthorized = apierrors.ErrUnauthorized

	// ErrInboxNotFound is returned when an inbox is not found.
	ErrInboxNotFound = apierrors.ErrInboxNotFound

	// ErrEmailNotFound is returned when an email is not found.
	ErrEmailNotFound = apierrors.ErrEmailNotFound

	// ErrInboxAlreadyExists is returned when trying to import an inbox that already exists.
	ErrInboxAlreadyExists = apierrors.ErrInboxAlreadyExists

	// ErrInvalidImportData is returned when imported inbox data is invalid.
	ErrInvalidImportData = apierrors.ErrInvalidImportData

	// ErrDecryptionFailed is returned when email decryption fails.
	ErrDecryptionFailed = apierrors.ErrDecryptionFailed

	// ErrSignatureInvalid is returned when signature verification fails.
	ErrSignatureInvalid = apierrors.ErrSignatureInvalid

	// ErrRateLimited is returned when the API rate limit is exceeded.
	ErrRateLimited = apierrors.ErrRateLimited

	// ErrWebhookNotFound is returned when a webhook is not found.
	ErrWebhookNotFound = apierrors.ErrWebhookNotFound
)

// ResourceType indicates which type of resource an error relates to.
type ResourceType = apierrors.ResourceType

const (
	// ResourceUnknown indicates the resource type is not specified.
	ResourceUnknown = apierrors.ResourceUnknown
	// ResourceInbox indicates the error relates to an inbox.
	ResourceInbox = apierrors.ResourceInbox
	// ResourceEmail indicates the error relates to an email.
	ResourceEmail = apierrors.ResourceEmail
	// ResourceWebhook indicates the error relates to a webhook.
	ResourceWebhook = apierrors.ResourceWebhook
)

// APIError represents an HTTP error from the VaultSandbox API.
type APIError = apierrors.APIError

// NetworkError represents a network-level failure.
type NetworkError = apierrors.NetworkError

// SignatureVerificationError indicates signature verification failed,
// including server key mismatch (potential MITM attack).
type SignatureVerificationError = apierrors.SignatureVerificationError
