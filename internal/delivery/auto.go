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
	sse := NewSSEStrategy(a.cfg)
	err := sse.Start(ctx, inboxes, handler)
	if err != nil {
		// SSE failed to start, fall back to polling immediately
		return a.startPolling(ctx, inboxes, handler)
	}

	// Wait for SSE connection to be established or timeout
	select {
	case <-sse.Connected():
		// SSE connected successfully
		a.current = sse
		return nil
	case <-time.After(AutoSSETimeout):
		// SSE didn't connect in time, fall back to polling
		sse.Stop()
		return a.startPolling(ctx, inboxes, handler)
	case <-ctx.Done():
		sse.Stop()
		return ctx.Err()
	}
}

func (a *AutoStrategy) startPolling(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error {
	polling := NewPollingStrategy(a.cfg)
	err := polling.Start(ctx, inboxes, handler)
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
	return a.WaitForEmailWithSync(ctx, inboxHash, fetcher, matcher, WaitOptions{
		PollInterval: pollInterval,
		SyncFetcher:  nil,
	})
}

// WaitForEmailWithSync waits for an email using sync-status-based change detection.
func (a *AutoStrategy) WaitForEmailWithSync(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, opts WaitOptions) (interface{}, error) {
	// Use polling for backward compatibility
	polling := NewPollingStrategy(a.cfg)
	return polling.WaitForEmailWithSync(ctx, inboxHash, fetcher, matcher, opts)
}

// WaitForEmailCount waits for multiple emails using the best available strategy.
func (a *AutoStrategy) WaitForEmailCount(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, pollInterval time.Duration) ([]interface{}, error) {
	return a.WaitForEmailCountWithSync(ctx, inboxHash, fetcher, matcher, count, WaitOptions{
		PollInterval: pollInterval,
		SyncFetcher:  nil,
	})
}

// WaitForEmailCountWithSync waits for multiple emails using sync-status-based change detection.
func (a *AutoStrategy) WaitForEmailCountWithSync(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, opts WaitOptions) ([]interface{}, error) {
	polling := NewPollingStrategy(a.cfg)
	return polling.WaitForEmailCountWithSync(ctx, inboxHash, fetcher, matcher, count, opts)
}

// Close closes the auto strategy.
func (a *AutoStrategy) Close() error {
	return a.Stop()
}
