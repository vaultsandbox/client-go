package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go/internal/apierrors"
)

func TestNew_RequiresAPIKey(t *testing.T) {
	t.Parallel()
	_, err := New("", WithBaseURL("https://example.com"))
	if err == nil {
		t.Error("expected error for empty API key")
	}
}

func TestNew_RequiresBaseURL(t *testing.T) {
	t.Parallel()
	_, err := New("test-key") // No base URL option
	if err == nil {
		t.Error("expected error for missing base URL")
	}
}

func TestNew_DefaultValues(t *testing.T) {
	t.Parallel()
	client, err := New("test-key", WithBaseURL("https://example.com"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if client.httpClient == nil {
		t.Error("httpClient is nil")
	}
	if client.httpClient.Timeout != DefaultTimeout {
		t.Errorf("timeout = %v, want %v", client.httpClient.Timeout, DefaultTimeout)
	}
	if client.maxRetries != DefaultMaxRetries {
		t.Errorf("maxRetries = %d, want %d", client.maxRetries, DefaultMaxRetries)
	}
	if client.retryDelay != DefaultRetryDelay {
		t.Errorf("retryDelay = %v, want %v", client.retryDelay, DefaultRetryDelay)
	}
}

func TestNew_CustomValues(t *testing.T) {
	t.Parallel()
	customHTTPClient := &http.Client{Timeout: 60 * time.Second}

	client, err := New("custom-key",
		WithBaseURL("https://custom.example.com"),
		WithHTTPClient(customHTTPClient),
		WithRetries(5),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if client.httpClient != customHTTPClient {
		t.Error("httpClient not set correctly")
	}
	if client.maxRetries != 5 {
		t.Errorf("maxRetries = %d, want 5", client.maxRetries)
	}
}

func TestNew_WithOptions(t *testing.T) {
	t.Parallel()
	client, err := New("test-key",
		WithBaseURL("https://example.com"),
		WithRetries(5),
		WithTimeout(60*time.Second),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if client.baseURL != "https://example.com" {
		t.Errorf("baseURL = %s, want https://example.com", client.baseURL)
	}
	if client.maxRetries != 5 {
		t.Errorf("maxRetries = %d, want 5", client.maxRetries)
	}
	if client.httpClient.Timeout != 60*time.Second {
		t.Errorf("timeout = %v, want 60s", client.httpClient.Timeout)
	}
}

func TestClient_Do_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("X-API-Key = %s, want test-key", r.Header.Get("X-API-Key"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", r.Header.Get("Content-Type"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))

	var result struct{ OK bool }
	err := client.Do(context.Background(), "GET", "/test", nil, &result)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if !result.OK {
		t.Error("result.OK = false, want true")
	}
}

func TestClient_Do_WithBody(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Name string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		if body.Name != "test" {
			t.Errorf("body.Name = %s, want test", body.Name)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"received": body.Name})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))

	request := struct{ Name string }{Name: "test"}
	var result struct{ Received string }

	err := client.Do(context.Background(), "POST", "/test", request, &result)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if result.Received != "test" {
		t.Errorf("result.Received = %s, want test", result.Received)
	}
}

func TestClient_Do_NoContent(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))

	err := client.Do(context.Background(), "DELETE", "/test", nil, nil)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
}

func TestClient_Do_Retry(t *testing.T) {
	t.Parallel()
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer server.Close()

	client, _ := New("test-key",
		WithBaseURL(server.URL),
		WithRetries(3),
	)
	// Override retry delay for faster tests
	client.retryDelay = time.Millisecond

	var result struct{ OK bool }
	err := client.Do(context.Background(), "GET", "/test", nil, &result)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestClient_Do_NoRetryOn4xx(t *testing.T) {
	t.Parallel()
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "bad request"})
	}))
	defer server.Close()

	client, _ := New("test-key",
		WithBaseURL(server.URL),
		WithRetries(3),
	)
	client.retryDelay = time.Millisecond

	err := client.Do(context.Background(), "GET", "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on 4xx)", attempts)
	}
}

