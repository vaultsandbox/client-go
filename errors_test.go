package vaultsandbox

import (
	"errors"
	"fmt"
	"testing"
	"time"
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
		{"ErrSSEConnection", ErrSSEConnection},
		{"ErrInvalidSecretKeySize", ErrInvalidSecretKeySize},
		{"ErrInboxExpired", ErrInboxExpired},
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
		name     string
		message  string
		target   error
		expected bool
	}{
		// Message contains "inbox" - only matches ErrInboxNotFound
		{"inbox message matches ErrInboxNotFound", "inbox not found", ErrInboxNotFound, true},
		{"inbox message does not match ErrEmailNotFound", "inbox not found", ErrEmailNotFound, false},

		// Message contains "email" - only matches ErrEmailNotFound
		{"email message matches ErrEmailNotFound", "email not found", ErrEmailNotFound, true},
		{"email message does not match ErrInboxNotFound", "email not found", ErrInboxNotFound, false},

		// Message contains both - matches both (first keyword wins)
		{"both keywords matches ErrInboxNotFound", "inbox email not found", ErrInboxNotFound, true},
		{"both keywords matches ErrEmailNotFound", "inbox email not found", ErrEmailNotFound, true},

		// Empty message - matches both (backward compat)
		{"empty message matches ErrInboxNotFound", "", ErrInboxNotFound, true},
		{"empty message matches ErrEmailNotFound", "", ErrEmailNotFound, true},

		// Case insensitive
		{"INBOX uppercase matches ErrInboxNotFound", "INBOX NOT FOUND", ErrInboxNotFound, true},
		{"EMAIL uppercase matches ErrEmailNotFound", "EMAIL NOT FOUND", ErrEmailNotFound, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &APIError{StatusCode: 404, Message: tt.message}
			result := errors.Is(err, tt.target)
			if result != tt.expected {
				t.Errorf("errors.Is() = %v, want %v for message %q", result, tt.expected, tt.message)
			}
		})
	}
}

