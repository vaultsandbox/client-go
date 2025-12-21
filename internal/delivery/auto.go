package delivery

import (
	"context"
	"time"
)

// AutoStrategy automatically selects between SSE and polling.
type AutoStrategy struct {
	apiClient interface{}
	sse       *SSEStrategy
	polling   *PollingStrategy
}

// NewAutoStrategy creates a new auto strategy.
func NewAutoStrategy(apiClient interface{}) *AutoStrategy {
	return &AutoStrategy{
		apiClient: apiClient,
		sse:       NewSSEStrategy(apiClient),
		polling:   NewPollingStrategy(apiClient),
	}
}

// WaitForEmail waits for an email using the best available strategy.
func (a *AutoStrategy) WaitForEmail(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, pollInterval time.Duration) (interface{}, error) {
	// Try SSE first, fall back to polling if SSE fails
	// For now, just use polling
	return a.polling.WaitForEmail(ctx, inboxHash, fetcher, matcher, pollInterval)
}

// WaitForEmailCount waits for multiple emails using the best available strategy.
func (a *AutoStrategy) WaitForEmailCount(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, pollInterval time.Duration) ([]interface{}, error) {
	return a.polling.WaitForEmailCount(ctx, inboxHash, fetcher, matcher, count, pollInterval)
}

// Close closes the auto strategy.
func (a *AutoStrategy) Close() error {
	if err := a.sse.Close(); err != nil {
		return err
	}
	return a.polling.Close()
}
