package delivery

import (
	"testing"
)

func TestInboxInfo(t *testing.T) {
	info := InboxInfo{
		Hash:         "abc123",
		EmailAddress: "test@example.com",
	}

	if info.Hash != "abc123" {
		t.Errorf("Hash = %s, want abc123", info.Hash)
	}
	if info.EmailAddress != "test@example.com" {
		t.Errorf("EmailAddress = %s, want test@example.com", info.EmailAddress)
	}
}

func TestConfig(t *testing.T) {
	cfg := Config{
		APIClient: nil,
	}

	// Just verify the struct works
	if cfg.APIClient != nil {
		t.Error("APIClient should be nil")
	}
}

// Test that strategies implement the Strategy interface
func TestStrategyInterface(t *testing.T) {
	// Verify PollingStrategy implements Strategy
	var _ Strategy = (*PollingStrategy)(nil)

	// Verify SSEStrategy implements Strategy
	var _ Strategy = (*SSEStrategy)(nil)

	// Verify AutoStrategy implements Strategy
	var _ Strategy = (*AutoStrategy)(nil)
}
