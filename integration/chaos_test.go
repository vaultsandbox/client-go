//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	vaultsandbox "github.com/vaultsandbox/client-go"
)

func TestIntegration_ChaosConfig_ServerCheck(t *testing.T) {
	client := newClient(t)

	info := client.ServerInfo()
	t.Logf("Server chaos enabled: %v", info.ChaosEnabled)

	if !info.ChaosEnabled {
		t.Skip("Chaos is not enabled on this server")
	}
}

func TestIntegration_ChaosConfig_GetDefault(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	if !client.ServerInfo().ChaosEnabled {
		t.Skip("Chaos is not enabled on this server")
	}

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Get default chaos config (should be disabled)
	config, err := inbox.GetChaosConfig(ctx)
	if err != nil {
		t.Fatalf("GetChaosConfig() error = %v", err)
	}

	t.Logf("Default chaos config: Enabled=%v", config.Enabled)

	if config.Enabled {
		t.Error("Expected chaos to be disabled by default")
	}
}

func TestIntegration_ChaosConfig_SetAndGet(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	if !client.ServerInfo().ChaosEnabled {
		t.Skip("Chaos is not enabled on this server")
	}

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Set chaos config with latency
	config := &vaultsandbox.ChaosConfig{
		Enabled: true,
		Latency: &vaultsandbox.LatencyConfig{
			Enabled:     true,
			MinDelayMs:  100,
			MaxDelayMs:  500,
			Jitter:      true,
			Probability: 0.5,
		},
	}

	result, err := inbox.SetChaosConfig(ctx, config)
	if err != nil {
		t.Fatalf("SetChaosConfig() error = %v", err)
	}

	t.Logf("Set chaos config: Enabled=%v, Latency.Enabled=%v", result.Enabled, result.Latency != nil && result.Latency.Enabled)

	if !result.Enabled {
		t.Error("Expected chaos to be enabled")
	}
	if result.Latency == nil {
		t.Fatal("Expected latency config to be set")
	}
	if !result.Latency.Enabled {
		t.Error("Expected latency to be enabled")
	}

	// Get and verify
	got, err := inbox.GetChaosConfig(ctx)
	if err != nil {
		t.Fatalf("GetChaosConfig() error = %v", err)
	}

	if !got.Enabled {
		t.Error("GetChaosConfig: Expected chaos to be enabled")
	}
	if got.Latency == nil || !got.Latency.Enabled {
		t.Error("GetChaosConfig: Expected latency to be enabled")
	}
}

func TestIntegration_ChaosConfig_LatencyConfig(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	if !client.ServerInfo().ChaosEnabled {
		t.Skip("Chaos is not enabled on this server")
	}

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	config := &vaultsandbox.ChaosConfig{
		Enabled: true,
		Latency: &vaultsandbox.LatencyConfig{
			Enabled:     true,
			MinDelayMs:  1000,
			MaxDelayMs:  5000,
			Jitter:      true,
			Probability: 0.8,
		},
	}

	result, err := inbox.SetChaosConfig(ctx, config)
	if err != nil {
		t.Fatalf("SetChaosConfig() error = %v", err)
	}

	if result.Latency == nil {
		t.Fatal("Latency config is nil")
	}

	t.Logf("Latency config: MinDelayMs=%d, MaxDelayMs=%d, Jitter=%v, Probability=%.2f",
		result.Latency.MinDelayMs, result.Latency.MaxDelayMs, result.Latency.Jitter, result.Latency.Probability)
}

func TestIntegration_ChaosConfig_ConnectionDrop(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	if !client.ServerInfo().ChaosEnabled {
		t.Skip("Chaos is not enabled on this server")
	}

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	config := &vaultsandbox.ChaosConfig{
		Enabled: true,
		ConnectionDrop: &vaultsandbox.ConnectionDropConfig{
			Enabled:     true,
			Probability: 0.3,
			Graceful:    false,
		},
	}

	result, err := inbox.SetChaosConfig(ctx, config)
	if err != nil {
		t.Fatalf("SetChaosConfig() error = %v", err)
	}

	if result.ConnectionDrop == nil {
		t.Fatal("ConnectionDrop config is nil")
	}

	t.Logf("ConnectionDrop config: Probability=%.2f, Graceful=%v",
		result.ConnectionDrop.Probability, result.ConnectionDrop.Graceful)

	if !result.ConnectionDrop.Enabled {
		t.Error("Expected connection drop to be enabled")
	}
}

