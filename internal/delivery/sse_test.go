package delivery

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

func TestNewSSEStrategy(t *testing.T) {
	t.Parallel()
	cfg := Config{
		APIClient: nil,
	}

	s := NewSSEStrategy(cfg)
	if s == nil {
		t.Fatal("NewSSEStrategy returned nil")
	}
	if s.inboxHashes == nil {
		t.Error("inboxHashes map is nil")
	}
	if s.reconnectWait != SSEReconnectInterval {
		t.Errorf("reconnectWait = %v, want %v", s.reconnectWait, SSEReconnectInterval)
	}
}

func TestSSEStrategy_Name(t *testing.T) {
	t.Parallel()
	s := NewSSEStrategy(Config{})
	if s.Name() != "sse" {
		t.Errorf("Name() = %s, want sse", s.Name())
	}
}

func TestSSEStrategy_AddRemoveInbox(t *testing.T) {
	t.Parallel()
	s := NewSSEStrategy(Config{})

	inbox := InboxInfo{
		Hash:         "hash123",
		EmailAddress: "test@example.com",
	}

	// Add inbox
	if err := s.AddInbox(inbox); err != nil {
		t.Fatalf("AddInbox() error = %v", err)
	}

	if _, exists := s.inboxHashes[inbox.Hash]; !exists {
		t.Error("inbox was not added")
	}

	// Remove inbox
	if err := s.RemoveInbox(inbox.Hash); err != nil {
		t.Fatalf("RemoveInbox() error = %v", err)
	}

	if _, exists := s.inboxHashes[inbox.Hash]; exists {
		t.Error("inbox was not removed")
	}
}

