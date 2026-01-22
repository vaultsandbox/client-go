package api

import "time"

// ChaosConfigRequest is the request body for setting chaos configuration on an inbox.
type ChaosConfigRequest struct {
	Enabled        bool                  `json:"enabled"`
	ExpiresAt      *time.Time            `json:"expiresAt,omitempty"`
	Latency        *LatencyConfigDTO     `json:"latency,omitempty"`
	ConnectionDrop *ConnectionDropDTO    `json:"connectionDrop,omitempty"`
	RandomError    *RandomErrorConfigDTO `json:"randomError,omitempty"`
	Greylist       *GreylistConfigDTO    `json:"greylist,omitempty"`
	Blackhole      *BlackholeConfigDTO   `json:"blackhole,omitempty"`
}

// ChaosConfigDTO represents the chaos configuration returned from the API.
type ChaosConfigDTO struct {
	Enabled        bool                  `json:"enabled"`
	ExpiresAt      *time.Time            `json:"expiresAt,omitempty"`
	Latency        *LatencyConfigDTO     `json:"latency,omitempty"`
	ConnectionDrop *ConnectionDropDTO    `json:"connectionDrop,omitempty"`
	RandomError    *RandomErrorConfigDTO `json:"randomError,omitempty"`
	Greylist       *GreylistConfigDTO    `json:"greylist,omitempty"`
	Blackhole      *BlackholeConfigDTO   `json:"blackhole,omitempty"`
}

// LatencyConfigDTO represents latency injection settings.
type LatencyConfigDTO struct {
	Enabled    bool     `json:"enabled"`
	MinDelayMs *int     `json:"minDelayMs,omitempty"`
	MaxDelayMs *int     `json:"maxDelayMs,omitempty"`
	Jitter     *bool    `json:"jitter,omitempty"`
	Probability *float64 `json:"probability,omitempty"`
}

// ConnectionDropDTO represents connection drop settings.
type ConnectionDropDTO struct {
	Enabled     bool     `json:"enabled"`
	Probability *float64 `json:"probability,omitempty"`
	Graceful    *bool    `json:"graceful,omitempty"`
}

// RandomErrorConfigDTO represents random error generation settings.
type RandomErrorConfigDTO struct {
	Enabled    bool      `json:"enabled"`
	ErrorRate  *float64  `json:"errorRate,omitempty"`
	ErrorTypes []string  `json:"errorTypes,omitempty"`
}

// GreylistConfigDTO represents greylisting simulation settings.
type GreylistConfigDTO struct {
	Enabled       bool    `json:"enabled"`
	RetryWindowMs *int    `json:"retryWindowMs,omitempty"`
	MaxAttempts   *int    `json:"maxAttempts,omitempty"`
	TrackBy       *string `json:"trackBy,omitempty"`
}

// BlackholeConfigDTO represents blackhole mode settings.
type BlackholeConfigDTO struct {
	Enabled         bool  `json:"enabled"`
	TriggerWebhooks *bool `json:"triggerWebhooks,omitempty"`
}