func TestIntegration_ChaosConfig_RandomError(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	if !client.ServerInfo().ChaosEnabled {
		t.Skip("Chaos is not enabled on this server")
	}

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	config := &vaultsandbox.ChaosConfig{
		Enabled: true,
		RandomError: &vaultsandbox.RandomErrorConfig{
			Enabled:    true,
			ErrorRate:  0.2,
			ErrorTypes: []vaultsandbox.RandomErrorType{vaultsandbox.RandomErrorTypeTemporary},
		},
	}

	result, err := inbox.SetChaosConfig(ctx, config)
	if err != nil {
		t.Fatalf("SetChaosConfig() error = %v", err)
	}

	if result.RandomError == nil {
		t.Fatal("RandomError config is nil")
	}

	t.Logf("RandomError config: ErrorRate=%.2f, ErrorTypes=%v",
		result.RandomError.ErrorRate, result.RandomError.ErrorTypes)

	if !result.RandomError.Enabled {
		t.Error("Expected random error to be enabled")
	}
}

func TestIntegration_ChaosConfig_Greylist(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	if !client.ServerInfo().ChaosEnabled {
		t.Skip("Chaos is not enabled on this server")
	}

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	config := &vaultsandbox.ChaosConfig{
		Enabled: true,
		Greylist: &vaultsandbox.GreylistConfig{
			Enabled:       true,
			RetryWindowMs: 600000,
			MaxAttempts:   3,
			TrackBy:       vaultsandbox.GreylistTrackByIPSender,
		},
	}

	result, err := inbox.SetChaosConfig(ctx, config)
	if err != nil {
		t.Fatalf("SetChaosConfig() error = %v", err)
	}

	if result.Greylist == nil {
		t.Fatal("Greylist config is nil")
	}

	t.Logf("Greylist config: RetryWindowMs=%d, MaxAttempts=%d, TrackBy=%s",
		result.Greylist.RetryWindowMs, result.Greylist.MaxAttempts, result.Greylist.TrackBy)

	if !result.Greylist.Enabled {
		t.Error("Expected greylist to be enabled")
	}
}

func TestIntegration_ChaosConfig_Blackhole(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	if !client.ServerInfo().ChaosEnabled {
		t.Skip("Chaos is not enabled on this server")
	}

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	config := &vaultsandbox.ChaosConfig{
		Enabled: true,
		Blackhole: &vaultsandbox.BlackholeConfig{
			Enabled:         true,
			TriggerWebhooks: false,
		},
	}

	result, err := inbox.SetChaosConfig(ctx, config)
	if err != nil {
		t.Fatalf("SetChaosConfig() error = %v", err)
	}

	if result.Blackhole == nil {
		t.Fatal("Blackhole config is nil")
	}

	t.Logf("Blackhole config: TriggerWebhooks=%v", result.Blackhole.TriggerWebhooks)

	if !result.Blackhole.Enabled {
		t.Error("Expected blackhole to be enabled")
	}
}

func TestIntegration_ChaosConfig_DisableChaos(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	if !client.ServerInfo().ChaosEnabled {
		t.Skip("Chaos is not enabled on this server")
	}

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Enable chaos first
	config := &vaultsandbox.ChaosConfig{
		Enabled: true,
		Latency: &vaultsandbox.LatencyConfig{
			Enabled:    true,
			MinDelayMs: 100,
			MaxDelayMs: 500,
		},
	}

	_, err = inbox.SetChaosConfig(ctx, config)
	if err != nil {
		t.Fatalf("SetChaosConfig() error = %v", err)
	}

	// Disable chaos via DELETE
	err = inbox.DisableChaos(ctx)
	if err != nil {
		t.Fatalf("DisableChaos() error = %v", err)
	}

	// Verify it's disabled
	got, err := inbox.GetChaosConfig(ctx)
	if err != nil {
		t.Fatalf("GetChaosConfig() error = %v", err)
	}

	if got.Enabled {
		t.Error("Expected chaos to be disabled after DisableChaos()")
	}
	t.Logf("Chaos disabled successfully")
}

func TestIntegration_ChaosConfig_DisableViaSet(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	if !client.ServerInfo().ChaosEnabled {
		t.Skip("Chaos is not enabled on this server")
	}

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Enable chaos first
	config := &vaultsandbox.ChaosConfig{
		Enabled: true,
		Latency: &vaultsandbox.LatencyConfig{
			Enabled:    true,
			MinDelayMs: 100,
			MaxDelayMs: 500,
		},
	}

	_, err = inbox.SetChaosConfig(ctx, config)
	if err != nil {
		t.Fatalf("SetChaosConfig() error = %v", err)
	}

	// Disable chaos via POST with enabled: false
	disableConfig := &vaultsandbox.ChaosConfig{
		Enabled: false,
	}

	result, err := inbox.SetChaosConfig(ctx, disableConfig)
	if err != nil {
		t.Fatalf("SetChaosConfig() disable error = %v", err)
	}

	if result.Enabled {
		t.Error("Expected chaos to be disabled after setting enabled=false")
	}
	t.Logf("Chaos disabled via SetChaosConfig(enabled=false)")
}

