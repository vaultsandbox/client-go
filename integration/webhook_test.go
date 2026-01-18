//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
)

func TestIntegration_WebhookTemplates(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	templates, err := client.GetWebhookTemplates(ctx)
	if err != nil {
		t.Fatalf("GetWebhookTemplates() error = %v", err)
	}

	t.Logf("Found %d webhook templates", len(templates))
	for _, tmpl := range templates {
		t.Logf("  Template: %s (%s)", tmpl.Label, tmpl.Value)
	}

	if len(templates) == 0 {
		t.Error("GetWebhookTemplates() returned empty list")
	}
}

func TestIntegration_WebhookMetrics(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	metrics, err := client.GetWebhookMetrics(ctx)
	if err != nil {
		t.Fatalf("GetWebhookMetrics() error = %v", err)
	}

	t.Logf("Webhook metrics: Total=%d, Active=%d, Deliveries=%d, SuccessRate=%.2f%%",
		metrics.TotalWebhooks, metrics.ActiveWebhooks, metrics.TotalDeliveries, metrics.SuccessRate)
}

func TestIntegration_GlobalWebhook_CRUD(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	admin := client.Admin()

	// Create a global webhook
	webhook, err := admin.CreateWebhook(ctx, "https://example.com/webhook",
		vaultsandbox.WithWebhookEvents(vaultsandbox.WebhookEventEmailReceived),
		vaultsandbox.WithWebhookDescription("Test global webhook"),
	)
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}
	t.Logf("Created global webhook: ID=%s, URL=%s", webhook.ID, webhook.URL)

	// Cleanup
	defer func() {
		if err := admin.DeleteWebhook(ctx, webhook.ID); err != nil {
			t.Errorf("DeleteWebhook() cleanup error = %v", err)
		}
	}()

	// Verify created webhook properties
	if webhook.ID == "" {
		t.Error("webhook.ID is empty")
	}
	if webhook.URL != "https://example.com/webhook" {
		t.Errorf("webhook.URL = %s, want https://example.com/webhook", webhook.URL)
	}
	if webhook.Scope != vaultsandbox.WebhookScopeGlobal {
		t.Errorf("webhook.Scope = %s, want global", webhook.Scope)
	}
	if webhook.Description != "Test global webhook" {
		t.Errorf("webhook.Description = %s, want 'Test global webhook'", webhook.Description)
	}
	if !webhook.Enabled {
		t.Error("webhook.Enabled = false, want true")
	}
	if webhook.Secret == "" {
		t.Error("webhook.Secret is empty")
	}
	if len(webhook.Events) != 1 || webhook.Events[0] != vaultsandbox.WebhookEventEmailReceived {
		t.Errorf("webhook.Events = %v, want [email.received]", webhook.Events)
	}

	// Get the webhook
	got, err := admin.GetWebhook(ctx, webhook.ID)
	if err != nil {
		t.Fatalf("GetWebhook() error = %v", err)
	}
	if got.ID != webhook.ID {
		t.Errorf("GetWebhook() ID = %s, want %s", got.ID, webhook.ID)
	}

	// List webhooks
	list, err := admin.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("ListWebhooks() error = %v", err)
	}
	t.Logf("ListWebhooks() returned %d webhooks (total: %d)", len(list.Webhooks), list.Total)

	found := false
	for _, w := range list.Webhooks {
		if w.ID == webhook.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Created webhook not found in list")
	}

	// Update the webhook
	updated, err := admin.UpdateWebhook(ctx, webhook.ID,
		vaultsandbox.WithUpdateDescription("Updated description"),
		vaultsandbox.WithUpdateEvents(vaultsandbox.WebhookEventEmailReceived, vaultsandbox.WebhookEventEmailDeleted),
	)
	if err != nil {
		t.Fatalf("UpdateWebhook() error = %v", err)
	}
	if updated.Description != "Updated description" {
		t.Errorf("updated.Description = %s, want 'Updated description'", updated.Description)
	}
	if len(updated.Events) != 2 {
		t.Errorf("updated.Events length = %d, want 2", len(updated.Events))
	}

	// Disable the webhook
	disabled, err := admin.UpdateWebhook(ctx, webhook.ID,
		vaultsandbox.WithUpdateEnabled(false),
	)
	if err != nil {
		t.Fatalf("UpdateWebhook() disable error = %v", err)
	}
	if disabled.Enabled {
		t.Error("disabled.Enabled = true, want false")
	}
}

