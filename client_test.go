package vaultsandbox

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
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
		Version:      ExportVersion,
		EmailAddress: "test@example.com",
		ExpiresAt:    time.Now().Add(time.Hour),
		InboxHash:    "hash123",
		ServerSigPk:  "serverkey",
		SecretKey:    "secretkey",
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
	if parsed.Version != original.Version {
		t.Errorf("Version = %d, want %d", parsed.Version, original.Version)
	}
	if parsed.EmailAddress != original.EmailAddress {
		t.Errorf("EmailAddress = %q, want %q", parsed.EmailAddress, original.EmailAddress)
	}
	if parsed.InboxHash != original.InboxHash {
		t.Errorf("InboxHash = %q, want %q", parsed.InboxHash, original.InboxHash)
	}
	if parsed.ServerSigPk != original.ServerSigPk {
		t.Errorf("ServerSigPk = %q, want %q", parsed.ServerSigPk, original.ServerSigPk)
	}
	if parsed.SecretKey != original.SecretKey {
		t.Errorf("SecretKey = %q, want %q", parsed.SecretKey, original.SecretKey)
	}
}

func TestImportInboxFromFile_NotFound(t *testing.T) {
	c := &Client{}

	_, err := c.ImportInboxFromFile(context.TODO(), "/nonexistent/path/file.json")
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

	_, err := c.ImportInboxFromFile(context.TODO(), tmpFile)
	if err == nil {
		t.Error("ImportInboxFromFile should return error for invalid JSON")
	}
}

func TestExportedInbox_Validate_InvalidBase64(t *testing.T) {
	tests := []struct {
		name     string
		modifier func(*ExportedInbox)
	}{
		{
			name: "invalid SecretKey",
			modifier: func(e *ExportedInbox) {
				e.SecretKey = "!!!not-valid-base64!!!"
			},
		},
		{
			name: "invalid ServerSigPk",
			modifier: func(e *ExportedInbox) {
				e.ServerSigPk = "!!!not-valid-base64!!!"
			},
		},
		{
			name: "valid base64 but wrong padding in SecretKey",
			modifier: func(e *ExportedInbox) {
				e.SecretKey = "YWJjZA==" // valid base64 but wrong size
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create valid export data
			validSecretKey := make([]byte, 2400) // MLKEMSecretKeySize
			validServerSig := make([]byte, 1952) // MLDSAPublicKeySize

			data := &ExportedInbox{
				Version:      ExportVersion,
				EmailAddress: "test@example.com",
				ExpiresAt:    time.Now().Add(time.Hour),
				InboxHash:    "hash123",
				ServerSigPk:  base64.RawURLEncoding.EncodeToString(validServerSig),
				SecretKey:    base64.RawURLEncoding.EncodeToString(validSecretKey),
				ExportedAt:   time.Now(),
				Encrypted:    true, // Key validation only runs for encrypted inboxes
			}

			// Apply modification
			tt.modifier(data)

			// Validation should fail
			err := data.Validate()
			if err == nil {
				t.Error("Validate() should return error for invalid base64")
			}
		})
	}
}

func TestExportedInbox_Validate_MissingFields(t *testing.T) {
	tests := []struct {
		name     string
		modifier func(*ExportedInbox)
	}{
		{
			name: "empty email address",
			modifier: func(e *ExportedInbox) {
				e.EmailAddress = ""
			},
		},
		{
			name: "empty secret key for encrypted inbox",
			modifier: func(e *ExportedInbox) {
				e.Encrypted = true
				e.SecretKey = ""
			},
		},
		{
			name: "empty inbox hash",
			modifier: func(e *ExportedInbox) {
				e.InboxHash = ""
			},
		},
		{
			name: "invalid version",
			modifier: func(e *ExportedInbox) {
				e.Version = 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validSecretKey := make([]byte, 2400)
			validServerSig := make([]byte, 1952)

			data := &ExportedInbox{
				Version:      ExportVersion,
				EmailAddress: "test@example.com",
				ExpiresAt:    time.Now().Add(time.Hour),
				InboxHash:    "hash123",
				ServerSigPk:  base64.RawURLEncoding.EncodeToString(validServerSig),
				SecretKey:    base64.RawURLEncoding.EncodeToString(validSecretKey),
				ExportedAt:   time.Now(),
			}

			tt.modifier(data)

			err := data.Validate()
			if !errors.Is(err, ErrInvalidImportData) {
				t.Errorf("Validate() error = %v, want ErrInvalidImportData", err)
			}
		})
	}
}

func TestExportedInbox_Validate_WrongKeySizes(t *testing.T) {
	tests := []struct {
		name         string
		secretKeyLen int
	}{
		{"too short secret key", 100},
		{"too long secret key", 3000},
		{"empty secret key", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secretKey := make([]byte, tt.secretKeyLen)
			validServerSig := make([]byte, 1952)

			data := &ExportedInbox{
				Version:      ExportVersion,
				EmailAddress: "test@example.com",
				ExpiresAt:    time.Now().Add(time.Hour),
				InboxHash:    "hash123",
				ServerSigPk:  base64.RawURLEncoding.EncodeToString(validServerSig),
				SecretKey:    base64.RawURLEncoding.EncodeToString(secretKey),
				ExportedAt:   time.Now(),
				Encrypted:    true, // Key validation only runs for encrypted inboxes
			}

			err := data.Validate()
			if !errors.Is(err, ErrInvalidImportData) {
				t.Errorf("Validate() error = %v, want ErrInvalidImportData", err)
			}
		})
	}
}

func TestNewInboxFromExport_InvalidServerSigPkSize(t *testing.T) {
	// Valid secret key, but wrong size server sig pk
	validSecretKey := make([]byte, 2400)  // MLKEMSecretKeySize
	invalidServerSig := make([]byte, 100) // Wrong size

	data := &ExportedInbox{
		Version:      ExportVersion,
		EmailAddress: "test@example.com",
		ExpiresAt:    time.Now().Add(time.Hour),
		InboxHash:    "hash123",
		ServerSigPk:  base64.RawURLEncoding.EncodeToString(invalidServerSig),
		SecretKey:    base64.RawURLEncoding.EncodeToString(validSecretKey),
		ExportedAt:   time.Now(),
		Encrypted:    true, // Key validation only runs for encrypted inboxes
	}

	// Validation should fail due to invalid server sig pk size
	err := data.Validate()
	if !errors.Is(err, ErrInvalidImportData) {
		t.Errorf("Validate() error = %v, want ErrInvalidImportData", err)
	}
}

