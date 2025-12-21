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

// Note: Full inbox tests require a real API connection
// These tests verify the data structures and validation
// Integration tests are in the integration/ directory
