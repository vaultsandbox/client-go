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

// Client is the HTTP API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	retries    int
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
		c.retries = retries
	}
}

// New creates a new API client.
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	c := &Client{
		baseURL: "https://api.vaultsandbox.com",
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		retries: 3,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// SetHTTPClient sets a custom HTTP client.
func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= c.retries; attempt++ {
		resp, lastErr = c.httpClient.Do(req)
		if lastErr == nil && resp.StatusCode < 500 {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		if attempt < c.retries {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	if lastErr != nil {
		return fmt.Errorf("request failed: %w", lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return parseErrorResponse(resp)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

func parseErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Error     string `json:"error"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		return &Error{
			StatusCode: resp.StatusCode,
			Message:    errResp.Error,
			RequestID:  errResp.RequestID,
		}
	}

	return &Error{
		StatusCode: resp.StatusCode,
		Message:    string(body),
	}
}

// Error represents an API error.
type Error struct {
	StatusCode int
	Message    string
	RequestID  string
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("API error: %d", e.StatusCode)
}