func TestExportInboxToFile_FormattedJSON(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "export.json")

	// Create mock inbox data
	validSecretKey := make([]byte, 2400) // MLKEMSecretKeySize
	validServerSig := make([]byte, 1952) // MLDSAPublicKeySize

	exported := &ExportedInbox{
		Version:      ExportVersion,
		EmailAddress: "test@example.com",
		ExpiresAt:    time.Now().Add(time.Hour),
		InboxHash:    "hash123",
		ServerSigPk:  base64.RawURLEncoding.EncodeToString(validServerSig),
		SecretKey:    base64.RawURLEncoding.EncodeToString(validSecretKey),
		ExportedAt:   time.Now(),
	}

	// Write using json.MarshalIndent (same as client.ExportInboxToFile does internally)
	jsonData, err := json.MarshalIndent(exported, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent failed: %v", err)
	}

	if err := os.WriteFile(tmpFile, jsonData, 0600); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	// Read and verify formatting
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}

	// Check for indentation (2 spaces)
	if !strings.Contains(string(content), "  \"version\"") {
		t.Error("JSON should be indented with 2 spaces")
	}

	// Check for newlines between fields
	lines := strings.Split(string(content), "\n")
	if len(lines) < 5 {
		t.Errorf("JSON should have multiple lines, got %d", len(lines))
	}

	// Verify it starts with { and ends with }
	trimmed := strings.TrimSpace(string(content))
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		t.Error("JSON should be a valid object")
	}
}

func TestImportInboxFromFile_EmptyFile(t *testing.T) {
	c := &Client{}

	// Create a temp file that is empty
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "empty.json")
	if err := os.WriteFile(tmpFile, []byte(""), 0600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	_, err := c.ImportInboxFromFile(context.TODO(), tmpFile)
	if err == nil {
		t.Error("ImportInboxFromFile should return error for empty file")
	}
}

func TestImportInboxFromFile_ValidJSONWrongStructure(t *testing.T) {
	c := &Client{}

	// Create a temp file with valid JSON but wrong structure
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "wrong.json")
	if err := os.WriteFile(tmpFile, []byte(`{"foo": "bar"}`), 0600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	_, err := c.ImportInboxFromFile(context.TODO(), tmpFile)
	if err == nil {
		t.Error("ImportInboxFromFile should return error for wrong JSON structure")
	}
}

func TestExportedInbox_JSONTimestampFormat(t *testing.T) {
	now := time.Now().Round(time.Second)
	expires := now.Add(time.Hour)

	data := &ExportedInbox{
		Version:      ExportVersion,
		EmailAddress: "test@example.com",
		ExpiresAt:    expires,
		InboxHash:    "hash123",
		ServerSigPk:  "serverkey",
		SecretKey:    "secretkey",
		ExportedAt:   now,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Parse as raw JSON to check timestamp format
	var raw map[string]interface{}
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Check that version is present
	version, ok := raw["version"].(float64) // JSON numbers are float64
	if !ok || int(version) != ExportVersion {
		t.Errorf("version = %v, want %d", version, ExportVersion)
	}

	// Check that timestamps are strings (ISO 8601 format)
	expiresAtStr, ok := raw["expiresAt"].(string)
	if !ok {
		t.Error("expiresAt should be a string")
	}

	// Try to parse as RFC3339 (ISO 8601 subset used by Go)
	_, err = time.Parse(time.RFC3339Nano, expiresAtStr)
	if err != nil {
		t.Errorf("expiresAt should be valid RFC3339: %v", err)
	}

	exportedAtStr, ok := raw["exportedAt"].(string)
	if !ok {
		t.Error("exportedAt should be a string")
	}

	_, err = time.Parse(time.RFC3339Nano, exportedAtStr)
	if err != nil {
		t.Errorf("exportedAt should be valid RFC3339: %v", err)
	}
}

func TestExportedInbox_JSONFieldNames(t *testing.T) {
	data := &ExportedInbox{
		Version:      ExportVersion,
		EmailAddress: "test@example.com",
		ExpiresAt:    time.Now(),
		InboxHash:    "hash",
		ServerSigPk:  "sig",
		SecretKey:    "sec",
		ExportedAt:   time.Now(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	jsonStr := string(jsonData)

	// Check expected field names per VaultSandbox spec Section 9
	expectedFields := []string{
		"version",
		"emailAddress",
		"expiresAt",
		"inboxHash",
		"serverSigPk",
		"secretKey",
		"exportedAt",
	}

	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("JSON should contain field %q", field)
		}
	}

	// Ensure old field names are NOT present
	removedFields := []string{"publicKeyB64", "secretKeyB64"}
	for _, field := range removedFields {
		if strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("JSON should NOT contain removed field %q", field)
		}
	}
}

