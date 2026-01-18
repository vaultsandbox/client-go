package vaultsandbox

import "context"

// CreateWebhook creates a new webhook for this inbox.
// Inbox webhooks only receive notifications for emails sent to this specific inbox.
func (i *Inbox) CreateWebhook(ctx context.Context, url string, opts ...WebhookCreateOption) (*Webhook, error) {
	if err := i.client.checkClosed(); err != nil {
		return nil, err
	}

	req := buildCreateRequest(url, opts)
	dto, err := i.client.apiClient.CreateInboxWebhook(ctx, i.emailAddress, req)
	if err != nil {
		return nil, err
	}

	return webhookFromDTO(dto), nil
}

// ListWebhooks returns all webhooks configured for this inbox.
func (i *Inbox) ListWebhooks(ctx context.Context) (*WebhookListResponse, error) {
	if err := i.client.checkClosed(); err != nil {
		return nil, err
	}

	dto, err := i.client.apiClient.ListInboxWebhooks(ctx, i.emailAddress)
	if err != nil {
		return nil, err
	}

	return webhookListResponseFromDTO(dto), nil
}

// GetWebhook returns a specific webhook by ID.
func (i *Inbox) GetWebhook(ctx context.Context, webhookID string) (*Webhook, error) {
	if err := i.client.checkClosed(); err != nil {
		return nil, err
	}

	dto, err := i.client.apiClient.GetInboxWebhook(ctx, i.emailAddress, webhookID)
	if err != nil {
		return nil, err
	}

	return webhookFromDTO(dto), nil
}

// UpdateWebhook updates a webhook for this inbox.
func (i *Inbox) UpdateWebhook(ctx context.Context, webhookID string, opts ...WebhookUpdateOption) (*Webhook, error) {
	if err := i.client.checkClosed(); err != nil {
		return nil, err
	}

	req := buildUpdateRequest(opts)
	dto, err := i.client.apiClient.UpdateInboxWebhook(ctx, i.emailAddress, webhookID, req)
	if err != nil {
		return nil, err
	}

	return webhookFromDTO(dto), nil
}

// DeleteWebhook deletes a webhook from this inbox.
func (i *Inbox) DeleteWebhook(ctx context.Context, webhookID string) error {
	if err := i.client.checkClosed(); err != nil {
		return err
	}

	return i.client.apiClient.DeleteInboxWebhook(ctx, i.emailAddress, webhookID)
}

// TestWebhook sends a test request to a webhook.
func (i *Inbox) TestWebhook(ctx context.Context, webhookID string) (*TestWebhookResponse, error) {
	if err := i.client.checkClosed(); err != nil {
		return nil, err
	}

	dto, err := i.client.apiClient.TestInboxWebhook(ctx, i.emailAddress, webhookID)
	if err != nil {
		return nil, err
	}

	return testWebhookResponseFromDTO(dto), nil
}

// RotateWebhookSecret rotates the signing secret for a webhook.
// The previous secret remains valid for a grace period to allow for seamless rotation.
func (i *Inbox) RotateWebhookSecret(ctx context.Context, webhookID string) (*RotateSecretResponse, error) {
	if err := i.client.checkClosed(); err != nil {
		return nil, err
	}

	dto, err := i.client.apiClient.RotateInboxWebhookSecret(ctx, i.emailAddress, webhookID)
	if err != nil {
		return nil, err
	}

	return rotateSecretResponseFromDTO(dto), nil
}
