package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go/internal/apierrors"
)

func TestNew_RequiresAPIKey(t *testing.T) {
	_, err := New("", WithBaseURL("https://example.com"))
	if err == nil {
		t.Error("expected error for empty API key")
	}
}

func TestNew_RequiresBaseURL(t *testing.T) {
	_, err := New("test-key") // No base URL option
	if err == nil {
		t.Error("expected error for missing base URL")
	}
}

func TestNew_DefaultValues(t *testing.T) {
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
	client, _ := New("test-key", WithBaseURL("https://example.com"))

	if client.BaseURL() != "https://example.com" {
		t.Errorf("BaseURL() = %s, want https://example.com", client.BaseURL())
	}
}

func TestClient_HTTPClient(t *testing.T) {
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
	client, _ := New("test-key", WithBaseURL("https://example.com"))

	newHTTPClient := &http.Client{Timeout: 120 * time.Second}
	client.SetHTTPClient(newHTTPClient)

	if client.HTTPClient() != newHTTPClient {
		t.Error("SetHTTPClient() did not update the client")
	}
}

func TestIsRetryable(t *testing.T) {
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
