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

// CheckKey validates the API key.
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

// GetServerInfo retrieves server configuration.
func (c *Client) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	var result ServerInfo
	if err := c.Do(ctx, "GET", "/api/server-info", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateInboxNew creates a new inbox using the new API format.
func (c *Client) CreateInboxNew(ctx context.Context, req CreateInboxRequest) (*CreateInboxResponse, error) {
	var result CreateInboxResponse
	if err := c.Do(ctx, "POST", "/api/inboxes", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteInboxByEmail deletes a specific inbox by email address.
func (c *Client) DeleteInboxByEmail(ctx context.Context, emailAddress string) error {
	path := fmt.Sprintf("/api/inboxes/%s", url.PathEscape(emailAddress))
	return c.Do(ctx, "DELETE", path, nil, nil)
}

// DeleteAllInboxes deletes all inboxes for the API key.
func (c *Client) DeleteAllInboxes(ctx context.Context) (int, error) {
	var result struct {
		Deleted int `json:"deleted"`
	}
	if err := c.Do(ctx, "DELETE", "/api/inboxes", nil, &result); err != nil {
		return 0, err
	}
	return result.Deleted, nil
}

// GetInboxSync returns inbox sync status.
func (c *Client) GetInboxSync(ctx context.Context, emailAddress string) (*SyncStatus, error) {
	path := fmt.Sprintf("/api/inboxes/%s/sync", url.PathEscape(emailAddress))
	var result SyncStatus
	if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetEmailsNew lists all emails in an inbox using the new API format.
func (c *Client) GetEmailsNew(ctx context.Context, emailAddress string) ([]RawEmail, error) {
	path := fmt.Sprintf("/api/inboxes/%s/emails", url.PathEscape(emailAddress))
	var result []RawEmail
	if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetEmailNew retrieves a specific email using the new API format.
func (c *Client) GetEmailNew(ctx context.Context, emailAddress, emailID string) (*RawEmail, error) {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s",
		url.PathEscape(emailAddress), url.PathEscape(emailID))
	var result RawEmail
	if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetEmailRawNew retrieves the raw email source using the new API format.
func (c *Client) GetEmailRawNew(ctx context.Context, emailAddress, emailID string) (*RawEmailSource, error) {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s/raw",
		url.PathEscape(emailAddress), url.PathEscape(emailID))
	var result RawEmailSource
	if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// MarkEmailAsReadNew marks an email as read using the new API format.
func (c *Client) MarkEmailAsReadNew(ctx context.Context, emailAddress, emailID string) error {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s/read",
		url.PathEscape(emailAddress), url.PathEscape(emailID))
	return c.Do(ctx, "PATCH", path, nil, nil)
}

// DeleteEmailNew deletes a specific email using the new API format.
func (c *Client) DeleteEmailNew(ctx context.Context, emailAddress, emailID string) error {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s",
		url.PathEscape(emailAddress), url.PathEscape(emailID))
	return c.Do(ctx, "DELETE", path, nil, nil)
}

// OpenEventStream opens an SSE connection for real-time events.
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

// Legacy methods for backward compatibility with existing code

// LegacyCreateInboxRequest is the request for creating an inbox.
type LegacyCreateInboxRequest struct {
	TTL          time.Duration
	EmailAddress string
}

// LegacyCreateInboxResponse is the response from creating an inbox.
type LegacyCreateInboxResponse struct {
	EmailAddress string
	ExpiresAt    time.Time
	InboxHash    string
	ServerSigPk  []byte
	Keypair      *crypto.Keypair
}

// CreateInbox creates a new inbox.
func (c *Client) CreateInbox(ctx context.Context, req *LegacyCreateInboxRequest) (*LegacyCreateInboxResponse, error) {
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

	return &LegacyCreateInboxResponse{
		EmailAddress: apiResp.EmailAddress,
		ExpiresAt:    apiResp.ExpiresAt,
		InboxHash:    apiResp.InboxHash,
		ServerSigPk:  serverSigPk,
		Keypair:      keypair,
	}, nil
}

// GetEmailsResponse is the response from fetching emails (legacy).
type GetEmailsResponse struct {
	Emails []*EncryptedEmail
}

// EncryptedEmail represents an encrypted email from the API (legacy).
type EncryptedEmail struct {
	ID              string
	EncapsulatedKey []byte
	Ciphertext      []byte
	Signature       []byte
	ReceivedAt      time.Time
	IsRead          bool
}

// GetEmails fetches all emails in an inbox (legacy API).
func (c *Client) GetEmails(ctx context.Context, emailAddress string) (*GetEmailsResponse, error) {
	var resp []emailAPIResponse
	path := fmt.Sprintf("/api/inboxes/%s/emails", url.PathEscape(emailAddress))
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	emails := make([]*EncryptedEmail, 0, len(resp))
	for _, e := range resp {
		encrypted, err := parseEncryptedEmail(&e)
		if err != nil {
			return nil, err
		}
		emails = append(emails, encrypted)
	}

	return &GetEmailsResponse{Emails: emails}, nil
}

// GetEmail fetches a specific email (legacy API).
func (c *Client) GetEmail(ctx context.Context, emailAddress, emailID string) (*EncryptedEmail, error) {
	var resp emailAPIResponse
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s", url.PathEscape(emailAddress), url.PathEscape(emailID))
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}

	return parseEncryptedEmail(&resp)
}

// GetEmailRaw fetches the raw email content (legacy API).
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

// MarkEmailAsRead marks an email as read (legacy API).
func (c *Client) MarkEmailAsRead(ctx context.Context, emailAddress, emailID string) error {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s", url.PathEscape(emailAddress), url.PathEscape(emailID))
	return c.do(ctx, http.MethodPatch, path, map[string]bool{"isRead": true}, nil)
}

// DeleteEmail deletes an email (legacy API).
func (c *Client) DeleteEmail(ctx context.Context, emailAddress, emailID string) error {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s", url.PathEscape(emailAddress), url.PathEscape(emailID))
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// DeleteInbox deletes an inbox (legacy API).
func (c *Client) DeleteInbox(ctx context.Context, emailAddress string) error {
	path := fmt.Sprintf("/api/inboxes/%s", url.PathEscape(emailAddress))
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func parseEncryptedEmail(resp *emailAPIResponse) (*EncryptedEmail, error) {
	encapsulatedKey, err := crypto.DecodeBase64(resp.EncapsulatedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode encapsulated key: %w", err)
	}

	ciphertext, err := crypto.DecodeBase64(resp.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	signature, err := crypto.DecodeBase64(resp.Signature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	return &EncryptedEmail{
		ID:              resp.ID,
		EncapsulatedKey: encapsulatedKey,
		Ciphertext:      ciphertext,
		Signature:       signature,
		ReceivedAt:      resp.ReceivedAt,
		IsRead:          resp.IsRead,
	}, nil
}
