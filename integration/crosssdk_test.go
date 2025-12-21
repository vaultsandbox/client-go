//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
	"github.com/vaultsandbox/client-go/internal/crypto"
)

// ExternalExportedInbox represents the expected export format for cross-SDK compatibility.
type ExternalExportedInbox struct {
	EmailAddress string `json:"emailAddress"`
	ExpiresAt    string `json:"expiresAt"`
	InboxHash    string `json:"inboxHash"`
	ServerSigPk  string `json:"serverSigPk"`
	PublicKeyB64 string `json:"publicKeyB64"`
	SecretKeyB64 string `json:"secretKeyB64"`
	ExportedAt   string `json:"exportedAt"`
}

// TestCrossSDK_ExportFormatCompatibility verifies the export format is compatible.
func TestCrossSDK_ExportFormatCompatibility(t *testing.T) {
	client := newClient(t)
	ctx := testContext(t)

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Export from Go SDK
	exported := inbox.Export()

	// Marshal to JSON (same format as file export)
	jsonData, err := json.MarshalIndent(exported, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	t.Logf("Go SDK export:\n%s", string(jsonData))

	// Verify it can be parsed as the standard format
	var externalFormat ExternalExportedInbox
	if err := json.Unmarshal(jsonData, &externalFormat); err != nil {
		t.Fatalf("Failed to parse as external format: %v", err)
	}

	// Verify all fields are present and valid
	if externalFormat.EmailAddress != exported.EmailAddress {
		t.Errorf("emailAddress mismatch: got %s, want %s", externalFormat.EmailAddress, exported.EmailAddress)
	}
	if externalFormat.InboxHash != exported.InboxHash {
		t.Errorf("inboxHash mismatch: got %s, want %s", externalFormat.InboxHash, exported.InboxHash)
	}
	if externalFormat.ServerSigPk != exported.ServerSigPk {
		t.Errorf("serverSigPk mismatch: got %s, want %s", externalFormat.ServerSigPk, exported.ServerSigPk)
	}
	if externalFormat.PublicKeyB64 != exported.PublicKeyB64 {
		t.Errorf("publicKeyB64 mismatch: got %s, want %s", externalFormat.PublicKeyB64, exported.PublicKeyB64)
	}
	if externalFormat.SecretKeyB64 != exported.SecretKeyB64 {
		t.Errorf("secretKeyB64 mismatch: got %s, want %s", externalFormat.SecretKeyB64, exported.SecretKeyB64)
	}

	// Verify timestamps can be parsed
	if _, err := time.Parse(time.RFC3339Nano, externalFormat.ExpiresAt); err != nil {
		t.Errorf("Failed to parse expiresAt as RFC3339: %v", err)
	}
	if _, err := time.Parse(time.RFC3339Nano, externalFormat.ExportedAt); err != nil {
		t.Errorf("Failed to parse exportedAt as RFC3339: %v", err)
	}
}

// TestCrossSDK_ImportExternalExport tests importing an inbox exported from another SDK.
func TestCrossSDK_ImportExternalExport(t *testing.T) {
	externalPath := os.Getenv("EXTERNAL_EXPORT_FILE")
	if externalPath == "" {
		t.Skip("skipping: EXTERNAL_EXPORT_FILE not set")
	}

	client := newClient(t)
	ctx := testContext(t)

	// Import from external export file
	inbox, err := client.ImportInboxFromFile(ctx, externalPath)
	if err != nil {
		t.Fatalf("ImportInboxFromFile() error = %v", err)
	}

	t.Logf("Imported from external SDK: %s", inbox.EmailAddress())

	// Verify inbox works
	if inbox.EmailAddress() == "" {
		t.Error("EmailAddress is empty")
	}
	if inbox.InboxHash() == "" {
		t.Error("InboxHash is empty")
	}

	// Try to get emails (verifies crypto works)
	emails, err := inbox.GetEmails(ctx)
	if err != nil {
		t.Fatalf("GetEmails() error = %v", err)
	}
	t.Logf("Found %d emails in imported inbox", len(emails))

	// If there are emails, verify we can decrypt them
	for i, email := range emails {
		if email.ID == "" {
			t.Errorf("Email %d has empty ID", i)
		}
		if email.From == "" {
			t.Errorf("Email %d has empty From", i)
		}
		t.Logf("Email %d: ID=%s, Subject=%s, From=%s", i, email.ID, email.Subject, email.From)
	}
}

// TestCrossSDK_KeypairCompatibility verifies keypair format compatibility.
func TestCrossSDK_KeypairCompatibility(t *testing.T) {
	// Generate a keypair in Go
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	// Verify key sizes (ML-KEM-768: publicKey=1184, secretKey=2400)
	if len(kp.PublicKey) != crypto.MLKEMPublicKeySize {
		t.Errorf("PublicKey size = %d, want %d", len(kp.PublicKey), crypto.MLKEMPublicKeySize)
	}
	if len(kp.SecretKey) != crypto.MLKEMSecretKeySize {
		t.Errorf("SecretKey size = %d, want %d", len(kp.SecretKey), crypto.MLKEMSecretKeySize)
	}

	// Verify base64 encoding produces same result when decoded
	decodedPK, err := crypto.FromBase64URL(kp.PublicKeyB64)
	if err != nil {
		t.Fatalf("Failed to decode PublicKeyB64: %v", err)
	}
	if len(decodedPK) != len(kp.PublicKey) {
		t.Errorf("Decoded PublicKey size = %d, want %d", len(decodedPK), len(kp.PublicKey))
	}

	t.Logf("Keypair: PublicKey=%d bytes, SecretKey=%d bytes, PublicKeyB64=%d chars",
		len(kp.PublicKey), len(kp.SecretKey), len(kp.PublicKeyB64))
}

// TestCrossSDK_Base64Compatibility verifies base64 encoding is correct.
func TestCrossSDK_Base64Compatibility(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string // Expected base64url encoding without padding
	}{
		{"hello", []byte("hello"), "aGVsbG8"},
		{"hello world", []byte("hello world"), "aGVsbG8gd29ybGQ"},
		{"binary with + and /", []byte{0xfb, 0xff, 0x3f}, "-_8_"}, // URL-safe characters
		{"empty", []byte{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := crypto.ToBase64URL(tt.input)
			// Uses RawURLEncoding (no padding, URL-safe)
			if tt.expected != "" && encoded != tt.expected {
				t.Errorf("ToBase64URL(%v) = %s, want %s", tt.input, encoded, tt.expected)
			}

			// Verify round-trip
			decoded, err := crypto.FromBase64URL(encoded)
			if err != nil {
				t.Fatalf("FromBase64URL() error = %v", err)
			}
			if string(decoded) != string(tt.input) {
				t.Errorf("Round-trip failed: got %v, want %v", decoded, tt.input)
			}
		})
	}
}

