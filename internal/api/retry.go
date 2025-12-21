package api

import (
	"context"
	"math"
	"time"
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
	RetryableOn func(statusCode int) bool
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries: 3,
		BaseDelay:  time.Second,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
		RetryableOn: func(statusCode int) bool {
			return statusCode >= 500 || statusCode == 429
		},
	}
}

// ShouldRetry determines if a request should be retried.
func (r *RetryConfig) ShouldRetry(attempt int, statusCode int) bool {
	if attempt >= r.MaxRetries {
		return false
	}
	return r.RetryableOn(statusCode)
}

// Delay calculates the delay before the next retry attempt.
func (r *RetryConfig) Delay(attempt int) time.Duration {
	delay := float64(r.BaseDelay) * math.Pow(r.Multiplier, float64(attempt))
	if delay > float64(r.MaxDelay) {
		delay = float64(r.MaxDelay)
	}
	return time.Duration(delay)
}

// Wait waits for the appropriate delay before retrying.
func (r *RetryConfig) Wait(ctx context.Context, attempt int) error {
	delay := r.Delay(attempt)
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
