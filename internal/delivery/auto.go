package delivery

import (
	"context"
	"time"
)

// AutoSSETimeout is the maximum time to wait for an SSE connection to be
// established before falling back to polling.
const AutoSSETimeout = 5 * time.Second

// AutoStrategy automatically selects between SSE and polling delivery.
// It provides the best of both worlds: low-latency SSE when available,
// with automatic fallback to reliable polling when SSE fails.
//
// Strategy selection logic:
//  1. AutoStrategy first attempts to establish an SSE connection
//  2. If SSE connects within AutoSSETimeout, SSE is used
//  3. If SSE fails to connect in time, the strategy falls back to polling
//
// Once a strategy is selected, it remains active for the lifetime of the
// AutoStrategy. There is no dynamic switching between strategies.
type AutoStrategy struct {
	cfg     Config    // Configuration shared with underlying strategies.
	current Strategy  // The currently active strategy (SSE or polling).
	handler EventHandler // Callback for new email events.
}

// NewAutoStrategy creates a new auto strategy with the given configuration.
// The actual delivery strategy (SSE or polling) is determined when Start is called.
func NewAutoStrategy(cfg Config) *AutoStrategy {
	return &AutoStrategy{
		cfg: cfg,
	}
}

// Name returns the strategy name for logging and debugging. The format is
// "auto:<underlying>" where <underlying> is "sse" or "polling" depending
// on which strategy was selected, or just "auto" if not yet started.
func (a *AutoStrategy) Name() string {
	if a.current != nil {
		return "auto:" + a.current.Name()
	}
	return "auto"
}

// Start begins listening for emails using the best available strategy.
// It first attempts SSE and waits up to AutoSSETimeout for the connection.
// If SSE fails to connect in time, it automatically falls back to polling.
//
// The strategy selection happens synchronously during Start. Once Start
// returns successfully, the selected strategy is active and events will
// be delivered to the handler.
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

// startPolling initializes and starts the polling strategy as a fallback.
func (a *AutoStrategy) startPolling(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error {
	polling := NewPollingStrategy(a.cfg)
	err := polling.Start(ctx, inboxes, handler)
	if err != nil {
		return err
	}
	a.current = polling
	return nil
}

// Stop gracefully shuts down the underlying strategy (SSE or polling).
// Stop is idempotent and safe to call multiple times or before Start.
func (a *AutoStrategy) Stop() error {
	if a.current != nil {
		return a.current.Stop()
	}
	return nil
}

// AddInbox adds an inbox to be monitored by the underlying strategy.
// If no strategy is active, the inbox is not added.
func (a *AutoStrategy) AddInbox(inbox InboxInfo) error {
	if a.current != nil {
		return a.current.AddInbox(inbox)
	}
	return nil
}

// RemoveInbox removes an inbox from monitoring by the underlying strategy.
// If no strategy is active, this is a no-op.
func (a *AutoStrategy) RemoveInbox(inboxHash string) error {
	if a.current != nil {
		return a.current.RemoveInbox(inboxHash)
	}
	return nil
}

// Legacy interface implementation for backward compatibility.
// These methods delegate to PollingStrategy for the WaitForEmail API.

// WaitForEmail waits for an email matching the given criteria.
// This method uses polling for backward compatibility.
func (a *AutoStrategy) WaitForEmail(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, pollInterval time.Duration) (interface{}, error) {
	return a.WaitForEmailWithSync(ctx, inboxHash, fetcher, matcher, WaitOptions{
		PollInterval: pollInterval,
		SyncFetcher:  nil,
	})
}

// WaitForEmailWithSync waits for an email using sync-status-based change detection.
// This method delegates to PollingStrategy for backward compatibility.
func (a *AutoStrategy) WaitForEmailWithSync(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, opts WaitOptions) (interface{}, error) {
	// Use polling for backward compatibility
	polling := NewPollingStrategy(a.cfg)
	return polling.WaitForEmailWithSync(ctx, inboxHash, fetcher, matcher, opts)
}

// WaitForEmailCount waits until at least count emails match the criteria.
// This method uses polling for backward compatibility.
func (a *AutoStrategy) WaitForEmailCount(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, pollInterval time.Duration) ([]interface{}, error) {
	return a.WaitForEmailCountWithSync(ctx, inboxHash, fetcher, matcher, count, WaitOptions{
		PollInterval: pollInterval,
		SyncFetcher:  nil,
	})
}

// WaitForEmailCountWithSync waits for multiple emails using sync-status-based
// change detection. This method delegates to PollingStrategy for backward compatibility.
func (a *AutoStrategy) WaitForEmailCountWithSync(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, opts WaitOptions) ([]interface{}, error) {
	polling := NewPollingStrategy(a.cfg)
	return polling.WaitForEmailCountWithSync(ctx, inboxHash, fetcher, matcher, count, opts)
}

// Close releases resources and stops the underlying strategy.
// It is equivalent to calling Stop.
func (a *AutoStrategy) Close() error {
	return a.Stop()
}
