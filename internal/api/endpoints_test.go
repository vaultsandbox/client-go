package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go/internal/crypto"
)

// failingReader is an io.Reader that always returns an error.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("simulated random read failure")
}

func TestCheckKey_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "server error"})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL), WithRetries(0))
	err := client.CheckKey(context.Background())
	if err == nil {
		t.Fatal("CheckKey() should return error for 500 response")
	}
}

func TestGetServerInfo_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "server error"})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL), WithRetries(0))
	_, err := client.GetServerInfo(context.Background())
	if err == nil {
		t.Fatal("GetServerInfo() should return error for 500 response")
	}
}

func TestGetInboxSync_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "inbox not found"})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL), WithRetries(0))
	_, err := client.GetInboxSync(context.Background(), "test@example.com")
	if err == nil {
		t.Fatal("GetInboxSync() should return error for 404 response")
	}
}

func TestOpenEventStream_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.Contains(r.URL.String(), "/api/events") {
			t.Errorf("path = %s, want /api/events", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("X-API-Key = %s, want test-key", r.Header.Get("X-API-Key"))
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("Accept = %s, want text/event-stream", r.Header.Get("Accept"))
		}
		if r.Header.Get("Cache-Control") != "no-cache" {
			t.Errorf("Cache-Control = %s, want no-cache", r.Header.Get("Cache-Control"))
		}

		// Verify query parameter
		inboxes := r.URL.Query().Get("inboxes")
		if inboxes != "hash1,hash2" {
			t.Errorf("inboxes query = %s, want hash1,hash2", inboxes)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: test\n\n"))
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	resp, err := client.OpenEventStream(context.Background(), []string{"hash1", "hash2"})
	if err != nil {
		t.Fatalf("OpenEventStream() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestOpenEventStream_Error(t *testing.T) {
	t.Parallel()
	// Use invalid URL to trigger error
	client, _ := New("test-key", WithBaseURL("http://invalid.invalid.invalid:99999"))
	_, err := client.OpenEventStream(context.Background(), []string{"hash1"})
	if err == nil {
		t.Fatal("OpenEventStream() should return error for invalid URL")
	}
}

func TestOpenEventStream_RequestCreationError(t *testing.T) {
	t.Parallel()
	// Use a URL with invalid characters that will cause NewRequestWithContext to fail
	// A URL containing a space character without encoding will cause url.Parse to fail
	client, _ := New("test-key", WithBaseURL("http://example .com"))
	_, err := client.OpenEventStream(context.Background(), []string{"hash1"})
	if err == nil {
		t.Fatal("OpenEventStream() should return error for malformed URL")
	}
}

func TestCreateInbox_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/inboxes" {
			t.Errorf("path = %s, want /api/inboxes", r.URL.Path)
		}

		// Verify request body contains expected fields
		var reqBody createInboxAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if reqBody.ClientKemPk == "" {
			t.Error("clientKemPk should not be empty")
		}
		if reqBody.TTL != 3600 {
			t.Errorf("ttl = %d, want 3600", reqBody.TTL)
		}
		if reqBody.EmailAddress != "custom@example.com" {
			t.Errorf("emailAddress = %s, want custom@example.com", reqBody.EmailAddress)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(createInboxAPIResponse{
			EmailAddress: "custom@example.com",
			ExpiresAt:    time.Now().Add(time.Hour),
			InboxHash:    "inbox123",
			ServerSigPk:  "c2VydmVyc2lncGs=", // base64 of "serversigpk"
			Encrypted:    true,                // Server indicates this is an encrypted inbox
		})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	result, err := client.CreateInbox(context.Background(), &CreateInboxParams{
		TTL:          time.Hour,
		EmailAddress: "custom@example.com",
	})
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}

	if result.EmailAddress != "custom@example.com" {
		t.Errorf("EmailAddress = %s, want custom@example.com", result.EmailAddress)
	}
	if result.InboxHash != "inbox123" {
		t.Errorf("InboxHash = %s, want inbox123", result.InboxHash)
	}
	if !result.Encrypted {
		t.Error("Encrypted should be true")
	}
	if result.Keypair == nil {
		t.Error("Keypair should not be nil for encrypted inbox")
	}
	if len(result.ServerSigPk) == 0 {
		t.Error("ServerSigPk should not be empty for encrypted inbox")
	}
}

func TestCreateInbox_APIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL), WithRetries(0))
	_, err := client.CreateInbox(context.Background(), &CreateInboxParams{
		TTL: time.Hour,
	})
	if err == nil {
		t.Fatal("CreateInbox() should return error for 400 response")
	}
}

func TestCreateInbox_InvalidServerSigPk(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(createInboxAPIResponse{
			EmailAddress: "test@example.com",
			ExpiresAt:    time.Now().Add(time.Hour),
			InboxHash:    "inbox123",
			ServerSigPk:  "!!!invalid-base64!!!", // Invalid base64
			Encrypted:    true,                   // Must be true to trigger ServerSigPk validation
		})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	_, err := client.CreateInbox(context.Background(), &CreateInboxParams{
		TTL: time.Hour,
	})
	if err == nil {
		t.Fatal("CreateInbox() should return error for invalid serverSigPk")
	}
	if !strings.Contains(err.Error(), "failed to decode server signature public key") {
		t.Errorf("error message = %v, want to contain 'failed to decode server signature public key'", err)
	}
}