func TestIntegration_GlobalWebhook_WithTemplate(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	admin := client.Admin()

	// Get available templates first
	templates, err := client.GetWebhookTemplates(ctx)
	if err != nil {
		t.Fatalf("GetWebhookTemplates() error = %v", err)
	}
	if len(templates) == 0 {
		t.Skip("No webhook templates available")
	}

	// Create webhook with a template
	templateValue := templates[0].Value
	webhook, err := admin.CreateWebhook(ctx, "https://example.com/slack-webhook",
		vaultsandbox.WithWebhookEvents(vaultsandbox.WebhookEventEmailReceived),
		vaultsandbox.WithWebhookTemplate(templateValue),
	)
	if err != nil {
		t.Fatalf("CreateWebhook() with template error = %v", err)
	}
	defer admin.DeleteWebhook(ctx, webhook.ID)

	t.Logf("Created webhook with template: ID=%s, Template=%s", webhook.ID, webhook.Template)

	if webhook.Template != templateValue {
		t.Errorf("webhook.Template = %s, want %s", webhook.Template, templateValue)
	}
}

func TestIntegration_GlobalWebhook_WithCustomTemplate(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	admin := client.Admin()

	customBody := `{"text": "New email from {{.From}}: {{.Subject}}"}`
	webhook, err := admin.CreateWebhook(ctx, "https://example.com/custom-webhook",
		vaultsandbox.WithWebhookEvents(vaultsandbox.WebhookEventEmailReceived),
		vaultsandbox.WithWebhookCustomTemplate(customBody, "application/json"),
	)
	if err != nil {
		// Custom templates might not be supported by the server
		t.Skipf("CreateWebhook() with custom template not supported: %v", err)
	}
	defer admin.DeleteWebhook(ctx, webhook.ID)

	t.Logf("Created webhook with custom template: ID=%s", webhook.ID)

	if webhook.CustomTemplate == nil {
		t.Fatal("webhook.CustomTemplate is nil")
	}
	if webhook.CustomTemplate.Body != customBody {
		t.Errorf("webhook.CustomTemplate.Body = %s, want %s", webhook.CustomTemplate.Body, customBody)
	}
	if webhook.CustomTemplate.ContentType != "application/json" {
		t.Errorf("webhook.CustomTemplate.ContentType = %s, want application/json", webhook.CustomTemplate.ContentType)
	}
}

func TestIntegration_GlobalWebhook_WithFilter(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	admin := client.Admin()

	filter := &vaultsandbox.FilterConfig{
		Rules: []vaultsandbox.FilterRule{
			{
				Field:    "from",
				Operator: vaultsandbox.FilterOperatorContains,
				Value:    "important",
			},
			{
				Field:    "subject",
				Operator: vaultsandbox.FilterOperatorStartsWith,
				Value:    "[URGENT]",
			},
		},
		Mode: vaultsandbox.FilterModeAny,
	}

	webhook, err := admin.CreateWebhook(ctx, "https://example.com/filtered-webhook",
		vaultsandbox.WithWebhookEvents(vaultsandbox.WebhookEventEmailReceived),
		vaultsandbox.WithWebhookFilter(filter),
	)
	if err != nil {
		// Filters might not be supported on global webhooks
		t.Skipf("CreateWebhook() with filter not supported on global webhooks: %v", err)
	}
	defer admin.DeleteWebhook(ctx, webhook.ID)

	t.Logf("Created webhook with filter: ID=%s", webhook.ID)

	if webhook.Filter == nil {
		t.Fatal("webhook.Filter is nil")
	}
	if len(webhook.Filter.Rules) != 2 {
		t.Errorf("webhook.Filter.Rules length = %d, want 2", len(webhook.Filter.Rules))
	}
	if webhook.Filter.Mode != vaultsandbox.FilterModeAny {
		t.Errorf("webhook.Filter.Mode = %s, want any", webhook.Filter.Mode)
	}

	// Update to clear the filter
	updated, err := admin.UpdateWebhook(ctx, webhook.ID,
		vaultsandbox.WithClearFilter(),
	)
	if err != nil {
		t.Fatalf("UpdateWebhook() clear filter error = %v", err)
	}
	if updated.Filter != nil {
		t.Error("updated.Filter should be nil after clearing")
	}
}

func TestIntegration_GlobalWebhook_RotateSecret(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	admin := client.Admin()

	webhook, err := admin.CreateWebhook(ctx, "https://example.com/rotate-test",
		vaultsandbox.WithWebhookEvents(vaultsandbox.WebhookEventEmailReceived),
	)
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}
	defer admin.DeleteWebhook(ctx, webhook.ID)

	originalSecret := webhook.Secret
	t.Logf("Original secret: %s...", originalSecret[:10])

	// Rotate the secret
	rotated, err := admin.RotateWebhookSecret(ctx, webhook.ID)
	if err != nil {
		t.Fatalf("RotateWebhookSecret() error = %v", err)
	}

	t.Logf("New secret: %s...", rotated.Secret[:10])
	if rotated.PreviousSecretValidUntil != nil {
		t.Logf("Previous secret valid until: %v", rotated.PreviousSecretValidUntil)
	}

	if rotated.ID != webhook.ID {
		t.Errorf("rotated.ID = %s, want %s", rotated.ID, webhook.ID)
	}
	if rotated.Secret == "" {
		t.Error("rotated.Secret is empty")
	}
	if rotated.Secret == originalSecret {
		t.Error("rotated.Secret should be different from original")
	}
}

