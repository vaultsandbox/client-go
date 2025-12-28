package apierrors

import (
	"errors"
	"fmt"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *APIError
		expected string
	}{
		{
			name:     "status code only",
			err:      &APIError{StatusCode: 500},
			expected: "API error 500",
		},
		{
			name:     "with message",
			err:      &APIError{StatusCode: 400, Message: "bad request"},
			expected: "API error 400: bad request",
		},
		{
			name:     "with request ID",
			err:      &APIError{StatusCode: 500, RequestID: "req-123"},
			expected: "API error 500 (request_id: req-123)",
		},
		{
			name:     "with message and request ID",
			err:      &APIError{StatusCode: 503, Message: "service unavailable", RequestID: "req-456"},
			expected: "API error 503: service unavailable (request_id: req-456)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAPIError_Is(t *testing.T) {
	tests := []struct {
		name     string
		err      *APIError
		target   error
		expected bool
	}{
		{
			name:     "401 matches ErrUnauthorized",
			err:      &APIError{StatusCode: 401},
			target:   ErrUnauthorized,
			expected: true,
		},
		{
			name:     "401 does not match ErrInboxNotFound",
			err:      &APIError{StatusCode: 401},
			target:   ErrInboxNotFound,
			expected: false,
		},
		{
			name:     "404 with inbox resource matches ErrInboxNotFound",
			err:      &APIError{StatusCode: 404, ResourceType: ResourceInbox},
			target:   ErrInboxNotFound,
			expected: true,
		},
		{
			name:     "404 with inbox resource does not match ErrEmailNotFound",
			err:      &APIError{StatusCode: 404, ResourceType: ResourceInbox},
			target:   ErrEmailNotFound,
			expected: false,
		},
		{
			name:     "404 with email resource matches ErrEmailNotFound",
			err:      &APIError{StatusCode: 404, ResourceType: ResourceEmail},
			target:   ErrEmailNotFound,
			expected: true,
		},
		{
			name:     "404 with email resource does not match ErrInboxNotFound",
			err:      &APIError{StatusCode: 404, ResourceType: ResourceEmail},
			target:   ErrInboxNotFound,
			expected: false,
		},
		{
			name:     "404 without resource type matches ErrInboxNotFound",
			err:      &APIError{StatusCode: 404},
			target:   ErrInboxNotFound,
			expected: true,
		},
		{
			name:     "404 without resource type matches ErrEmailNotFound",
			err:      &APIError{StatusCode: 404},
			target:   ErrEmailNotFound,
			expected: true,
		},
		{
			name:     "409 matches ErrInboxAlreadyExists",
			err:      &APIError{StatusCode: 409},
			target:   ErrInboxAlreadyExists,
			expected: true,
		},
		{
			name:     "429 matches ErrRateLimited",
			err:      &APIError{StatusCode: 429},
			target:   ErrRateLimited,
			expected: true,
		},
		{
			name:     "500 does not match any sentinel",
			err:      &APIError{StatusCode: 500},
			target:   ErrUnauthorized,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Is(tt.target)
			if got != tt.expected {
				t.Errorf("Is(%v) = %v, want %v", tt.target, got, tt.expected)
			}
		})
	}
}

func TestAPIError_ErrorsIs(t *testing.T) {
	// Test that errors.Is works correctly with APIError
	err := &APIError{StatusCode: 401}
	if !errors.Is(err, ErrUnauthorized) {
		t.Error("errors.Is should match ErrUnauthorized for 401")
	}

	err = &APIError{StatusCode: 404, ResourceType: ResourceInbox}
	if !errors.Is(err, ErrInboxNotFound) {
		t.Error("errors.Is should match ErrInboxNotFound for 404 inbox")
	}
}

func TestWithResourceType(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		resourceType ResourceType
		checkResult  func(t *testing.T, result error)
	}{
		{
			name:         "nil error returns nil",
			err:          nil,
			resourceType: ResourceInbox,
			checkResult: func(t *testing.T, result error) {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
			},
		},
		{
			name:         "APIError gets resource type",
			err:          &APIError{StatusCode: 404, Message: "not found"},
			resourceType: ResourceInbox,
			checkResult: func(t *testing.T, result error) {
				apiErr, ok := result.(*APIError)
				if !ok {
					t.Fatal("expected *APIError")
				}
				if apiErr.ResourceType != ResourceInbox {
					t.Errorf("ResourceType = %v, want %v", apiErr.ResourceType, ResourceInbox)
				}
				if apiErr.StatusCode != 404 {
					t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
				}
				if apiErr.Message != "not found" {
					t.Errorf("Message = %q, want %q", apiErr.Message, "not found")
				}
			},
		},
		{
			name:         "non-APIError returned unchanged",
			err:          fmt.Errorf("some other error"),
			resourceType: ResourceEmail,
			checkResult: func(t *testing.T, result error) {
				if result.Error() != "some other error" {
					t.Errorf("expected original error, got %v", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WithResourceType(tt.err, tt.resourceType)
			tt.checkResult(t, result)
		})
	}
}

func TestNetworkError_Error(t *testing.T) {
	underlying := fmt.Errorf("connection refused")
	err := &NetworkError{Err: underlying}

	expected := "network error: connection refused"
	if got := err.Error(); got != expected {
		t.Errorf("Error() = %q, want %q", got, expected)
	}
}

func TestNetworkError_Unwrap(t *testing.T) {
	underlying := fmt.Errorf("connection refused")
	err := &NetworkError{Err: underlying}

	if unwrapped := err.Unwrap(); unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}

	// Test with errors.Unwrap
	if errors.Unwrap(err) != underlying {
		t.Error("errors.Unwrap should return underlying error")
	}
}

func TestSignatureVerificationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *SignatureVerificationError
		expected string
	}{
		{
			name:     "regular verification failure",
			err:      &SignatureVerificationError{Message: "invalid signature"},
			expected: "signature verification failed: invalid signature",
		},
		{
			name:     "key mismatch",
			err:      &SignatureVerificationError{Message: "unexpected server key", IsKeyMismatch: true},
			expected: "server key mismatch: unexpected server key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSignatureVerificationError_Is(t *testing.T) {
	err := &SignatureVerificationError{Message: "test"}

	if !err.Is(ErrSignatureInvalid) {
		t.Error("Is(ErrSignatureInvalid) should return true")
	}

	if err.Is(ErrUnauthorized) {
		t.Error("Is(ErrUnauthorized) should return false")
	}

	// Test with errors.Is
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Error("errors.Is should match ErrSignatureInvalid")
	}
}

func TestSentinelErrors(t *testing.T) {
	// Verify all sentinel errors are properly defined
	sentinels := []error{
		ErrMissingAPIKey,
		ErrClientClosed,
		ErrUnauthorized,
		ErrInboxNotFound,
		ErrEmailNotFound,
		ErrInboxAlreadyExists,
		ErrInvalidImportData,
		ErrDecryptionFailed,
		ErrSignatureInvalid,
		ErrRateLimited,
	}

	for _, err := range sentinels {
		if err == nil {
			t.Error("sentinel error should not be nil")
		}
		if err.Error() == "" {
			t.Error("sentinel error message should not be empty")
		}
	}
}

func TestResourceTypeConstants(t *testing.T) {
	if ResourceUnknown != "" {
		t.Errorf("ResourceUnknown = %q, want empty string", ResourceUnknown)
	}
	if ResourceInbox != "inbox" {
		t.Errorf("ResourceInbox = %q, want 'inbox'", ResourceInbox)
	}
	if ResourceEmail != "email" {
		t.Errorf("ResourceEmail = %q, want 'email'", ResourceEmail)
	}
}