// TestCrossSDK_ExportedInboxJSONFields verifies JSON field naming is correct.
func TestCrossSDK_ExportedInboxJSONFields(t *testing.T) {
	exported := &vaultsandbox.ExportedInbox{
		EmailAddress: "test@example.com",
		ExpiresAt:    time.Now().Add(time.Hour),
		InboxHash:    "hash123",
		ServerSigPk:  "sigpk",
		PublicKeyB64: "pubkey",
		SecretKeyB64: "seckey",
		ExportedAt:   time.Now(),
	}

	jsonData, err := json.Marshal(exported)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Parse as generic map to check field names
	var fields map[string]interface{}
	if err := json.Unmarshal(jsonData, &fields); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify expected field names (camelCase for cross-SDK compatibility)
	expectedFields := []string{
		"emailAddress",
		"expiresAt",
		"inboxHash",
		"serverSigPk",
		"publicKeyB64",
		"secretKeyB64",
		"exportedAt",
	}

	for _, field := range expectedFields {
		if _, ok := fields[field]; !ok {
			t.Errorf("Missing field: %s", field)
		}
	}

	// Verify no unexpected fields
	if len(fields) != len(expectedFields) {
		t.Errorf("Got %d fields, want %d", len(fields), len(expectedFields))
		t.Logf("Fields: %v", fields)
	}
}

