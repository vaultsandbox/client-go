package vaultsandbox

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestExportInboxToFile_NilInbox(t *testing.T) {
	// Create a minimal client (we can't fully initialize without API)
	c := &Client{}

	err := c.ExportInboxToFile(nil, "/tmp/test.json")
	if err == nil {
		t.Error("ExportInboxToFile(nil, ...) should return error")
	}
	if err.Error() != "inbox is nil" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExportedInbox_JSONRoundtrip(t *testing.T) {
	original := &ExportedInbox{
		EmailAddress: "test@example.com",
		ExpiresAt:    time.Now().Add(time.Hour),
		InboxHash:    "hash123",
		ServerSigPk:  "serverkey",
		PublicKeyB64: "publickey",
		SecretKeyB64: "secretkey",
		ExportedAt:   time.Now(),
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Unmarshal back
	var parsed ExportedInbox
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify fields
	if parsed.EmailAddress != original.EmailAddress {
		t.Errorf("EmailAddress = %q, want %q", parsed.EmailAddress, original.EmailAddress)
	}
	if parsed.InboxHash != original.InboxHash {
		t.Errorf("InboxHash = %q, want %q", parsed.InboxHash, original.InboxHash)
	}
	if parsed.ServerSigPk != original.ServerSigPk {
		t.Errorf("ServerSigPk = %q, want %q", parsed.ServerSigPk, original.ServerSigPk)
	}
	if parsed.PublicKeyB64 != original.PublicKeyB64 {
		t.Errorf("PublicKeyB64 = %q, want %q", parsed.PublicKeyB64, original.PublicKeyB64)
	}
	if parsed.SecretKeyB64 != original.SecretKeyB64 {
		t.Errorf("SecretKeyB64 = %q, want %q", parsed.SecretKeyB64, original.SecretKeyB64)
	}
}

func TestImportInboxFromFile_NotFound(t *testing.T) {
	c := &Client{}

	_, err := c.ImportInboxFromFile(nil, "/nonexistent/path/file.json")
	if err == nil {
		t.Error("ImportInboxFromFile should return error for nonexistent file")
	}
}

func TestImportInboxFromFile_InvalidJSON(t *testing.T) {
	c := &Client{}

	// Create a temp file with invalid JSON
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(tmpFile, []byte("not valid json"), 0600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	_, err := c.ImportInboxFromFile(nil, tmpFile)
	if err == nil {
		t.Error("ImportInboxFromFile should return error for invalid JSON")
	}
}

// Note: Full client tests require a real API connection
// These tests verify the configuration and error handling
// Integration tests are in the integration/ directory
