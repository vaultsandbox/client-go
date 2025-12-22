package delivery

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

// SSE reconnection constants control the behavior when the SSE connection
// is lost. The strategy uses exponential backoff between reconnection attempts.
const (
	// SSEReconnectInterval is the base interval between reconnection attempts.
	SSEReconnectInterval = 5 * time.Second

	// SSEMaxReconnectAttempts is the maximum number of consecutive reconnection
	// attempts before giving up. After this many failures, the strategy stops.
	SSEMaxReconnectAttempts = 10

	// SSEBackoffMultiplier is the factor by which the reconnect interval
	// increases after each failed attempt (exponential backoff).
	SSEBackoffMultiplier = 2
)

// SSEStrategy implements email delivery via Server-Sent Events (SSE).
// SSE provides real-time push notifications with lower latency than polling.
//
// The strategy maintains a persistent HTTP connection to the server and
// receives events as they occur. If the connection is lost, it automatically
// reconnects with exponential backoff up to SSEMaxReconnectAttempts.
//
// SSE Protocol: The server sends events in the standard SSE format:
//
//	data: {"inbox_id":"...","email_id":"...","encrypted_metadata":"..."}
//
// Lines starting with ":" are comments (used for keep-alive) and are ignored.
// Empty lines delimit events.
type SSEStrategy struct {
	apiClient     *api.Client          // API client for establishing connections.
	inboxHashes   map[string]struct{}  // Set of inbox hashes to monitor.
	handler       EventHandler         // Callback for new email events.
	cancel        context.CancelFunc   // Cancels the connection goroutine.
	mu            sync.RWMutex         // Protects inboxHashes and handler.
	reconnectWait time.Duration        // Base interval for reconnection backoff.
	attempts      int                  // Consecutive failed connection attempts.
	started       bool                 // Whether the strategy is active.
	connected     chan struct{}        // Closed when first connection succeeds.
	connectedOnce sync.Once            // Ensures connected is closed only once.
	lastError     error                // Most recent connection error.
}

// NewSSEStrategy creates a new SSE strategy with the given configuration.
// The strategy is created in a stopped state; call Start to begin listening.
func NewSSEStrategy(cfg Config) *SSEStrategy {
	return &SSEStrategy{
		apiClient:     cfg.APIClient,
		inboxHashes:   make(map[string]struct{}),
		reconnectWait: SSEReconnectInterval,
		connected:     make(chan struct{}),
	}
}

// Name returns the strategy name for logging and debugging.
func (s *SSEStrategy) Name() string {
	return "sse"
}

// Connected returns a channel that is closed when the first SSE connection
// is successfully established. This can be used to wait for the connection
// before proceeding, or to implement connection timeouts (see AutoStrategy).
func (s *SSEStrategy) Connected() <-chan struct{} {
	return s.connected
}