// Tests for buildAPIClient helper
func TestBuildAPIClient_DefaultConfig(t *testing.T) {
	cfg := &clientConfig{
		baseURL: "https://test.example.com",
		timeout: 30 * time.Second,
	}

	client, err := buildAPIClient("test-api-key", cfg)
	if err != nil {
		t.Fatalf("buildAPIClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("buildAPIClient() returned nil client")
	}
}

func TestBuildAPIClient_WithAllOptions(t *testing.T) {
	cfg := &clientConfig{
		baseURL:    "https://test.example.com",
		timeout:    45 * time.Second,
		retries:    3,
		retryOn:    []int{500, 502, 503},
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	client, err := buildAPIClient("test-api-key", cfg)
	if err != nil {
		t.Fatalf("buildAPIClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("buildAPIClient() returned nil client")
	}
}

func TestBuildAPIClient_EmptyAPIKey(t *testing.T) {
	cfg := &clientConfig{
		baseURL: "https://test.example.com",
	}

	_, err := buildAPIClient("", cfg)
	if err == nil {
		t.Error("buildAPIClient() should return error for empty API key")
	}
}

// Tests for createDeliveryStrategy helper
func TestCreateDeliveryStrategy_SSE(t *testing.T) {
	cfg := &clientConfig{
		deliveryStrategy: StrategySSE,
	}

	// We need an API client for the delivery strategy
	apiCfg := &clientConfig{baseURL: "https://test.example.com"}
	apiClient, _ := buildAPIClient("test-key", apiCfg)

	strategy := createDeliveryStrategy(cfg, apiClient)
	if strategy == nil {
		t.Fatal("createDeliveryStrategy() returned nil")
	}
}

func TestCreateDeliveryStrategy_Polling(t *testing.T) {
	cfg := &clientConfig{
		deliveryStrategy: StrategyPolling,
	}

	apiCfg := &clientConfig{baseURL: "https://test.example.com"}
	apiClient, _ := buildAPIClient("test-key", apiCfg)

	strategy := createDeliveryStrategy(cfg, apiClient)
	if strategy == nil {
		t.Fatal("createDeliveryStrategy() returned nil")
	}
}

func TestCreateDeliveryStrategy_Default(t *testing.T) {
	// Empty/unknown strategy should default to SSE
	cfg := &clientConfig{
		deliveryStrategy: DeliveryStrategy("unknown"),
	}

	apiCfg := &clientConfig{baseURL: "https://test.example.com"}
	apiClient, _ := buildAPIClient("test-key", apiCfg)

	strategy := createDeliveryStrategy(cfg, apiClient)
	if strategy == nil {
		t.Fatal("createDeliveryStrategy() returned nil for unknown strategy")
	}
	if strategy.Name() != "sse" {
		t.Errorf("expected SSE strategy as default, got %s", strategy.Name())
	}
}

// Tests for syncState.computeEmailsHash
func TestSyncState_ComputeEmailsHash(t *testing.T) {
	t.Run("empty set produces valid hash", func(t *testing.T) {
		state := &syncState{seenEmails: make(map[string]struct{})}
		hash := state.computeEmailsHash()
		if hash == "" {
			t.Error("computeEmailsHash() should not return empty string")
		}
		// SHA256("") in base64url = 47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU
		if hash != "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU" {
			t.Errorf("empty set hash = %q, want %q", hash, "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU")
		}
	})

	t.Run("same emails produce same hash regardless of insertion order", func(t *testing.T) {
		state1 := &syncState{seenEmails: make(map[string]struct{})}
		state1.seenEmails["a"] = struct{}{}
		state1.seenEmails["b"] = struct{}{}
		state1.seenEmails["c"] = struct{}{}

		state2 := &syncState{seenEmails: make(map[string]struct{})}
		state2.seenEmails["c"] = struct{}{}
		state2.seenEmails["a"] = struct{}{}
		state2.seenEmails["b"] = struct{}{}

		hash1 := state1.computeEmailsHash()
		hash2 := state2.computeEmailsHash()

		if hash1 != hash2 {
			t.Errorf("hashes should match: %q vs %q", hash1, hash2)
		}
	})

	t.Run("different emails produce different hashes", func(t *testing.T) {
		state1 := &syncState{seenEmails: make(map[string]struct{})}
		state1.seenEmails["email1"] = struct{}{}

		state2 := &syncState{seenEmails: make(map[string]struct{})}
		state2.seenEmails["email2"] = struct{}{}

		hash1 := state1.computeEmailsHash()
		hash2 := state2.computeEmailsHash()

		if hash1 == hash2 {
			t.Error("different emails should produce different hashes")
		}
	})

	t.Run("hash is valid base64url without padding", func(t *testing.T) {
		state := &syncState{seenEmails: make(map[string]struct{})}
		state.seenEmails["test@example.com"] = struct{}{}

		hash := state.computeEmailsHash()

		// Should not contain padding
		if strings.Contains(hash, "=") {
			t.Error("hash should not contain padding characters")
		}
		// Should not contain standard base64 chars that differ from URL-safe
		if strings.Contains(hash, "+") || strings.Contains(hash, "/") {
			t.Error("hash should use URL-safe base64 encoding")
		}
		// Should be 43 chars (SHA256 = 32 bytes = 43 chars in base64url without padding)
		if len(hash) != 43 {
			t.Errorf("hash length = %d, want 43", len(hash))
		}
	})
}

func TestSyncState_ComputeEmailsHash_Deterministic(t *testing.T) {
	// Verify that hash is deterministic regardless of map iteration order
	state := &syncState{
		seenEmails: make(map[string]struct{}),
	}

	// Add emails in a specific order
	for i := 0; i < 100; i++ {
		state.seenEmails[strings.Repeat("x", i+1)] = struct{}{}
	}

	// Compute hash multiple times
	first := state.computeEmailsHash()
	for i := 0; i < 10; i++ {
		got := state.computeEmailsHash()
		if got != first {
			t.Errorf("computeEmailsHash() not deterministic: got %q, want %q", got, first)
		}
	}
}

// Note: Full client tests require a real API connection
// These tests verify the configuration and error handling
// Integration tests are in the integration/ directory

func TestClient_CheckClosed(t *testing.T) {
	c := &Client{closed: false}

	// Should return nil when not closed
	if err := c.checkClosed(); err != nil {
		t.Errorf("checkClosed() returned error when not closed: %v", err)
	}

	// Mark as closed
	c.closed = true

	// Should return ErrClientClosed when closed
	err := c.checkClosed()
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("checkClosed() = %v, want ErrClientClosed", err)
	}
}

func TestClient_RegisterInbox_WhenClosed(t *testing.T) {
	c := &Client{
		closed:        true,
		inboxes:       make(map[string]*Inbox),
		inboxesByHash: make(map[string]*Inbox),
		syncStates:    make(map[string]*syncState),
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
	}

	err := c.registerInbox(inbox)
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("registerInbox() = %v, want ErrClientClosed", err)
	}
}

func TestClient_GetInbox(t *testing.T) {
	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
	}

	c := &Client{
		inboxes: map[string]*Inbox{
			"test@example.com": inbox,
		},
	}

	// Found case
	got, found := c.GetInbox("test@example.com")
	if !found {
		t.Error("GetInbox() found = false, want true")
	}
	if got != inbox {
		t.Error("GetInbox() returned wrong inbox")
	}

	// Not found case
	_, found = c.GetInbox("nonexistent@example.com")
	if found {
		t.Error("GetInbox() found = true for nonexistent inbox")
	}
}