func TestCreateInbox_KeypairGenerationError(t *testing.T) {
	// This test modifies global state (randReader) so it cannot run in parallel
	// Force keypair generation to fail by using a failing random reader
	restore := crypto.SetRandReaderForTesting(failingReader{})
	defer restore()

	client, _ := New("test-key", WithBaseURL("https://example.com"))
	_, err := client.CreateInbox(context.Background(), &CreateInboxParams{
		TTL: time.Hour,
	})
	if err == nil {
		t.Fatal("CreateInbox() should return error when keypair generation fails")
	}
	if !strings.Contains(err.Error(), "failed to generate keypair") {
		t.Errorf("error message = %v, want to contain 'failed to generate keypair'", err)
	}
}

func TestGetEmails_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/api/inboxes/") {
			t.Errorf("path = %s, should start with /api/inboxes/", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*RawEmail{
			{ID: "email1", InboxID: "inbox1", IsRead: false},
			{ID: "email2", InboxID: "inbox1", IsRead: true},
		})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	result, err := client.GetEmails(context.Background(), "test@example.com", false)
	if err != nil {
		t.Fatalf("GetEmails() error = %v", err)
	}

	if len(result.Emails) != 2 {
		t.Errorf("len(Emails) = %d, want 2", len(result.Emails))
	}
	if result.Emails[0].ID != "email1" {
		t.Errorf("Emails[0].ID = %s, want email1", result.Emails[0].ID)
	}
}

func TestGetEmails_WithIncludeContent(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("includeContent") != "true" {
			t.Errorf("includeContent query = %s, want true", r.URL.Query().Get("includeContent"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]*RawEmail{})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	_, err := client.GetEmails(context.Background(), "test@example.com", true)
	if err != nil {
		t.Fatalf("GetEmails() error = %v", err)
	}
}

func TestGetEmails_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "inbox not found"})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL), WithRetries(0))
	_, err := client.GetEmails(context.Background(), "test@example.com", false)
	if err == nil {
		t.Fatal("GetEmails() should return error for 404 response")
	}
}

func TestGetEmail_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RawEmail{
			ID:      "email123",
			InboxID: "inbox1",
			IsRead:  false,
		})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	email, err := client.GetEmail(context.Background(), "test@example.com", "email123")
	if err != nil {
		t.Fatalf("GetEmail() error = %v", err)
	}

	if email.ID != "email123" {
		t.Errorf("ID = %s, want email123", email.ID)
	}
}

func TestGetEmail_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "email not found"})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL), WithRetries(0))
	_, err := client.GetEmail(context.Background(), "test@example.com", "email123")
	if err == nil {
		t.Fatal("GetEmail() should return error for 404 response")
	}
}

func TestGetEmailRaw_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/raw") {
			t.Errorf("path = %s, should end with /raw", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		// Return a plain format raw email source (Base64-encoded)
		json.NewEncoder(w).Encode(map[string]string{
			"id":  "email123",
			"raw": "RnJvbTogc2VuZGVyQGV4YW1wbGUuY29tDQpUbzogdGVzdEBleGFtcGxlLmNvbQ0KDQpIZWxsbyBXb3JsZA==", // Base64 encoded raw email
		})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	rawSource, err := client.GetEmailRaw(context.Background(), "test@example.com", "email123")
	if err != nil {
		t.Fatalf("GetEmailRaw() error = %v", err)
	}

	if rawSource.ID != "email123" {
		t.Errorf("rawSource.ID = %s, want email123", rawSource.ID)
	}

	// Plain format should have Raw field set
	if rawSource.Raw == "" {
		t.Error("rawSource.Raw should not be empty for plain format")
	}
}

func TestGetEmailRaw_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "email not found"})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL), WithRetries(0))
	_, err := client.GetEmailRaw(context.Background(), "test@example.com", "email123")
	if err == nil {
		t.Fatal("GetEmailRaw() should return error for 404 response")
	}
}

func TestMarkEmailAsRead_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/read") {
			t.Errorf("path = %s, should end with /read", r.URL.Path)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	err := client.MarkEmailAsRead(context.Background(), "test@example.com", "email123")
	if err != nil {
		t.Fatalf("MarkEmailAsRead() error = %v", err)
	}
}

func TestMarkEmailAsRead_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "email not found"})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL), WithRetries(0))
	err := client.MarkEmailAsRead(context.Background(), "test@example.com", "email123")
	if err == nil {
		t.Fatal("MarkEmailAsRead() should return error for 404 response")
	}
}

func TestDeleteEmail_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	err := client.DeleteEmail(context.Background(), "test@example.com", "email123")
	if err != nil {
		t.Fatalf("DeleteEmail() error = %v", err)
	}
}

func TestDeleteEmail_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "email not found"})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL), WithRetries(0))
	err := client.DeleteEmail(context.Background(), "test@example.com", "email123")
	if err == nil {
		t.Fatal("DeleteEmail() should return error for 404 response")
	}
}
