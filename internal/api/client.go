package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vaultsandbox/client-go/internal/apierrors"
)

const (
	DefaultTimeout    = 30 * time.Second
	DefaultMaxRetries = 3
	DefaultRetryDelay = 1 * time.Second
)

// DefaultRetryOn contains the default HTTP status codes that trigger a retry.
var DefaultRetryOn = []int{408, 429, 500, 502, 503, 504}

// Client handles HTTP communication with the VaultSandbox API.
// It provides automatic retry logic with exponential backoff for transient failures.
type Client struct {
	// httpClient is the underlying HTTP client used for requests.
	httpClient *http.Client
	// baseURL is the VaultSandbox API base URL (e.g., "https://api.vaultsandbox.com").
	baseURL string
	// apiKey is the API key used for authentication via the X-API-Key header.
	apiKey string
	// maxRetries is the maximum number of retry attempts for failed requests.
	maxRetries int
	// retryDelay is the base delay between retry attempts (doubles with each attempt).
	retryDelay time.Duration
	// retryOn contains HTTP status codes that trigger automatic retry.
	retryOn []int
}

// New creates a new API client using the functional options pattern.
// The apiKey is required for authentication. Use [Option] functions like
// [WithBaseURL], [WithTimeout], and [WithRetries] to customize behavior.
//
// Returns an error if apiKey is empty or if baseURL is not set via [WithBaseURL].
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	c := &Client{
		baseURL: "",
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		maxRetries: DefaultMaxRetries,
		retryDelay: DefaultRetryDelay,
		retryOn:    DefaultRetryOn,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.baseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	return c, nil
}

// Option configures the API client.
type Option func(*Client)

// WithBaseURL sets the base URL.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithRetries sets the number of retries.
func WithRetries(retries int) Option {
	return func(c *Client) {
		c.maxRetries = retries
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithRetryOn sets the HTTP status codes that trigger a retry.
func WithRetryOn(statusCodes []int) Option {
	return func(c *Client) {
		c.retryOn = statusCodes
	}
}

// SetHTTPClient sets a custom HTTP client.
func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

// BaseURL returns the base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// HTTPClient returns the underlying HTTP client.
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

// Do executes an HTTP request with automatic retry logic.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control.
//   - method: HTTP method (GET, POST, DELETE, etc.).
//   - path: API path to append to the base URL (e.g., "/api/inboxes").
//   - body: Request body to JSON-encode, or nil for no body.
//   - result: Pointer to unmarshal the JSON response into, or nil to discard.
//
// The request includes X-API-Key, Content-Type, and Accept headers automatically.
// Retries are attempted with exponential backoff for status codes in retryOn.
func (c *Client) Do(ctx context.Context, method, path string, body any, result any) error {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	return c.doWithRetry(ctx, method, path, bodyReader, result)
}

// doWithRetry implements the retry logic with exponential backoff.
// It handles network errors, retryable status codes, error response parsing,
// and successful response decoding. The body must be an io.Seeker if retries
// are needed, as it will be reset between attempts.
func (c *Client) doWithRetry(ctx context.Context, method, path string, body io.Reader, result any) error {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retryDelay * time.Duration(1<<(attempt-1)) // Exponential backoff
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}

			// Reset body reader if needed
			if seeker, ok := body.(io.Seeker); ok {
				if _, err := seeker.Seek(0, io.SeekStart); err != nil {
					return fmt.Errorf("reset request body: %w", err)
				}
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("X-API-Key", c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = &apierrors.NetworkError{Err: err}
			continue
		}

		// Check for retryable status codes
		if c.isRetryable(resp.StatusCode) && attempt < c.maxRetries {
			lastErr = &apierrors.APIError{StatusCode: resp.StatusCode}
			resp.Body.Close()
			continue
		}

		// Handle error responses
		if resp.StatusCode >= 400 {
			err := parseErrorResponse(resp)
			resp.Body.Close()
			return err
		}

		// Handle 204 No Content
		if resp.StatusCode == http.StatusNoContent {
			resp.Body.Close()
			return nil
		}

		// Parse response
		if result != nil {
			if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
				resp.Body.Close()
				return fmt.Errorf("decode response: %w", err)
			}
		}
		resp.Body.Close()

		return nil
	}

	return lastErr
}

// isRetryable checks if a status code should trigger a retry based on retryOn.
func (c *Client) isRetryable(statusCode int) bool {
	for _, code := range c.retryOn {
		if statusCode == code {
			return true
		}
	}
	return false
}

// parseErrorResponse extracts error information from an HTTP error response.
// It attempts to parse a JSON error body with "error", "message", and "request_id"
// fields. If parsing fails, the raw body is used as the error message.
func parseErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Error     string `json:"error"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil {
		msg := errResp.Error
		if msg == "" {
			msg = errResp.Message
		}
		if msg == "" {
			msg = string(body)
		}
		return &apierrors.APIError{
			StatusCode: resp.StatusCode,
			Message:    msg,
			RequestID:  errResp.RequestID,
		}
	}

	return &apierrors.APIError{
		StatusCode: resp.StatusCode,
		Message:    string(body),
	}
}
