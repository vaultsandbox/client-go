package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/vaultsandbox/client-go/internal/crypto"
)

// CreateInbox creates a new inbox.
func (c *Client) CreateInbox(ctx context.Context, req *CreateInboxRequest) (*CreateInboxResponse, error) {
	keypair, err := crypto.GenerateKeypair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	apiReq := &createInboxAPIRequest{
		PublicKey:    crypto.EncodeBase64(keypair.PublicKey),
		TTL:          int(req.TTL.Seconds()),
		EmailAddress: req.EmailAddress,
	}

	var apiResp createInboxAPIResponse
	if err := c.do(ctx, http.MethodPost, "/v1/inboxes", apiReq, &apiResp); err != nil {
		return nil, err
	}

	serverSigPk, err := crypto.DecodeBase64(apiResp.ServerSignaturePublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode server signature public key: %w", err)
	}

	return &CreateInboxResponse{
		EmailAddress: apiResp.EmailAddress,
		ExpiresAt:    apiResp.ExpiresAt,
		InboxHash:    apiResp.InboxHash,
		ServerSigPk:  serverSigPk,
		Keypair:      keypair,
	}, nil
}

// GetEmails fetches all emails in an inbox.
func (c *Client) GetEmails(ctx context.Context, inboxHash string) (*GetEmailsResponse, error) {
	var resp getEmailsAPIResponse
	if err := c.do(ctx, http.MethodGet, "/v1/inboxes/"+inboxHash+"/emails", nil, &resp); err != nil {
		return nil, err
	}

	emails := make([]*EncryptedEmail, 0, len(resp.Emails))
	for _, e := range resp.Emails {
		encrypted, err := parseEncryptedEmail(&e)
		if err != nil {
			return nil, err
		}
		emails = append(emails, encrypted)
	}

	return &GetEmailsResponse{Emails: emails}, nil
}

// GetEmail fetches a specific email.
func (c *Client) GetEmail(ctx context.Context, inboxHash, emailID string) (*EncryptedEmail, error) {
	var resp emailAPIResponse
	if err := c.do(ctx, http.MethodGet, "/v1/inboxes/"+inboxHash+"/emails/"+emailID, nil, &resp); err != nil {
		return nil, err
	}

	return parseEncryptedEmail(&resp)
}

// GetEmailRaw fetches the raw email content.
func (c *Client) GetEmailRaw(ctx context.Context, inboxHash, emailID string) (string, error) {
	var resp struct {
		Raw string `json:"raw"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/inboxes/"+inboxHash+"/emails/"+emailID+"/raw", nil, &resp); err != nil {
		return "", err
	}
	return resp.Raw, nil
}

// MarkEmailAsRead marks an email as read.
func (c *Client) MarkEmailAsRead(ctx context.Context, inboxHash, emailID string) error {
	return c.do(ctx, http.MethodPatch, "/v1/inboxes/"+inboxHash+"/emails/"+emailID, map[string]bool{"is_read": true}, nil)
}

// DeleteEmail deletes an email.
func (c *Client) DeleteEmail(ctx context.Context, inboxHash, emailID string) error {
	return c.do(ctx, http.MethodDelete, "/v1/inboxes/"+inboxHash+"/emails/"+emailID, nil, nil)
}

// DeleteInbox deletes an inbox.
func (c *Client) DeleteInbox(ctx context.Context, inboxHash string) error {
	return c.do(ctx, http.MethodDelete, "/v1/inboxes/"+inboxHash, nil, nil)
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

// CreateInboxRequest is the request for creating an inbox.
type CreateInboxRequest struct {
	TTL          time.Duration
	EmailAddress string
}

// CreateInboxResponse is the response from creating an inbox.
type CreateInboxResponse struct {
	EmailAddress string
	ExpiresAt    time.Time
	InboxHash    string
	ServerSigPk  []byte
	Keypair      *crypto.Keypair
}

// GetEmailsResponse is the response from fetching emails.
type GetEmailsResponse struct {
	Emails []*EncryptedEmail
}

// EncryptedEmail represents an encrypted email from the API.
type EncryptedEmail struct {
	ID              string
	EncapsulatedKey []byte
	Ciphertext      []byte
	Signature       []byte
	ReceivedAt      time.Time
	IsRead          bool
}
