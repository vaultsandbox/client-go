package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
)

// mockServerSigPk is a valid base64-encoded ML-DSA public key for testing (1952 bytes)
var mockServerSigPk = base64.RawURLEncoding.EncodeToString(make([]byte, 1952))

// mockSecretKey is a valid base64-encoded ML-KEM secret key for testing (2400 bytes)
var mockSecretKey = base64.RawURLEncoding.EncodeToString(make([]byte, 2400))

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Stdin != os.Stdin {
		t.Error("DefaultConfig().Stdin should be os.Stdin")
	}
	if cfg.Stdout != os.Stdout {
		t.Error("DefaultConfig().Stdout should be os.Stdout")
	}
	if cfg.Stderr != os.Stderr {
		t.Error("DefaultConfig().Stderr should be os.Stderr")
	}
}

func TestEmailOutput_JSONMarshal(t *testing.T) {
	receivedAt := time.Now().Round(time.Second)
	email := EmailOutput{
		ID:         "email-123",
		Subject:    "Test Subject",
		From:       "sender@example.com",
		To:         []string{"recipient@example.com"},
		Text:       "Hello, World!",
		HTML:       "<p>Hello, World!</p>",
		ReceivedAt: receivedAt.Format(time.RFC3339),
		Attachments: []AttachmentOutput{
			{
				Filename:    "file.txt",
				ContentType: "text/plain",
				Size:        100,
			},
		},
	}

	data, err := json.Marshal(email)
	if err != nil {
		t.Fatalf("json.Marshal(EmailOutput) error = %v", err)
	}

	var parsed EmailOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal(EmailOutput) error = %v", err)
	}

	if parsed.ID != email.ID {
		t.Errorf("ID = %q, want %q", parsed.ID, email.ID)
	}
	if parsed.Subject != email.Subject {
		t.Errorf("Subject = %q, want %q", parsed.Subject, email.Subject)
	}
	if parsed.From != email.From {
		t.Errorf("From = %q, want %q", parsed.From, email.From)
	}
	if len(parsed.To) != 1 || parsed.To[0] != email.To[0] {
		t.Errorf("To = %v, want %v", parsed.To, email.To)
	}
	if parsed.Text != email.Text {
		t.Errorf("Text = %q, want %q", parsed.Text, email.Text)
	}
	if parsed.HTML != email.HTML {
		t.Errorf("HTML = %q, want %q", parsed.HTML, email.HTML)
	}
	if parsed.ReceivedAt != email.ReceivedAt {
		t.Errorf("ReceivedAt = %q, want %q", parsed.ReceivedAt, email.ReceivedAt)
	}
	if len(parsed.Attachments) != 1 {
		t.Fatalf("Attachments len = %d, want 1", len(parsed.Attachments))
	}
	if parsed.Attachments[0].Filename != "file.txt" {
		t.Errorf("Attachment.Filename = %q, want %q", parsed.Attachments[0].Filename, "file.txt")
	}
}

func TestEmailOutput_JSONOmitEmpty(t *testing.T) {
	email := EmailOutput{
		ID:         "email-123",
		Subject:    "Test",
		From:       "sender@example.com",
		To:         []string{"recipient@example.com"},
		Text:       "Hello",
		ReceivedAt: time.Now().Format(time.RFC3339),
		// HTML and Attachments intentionally empty
	}

	data, err := json.Marshal(email)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	jsonStr := string(data)

	// HTML should be omitted when empty
	if strings.Contains(jsonStr, `"html":""`) {
		t.Error("empty HTML should be omitted from JSON")
	}

	// Attachments should be omitted when nil/empty
	if strings.Contains(jsonStr, `"attachments":null`) || strings.Contains(jsonStr, `"attachments":[]`) {
		t.Error("empty Attachments should be omitted from JSON")
	}
}

func TestAttachmentOutput_JSONMarshal(t *testing.T) {
	att := AttachmentOutput{
		Filename:    "document.pdf",
		ContentType: "application/pdf",
		Size:        1024,
	}

	data, err := json.Marshal(att)
	if err != nil {
		t.Fatalf("json.Marshal(AttachmentOutput) error = %v", err)
	}

	var parsed AttachmentOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal(AttachmentOutput) error = %v", err)
	}

	if parsed.Filename != att.Filename {
		t.Errorf("Filename = %q, want %q", parsed.Filename, att.Filename)
	}
	if parsed.ContentType != att.ContentType {
		t.Errorf("ContentType = %q, want %q", parsed.ContentType, att.ContentType)
	}
	if parsed.Size != att.Size {
		t.Errorf("Size = %d, want %d", parsed.Size, att.Size)
	}
}