func TestAPIError_VaultSandboxError(t *testing.T) {
	err := &APIError{StatusCode: 400}
	err.VaultSandboxError() // Just verify method exists

	// Verify it implements the interface
	var _ VaultSandboxError = err
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

func TestNetworkError_VaultSandboxError(t *testing.T) {
	err := &NetworkError{}
	err.VaultSandboxError()

	var _ VaultSandboxError = err
}

func TestTimeoutError_Error(t *testing.T) {
	err := &TimeoutError{
		Operation: "WaitForEmail",
		Timeout:   60 * time.Second,
	}

	expected := "WaitForEmail timed out after 1m0s"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestTimeoutError_VaultSandboxError(t *testing.T) {
	err := &TimeoutError{}
	err.VaultSandboxError()

	var _ VaultSandboxError = err
}

func TestDecryptionError_Error(t *testing.T) {
	t.Run("with underlying error", func(t *testing.T) {
		underlying := errors.New("AES failed")
		err := &DecryptionError{Stage: "aes", Err: underlying}

		expected := "decryption failed at aes: AES failed"
		if err.Error() != expected {
			t.Errorf("Error() = %s, want %s", err.Error(), expected)
		}
	})

	t.Run("with message", func(t *testing.T) {
		err := &DecryptionError{Stage: "kem", Message: "invalid ciphertext"}

		expected := "decryption failed at kem: invalid ciphertext"
		if err.Error() != expected {
			t.Errorf("Error() = %s, want %s", err.Error(), expected)
		}
	})
}

func TestDecryptionError_Unwrap(t *testing.T) {
	underlying := errors.New("root cause")
	err := &DecryptionError{Stage: "test", Err: underlying}

	unwrapped := err.Unwrap()
	if unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}
}

func TestDecryptionError_Is(t *testing.T) {
	err := &DecryptionError{Stage: "test"}

	if !errors.Is(err, ErrDecryptionFailed) {
		t.Error("errors.Is() should match ErrDecryptionFailed")
	}
}

func TestDecryptionError_VaultSandboxError(t *testing.T) {
	err := &DecryptionError{}
	err.VaultSandboxError()

	var _ VaultSandboxError = err
}

func TestSignatureVerificationError_Error(t *testing.T) {
	err := &SignatureVerificationError{Message: "tampered data"}

	expected := "signature verification failed: tampered data"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestSignatureVerificationError_Is(t *testing.T) {
	err := &SignatureVerificationError{}

	if !errors.Is(err, ErrSignatureInvalid) {
		t.Error("errors.Is() should match ErrSignatureInvalid")
	}
}

func TestSignatureVerificationError_VaultSandboxError(t *testing.T) {
	err := &SignatureVerificationError{}
	err.VaultSandboxError()

	var _ VaultSandboxError = err
}

func TestSSEError_Error(t *testing.T) {
	underlying := errors.New("connection closed")
	err := &SSEError{Err: underlying, Attempts: 5}

	expected := "SSE connection failed after 5 attempts: connection closed"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestSSEError_Unwrap(t *testing.T) {
	underlying := errors.New("connection closed")
	err := &SSEError{Err: underlying}

	unwrapped := err.Unwrap()
	if unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}
}

func TestSSEError_Is(t *testing.T) {
	err := &SSEError{}

	if !errors.Is(err, ErrSSEConnection) {
		t.Error("errors.Is() should match ErrSSEConnection")
	}
}

func TestSSEError_VaultSandboxError(t *testing.T) {
	err := &SSEError{}
	err.VaultSandboxError()

	var _ VaultSandboxError = err
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Errors: []string{"missing field", "invalid format"}}

	result := err.Error()
	if result == "" {
		t.Error("Error() returned empty string")
	}
}

func TestValidationError_VaultSandboxError(t *testing.T) {
	err := &ValidationError{}
	err.VaultSandboxError()

	var _ VaultSandboxError = err
}

func TestStrategyError_Error(t *testing.T) {
	t.Run("with underlying error", func(t *testing.T) {
		underlying := errors.New("SSE failed")
		err := &StrategyError{Message: "delivery failed", Err: underlying}

		expected := "strategy error: delivery failed: SSE failed"
		if err.Error() != expected {
			t.Errorf("Error() = %s, want %s", err.Error(), expected)
		}
	})

	t.Run("without underlying error", func(t *testing.T) {
		err := &StrategyError{Message: "no strategy available"}

		expected := "strategy error: no strategy available"
		if err.Error() != expected {
			t.Errorf("Error() = %s, want %s", err.Error(), expected)
		}
	})
}

func TestStrategyError_Unwrap(t *testing.T) {
	underlying := errors.New("root cause")
	err := &StrategyError{Message: "test", Err: underlying}

	unwrapped := err.Unwrap()
	if unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}
}

func TestStrategyError_Is(t *testing.T) {
	underlying := errors.New("connection refused")
	err := &StrategyError{Message: "test", Err: underlying}

	if !errors.Is(err, underlying) {
		t.Error("errors.Is() should match underlying error")
	}
}

func TestStrategyError_VaultSandboxError(t *testing.T) {
	err := &StrategyError{}
	err.VaultSandboxError()

	var _ VaultSandboxError = err
}

func TestVaultSandboxError_Interface(t *testing.T) {
	// Verify all error types implement VaultSandboxError interface
	var _ VaultSandboxError = &APIError{}
	var _ VaultSandboxError = &NetworkError{}
	var _ VaultSandboxError = &TimeoutError{}
	var _ VaultSandboxError = &DecryptionError{}
	var _ VaultSandboxError = &SignatureVerificationError{}
	var _ VaultSandboxError = &SSEError{}
	var _ VaultSandboxError = &ValidationError{}
	var _ VaultSandboxError = &StrategyError{}
}

func TestErrorWrapping(t *testing.T) {
	root := errors.New("root cause")
	wrapped := fmt.Errorf("wrapped: %w", root)
	netErr := &NetworkError{Err: wrapped}

	if !errors.Is(netErr, root) {
		t.Error("errors.Is() should match through wrapped chain")
	}
}
