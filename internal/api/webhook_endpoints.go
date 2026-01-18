package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/vaultsandbox/client-go/internal/apierrors"
)

// Global webhook endpoints

// CreateGlobalWebhook creates a new global webhook.
func (c *Client) CreateGlobalWebhook(ctx context.Context, req *CreateWebhookRequest) (*WebhookDTO, error) {
	var result WebhookDTO
	if err := c.Do(ctx, http.MethodPost, "/api/webhooks", req, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceWebhook)
	}
	return &result, nil
}

// ListGlobalWebhooks returns all global webhooks.
func (c *Client) ListGlobalWebhooks(ctx context.Context) (*WebhookListResponseDTO, error) {
	var result WebhookListResponseDTO
	if err := c.Do(ctx, http.MethodGet, "/api/webhooks", nil, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceWebhook)
	}
	return &result, nil
}

// GetGlobalWebhook returns a specific global webhook by ID.
func (c *Client) GetGlobalWebhook(ctx context.Context, webhookID string) (*WebhookDTO, error) {
	var result WebhookDTO
	path := fmt.Sprintf("/api/webhooks/%s", url.PathEscape(webhookID))
	if err := c.Do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceWebhook)
	}
	return &result, nil
}

// UpdateGlobalWebhook updates a global webhook.
func (c *Client) UpdateGlobalWebhook(ctx context.Context, webhookID string, req *UpdateWebhookRequest) (*WebhookDTO, error) {
	var result WebhookDTO
	path := fmt.Sprintf("/api/webhooks/%s", url.PathEscape(webhookID))
	if err := c.Do(ctx, http.MethodPatch, path, req, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceWebhook)
	}
	return &result, nil
}

// DeleteGlobalWebhook deletes a global webhook.
func (c *Client) DeleteGlobalWebhook(ctx context.Context, webhookID string) error {
	path := fmt.Sprintf("/api/webhooks/%s", url.PathEscape(webhookID))
	return apierrors.WithResourceType(c.Do(ctx, http.MethodDelete, path, nil, nil), apierrors.ResourceWebhook)
}

// TestGlobalWebhook sends a test request to a global webhook.
func (c *Client) TestGlobalWebhook(ctx context.Context, webhookID string) (*TestWebhookResponseDTO, error) {
	var result TestWebhookResponseDTO
	path := fmt.Sprintf("/api/webhooks/%s/test", url.PathEscape(webhookID))
	if err := c.Do(ctx, http.MethodPost, path, nil, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceWebhook)
	}
	return &result, nil
}

// RotateGlobalWebhookSecret rotates the secret for a global webhook.
func (c *Client) RotateGlobalWebhookSecret(ctx context.Context, webhookID string) (*RotateSecretResponseDTO, error) {
	var result RotateSecretResponseDTO
	path := fmt.Sprintf("/api/webhooks/%s/rotate-secret", url.PathEscape(webhookID))
	if err := c.Do(ctx, http.MethodPost, path, nil, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceWebhook)
	}
	return &result, nil
}

// Inbox webhook endpoints

// CreateInboxWebhook creates a new webhook for a specific inbox.
func (c *Client) CreateInboxWebhook(ctx context.Context, emailAddress string, req *CreateWebhookRequest) (*WebhookDTO, error) {
	var result WebhookDTO
	path := fmt.Sprintf("/api/inboxes/%s/webhooks", url.PathEscape(emailAddress))
	if err := c.Do(ctx, http.MethodPost, path, req, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceWebhook)
	}
	return &result, nil
}

// ListInboxWebhooks returns all webhooks for a specific inbox.
func (c *Client) ListInboxWebhooks(ctx context.Context, emailAddress string) (*WebhookListResponseDTO, error) {
	var result WebhookListResponseDTO
	path := fmt.Sprintf("/api/inboxes/%s/webhooks", url.PathEscape(emailAddress))
	if err := c.Do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceWebhook)
	}
	return &result, nil
}

// GetInboxWebhook returns a specific webhook for an inbox.
func (c *Client) GetInboxWebhook(ctx context.Context, emailAddress, webhookID string) (*WebhookDTO, error) {
	var result WebhookDTO
	path := fmt.Sprintf("/api/inboxes/%s/webhooks/%s", url.PathEscape(emailAddress), url.PathEscape(webhookID))
	if err := c.Do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceWebhook)
	}
	return &result, nil
}

// UpdateInboxWebhook updates a webhook for an inbox.
func (c *Client) UpdateInboxWebhook(ctx context.Context, emailAddress, webhookID string, req *UpdateWebhookRequest) (*WebhookDTO, error) {
	var result WebhookDTO
	path := fmt.Sprintf("/api/inboxes/%s/webhooks/%s", url.PathEscape(emailAddress), url.PathEscape(webhookID))
	if err := c.Do(ctx, http.MethodPatch, path, req, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceWebhook)
	}
	return &result, nil
}

// DeleteInboxWebhook deletes a webhook for an inbox.
func (c *Client) DeleteInboxWebhook(ctx context.Context, emailAddress, webhookID string) error {
	path := fmt.Sprintf("/api/inboxes/%s/webhooks/%s", url.PathEscape(emailAddress), url.PathEscape(webhookID))
	return apierrors.WithResourceType(c.Do(ctx, http.MethodDelete, path, nil, nil), apierrors.ResourceWebhook)
}

// TestInboxWebhook sends a test request to an inbox webhook.
func (c *Client) TestInboxWebhook(ctx context.Context, emailAddress, webhookID string) (*TestWebhookResponseDTO, error) {
	var result TestWebhookResponseDTO
	path := fmt.Sprintf("/api/inboxes/%s/webhooks/%s/test", url.PathEscape(emailAddress), url.PathEscape(webhookID))
	if err := c.Do(ctx, http.MethodPost, path, nil, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceWebhook)
	}
	return &result, nil
}

// RotateInboxWebhookSecret rotates the secret for an inbox webhook.
func (c *Client) RotateInboxWebhookSecret(ctx context.Context, emailAddress, webhookID string) (*RotateSecretResponseDTO, error) {
	var result RotateSecretResponseDTO
	path := fmt.Sprintf("/api/inboxes/%s/webhooks/%s/rotate-secret", url.PathEscape(emailAddress), url.PathEscape(webhookID))
	if err := c.Do(ctx, http.MethodPost, path, nil, &result); err != nil {
		return nil, apierrors.WithResourceType(err, apierrors.ResourceWebhook)
	}
	return &result, nil
}

// Utility endpoints

// GetWebhookTemplates returns all available webhook templates.
func (c *Client) GetWebhookTemplates(ctx context.Context) ([]*WebhookTemplateDTO, error) {
	var result WebhookTemplatesResponseDTO
	if err := c.Do(ctx, http.MethodGet, "/api/webhooks/templates", nil, &result); err != nil {
		return nil, err
	}
	return result.Templates, nil
}

// GetWebhookMetrics returns global webhook metrics.
func (c *Client) GetWebhookMetrics(ctx context.Context) (*WebhookMetricsDTO, error) {
	var result WebhookMetricsDTO
	if err := c.Do(ctx, http.MethodGet, "/api/webhooks/metrics", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
