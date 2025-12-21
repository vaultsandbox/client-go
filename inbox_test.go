package vaultsandbox

import (
	"errors"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go/internal/crypto"
)

func TestExportedInbox_Validate(t *testing.T) {
	// Generate a valid keypair for testing
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	tests := []struct {
		name    string
		data    *ExportedInbox
		wantErr bool
	}{
		{
			name: "valid data",
			data: &ExportedInbox{
				EmailAddress: "test@example.com",
				ExpiresAt:    time.Now().Add(time.Hour),
				InboxHash:    "hash123",
				ServerSigPk:  crypto.ToBase64URL(make([]byte, crypto.MLDSAPublicKeySize)),
				PublicKeyB64: crypto.ToBase64URL(kp.PublicKey),
				SecretKeyB64: crypto.ToBase64URL(kp.SecretKey),
				ExportedAt:   time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing email address",
			data: &ExportedInbox{
				EmailAddress: "",
				SecretKeyB64: crypto.ToBase64URL(kp.SecretKey),
			},
			wantErr: true,
		},
		{
			name: "missing secret key",
			data: &ExportedInbox{
				EmailAddress: "test@example.com",
				SecretKeyB64: "",
			},
			wantErr: true,
		},
		{
			name: "invalid secret key size",
			data: &ExportedInbox{
				EmailAddress: "test@example.com",
				SecretKeyB64: crypto.ToBase64URL([]byte("too short")),
			},
			wantErr: true,
		},
		{
			name: "invalid base64 secret key",
			data: &ExportedInbox{
				EmailAddress: "test@example.com",
				SecretKeyB64: "!!!invalid base64!!!",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.data.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidImportData) {
				t.Errorf("Validate() error = %v, want ErrInvalidImportData", err)
			}
		})
	}
}

func TestExportedInbox_Fields(t *testing.T) {
	now := time.Now()
	data := &ExportedInbox{
		EmailAddress: "test@example.com",
		ExpiresAt:    now.Add(time.Hour),
		InboxHash:    "hash123",
		ServerSigPk:  "sigpk",
		PublicKeyB64: "pubkey",
		SecretKeyB64: "seckey",
		ExportedAt:   now,
	}

	if data.EmailAddress != "test@example.com" {
		t.Errorf("EmailAddress = %s, want test@example.com", data.EmailAddress)
	}
	if data.InboxHash != "hash123" {
		t.Errorf("InboxHash = %s, want hash123", data.InboxHash)
	}
}