func TestEmailOutput_JSONFieldNames(t *testing.T) {
	email := EmailOutput{
		ID:         "id",
		Subject:    "subj",
		From:       "from",
		To:         []string{"to"},
		Text:       "text",
		HTML:       "html",
		ReceivedAt: "2024-01-01T00:00:00Z",
		Attachments: []AttachmentOutput{
			{Filename: "f", ContentType: "c", Size: 1},
		},
	}

	data, err := json.Marshal(email)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	jsonStr := string(data)

	expectedFields := []string{
		`"id"`,
		`"subject"`,
		`"from"`,
		`"to"`,
		`"text"`,
		`"html"`,
		`"receivedAt"`,
		`"attachments"`,
		`"filename"`,
		`"contentType"`,
		`"size"`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON should contain field %s", field)
		}
	}
}

// mockClient implements ClientInterface for testing
type mockClient struct {
	createInboxFn  func(ctx context.Context, opts ...vaultsandbox.InboxOption) (*vaultsandbox.Inbox, error)
	importInboxFn  func(ctx context.Context, data *vaultsandbox.ExportedInbox) (*vaultsandbox.Inbox, error)
	deleteInboxFn  func(ctx context.Context, emailAddress string) error
}

func (m *mockClient) CreateInbox(ctx context.Context, opts ...vaultsandbox.InboxOption) (*vaultsandbox.Inbox, error) {
	if m.createInboxFn != nil {
		return m.createInboxFn(ctx, opts...)
	}
	return nil, errors.New("not implemented")
}

func (m *mockClient) ImportInbox(ctx context.Context, data *vaultsandbox.ExportedInbox) (*vaultsandbox.Inbox, error) {
	if m.importInboxFn != nil {
		return m.importInboxFn(ctx, data)
	}
	return nil, errors.New("not implemented")
}

func (m *mockClient) DeleteInbox(ctx context.Context, emailAddress string) error {
	if m.deleteInboxFn != nil {
		return m.deleteInboxFn(ctx, emailAddress)
	}
	return errors.New("not implemented")
}

func TestRunCreateInbox_Error(t *testing.T) {
	client := &mockClient{
		createInboxFn: func(ctx context.Context, opts ...vaultsandbox.InboxOption) (*vaultsandbox.Inbox, error) {
			return nil, errors.New("create failed")
		},
	}

	cfg := &Config{
		Stdout: &bytes.Buffer{},
	}

	err := runCreateInbox(context.Background(), client, cfg)
	if err == nil {
		t.Error("runCreateInbox should return error when CreateInbox fails")
	}
	if !strings.Contains(err.Error(), "create inbox") {
		t.Errorf("error should contain 'create inbox', got %v", err)
	}
}

func TestRunImportInbox_ReadError(t *testing.T) {
	client := &mockClient{}
	cfg := &Config{
		Stdin:  &errorReader{},
		Stdout: &bytes.Buffer{},
	}

	err := runImportInbox(context.Background(), client, cfg)
	if err == nil {
		t.Error("runImportInbox should return error when reading stdin fails")
	}
	if !strings.Contains(err.Error(), "read stdin") {
		t.Errorf("error should contain 'read stdin', got %v", err)
	}
}