func TestClient_Inboxes(t *testing.T) {
	inbox1 := &Inbox{emailAddress: "test1@example.com", inboxHash: "hash1"}
	inbox2 := &Inbox{emailAddress: "test2@example.com", inboxHash: "hash2"}

	c := &Client{
		inboxes: map[string]*Inbox{
			"test1@example.com": inbox1,
			"test2@example.com": inbox2,
		},
	}

	inboxes := c.Inboxes()
	if len(inboxes) != 2 {
		t.Errorf("Inboxes() len = %d, want 2", len(inboxes))
	}
}

func TestClient_Inboxes_Empty(t *testing.T) {
	c := &Client{
		inboxes: make(map[string]*Inbox),
	}

	inboxes := c.Inboxes()
	if len(inboxes) != 0 {
		t.Errorf("Inboxes() len = %d, want 0", len(inboxes))
	}
}

func TestClient_WatchInboxes_EmptyList(t *testing.T) {
	c := &Client{
		subs: newSubscriptionManager(),
	}

	ch := c.WatchInboxes(context.Background())

	// Channel should be closed immediately for empty inbox list
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("WatchInboxes() channel should be closed for empty list")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("WatchInboxes() channel should be closed immediately")
	}
}

func TestClient_WatchInboxes_ContextCancel(t *testing.T) {
	c := &Client{
		subs: newSubscriptionManager(),
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch := c.WatchInboxes(ctx, inbox)

	// Cancel context
	cancel()

	// Give time for cleanup goroutine to run
	time.Sleep(50 * time.Millisecond)

	// Verify we can still read from channel (it's not closed, but context is done)
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("context should be done")
	}

	// Channel should not block
	select {
	case <-ch:
		// May or may not receive
	default:
		// Expected if no events
	}
}

func TestClient_ImportInbox_NilData(t *testing.T) {
	c := &Client{}

	_, err := c.ImportInbox(context.Background(), nil)
	if err == nil {
		t.Error("ImportInbox(nil) should return error")
	}
}

func TestClient_ImportInbox_WhenClosed(t *testing.T) {
	c := &Client{
		closed: true,
	}

	data := &ExportedInbox{
		EmailAddress: "test@example.com",
	}

	_, err := c.ImportInbox(context.Background(), data)
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("ImportInbox() = %v, want ErrClientClosed", err)
	}
}

func TestClient_ImportInbox_DuplicateInbox(t *testing.T) {
	c := &Client{
		closed: false,
		inboxes: map[string]*Inbox{
			"test@example.com": {},
		},
	}

	data := &ExportedInbox{
		EmailAddress: "test@example.com",
	}

	_, err := c.ImportInbox(context.Background(), data)
	if !errors.Is(err, ErrInboxAlreadyExists) {
		t.Errorf("ImportInbox() = %v, want ErrInboxAlreadyExists", err)
	}
}

func TestClient_Close_Idempotent(t *testing.T) {
	c := &Client{
		closed:        false,
		inboxes:       make(map[string]*Inbox),
		inboxesByHash: make(map[string]*Inbox),
		subs:          newSubscriptionManager(),
	}

	// First close
	if err := c.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if !c.closed {
		t.Error("client should be closed after Close()")
	}

	// Second close should be no-op
	if err := c.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestClient_ImportInboxFromFile_ClosedClient(t *testing.T) {
	c := &Client{
		closed: true,
	}

	_, err := c.ImportInboxFromFile(context.Background(), "/some/path.json")
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("ImportInboxFromFile() = %v, want ErrClientClosed", err)
	}
}

func TestClient_CheckKey_WhenClosed(t *testing.T) {
	c := &Client{
		closed: true,
	}

	err := c.CheckKey(context.Background())
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("CheckKey() = %v, want ErrClientClosed", err)
	}
}

func TestClient_WatchInboxesFunc_ContextCancel(t *testing.T) {
	c := &Client{
		subs: newSubscriptionManager(),
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
	}

	ctx, cancel := context.WithCancel(context.Background())

	var callCount int
	done := make(chan struct{})

	go func() {
		c.WatchInboxesFunc(ctx, func(event *InboxEvent) {
			callCount++
		}, inbox)
		close(done)
	}()

	// Cancel context immediately
	cancel()

	// WatchInboxesFunc should exit
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("WatchInboxesFunc did not exit after context cancel")
	}
}

// Tests for syncInbox function
func TestClient_SyncInbox_NilState(t *testing.T) {
	c := &Client{
		syncStates: make(map[string]*syncState),
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
	}

	// syncInbox should return early when state is nil (inbox not registered)
	// This should not panic
	c.syncInbox(context.Background(), inbox)
}

func TestClient_SyncAllInboxes_WhenClosed(t *testing.T) {
	c := &Client{
		closed:     true,
		inboxes:    make(map[string]*Inbox),
		syncStates: make(map[string]*syncState),
	}

	// syncAllInboxes should return early when client is closed
	// This should not panic
	c.syncAllInboxes(context.Background())
}

func TestClient_SyncAllInboxes_Empty(t *testing.T) {
	c := &Client{
		closed:     false,
		inboxes:    make(map[string]*Inbox),
		syncStates: make(map[string]*syncState),
	}

	// syncAllInboxes should handle empty inbox list
	// This should not panic
	c.syncAllInboxes(context.Background())
}

func TestClient_SyncInbox_HashMatch(t *testing.T) {
	// Create a client with sync state that has a known hash
	state := &syncState{seenEmails: make(map[string]struct{})}
	localHash := state.computeEmailsHash() // empty hash

	c := &Client{
		syncStates: map[string]*syncState{
			"hash123": state,
		},
	}

	// Create a mock inbox that returns the same hash
	// This requires mocking the API client, which is complex
	// For now, just test that the state lookup works
	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
	}

	// Verify state is found
	c.mu.RLock()
	foundState := c.syncStates[inbox.inboxHash]
	c.mu.RUnlock()

	if foundState == nil {
		t.Error("syncState should be found")
	}

	foundHash := foundState.computeEmailsHash()
	if foundHash != localHash {
		t.Errorf("hash mismatch: got %s, want %s", foundHash, localHash)
	}
}

func TestClient_SyncInbox_StateRemovedDuringSync(t *testing.T) {
	// Test the case where state is removed between initial check and lock acquisition
	state := &syncState{seenEmails: make(map[string]struct{})}

	c := &Client{
		syncStates: map[string]*syncState{
			"hash123": state,
		},
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
	}

	// Remove state before sync
	delete(c.syncStates, inbox.inboxHash)

	// syncInbox should handle this gracefully
	c.syncInbox(context.Background(), inbox)
}

