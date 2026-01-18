package api

import "time"

// CreateWebhookRequest is the request body for creating a webhook.
type CreateWebhookRequest struct {
	URL            string              `json:"url"`
	Events         []string            `json:"events,omitempty"`
	Template       string              `json:"template,omitempty"`
	CustomTemplate *CustomTemplateDTO  `json:"customTemplate,omitempty"`
	Filter         *FilterConfigDTO    `json:"filter,omitempty"`
	Description    string              `json:"description,omitempty"`
}

// UpdateWebhookRequest is the request body for updating a webhook.
// All fields are optional - only provided fields will be updated.
type UpdateWebhookRequest struct {
	URL            *string             `json:"url,omitempty"`
	Events         []string            `json:"events,omitempty"`
	Template       *string             `json:"template,omitempty"`
	CustomTemplate *CustomTemplateDTO  `json:"customTemplate,omitempty"`
	Filter         *FilterConfigDTO    `json:"filter,omitempty"`
	ClearFilter    bool                `json:"-"` // Internal flag to set filter to null
	Description    *string             `json:"description,omitempty"`
	Enabled        *bool               `json:"enabled,omitempty"`
}

// FilterConfigDTO represents the filter configuration for a webhook.
type FilterConfigDTO struct {
	Rules       []FilterRuleDTO `json:"rules"`
	Mode        string          `json:"mode,omitempty"`
	RequireAuth bool            `json:"requireAuth,omitempty"`
}

// FilterRuleDTO represents a single filter rule.
type FilterRuleDTO struct {
	Field         string `json:"field"`
	Operator      string `json:"operator"`
	Value         string `json:"value,omitempty"`
	CaseSensitive bool   `json:"caseSensitive,omitempty"`
}

// CustomTemplateDTO represents a custom template for webhook payloads.
type CustomTemplateDTO struct {
	Body        string `json:"body"`
	ContentType string `json:"contentType,omitempty"`
}

// WebhookDTO represents a webhook from the API.
type WebhookDTO struct {
	ID             string              `json:"id"`
	URL            string              `json:"url"`
	Events         []string            `json:"events"`
	Scope          string              `json:"scope"`
	InboxEmail     string              `json:"inboxEmail,omitempty"`
	Secret         string              `json:"secret,omitempty"`
	Template       string              `json:"template,omitempty"`
	CustomTemplate *CustomTemplateDTO  `json:"customTemplate,omitempty"`
	Filter         *FilterConfigDTO    `json:"filter,omitempty"`
	Description    string              `json:"description,omitempty"`
	Enabled        bool                `json:"enabled"`
	Stats          *WebhookStatsDTO    `json:"stats,omitempty"`
	CreatedAt      time.Time           `json:"createdAt"`
	UpdatedAt      time.Time           `json:"updatedAt"`
}

// WebhookStatsDTO represents webhook delivery statistics.
type WebhookStatsDTO struct {
	TotalDeliveries      int        `json:"totalDeliveries"`
	SuccessfulDeliveries int        `json:"successfulDeliveries"`
	FailedDeliveries     int        `json:"failedDeliveries"`
	LastDeliveryAt       *time.Time `json:"lastDeliveryAt,omitempty"`
	LastSuccessAt        *time.Time `json:"lastSuccessAt,omitempty"`
	LastFailureAt        *time.Time `json:"lastFailureAt,omitempty"`
}

// WebhookListResponseDTO represents the response from listing webhooks.
type WebhookListResponseDTO struct {
	Webhooks []*WebhookDTO `json:"webhooks"`
	Total    int           `json:"total"`
}

// TestWebhookResponseDTO represents the response from testing a webhook.
type TestWebhookResponseDTO struct {
	Success      bool   `json:"success"`
	StatusCode   int    `json:"statusCode,omitempty"`
	ResponseTime int    `json:"responseTime,omitempty"`
	Error        string `json:"error,omitempty"`
	RequestID    string `json:"requestId,omitempty"`
}

// RotateSecretResponseDTO represents the response from rotating a webhook secret.
type RotateSecretResponseDTO struct {
	ID                       string     `json:"id"`
	Secret                   string     `json:"secret"`
	PreviousSecretValidUntil *time.Time `json:"previousSecretValidUntil,omitempty"`
}

// WebhookTemplateDTO represents a built-in webhook template.
type WebhookTemplateDTO struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// WebhookTemplatesResponseDTO represents the response from listing webhook templates.
type WebhookTemplatesResponseDTO struct {
	Templates []*WebhookTemplateDTO `json:"templates"`
}

// WebhookMetricsDTO represents global webhook metrics.
type WebhookMetricsDTO struct {
	TotalWebhooks        int                      `json:"totalWebhooks"`
	ActiveWebhooks       int                      `json:"activeWebhooks"`
	TotalDeliveries      int                      `json:"totalDeliveries"`
	SuccessfulDeliveries int                      `json:"successfulDeliveries"`
	FailedDeliveries     int                      `json:"failedDeliveries"`
	SuccessRate          float64                  `json:"successRate"`
	ByScope              map[string]int           `json:"byScope,omitempty"`
	ByEvent              map[string]int           `json:"byEvent,omitempty"`
}