func TestIntegration_ChaosConfig_MultipleChaosTypes(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	if !client.ServerInfo().ChaosEnabled {
		t.Skip("Chaos is not enabled on this server")
	}

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Enable multiple chaos types
	config := &vaultsandbox.ChaosConfig{
		Enabled: true,
		Latency: &vaultsandbox.LatencyConfig{
			Enabled:     true,
			MinDelayMs:  500,
			MaxDelayMs:  3000,
			Probability: 0.8,
		},
		RandomError: &vaultsandbox.RandomErrorConfig{
			Enabled:    true,
			ErrorRate:  0.1,
			ErrorTypes: []vaultsandbox.RandomErrorType{vaultsandbox.RandomErrorTypeTemporary, vaultsandbox.RandomErrorTypePermanent},
		},
	}

	result, err := inbox.SetChaosConfig(ctx, config)
	if err != nil {
		t.Fatalf("SetChaosConfig() error = %v", err)
	}

	t.Logf("Multiple chaos types configured:")
	if result.Latency != nil {
		t.Logf("  Latency: enabled=%v, min=%d, max=%d", result.Latency.Enabled, result.Latency.MinDelayMs, result.Latency.MaxDelayMs)
	}
	if result.RandomError != nil {
		t.Logf("  RandomError: enabled=%v, rate=%.2f", result.RandomError.Enabled, result.RandomError.ErrorRate)
	}

	if result.Latency == nil || !result.Latency.Enabled {
		t.Error("Expected latency to be enabled")
	}
	if result.RandomError == nil || !result.RandomError.Enabled {
		t.Error("Expected random error to be enabled")
	}
}

func TestIntegration_ChaosConfig_WithExpiration(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	if !client.ServerInfo().ChaosEnabled {
		t.Skip("Chaos is not enabled on this server")
	}

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Set expiration time in the future
	expiresAt := time.Now().Add(1 * time.Hour)

	config := &vaultsandbox.ChaosConfig{
		Enabled:   true,
		ExpiresAt: &expiresAt,
		Latency: &vaultsandbox.LatencyConfig{
			Enabled:    true,
			MinDelayMs: 100,
			MaxDelayMs: 500,
		},
	}

	result, err := inbox.SetChaosConfig(ctx, config)
	if err != nil {
		t.Fatalf("SetChaosConfig() error = %v", err)
	}

	if result.ExpiresAt == nil {
		t.Fatal("Expected expiresAt to be set")
	}

	t.Logf("Chaos config with expiration: ExpiresAt=%v", result.ExpiresAt)

	// Verify the expiration time is approximately what we set
	diff := result.ExpiresAt.Sub(expiresAt)
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("ExpiresAt differs too much: got %v, want ~%v", result.ExpiresAt, expiresAt)
	}
}

func TestIntegration_ChaosConfig_UpdateConfig(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	if !client.ServerInfo().ChaosEnabled {
		t.Skip("Chaos is not enabled on this server")
	}

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("CreateInbox() error = %v", err)
	}
	defer inbox.Delete(ctx)

	// Set initial config
	config := &vaultsandbox.ChaosConfig{
		Enabled: true,
		Latency: &vaultsandbox.LatencyConfig{
			Enabled:    true,
			MinDelayMs: 100,
			MaxDelayMs: 500,
		},
	}

	_, err = inbox.SetChaosConfig(ctx, config)
	if err != nil {
		t.Fatalf("SetChaosConfig() initial error = %v", err)
	}

	// Update with different config
	updatedConfig := &vaultsandbox.ChaosConfig{
		Enabled: true,
		Latency: &vaultsandbox.LatencyConfig{
			Enabled:    true,
			MinDelayMs: 1000,
			MaxDelayMs: 2000,
		},
		RandomError: &vaultsandbox.RandomErrorConfig{
			Enabled:   true,
			ErrorRate: 0.1,
		},
	}

	result, err := inbox.SetChaosConfig(ctx, updatedConfig)
	if err != nil {
		t.Fatalf("SetChaosConfig() update error = %v", err)
	}

	t.Logf("Updated config: Latency.MaxDelayMs=%d", result.Latency.MaxDelayMs)

	if result.RandomError == nil || !result.RandomError.Enabled {
		t.Error("Expected random error to be added in update")
	}
}
