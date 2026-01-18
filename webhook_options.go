package vaultsandbox

import "github.com/vaultsandbox/client-go/internal/api"

// webhookCreateConfig holds configuration for creating a webhook.
type webhookCreateConfig struct {
	events         []WebhookEventType
	template       string
	customTemplate *CustomTemplate
	filter         *FilterConfig
	description    string
}

// webhookUpdateConfig holds configuration for updating a webhook.
type webhookUpdateConfig struct {
	url            *string
	events         []WebhookEventType
	template       *string
	customTemplate *CustomTemplate
	filter         *FilterConfig
	clearFilter    bool
	description    *string
	enabled        *bool
}

// WebhookCreateOption configures webhook creation.
type WebhookCreateOption func(*webhookCreateConfig)

// WebhookUpdateOption configures webhook updates.
type WebhookUpdateOption func(*webhookUpdateConfig)

// Create options

// WithWebhookEvents sets the event types that trigger the webhook.
func WithWebhookEvents(events ...WebhookEventType) WebhookCreateOption {
	return func(c *webhookCreateConfig) {
		c.events = events
	}
}

// WithWebhookTemplate sets a built-in template for the webhook payload.
// Common templates include "slack", "discord", "teams", "generic".
func WithWebhookTemplate(template string) WebhookCreateOption {
	return func(c *webhookCreateConfig) {
		c.template = template
	}
}

// WithWebhookCustomTemplate sets a custom template for the webhook payload.
// The body uses Go template syntax with access to email data.
func WithWebhookCustomTemplate(body, contentType string) WebhookCreateOption {
	return func(c *webhookCreateConfig) {
		c.customTemplate = &CustomTemplate{
			Body:        body,
			ContentType: contentType,
		}
	}
}

// WithWebhookFilter sets the filter configuration for the webhook.
// Only emails matching the filter will trigger the webhook.
func WithWebhookFilter(filter *FilterConfig) WebhookCreateOption {
	return func(c *webhookCreateConfig) {
		c.filter = filter
	}
}

// WithWebhookDescription sets the description for the webhook.
func WithWebhookDescription(description string) WebhookCreateOption {
	return func(c *webhookCreateConfig) {
		c.description = description
	}
}

// Update options

// WithUpdateURL updates the webhook URL.
func WithUpdateURL(url string) WebhookUpdateOption {
	return func(c *webhookUpdateConfig) {
		c.url = &url
	}
}

// WithUpdateEvents updates the event types that trigger the webhook.
func WithUpdateEvents(events ...WebhookEventType) WebhookUpdateOption {
	return func(c *webhookUpdateConfig) {
		c.events = events
	}
}

// WithUpdateTemplate updates the built-in template for the webhook payload.
func WithUpdateTemplate(template string) WebhookUpdateOption {
	return func(c *webhookUpdateConfig) {
		c.template = &template
	}
}

// WithUpdateCustomTemplate updates the custom template for the webhook payload.
func WithUpdateCustomTemplate(body, contentType string) WebhookUpdateOption {
	return func(c *webhookUpdateConfig) {
		c.customTemplate = &CustomTemplate{
			Body:        body,
			ContentType: contentType,
		}
	}
}

// WithUpdateFilter updates the filter configuration for the webhook.
func WithUpdateFilter(filter *FilterConfig) WebhookUpdateOption {
	return func(c *webhookUpdateConfig) {
		c.filter = filter
	}
}

// WithClearFilter removes the filter from the webhook.
func WithClearFilter() WebhookUpdateOption {
	return func(c *webhookUpdateConfig) {
		c.clearFilter = true
	}
}

// WithUpdateDescription updates the description for the webhook.
func WithUpdateDescription(description string) WebhookUpdateOption {
	return func(c *webhookUpdateConfig) {
		c.description = &description
	}
}

// WithUpdateEnabled enables or disables the webhook.
func WithUpdateEnabled(enabled bool) WebhookUpdateOption {
	return func(c *webhookUpdateConfig) {
		c.enabled = &enabled
	}
}

// buildCreateRequest builds an API request from create options.
func buildCreateRequest(url string, opts []WebhookCreateOption) *api.CreateWebhookRequest {
	cfg := &webhookCreateConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	req := &api.CreateWebhookRequest{
		URL:         url,
		Template:    cfg.template,
		Description: cfg.description,
	}

	// Convert events
	if len(cfg.events) > 0 {
		req.Events = make([]string, len(cfg.events))
		for i, e := range cfg.events {
			req.Events[i] = string(e)
		}
	}

	// Convert custom template
	if cfg.customTemplate != nil {
		req.CustomTemplate = customTemplateToDTO(cfg.customTemplate)
	}

	// Convert filter
	if cfg.filter != nil {
		req.Filter = filterConfigToDTO(cfg.filter)
	}

	return req
}

// buildUpdateRequest builds an API request from update options.
func buildUpdateRequest(opts []WebhookUpdateOption) *api.UpdateWebhookRequest {
	cfg := &webhookUpdateConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	req := &api.UpdateWebhookRequest{
		URL:         cfg.url,
		Template:    cfg.template,
		Description: cfg.description,
		Enabled:     cfg.enabled,
		ClearFilter: cfg.clearFilter,
	}

	// Convert events
	if cfg.events != nil {
		req.Events = make([]string, len(cfg.events))
		for i, e := range cfg.events {
			req.Events[i] = string(e)
		}
	}

	// Convert custom template
	if cfg.customTemplate != nil {
		req.CustomTemplate = customTemplateToDTO(cfg.customTemplate)
	}

	// Convert filter
	if cfg.filter != nil {
		req.Filter = filterConfigToDTO(cfg.filter)
	}

	return req
}
