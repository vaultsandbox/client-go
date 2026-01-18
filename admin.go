package vaultsandbox

import "context"

// Admin provides access to administrative operations such as global webhook management.
type Admin interface {
	// CreateWebhook creates a new global webhook.
	// Global webhooks receive notifications for all inboxes associated with the API key.
	CreateWebhook(ctx context.Context, url string, opts ...WebhookCreateOption) (*Webhook, error)

	// ListWebhooks returns all global webhooks.
	ListWebhooks(ctx context.Context) (*WebhookListResponse, error)

	// GetWebhook returns a specific global webhook by ID.
	GetWebhook(ctx context.Context, webhookID string) (*Webhook, error)

	// UpdateWebhook updates a global webhook.
	UpdateWebhook(ctx context.Context, webhookID string, opts ...WebhookUpdateOption) (*Webhook, error)

	// DeleteWebhook deletes a global webhook.
	DeleteWebhook(ctx context.Context, webhookID string) error

	// TestWebhook sends a test request to a global webhook.
	TestWebhook(ctx context.Context, webhookID string) (*TestWebhookResponse, error)

	// RotateWebhookSecret rotates the signing secret for a global webhook.
	// The previous secret remains valid for a grace period to allow for seamless rotation.
	RotateWebhookSecret(ctx context.Context, webhookID string) (*RotateSecretResponse, error)
}

// adminImpl implements the Admin interface.
type adminImpl struct {
	client *Client
}

// Admin returns an Admin interface for managing global webhooks and other administrative operations.
func (c *Client) Admin() Admin {
	return &adminImpl{client: c}
}

// CreateWebhook creates a new global webhook.
func (a *adminImpl) CreateWebhook(ctx context.Context, url string, opts ...WebhookCreateOption) (*Webhook, error) {
	if err := a.client.checkClosed(); err != nil {
		return nil, err
	}

	req := buildCreateRequest(url, opts)
	dto, err := a.client.apiClient.CreateGlobalWebhook(ctx, req)
	if err != nil {
		return nil, err
	}

	return webhookFromDTO(dto), nil
}

// ListWebhooks returns all global webhooks.
func (a *adminImpl) ListWebhooks(ctx context.Context) (*WebhookListResponse, error) {
	if err := a.client.checkClosed(); err != nil {
		return nil, err
	}

	dto, err := a.client.apiClient.ListGlobalWebhooks(ctx)
	if err != nil {
		return nil, err
	}

	return webhookListResponseFromDTO(dto), nil
}

// GetWebhook returns a specific global webhook by ID.
func (a *adminImpl) GetWebhook(ctx context.Context, webhookID string) (*Webhook, error) {
	if err := a.client.checkClosed(); err != nil {
		return nil, err
	}

	dto, err := a.client.apiClient.GetGlobalWebhook(ctx, webhookID)
	if err != nil {
		return nil, err
	}

	return webhookFromDTO(dto), nil
}

// UpdateWebhook updates a global webhook.
func (a *adminImpl) UpdateWebhook(ctx context.Context, webhookID string, opts ...WebhookUpdateOption) (*Webhook, error) {
	if err := a.client.checkClosed(); err != nil {
		return nil, err
	}

	req := buildUpdateRequest(opts)
	dto, err := a.client.apiClient.UpdateGlobalWebhook(ctx, webhookID, req)
	if err != nil {
		return nil, err
	}

	return webhookFromDTO(dto), nil
}

// DeleteWebhook deletes a global webhook.
func (a *adminImpl) DeleteWebhook(ctx context.Context, webhookID string) error {
	if err := a.client.checkClosed(); err != nil {
		return err
	}

	return a.client.apiClient.DeleteGlobalWebhook(ctx, webhookID)
}

// TestWebhook sends a test request to a global webhook.
func (a *adminImpl) TestWebhook(ctx context.Context, webhookID string) (*TestWebhookResponse, error) {
	if err := a.client.checkClosed(); err != nil {
		return nil, err
	}

	dto, err := a.client.apiClient.TestGlobalWebhook(ctx, webhookID)
	if err != nil {
		return nil, err
	}

	return testWebhookResponseFromDTO(dto), nil
}

// RotateWebhookSecret rotates the signing secret for a global webhook.
func (a *adminImpl) RotateWebhookSecret(ctx context.Context, webhookID string) (*RotateSecretResponse, error) {
	if err := a.client.checkClosed(); err != nil {
		return nil, err
	}

	dto, err := a.client.apiClient.RotateGlobalWebhookSecret(ctx, webhookID)
	if err != nil {
		return nil, err
	}

	return rotateSecretResponseFromDTO(dto), nil
}