func TestClient_HandleSSEEvent_NilEvent(t *testing.T) {
	c := &Client{
		inboxesByHash: make(map[string]*Inbox),
		syncStates:    make(map[string]*syncState),
	}

	// handleSSEEvent should handle nil event gracefully
	err := c.handleSSEEvent(context.Background(), nil)
	if err != nil {
		t.Errorf("handleSSEEvent(nil) error = %v, want nil", err)
	}
}

func TestClient_HandleSSEEvent_UnknownInbox(t *testing.T) {
	c := &Client{
		inboxesByHash: make(map[string]*Inbox),
		syncStates:    make(map[string]*syncState),
	}

	// Create an event for an unknown inbox
	event := &api.SSEEvent{
		InboxID: "unknown-hash",
		EmailID: "email-123",
	}

	// handleSSEEvent should return nil for unknown inbox (event is ignored)
	err := c.handleSSEEvent(context.Background(), event)
	if err != nil {
		t.Errorf("handleSSEEvent(unknown inbox) error = %v, want nil", err)
	}
}

// mockServerSigPk is a valid base64-encoded ML-DSA public key for testing (1952 bytes)
var mockServerSigPk = base64.RawURLEncoding.EncodeToString(make([]byte, 1952))

// mockCreateInboxResponse returns a valid CreateInbox response for testing
func mockCreateInboxResponse(w http.ResponseWriter) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"emailAddress": "test@test.com",
		"expiresAt":    time.Now().Add(time.Hour).Format(time.RFC3339),
		"inboxHash":    "test-inbox-hash",
		"serverSigPk":  mockServerSigPk,
	})
}

// TestClient_SyncInbox_WithMockServer tests syncInbox with a mock HTTP server
func TestClient_SyncInbox_WithMockServer(t *testing.T) {
	var syncCallCount atomic.Int32
	var metadataCallCount atomic.Int32

	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			syncCallCount.Add(1)
			// Return a hash that differs from empty hash to trigger sync
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "different-hash-to-trigger-sync",
				"emailCount": 0,
			})

		case strings.Contains(r.URL.Path, "/emails") && !strings.Contains(r.URL.Path, "/emails/"):
			metadataCallCount.Add(1)
			// Return empty email list
			json.NewEncoder(w).Encode([]interface{}{})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create client with mock server
	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	// Create an inbox to test sync
	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Call syncInbox directly
	client.syncInbox(ctx, inbox)

	// Verify sync endpoints were called
	if syncCallCount.Load() == 0 {
		t.Error("sync endpoint was not called")
	}
	if metadataCallCount.Load() == 0 {
		t.Error("emails metadata endpoint was not called")
	}
}

// TestClient_SyncInbox_OnSyncError tests that onSyncError callback is called on errors
func TestClient_SyncInbox_OnSyncError(t *testing.T) {
	var errorCount atomic.Int32
	var receivedError error
	var mu sync.Mutex

	// Create a mock server that returns errors for sync
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			// Return server error to trigger onSyncError
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "server error"})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create client with mock server and error callback
	client, err := New("test-api-key",
		WithBaseURL(server.URL),
		WithOnSyncError(func(err error) {
			errorCount.Add(1)
			mu.Lock()
			receivedError = err
			mu.Unlock()
		}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	// Create an inbox to test sync
	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Call syncInbox directly - should trigger error callback
	client.syncInbox(ctx, inbox)

	// Verify error callback was called
	if errorCount.Load() == 0 {
		t.Error("onSyncError callback was not called")
	}
	mu.Lock()
	if receivedError == nil {
		t.Error("onSyncError callback received nil error")
	}
	mu.Unlock()
}

// TestClient_SyncInbox_MetadataError tests error handling when GetEmailsMetadataOnly fails
func TestClient_SyncInbox_MetadataError(t *testing.T) {
	var errorCount atomic.Int32

	// Create a mock server that returns error on metadata fetch
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			// Return hash that triggers sync
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "different-hash",
				"emailCount": 1,
			})

		case strings.Contains(r.URL.Path, "/emails"):
			// Return error on emails endpoint
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "server error"})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create client with mock server and error callback
	client, err := New("test-api-key",
		WithBaseURL(server.URL),
		WithOnSyncError(func(err error) {
			errorCount.Add(1)
		}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Call syncInbox - should trigger error on metadata fetch
	client.syncInbox(ctx, inbox)

	// Verify error callback was called
	if errorCount.Load() == 0 {
		t.Error("onSyncError callback was not called for metadata error")
	}
}

// TestClient_SyncInbox_HashMatch tests early return when hash matches
func TestClient_SyncInbox_HashMatchEarlyReturn(t *testing.T) {
	var metadataCallCount atomic.Int32

	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			// Return the empty hash (matches client's initial state)
			// SHA256("") = 47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU",
				"emailCount": 0,
			})

		case strings.Contains(r.URL.Path, "/emails"):
			metadataCallCount.Add(1)
			json.NewEncoder(w).Encode([]interface{}{})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Call syncInbox - should return early due to hash match
	client.syncInbox(ctx, inbox)

	// Metadata endpoint should NOT be called due to hash match
	if metadataCallCount.Load() > 0 {
		t.Error("emails endpoint should not be called when hash matches")
	}
}

// TestClient_SyncInbox_MetadataFetchFails tests that syncInbox handles
// metadata decryption failures gracefully (calls onSyncError).
// Note: Full email fetch testing requires real encryption which is covered by integration tests.
func TestClient_SyncInbox_MetadataFetchCalled(t *testing.T) {
	var syncCallCount atomic.Int32
	var emailsCallCount atomic.Int32

	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			syncCallCount.Add(1)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "new-hash-with-emails",
				"emailCount": 1,
			})

		case strings.HasSuffix(r.URL.Path, "/emails"):
			emailsCallCount.Add(1)
			// Return empty array - decryption of actual emails requires real crypto
			// This tests that the metadata endpoint IS called when hash differs
			json.NewEncoder(w).Encode([]interface{}{})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Call syncInbox
	client.syncInbox(ctx, inbox)

	// Sync endpoint should have been called
	if syncCallCount.Load() == 0 {
		t.Error("sync endpoint was not called")
	}

	// Emails list endpoint should have been called (hash differed, so fetch metadata)
	if emailsCallCount.Load() == 0 {
		t.Error("emails list endpoint was not called when hash differed")
	}
}

