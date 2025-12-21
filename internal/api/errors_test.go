package api

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
		{"401 matches ErrInvalidAPIKey", 401, ErrInvalidAPIKey, true},
		{"404 matches ErrInboxNotFound", 404, ErrInboxNotFound, true},
		{"404 matches ErrEmailNotFound", 404, ErrEmailNotFound, true},
		{"409 matches ErrInboxAlreadyExists", 409, ErrInboxAlreadyExists, true},
		{"429 matches ErrRateLimited", 429, ErrRateLimited, true},
		{"500 does not match ErrUnauthorized", 500, ErrUnauthorized, false},
		{"401 does not match ErrInboxNotFound", 401, ErrInboxNotFound, false},
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

func TestAPIError_VaultSandboxError(t *testing.T) {
	err := &APIError{StatusCode: 400}
	// This just verifies the method exists and is callable
	err.VaultSandboxError()
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

func TestNetworkError_As(t *testing.T) {
	underlying := fmt.Errorf("wrapped: %w", errors.New("root error"))
	err := &NetworkError{Err: underlying}

	var netErr *NetworkError
	if !errors.As(err, &netErr) {
		t.Error("errors.As() should match NetworkError")
	}
}

func TestNetworkError_VaultSandboxError(t *testing.T) {
	err := &NetworkError{}
	// This just verifies the method exists and is callable
	err.VaultSandboxError()
}

func TestNetworkError_WithFields(t *testing.T) {
	err := &NetworkError{
		Err:     errors.New("timeout"),
		URL:     "https://example.com/api",
		Attempt: 3,
	}

	if err.URL != "https://example.com/api" {
		t.Errorf("URL = %s, want https://example.com/api", err.URL)
	}
	if err.Attempt != 3 {
		t.Errorf("Attempt = %d, want 3", err.Attempt)
	}
}

func TestSentinelErrors(t *testing.T) {
	// Verify sentinel errors are properly defined
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrUnauthorized", ErrUnauthorized},
		{"ErrInboxNotFound", ErrInboxNotFound},
		{"ErrEmailNotFound", ErrEmailNotFound},
		{"ErrInboxAlreadyExists", ErrInboxAlreadyExists},
		{"ErrInvalidAPIKey", ErrInvalidAPIKey},
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

func TestErrorTypeAlias(t *testing.T) {
	// Error is an alias for APIError
	var err Error = APIError{StatusCode: 400, Message: "test"}
	if err.StatusCode != 400 {
		t.Errorf("StatusCode = %d, want 400", err.StatusCode)
	}
}
