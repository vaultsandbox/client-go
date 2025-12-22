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

// DeleteInboxByEmail deletes the inbox with the given email address.
// Returns [ErrInboxNotFound] if the inbox does not exist.
func (c *Client) DeleteInboxByEmail(ctx context.Context, emailAddress string) error {
	path := fmt.Sprintf("/api/inboxes/%s", url.PathEscape(emailAddress))
	return WithResourceType(c.Do(ctx, "DELETE", path, nil, nil), ResourceInbox)
}

// DeleteAllInboxes deletes all inboxes associated with the API key.
// Returns the number of inboxes deleted.
func (c *Client) DeleteAllInboxes(ctx context.Context) (int, error) {
	var result struct {
		Deleted int `json:"deleted"`
	}
	if err := c.Do(ctx, "DELETE", "/api/inboxes", nil, &result); err != nil {
		return 0, WithResourceType(err, ResourceInbox)
	}
	return result.Deleted, nil
}

// GetInboxSync returns the sync status for an inbox, including the email
// count and a hash that changes when emails are added or removed.
func (c *Client) GetInboxSync(ctx context.Context, emailAddress string) (*SyncStatus, error) {
	path := fmt.Sprintf("/api/inboxes/%s/sync", url.PathEscape(emailAddress))
	var result SyncStatus
	if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
		return nil, WithResourceType(err, ResourceInbox)
	}
	return &result, nil
}

// OpenEventStream opens a Server-Sent Events connection for real-time
// email notifications. The caller is responsible for reading events from
// the response body and closing it when done.
//
// This method uses a dedicated HTTP client without a timeout to support
// long-lived SSE connections. Use the context for cancellation control.
func (c *Client) OpenEventStream(ctx context.Context, inboxHashes []string) (*http.Response, error) {
	path := fmt.Sprintf("/api/events?inboxes=%s", url.QueryEscape(strings.Join(inboxHashes, ",")))

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// Clone transport from existing client, but disable timeout for SSE
	sseClient := &http.Client{
		Transport: c.httpClient.Transport,
		Timeout:   0,
	}
	return sseClient.Do(req)
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
		return nil, WithResourceType(err, ResourceInbox)
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
		// This endpoint can fail due to inbox not found
		return nil, WithResourceType(err, ResourceInbox)
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
		return nil, WithResourceType(err, ResourceEmail)
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
		return "", WithResourceType(err, ResourceEmail)
	}
	return resp.Raw, nil
}

// MarkEmailAsRead marks an email as read.
func (c *Client) MarkEmailAsRead(ctx context.Context, emailAddress, emailID string) error {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s/read", url.PathEscape(emailAddress), url.PathEscape(emailID))
	return WithResourceType(c.do(ctx, http.MethodPatch, path, nil, nil), ResourceEmail)
}

// DeleteEmail deletes the specified email from an inbox.
func (c *Client) DeleteEmail(ctx context.Context, emailAddress, emailID string) error {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s", url.PathEscape(emailAddress), url.PathEscape(emailID))
	return WithResourceType(c.do(ctx, http.MethodDelete, path, nil, nil), ResourceEmail)
}

// DeleteInbox deletes the inbox with the given email address.
func (c *Client) DeleteInbox(ctx context.Context, emailAddress string) error {
	path := fmt.Sprintf("/api/inboxes/%s", url.PathEscape(emailAddress))
	return WithResourceType(c.do(ctx, http.MethodDelete, path, nil, nil), ResourceInbox)
}

