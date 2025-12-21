package delivery

import (
	"context"
	"time"
)

// SSEStrategy implements email delivery via Server-Sent Events.
type SSEStrategy struct {
	apiClient interface{}
}

// NewSSEStrategy creates a new SSE strategy.
func NewSSEStrategy(apiClient interface{}) *SSEStrategy {
	return &SSEStrategy{
		apiClient: apiClient,
	}
}

// WaitForEmail waits for an email using SSE.
func (s *SSEStrategy) WaitForEmail(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, pollInterval time.Duration) (interface{}, error) {
	// TODO: Implement SSE-based email waiting
	// 1. Connect to SSE endpoint
	// 2. Listen for email events
	// 3. Decrypt and return matching email
	// For now, fall back to polling
	polling := NewPollingStrategy(s.apiClient)
	return polling.WaitForEmail(ctx, inboxHash, fetcher, matcher, pollInterval)
}

// WaitForEmailCount waits for multiple emails using SSE.
func (s *SSEStrategy) WaitForEmailCount(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, pollInterval time.Duration) ([]interface{}, error) {
	// TODO: Implement SSE-based email counting
	// For now, fall back to polling
	polling := NewPollingStrategy(s.apiClient)
	return polling.WaitForEmailCount(ctx, inboxHash, fetcher, matcher, count, pollInterval)
}

// Close closes the SSE strategy.
func (s *SSEStrategy) Close() error {
	return nil
}