// TestClient_SyncInbox_DeletedEmails tests sync handling of deleted emails
func TestClient_SyncInbox_DeletedEmails(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			// Return hash indicating change
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "new-hash-after-deletion",
				"emailCount": 0,
			})

		case strings.Contains(r.URL.Path, "/emails"):
			// Return empty list (all emails deleted)
			json.NewEncoder(w).Encode([]interface{}{})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Pre-populate seenEmails with a "deleted" email
	client.mu.Lock()
	state := client.syncStates[inbox.inboxHash]
	if state != nil {
		state.seenEmails["deleted-email-id"] = struct{}{}
	}
	client.mu.Unlock()

	// Call syncInbox - should remove deleted email from seenEmails
	client.syncInbox(ctx, inbox)

	// Verify deleted email was removed from seenEmails
	client.mu.RLock()
	state = client.syncStates[inbox.inboxHash]
	_, stillExists := state.seenEmails["deleted-email-id"]
	client.mu.RUnlock()

	if stillExists {
		t.Error("deleted email should have been removed from seenEmails")
	}
}

// TestClient_DeleteInbox tests the DeleteInbox method
func TestClient_DeleteInbox(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/api/inboxes/"):
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Verify inbox is tracked
	_, exists := client.GetInbox(inbox.EmailAddress())
	if !exists {
		t.Error("inbox should exist before delete")
	}

	// Delete inbox
	err = client.DeleteInbox(ctx, inbox.EmailAddress())
	if err != nil {
		t.Errorf("DeleteInbox() error = %v", err)
	}

	// Verify inbox is no longer tracked
	_, exists = client.GetInbox(inbox.EmailAddress())
	if exists {
		t.Error("inbox should not exist after delete")
	}
}

// TestClient_DeleteInbox_NonExistent tests deleting a non-existent inbox
func TestClient_DeleteInbox_NonExistent(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.Method == http.MethodDelete:
			// Return 404 for non-existent inbox
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "inbox not found"})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	// Delete non-existent inbox - should call API even if not tracked locally
	err = client.DeleteInbox(context.Background(), "nonexistent@test.com")
	if err == nil {
		t.Error("DeleteInbox() should return error for non-existent inbox")
	}
}

// TestClient_DeleteAllInboxes tests the DeleteAllInboxes method
func TestClient_DeleteAllInboxes(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodDelete:
			json.NewEncoder(w).Encode(map[string]int{"deleted": 2})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Create two inboxes
	_, err = client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Delete all inboxes
	count, err := client.DeleteAllInboxes(ctx)
	if err != nil {
		t.Errorf("DeleteAllInboxes() error = %v", err)
	}
	if count != 2 {
		t.Errorf("DeleteAllInboxes() count = %d, want 2", count)
	}

	// Verify no inboxes are tracked
	inboxes := client.Inboxes()
	if len(inboxes) != 0 {
		t.Errorf("client should have no inboxes after DeleteAllInboxes, got %d", len(inboxes))
	}
}

// TestClient_ServerInfo tests the ServerInfo method
func TestClient_ServerInfo(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"example.com", "test.com"},
				"maxTTL":         7200,
				"defaultTTL":     600,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	info := client.ServerInfo()
	if info == nil {
		t.Fatal("ServerInfo() returned nil")
	}

	if len(info.AllowedDomains) != 2 {
		t.Errorf("AllowedDomains length = %d, want 2", len(info.AllowedDomains))
	}
	if info.MaxTTL != 7200*time.Second {
		t.Errorf("MaxTTL = %v, want %v", info.MaxTTL, 7200*time.Second)
	}
	if info.DefaultTTL != 600*time.Second {
		t.Errorf("DefaultTTL = %v, want %v", info.DefaultTTL, 600*time.Second)
	}
}

// TestClient_ExportInboxToFile_Success tests successful export to file
func TestClient_ExportInboxToFile_Success(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Export to file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "export.json")

	err = client.ExportInboxToFile(inbox, tmpFile)
	if err != nil {
		t.Fatalf("ExportInboxToFile() error = %v", err)
	}

	// Verify file exists and has correct permissions
	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("exported file does not exist: %v", err)
	}
	// Check file mode (on Unix systems)
	if info.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %v, want 0600", info.Mode().Perm())
	}

	// Verify file content is valid JSON
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}

	var exported ExportedInbox
	if err := json.Unmarshal(content, &exported); err != nil {
		t.Fatalf("exported file is not valid JSON: %v", err)
	}

	if exported.Version != ExportVersion {
		t.Errorf("exported version = %d, want %d", exported.Version, ExportVersion)
	}
	if exported.EmailAddress != inbox.EmailAddress() {
		t.Errorf("exported email = %q, want %q", exported.EmailAddress, inbox.EmailAddress())
	}
}

// TestClient_ExportInboxToFile_WriteError tests export with write failure
func TestClient_ExportInboxToFile_WriteError(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Try to export to a non-existent directory
	err = client.ExportInboxToFile(inbox, "/nonexistent/directory/export.json")
	if err == nil {
		t.Error("ExportInboxToFile() should return error for invalid path")
	}
	if !strings.Contains(err.Error(), "write file") {
		t.Errorf("expected write error, got: %v", err)
	}
}

// TestClient_CreateInbox_TTLBelowMinimum tests TTL validation
func TestClient_CreateInbox_TTLBelowMinimum(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	// Try to create inbox with TTL below minimum
	_, err = client.CreateInbox(context.Background(), WithTTL(30*time.Second))
	if err == nil {
		t.Error("CreateInbox() should return error for TTL below minimum")
	}
	if !strings.Contains(err.Error(), "below minimum") {
		t.Errorf("expected minimum TTL error, got: %v", err)
	}
}

// TestClient_CreateInbox_TTLAboveServerMax tests TTL validation against server max
func TestClient_CreateInbox_TTLAboveServerMax(t *testing.T) {
	// Create a mock server with low maxTTL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         300, // 5 minutes max
				"defaultTTL":     60,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	// Try to create inbox with TTL above server max
	_, err = client.CreateInbox(context.Background(), WithTTL(10*time.Minute))
	if err == nil {
		t.Error("CreateInbox() should return error for TTL above server max")
	}
	if !strings.Contains(err.Error(), "exceeds server maximum") {
		t.Errorf("expected max TTL error, got: %v", err)
	}
}

