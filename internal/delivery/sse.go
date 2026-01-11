package delivery

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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
// When inboxes are added or removed, the strategy closes the current connection
// and establishes a new one with the updated inbox list.
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
	connCancel    context.CancelFunc   // Cancels the current connection (for reconnection).
	mu            sync.RWMutex         // Protects inboxHashes, handler, and connCancel.
	reconnectWait time.Duration        // Base interval for reconnection backoff.
	attempts      atomic.Int32         // Consecutive failed connection attempts.
	started       bool                 // Whether the strategy is active.
	connected     chan struct{}        // Closed when first connection succeeds.
	connectedOnce sync.Once            // Ensures connected is closed only once.
	lastError     error                // Most recent connection error.
	inboxAdded    chan struct{}        // Signaled when an inbox is added (0→1 case).
	onReconnect   func(ctx context.Context) // Called after each successful connection.
}

// NewSSEStrategy creates a new SSE strategy with the given configuration.
// The strategy is created in a stopped state; call Start to begin listening.
func NewSSEStrategy(cfg Config) *SSEStrategy {
	return &SSEStrategy{
		apiClient:     cfg.APIClient,
		inboxHashes:   make(map[string]struct{}),
		reconnectWait: SSEReconnectInterval,
		connected:     make(chan struct{}),
		inboxAdded:    make(chan struct{}, 1),
	}
}

// Name returns the strategy name for logging and debugging.
func (s *SSEStrategy) Name() string {
	return "sse"
}

// Connected returns a channel that is closed when the first SSE connection
// is successfully established. This can be used to wait for the connection
// before proceeding, or to implement connection timeouts.
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

// Inboxes returns a copy of the currently monitored inbox hashes.
func (s *SSEStrategy) Inboxes() []InboxInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]InboxInfo, 0, len(s.inboxHashes))
	for hash := range s.inboxHashes {
		result = append(result, InboxInfo{Hash: hash})
	}
	return result
}

// OnReconnect sets a callback that is invoked after each successful SSE
// connection (including the first connection). This can be used to sync
// emails that may have arrived during the reconnection window.
func (s *SSEStrategy) OnReconnect(fn func(ctx context.Context)) {
	s.mu.Lock()
	s.onReconnect = fn
	s.mu.Unlock()
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

// AddInbox adds an inbox to be monitored. If the strategy is running,
// this triggers an immediate reconnection with the updated inbox list.
func (s *SSEStrategy) AddInbox(inbox InboxInfo) error {
	s.mu.Lock()
	wasEmpty := len(s.inboxHashes) == 0
	s.inboxHashes[inbox.Hash] = struct{}{}
	started := s.started
	connCancel := s.connCancel
	s.mu.Unlock()

	if !started {
		return nil
	}

	if wasEmpty {
		// Signal that an inbox was added - this wakes up connectLoop if it's
		// waiting for inboxes
		select {
		case s.inboxAdded <- struct{}{}:
		default:
		}
	} else {
		// Already have inboxes and a connection - cancel current connection
		// to force reconnection with the new inbox included
		if connCancel != nil {
			connCancel()
		}
	}

	return nil
}

// RemoveInbox removes an inbox from monitoring. If the strategy is running,
// this triggers an immediate reconnection with the updated inbox list.
func (s *SSEStrategy) RemoveInbox(inboxHash string) error {
	s.mu.Lock()
	delete(s.inboxHashes, inboxHash)
	started := s.started
	connCancel := s.connCancel
	s.mu.Unlock()

	// Cancel current connection to force reconnection without the removed inbox
	if started && connCancel != nil {
		connCancel()
	}

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

		// Check if we have any inboxes to monitor
		s.mu.RLock()
		hasInboxes := len(s.inboxHashes) > 0
		s.mu.RUnlock()

		if !hasInboxes {
			// Wait for an inbox to be added before attempting connection
			select {
			case <-ctx.Done():
				return
			case <-s.inboxAdded:
				// Inbox was added, try to connect
				continue
			}
		}

		err := s.connect(ctx)
		if err == nil {
			// Clean disconnect - reconnect immediately
			continue
		}

		// Check if the main context was canceled (shutdown)
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Check if this was a context.Canceled error from AddInbox/RemoveInbox
		// triggering a reconnection - in that case, reconnect immediately without backoff
		if errors.Is(err, context.Canceled) {
			s.attempts.Store(0)
			continue
		}

		// Handle reconnection with backoff for real errors
		attempts := s.attempts.Add(1)
		if attempts >= SSEMaxReconnectAttempts {
			// Max attempts reached, give up
			return
		}

		wait := s.reconnectWait * time.Duration(1<<(attempts-1))
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
	// Create a child context that can be canceled for reconnection
	connCtx, connCancel := context.WithCancel(ctx)
	defer connCancel()

	// Store the cancel function so AddInbox/RemoveInbox can trigger reconnection
	s.mu.Lock()
	s.connCancel = connCancel
	hashes := make([]string, 0, len(s.inboxHashes))
	for h := range s.inboxHashes {
		hashes = append(hashes, h)
	}
	s.mu.Unlock()

	// Clean up connCancel when we exit
	defer func() {
		s.mu.Lock()
		s.connCancel = nil
		s.mu.Unlock()
	}()

	// Note: connectLoop ensures we have at least one inbox before calling connect,
	// but we still handle empty case gracefully by returning an error
	if len(hashes) == 0 {
		return fmt.Errorf("no inboxes to monitor")
	}

	// Check for nil API client
	if s.apiClient == nil {
		err := fmt.Errorf("SSE strategy: API client is nil")
		s.mu.Lock()
		s.lastError = err
		s.mu.Unlock()
		return err
	}

	resp, err := s.apiClient.OpenEventStream(connCtx, hashes)
	if err != nil {
		s.mu.Lock()
		s.lastError = err
		s.mu.Unlock()
		return err
	}
	defer resp.Body.Close()

	// Reset attempts on successful connection
	s.attempts.Store(0)

	// Signal that connection is established
	s.connectedOnce.Do(func() {
		close(s.connected)
	})

	// Call reconnect handler to sync emails that may have arrived
	// during the reconnection window. Run async to not block the event loop.
	s.mu.RLock()
	onReconnect := s.onReconnect
	s.mu.RUnlock()
	if onReconnect != nil {
		go onReconnect(connCtx)
	}

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
				handler(connCtx, &event)
			}
		}
	}

	return scanner.Err()
}