func TestClient_Do_ContextCancellation(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := client.Do(ctx, "GET", "/test", nil, nil)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestClient_Do_ErrorResponse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		statusCode int
		body       string
		checkError func(t *testing.T, err error)
	}{
		{
			name:       "unauthorized",
			statusCode: 401,
			body:       `{"error": "invalid API key"}`,
			checkError: func(t *testing.T, err error) {
				var apiErr *apierrors.APIError
				if !isAPIError(err, &apiErr) {
					t.Errorf("expected APIError, got %T", err)
					return
				}
				if apiErr.StatusCode != 401 {
					t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
				}
			},
		},
		{
			name:       "not found",
			statusCode: 404,
			body:       `{"error": "inbox not found"}`,
			checkError: func(t *testing.T, err error) {
				var apiErr *apierrors.APIError
				if !isAPIError(err, &apiErr) {
					t.Errorf("expected APIError, got %T", err)
					return
				}
				if apiErr.StatusCode != 404 {
					t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
				}
			},
		},
		{
			name:       "rate limited",
			statusCode: 429,
			body:       `{"error": "rate limit exceeded"}`,
			checkError: func(t *testing.T, err error) {
				var apiErr *apierrors.APIError
				if !isAPIError(err, &apiErr) {
					t.Errorf("expected APIError, got %T", err)
					return
				}
				if apiErr.StatusCode != 429 {
					t.Errorf("StatusCode = %d, want 429", apiErr.StatusCode)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client, _ := New("test-key",
				WithBaseURL(server.URL),
				WithRetries(0), // No retries for faster tests
			)

			err := client.Do(context.Background(), "GET", "/test", nil, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			tt.checkError(t, err)
		})
	}
}

func TestClient_BaseURL(t *testing.T) {
	t.Parallel()
	client, _ := New("test-key", WithBaseURL("https://example.com"))

	if client.BaseURL() != "https://example.com" {
		t.Errorf("BaseURL() = %s, want https://example.com", client.BaseURL())
	}
}

func TestClient_HTTPClient(t *testing.T) {
	t.Parallel()
	customHTTPClient := &http.Client{Timeout: 60 * time.Second}

	client, _ := New("test-key",
		WithBaseURL("https://example.com"),
		WithHTTPClient(customHTTPClient),
	)

	if client.HTTPClient() != customHTTPClient {
		t.Error("HTTPClient() did not return the custom client")
	}
}

func TestClient_SetHTTPClient(t *testing.T) {
	t.Parallel()
	client, _ := New("test-key", WithBaseURL("https://example.com"))

	newHTTPClient := &http.Client{Timeout: 120 * time.Second}
	client.SetHTTPClient(newHTTPClient)

	if client.HTTPClient() != newHTTPClient {
		t.Error("SetHTTPClient() did not update the client")
	}
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()
	// Create a client with default retryOn status codes
	client, _ := New("test-key", WithBaseURL("https://example.com"))

	tests := []struct {
		statusCode int
		expected   bool
	}{
		{200, false},
		{201, false},
		{204, false},
		{400, false},
		{401, false},
		{403, false},
		{404, false},
		{408, true},  // Request Timeout
		{429, true},  // Too Many Requests
		{500, true},  // Internal Server Error
		{502, true},  // Bad Gateway
		{503, true},  // Service Unavailable
		{504, true},  // Gateway Timeout
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.statusCode), func(t *testing.T) {
			result := client.isRetryable(tt.statusCode)
			if result != tt.expected {
				t.Errorf("isRetryable(%d) = %v, want %v", tt.statusCode, result, tt.expected)
			}
		})
	}
}

func TestIsRetryable_CustomStatusCodes(t *testing.T) {
	t.Parallel()
	// Create a client with custom retryOn status codes
	client, _ := New("test-key",
		WithBaseURL("https://example.com"),
		WithRetryOn([]int{502, 503}), // Only retry on these
	)

	tests := []struct {
		statusCode int
		expected   bool
	}{
		{429, false}, // Not in custom list
		{500, false}, // Not in custom list
		{502, true},  // In custom list
		{503, true},  // In custom list
		{504, false}, // Not in custom list
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.statusCode), func(t *testing.T) {
			result := client.isRetryable(tt.statusCode)
			if result != tt.expected {
				t.Errorf("isRetryable(%d) = %v, want %v", tt.statusCode, result, tt.expected)
			}
		})
	}
}

