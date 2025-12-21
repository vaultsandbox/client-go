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

const (
	SSEReconnectInterval    = 5 * time.Second
	SSEMaxReconnectAttempts = 10
	SSEBackoffMultiplier    = 2
)

// SSEStrategy implements email delivery via Server-Sent Events.
type SSEStrategy struct {
	apiClient     *api.Client
	inboxHashes   map[string]struct{} // Only need hashes for SSE endpoint
	handler       EventHandler
	cancel        context.CancelFunc
	mu            sync.RWMutex
	reconnectWait time.Duration
	attempts      int
	started       bool
	connected     chan struct{} // Signals when first connection is established
	connectedOnce sync.Once
	lastError     error
}

// NewSSEStrategy creates a new SSE strategy.
func NewSSEStrategy(cfg Config) *SSEStrategy {
	return &SSEStrategy{
		apiClient:     cfg.APIClient,
		inboxHashes:   make(map[string]struct{}),
		reconnectWait: SSEReconnectInterval,
		connected:     make(chan struct{}),
	}
}

// Name returns the strategy name.
func (s *SSEStrategy) Name() string {
	return "sse"
}

// Connected returns a channel that's closed when the SSE connection is established.
func (s *SSEStrategy) Connected() <-chan struct{} {
	return s.connected
}

// LastError returns the last connection error, if any.
func (s *SSEStrategy) LastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// Start begins listening for emails on the given inboxes.
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

// Stop gracefully shuts down the strategy.
func (s *SSEStrategy) Stop() error {
	s.mu.Lock()
	s.started = false
	s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

// AddInbox adds an inbox to monitor.
func (s *SSEStrategy) AddInbox(inbox InboxInfo) error {
	s.mu.Lock()
	s.inboxHashes[inbox.Hash] = struct{}{}
	s.mu.Unlock()
	// Trigger reconnection with new inbox set would require
	// closing the current connection. For now, new inboxes
	// will be picked up on the next reconnection.
	return nil
}

// RemoveInbox removes an inbox from monitoring.
func (s *SSEStrategy) RemoveInbox(inboxHash string) error {
	s.mu.Lock()
	delete(s.inboxHashes, inboxHash)
	s.mu.Unlock()
	return nil
}

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

// Legacy interface implementation for backward compatibility.

// WaitForEmail waits for an email using SSE (with polling fallback).
func (s *SSEStrategy) WaitForEmail(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, pollInterval time.Duration) (interface{}, error) {
	return s.WaitForEmailWithSync(ctx, inboxHash, fetcher, matcher, WaitOptions{
		PollInterval: pollInterval,
		SyncFetcher:  nil,
	})
}

// WaitForEmailWithSync waits for an email using sync-status-based change detection.
// SSE strategy delegates to polling for WaitForEmail operations for backward compatibility.
func (s *SSEStrategy) WaitForEmailWithSync(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, opts WaitOptions) (interface{}, error) {
	// SSE strategy uses polling for WaitForEmail operations
	polling := &PollingStrategy{apiClient: s.apiClient}
	return polling.WaitForEmailWithSync(ctx, inboxHash, fetcher, matcher, opts)
}

// WaitForEmailCount waits for multiple emails using SSE (with polling fallback).
func (s *SSEStrategy) WaitForEmailCount(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, pollInterval time.Duration) ([]interface{}, error) {
	return s.WaitForEmailCountWithSync(ctx, inboxHash, fetcher, matcher, count, WaitOptions{
		PollInterval: pollInterval,
		SyncFetcher:  nil,
	})
}

// WaitForEmailCountWithSync waits for multiple emails using sync-status-based change detection.
// SSE strategy delegates to polling for WaitForEmailCount operations for backward compatibility.
func (s *SSEStrategy) WaitForEmailCountWithSync(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, opts WaitOptions) ([]interface{}, error) {
	// SSE strategy uses polling for WaitForEmailCount operations
	polling := &PollingStrategy{apiClient: s.apiClient}
	return polling.WaitForEmailCountWithSync(ctx, inboxHash, fetcher, matcher, count, opts)
}

// Close closes the SSE strategy.
func (s *SSEStrategy) Close() error {
	return s.Stop()
}