// LastError returns the most recent connection error, or nil if no error
// has occurred. This is useful for diagnosing connection failures.
func (s *SSEStrategy) LastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// Start begins listening for emails on the given inboxes via SSE. It spawns
// a background goroutine that maintains a persistent connection to the server
// and calls the handler for each new email event.
//
// The connection is established asynchronously. Use the Connected() channel
// to wait for the connection to be established. If the connection fails,
// Start still returns nil and reconnection attempts happen in the background.
func (s *SSEStrategy) Start(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error {
	s.mu.Lock()
	for _, inbox := range inboxes {
		s.inboxHashes[inbox.Hash] = struct{}{}
	}
	s.handler = handler
	s.started = true
	s.mu.Unlock()

	ctx, s.cancel = context.WithCancel(ctx)
	go s.connectLoop(ctx)
	return nil
}

// Stop gracefully shuts down the SSE strategy. It closes the active connection
// and stops reconnection attempts. Stop is idempotent and safe to call multiple
// times. After Stop returns, no more events will be delivered.
func (s *SSEStrategy) Stop() error {
	s.mu.Lock()
	s.started = false
	s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

// AddInbox adds an inbox to be monitored. Note that SSE connections are
// established with a fixed set of inbox hashes, so newly added inboxes
// will only be monitored after the next reconnection. To force immediate
// inclusion, stop and restart the strategy.
func (s *SSEStrategy) AddInbox(inbox InboxInfo) error {
	s.mu.Lock()
	s.inboxHashes[inbox.Hash] = struct{}{}
	s.mu.Unlock()
	// Trigger reconnection with new inbox set would require
	// closing the current connection. For now, new inboxes
	// will be picked up on the next reconnection.
	return nil
}

// RemoveInbox removes an inbox from monitoring. The inbox will no longer
// receive events after the current connection closes. This method is safe
// to call while the strategy is active.
func (s *SSEStrategy) RemoveInbox(inboxHash string) error {
	s.mu.Lock()
	delete(s.inboxHashes, inboxHash)
	s.mu.Unlock()
	return nil
}

// connectLoop manages the SSE connection lifecycle, handling reconnection
// with exponential backoff when the connection is lost.
func (s *SSEStrategy) connectLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := s.connect(ctx)
		if err == nil {
			// Clean disconnect
			return
		}

		// Handle reconnection with backoff
		s.attempts++
		if s.attempts >= SSEMaxReconnectAttempts {
			// Max attempts reached, give up
			return
		}

		wait := s.reconnectWait * time.Duration(1<<(s.attempts-1))
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// connect establishes an SSE connection and processes events until the
// connection closes or an error occurs. Returns nil on clean disconnect,
// or an error if the connection failed.
func (s *SSEStrategy) connect(ctx context.Context) error {
	s.mu.RLock()
	hashes := make([]string, 0, len(s.inboxHashes))
	for h := range s.inboxHashes {
		hashes = append(hashes, h)
	}
	s.mu.RUnlock()

	if len(hashes) == 0 {
		return nil
	}

	// Check for nil API client
	if s.apiClient == nil {
		err := fmt.Errorf("SSE strategy: API client is nil")
		s.mu.Lock()
		s.lastError = err
		s.mu.Unlock()
		return err
	}

	resp, err := s.apiClient.OpenEventStream(ctx, hashes)
	if err != nil {
		s.mu.Lock()
		s.lastError = err
		s.mu.Unlock()
		return err
	}
	defer resp.Body.Close()

	// Reset attempts on successful connection
	s.attempts = 0

	// Signal that connection is established
	s.connectedOnce.Do(func() {
		close(s.connected)
	})

	scanner := bufio.NewScanner(resp.Body)
	// Allow lines up to 1MB (default is 64KB)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Parse SSE data line
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			var event api.SSEEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue // Skip malformed events
			}

			s.mu.RLock()
			handler := s.handler
			s.mu.RUnlock()

			if handler != nil {
				handler(&event)
			}
		}
	}

	return scanner.Err()
}

// WaitForEmail waits for an email matching the given criteria.
// SSE strategy delegates to PollingStrategy for WaitForEmail operations.
func (s *SSEStrategy) WaitForEmail(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, pollInterval time.Duration) (interface{}, error) {
	return s.WaitForEmailWithSync(ctx, inboxHash, fetcher, matcher, WaitOptions{
		PollInterval: pollInterval,
		SyncFetcher:  nil,
	})
}

// WaitForEmailWithSync waits for an email using sync-status-based change detection.
// SSE strategy delegates to PollingStrategy for WaitForEmail operations.
func (s *SSEStrategy) WaitForEmailWithSync(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, opts WaitOptions) (interface{}, error) {
	// SSE strategy uses polling for WaitForEmail operations
	polling := &PollingStrategy{apiClient: s.apiClient}
	return polling.WaitForEmailWithSync(ctx, inboxHash, fetcher, matcher, opts)
}

// WaitForEmailCount waits until at least count emails match the criteria.
// SSE strategy delegates to PollingStrategy for WaitForEmail operations.
func (s *SSEStrategy) WaitForEmailCount(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, pollInterval time.Duration) ([]interface{}, error) {
	return s.WaitForEmailCountWithSync(ctx, inboxHash, fetcher, matcher, count, WaitOptions{
		PollInterval: pollInterval,
		SyncFetcher:  nil,
	})
}

// WaitForEmailCountWithSync waits for multiple emails using sync-status-based
// change detection. SSE strategy delegates to PollingStrategy.
func (s *SSEStrategy) WaitForEmailCountWithSync(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, opts WaitOptions) ([]interface{}, error) {
	// SSE strategy uses polling for WaitForEmailCount operations
	polling := &PollingStrategy{apiClient: s.apiClient}
	return polling.WaitForEmailCountWithSync(ctx, inboxHash, fetcher, matcher, count, opts)
}

// Close releases resources and stops the SSE strategy.
// It is equivalent to calling Stop.
func (s *SSEStrategy) Close() error {
	return s.Stop()
}