func TestWithHTTPClient(t *testing.T) {
	t.Parallel()
	customClient := &http.Client{Timeout: 99 * time.Second}

	client, err := New("test-key",
		WithBaseURL("https://example.com"),
		WithHTTPClient(customClient),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if client.httpClient != customClient {
		t.Error("WithHTTPClient did not set the custom client")
	}
}

// Helper function to check if error is APIError
func isAPIError(err error, target **apierrors.APIError) bool {
	apiErr, ok := err.(*apierrors.APIError)
	if ok {
		*target = apiErr
		return true
	}
	return false
}

// ExampleNew demonstrates creating an API client with functional options.
func ExampleNew() {
	// Create a client using the functional options pattern.
	client, err := New("your-api-key",
		WithBaseURL("https://api.vaultsandbox.com"),
		WithRetries(5),
		WithTimeout(60*time.Second),
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Client created for: %s\n", client.BaseURL())
	// Output: Client created for: https://api.vaultsandbox.com
}

func TestCheckKey_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/check-key" {
			t.Errorf("path = %s, want /api/check-key", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	err := client.CheckKey(context.Background())
	if err != nil {
		t.Fatalf("CheckKey() error = %v", err)
	}
}

func TestCheckKey_NotOK(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": false})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	err := client.CheckKey(context.Background())
	if err == nil {
		t.Fatal("CheckKey() should return error when ok=false")
	}
}

func TestGetServerInfo_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/server-info" {
			t.Errorf("path = %s, want /api/server-info", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"allowedDomains": []string{"example.com", "test.com"},
			"maxTTL":         604800,
			"defaultTTL":     3600,
		})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	info, err := client.GetServerInfo(context.Background())
	if err != nil {
		t.Fatalf("GetServerInfo() error = %v", err)
	}
	if len(info.AllowedDomains) != 2 {
		t.Errorf("AllowedDomains len = %d, want 2", len(info.AllowedDomains))
	}
	if info.MaxTTL != 604800 {
		t.Errorf("MaxTTL = %d, want 604800", info.MaxTTL)
	}
}

func TestDeleteAllInboxes_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/inboxes" {
			t.Errorf("path = %s, want /api/inboxes", r.URL.Path)
		}
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"deleted": 5})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	count, err := client.DeleteAllInboxes(context.Background())
	if err != nil {
		t.Fatalf("DeleteAllInboxes() error = %v", err)
	}
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}
}

func TestDeleteAllInboxes_ZeroDeleted(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"deleted": 0})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	count, err := client.DeleteAllInboxes(context.Background())
	if err != nil {
		t.Fatalf("DeleteAllInboxes() error = %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestDeleteAllInboxes_Error(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL), WithRetries(0))
	_, err := client.DeleteAllInboxes(context.Background())
	if err == nil {
		t.Fatal("DeleteAllInboxes() should return error for 500 response")
	}
}

func TestDeleteInboxByEmail_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		// URL should be /api/inboxes/test%40example.com
		if r.URL.Path != "/api/inboxes/test%40example.com" && r.URL.Path != "/api/inboxes/test@example.com" {
			t.Errorf("path = %s, want /api/inboxes/test@example.com", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	err := client.DeleteInboxByEmail(context.Background(), "test@example.com")
	if err != nil {
		t.Fatalf("DeleteInboxByEmail() error = %v", err)
	}
}

func TestGetInboxSync_Success(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"emailCount": 3,
			"emailsHash": "abc123",
		})
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))
	status, err := client.GetInboxSync(context.Background(), "test@example.com")
	if err != nil {
		t.Fatalf("GetInboxSync() error = %v", err)
	}
	if status.EmailCount != 3 {
		t.Errorf("EmailCount = %d, want 3", status.EmailCount)
	}
	if status.EmailsHash != "abc123" {
		t.Errorf("EmailsHash = %s, want abc123", status.EmailsHash)
	}
}

func TestClient_Do_MarshalError(t *testing.T) {
	t.Parallel()
	client, _ := New("test-key", WithBaseURL("https://example.com"))

	// Channels cannot be marshaled to JSON
	unmarshalable := make(chan int)

	err := client.Do(context.Background(), "POST", "/test", unmarshalable, nil)
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if !contains(err.Error(), "marshal request body") {
		t.Errorf("error = %v, want to contain 'marshal request body'", err)
	}
}

func TestClient_Do_ContextCancellationDuringRetryDelay(t *testing.T) {
	t.Parallel()
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, _ := New("test-key",
		WithBaseURL(server.URL),
		WithRetries(5),
	)
	client.retryDelay = 500 * time.Millisecond // Long enough to cancel

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after the first request but during the retry delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := client.Do(ctx, "GET", "/test", nil, nil)
	if err != context.Canceled {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("attempts = %d, want 1 (cancelled during delay)", attempts)
	}
}

func TestClient_Do_NetworkError(t *testing.T) {
	t.Parallel()
	client, _ := New("test-key",
		WithBaseURL("http://localhost:1"), // Invalid port - connection refused
		WithRetries(0),
	)

	err := client.Do(context.Background(), "GET", "/test", nil, nil)
	if err == nil {
		t.Fatal("expected network error")
	}

	netErr, ok := err.(*apierrors.NetworkError)
	if !ok {
		t.Errorf("expected NetworkError, got %T: %v", err, err)
	} else if netErr.Err == nil {
		t.Error("NetworkError.Err should not be nil")
	}
}

