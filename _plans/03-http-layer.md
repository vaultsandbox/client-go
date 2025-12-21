# 03 - HTTP Layer

## Overview

The HTTP layer provides a robust API client with:
- Automatic retry logic for transient failures
- Exponential backoff with jitter
- Request/response logging (optional)
- Context-based cancellation and timeouts

## File: `internal/api/client.go`

```go
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
    DefaultBaseURL    = "https://smtp.vaultsandbox.com"
    DefaultTimeout    = 30 * time.Second
    DefaultMaxRetries = 3
    DefaultRetryDelay = 1 * time.Second
)

// Client handles HTTP communication with the VaultSandbox API
type Client struct {
    httpClient *http.Client
    baseURL    string
    apiKey     string
    maxRetries int
    retryDelay time.Duration
}

// Config holds API client configuration
type Config struct {
    BaseURL    string
    APIKey     string
    HTTPClient *http.Client
    MaxRetries int
    RetryDelay time.Duration
    Timeout    time.Duration
}

// NewClient creates a new API client
func NewClient(cfg Config) *Client {
    if cfg.BaseURL == "" {
        cfg.BaseURL = DefaultBaseURL
    }
    if cfg.HTTPClient == nil {
        cfg.HTTPClient = &http.Client{
            Timeout: cfg.Timeout,
        }
        if cfg.Timeout == 0 {
            cfg.HTTPClient.Timeout = DefaultTimeout
        }
    }
    if cfg.MaxRetries == 0 {
        cfg.MaxRetries = DefaultMaxRetries
    }
    if cfg.RetryDelay == 0 {
        cfg.RetryDelay = DefaultRetryDelay
    }

    return &Client{
        httpClient: cfg.HTTPClient,
        baseURL:    cfg.BaseURL,
        apiKey:     cfg.APIKey,
        maxRetries: cfg.MaxRetries,
        retryDelay: cfg.RetryDelay,
    }
}

// Do executes an HTTP request with retry logic
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
                seeker.Seek(0, io.SeekStart)
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
        defer resp.Body.Close()

        // Check for retryable status codes
        if isRetryable(resp.StatusCode) && attempt < c.maxRetries {
            lastErr = &APIError{StatusCode: resp.StatusCode}
            continue
        }

        // Handle error responses
        if resp.StatusCode >= 400 {
            return parseErrorResponse(resp)
        }

        // Handle 204 No Content
        if resp.StatusCode == http.StatusNoContent {
            return nil
        }

        // Parse response
        if result != nil {
            if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
                return fmt.Errorf("decode response: %w", err)
            }
        }

        return nil
    }

    return lastErr
}

// isRetryable checks if a status code should trigger a retry
func isRetryable(statusCode int) bool {
    switch statusCode {
    case 408, 429, 500, 502, 503, 504:
        return true
    default:
        return false
    }
}
```

## File: `internal/api/endpoints.go`

```go
package api

import (
    "context"
    "fmt"
    "net/url"
    "strings"
)

// CheckKey validates the API key
func (c *Client) CheckKey(ctx context.Context) error {
    var result struct {
        OK bool `json:"ok"`
    }
    if err := c.Do(ctx, "GET", "/api/check-key", nil, &result); err != nil {
        return err
    }
    if !result.OK {
        return ErrInvalidAPIKey
    }
    return nil
}

// GetServerInfo retrieves server configuration
func (c *Client) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
    var result ServerInfo
    if err := c.Do(ctx, "GET", "/api/server-info", nil, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// CreateInbox creates a new inbox
func (c *Client) CreateInbox(ctx context.Context, req CreateInboxRequest) (*CreateInboxResponse, error) {
    var result CreateInboxResponse
    if err := c.Do(ctx, "POST", "/api/inboxes", req, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// DeleteInbox deletes a specific inbox
func (c *Client) DeleteInbox(ctx context.Context, emailAddress string) error {
    path := fmt.Sprintf("/api/inboxes/%s", url.PathEscape(emailAddress))
    return c.Do(ctx, "DELETE", path, nil, nil)
}

// DeleteAllInboxes deletes all inboxes for the API key
func (c *Client) DeleteAllInboxes(ctx context.Context) (int, error) {
    var result struct {
        Deleted int `json:"deleted"`
    }
    if err := c.Do(ctx, "DELETE", "/api/inboxes", nil, &result); err != nil {
        return 0, err
    }
    return result.Deleted, nil
}

// GetInboxSync returns inbox sync status
func (c *Client) GetInboxSync(ctx context.Context, emailAddress string) (*SyncStatus, error) {
    path := fmt.Sprintf("/api/inboxes/%s/sync", url.PathEscape(emailAddress))
    var result SyncStatus
    if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// GetEmails lists all emails in an inbox
func (c *Client) GetEmails(ctx context.Context, emailAddress string) ([]RawEmail, error) {
    path := fmt.Sprintf("/api/inboxes/%s/emails", url.PathEscape(emailAddress))
    var result []RawEmail
    if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
        return nil, err
    }
    return result, nil
}

// GetEmail retrieves a specific email
func (c *Client) GetEmail(ctx context.Context, emailAddress, emailID string) (*RawEmail, error) {
    path := fmt.Sprintf("/api/inboxes/%s/emails/%s",
        url.PathEscape(emailAddress), url.PathEscape(emailID))
    var result RawEmail
    if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// GetEmailRaw retrieves the raw email source
func (c *Client) GetEmailRaw(ctx context.Context, emailAddress, emailID string) (*RawEmailSource, error) {
    path := fmt.Sprintf("/api/inboxes/%s/emails/%s/raw",
        url.PathEscape(emailAddress), url.PathEscape(emailID))
    var result RawEmailSource
    if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
        return nil, err
    }
    return &result, nil
}

// MarkEmailAsRead marks an email as read
func (c *Client) MarkEmailAsRead(ctx context.Context, emailAddress, emailID string) error {
    path := fmt.Sprintf("/api/inboxes/%s/emails/%s/read",
        url.PathEscape(emailAddress), url.PathEscape(emailID))
    return c.Do(ctx, "PATCH", path, nil, nil)
}

// DeleteEmail deletes a specific email
func (c *Client) DeleteEmail(ctx context.Context, emailAddress, emailID string) error {
    path := fmt.Sprintf("/api/inboxes/%s/emails/%s",
        url.PathEscape(emailAddress), url.PathEscape(emailID))
    return c.Do(ctx, "DELETE", path, nil, nil)
}

// OpenEventStream opens an SSE connection for real-time events
func (c *Client) OpenEventStream(ctx context.Context, inboxHashes []string) (*http.Response, error) {
    path := fmt.Sprintf("/api/events?inboxes=%s", url.QueryEscape(strings.Join(inboxHashes, ",")))

    req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("X-API-Key", c.apiKey)
    req.Header.Set("Accept", "text/event-stream")
    req.Header.Set("Cache-Control", "no-cache")

    return c.httpClient.Do(req)
}
```

