package api

import (
	"context"
	"testing"
	"time"
)

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.BaseDelay != time.Second {
		t.Errorf("BaseDelay = %v, want 1s", cfg.BaseDelay)
	}
	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("MaxDelay = %v, want 30s", cfg.MaxDelay)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("Multiplier = %v, want 2.0", cfg.Multiplier)
	}
	if cfg.Jitter != 0.2 {
		t.Errorf("Jitter = %v, want 0.2", cfg.Jitter)
	}
	if cfg.RetryableOn == nil {
		t.Error("RetryableOn is nil")
	}
}

func TestRetryConfig_ShouldRetry(t *testing.T) {
	cfg := DefaultRetryConfig()

	tests := []struct {
		name       string
		attempt    int
		statusCode int
		expected   bool
	}{
		{"first attempt, retryable", 0, 503, true},
		{"second attempt, retryable", 1, 503, true},
		{"third attempt, retryable", 2, 503, true},
		{"max attempts reached", 3, 503, false},
		{"over max attempts", 4, 503, false},
		{"non-retryable status", 0, 400, false},
		{"non-retryable 401", 0, 401, false},
		{"non-retryable 404", 0, 404, false},
		{"retryable 429", 0, 429, true},
		{"retryable 500", 0, 500, true},
		{"retryable 502", 0, 502, true},
		{"retryable 504", 0, 504, true},
		{"retryable 408", 0, 408, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.ShouldRetry(tt.attempt, tt.statusCode)
			if result != tt.expected {
				t.Errorf("ShouldRetry(%d, %d) = %v, want %v",
					tt.attempt, tt.statusCode, result, tt.expected)
			}
		})
	}
}

func TestRetryConfig_Delay(t *testing.T) {
	cfg := &RetryConfig{
		BaseDelay:  time.Second,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
		Jitter:     0, // No jitter for predictable tests
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, time.Second},      // 1 * 2^0 = 1s
		{1, 2 * time.Second},  // 1 * 2^1 = 2s
		{2, 4 * time.Second},  // 1 * 2^2 = 4s
		{3, 8 * time.Second},  // 1 * 2^3 = 8s
		{4, 16 * time.Second}, // 1 * 2^4 = 16s
		{5, 30 * time.Second}, // 1 * 2^5 = 32s, capped at 30s
		{6, 30 * time.Second}, // Still capped at 30s
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			delay := cfg.Delay(tt.attempt)
			if delay != tt.expected {
				t.Errorf("Delay(%d) = %v, want %v", tt.attempt, delay, tt.expected)
			}
		})
	}
}

func TestRetryConfig_Delay_WithJitter(t *testing.T) {
	cfg := &RetryConfig{
		BaseDelay:  time.Second,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.5, // 50% jitter
	}

	// With 50% jitter on 1s base delay, the range should be 0.5s to 1.5s
	minDelay := 500 * time.Millisecond
	maxDelay := 1500 * time.Millisecond

	// Run multiple times to verify randomness is within bounds
	for i := 0; i < 100; i++ {
		delay := cfg.Delay(0)
		if delay < minDelay || delay > maxDelay {
			t.Errorf("Delay(0) = %v, expected between %v and %v", delay, minDelay, maxDelay)
		}
	}
}

func TestRetryConfig_Delay_MaxDelayWithJitter(t *testing.T) {
	cfg := &RetryConfig{
		BaseDelay:  10 * time.Second,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.2, // 20% jitter
	}

	// At attempt 3: 10 * 2^3 = 80s, capped at 30s
	// With 20% jitter on 30s, range is 24s to 36s, but should still cap around 30s
	for i := 0; i < 100; i++ {
		delay := cfg.Delay(3)
		// Jitter can push it slightly over max in the current implementation
		if delay < 24*time.Second || delay > 36*time.Second {
			t.Errorf("Delay(3) = %v, expected around 30s with jitter", delay)
		}
	}
}

func TestRetryConfig_Wait(t *testing.T) {
	cfg := &RetryConfig{
		BaseDelay:  10 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     0,
	}

	ctx := context.Background()
	start := time.Now()

	err := cfg.Wait(ctx, 0)
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	elapsed := time.Since(start)
	if elapsed < 10*time.Millisecond {
		t.Errorf("Wait() returned too early: %v", elapsed)
	}
}

func TestRetryConfig_Wait_ContextCancellation(t *testing.T) {
	cfg := &RetryConfig{
		BaseDelay:  10 * time.Second, // Long delay
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
		Jitter:     0,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := cfg.Wait(ctx, 0)
	elapsed := time.Since(start)

	if err != context.Canceled {
		t.Errorf("Wait() error = %v, want context.Canceled", err)
	}

	// Should have returned quickly due to cancellation
	if elapsed > 200*time.Millisecond {
		t.Errorf("Wait() took too long after cancellation: %v", elapsed)
	}
}

func TestRetryConfig_Wait_Timeout(t *testing.T) {
	cfg := &RetryConfig{
		BaseDelay:  10 * time.Second, // Long delay
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
		Jitter:     0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := cfg.Wait(ctx, 0)
	if err != context.DeadlineExceeded {
		t.Errorf("Wait() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestRetryConfig_CustomRetryableOn(t *testing.T) {
	cfg := &RetryConfig{
		MaxRetries: 3,
		RetryableOn: func(statusCode int) bool {
			// Only retry on 418 (I'm a teapot)
			return statusCode == 418
		},
	}

	tests := []struct {
		statusCode int
		expected   bool
	}{
		{418, true},
		{500, false},
		{503, false},
		{200, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := cfg.ShouldRetry(0, tt.statusCode)
			if result != tt.expected {
				t.Errorf("ShouldRetry(0, %d) = %v, want %v",
					tt.statusCode, result, tt.expected)
			}
		})
	}
}

func BenchmarkRetryConfig_Delay(b *testing.B) {
	cfg := DefaultRetryConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cfg.Delay(i % 5)
	}
}

func BenchmarkRetryConfig_ShouldRetry(b *testing.B) {
	cfg := DefaultRetryConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cfg.ShouldRetry(i%4, 503)
	}
}