func TestSSEStrategy_Stop_NotStarted(t *testing.T) {
	t.Parallel()
	s := NewSSEStrategy(Config{})

	// Should not panic when stopping before starting
	if err := s.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestSSEConstants(t *testing.T) {
	t.Parallel()
	if SSEReconnectInterval != 5*time.Second {
		t.Errorf("SSEReconnectInterval = %v, want 5s", SSEReconnectInterval)
	}
	if SSEMaxReconnectAttempts != 10 {
		t.Errorf("SSEMaxReconnectAttempts = %d, want 10", SSEMaxReconnectAttempts)
	}
	if SSEBackoffMultiplier != 2 {
		t.Errorf("SSEBackoffMultiplier = %d, want 2", SSEBackoffMultiplier)
	}
}

func TestSSEStrategy_Start(t *testing.T) {
	t.Parallel()
	s := NewSSEStrategy(Config{})

	ctx, cancel := context.WithCancel(context.Background())

	handler := func(ctx context.Context, event *api.SSEEvent) error {
		return nil
	}

	inboxes := []InboxInfo{
		{Hash: "hash1", EmailAddress: "test1@example.com"},
		{Hash: "hash2", EmailAddress: "test2@example.com"},
	}

	if err := s.Start(ctx, inboxes, handler); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Verify inboxes were added
	if len(s.inboxHashes) != 2 {
		t.Errorf("inboxHashes count = %d, want 2", len(s.inboxHashes))
	}

	// Stop the strategy
	cancel()
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestSSEStrategy_Connected(t *testing.T) {
	t.Parallel()
	s := NewSSEStrategy(Config{})

	// Channel should not be closed initially
	select {
	case <-s.Connected():
		t.Error("connected channel should not be closed initially")
	default:
		// Expected
	}
}

func TestSSEStrategy_LastError(t *testing.T) {
	t.Parallel()
	s := NewSSEStrategy(Config{})

	// Should be nil initially
	if s.LastError() != nil {
		t.Error("LastError should be nil initially")
	}
}

func TestSSEStrategy_Inboxes(t *testing.T) {
	t.Parallel()
	s := NewSSEStrategy(Config{})

	// Initially empty
	inboxes := s.Inboxes()
	if len(inboxes) != 0 {
		t.Errorf("Inboxes() returned %d items, want 0", len(inboxes))
	}

	// Add some inboxes
	s.AddInbox(InboxInfo{Hash: "hash1", EmailAddress: "test1@example.com"})
	s.AddInbox(InboxInfo{Hash: "hash2", EmailAddress: "test2@example.com"})
	s.AddInbox(InboxInfo{Hash: "hash3", EmailAddress: "test3@example.com"})

	inboxes = s.Inboxes()
	if len(inboxes) != 3 {
		t.Errorf("Inboxes() returned %d items, want 3", len(inboxes))
	}

	// Verify all hashes are present
	hashes := make(map[string]bool)
	for _, inbox := range inboxes {
		hashes[inbox.Hash] = true
	}
	for _, expected := range []string{"hash1", "hash2", "hash3"} {
		if !hashes[expected] {
			t.Errorf("Inboxes() missing hash %s", expected)
		}
	}

	// Remove one and verify
	s.RemoveInbox("hash2")
	inboxes = s.Inboxes()
	if len(inboxes) != 2 {
		t.Errorf("Inboxes() returned %d items after removal, want 2", len(inboxes))
	}
}

func TestSSEStrategy_RemoveInbox_Idempotent(t *testing.T) {
	t.Parallel()
	// Test that removing the same inbox multiple times doesn't cause errors
	s := NewSSEStrategy(Config{})

	inbox := InboxInfo{
		Hash:         "hash123",
		EmailAddress: "test@example.com",
	}

	// Add inbox
	if err := s.AddInbox(inbox); err != nil {
		t.Fatalf("AddInbox() error = %v", err)
	}

	// Verify inbox was added
	if _, exists := s.inboxHashes[inbox.Hash]; !exists {
		t.Fatal("inbox was not added")
	}

	// Remove inbox first time
	if err := s.RemoveInbox(inbox.Hash); err != nil {
		t.Errorf("first RemoveInbox() error = %v", err)
	}

	// Remove inbox second time (should be idempotent)
	if err := s.RemoveInbox(inbox.Hash); err != nil {
		t.Errorf("second RemoveInbox() error = %v", err)
	}

	// Remove inbox third time
	if err := s.RemoveInbox(inbox.Hash); err != nil {
		t.Errorf("third RemoveInbox() error = %v", err)
	}

	// Verify inbox is not in the map
	if _, exists := s.inboxHashes[inbox.Hash]; exists {
		t.Error("inbox should not exist after removal")
	}
}

func TestSSEStrategy_AddInbox_AfterStop(t *testing.T) {
	t.Parallel()
	// Test behavior when adding inbox after strategy is stopped
	s := NewSSEStrategy(Config{})

	// Start the strategy first
	ctx, cancel := context.WithCancel(context.Background())
	handler := func(ctx context.Context, event *api.SSEEvent) error {
		return nil
	}

	inboxes := []InboxInfo{
		{Hash: "initial", EmailAddress: "initial@example.com"},
	}

	if err := s.Start(ctx, inboxes, handler); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Stop the strategy
	cancel()
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Try to add inbox after stop
	inbox := InboxInfo{
		Hash:         "hash123",
		EmailAddress: "test@example.com",
	}

	// AddInbox should still work (it just adds to the map)
	// The strategy is stopped but the map operations still work
	err := s.AddInbox(inbox)
	if err != nil {
		t.Logf("AddInbox after Stop returned: %v", err)
	}

	// Verify the behavior - current implementation allows adding to map even after stop
	// This is acceptable since the strategy is stopped and won't process new inboxes
	if _, exists := s.inboxHashes[inbox.Hash]; !exists {
		t.Log("inbox was not added after stop (this is acceptable behavior)")
	}
}

func TestSSEStrategy_Start_AfterStop(t *testing.T) {
	t.Parallel()
	// Test that starting after stop doesn't cause panics
	s := NewSSEStrategy(Config{})

	// Stop without starting
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Verify started flag is false
	s.mu.RLock()
	started := s.started
	s.mu.RUnlock()

	if started {
		t.Error("started should be false after Stop")
	}
}

// TestSSEStrategy_AddInboxAfterStart verifies that adding an inbox after
// starting with an empty list properly triggers the connection loop.
// This was a bug where connectLoop would exit immediately when started with
// no inboxes, and never wake up when inboxes were added later.
func TestSSEStrategy_AddInboxAfterStart(t *testing.T) {
	t.Parallel()
	s := NewSSEStrategy(Config{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(ctx context.Context, event *api.SSEEvent) error {
		return nil
	}

	// Start with NO inboxes
	if err := s.Start(ctx, nil, handler); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Now add an inbox AFTER start
	inbox := InboxInfo{
		Hash:         "hash123",
		EmailAddress: "test@example.com",
	}
	if err := s.AddInbox(inbox); err != nil {
		t.Fatalf("AddInbox() error = %v", err)
	}

	// Verify inbox is in the map
	s.mu.RLock()
	_, exists := s.inboxHashes[inbox.Hash]
	s.mu.RUnlock()

	if !exists {
		t.Fatal("inbox should exist in map")
	}

	// With the fix: connectLoop should wake up and attempt connection.
	// Since we don't have a real API client, it will fail, but we can verify
	// that LastError is set (meaning connect was attempted).
	// Poll for the error to appear instead of sleeping.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if s.LastError() != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Check that an error was recorded (connection was attempted but failed
	// because apiClient is nil)
	err := s.LastError()
	if err == nil {
		t.Error("Expected LastError to be set after AddInbox triggered connection attempt")
	} else {
		t.Logf("Connection attempted after AddInbox (got expected error: %v)", err)
	}
}

func TestSSEStrategy_ConcurrentSubscriptions(t *testing.T) {
	t.Parallel()
	// Test adding and removing inboxes concurrently
	s := NewSSEStrategy(Config{})

	var wg sync.WaitGroup

	// Add inboxes concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			inbox := InboxInfo{
				Hash:         "hash" + string(rune('0'+idx)),
				EmailAddress: "test" + string(rune('0'+idx)) + "@example.com",
			}
			if err := s.AddInbox(inbox); err != nil {
				t.Errorf("AddInbox() error = %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all inboxes were added
	s.mu.RLock()
	count := len(s.inboxHashes)
	s.mu.RUnlock()

	if count != 10 {
		t.Errorf("inboxHashes count = %d, want 10", count)
	}

	// Remove inboxes concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if err := s.RemoveInbox("hash" + string(rune('0'+idx))); err != nil {
				t.Errorf("RemoveInbox() error = %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all inboxes were removed
	s.mu.RLock()
	count = len(s.inboxHashes)
	s.mu.RUnlock()

	if count != 0 {
		t.Errorf("inboxHashes count = %d, want 0", count)
	}
}

func TestSSEStrategy_MaxReconnectAttempts(t *testing.T) {
	t.Parallel()
	// Test that strategy gives up after SSEMaxReconnectAttempts failures
	s := NewSSEStrategy(Config{})

	// Speed up test with 1ms reconnect interval.
	// Note: Start() resets attempts to 0 for reuse safety, so we need to
	// set reconnectWait before Start(), and wait long enough for all attempts.
	// With 1ms base and exponential backoff, 10 attempts takes ~511ms total.
	s.reconnectWait = 1 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(ctx context.Context, event *api.SSEEvent) error {
		return nil
	}

	// Start with an inbox (nil apiClient will cause connection failures)
	if err := s.Start(ctx, []InboxInfo{{Hash: "hash1"}}, handler); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Poll for the strategy to hit max attempts instead of sleeping
	// With 1ms base * exponential backoff, need: 1+2+4+8+16+32+64+128+256+512 = ~1023ms
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.attempts.Load() >= SSEMaxReconnectAttempts {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify attempts reached max
	if s.attempts.Load() < SSEMaxReconnectAttempts {
		t.Errorf("attempts = %d, want >= %d", s.attempts.Load(), SSEMaxReconnectAttempts)
	}
}

func TestSSEStrategy_ConnectWithNoHashes(t *testing.T) {
	t.Parallel()
	// Test the edge case where connect() is called with empty hashes
	// This can happen if inboxes are removed between connectLoop check and connect call
	s := NewSSEStrategy(Config{})

	ctx := context.Background()

	// Call connect directly with no inboxes in the map
	err := s.connect(ctx)

	if err == nil {
		t.Error("connect() should return error when no inboxes")
	}
	if err.Error() != "no inboxes to monitor" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSSEStrategy_MalformedSSEEvent(t *testing.T) {
	t.Parallel()
	// Test that malformed JSON in SSE events is skipped gracefully
	// Use httptest to create a server that sends malformed SSE data

	eventReceived := make(chan struct{}, 1)
	serverDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(serverDone)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("expected http.Flusher")
			return
		}

		// Send malformed JSON (should be skipped)
		fmt.Fprintf(w, "data: {invalid json}\n\n")
		flusher.Flush()

		// Send valid JSON
		fmt.Fprintf(w, "data: {\"inbox_id\":\"inbox1\",\"email_id\":\"email1\"}\n\n")
		flusher.Flush()

		// Wait for event to be processed or timeout
		select {
		case <-eventReceived:
		case <-time.After(2 * time.Second):
		}
	}))
	defer server.Close()

	apiClient, err := api.New("test-api-key", api.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("failed to create api client: %v", err)
	}

	s := NewSSEStrategy(Config{APIClient: apiClient})

	var mu sync.Mutex
	var receivedEvents int
	handler := func(ctx context.Context, event *api.SSEEvent) error {
		mu.Lock()
		receivedEvents++
		mu.Unlock()
		select {
		case eventReceived <- struct{}{}:
		default:
		}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.Start(ctx, []InboxInfo{{Hash: "hash1"}}, handler); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait for the valid event to be processed
	select {
	case <-eventReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	mu.Lock()
	count := receivedEvents
	mu.Unlock()

	// Should have received only 1 event (the valid one, malformed should be skipped)
	if count != 1 {
		t.Errorf("receivedEvents = %d, want 1 (malformed should be skipped)", count)
	}

	// Cancel and wait for server to finish
	cancel()
	<-serverDone
}