func TestInbox_Export(t *testing.T) {
	// Generate valid keypair
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	serverSigPk := make([]byte, crypto.MLDSAPublicKeySize)
	now := time.Now()
	expiresAt := now.Add(time.Hour)

	inbox := &Inbox{
		emailAddress: "test@example.com",
		expiresAt:    expiresAt,
		inboxHash:    "hash123abc",
		serverSigPk:  serverSigPk,
		keypair:      kp,
	}

	exported := inbox.Export()

	// Test all required fields are present
	t.Run("required fields present", func(t *testing.T) {
		if exported.EmailAddress == "" {
			t.Error("EmailAddress should not be empty")
		}
		if exported.InboxHash == "" {
			t.Error("InboxHash should not be empty")
		}
		if exported.ServerSigPk == "" {
			t.Error("ServerSigPk should not be empty")
		}
		if exported.PublicKeyB64 == "" {
			t.Error("PublicKeyB64 should not be empty")
		}
		if exported.SecretKeyB64 == "" {
			t.Error("SecretKeyB64 should not be empty")
		}
		if exported.ExportedAt.IsZero() {
			t.Error("ExportedAt should not be zero")
		}
	})

	// Test field values match inbox
	t.Run("field values match", func(t *testing.T) {
		if exported.EmailAddress != "test@example.com" {
			t.Errorf("EmailAddress = %q, want %q", exported.EmailAddress, "test@example.com")
		}
		if exported.InboxHash != "hash123abc" {
			t.Errorf("InboxHash = %q, want %q", exported.InboxHash, "hash123abc")
		}
		if !exported.ExpiresAt.Equal(expiresAt) {
			t.Errorf("ExpiresAt = %v, want %v", exported.ExpiresAt, expiresAt)
		}
	})

	// Test valid timestamps (can be parsed as time.Time - Go's native format)
	t.Run("valid timestamps", func(t *testing.T) {
		if exported.ExpiresAt.IsZero() {
			t.Error("ExpiresAt should be a valid timestamp")
		}
		if exported.ExportedAt.IsZero() {
			t.Error("ExportedAt should be a valid timestamp")
		}
		// ExportedAt should be close to now (within 1 second)
		if time.Since(exported.ExportedAt) > time.Second {
			t.Errorf("ExportedAt too far from now: %v", exported.ExportedAt)
		}
	})

	// Test valid base64 keys
	t.Run("valid base64 keys", func(t *testing.T) {
		// PublicKeyB64
		pubKey, err := crypto.FromBase64URL(exported.PublicKeyB64)
		if err != nil {
			t.Errorf("PublicKeyB64 is not valid base64: %v", err)
		}
		if len(pubKey) != crypto.MLKEMPublicKeySize {
			t.Errorf("PublicKeyB64 decoded length = %d, want %d", len(pubKey), crypto.MLKEMPublicKeySize)
		}

		// SecretKeyB64
		secKey, err := crypto.FromBase64URL(exported.SecretKeyB64)
		if err != nil {
			t.Errorf("SecretKeyB64 is not valid base64: %v", err)
		}
		if len(secKey) != crypto.MLKEMSecretKeySize {
			t.Errorf("SecretKeyB64 decoded length = %d, want %d", len(secKey), crypto.MLKEMSecretKeySize)
		}

		// ServerSigPk
		sigPk, err := crypto.FromBase64URL(exported.ServerSigPk)
		if err != nil {
			t.Errorf("ServerSigPk is not valid base64: %v", err)
		}
		if len(sigPk) != crypto.MLDSAPublicKeySize {
			t.Errorf("ServerSigPk decoded length = %d, want %d", len(sigPk), crypto.MLDSAPublicKeySize)
		}
	})
}

func TestInbox_Export_Roundtrip(t *testing.T) {
	// Generate valid keypair
	kp, err := crypto.GenerateKeypair()
	if err != nil {
		t.Fatalf("GenerateKeypair() error = %v", err)
	}

	serverSigPk := make([]byte, crypto.MLDSAPublicKeySize)
	expiresAt := time.Now().Add(time.Hour)

	original := &Inbox{
		emailAddress: "test@example.com",
		expiresAt:    expiresAt,
		inboxHash:    "hash123",
		serverSigPk:  serverSigPk,
		keypair:      kp,
	}

	// Export and create new inbox from export
	exported := original.Export()

	// Validate exported data
	if err := exported.Validate(); err != nil {
		t.Fatalf("exported data validation failed: %v", err)
	}

	// Verify we can reconstruct from export
	reconstructed, err := newInboxFromExport(exported, nil)
	if err != nil {
		t.Fatalf("newInboxFromExport() error = %v", err)
	}

	// Verify all fields match
	if reconstructed.emailAddress != original.emailAddress {
		t.Errorf("emailAddress = %q, want %q", reconstructed.emailAddress, original.emailAddress)
	}
	if reconstructed.inboxHash != original.inboxHash {
		t.Errorf("inboxHash = %q, want %q", reconstructed.inboxHash, original.inboxHash)
	}
	if !reconstructed.expiresAt.Equal(original.expiresAt) {
		t.Errorf("expiresAt = %v, want %v", reconstructed.expiresAt, original.expiresAt)
	}
}

// Note: Full inbox tests require a real API connection
// These tests verify the data structures and validation
// Integration tests are in the integration/ directory