func TestIntegration_GlobalWebhook_TestEndpoint(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	admin := client.Admin()

	// Create webhook pointing to httpbin
	webhook, err := admin.CreateWebhook(ctx, "https://httpbin.org/post",
		vaultsandbox.WithWebhookEvents(vaultsandbox.WebhookEventEmailReceived),
	)
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}
	defer admin.DeleteWebhook(ctx, webhook.ID)

	// Test the webhook
	result, err := admin.TestWebhook(ctx, webhook.ID)
	if err != nil {
		t.Fatalf("TestWebhook() error = %v", err)
	}

	t.Logf("Test result: Success=%v, StatusCode=%d, ResponseTime=%dms, RequestID=%s",
		result.Success, result.StatusCode, result.ResponseTime, result.RequestID)
}

func TestIntegration_GlobalWebhook_NotFound(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()
	admin := client.Admin()

	_, err := admin.GetWebhook(ctx, "nonexistent-webhook-id")
	if err == nil {
		t.Fatal("GetWebhook() expected error for nonexistent webhook")
	}

	if !errors.Is(err, vaultsandbox.ErrWebhookNotFound) {
		t.Errorf("GetWebhook() error = %v, want ErrWebhookNotFound", err)
	}
}

func TestIntegration_InboxWebhook_CRUD(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	// Create an inbox first
	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Created inbox: %s", inbox.EmailAddress())

	// Create a webhook for the inbox
	webhook, err := inbox.CreateWebhook(ctx, "https://example.com/inbox-webhook",
		vaultsandbox.WithWebhookEvents(vaultsandbox.WebhookEventEmailReceived, vaultsandbox.WebhookEventEmailStored),
		vaultsandbox.WithWebhookDescription("Test inbox webhook"),
	)
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}
	t.Logf("Created inbox webhook: ID=%s, URL=%s", webhook.ID, webhook.URL)

	// Cleanup
	defer func() {
		if err := inbox.DeleteWebhook(ctx, webhook.ID); err != nil {
			t.Errorf("DeleteWebhook() cleanup error = %v", err)
		}
	}()

	// Verify created webhook properties
	if webhook.ID == "" {
		t.Error("webhook.ID is empty")
	}
	if webhook.Scope != vaultsandbox.WebhookScopeInbox {
		t.Errorf("webhook.Scope = %s, want inbox", webhook.Scope)
	}
	if webhook.InboxEmail != inbox.EmailAddress() {
		t.Errorf("webhook.InboxEmail = %s, want %s", webhook.InboxEmail, inbox.EmailAddress())
	}
	if len(webhook.Events) != 2 {
		t.Errorf("webhook.Events length = %d, want 2", len(webhook.Events))
	}

	// Get the webhook
	got, err := inbox.GetWebhook(ctx, webhook.ID)
	if err != nil {
		t.Fatalf("GetWebhook() error = %v", err)
	}
	if got.ID != webhook.ID {
		t.Errorf("GetWebhook() ID = %s, want %s", got.ID, webhook.ID)
	}

	// List webhooks for inbox
	list, err := inbox.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("ListWebhooks() error = %v", err)
	}
	t.Logf("ListWebhooks() returned %d webhooks (total: %d)", len(list.Webhooks), list.Total)

	found := false
	for _, w := range list.Webhooks {
		if w.ID == webhook.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Created webhook not found in list")
	}

	// Update the webhook
	updated, err := inbox.UpdateWebhook(ctx, webhook.ID,
		vaultsandbox.WithUpdateURL("https://example.com/updated-inbox-webhook"),
		vaultsandbox.WithUpdateDescription("Updated inbox webhook"),
	)
	if err != nil {
		t.Fatalf("UpdateWebhook() error = %v", err)
	}
	if updated.URL != "https://example.com/updated-inbox-webhook" {
		t.Errorf("updated.URL = %s, want https://example.com/updated-inbox-webhook", updated.URL)
	}
	if updated.Description != "Updated inbox webhook" {
		t.Errorf("updated.Description = %s, want 'Updated inbox webhook'", updated.Description)
	}
}

