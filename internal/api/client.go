package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	DefaultTimeout    = 30 * time.Second
	DefaultMaxRetries = 3
	DefaultRetryDelay = 1 * time.Second
)

// Client handles HTTP communication with the VaultSandbox API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	maxRetries int
	retryDelay time.Duration
}

// Config holds API client configuration.
type Config struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	MaxRetries int
	RetryDelay time.Duration
	Timeout    time.Duration
}

// NewClient creates a new API client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = DefaultTimeout
		}
		httpClient = &http.Client{
			Timeout: timeout,
		}
	}

	maxRetries := cfg.MaxRetries
	if maxRetries == 0 {
		maxRetries = DefaultMaxRetries
	}

	retryDelay := cfg.RetryDelay
	if retryDelay == 0 {
		retryDelay = DefaultRetryDelay
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}, nil
}

// New creates a new API client (backward compatible).
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

// Do executes an HTTP request with retry logic.
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
			lastErr = &NetworkError{Err: err}
			continue
		}

		// Check for retryable status codes
		if isRetryable(resp.StatusCode) && attempt < c.maxRetries {
			lastErr = &APIError{StatusCode: resp.StatusCode}
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

// do is the backward-compatible method for existing code.
func (c *Client) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	return c.Do(ctx, method, path, body, result)
}

// isRetryable checks if a status code should trigger a retry.
func isRetryable(statusCode int) bool {
	switch statusCode {
	case 408, 429, 500, 502, 503, 504:
		return true
	default:
		return false
	}
}

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
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    msg,
			RequestID:  errResp.RequestID,
		}
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    string(body),
	}
}