func TestClient_Do_NetworkErrorWithRetries(t *testing.T) {
	t.Parallel()
	client, _ := New("test-key",
		WithBaseURL("http://localhost:1"), // Invalid port - connection refused
		WithRetries(2),
	)
	client.retryDelay = time.Millisecond // Fast retries for test

	err := client.Do(context.Background(), "GET", "/test", nil, nil)
	if err == nil {
		t.Fatal("expected network error after retries")
	}

	_, ok := err.(*apierrors.NetworkError)
	if !ok {
		t.Errorf("expected NetworkError after all retries exhausted, got %T: %v", err, err)
	}
}

func TestClient_Do_DecodeError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL))

	var result struct{ OK bool }
	err := client.Do(context.Background(), "GET", "/test", nil, &result)
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !contains(err.Error(), "decode response") {
		t.Errorf("error = %v, want to contain 'decode response'", err)
	}
}

func TestParseErrorResponse_MessageFieldFallback(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		// Use "message" field instead of "error" field
		w.Write([]byte(`{"message": "validation failed", "request_id": "req-123"}`))
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL), WithRetries(0))

	err := client.Do(context.Background(), "GET", "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	apiErr, ok := err.(*apierrors.APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Message != "validation failed" {
		t.Errorf("Message = %s, want 'validation failed'", apiErr.Message)
	}
	if apiErr.RequestID != "req-123" {
		t.Errorf("RequestID = %s, want 'req-123'", apiErr.RequestID)
	}
}

func TestParseErrorResponse_EmptyMessageFields(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		// Both error and message are empty - should use raw body
		w.Write([]byte(`{"error": "", "message": ""}`))
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL), WithRetries(0))

	err := client.Do(context.Background(), "GET", "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	apiErr, ok := err.(*apierrors.APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	// When both fields are empty, it should use the raw body
	if apiErr.Message != `{"error": "", "message": ""}` {
		t.Errorf("Message = %s, want raw body", apiErr.Message)
	}
}

func TestParseErrorResponse_InvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("plain text error"))
	}))
	defer server.Close()

	client, _ := New("test-key", WithBaseURL(server.URL), WithRetries(0))

	err := client.Do(context.Background(), "GET", "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	apiErr, ok := err.(*apierrors.APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", apiErr.StatusCode)
	}
	if apiErr.Message != "plain text error" {
		t.Errorf("Message = %s, want 'plain text error'", apiErr.Message)
	}
}

func TestClient_Do_RetryExhausted(t *testing.T) {
	t.Parallel()
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, _ := New("test-key",
		WithBaseURL(server.URL),
		WithRetries(2),
	)
	client.retryDelay = time.Millisecond

	err := client.Do(context.Background(), "GET", "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	// After exhausting retries, should return APIError (final attempt returns non-retryable error)
	apiErr, ok := err.(*apierrors.APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 503 {
		t.Errorf("StatusCode = %d, want 503", apiErr.StatusCode)
	}
	// Initial + 2 retries = 3 total attempts
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestClient_Do_RequestCreationError(t *testing.T) {
	t.Parallel()
	client, _ := New("test-key", WithBaseURL("https://example.com"))

	// Invalid HTTP method causes request creation to fail
	err := client.Do(context.Background(), "INVALID METHOD WITH SPACES", "/test", nil, nil)
	if err == nil {
		t.Fatal("expected request creation error")
	}
	if !contains(err.Error(), "create request") {
		t.Errorf("error = %v, want to contain 'create request'", err)
	}
}

// errorSeeker is a reader that returns an error when Seek is called
type errorSeeker struct {
	data   []byte
	offset int
}

func (e *errorSeeker) Read(p []byte) (n int, err error) {
	if e.offset >= len(e.data) {
		return 0, io.EOF
	}
	n = copy(p, e.data[e.offset:])
	e.offset += n
	return n, nil
}

func (e *errorSeeker) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("seek error")
}

func TestClient_Do_SeekError(t *testing.T) {
	t.Parallel()
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, _ := New("test-key",
		WithBaseURL(server.URL),
		WithRetries(3),
	)
	client.retryDelay = time.Millisecond

	// Use a reader that returns error on Seek
	body := &errorSeeker{data: []byte(`{"test": "data"}`)}

	err := client.doWithRetry(context.Background(), "POST", "/test", body, nil)
	if err == nil {
		t.Fatal("expected seek error")
	}
	if !contains(err.Error(), "reset request body") {
		t.Errorf("error = %v, want to contain 'reset request body'", err)
	}
	// Should have made 1 attempt before seek error on retry
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}