// TestCrossSDK_CryptoConstants verifies crypto constants are correct.
func TestCrossSDK_CryptoConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      int
		expected int
	}{
		{"MLKEMPublicKeySize", crypto.MLKEMPublicKeySize, 1184},
		{"MLKEMSecretKeySize", crypto.MLKEMSecretKeySize, 2400},
		{"MLKEMCiphertextSize", crypto.MLKEMCiphertextSize, 1088},
		{"MLDSAPublicKeySize", crypto.MLDSAPublicKeySize, 1952},
		{"MLDSASignatureSize", crypto.MLDSASignatureSize, 3309},
		{"AESKeySize", crypto.AESKeySize, 32},
		{"AESNonceSize", crypto.AESNonceSize, 12},
		{"AESTagSize", crypto.AESTagSize, 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.expected)
			}
		})
	}
}

// TestCrossSDK_HKDFContext verifies HKDF context string is correct.
func TestCrossSDK_HKDFContext(t *testing.T) {
	expected := "vaultsandbox:email:v1"
	if crypto.HKDFContext != expected {
		t.Errorf("HKDFContext = %s, want %s", crypto.HKDFContext, expected)
	}
}

// TestCrossSDK_CompareExports compares exports from different SDKs.
// Requires both a Go export and external export file to be provided.
func TestCrossSDK_CompareExports(t *testing.T) {
	goPath := os.Getenv("GO_EXPORT_FILE")
	externalPath := os.Getenv("EXTERNAL_EXPORT_FILE")

	if goPath == "" || externalPath == "" {
		t.Skip("skipping: GO_EXPORT_FILE and EXTERNAL_EXPORT_FILE not set")
	}

	// Read Go export
	goData, err := os.ReadFile(goPath)
	if err != nil {
		t.Fatalf("Failed to read Go export: %v", err)
	}

	// Read external export
	externalData, err := os.ReadFile(externalPath)
	if err != nil {
		t.Fatalf("Failed to read external export: %v", err)
	}

	// Parse both
	var goExport vaultsandbox.ExportedInbox
	if err := json.Unmarshal(goData, &goExport); err != nil {
		t.Fatalf("Failed to parse Go export: %v", err)
	}

	var externalExport ExternalExportedInbox
	if err := json.Unmarshal(externalData, &externalExport); err != nil {
		t.Fatalf("Failed to parse external export: %v", err)
	}

	t.Log("=== Go Export ===")
	t.Logf("EmailAddress: %s", goExport.EmailAddress)
	t.Logf("InboxHash: %s", goExport.InboxHash)
	t.Logf("SecretKeyB64: %s...%s", goExport.SecretKeyB64[:20], goExport.SecretKeyB64[len(goExport.SecretKeyB64)-20:])

	t.Log("=== External Export ===")
	t.Logf("EmailAddress: %s", externalExport.EmailAddress)
	t.Logf("InboxHash: %s", externalExport.InboxHash)
	t.Logf("SecretKeyB64: %s...%s", externalExport.SecretKeyB64[:20], externalExport.SecretKeyB64[len(externalExport.SecretKeyB64)-20:])

	// If they're the same inbox, verify keys match
	if goExport.EmailAddress == externalExport.EmailAddress {
		if goExport.SecretKeyB64 != externalExport.SecretKeyB64 {
			t.Error("Same inbox but secretKeyB64 differs")
		}
		if goExport.PublicKeyB64 != externalExport.PublicKeyB64 {
			t.Error("Same inbox but publicKeyB64 differs")
		}
	}
}

// testContext creates a test context with cleanup.
func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	return ctx
}