func TestIntegration_InboxWebhook_RotateSecret(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	webhook, err := inbox.CreateWebhook(ctx, "https://example.com/inbox-rotate-test",
		vaultsandbox.WithWebhookEvents(vaultsandbox.WebhookEventEmailReceived),
	)
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}
	defer inbox.DeleteWebhook(ctx, webhook.ID)

	originalSecret := webhook.Secret

	// Rotate the secret
	rotated, err := inbox.RotateWebhookSecret(ctx, webhook.ID)
	if err != nil {
		t.Fatalf("RotateWebhookSecret() error = %v", err)
	}

	if rotated.Secret == originalSecret {
		t.Error("rotated.Secret should be different from original")
	}
	t.Logf("Secret rotated successfully")
}

func TestIntegration_InboxWebhook_TestEndpoint(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	webhook, err := inbox.CreateWebhook(ctx, "https://httpbin.org/post",
		vaultsandbox.WithWebhookEvents(vaultsandbox.WebhookEventEmailReceived),
	)
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}
	defer inbox.DeleteWebhook(ctx, webhook.ID)

	// Test the webhook
	result, err := inbox.TestWebhook(ctx, webhook.ID)
	if err != nil {
		t.Fatalf("TestWebhook() error = %v", err)
	}

	t.Logf("Test result: Success=%v, StatusCode=%d, ResponseTime=%dms",
		result.Success, result.StatusCode, result.ResponseTime)
}

func TestIntegration_InboxWebhook_NotFound(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	_, err = inbox.GetWebhook(ctx, "nonexistent-webhook-id")
	if err == nil {
		t.Fatal("GetWebhook() expected error for nonexistent webhook")
	}

	if !errors.Is(err, vaultsandbox.ErrWebhookNotFound) {
		t.Errorf("GetWebhook() error = %v, want ErrWebhookNotFound", err)
	}
}

func TestIntegration_InboxWebhook_WithFilter(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	filter := &vaultsandbox.FilterConfig{
		Rules: []vaultsandbox.FilterRule{
			{
				Field:         "subject",
				Operator:      vaultsandbox.FilterOperatorRegex,
				Value:         "^\\[ALERT\\]",
				CaseSensitive: true,
			},
		},
		Mode:        vaultsandbox.FilterModeAll,
		RequireAuth: true,
	}

	webhook, err := inbox.CreateWebhook(ctx, "https://example.com/filtered-inbox-webhook",
		vaultsandbox.WithWebhookEvents(vaultsandbox.WebhookEventEmailReceived),
		vaultsandbox.WithWebhookFilter(filter),
	)
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}
	defer inbox.DeleteWebhook(ctx, webhook.ID)

	if webhook.Filter == nil {
		t.Fatal("webhook.Filter is nil")
	}
	if len(webhook.Filter.Rules) != 1 {
		t.Errorf("webhook.Filter.Rules length = %d, want 1", len(webhook.Filter.Rules))
	}
	if webhook.Filter.Mode != vaultsandbox.FilterModeAll {
		t.Errorf("webhook.Filter.Mode = %s, want all", webhook.Filter.Mode)
	}
	if !webhook.Filter.RequireAuth {
		t.Error("webhook.Filter.RequireAuth = false, want true")
	}

	rule := webhook.Filter.Rules[0]
	if rule.Operator != vaultsandbox.FilterOperatorRegex {
		t.Errorf("rule.Operator = %s, want regex", rule.Operator)
	}
	if !rule.CaseSensitive {
		t.Error("rule.CaseSensitive = false, want true")
	}
}

func TestIntegration_MultipleInboxWebhooks(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Create multiple webhooks for the same inbox
	const numWebhooks = 3
	webhooks := make([]*vaultsandbox.Webhook, numWebhooks)

	for i := 0; i < numWebhooks; i++ {
		w, err := inbox.CreateWebhook(ctx, "https://example.com/webhook-"+string(rune('a'+i)),
			vaultsandbox.WithWebhookEvents(vaultsandbox.WebhookEventEmailReceived),
			vaultsandbox.WithWebhookDescription("Webhook "+string(rune('A'+i))),
		)
		if err != nil {
			t.Fatalf("CreateWebhook() %d error = %v", i, err)
		}
		webhooks[i] = w
		t.Logf("Created webhook %d: %s", i, w.ID)
	}

	// Cleanup
	defer func() {
		for _, w := range webhooks {
			inbox.DeleteWebhook(ctx, w.ID)
		}
	}()

	// List and verify all webhooks
	list, err := inbox.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("ListWebhooks() error = %v", err)
	}

	if len(list.Webhooks) < numWebhooks {
		t.Errorf("ListWebhooks() returned %d, want at least %d", len(list.Webhooks), numWebhooks)
	}

	// Verify each created webhook is in the list
	for _, created := range webhooks {
		found := false
		for _, listed := range list.Webhooks {
			if listed.ID == created.ID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Webhook %s not found in list", created.ID)
		}
	}
}
