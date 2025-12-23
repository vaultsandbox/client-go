package vaultsandbox

import (
	"errors"
	"fmt"
	"testing"

	"github.com/vaultsandbox/client-go/internal/api"
)

func TestSentinelErrors(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrMissingAPIKey", ErrMissingAPIKey},
		{"ErrClientClosed", ErrClientClosed},
		{"ErrUnauthorized", ErrUnauthorized},
		{"ErrInboxNotFound", ErrInboxNotFound},
		{"ErrEmailNotFound", ErrEmailNotFound},
		{"ErrInboxAlreadyExists", ErrInboxAlreadyExists},
		{"ErrInvalidImportData", ErrInvalidImportData},
		{"ErrDecryptionFailed", ErrDecryptionFailed},
		{"ErrSignatureInvalid", ErrSignatureInvalid},
		{"ErrRateLimited", ErrRateLimited},
	}

	for _, s := range sentinels {
		t.Run(s.name, func(t *testing.T) {
			if s.err == nil {
				t.Error("sentinel error is nil")
			}
			if s.err.Error() == "" {
				t.Error("sentinel error has empty message")
			}
		})
	}
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *APIError
		expected string
	}{
		{
			name:     "with message",
			err:      &APIError{StatusCode: 401, Message: "invalid API key"},
			expected: "API error 401: invalid API key",
		},
		{
			name:     "without message",
			err:      &APIError{StatusCode: 500},
			expected: "API error 500",
		},
		{
			name:     "with request ID",
			err:      &APIError{StatusCode: 404, Message: "not found", RequestID: "req-123"},
			expected: "API error 404: not found (request_id: req-123)",
		},
		{
			name:     "with request ID only",
			err:      &APIError{StatusCode: 500, RequestID: "req-456"},
			expected: "API error 500 (request_id: req-456)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Error() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestAPIError_Is(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		target     error
		expected   bool
	}{
		{"401 matches ErrUnauthorized", 401, ErrUnauthorized, true},
		{"404 matches ErrInboxNotFound", 404, ErrInboxNotFound, true},
		{"404 matches ErrEmailNotFound", 404, ErrEmailNotFound, true},
		{"409 matches ErrInboxAlreadyExists", 409, ErrInboxAlreadyExists, true},
		{"429 matches ErrRateLimited", 429, ErrRateLimited, true},
		{"500 does not match ErrUnauthorized", 500, ErrUnauthorized, false},
		{"200 does not match anything", 200, ErrUnauthorized, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &APIError{StatusCode: tt.statusCode}
			result := errors.Is(err, tt.target)
			if result != tt.expected {
				t.Errorf("errors.Is() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAPIError_Is_404Differentiation(t *testing.T) {
	tests := []struct {
		name         string
		resourceType ResourceType
		target       error
		expected     bool
	}{
		{"inbox resource matches ErrInboxNotFound", ResourceInbox, ErrInboxNotFound, true},
		{"inbox resource does not match ErrEmailNotFound", ResourceInbox, ErrEmailNotFound, false},
		{"email resource matches ErrEmailNotFound", ResourceEmail, ErrEmailNotFound, true},
		{"email resource does not match ErrInboxNotFound", ResourceEmail, ErrInboxNotFound, false},
		{"unknown resource matches ErrInboxNotFound", ResourceUnknown, ErrInboxNotFound, true},
		{"unknown resource matches ErrEmailNotFound", ResourceUnknown, ErrEmailNotFound, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &APIError{StatusCode: 404, ResourceType: tt.resourceType}
			result := errors.Is(err, tt.target)
			if result != tt.expected {
				t.Errorf("errors.Is() = %v, want %v for resource type %q", result, tt.expected, tt.resourceType)
			}
		})
	}
}

func TestNetworkError_Error(t *testing.T) {
	underlying := errors.New("connection refused")
	err := &NetworkError{Err: underlying}

	expected := "network error: connection refused"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestNetworkError_Unwrap(t *testing.T) {
	underlying := errors.New("connection refused")
	err := &NetworkError{Err: underlying}

	unwrapped := err.Unwrap()
	if unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}
}

func TestNetworkError_Is(t *testing.T) {
	underlying := errors.New("connection refused")
	err := &NetworkError{Err: underlying}

	if !errors.Is(err, underlying) {
		t.Error("errors.Is() should match underlying error")
	}
}

func TestSignatureVerificationError_Error(t *testing.T) {
	t.Run("signature failure", func(t *testing.T) {
		err := &SignatureVerificationError{Message: "tampered data", IsKeyMismatch: false}
		expected := "signature verification failed: tampered data"
		if err.Error() != expected {
			t.Errorf("Error() = %s, want %s", err.Error(), expected)
		}
	})

	t.Run("key mismatch", func(t *testing.T) {
		err := &SignatureVerificationError{Message: "payload key differs", IsKeyMismatch: true}
		expected := "server key mismatch: payload key differs"
		if err.Error() != expected {
			t.Errorf("Error() = %s, want %s", err.Error(), expected)
		}
	})
}

func TestSignatureVerificationError_Is(t *testing.T) {
	t.Run("matches ErrSignatureInvalid when not key mismatch", func(t *testing.T) {
		err := &SignatureVerificationError{IsKeyMismatch: false}
		if !errors.Is(err, ErrSignatureInvalid) {
			t.Error("errors.Is() should match ErrSignatureInvalid")
		}
	})

	t.Run("matches ErrSignatureInvalid when key mismatch", func(t *testing.T) {
		err := &SignatureVerificationError{IsKeyMismatch: true}
		if !errors.Is(err, ErrSignatureInvalid) {
			t.Error("errors.Is() should match ErrSignatureInvalid")
		}
	})
}

func TestErrorWrapping(t *testing.T) {
	root := errors.New("root cause")
	wrapped := fmt.Errorf("wrapped: %w", root)
	netErr := &NetworkError{Err: wrapped}

	if !errors.Is(netErr, root) {
		t.Error("errors.Is() should match through wrapped chain")
	}
}

func TestWrapError_PreservesAPIError(t *testing.T) {
	internalErr := &api.APIError{
		StatusCode: 401,
		Message:    "invalid API key",
		RequestID:  "req-123",
	}

	wrapped := wrapError(internalErr)

	var publicErr *APIError
	if !errors.As(wrapped, &publicErr) {
		t.Fatal("wrapError should convert internal API error to public APIError")
	}

	if publicErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", publicErr.StatusCode)
	}
	if publicErr.Message != "invalid API key" {
		t.Errorf("Message = %s, want 'invalid API key'", publicErr.Message)
	}
	if publicErr.RequestID != "req-123" {
		t.Errorf("RequestID = %s, want 'req-123'", publicErr.RequestID)
	}

	if !errors.Is(wrapped, ErrUnauthorized) {
		t.Error("wrapped error should match ErrUnauthorized sentinel")
	}
}

func TestWrapError_PreservesNetworkError(t *testing.T) {
	underlying := errors.New("connection refused")
	internalErr := &api.NetworkError{
		Err:     underlying,
		URL:     "https://api.example.com/test",
		Attempt: 3,
	}

	wrapped := wrapError(internalErr)

	var publicErr *NetworkError
	if !errors.As(wrapped, &publicErr) {
		t.Fatal("wrapError should convert internal network error to public NetworkError")
	}

	if publicErr.URL != "https://api.example.com/test" {
		t.Errorf("URL = %s, want 'https://api.example.com/test'", publicErr.URL)
	}
	if publicErr.Attempt != 3 {
		t.Errorf("Attempt = %d, want 3", publicErr.Attempt)
	}

	if !errors.Is(wrapped, underlying) {
		t.Error("wrapped error should still match underlying error")
	}
}

func TestWrapError_PassesThroughOther(t *testing.T) {
	originalErr := errors.New("some other error")

	wrapped := wrapError(originalErr)

	if wrapped != originalErr {
		t.Error("wrapError should pass through non-API/non-Network errors unchanged")
	}
}

func TestWrapError_NilReturnsNil(t *testing.T) {
	wrapped := wrapError(nil)
	if wrapped != nil {
		t.Error("wrapError(nil) should return nil")
	}
}

func TestErrorChain_CanUnwrapToSentinel(t *testing.T) {
	tests := []struct {
		name          string
		internalErr   error
		expectedMatch error
	}{
		{
			name:          "401 matches ErrUnauthorized",
			internalErr:   &api.APIError{StatusCode: 401, Message: "unauthorized"},
			expectedMatch: ErrUnauthorized,
		},
		{
			name:          "404 with inbox resource matches ErrInboxNotFound",
			internalErr:   &api.APIError{StatusCode: 404, Message: "not found", ResourceType: api.ResourceInbox},
			expectedMatch: ErrInboxNotFound,
		},
		{
			name:          "404 with email resource matches ErrEmailNotFound",
			internalErr:   &api.APIError{StatusCode: 404, Message: "not found", ResourceType: api.ResourceEmail},
			expectedMatch: ErrEmailNotFound,
		},
		{
			name:          "409 matches ErrInboxAlreadyExists",
			internalErr:   &api.APIError{StatusCode: 409, Message: "already exists"},
			expectedMatch: ErrInboxAlreadyExists,
		},
		{
			name:          "429 matches ErrRateLimited",
			internalErr:   &api.APIError{StatusCode: 429, Message: "rate limit exceeded"},
			expectedMatch: ErrRateLimited,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := wrapError(tt.internalErr)

			if !errors.Is(wrapped, tt.expectedMatch) {
				t.Errorf("wrapped error should match %v", tt.expectedMatch)
			}

			doubleWrapped := fmt.Errorf("operation failed: %w", wrapped)
			if !errors.Is(doubleWrapped, tt.expectedMatch) {
				t.Errorf("double-wrapped error should still match %v", tt.expectedMatch)
			}
		})
	}
}

func TestWrapCryptoError_PreservesKeyMismatch(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		result := wrapCryptoError(nil)
		if result != nil {
			t.Error("wrapCryptoError(nil) should return nil")
		}
	})

	t.Run("non-crypto error passes through", func(t *testing.T) {
		originalErr := errors.New("some other error")
		result := wrapCryptoError(originalErr)
		if result != originalErr {
			t.Error("wrapCryptoError should pass through non-crypto errors unchanged")
		}
	})
}