## File: `internal/api/types.go`

```go
package api

import (
    "time"

    "github.com/vaultsandbox/client-go/internal/crypto"
)

// ServerInfo represents the /api/server-info response
type ServerInfo struct {
    ServerSigPk    string              `json:"serverSigPk"`
    Algs           crypto.AlgorithmSuite `json:"algs"`
    Context        string              `json:"context"`
    MaxTTL         int                 `json:"maxTtl"`
    DefaultTTL     int                 `json:"defaultTtl"`
    SSEConsole     bool                `json:"sseConsole"`
    AllowedDomains []string            `json:"allowedDomains"`
}

// CreateInboxRequest represents the POST /api/inboxes request
type CreateInboxRequest struct {
    ClientKemPk  string `json:"clientKemPk"`
    TTL          int    `json:"ttl,omitempty"`
    EmailAddress string `json:"emailAddress,omitempty"`
}

// CreateInboxResponse represents the POST /api/inboxes response
type CreateInboxResponse struct {
    EmailAddress string    `json:"emailAddress"`
    ExpiresAt    time.Time `json:"expiresAt"`
    InboxHash    string    `json:"inboxHash"`
    ServerSigPk  string    `json:"serverSigPk"`
}

// SyncStatus represents the /api/inboxes/{email}/sync response
type SyncStatus struct {
    EmailCount int    `json:"emailCount"`
    EmailsHash string `json:"emailsHash"`
}

// RawEmail represents an encrypted email from the API
type RawEmail struct {
    ID                string                  `json:"id"`
    InboxID           string                  `json:"inboxId"`
    ReceivedAt        time.Time               `json:"receivedAt"`
    IsRead            bool                    `json:"isRead"`
    EncryptedMetadata *crypto.EncryptedPayload `json:"encryptedMetadata"`
    EncryptedParsed   *crypto.EncryptedPayload `json:"encryptedParsed,omitempty"`
}

// RawEmailSource represents the raw email source response
type RawEmailSource struct {
    ID           string                  `json:"id"`
    EncryptedRaw *crypto.EncryptedPayload `json:"encryptedRaw"`
}

// SSEEvent represents an SSE event payload
type SSEEvent struct {
    InboxID           string                  `json:"inboxId"`
    EmailID           string                  `json:"emailId"`
    EncryptedMetadata *crypto.EncryptedPayload `json:"encryptedMetadata"`
}
```

## Retry Strategy

```
┌─────────────────────────────────────────────────────────────┐
│                       HTTP Request                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │  Send Request   │
                    └─────────────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
        Network Error    Status 2xx     Status 4xx/5xx
              │               │               │
              ▼               ▼               ▼
        ┌─────────┐    ┌─────────────┐   ┌──────────┐
        │  Retry  │    │   Success   │   │ Retryable│
        │ (if <N) │    │   Return    │   │   Code?  │
        └─────────┘    └─────────────┘   └──────────┘
              │                               │
              │                    ┌──────────┴──────────┐
              │                    │                     │
              │                   Yes                   No
              │                    │                     │
              │                    ▼                     ▼
              │              ┌─────────┐          ┌──────────┐
              │              │  Retry  │          │  Return  │
              │              │ (if <N) │          │  Error   │
              │              └─────────┘          └──────────┘
              │                    │
              └────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │ Exponential     │
                    │ Backoff Wait    │
                    │ (1s, 2s, 4s...) │
                    └─────────────────┘
```

## Retryable Status Codes

| Code | Name                  | Retry Reason                    |
|------|-----------------------|---------------------------------|
| 408  | Request Timeout       | Client took too long            |
| 429  | Too Many Requests     | Rate limited, back off          |
| 500  | Internal Server Error | Server issue, may be transient  |
| 502  | Bad Gateway           | Proxy/upstream issue            |
| 503  | Service Unavailable   | Temporary overload              |
| 504  | Gateway Timeout       | Upstream timeout                |

## Error Response Parsing

```go
func parseErrorResponse(resp *http.Response) error {
    body, _ := io.ReadAll(resp.Body)

    switch resp.StatusCode {
    case 401:
        return ErrUnauthorized
    case 404:
        // Determine if inbox or email not found based on path
        if strings.Contains(resp.Request.URL.Path, "/emails/") {
            return ErrEmailNotFound
        }
        return ErrInboxNotFound
    case 409:
        return ErrInboxAlreadyExists
    default:
        return &APIError{
            StatusCode: resp.StatusCode,
            Message:    string(body),
        }
    }
}
```
