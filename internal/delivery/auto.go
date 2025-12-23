package delivery

import (
	"context"
	"sync"
	"time"
)

// AutoStrategy implements automatic fallback from SSE to polling.
// It starts with SSE and falls back to polling if SSE doesn't connect
// within the configured timeout.
type AutoStrategy struct {
	cfg         Config
	sse         *SSEStrategy
	polling     *PollingStrategy
	active      Strategy
	handler     EventHandler
	onReconnect func(ctx context.Context)
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	switched    bool
	timeout     time.Duration

	// Track inboxes for fallback (since SSEStrategy.inboxHashes is unexported)
	inboxes []InboxInfo
}

// NewAutoStrategy creates a new auto strategy that starts with SSE
// and falls back to polling if SSE doesn't connect in time.
func NewAutoStrategy(cfg Config) *AutoStrategy {
	timeout := cfg.SSEConnectionTimeout
	if timeout == 0 {
		timeout = DefaultSSEConnectionTimeout
	}
	return &AutoStrategy{
		cfg:     cfg,
		timeout: timeout,
	}
}

// Name returns the strategy name, indicating whether it's currently using SSE or polling.
func (a *AutoStrategy) Name() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.switched {
		return "auto:polling"
	}
	return "auto:sse"
}

// Start begins listening for emails on the given inboxes via SSE.
// If SSE doesn't connect within the timeout, it automatically falls back to polling.
func (a *AutoStrategy) Start(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error {
	a.mu.Lock()
	a.handler = handler
	a.ctx, a.cancel = context.WithCancel(ctx)
	a.inboxes = make([]InboxInfo, len(inboxes))
	copy(a.inboxes, inboxes)

	// Create both strategies
	a.sse = NewSSEStrategy(a.cfg)
	a.polling = NewPollingStrategy(a.cfg)
	a.active = a.sse
	a.mu.Unlock()

	// Start SSE
	if err := a.sse.Start(a.ctx, inboxes, handler); err != nil {
		return err
	}

	// Register reconnect handler on SSE
	if a.onReconnect != nil {
		a.sse.OnReconnect(a.onReconnect)
	}

	// Monitor SSE connection in background
	go a.monitorConnection()

	return nil
}

// monitorConnection waits for SSE to connect or times out and falls back to polling.
func (a *AutoStrategy) monitorConnection() {
	select {
	case <-a.sse.Connected():
		// SSE connected successfully, keep using it
		return
	case <-time.After(a.timeout):
		// SSE didn't connect in time, fall back to polling
		a.fallbackToPolling()
	case <-a.ctx.Done():
		// Context canceled, don't switch
		return
	}
}

// fallbackToPolling switches from SSE to polling strategy.
func (a *AutoStrategy) fallbackToPolling() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.switched {
		return // Already switched
	}

	// Get current inboxes (including any added after Start)
	currentInboxes := make([]InboxInfo, len(a.inboxes))
	copy(currentInboxes, a.inboxes)

	// Start polling first, before stopping SSE
	if err := a.polling.Start(a.ctx, currentInboxes, a.handler); err != nil {
		return // Keep SSE running on polling start failure
	}

	// Only stop SSE after polling successfully started
	a.sse.Stop()

	// Register reconnect handler on polling
	if a.onReconnect != nil {
		a.polling.OnReconnect(a.onReconnect)
	}

	a.active = a.polling
	a.switched = true
}

// Stop gracefully shuts down the strategy.
func (a *AutoStrategy) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cancel != nil {
		a.cancel()
	}

	if a.active != nil {
		return a.active.Stop()
	}
	return nil
}

// AddInbox adds an inbox to be monitored.
func (a *AutoStrategy) AddInbox(inbox InboxInfo) error {
	a.mu.Lock()
	a.inboxes = append(a.inboxes, inbox)
	active := a.active
	a.mu.Unlock()

	if active != nil {
		return active.AddInbox(inbox)
	}
	return nil
}

// RemoveInbox removes an inbox from monitoring.
func (a *AutoStrategy) RemoveInbox(inboxHash string) error {
	a.mu.Lock()
	// Remove from tracked inboxes
	for i, inbox := range a.inboxes {
		if inbox.Hash == inboxHash {
			a.inboxes = append(a.inboxes[:i], a.inboxes[i+1:]...)
			break
		}
	}
	active := a.active
	a.mu.Unlock()

	if active != nil {
		return active.RemoveInbox(inboxHash)
	}
	return nil
}

// OnReconnect sets a callback that is invoked after each successful connection.
func (a *AutoStrategy) OnReconnect(fn func(ctx context.Context)) {
	a.mu.Lock()
	a.onReconnect = fn
	active := a.active
	a.mu.Unlock()

	if active != nil {
		active.OnReconnect(fn)
	}
}

// Connected returns a channel that is closed when the SSE connection is established.
// This is useful for checking if SSE connected before fallback occurred.
func (a *AutoStrategy) Connected() <-chan struct{} {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.sse != nil {
		return a.sse.Connected()
	}
	// Return a closed channel if SSE not initialized
	ch := make(chan struct{})
	close(ch)
	return ch
}

// Switched returns true if the strategy has fallen back to polling.
func (a *AutoStrategy) Switched() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.switched
}
