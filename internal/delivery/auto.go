package delivery

import (
	"context"
	"time"
)

const (
	AutoSSETimeout = 5 * time.Second
)

// AutoStrategy automatically selects between SSE and polling.
type AutoStrategy struct {
	cfg     Config
	current Strategy
	handler EventHandler
}

// NewAutoStrategy creates a new auto strategy.
func NewAutoStrategy(cfg Config) *AutoStrategy {
	return &AutoStrategy{
		cfg: cfg,
	}
}

// Name returns the strategy name.
func (a *AutoStrategy) Name() string {
	if a.current != nil {
		return "auto:" + a.current.Name()
	}
	return "auto"
}

// Start begins listening for emails, trying SSE first then falling back to polling.
func (a *AutoStrategy) Start(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error {
	a.handler = handler

	// Try SSE first with timeout
	sseCtx, cancel := context.WithTimeout(ctx, AutoSSETimeout)
	defer cancel()

	sse := NewSSEStrategy(a.cfg)
	err := sse.Start(sseCtx, inboxes, handler)

	if err == nil {
		a.current = sse
		return nil
	}

	// Fall back to polling
	polling := NewPollingStrategy(a.cfg)
	err = polling.Start(ctx, inboxes, handler)
	if err != nil {
		return err
	}

	a.current = polling
	return nil
}

// Stop gracefully shuts down the strategy.
func (a *AutoStrategy) Stop() error {
	if a.current != nil {
		return a.current.Stop()
	}
	return nil
}

// AddInbox adds an inbox to monitor.
func (a *AutoStrategy) AddInbox(inbox InboxInfo) error {
	if a.current != nil {
		return a.current.AddInbox(inbox)
	}
	return nil
}

// RemoveInbox removes an inbox from monitoring.
func (a *AutoStrategy) RemoveInbox(inboxHash string) error {
	if a.current != nil {
		return a.current.RemoveInbox(inboxHash)
	}
	return nil
}

// Legacy interface implementation for backward compatibility.

// WaitForEmail waits for an email using the best available strategy.
func (a *AutoStrategy) WaitForEmail(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, pollInterval time.Duration) (interface{}, error) {
	// Use polling for backward compatibility
	polling := NewPollingStrategy(a.cfg)
	return polling.WaitForEmail(ctx, inboxHash, fetcher, matcher, pollInterval)
}

// WaitForEmailCount waits for multiple emails using the best available strategy.
func (a *AutoStrategy) WaitForEmailCount(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, pollInterval time.Duration) ([]interface{}, error) {
	polling := NewPollingStrategy(a.cfg)
	return polling.WaitForEmailCount(ctx, inboxHash, fetcher, matcher, count, pollInterval)
}

// Close closes the auto strategy.
func (a *AutoStrategy) Close() error {
	return a.Stop()
}