func TestRunImportInbox_InvalidJSON(t *testing.T) {
	client := &mockClient{}
	cfg := &Config{
		Stdin:  strings.NewReader("not valid json"),
		Stdout: &bytes.Buffer{},
	}

	err := runImportInbox(context.Background(), client, cfg)
	if err == nil {
		t.Error("runImportInbox should return error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse export") {
		t.Errorf("error should contain 'parse export', got %v", err)
	}
}

func TestRunImportInbox_ImportError(t *testing.T) {
	client := &mockClient{
		importInboxFn: func(ctx context.Context, data *vaultsandbox.ExportedInbox) (*vaultsandbox.Inbox, error) {
			return nil, errors.New("import failed")
		},
	}

	exportData := vaultsandbox.ExportedInbox{
		Version:      1,
		EmailAddress: "test@example.com",
		InboxHash:    "hash",
		SecretKey:    mockSecretKey,
		ServerSigPk:  mockServerSigPk,
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	inputJSON, _ := json.Marshal(exportData)

	cfg := &Config{
		Stdin:  bytes.NewReader(inputJSON),
		Stdout: &bytes.Buffer{},
	}

	err := runImportInbox(context.Background(), client, cfg)
	if err == nil {
		t.Error("runImportInbox should return error when ImportInbox fails")
	}
	if !strings.Contains(err.Error(), "import inbox") {
		t.Errorf("error should contain 'import inbox', got %v", err)
	}
}

func TestRunReadEmails_ReadError(t *testing.T) {
	client := &mockClient{}
	cfg := &Config{
		Stdin:  &errorReader{},
		Stdout: &bytes.Buffer{},
	}

	err := runReadEmails(context.Background(), client, cfg)
	if err == nil {
		t.Error("runReadEmails should return error when reading stdin fails")
	}
	if !strings.Contains(err.Error(), "read stdin") {
		t.Errorf("error should contain 'read stdin', got %v", err)
	}
}

func TestRunReadEmails_InvalidJSON(t *testing.T) {
	client := &mockClient{}
	cfg := &Config{
		Stdin:  strings.NewReader("invalid json"),
		Stdout: &bytes.Buffer{},
	}

	err := runReadEmails(context.Background(), client, cfg)
	if err == nil {
		t.Error("runReadEmails should return error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse export") {
		t.Errorf("error should contain 'parse export', got %v", err)
	}
}

func TestRunReadEmails_ImportError(t *testing.T) {
	client := &mockClient{
		importInboxFn: func(ctx context.Context, data *vaultsandbox.ExportedInbox) (*vaultsandbox.Inbox, error) {
			return nil, errors.New("import failed")
		},
	}

	exportData := vaultsandbox.ExportedInbox{
		Version:      1,
		EmailAddress: "test@example.com",
		InboxHash:    "hash",
		SecretKey:    mockSecretKey,
		ServerSigPk:  mockServerSigPk,
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	inputJSON, _ := json.Marshal(exportData)

	cfg := &Config{
		Stdin:  bytes.NewReader(inputJSON),
		Stdout: &bytes.Buffer{},
	}

	err := runReadEmails(context.Background(), client, cfg)
	if err == nil {
		t.Error("runReadEmails should return error when ImportInbox fails")
	}
	if !strings.Contains(err.Error(), "import inbox") {
		t.Errorf("error should contain 'import inbox', got %v", err)
	}
}

func TestRunCleanup_Success(t *testing.T) {
	var deletedAddress string
	client := &mockClient{
		deleteInboxFn: func(ctx context.Context, emailAddress string) error {
			deletedAddress = emailAddress
			return nil
		},
	}

	var stdout bytes.Buffer
	cfg := &Config{
		Stdout: &stdout,
	}

	err := runCleanup(context.Background(), client, cfg, "test@example.com")
	if err != nil {
		t.Fatalf("runCleanup error = %v", err)
	}

	if deletedAddress != "test@example.com" {
		t.Errorf("deleted address = %q, want %q", deletedAddress, "test@example.com")
	}

	var result map[string]bool
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if !result["success"] {
		t.Error("output should contain success: true")
	}
}

func TestRunCleanup_Error(t *testing.T) {
	client := &mockClient{
		deleteInboxFn: func(ctx context.Context, emailAddress string) error {
			return errors.New("delete failed")
		},
	}

	cfg := &Config{
		Stdout: &bytes.Buffer{},
	}

	err := runCleanup(context.Background(), client, cfg, "test@example.com")
	if err == nil {
		t.Error("runCleanup should return error when DeleteInbox fails")
	}
	if !strings.Contains(err.Error(), "delete inbox") {
		t.Errorf("error should contain 'delete inbox', got %v", err)
	}
}

// errorReader is an io.Reader that always returns an error
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

// errorWriter is an io.Writer that always returns an error
type errorWriter struct{}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("write error")
}

func TestRunImportInbox_Success(t *testing.T) {
	// Use httptest mock server to create a real client and inbox
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

	// Create a real client
	client, err := vaultsandbox.New("test-api-key", vaultsandbox.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	// Create valid export data
	exportData := vaultsandbox.ExportedInbox{
		Version:      1,
		EmailAddress: "test@test.com",
		InboxHash:    "test-inbox-hash",
		SecretKey:    mockSecretKey,
		ServerSigPk:  mockServerSigPk,
		ExpiresAt:    time.Now().Add(time.Hour),
		ExportedAt:   time.Now(),
	}
	inputJSON, _ := json.Marshal(exportData)

	var stdout bytes.Buffer
	cfg := &Config{
		Stdin:  bytes.NewReader(inputJSON),
		Stdout: &stdout,
	}

	err = runImportInbox(context.Background(), client, cfg)
	if err != nil {
		t.Fatalf("runImportInbox error = %v", err)
	}

	var result map[string]bool
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if !result["success"] {
		t.Error("output should contain success: true")
	}
}

func TestRunCreateInbox_Success(t *testing.T) {
	// Use httptest mock server
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
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailAddress": "newbox@test.com",
				"expiresAt":    time.Now().Add(time.Hour).Format(time.RFC3339),
				"inboxHash":    "new-inbox-hash",
				"serverSigPk":  mockServerSigPk,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := vaultsandbox.New("test-api-key", vaultsandbox.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	var stdout bytes.Buffer
	cfg := &Config{
		Stdout: &stdout,
	}

	err = runCreateInbox(context.Background(), client, cfg)
	if err != nil {
		t.Fatalf("runCreateInbox error = %v", err)
	}

	// Verify output is valid JSON with expected fields
	var result vaultsandbox.ExportedInbox
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if result.EmailAddress != "newbox@test.com" {
		t.Errorf("EmailAddress = %q, want %q", result.EmailAddress, "newbox@test.com")
	}
	if result.InboxHash != "new-inbox-hash" {
		t.Errorf("InboxHash = %q, want %q", result.InboxHash, "new-inbox-hash")
	}
}

func TestRunReadEmails_Success(t *testing.T) {
	// Use httptest mock server
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

		case strings.HasSuffix(r.URL.Path, "/sync"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU",
				"emailCount": 0,
			})

		case strings.HasSuffix(r.URL.Path, "/emails"):
			// Return empty email list
			json.NewEncoder(w).Encode([]interface{}{})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := vaultsandbox.New("test-api-key", vaultsandbox.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	exportData := vaultsandbox.ExportedInbox{
		Version:      1,
		EmailAddress: "test@test.com",
		InboxHash:    "test-inbox-hash",
		SecretKey:    mockSecretKey,
		ServerSigPk:  mockServerSigPk,
		ExpiresAt:    time.Now().Add(time.Hour),
		ExportedAt:   time.Now(),
	}
	inputJSON, _ := json.Marshal(exportData)

	var stdout bytes.Buffer
	cfg := &Config{
		Stdin:  bytes.NewReader(inputJSON),
		Stdout: &stdout,
	}

	err = runReadEmails(context.Background(), client, cfg)
	if err != nil {
		t.Fatalf("runReadEmails error = %v", err)
	}

	// Verify output structure
	var result struct {
		Emails []EmailOutput `json:"emails"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse output: %v", err)
	}
	if result.Emails == nil {
		t.Error("Emails should not be nil")
	}
}

func TestRunCreateInbox_EncodeError(t *testing.T) {
	// Use httptest mock server
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
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailAddress": "newbox@test.com",
				"expiresAt":    time.Now().Add(time.Hour).Format(time.RFC3339),
				"inboxHash":    "new-inbox-hash",
				"serverSigPk":  mockServerSigPk,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := vaultsandbox.New("test-api-key", vaultsandbox.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	cfg := &Config{
		Stdout: &errorWriter{},
	}

	err = runCreateInbox(context.Background(), client, cfg)
	if err == nil {
		t.Error("runCreateInbox should return error when encoding fails")
	}
	if !strings.Contains(err.Error(), "encode export") {
		t.Errorf("error should contain 'encode export', got %v", err)
	}
}

func TestRunReadEmails_EncodeError(t *testing.T) {
	// Use httptest mock server
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

		case strings.HasSuffix(r.URL.Path, "/sync"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU",
				"emailCount": 0,
			})

		case strings.HasSuffix(r.URL.Path, "/emails"):
			json.NewEncoder(w).Encode([]interface{}{})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := vaultsandbox.New("test-api-key", vaultsandbox.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	exportData := vaultsandbox.ExportedInbox{
		Version:      1,
		EmailAddress: "test@test.com",
		InboxHash:    "test-inbox-hash",
		SecretKey:    mockSecretKey,
		ServerSigPk:  mockServerSigPk,
		ExpiresAt:    time.Now().Add(time.Hour),
		ExportedAt:   time.Now(),
	}
	inputJSON, _ := json.Marshal(exportData)

	cfg := &Config{
		Stdin:  bytes.NewReader(inputJSON),
		Stdout: &errorWriter{},
	}

	err = runReadEmails(context.Background(), client, cfg)
	if err == nil {
		t.Error("runReadEmails should return error when encoding fails")
	}
	if !strings.Contains(err.Error(), "encode output") {
		t.Errorf("error should contain 'encode output', got %v", err)
	}
}

func TestRunReadEmails_GetEmailsError(t *testing.T) {
	// Use httptest mock server that returns error for GetEmails
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

		case strings.HasSuffix(r.URL.Path, "/sync"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailsHash": "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU",
				"emailCount": 0,
			})

		case strings.HasSuffix(r.URL.Path, "/emails"):
			// Return server error
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "server error"})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := vaultsandbox.New("test-api-key", vaultsandbox.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	exportData := vaultsandbox.ExportedInbox{
		Version:      1,
		EmailAddress: "test@test.com",
		InboxHash:    "test-inbox-hash",
		SecretKey:    mockSecretKey,
		ServerSigPk:  mockServerSigPk,
		ExpiresAt:    time.Now().Add(time.Hour),
		ExportedAt:   time.Now(),
	}
	inputJSON, _ := json.Marshal(exportData)

	var stdout bytes.Buffer
	cfg := &Config{
		Stdin:  bytes.NewReader(inputJSON),
		Stdout: &stdout,
	}

	err = runReadEmails(context.Background(), client, cfg)
	if err == nil {
		t.Error("runReadEmails should return error when GetEmails fails")
	}
	if !strings.Contains(err.Error(), "list emails") {
		t.Errorf("error should contain 'list emails', got %v", err)
	}
}

func TestConfig_CustomIO(t *testing.T) {
	var stdin bytes.Buffer
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cfg := &Config{
		Stdin:  &stdin,
		Stdout: &stdout,
		Stderr: &stderr,
	}

	// Verify the buffers are assigned correctly
	if cfg.Stdin != &stdin {
		t.Error("Stdin should be the custom buffer")
	}
	if cfg.Stdout != &stdout {
		t.Error("Stdout should be the custom buffer")
	}
	if cfg.Stderr != &stderr {
		t.Error("Stderr should be the custom buffer")
	}

	// Write to stdout and verify
	cfg.Stdout.Write([]byte("test output"))
	if stdout.String() != "test output" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "test output")
	}
}

func TestClientInterface_Implemented(t *testing.T) {
	// Verify that vaultsandbox.Client implements ClientInterface
	var _ ClientInterface = (*vaultsandbox.Client)(nil)
}

func TestConvertEmails_Empty(t *testing.T) {
	result := convertEmails(nil)
	if len(result) != 0 {
		t.Errorf("convertEmails(nil) len = %d, want 0", len(result))
	}

	result = convertEmails([]*vaultsandbox.Email{})
	if len(result) != 0 {
		t.Errorf("convertEmails([]) len = %d, want 0", len(result))
	}
}

func TestConvertEmails_SingleEmail(t *testing.T) {
	receivedAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	emails := []*vaultsandbox.Email{
		{
			ID:         "email-1",
			Subject:    "Test Subject",
			From:       "sender@example.com",
			To:         []string{"recipient@example.com"},
			Text:       "Hello, World!",
			HTML:       "<p>Hello, World!</p>",
			ReceivedAt: receivedAt,
		},
	}

	result := convertEmails(emails)

	if len(result) != 1 {
		t.Fatalf("convertEmails len = %d, want 1", len(result))
	}

	e := result[0]
	if e.ID != "email-1" {
		t.Errorf("ID = %q, want %q", e.ID, "email-1")
	}
	if e.Subject != "Test Subject" {
		t.Errorf("Subject = %q, want %q", e.Subject, "Test Subject")
	}
	if e.From != "sender@example.com" {
		t.Errorf("From = %q, want %q", e.From, "sender@example.com")
	}
	if len(e.To) != 1 || e.To[0] != "recipient@example.com" {
		t.Errorf("To = %v, want [recipient@example.com]", e.To)
	}
	if e.Text != "Hello, World!" {
		t.Errorf("Text = %q, want %q", e.Text, "Hello, World!")
	}
	if e.HTML != "<p>Hello, World!</p>" {
		t.Errorf("HTML = %q, want %q", e.HTML, "<p>Hello, World!</p>")
	}
	if e.ReceivedAt != "2024-01-15T10:30:00Z" {
		t.Errorf("ReceivedAt = %q, want %q", e.ReceivedAt, "2024-01-15T10:30:00Z")
	}
}

func TestConvertEmails_WithAttachments(t *testing.T) {
	receivedAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	emails := []*vaultsandbox.Email{
		{
			ID:         "email-1",
			Subject:    "Email with attachments",
			From:       "sender@example.com",
			To:         []string{"recipient@example.com"},
			Text:       "See attached",
			ReceivedAt: receivedAt,
			Attachments: []vaultsandbox.Attachment{
				{
					Filename:    "document.pdf",
					ContentType: "application/pdf",
					Content:     []byte("pdf content here"),
				},
				{
					Filename:    "image.png",
					ContentType: "image/png",
					Content:     []byte("png data"),
				},
			},
		},
	}

	result := convertEmails(emails)

	if len(result) != 1 {
		t.Fatalf("convertEmails len = %d, want 1", len(result))
	}

	e := result[0]
	if len(e.Attachments) != 2 {
		t.Fatalf("Attachments len = %d, want 2", len(e.Attachments))
	}

	// First attachment
	att1 := e.Attachments[0]
	if att1.Filename != "document.pdf" {
		t.Errorf("Attachment[0].Filename = %q, want %q", att1.Filename, "document.pdf")
	}
	if att1.ContentType != "application/pdf" {
		t.Errorf("Attachment[0].ContentType = %q, want %q", att1.ContentType, "application/pdf")
	}
	if att1.Size != 16 { // len("pdf content here")
		t.Errorf("Attachment[0].Size = %d, want 16", att1.Size)
	}

	// Second attachment
	att2 := e.Attachments[1]
	if att2.Filename != "image.png" {
		t.Errorf("Attachment[1].Filename = %q, want %q", att2.Filename, "image.png")
	}
	if att2.ContentType != "image/png" {
		t.Errorf("Attachment[1].ContentType = %q, want %q", att2.ContentType, "image/png")
	}
	if att2.Size != 8 { // len("png data")
		t.Errorf("Attachment[1].Size = %d, want 8", att2.Size)
	}
}

func TestConvertEmails_MultipleEmails(t *testing.T) {
	receivedAt1 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	receivedAt2 := time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC)
	receivedAt3 := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	emails := []*vaultsandbox.Email{
		{
			ID:         "email-1",
			Subject:    "First",
			From:       "a@example.com",
			To:         []string{"recipient@example.com"},
			Text:       "First email",
			ReceivedAt: receivedAt1,
		},
		{
			ID:         "email-2",
			Subject:    "Second",
			From:       "b@example.com",
			To:         []string{"recipient@example.com", "cc@example.com"},
			Text:       "Second email",
			ReceivedAt: receivedAt2,
		},
		{
			ID:         "email-3",
			Subject:    "Third",
			From:       "c@example.com",
			To:         []string{"recipient@example.com"},
			Text:       "Third email",
			HTML:       "<p>Third</p>",
			ReceivedAt: receivedAt3,
			Attachments: []vaultsandbox.Attachment{
				{Filename: "file.txt", ContentType: "text/plain", Content: []byte("hi")},
			},
		},
	}

	result := convertEmails(emails)

	if len(result) != 3 {
		t.Fatalf("convertEmails len = %d, want 3", len(result))
	}

	// Verify order is preserved
	if result[0].ID != "email-1" {
		t.Errorf("result[0].ID = %q, want %q", result[0].ID, "email-1")
	}
	if result[1].ID != "email-2" {
		t.Errorf("result[1].ID = %q, want %q", result[1].ID, "email-2")
	}
	if result[2].ID != "email-3" {
		t.Errorf("result[2].ID = %q, want %q", result[2].ID, "email-3")
	}

	// Second email has two recipients
	if len(result[1].To) != 2 {
		t.Errorf("result[1].To len = %d, want 2", len(result[1].To))
	}

	// Third email has attachment
	if len(result[2].Attachments) != 1 {
		t.Errorf("result[2].Attachments len = %d, want 1", len(result[2].Attachments))
	}
}

func TestConvertEmails_EmptyAttachment(t *testing.T) {
	receivedAt := time.Now()
	emails := []*vaultsandbox.Email{
		{
			ID:         "email-1",
			Subject:    "Empty attachment",
			From:       "sender@example.com",
			To:         []string{"recipient@example.com"},
			Text:       "Text",
			ReceivedAt: receivedAt,
			Attachments: []vaultsandbox.Attachment{
				{
					Filename:    "empty.txt",
					ContentType: "text/plain",
					Content:     []byte{}, // Empty content
				},
			},
		},
	}

	result := convertEmails(emails)

	if len(result[0].Attachments) != 1 {
		t.Fatalf("Attachments len = %d, want 1", len(result[0].Attachments))
	}

	att := result[0].Attachments[0]
	if att.Size != 0 {
		t.Errorf("Size = %d, want 0", att.Size)
	}
}

func TestFatal(t *testing.T) {
	// Save original exit function and restore after test
	originalExitFunc := exitFunc
	defer func() { exitFunc = originalExitFunc }()

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	fatal("test error: %s", "details")

	// Restore stderr and read output
	w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if exitCode != 1 {
		t.Errorf("exitCode = %d, want 1", exitCode)
	}

	if !strings.Contains(output, "test error: details") {
		t.Errorf("output = %q, should contain 'test error: details'", output)
	}

	if !strings.HasSuffix(output, "\n") {
		t.Error("output should end with newline")
	}
}

func TestFatal_FormatsCorrectly(t *testing.T) {
	originalExitFunc := exitFunc
	defer func() { exitFunc = originalExitFunc }()

	exitFunc = func(code int) {} // No-op

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	fatal("error %d: %s", 42, "something went wrong")

	w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	expected := "error 42: something went wrong\n"
	if output != expected {
		t.Errorf("output = %q, want %q", output, expected)
	}
}

func TestRun_NoArgs(t *testing.T) {
	cfg := &Config{Stdout: &bytes.Buffer{}}
	err := run([]string{"testhelper"}, cfg)
	if err == nil {
		t.Error("run() should return error with no args")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("error should contain 'usage', got %v", err)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	// Mock the client factory
	originalFactory := clientFactory
	defer func() { clientFactory = originalFactory }()

	clientFactory = func() (ClientInterface, error) {
		return &mockClient{}, nil
	}

	cfg := &Config{Stdout: &bytes.Buffer{}}
	err := run([]string{"testhelper", "unknown-command"}, cfg)
	if err == nil {
		t.Error("run() should return error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("error should contain 'unknown command', got %v", err)
	}
}

func TestRun_ClientFactoryError(t *testing.T) {
	originalFactory := clientFactory
	defer func() { clientFactory = originalFactory }()

	clientFactory = func() (ClientInterface, error) {
		return nil, errors.New("factory error")
	}

	cfg := &Config{Stdout: &bytes.Buffer{}}
	err := run([]string{"testhelper", "create-inbox"}, cfg)
	if err == nil {
		t.Error("run() should return error when client factory fails")
	}
	if !strings.Contains(err.Error(), "create client") {
		t.Errorf("error should contain 'create client', got %v", err)
	}
}

func TestRun_CreateInbox(t *testing.T) {
	originalFactory := clientFactory
	defer func() { clientFactory = originalFactory }()

	var createCalled bool
	clientFactory = func() (ClientInterface, error) {
		return &mockClient{
			createInboxFn: func(ctx context.Context, opts ...vaultsandbox.InboxOption) (*vaultsandbox.Inbox, error) {
				createCalled = true
				return nil, errors.New("test error")
			},
		}, nil
	}

	cfg := &Config{Stdout: &bytes.Buffer{}}
	err := run([]string{"testhelper", "create-inbox"}, cfg)

	if !createCalled {
		t.Error("create-inbox command should call CreateInbox")
	}
	if err == nil {
		t.Error("run() should return error from runCreateInbox")
	}
}

func TestRun_ImportInbox(t *testing.T) {
	originalFactory := clientFactory
	defer func() { clientFactory = originalFactory }()

	var importCalled bool
	clientFactory = func() (ClientInterface, error) {
		return &mockClient{
			importInboxFn: func(ctx context.Context, data *vaultsandbox.ExportedInbox) (*vaultsandbox.Inbox, error) {
				importCalled = true
				return nil, errors.New("test error")
			},
		}, nil
	}

	exportData := vaultsandbox.ExportedInbox{
		Version:      1,
		EmailAddress: "test@example.com",
		InboxHash:    "hash",
		SecretKey:    mockSecretKey,
		ServerSigPk:  mockServerSigPk,
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	inputJSON, _ := json.Marshal(exportData)

	cfg := &Config{
		Stdin:  bytes.NewReader(inputJSON),
		Stdout: &bytes.Buffer{},
	}
	err := run([]string{"testhelper", "import-inbox"}, cfg)

	if !importCalled {
		t.Error("import-inbox command should call ImportInbox")
	}
	if err == nil {
		t.Error("run() should return error from runImportInbox")
	}
}

func TestRun_ReadEmails(t *testing.T) {
	originalFactory := clientFactory
	defer func() { clientFactory = originalFactory }()

	var importCalled bool
	clientFactory = func() (ClientInterface, error) {
		return &mockClient{
			importInboxFn: func(ctx context.Context, data *vaultsandbox.ExportedInbox) (*vaultsandbox.Inbox, error) {
				importCalled = true
				return nil, errors.New("test error")
			},
		}, nil
	}

	exportData := vaultsandbox.ExportedInbox{
		Version:      1,
		EmailAddress: "test@example.com",
		InboxHash:    "hash",
		SecretKey:    mockSecretKey,
		ServerSigPk:  mockServerSigPk,
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	inputJSON, _ := json.Marshal(exportData)

	cfg := &Config{
		Stdin:  bytes.NewReader(inputJSON),
		Stdout: &bytes.Buffer{},
	}
	err := run([]string{"testhelper", "read-emails"}, cfg)

	if !importCalled {
		t.Error("read-emails command should call ImportInbox")
	}
	if err == nil {
		t.Error("run() should return error from runReadEmails")
	}
}

func TestRun_Cleanup_NoAddress(t *testing.T) {
	originalFactory := clientFactory
	defer func() { clientFactory = originalFactory }()

	clientFactory = func() (ClientInterface, error) {
		return &mockClient{}, nil
	}

	cfg := &Config{Stdout: &bytes.Buffer{}}
	err := run([]string{"testhelper", "cleanup"}, cfg)

	if err == nil {
		t.Error("run() should return error when cleanup has no address")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("error should contain 'usage', got %v", err)
	}
}

func TestRun_Cleanup_Success(t *testing.T) {
	originalFactory := clientFactory
	defer func() { clientFactory = originalFactory }()

	var deletedAddress string
	clientFactory = func() (ClientInterface, error) {
		return &mockClient{
			deleteInboxFn: func(ctx context.Context, emailAddress string) error {
				deletedAddress = emailAddress
				return nil
			},
		}, nil
	}

	var stdout bytes.Buffer
	cfg := &Config{Stdout: &stdout}
	err := run([]string{"testhelper", "cleanup", "test@example.com"}, cfg)

	if err != nil {
		t.Errorf("run() error = %v", err)
	}
	if deletedAddress != "test@example.com" {
		t.Errorf("deleted address = %q, want %q", deletedAddress, "test@example.com")
	}
}

func TestDefaultClientFactory_EmptyAPIKey(t *testing.T) {
	// Test the default clientFactory with an empty API key.
	// This exercises the actual clientFactory code path.
	t.Setenv("VAULTSANDBOX_API_KEY", "")
	t.Setenv("VAULTSANDBOX_URL", "")

	_, err := clientFactory()
	if err == nil {
		t.Error("clientFactory should return error with empty API key")
	}
	if !errors.Is(err, vaultsandbox.ErrMissingAPIKey) {
		t.Errorf("error = %v, want ErrMissingAPIKey", err)
	}
}
