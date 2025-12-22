package vaultsandbox

import (
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
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

func TestConvertDecryptedEmail_AuthResults(t *testing.T) {
	inbox := &Inbox{}

	t.Run("with valid auth results", func(t *testing.T) {
		authResultsJSON := json.RawMessage(`{
			"spf": {"status": "pass", "domain": "example.com"},
			"dkim": [{"status": "pass", "domain": "example.com", "selector": "default"}],
			"dmarc": {"status": "pass", "policy": "reject"},
			"reverseDns": {"status": "pass", "hostname": "mail.example.com"}
		}`)

		decrypted := &crypto.DecryptedEmail{
			ID:          "test-id",
			From:        "sender@example.com",
			To:          []string{"recipient@example.com"},
			Subject:     "Test Subject",
			AuthResults: authResultsJSON,
		}

		email := inbox.convertDecryptedEmail(decrypted)

		if email.AuthResults == nil {
			t.Fatal("AuthResults should not be nil")
		}
		if email.AuthResults.SPF == nil {
			t.Fatal("SPF should not be nil")
		}
		if email.AuthResults.SPF.Status != "pass" {
			t.Errorf("SPF.Status = %s, want pass", email.AuthResults.SPF.Status)
		}
		if email.AuthResults.SPF.Domain != "example.com" {
			t.Errorf("SPF.Domain = %s, want example.com", email.AuthResults.SPF.Domain)
		}
		if len(email.AuthResults.DKIM) != 1 {
			t.Fatalf("DKIM length = %d, want 1", len(email.AuthResults.DKIM))
		}
		if email.AuthResults.DKIM[0].Status != "pass" {
			t.Errorf("DKIM[0].Status = %s, want pass", email.AuthResults.DKIM[0].Status)
		}
		if email.AuthResults.DMARC == nil {
			t.Fatal("DMARC should not be nil")
		}
		if email.AuthResults.DMARC.Status != "pass" {
			t.Errorf("DMARC.Status = %s, want pass", email.AuthResults.DMARC.Status)
		}
		if email.AuthResults.ReverseDNS == nil {
			t.Fatal("ReverseDNS should not be nil")
		}
		if email.AuthResults.ReverseDNS.Status != "pass" {
			t.Errorf("ReverseDNS.Status = %s, want pass", email.AuthResults.ReverseDNS.Status)
		}
	})

	t.Run("with empty auth results", func(t *testing.T) {
		decrypted := &crypto.DecryptedEmail{
			ID:          "test-id",
			From:        "sender@example.com",
			To:          []string{"recipient@example.com"},
			Subject:     "Test Subject",
			AuthResults: nil,
		}

		email := inbox.convertDecryptedEmail(decrypted)

		if email.AuthResults != nil {
			t.Error("AuthResults should be nil when not present")
		}
	})

	t.Run("with invalid JSON auth results", func(t *testing.T) {
		decrypted := &crypto.DecryptedEmail{
			ID:          "test-id",
			From:        "sender@example.com",
			To:          []string{"recipient@example.com"},
			Subject:     "Test Subject",
			AuthResults: json.RawMessage(`{invalid json`),
		}

		email := inbox.convertDecryptedEmail(decrypted)

		// Should gracefully handle invalid JSON
		if email.AuthResults != nil {
			t.Error("AuthResults should be nil for invalid JSON")
		}
	})

	t.Run("validate and IsPassing work correctly", func(t *testing.T) {
		authResultsJSON := json.RawMessage(`{
			"spf": {"status": "pass"},
			"dkim": [{"status": "pass"}],
			"dmarc": {"status": "pass"}
		}`)

		decrypted := &crypto.DecryptedEmail{
			ID:          "test-id",
			AuthResults: authResultsJSON,
		}

		email := inbox.convertDecryptedEmail(decrypted)

		if email.AuthResults == nil {
			t.Fatal("AuthResults should not be nil")
		}

		validation := email.AuthResults.Validate()
		if !validation.Passed {
			t.Error("Validate().Passed should be true")
		}
		if !email.AuthResults.IsPassing() {
			t.Error("IsPassing() should return true")
		}
	})
}

func TestWaitConfig_MatchesEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    *Email
		cfg      *waitConfig
		expected bool
	}{
		{
			name:     "matches with empty config",
			email:    &Email{Subject: "Test", From: "sender@example.com"},
			cfg:      &waitConfig{},
			expected: true,
		},
		{
			name:     "matches exact subject",
			email:    &Email{Subject: "Hello World", From: "sender@example.com"},
			cfg:      &waitConfig{subject: "Hello World"},
			expected: true,
		},
		{
			name:     "does not match different subject",
			email:    &Email{Subject: "Hello World", From: "sender@example.com"},
			cfg:      &waitConfig{subject: "Goodbye World"},
			expected: false,
		},
		{
			name:     "matches exact from",
			email:    &Email{Subject: "Test", From: "sender@example.com"},
			cfg:      &waitConfig{from: "sender@example.com"},
			expected: true,
		},
		{
			name:     "does not match different from",
			email:    &Email{Subject: "Test", From: "sender@example.com"},
			cfg:      &waitConfig{from: "other@example.com"},
			expected: false,
		},
		{
			name:  "matches subject regex",
			email: &Email{Subject: "Order #12345 Confirmation", From: "shop@example.com"},
			cfg: &waitConfig{
				subjectRegex: regexp.MustCompile(`Order #\d+ Confirmation`),
			},
			expected: true,
		},
		{
			name:  "does not match subject regex",
			email: &Email{Subject: "Shipping Update", From: "shop@example.com"},
			cfg: &waitConfig{
				subjectRegex: regexp.MustCompile(`Order #\d+ Confirmation`),
			},
			expected: false,
		},
		{
			name:  "matches custom predicate",
			email: &Email{Subject: "Test", From: "sender@example.com", Text: "important content"},
			cfg: &waitConfig{
				predicate: func(e *Email) bool {
					return e.Text == "important content"
				},
			},
			expected: true,
		},
		{
			name:  "does not match custom predicate",
			email: &Email{Subject: "Test", From: "sender@example.com", Text: "regular content"},
			cfg: &waitConfig{
				predicate: func(e *Email) bool {
					return e.Text == "important content"
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.Matches(tt.email)
			if result != tt.expected {
				t.Errorf("waitConfig.Matches() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseMetadata_Valid(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected *crypto.DecryptedMetadata
	}{
		{
			name: "complete metadata",
			json: `{"from":"sender@example.com","to":"recipient@example.com","subject":"Test Subject","receivedAt":"2024-01-15T10:30:00Z"}`,
			expected: &crypto.DecryptedMetadata{
				From:       "sender@example.com",
				To:         "recipient@example.com",
				Subject:    "Test Subject",
				ReceivedAt: "2024-01-15T10:30:00Z",
			},
		},
		{
			name: "minimal metadata",
			json: `{"from":"sender@example.com","to":"recipient@example.com","subject":""}`,
			expected: &crypto.DecryptedMetadata{
				From:    "sender@example.com",
				To:      "recipient@example.com",
				Subject: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseMetadata([]byte(tt.json))
			if err != nil {
				t.Fatalf("parseMetadata() error = %v", err)
			}
			if result.From != tt.expected.From {
				t.Errorf("From = %q, want %q", result.From, tt.expected.From)
			}
			if result.To != tt.expected.To {
				t.Errorf("To = %q, want %q", result.To, tt.expected.To)
			}
			if result.Subject != tt.expected.Subject {
				t.Errorf("Subject = %q, want %q", result.Subject, tt.expected.Subject)
			}
			if result.ReceivedAt != tt.expected.ReceivedAt {
				t.Errorf("ReceivedAt = %q, want %q", result.ReceivedAt, tt.expected.ReceivedAt)
			}
		})
	}
}

func TestParseMetadata_InvalidJSON(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{"invalid JSON", `{invalid json`},
		{"empty string", ``},
		{"not an object", `"just a string"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseMetadata([]byte(tt.json))
			if err == nil {
				t.Error("parseMetadata() expected error, got nil")
			}
		})
	}
}

func TestParseParsedContent_WithHeaders(t *testing.T) {
	jsonData := `{
		"text": "Plain text body",
		"html": "<p>HTML body</p>",
		"headers": {
			"X-Custom-Header": "custom-value",
			"X-Another": "another-value"
		},
		"links": ["https://example.com"],
		"attachments": []
	}`

	parsed, headers, err := parseParsedContent([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseParsedContent() error = %v", err)
	}

	if parsed.Text != "Plain text body" {
		t.Errorf("Text = %q, want %q", parsed.Text, "Plain text body")
	}
	if parsed.HTML != "<p>HTML body</p>" {
		t.Errorf("HTML = %q, want %q", parsed.HTML, "<p>HTML body</p>")
	}
	if len(parsed.Links) != 1 || parsed.Links[0] != "https://example.com" {
		t.Errorf("Links = %v, want [https://example.com]", parsed.Links)
	}

	// Verify headers conversion
	if headers == nil {
		t.Fatal("headers should not be nil")
	}
	if headers["X-Custom-Header"] != "custom-value" {
		t.Errorf("headers[X-Custom-Header] = %q, want %q", headers["X-Custom-Header"], "custom-value")
	}
	if headers["X-Another"] != "another-value" {
		t.Errorf("headers[X-Another] = %q, want %q", headers["X-Another"], "another-value")
	}
}

func TestParseParsedContent_NonStringHeaders(t *testing.T) {
	// Headers with non-string values should be filtered out
	jsonData := `{
		"text": "body",
		"html": "",
		"headers": {
			"X-String-Header": "string-value",
			"X-Number-Header": 123,
			"X-Bool-Header": true,
			"X-Null-Header": null,
			"X-Array-Header": ["a", "b"]
		}
	}`

	_, headers, err := parseParsedContent([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseParsedContent() error = %v", err)
	}

	// Only string-typed headers should be preserved
	if len(headers) != 1 {
		t.Errorf("headers length = %d, want 1 (only string headers)", len(headers))
	}
	if headers["X-String-Header"] != "string-value" {
		t.Errorf("headers[X-String-Header] = %q, want %q", headers["X-String-Header"], "string-value")
	}
	// Non-string headers should not be present
	if _, ok := headers["X-Number-Header"]; ok {
		t.Error("X-Number-Header should not be in headers")
	}
}

func TestParseParsedContent_EmptyHeaders(t *testing.T) {
	jsonData := `{"text": "body", "html": "", "headers": {}}`

	_, headers, err := parseParsedContent([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseParsedContent() error = %v", err)
	}

	// Empty headers map should result in nil
	if headers != nil {
		t.Errorf("headers = %v, want nil for empty headers", headers)
	}
}

func TestParseParsedContent_InvalidJSON(t *testing.T) {
	_, _, err := parseParsedContent([]byte(`{invalid json`))
	if err == nil {
		t.Error("parseParsedContent() expected error, got nil")
	}
}

func TestBuildDecryptedEmail_Complete(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	rawEmail := &api.RawEmail{
		ID:         "email-123",
		ReceivedAt: now,
		IsRead:     true,
	}
	metadata := &crypto.DecryptedMetadata{
		From:       "sender@example.com",
		To:         "recipient@example.com",
		Subject:    "Test Subject",
		ReceivedAt: "2024-01-15T10:30:00Z",
	}

	result := buildDecryptedEmail(rawEmail, metadata)

	if result.ID != "email-123" {
		t.Errorf("ID = %q, want %q", result.ID, "email-123")
	}
	if result.From != "sender@example.com" {
		t.Errorf("From = %q, want %q", result.From, "sender@example.com")
	}
	if len(result.To) != 1 || result.To[0] != "recipient@example.com" {
		t.Errorf("To = %v, want [recipient@example.com]", result.To)
	}
	if result.Subject != "Test Subject" {
		t.Errorf("Subject = %q, want %q", result.Subject, "Test Subject")
	}
	if result.IsRead != true {
		t.Error("IsRead = false, want true")
	}
	// ReceivedAt should be parsed from metadata
	expected, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
	if !result.ReceivedAt.Equal(expected) {
		t.Errorf("ReceivedAt = %v, want %v", result.ReceivedAt, expected)
	}
}

func TestBuildDecryptedEmail_ReceivedAtFallback(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	tests := []struct {
		name              string
		metadataReceivedAt string
		expectedTime      time.Time
	}{
		{
			name:              "empty receivedAt uses API timestamp",
			metadataReceivedAt: "",
			expectedTime:      now,
		},
		{
			name:              "invalid receivedAt uses API timestamp",
			metadataReceivedAt: "not-a-valid-date",
			expectedTime:      now,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawEmail := &api.RawEmail{
				ID:         "email-123",
				ReceivedAt: now,
				IsRead:     false,
			}
			metadata := &crypto.DecryptedMetadata{
				From:       "sender@example.com",
				To:         "recipient@example.com",
				Subject:    "Test",
				ReceivedAt: tt.metadataReceivedAt,
			}

			result := buildDecryptedEmail(rawEmail, metadata)

			if !result.ReceivedAt.Equal(tt.expectedTime) {
				t.Errorf("ReceivedAt = %v, want %v", result.ReceivedAt, tt.expectedTime)
			}
		})
	}
}

// Note: Full inbox tests require a real API connection
// These tests verify the data structures and validation
// Integration tests are in the integration/ directory
