package vaultsandbox

import (
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

// GreylistTrackBy represents how to identify unique senders for greylisting.
type GreylistTrackBy string

const (
	// GreylistTrackByIP tracks by sender IP address only.
	GreylistTrackByIP GreylistTrackBy = "ip"
	// GreylistTrackBySender tracks by sender email address only.
	GreylistTrackBySender GreylistTrackBy = "sender"
	// GreylistTrackByIPSender tracks by combination of IP and sender email.
	GreylistTrackByIPSender GreylistTrackBy = "ip_sender"
)

// RandomErrorType represents the type of SMTP errors to return.
type RandomErrorType string

const (
	// RandomErrorTypeTemporary returns 4xx errors (421, 450, 451, 452).
	RandomErrorTypeTemporary RandomErrorType = "temporary"
	// RandomErrorTypePermanent returns 5xx errors (550, 551, 552, 553, 554).
	RandomErrorTypePermanent RandomErrorType = "permanent"
)

// ChaosConfig represents the chaos engineering configuration for an inbox.
type ChaosConfig struct {
	// Enabled is the master switch for chaos on this inbox.
	Enabled bool
	// ExpiresAt is the optional auto-disable timestamp.
	ExpiresAt *time.Time
	// Latency configures latency injection.
	Latency *LatencyConfig
	// ConnectionDrop configures connection drop behavior.
	ConnectionDrop *ConnectionDropConfig
	// RandomError configures random error generation.
	RandomError *RandomErrorConfig
	// Greylist configures greylisting simulation.
	Greylist *GreylistConfig
	// Blackhole configures blackhole mode.
	Blackhole *BlackholeConfig
}

// LatencyConfig represents latency injection settings.
type LatencyConfig struct {
	// Enabled enables latency injection.
	Enabled bool
	// MinDelayMs is the minimum delay in milliseconds (default: 500).
	MinDelayMs int
	// MaxDelayMs is the maximum delay in milliseconds (default: 10000, max: 60000).
	MaxDelayMs int
	// Jitter randomizes delay within range (default: true). If false, uses fixed delay at MaxDelayMs.
	Jitter bool
	// Probability is the probability of applying delay, 0.0-1.0 (default: 1.0).
	Probability float64
}

// ConnectionDropConfig represents connection drop settings.
type ConnectionDropConfig struct {
	// Enabled enables connection dropping.
	Enabled bool
	// Probability is the probability of dropping, 0.0-1.0 (default: 1.0).
	Probability float64
	// Graceful uses graceful close (FIN) vs abrupt (RST) (default: true).
	Graceful bool
}

// RandomErrorConfig represents random error generation settings.
type RandomErrorConfig struct {
	// Enabled enables random error generation.
	Enabled bool
	// ErrorRate is the probability of returning an error, 0.0-1.0 (default: 0.1).
	ErrorRate float64
	// ErrorTypes is the list of error types to return (default: ["temporary"]).
	ErrorTypes []RandomErrorType
}

// GreylistConfig represents greylisting simulation settings.
type GreylistConfig struct {
	// Enabled enables greylisting simulation.
	Enabled bool
	// RetryWindowMs is the window for tracking retry attempts in milliseconds (default: 300000).
	RetryWindowMs int
	// MaxAttempts is the number of attempts before accepting (default: 2).
	MaxAttempts int
	// TrackBy is how to identify unique senders (default: "ip_sender").
	TrackBy GreylistTrackBy
}

// BlackholeConfig represents blackhole mode settings.
type BlackholeConfig struct {
	// Enabled enables blackhole mode (accepts but silently discards emails).
	Enabled bool
	// TriggerWebhooks determines whether to still trigger webhooks (default: false).
	TriggerWebhooks bool
}

// chaosConfigFromDTO converts an API DTO to a public ChaosConfig type.
func chaosConfigFromDTO(dto *api.ChaosConfigDTO) *ChaosConfig {
	if dto == nil {
		return nil
	}

	cfg := &ChaosConfig{
		Enabled:   dto.Enabled,
		ExpiresAt: dto.ExpiresAt,
	}

	if dto.Latency != nil {
		cfg.Latency = &LatencyConfig{
			Enabled: dto.Latency.Enabled,
		}
		if dto.Latency.MinDelayMs != nil {
			cfg.Latency.MinDelayMs = *dto.Latency.MinDelayMs
		}
		if dto.Latency.MaxDelayMs != nil {
			cfg.Latency.MaxDelayMs = *dto.Latency.MaxDelayMs
		}
		if dto.Latency.Jitter != nil {
			cfg.Latency.Jitter = *dto.Latency.Jitter
		}
		if dto.Latency.Probability != nil {
			cfg.Latency.Probability = *dto.Latency.Probability
		}
	}

	if dto.ConnectionDrop != nil {
		cfg.ConnectionDrop = &ConnectionDropConfig{
			Enabled: dto.ConnectionDrop.Enabled,
		}
		if dto.ConnectionDrop.Probability != nil {
			cfg.ConnectionDrop.Probability = *dto.ConnectionDrop.Probability
		}
		if dto.ConnectionDrop.Graceful != nil {
			cfg.ConnectionDrop.Graceful = *dto.ConnectionDrop.Graceful
		}
	}

	if dto.RandomError != nil {
		cfg.RandomError = &RandomErrorConfig{
			Enabled: dto.RandomError.Enabled,
		}
		if dto.RandomError.ErrorRate != nil {
			cfg.RandomError.ErrorRate = *dto.RandomError.ErrorRate
		}
		if dto.RandomError.ErrorTypes != nil {
			cfg.RandomError.ErrorTypes = make([]RandomErrorType, len(dto.RandomError.ErrorTypes))
			for i, et := range dto.RandomError.ErrorTypes {
				cfg.RandomError.ErrorTypes[i] = RandomErrorType(et)
			}
		}
	}

	if dto.Greylist != nil {
		cfg.Greylist = &GreylistConfig{
			Enabled: dto.Greylist.Enabled,
		}
		if dto.Greylist.RetryWindowMs != nil {
			cfg.Greylist.RetryWindowMs = *dto.Greylist.RetryWindowMs
		}
		if dto.Greylist.MaxAttempts != nil {
			cfg.Greylist.MaxAttempts = *dto.Greylist.MaxAttempts
		}
		if dto.Greylist.TrackBy != nil {
			cfg.Greylist.TrackBy = GreylistTrackBy(*dto.Greylist.TrackBy)
		}
	}

	if dto.Blackhole != nil {
		cfg.Blackhole = &BlackholeConfig{
			Enabled: dto.Blackhole.Enabled,
		}
		if dto.Blackhole.TriggerWebhooks != nil {
			cfg.Blackhole.TriggerWebhooks = *dto.Blackhole.TriggerWebhooks
		}
	}

	return cfg
}

// chaosConfigToRequest converts a public ChaosConfig to an API request.
func chaosConfigToRequest(cfg *ChaosConfig) *api.ChaosConfigRequest {
	if cfg == nil {
		return nil
	}

	req := &api.ChaosConfigRequest{
		Enabled:   cfg.Enabled,
		ExpiresAt: cfg.ExpiresAt,
	}

	if cfg.Latency != nil {
		req.Latency = &api.LatencyConfigDTO{
			Enabled:     cfg.Latency.Enabled,
			MinDelayMs:  &cfg.Latency.MinDelayMs,
			MaxDelayMs:  &cfg.Latency.MaxDelayMs,
			Jitter:      &cfg.Latency.Jitter,
			Probability: &cfg.Latency.Probability,
		}
	}

	if cfg.ConnectionDrop != nil {
		req.ConnectionDrop = &api.ConnectionDropDTO{
			Enabled:     cfg.ConnectionDrop.Enabled,
			Probability: &cfg.ConnectionDrop.Probability,
			Graceful:    &cfg.ConnectionDrop.Graceful,
		}
	}

	if cfg.RandomError != nil {
		req.RandomError = &api.RandomErrorConfigDTO{
			Enabled:   cfg.RandomError.Enabled,
			ErrorRate: &cfg.RandomError.ErrorRate,
		}
		if cfg.RandomError.ErrorTypes != nil {
			req.RandomError.ErrorTypes = make([]string, len(cfg.RandomError.ErrorTypes))
			for i, et := range cfg.RandomError.ErrorTypes {
				req.RandomError.ErrorTypes[i] = string(et)
			}
		}
	}

	if cfg.Greylist != nil {
		trackBy := string(cfg.Greylist.TrackBy)
		req.Greylist = &api.GreylistConfigDTO{
			Enabled:       cfg.Greylist.Enabled,
			RetryWindowMs: &cfg.Greylist.RetryWindowMs,
			MaxAttempts:   &cfg.Greylist.MaxAttempts,
			TrackBy:       &trackBy,
		}
	}

	if cfg.Blackhole != nil {
		req.Blackhole = &api.BlackholeConfigDTO{
			Enabled:         cfg.Blackhole.Enabled,
			TriggerWebhooks: &cfg.Blackhole.TriggerWebhooks,
		}
	}

	return req
}
