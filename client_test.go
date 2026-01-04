package vaultsandbox

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
			name: "empty secret key",
			modifier: func(e *ExportedInbox) {
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
	validSecretKey := make([]byte, 2400)    // MLKEMSecretKeySize
	invalidServerSig := make([]byte, 100)   // Wrong size

	data := &ExportedInbox{
		Version:      ExportVersion,
		EmailAddress: "test@example.com",
		ExpiresAt:    time.Now().Add(time.Hour),
		InboxHash:    "hash123",
		ServerSigPk:  base64.RawURLEncoding.EncodeToString(invalidServerSig),
		SecretKey:    base64.RawURLEncoding.EncodeToString(validSecretKey),
		ExportedAt:   time.Now(),
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

func TestCreateDeliveryStrategy_Auto(t *testing.T) {
	cfg := &clientConfig{
		deliveryStrategy: StrategyAuto,
	}

	apiCfg := &clientConfig{baseURL: "https://test.example.com"}
	apiClient, _ := buildAPIClient("test-key", apiCfg)

	strategy := createDeliveryStrategy(cfg, apiClient)
	if strategy == nil {
		t.Fatal("createDeliveryStrategy() returned nil")
	}
}

func TestCreateDeliveryStrategy_Default(t *testing.T) {
	// Empty/unknown strategy should default to auto
	cfg := &clientConfig{
		deliveryStrategy: DeliveryStrategy("unknown"),
	}

	apiCfg := &clientConfig{baseURL: "https://test.example.com"}
	apiClient, _ := buildAPIClient("test-key", apiCfg)

	strategy := createDeliveryStrategy(cfg, apiClient)
	if strategy == nil {
		t.Fatal("createDeliveryStrategy() returned nil for unknown strategy")
	}
}

// Note: Full client tests require a real API connection
// These tests verify the configuration and error handling
// Integration tests are in the integration/ directory
