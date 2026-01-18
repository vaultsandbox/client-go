package vaultsandbox

import (
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

// WebhookEventType represents the type of event that triggers a webhook.
type WebhookEventType string

const (
	// WebhookEventEmailReceived is triggered when an email is received.
	WebhookEventEmailReceived WebhookEventType = "email.received"
	// WebhookEventEmailStored is triggered when an email is stored.
	WebhookEventEmailStored WebhookEventType = "email.stored"
	// WebhookEventEmailDeleted is triggered when an email is deleted.
	WebhookEventEmailDeleted WebhookEventType = "email.deleted"
)

// WebhookScope represents the scope of a webhook.
type WebhookScope string

const (
	// WebhookScopeGlobal indicates a webhook that applies to all inboxes.
	WebhookScopeGlobal WebhookScope = "global"
	// WebhookScopeInbox indicates a webhook that applies to a specific inbox.
	WebhookScopeInbox WebhookScope = "inbox"
)

// FilterOperator represents the operator used in a filter rule.
type FilterOperator string

const (
	// FilterOperatorEquals matches when the field value equals the specified value.
	FilterOperatorEquals FilterOperator = "equals"
	// FilterOperatorContains matches when the field value contains the specified value.
	FilterOperatorContains FilterOperator = "contains"
	// FilterOperatorStartsWith matches when the field value starts with the specified value.
	FilterOperatorStartsWith FilterOperator = "starts_with"
	// FilterOperatorEndsWith matches when the field value ends with the specified value.
	FilterOperatorEndsWith FilterOperator = "ends_with"
	// FilterOperatorDomain matches when the email address is from the specified domain.
	FilterOperatorDomain FilterOperator = "domain"
	// FilterOperatorRegex matches when the field value matches the specified regex pattern.
	FilterOperatorRegex FilterOperator = "regex"
	// FilterOperatorExists matches when the field exists (for headers).
	FilterOperatorExists FilterOperator = "exists"
)

// FilterMode represents how multiple filter rules are combined.
type FilterMode string

const (
	// FilterModeAll requires all rules to match.
	FilterModeAll FilterMode = "all"
	// FilterModeAny requires at least one rule to match.
	FilterModeAny FilterMode = "any"
)

// FilterRule represents a single filter rule for webhook filtering.
type FilterRule struct {
	// Field is the email field to match against (e.g., "from", "subject", "to").
	Field string
	// Operator is the comparison operator to use.
	Operator FilterOperator
	// Value is the value to compare against.
	Value string
	// CaseSensitive indicates whether the comparison should be case-sensitive.
	CaseSensitive bool
}

// FilterConfig represents the filter configuration for a webhook.
type FilterConfig struct {
	// Rules is the list of filter rules.
	Rules []FilterRule
	// Mode determines how rules are combined (all or any).
	Mode FilterMode
	// RequireAuth requires that the email passes authentication checks.
	RequireAuth bool
}

// CustomTemplate represents a custom template for webhook payloads.
type CustomTemplate struct {
	// Body is the template body using Go template syntax.
	Body string
	// ContentType is the Content-Type header for the webhook request.
	ContentType string
}

// Webhook represents a webhook configuration.
type Webhook struct {
	// ID is the unique identifier for the webhook.
	ID string
	// URL is the endpoint that receives webhook notifications.
	URL string
	// Events is the list of event types that trigger this webhook.
	Events []WebhookEventType
	// Scope indicates whether the webhook is global or inbox-specific.
	Scope WebhookScope
	// InboxEmail is the inbox email address (only for inbox-scoped webhooks).
	InboxEmail string
	// Secret is the signing secret for verifying webhook payloads.
	Secret string
	// Template is the name of the built-in template (e.g., "slack", "discord").
	Template string
	// CustomTemplate is the custom template configuration.
	CustomTemplate *CustomTemplate
	// Filter is the filter configuration for this webhook.
	Filter *FilterConfig
	// Description is the optional description of the webhook.
	Description string
	// Enabled indicates whether the webhook is active.
	Enabled bool
	// Stats contains delivery statistics for this webhook.
	Stats *WebhookStats
	// CreatedAt is when the webhook was created.
	CreatedAt time.Time
	// UpdatedAt is when the webhook was last updated.
	UpdatedAt time.Time
}

// WebhookStats represents webhook delivery statistics.
type WebhookStats struct {
	// TotalDeliveries is the total number of delivery attempts.
	TotalDeliveries int
	// SuccessfulDeliveries is the number of successful deliveries.
	SuccessfulDeliveries int
	// FailedDeliveries is the number of failed deliveries.
	FailedDeliveries int
	// LastDeliveryAt is when the last delivery attempt was made.
	LastDeliveryAt *time.Time
	// LastSuccessAt is when the last successful delivery was made.
	LastSuccessAt *time.Time
	// LastFailureAt is when the last failed delivery was made.
	LastFailureAt *time.Time
}

// WebhookListResponse represents the response from listing webhooks.
type WebhookListResponse struct {
	// Webhooks is the list of webhooks.
	Webhooks []*Webhook
	// Total is the total number of webhooks.
	Total int
}

// TestWebhookResponse represents the response from testing a webhook.
type TestWebhookResponse struct {
	// Success indicates whether the test was successful.
	Success bool
	// StatusCode is the HTTP status code returned by the webhook endpoint.
	StatusCode int
	// ResponseTime is the response time in milliseconds.
	ResponseTime int
	// Error is the error message if the test failed.
	Error string
	// RequestID is the unique identifier for the test request.
	RequestID string
}

// RotateSecretResponse represents the response from rotating a webhook secret.
type RotateSecretResponse struct {
	// ID is the webhook ID.
	ID string
	// Secret is the new signing secret.
	Secret string
	// PreviousSecretValidUntil is when the previous secret will stop being valid.
	PreviousSecretValidUntil *time.Time
}

// WebhookTemplate represents a built-in webhook template.
type WebhookTemplate struct {
	// Label is the display name of the template.
	Label string
	// Value is the template identifier.
	Value string
}

// WebhookMetrics represents global webhook metrics.
type WebhookMetrics struct {
	// TotalWebhooks is the total number of registered webhooks.
	TotalWebhooks int
	// ActiveWebhooks is the number of enabled webhooks.
	ActiveWebhooks int
	// TotalDeliveries is the total number of delivery attempts.
	TotalDeliveries int
	// SuccessfulDeliveries is the number of successful deliveries.
	SuccessfulDeliveries int
	// FailedDeliveries is the number of failed deliveries.
	FailedDeliveries int
	// SuccessRate is the percentage of successful deliveries.
	SuccessRate float64
	// ByScope is the count of webhooks by scope.
	ByScope map[string]int
	// ByEvent is the count of webhooks by event type.
	ByEvent map[string]int
}

// webhookFromDTO converts an API DTO to a public Webhook type.
func webhookFromDTO(dto *api.WebhookDTO) *Webhook {
	if dto == nil {
		return nil
	}

	w := &Webhook{
		ID:          dto.ID,
		URL:         dto.URL,
		Scope:       WebhookScope(dto.Scope),
		InboxEmail:  dto.InboxEmail,
		Secret:      dto.Secret,
		Template:    dto.Template,
		Description: dto.Description,
		Enabled:     dto.Enabled,
		CreatedAt:   dto.CreatedAt,
		UpdatedAt:   dto.UpdatedAt,
	}

	// Convert events
	w.Events = make([]WebhookEventType, len(dto.Events))
	for i, e := range dto.Events {
		w.Events[i] = WebhookEventType(e)
	}

	// Convert custom template
	if dto.CustomTemplate != nil {
		w.CustomTemplate = &CustomTemplate{
			Body:        dto.CustomTemplate.Body,
			ContentType: dto.CustomTemplate.ContentType,
		}
	}

	// Convert filter
	if dto.Filter != nil {
		w.Filter = filterConfigFromDTO(dto.Filter)
	}

	// Convert stats
	if dto.Stats != nil {
		w.Stats = &WebhookStats{
			TotalDeliveries:      dto.Stats.TotalDeliveries,
			SuccessfulDeliveries: dto.Stats.SuccessfulDeliveries,
			FailedDeliveries:     dto.Stats.FailedDeliveries,
			LastDeliveryAt:       dto.Stats.LastDeliveryAt,
			LastSuccessAt:        dto.Stats.LastSuccessAt,
			LastFailureAt:        dto.Stats.LastFailureAt,
		}
	}

	return w
}

// filterConfigFromDTO converts an API DTO to a public FilterConfig type.
func filterConfigFromDTO(dto *api.FilterConfigDTO) *FilterConfig {
	if dto == nil {
		return nil
	}

	f := &FilterConfig{
		Mode:        FilterMode(dto.Mode),
		RequireAuth: dto.RequireAuth,
	}

	f.Rules = make([]FilterRule, len(dto.Rules))
	for i, r := range dto.Rules {
		f.Rules[i] = FilterRule{
			Field:         r.Field,
			Operator:      FilterOperator(r.Operator),
			Value:         r.Value,
			CaseSensitive: r.CaseSensitive,
		}
	}

	return f
}

// filterConfigToDTO converts a public FilterConfig to an API DTO.
func filterConfigToDTO(f *FilterConfig) *api.FilterConfigDTO {
	if f == nil {
		return nil
	}

	dto := &api.FilterConfigDTO{
		Mode:        string(f.Mode),
		RequireAuth: f.RequireAuth,
	}

	dto.Rules = make([]api.FilterRuleDTO, len(f.Rules))
	for i, r := range f.Rules {
		dto.Rules[i] = api.FilterRuleDTO{
			Field:         r.Field,
			Operator:      string(r.Operator),
			Value:         r.Value,
			CaseSensitive: r.CaseSensitive,
		}
	}

	return dto
}

// customTemplateToDTO converts a public CustomTemplate to an API DTO.
func customTemplateToDTO(t *CustomTemplate) *api.CustomTemplateDTO {
	if t == nil {
		return nil
	}
	return &api.CustomTemplateDTO{
		Body:        t.Body,
		ContentType: t.ContentType,
	}
}

// webhookListResponseFromDTO converts an API DTO to a public WebhookListResponse.
func webhookListResponseFromDTO(dto *api.WebhookListResponseDTO) *WebhookListResponse {
	if dto == nil {
		return nil
	}

	resp := &WebhookListResponse{
		Total: dto.Total,
	}

	resp.Webhooks = make([]*Webhook, len(dto.Webhooks))
	for i, w := range dto.Webhooks {
		resp.Webhooks[i] = webhookFromDTO(w)
	}

	return resp
}

// testWebhookResponseFromDTO converts an API DTO to a public TestWebhookResponse.
func testWebhookResponseFromDTO(dto *api.TestWebhookResponseDTO) *TestWebhookResponse {
	if dto == nil {
		return nil
	}
	return &TestWebhookResponse{
		Success:      dto.Success,
		StatusCode:   dto.StatusCode,
		ResponseTime: dto.ResponseTime,
		Error:        dto.Error,
		RequestID:    dto.RequestID,
	}
}

// rotateSecretResponseFromDTO converts an API DTO to a public RotateSecretResponse.
func rotateSecretResponseFromDTO(dto *api.RotateSecretResponseDTO) *RotateSecretResponse {
	if dto == nil {
		return nil
	}
	return &RotateSecretResponse{
		ID:                       dto.ID,
		Secret:                   dto.Secret,
		PreviousSecretValidUntil: dto.PreviousSecretValidUntil,
	}
}

// webhookTemplateFromDTO converts an API DTO to a public WebhookTemplate.
func webhookTemplateFromDTO(dto *api.WebhookTemplateDTO) *WebhookTemplate {
	if dto == nil {
		return nil
	}
	return &WebhookTemplate{
		Label: dto.Label,
		Value: dto.Value,
	}
}

// webhookMetricsFromDTO converts an API DTO to a public WebhookMetrics.
func webhookMetricsFromDTO(dto *api.WebhookMetricsDTO) *WebhookMetrics {
	if dto == nil {
		return nil
	}
	return &WebhookMetrics{
		TotalWebhooks:        dto.TotalWebhooks,
		ActiveWebhooks:       dto.ActiveWebhooks,
		TotalDeliveries:      dto.TotalDeliveries,
		SuccessfulDeliveries: dto.SuccessfulDeliveries,
		FailedDeliveries:     dto.FailedDeliveries,
		SuccessRate:          dto.SuccessRate,
		ByScope:              dto.ByScope,
		ByEvent:              dto.ByEvent,
	}
}