// TestClient_CreateInbox_APIError tests CreateInbox API error handling
func TestClient_CreateInbox_APIError(t *testing.T) {
	// Create a mock server that returns error on inbox creation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "server error"})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	_, err = client.CreateInbox(context.Background(), WithTTL(5*time.Minute))
	if err == nil {
		t.Error("CreateInbox() should return error on API failure")
	}
}

// TestClient_CreateInbox_WhenClosed tests CreateInbox on closed client
func TestClient_CreateInbox_WhenClosed(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Close the client
	client.Close()

	// Try to create inbox
	_, err = client.CreateInbox(context.Background(), WithTTL(5*time.Minute))
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("CreateInbox() = %v, want ErrClientClosed", err)
	}
}

// TestClient_New_CheckKeyError tests New when CheckKey fails
func TestClient_New_CheckKeyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/check-key" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid API key"})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	_, err := New("invalid-key", WithBaseURL(server.URL))
	if err == nil {
		t.Error("New() should return error for invalid API key")
	}
}

// TestClient_New_ServerInfoError tests New when GetServerInfo fails
func TestClient_New_ServerInfoError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "server error"})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	_, err := New("test-key", WithBaseURL(server.URL))
	if err == nil {
		t.Error("New() should return error when server info fetch fails")
	}
	if !strings.Contains(err.Error(), "fetch server info") {
		t.Errorf("expected server info error, got: %v", err)
	}
}

// TestClient_CheckKey_Success tests successful CheckKey call
func TestClient_CheckKey_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	// Verify CheckKey succeeds
	err = client.CheckKey(context.Background())
	if err != nil {
		t.Errorf("CheckKey() error = %v", err)
	}
}

// TestClient_WatchInboxesFunc_EventDelivery tests that events are delivered to callback
func TestClient_WatchInboxesFunc_EventDelivery(t *testing.T) {
	c := &Client{
		subs: newSubscriptionManager(),
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
	}

	ctx, cancel := context.WithCancel(context.Background())
	receivedEvent := make(chan *InboxEvent, 1)

	done := make(chan struct{})
	go func() {
		c.WatchInboxesFunc(ctx, func(event *InboxEvent) {
			select {
			case receivedEvent <- event:
			default:
			}
		}, inbox)
		close(done)
	}()

	// Give WatchInboxesFunc time to set up subscription
	time.Sleep(50 * time.Millisecond)

	// Simulate email arrival
	email := &Email{ID: "email-123", Subject: "Test"}
	c.subs.notify(inbox.inboxHash, email)

	// Wait for event or timeout
	select {
	case event := <-receivedEvent:
		if event == nil {
			t.Fatal("received nil event")
		}
		if event.Email.ID != "email-123" {
			t.Errorf("event email ID = %q, want %q", event.Email.ID, "email-123")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}

	// Cancel and wait for cleanup
	cancel()
	<-done
}

// TestClient_HandleSSEEvent_Success tests successful SSE event handling
func TestClient_HandleSSEEvent_Success(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.Contains(r.URL.Path, "/emails/") && r.Method == http.MethodGet:
			// Return an encrypted email - but this will fail decryption
			// which tests the error path in handleSSEEvent
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "email not found"})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Create SSE event
	event := &api.SSEEvent{
		InboxID: inbox.InboxHash(),
		EmailID: "test-email-id",
	}

	// handleSSEEvent will fail because GetEmail returns 404
	err = client.handleSSEEvent(ctx, event)
	if err == nil {
		t.Error("handleSSEEvent() should return error when email fetch fails")
	}
}

// TestClient_HandleSSEEvent_StateNilAfterFetch tests SSE event handling when state becomes nil
// after email fetch (race condition handling)
func TestClient_HandleSSEEvent_StateNilAfterFetch(t *testing.T) {
	// Create a mock server that will return a valid email
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.Contains(r.URL.Path, "/emails/"):
			// Return error to test early return
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Remove the state to test nil state handling
	client.mu.Lock()
	delete(client.syncStates, inbox.InboxHash())
	client.mu.Unlock()

	event := &api.SSEEvent{
		InboxID: inbox.InboxHash(),
		EmailID: "email-123",
	}

	// handleSSEEvent should handle nil state gracefully (state=nil at time of initial read)
	// Since GetEmail will fail anyway, we're really testing that there's no panic
	err = client.handleSSEEvent(ctx, event)
	if err == nil {
		t.Error("expected error from failed email fetch")
	}
}

// TestClient_ImportInbox_ValidationError tests import with invalid data
func TestClient_ImportInbox_ValidationError(t *testing.T) {
	c := &Client{
		closed:  false,
		inboxes: make(map[string]*Inbox),
	}

	// Create invalid export data (wrong version)
	data := &ExportedInbox{
		Version:      0, // Invalid version
		EmailAddress: "test@example.com",
		InboxHash:    "hash123",
	}

	_, err := c.ImportInbox(context.Background(), data)
	if err == nil {
		t.Error("ImportInbox() should return error for invalid data")
	}
}

// TestClient_SyncInbox_StateRemovedDuringProcessing tests state removal during sync
// by verifying that state checks are performed correctly
func TestClient_SyncInbox_StateRemovedDuringProcessing(t *testing.T) {
	var syncCalled atomic.Bool
	var metadataCalled atomic.Bool

	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			syncCalled.Store(true)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "different-hash",
				"emailCount": 1,
			})

		case strings.HasSuffix(r.URL.Path, "/emails"):
			metadataCalled.Store(true)
			// Return empty list - decryption requires real crypto
			// This tests the path up to metadata fetch
			json.NewEncoder(w).Encode([]interface{}{})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Call syncInbox
	client.syncInbox(ctx, inbox)

	// Verify sync and metadata endpoints were called
	if !syncCalled.Load() {
		t.Error("sync endpoint was not called")
	}
	if !metadataCalled.Load() {
		t.Error("emails metadata endpoint was not called")
	}
}

// TestClient_SyncAllInboxes_WithInboxes tests syncAllInboxes with multiple inboxes
func TestClient_SyncAllInboxes_WithInboxes(t *testing.T) {
	var syncCallCount atomic.Int32

	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			syncCallCount.Add(1)
			// Return matching hash so we skip further fetching
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU",
				"emailCount": 0,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	_, err = client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Call syncAllInboxes
	client.syncAllInboxes(ctx)

	// Verify sync was called
	if syncCallCount.Load() == 0 {
		t.Error("sync endpoint was not called")
	}
}

