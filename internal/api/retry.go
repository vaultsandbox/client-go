package api

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
	Jitter      float64 // 0.0 to 1.0, adds randomness to delays
	RetryableOn func(statusCode int) bool
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries: 3,
		BaseDelay:  time.Second,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.2,
		RetryableOn: func(statusCode int) bool {
			switch statusCode {
			case 408, 429, 500, 502, 503, 504:
				return true
			default:
				return false
			}
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

// Delay calculates the delay before the next retry attempt with optional jitter.
func (r *RetryConfig) Delay(attempt int) time.Duration {
	delay := float64(r.BaseDelay) * math.Pow(r.Multiplier, float64(attempt))
	if delay > float64(r.MaxDelay) {
		delay = float64(r.MaxDelay)
	}

	// Add jitter
	if r.Jitter > 0 {
		jitterAmount := delay * r.Jitter
		delay = delay - jitterAmount + (rand.Float64() * 2 * jitterAmount)
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
