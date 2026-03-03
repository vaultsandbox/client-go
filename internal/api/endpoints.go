package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vaultsandbox/client-go/internal/apierrors"
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
		return apierrors.ErrUnauthorized
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
	return apierrors.WithResourceType(c.Do(ctx, "DELETE", path, nil, nil), apierrors.ResourceInbox)
}

// DeleteAllInboxes deletes all inboxes associated with the API key.
// Returns the number of inboxes deleted.
func (c *Client) DeleteAllInboxes(ctx context.Context) (int, error) {
	var result struct {
		Deleted int `json:"deleted"`
	}
	if err := c.Do(ctx, "DELETE", "/api/inboxes", nil, &result); err != nil {
		return 0, apierrors.WithResourceType(err, apierrors.ResourceInbox)
	}
	return result.Deleted, nil
}

// GetInboxSync returns the sync status for an inbox, including the email
// count and a hash that changes when emails are added or removed.
func (c *Client) GetInboxSync(ctx context.Context, emailAddress string) (*SyncStatus, error) {
	path := fmt.Sprintf("/api/inboxes/%s/sync", url.PathEscape(emailAddress))
	var result SyncStatus
	if err := c.Do(ctx, "GET", path, nil, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceInbox)
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
	// EmailAuth controls whether email authentication (SPF, DKIM, DMARC, PTR) is enabled.
	// nil = use server default, true = enable, false = disable.
	EmailAuth *bool
	// Encryption specifies the desired encryption mode ("encrypted" or "plain").
	// Empty string means use server default.
	Encryption string
	// Persistence specifies the desired persistence mode ("persistent" or "ephemeral").
	// Empty string means use server default.
	Persistence string
	// SpamAnalysis controls whether spam analysis (Rspamd) is enabled for this inbox.
	// nil = use server default, true = enable, false = disable.
	SpamAnalysis *bool
}

// CreateInboxResult contains the result of creating an inbox,
// including the generated keypair for encrypted inboxes.
type CreateInboxResult struct {
	// EmailAddress is the created inbox's email address.
	EmailAddress string
	// ExpiresAt is when the inbox will be deleted.
	ExpiresAt time.Time
	// InboxHash is the unique identifier for the inbox.
	InboxHash string
	// ServerSigPk is the server's signing public key. Only set for encrypted inboxes.
	ServerSigPk []byte
	// Keypair is the generated ML-KEM-768 keypair for decryption. Only set for encrypted inboxes.
	Keypair *crypto.Keypair
	// EmailAuth indicates whether email authentication is enabled for this inbox.
	EmailAuth bool
	// Encrypted indicates whether the inbox is encrypted.
	Encrypted bool
	// Persistent indicates whether the inbox is persistent.
	Persistent bool
	// SpamAnalysis indicates whether spam analysis is enabled for this inbox.
	// May be nil if using server default.
	SpamAnalysis *bool
}

// CreateInbox creates a new inbox.
// For encrypted inboxes, a keypair is automatically generated.
// For plain inboxes, no keypair is generated and emails are returned unencrypted.
func (c *Client) CreateInbox(ctx context.Context, req *CreateInboxParams) (*CreateInboxResult, error) {
	apiReq := &createInboxAPIRequest{
		TTL:          int(req.TTL.Seconds()),
		EmailAddress: req.EmailAddress,
		EmailAuth:    req.EmailAuth,
		Encryption:   req.Encryption,
		Persistence:  req.Persistence,
		SpamAnalysis: req.SpamAnalysis,
	}

	// Only generate keypair if requesting encrypted inbox (or server default which may be encrypted).
	// If explicitly requesting plain, skip keypair generation.
	var keypair *crypto.Keypair
	if req.Encryption != "plain" {
		var err error
		keypair, err = crypto.GenerateKeypair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate keypair: %w", err)
		}
		apiReq.ClientKemPk = crypto.ToBase64URL(keypair.PublicKey)
	}

	var apiResp createInboxAPIResponse
	if err := c.Do(ctx, http.MethodPost, "/api/inboxes", apiReq, &apiResp); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceInbox)
	}

	result := &CreateInboxResult{
		EmailAddress: apiResp.EmailAddress,
		ExpiresAt:    apiResp.ExpiresAt,
		InboxHash:    apiResp.InboxHash,
		EmailAuth:    apiResp.EmailAuth,
		Encrypted:    apiResp.Encrypted,
		Persistent:   apiResp.Persistent,
		SpamAnalysis: apiResp.SpamAnalysis,
	}

	// Only decode server signature key and set keypair for encrypted inboxes
	if apiResp.Encrypted {
		if apiResp.ServerSigPk == "" {
			return nil, fmt.Errorf("server did not return signature public key for encrypted inbox")
		}
		serverSigPk, err := crypto.DecodeBase64(apiResp.ServerSigPk)
		if err != nil {
			return nil, fmt.Errorf("failed to decode server signature public key: %w", err)
		}
		result.ServerSigPk = serverSigPk
		result.Keypair = keypair
	}

	return result, nil
}

// GetEmailsResponse contains the result of listing emails in an inbox.
type GetEmailsResponse struct {
	// Emails is the list of emails in the inbox.
	Emails []*RawEmail
}

// GetEmails returns all emails in an inbox.
// If includeContent is true, the server returns full email content.
func (c *Client) GetEmails(ctx context.Context, emailAddress string, includeContent bool) (*GetEmailsResponse, error) {
	var resp []*RawEmail
	path := fmt.Sprintf("/api/inboxes/%s/emails", url.PathEscape(emailAddress))
	if includeContent {
		path += "?includeContent=true"
	}
	if err := c.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		// This endpoint can fail due to inbox not found
		return nil, apierrors.WithResourceType(err, apierrors.ResourceInbox)
	}

	return &GetEmailsResponse{Emails: resp}, nil
}

// GetEmail returns a specific email by ID.
func (c *Client) GetEmail(ctx context.Context, emailAddress, emailID string) (*RawEmail, error) {
	var resp RawEmail
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s", url.PathEscape(emailAddress), url.PathEscape(emailID))
	if err := c.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceEmail)
	}

	return &resp, nil
}

// GetEmailRaw returns the raw RFC 5322 email source.
// Returns a RawEmailSource which can be either encrypted or plain.
func (c *Client) GetEmailRaw(ctx context.Context, emailAddress, emailID string) (*RawEmailSource, error) {
	var resp RawEmailSource
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s/raw", url.PathEscape(emailAddress), url.PathEscape(emailID))
	if err := c.Do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceEmail)
	}
	return &resp, nil
}

// MarkEmailAsRead marks an email as read.
func (c *Client) MarkEmailAsRead(ctx context.Context, emailAddress, emailID string) error {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s/read", url.PathEscape(emailAddress), url.PathEscape(emailID))
	return apierrors.WithResourceType(c.Do(ctx, http.MethodPatch, path, nil, nil), apierrors.ResourceEmail)
}

// DeleteEmail deletes the specified email from an inbox.
func (c *Client) DeleteEmail(ctx context.Context, emailAddress, emailID string) error {
	path := fmt.Sprintf("/api/inboxes/%s/emails/%s", url.PathEscape(emailAddress), url.PathEscape(emailID))
	return apierrors.WithResourceType(c.Do(ctx, http.MethodDelete, path, nil, nil), apierrors.ResourceEmail)
}


