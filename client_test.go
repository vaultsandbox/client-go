package vaultsandbox

import (
	"errors"
	"testing"
)

func TestNew_RequiresAPIKey(t *testing.T) {
	_, err := New("")
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Errorf("New() error = %v, want ErrMissingAPIKey", err)
	}
}

func TestServerInfo_Fields(t *testing.T) {
	info := &ServerInfo{
		AllowedDomains: []string{"example.com", "test.com"},
		MaxTTL:         MaxTTL,
		DefaultTTL:     MinTTL,
	}

	if len(info.AllowedDomains) != 2 {
		t.Errorf("AllowedDomains length = %d, want 2", len(info.AllowedDomains))
	}
	if info.MaxTTL != MaxTTL {
		t.Errorf("MaxTTL = %v, want %v", info.MaxTTL, MaxTTL)
	}
	if info.DefaultTTL != MinTTL {
		t.Errorf("DefaultTTL = %v, want %v", info.DefaultTTL, MinTTL)
	}
}

// Note: Full client tests require a real API connection
// These tests verify the configuration and error handling
// Integration tests are in the integration/ directory
