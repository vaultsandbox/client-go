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

// NodeSDKExportedInbox represents the export format from Node SDK.
// This matches the ExportedInboxData interface in Node SDK.
type NodeSDKExportedInbox struct {
	EmailAddress string `json:"emailAddress"`
	ExpiresAt    string `json:"expiresAt"`    // ISO string
	InboxHash    string `json:"inboxHash"`
	ServerSigPk  string `json:"serverSigPk"`
	PublicKeyB64 string `json:"publicKeyB64"`
	SecretKeyB64 string `json:"secretKeyB64"`
	ExportedAt   string `json:"exportedAt"`   // ISO string
}

// TestCrossSDK_ExportFormatCompatibility verifies Go SDK export format
// is compatible with Node SDK import format.
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

	// Verify it can be parsed as Node SDK format
	var nodeFormat NodeSDKExportedInbox
	if err := json.Unmarshal(jsonData, &nodeFormat); err != nil {
		t.Fatalf("Failed to parse as Node SDK format: %v", err)
	}

	// Verify all fields are present and valid
	if nodeFormat.EmailAddress != exported.EmailAddress {
		t.Errorf("emailAddress mismatch: got %s, want %s", nodeFormat.EmailAddress, exported.EmailAddress)
	}
	if nodeFormat.InboxHash != exported.InboxHash {
		t.Errorf("inboxHash mismatch: got %s, want %s", nodeFormat.InboxHash, exported.InboxHash)
	}
	if nodeFormat.ServerSigPk != exported.ServerSigPk {
		t.Errorf("serverSigPk mismatch: got %s, want %s", nodeFormat.ServerSigPk, exported.ServerSigPk)
	}
	if nodeFormat.PublicKeyB64 != exported.PublicKeyB64 {
		t.Errorf("publicKeyB64 mismatch: got %s, want %s", nodeFormat.PublicKeyB64, exported.PublicKeyB64)
	}
	if nodeFormat.SecretKeyB64 != exported.SecretKeyB64 {
		t.Errorf("secretKeyB64 mismatch: got %s, want %s", nodeFormat.SecretKeyB64, exported.SecretKeyB64)
	}

	// Verify timestamps can be parsed
	if _, err := time.Parse(time.RFC3339Nano, nodeFormat.ExpiresAt); err != nil {
		t.Errorf("Failed to parse expiresAt as RFC3339: %v", err)
	}
	if _, err := time.Parse(time.RFC3339Nano, nodeFormat.ExportedAt); err != nil {
		t.Errorf("Failed to parse exportedAt as RFC3339: %v", err)
	}
}

// TestCrossSDK_ImportNodeExport tests importing an inbox exported from Node SDK.
func TestCrossSDK_ImportNodeExport(t *testing.T) {
	nodePath := os.Getenv("NODE_EXPORT_FILE")
	if nodePath == "" {
		t.Skip("skipping: NODE_EXPORT_FILE not set")
	}

	client := newClient(t)
	ctx := testContext(t)

	// Import from Node SDK export file
	inbox, err := client.ImportInboxFromFile(ctx, nodePath)
	if err != nil {
		t.Fatalf("ImportInboxFromFile() error = %v", err)
	}

	t.Logf("Imported from Node SDK: %s", inbox.EmailAddress())

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

	// Verify key sizes match Node SDK expectations
	// Node SDK uses ML-KEM-768: publicKey=1184, secretKey=2400
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

// TestCrossSDK_Base64Compatibility verifies base64 encoding matches Node SDK.
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
			// Node SDK uses RawURLEncoding (no padding, URL-safe)
			// Verify it matches expected
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

// TestCrossSDK_ExportedInboxJSONFields verifies JSON field naming matches Node SDK.
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

	// Verify expected field names (camelCase to match Node SDK)
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

// TestCrossSDK_CryptoConstants verifies crypto constants match Node SDK.
func TestCrossSDK_CryptoConstants(t *testing.T) {
	// These values must match Node SDK for cross-SDK compatibility
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

// TestCrossSDK_HKDFContext verifies HKDF context string matches Node SDK.
func TestCrossSDK_HKDFContext(t *testing.T) {
	expected := "vaultsandbox:email:v1"
	if crypto.HKDFContext != expected {
		t.Errorf("HKDFContext = %s, want %s", crypto.HKDFContext, expected)
	}
}

// TestCrossSDK_CompareExports compares export from both SDKs.
// Requires both a Go export and Node export file to be provided.
func TestCrossSDK_CompareExports(t *testing.T) {
	goPath := os.Getenv("GO_EXPORT_FILE")
	nodePath := os.Getenv("NODE_EXPORT_FILE")

	if goPath == "" || nodePath == "" {
		t.Skip("skipping: GO_EXPORT_FILE and NODE_EXPORT_FILE not set")
	}

	// Read Go export
	goData, err := os.ReadFile(goPath)
	if err != nil {
		t.Fatalf("Failed to read Go export: %v", err)
	}

	// Read Node export
	nodeData, err := os.ReadFile(nodePath)
	if err != nil {
		t.Fatalf("Failed to read Node export: %v", err)
	}

	// Parse both
	var goExport vaultsandbox.ExportedInbox
	if err := json.Unmarshal(goData, &goExport); err != nil {
		t.Fatalf("Failed to parse Go export: %v", err)
	}

	var nodeExport NodeSDKExportedInbox
	if err := json.Unmarshal(nodeData, &nodeExport); err != nil {
		t.Fatalf("Failed to parse Node export: %v", err)
	}

	t.Log("=== Go Export ===")
	t.Logf("EmailAddress: %s", goExport.EmailAddress)
	t.Logf("InboxHash: %s", goExport.InboxHash)
	t.Logf("SecretKeyB64: %s...%s", goExport.SecretKeyB64[:20], goExport.SecretKeyB64[len(goExport.SecretKeyB64)-20:])

	t.Log("=== Node Export ===")
	t.Logf("EmailAddress: %s", nodeExport.EmailAddress)
	t.Logf("InboxHash: %s", nodeExport.InboxHash)
	t.Logf("SecretKeyB64: %s...%s", nodeExport.SecretKeyB64[:20], nodeExport.SecretKeyB64[len(nodeExport.SecretKeyB64)-20:])

	// If they're the same inbox, verify keys match
	if goExport.EmailAddress == nodeExport.EmailAddress {
		if goExport.SecretKeyB64 != nodeExport.SecretKeyB64 {
			t.Error("Same inbox but secretKeyB64 differs")
		}
		if goExport.PublicKeyB64 != nodeExport.PublicKeyB64 {
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