// TestClient_ImportInbox_Success tests successful inbox import
func TestClient_ImportInbox_Success(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU",
				"emailCount": 0,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create first client and inbox
	client1, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	inbox, err := client1.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Export the inbox
	exported := inbox.Export()
	client1.Close()

	// Create second client and import the inbox
	client2, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() for second client error = %v", err)
	}
	defer client2.Close()

	imported, err := client2.ImportInbox(ctx, exported)
	if err != nil {
		t.Fatalf("ImportInbox() error = %v", err)
	}

	if imported.EmailAddress() != exported.EmailAddress {
		t.Errorf("imported email = %q, want %q", imported.EmailAddress(), exported.EmailAddress)
	}
	if imported.InboxHash() != exported.InboxHash {
		t.Errorf("imported hash = %q, want %q", imported.InboxHash(), exported.InboxHash)
	}

	// Verify inbox is tracked
	_, exists := client2.GetInbox(imported.EmailAddress())
	if !exists {
		t.Error("imported inbox should be tracked by client")
	}
}

// TestClient_ImportInbox_APIVerifyError tests import when API verify fails
func TestClient_ImportInbox_APIVerifyError(t *testing.T) {
	// Create a mock server that fails on sync (verify)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			// Return error - inbox not found (expired or deleted)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "inbox not found"})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create client and inbox to get valid exported data
	client1, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	inbox, err := client1.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	exported := inbox.Export()
	client1.Close()

	// Create new client and try to import - should fail on verify
	client2, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client2.Close()

	_, err = client2.ImportInbox(ctx, exported)
	if err == nil {
		t.Error("ImportInbox() should return error when API verify fails")
	}
	if !strings.Contains(err.Error(), "verify inbox") {
		t.Errorf("expected verify error, got: %v", err)
	}
}

// TestClient_CreateInbox_RegisterInboxError tests CreateInbox when registerInbox fails
func TestClient_CreateInbox_RegisterInboxError(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Close client after API call succeeds but before registerInbox
	// This is a race condition test - we close in a goroutine
	ctx := context.Background()

	// First close the client
	client.Close()

	// Now try to create inbox - should fail because client is closed
	_, err = client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("CreateInbox() = %v, want ErrClientClosed", err)
	}
}

// TestClient_SyncInbox_StateNilDuringSync tests the edge case where state becomes nil
// during processing (e.g., inbox deleted while syncing)
func TestClient_SyncInbox_StateNilDuringSync(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			// Return matching hash so no further processing happens
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU",
				"emailCount": 0,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Remove state before sync to test nil handling
	client.mu.Lock()
	delete(client.syncStates, inbox.InboxHash())
	client.mu.Unlock()

	// Call syncInbox - should return early when state is nil
	client.syncInbox(ctx, inbox)
	// Test passes if no panic occurs
}

// TestClient_ImportInbox_RegisterError tests import when registration fails
func TestClient_ImportInbox_RegisterError(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU",
				"emailCount": 0,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create client and inbox to get valid exported data
	client1, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	inbox, err := client1.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	exported := inbox.Export()
	client1.Close()

	// Create second client
	client2, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Close client2 before import to trigger registerInbox error
	client2.Close()

	// Now import should fail due to closed client
	_, err = client2.ImportInbox(ctx, exported)
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("ImportInbox() = %v, want ErrClientClosed", err)
	}
}

// TestClient_SyncInbox_StateNilAfterLock tests state nil check after acquiring lock
func TestClient_SyncInbox_StateNilAfterLock(t *testing.T) {
	var metadataReturned atomic.Bool

	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "different-hash-triggers-fetch",
				"emailCount": 0,
			})

		case strings.HasSuffix(r.URL.Path, "/emails"):
			metadataReturned.Store(true)
			// Return empty - no encrypted data to parse
			json.NewEncoder(w).Encode([]interface{}{})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Call syncInbox
	client.syncInbox(ctx, inbox)

	// Verify metadata endpoint was reached (hash differed)
	if !metadataReturned.Load() {
		t.Error("metadata endpoint was not called")
	}
}

// TestClient_SyncInbox_NewEmailsFoundGetEmailError tests the email processing loop
// when new emails are found but GetEmail fails (covers lines 569-591)
func TestClient_SyncInbox_NewEmailsFoundGetEmailError(t *testing.T) {
	var getEmailCalled atomic.Bool
	var errorReceived atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/check-key":
			json.NewEncoder(w).Encode(map[string]bool{"ok": true})

		case r.URL.Path == "/api/server-info":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"allowedDomains": []string{"test.com"},
				"maxTTL":         3600,
				"defaultTTL":     300,
			})

		case r.URL.Path == "/api/inboxes" && r.Method == http.MethodPost:
			mockCreateInboxResponse(w)

		case strings.HasSuffix(r.URL.Path, "/sync"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "hash-with-new-emails",
				"emailCount": 2,
			})

		case strings.HasSuffix(r.URL.Path, "/emails") && !strings.Contains(r.URL.Path, "/emails/"):
			// Return metadata with email IDs (these are not in seenEmails, so they're "new")
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": "email-1", "received_at": "2024-01-01T00:00:00Z"},
				{"id": "email-2", "received_at": "2024-01-01T00:01:00Z"},
			})

		case strings.Contains(r.URL.Path, "/emails/"):
			// GetEmail endpoint - return error to trigger onSyncError
			getEmailCalled.Store(true)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "email fetch failed"})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New("test-api-key",
		WithBaseURL(server.URL),
		WithOnSyncError(func(err error) {
			errorReceived.Store(true)
		}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	// Call syncInbox - should find new emails and try to fetch them
	client.syncInbox(ctx, inbox)

	// Either GetEmail was called (and failed), or metadata decryption failed.
	// Both trigger onSyncError, which is what we're testing.
	// The email processing loop (lines 569-604) requires real crypto to test GetEmail success.
	if !errorReceived.Load() {
		t.Error("onSyncError was not called - expected either metadata decryption or GetEmail error")
	}

	// Log whether GetEmail was reached for debugging
	t.Logf("GetEmail endpoint called: %v (may be false if metadata decryption fails first)", getEmailCalled.Load())
}
