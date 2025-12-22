package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vaultsandbox/client-go/internal/crypto"
)

// CheckKey validates the API key by making a request to the server.
// Returns nil if the key is valid, or an error if validation fails.
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

// GetServerInfo retrieves the server configuration including supported
// algorithms, TTL limits, and allowed email domains.
func (c *Client) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	var result ServerInfo
	if err := c.Do(ctx, "GET", "/api/server-info", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateInboxNew creates a new inbox using the new API format with
// the provided client public key and optional TTL/email address.
func (c *Client) CreateInboxNew(ctx context.Context, req CreateInboxRequest) (*CreateInboxResponse, error) {
	var result CreateInboxResponse
	if err := c.Do(ctx, "POST", "/api/inboxes", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteInboxByEmail deletes the inbox with the given email address.
// Returns [ErrInboxNotFound] if the inbox does not exist.
func (c *Client) DeleteInboxByEmail(ctx context.Context, emailAddress string) error {
	path := fmt.Sprintf("/api/inboxes/%s", url.PathEscape(emailAddress))
	return c.Do(ctx, "DELETE", path, nil, nil)
}

// DeleteAllInboxes deletes all inboxes associated with the API key.
// Returns the number of inboxes deleted.
func (c *Client) DeleteAllInboxes(ctx context.Context) (int, error) {
	var result struct {
		Deleted int `json:"deleted"`
	}
	if err := c.Do(ctx, "DELETE", "/api/inboxes", nil, &result); err != nil {
		return 0, err
	}
	return result.Deleted, nil
}

// GetInboxSync returns the sync status for an inbox, including the email
// count and a hash that changes when emails are added or removed.
func (c *Client) GetInboxSync(ctx context.Context, emailAddress string) (*SyncStatus, error) {
	path := fmt.Sprintf("/api/inboxes/%s/sync", url.PathEscape(emailAddress))
	var result SyncStatus
	if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetEmailsNew returns all emails in an inbox with encrypted metadata.
// The full email content requires fetching each email individually.
func (c *Client) GetEmailsNew(ctx context.Context, emailAddress string) ([]RawEmail, error) {
	path := fmt.Sprintf("/api/inboxes/%s/emails", url.PathEscape(emailAddress))
	var result []RawEmail
	if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetEmailNew retrieves a specific email with its full encrypted content
// including the parsed body and attachments.
func (c *Client) GetEmailNew(ctx context.Context, emailAddress, emailID string) (*RawEmail, error) {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s",
		url.PathEscape(emailAddress), url.PathEscape(emailID))
	var result RawEmail
	if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetEmailRawNew retrieves the raw RFC 5322 email source in encrypted form.
func (c *Client) GetEmailRawNew(ctx context.Context, emailAddress, emailID string) (*RawEmailSource, error) {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s/raw",
		url.PathEscape(emailAddress), url.PathEscape(emailID))
	var result RawEmailSource
	if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// MarkEmailAsReadNew marks an email as read.
func (c *Client) MarkEmailAsReadNew(ctx context.Context, emailAddress, emailID string) error {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s/read",
		url.PathEscape(emailAddress), url.PathEscape(emailID))
	return c.Do(ctx, "PATCH", path, nil, nil)
}

// DeleteEmailNew deletes the specified email from an inbox.
func (c *Client) DeleteEmailNew(ctx context.Context, emailAddress, emailID string) error {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s",
		url.PathEscape(emailAddress), url.PathEscape(emailID))
	return c.Do(ctx, "DELETE", path, nil, nil)
}

// OpenEventStream opens a Server-Sent Events connection for real-time
// email notifications. The caller is responsible for reading events from
// the response body and closing it when done.
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

// CreateInboxParams contains parameters for creating an inbox.
type CreateInboxParams struct {
	// TTL is the time-to-live for the inbox.
	TTL time.Duration
	// EmailAddress is the optional desired email address.
	EmailAddress string
}

// CreateInboxResult contains the result of creating an inbox,
// including the generated keypair.
type CreateInboxResult struct {
	// EmailAddress is the created inbox's email address.
	EmailAddress string
	// ExpiresAt is when the inbox will be deleted.
	ExpiresAt time.Time
	// InboxHash is the unique identifier for the inbox.
	InboxHash string
	// ServerSigPk is the server's signing public key.
	ServerSigPk []byte
	// Keypair is the generated ML-KEM-768 keypair for decryption.
	Keypair *crypto.Keypair
}

// CreateInbox creates a new inbox with an automatically generated keypair.
func (c *Client) CreateInbox(ctx context.Context, req *CreateInboxParams) (*CreateInboxResult, error) {
	keypair, err := crypto.GenerateKeypair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	apiReq := &createInboxAPIRequest{
		ClientKemPk:  crypto.EncodeBase64(keypair.PublicKey),
		TTL:          int(req.TTL.Seconds()),
		EmailAddress: req.EmailAddress,
	}

	var apiResp createInboxAPIResponse
	if err := c.do(ctx, http.MethodPost, "/api/inboxes", apiReq, &apiResp); err != nil {
		return nil, err
	}

	serverSigPk, err := crypto.DecodeBase64(apiResp.ServerSigPk)
	if err != nil {
		return nil, fmt.Errorf("failed to decode server signature public key: %w", err)
	}

	return &CreateInboxResult{
		EmailAddress: apiResp.EmailAddress,
		ExpiresAt:    apiResp.ExpiresAt,
		InboxHash:    apiResp.InboxHash,
		ServerSigPk:  serverSigPk,
		Keypair:      keypair,
	}, nil
}

// GetEmailsResponse contains the result of listing emails in an inbox.
type GetEmailsResponse struct {
	// Emails is the list of emails in the inbox.
	Emails []*RawEmail
}

// GetEmails returns all emails in an inbox.
func (c *Client) GetEmails(ctx context.Context, emailAddress string) (*GetEmailsResponse, error) {
	var resp []RawEmail
	path := fmt.Sprintf("/api/inboxes/%s/emails", url.PathEscape(emailAddress))
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	emails := make([]*RawEmail, 0, len(resp))
	for i := range resp {
		emails = append(emails, &resp[i])
	}

	return &GetEmailsResponse{Emails: emails}, nil
}

// GetEmail returns a specific email by ID.
func (c *Client) GetEmail(ctx context.Context, emailAddress, emailID string) (*RawEmail, error) {
	var resp RawEmail
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s", url.PathEscape(emailAddress), url.PathEscape(emailID))
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetEmailRaw returns the raw RFC 5322 email source.
func (c *Client) GetEmailRaw(ctx context.Context, emailAddress, emailID string) (string, error) {
	var resp struct {
		Raw string `json:"raw"`
	}
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s/raw", url.PathEscape(emailAddress), url.PathEscape(emailID))
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return "", err
	}
	return resp.Raw, nil
}

// MarkEmailAsRead marks an email as read.
func (c *Client) MarkEmailAsRead(ctx context.Context, emailAddress, emailID string) error {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s/read", url.PathEscape(emailAddress), url.PathEscape(emailID))
	return c.do(ctx, http.MethodPatch, path, nil, nil)
}

// DeleteEmail deletes the specified email from an inbox.
func (c *Client) DeleteEmail(ctx context.Context, emailAddress, emailID string) error {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s", url.PathEscape(emailAddress), url.PathEscape(emailID))
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// DeleteInbox deletes the inbox with the given email address.
func (c *Client) DeleteInbox(ctx context.Context, emailAddress string) error {
	path := fmt.Sprintf("/api/inboxes/%s", url.PathEscape(emailAddress))
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

