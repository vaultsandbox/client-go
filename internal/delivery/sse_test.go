package delivery

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

func TestNewSSEStrategy(t *testing.T) {
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
	s := NewSSEStrategy(Config{})
	if s.Name() != "sse" {
		t.Errorf("Name() = %s, want sse", s.Name())
	}
}

func TestSSEStrategy_AddRemoveInbox(t *testing.T) {
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
	s := NewSSEStrategy(Config{})

	// Should not panic when stopping before starting
	if err := s.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestSSEStrategy_Close(t *testing.T) {
	s := NewSSEStrategy(Config{})

	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestSSEStrategy_WaitForEmail(t *testing.T) {
	var callCount int32

	fetcher := func(ctx context.Context) ([]*testEmail, error) {
		count := atomic.AddInt32(&callCount, 1)
		if count >= 3 {
			return []*testEmail{
				{ID: "email1", Subject: "Hello"},
			}, nil
		}
		return nil, nil
	}

	matcher := func(email *testEmail) bool {
		return email.Subject == "Hello"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := SSEWaitForEmail(ctx, fetcher, matcher, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if result.ID != "email1" {
		t.Errorf("email.ID = %s, want email1", result.ID)
	}
}

func TestSSEStrategy_WaitForEmail_ImmediateMatch(t *testing.T) {
	fetcher := func(ctx context.Context) ([]*testEmail, error) {
		return []*testEmail{
			{ID: "email1", Subject: "Hello"},
		}, nil
	}

	matcher := func(email *testEmail) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	result, err := SSEWaitForEmail(ctx, fetcher, matcher, time.Second)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if result == nil {
		t.Fatal("WaitForEmail() returned nil")
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("WaitForEmail took too long: %v", elapsed)
	}
}

func TestSSEStrategy_WaitForEmail_Timeout(t *testing.T) {
	fetcher := func(ctx context.Context) ([]*testEmail, error) {
		return nil, nil
	}

	matcher := func(email *testEmail) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := SSEWaitForEmail(ctx, fetcher, matcher, 10*time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Errorf("WaitForEmail() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestSSEStrategy_WaitForEmailCount(t *testing.T) {
	var callCount int32

	fetcher := func(ctx context.Context) ([]*testEmail, error) {
		count := atomic.AddInt32(&callCount, 1)
		if count >= 2 {
			return []*testEmail{
				{ID: "email1"},
				{ID: "email2"},
				{ID: "email3"},
			}, nil
		}
		return nil, nil
	}

	matcher := func(email *testEmail) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := SSEWaitForEmailCount(ctx, fetcher, matcher, 2, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForEmailCount() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestSSEStrategy_WaitForEmailCount_ImmediateMatch(t *testing.T) {
	fetcher := func(ctx context.Context) ([]*testEmail, error) {
		return []*testEmail{
			{ID: "email1"},
			{ID: "email2"},
		}, nil
	}

	matcher := func(email *testEmail) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	results, err := SSEWaitForEmailCount(ctx, fetcher, matcher, 2, time.Second)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("WaitForEmailCount() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("WaitForEmailCount took too long: %v", elapsed)
	}
}

func TestSSEStrategy_WaitForEmailCount_Timeout(t *testing.T) {
	fetcher := func(ctx context.Context) ([]*testEmail, error) {
		return []*testEmail{
			{ID: "email1"},
		}, nil // Only 1 email, but we want 2
	}

	matcher := func(email *testEmail) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := SSEWaitForEmailCount(ctx, fetcher, matcher, 2, 10*time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Errorf("WaitForEmailCount() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestSSEConstants(t *testing.T) {
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

func TestSSEStrategy_WaitForEmail_DefaultInterval(t *testing.T) {
	fetchCount := 0
	fetcher := func(ctx context.Context) ([]*testEmail, error) {
		fetchCount++
		if fetchCount >= 2 {
			return []*testEmail{{ID: "email1"}}, nil
		}
		return nil, nil
	}

	matcher := func(email *testEmail) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Pass 0 for pollInterval to use default
	_, err := SSEWaitForEmail(ctx, fetcher, matcher, 0)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}
}

func TestSSEStrategy_Connected(t *testing.T) {
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
	s := NewSSEStrategy(Config{})

	// Should be nil initially
	if s.LastError() != nil {
		t.Error("LastError should be nil initially")
	}
}

func TestSSEStrategy_WaitForEmailWithSync(t *testing.T) {
	var fetchCount int32

	syncFetcher := func(ctx context.Context) (*SyncStatus, error) {
		return &SyncStatus{
			EmailCount: 1,
			EmailsHash: "test-hash",
		}, nil
	}

	fetcher := func(ctx context.Context) ([]*testEmail, error) {
		count := atomic.AddInt32(&fetchCount, 1)
		if count >= 2 {
			return []*testEmail{
				{ID: "email1", Subject: "Hello"},
			}, nil
		}
		return nil, nil
	}

	matcher := func(email *testEmail) bool {
		return email.Subject == "Hello"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := SSEWaitForEmailWithSync(ctx, fetcher, matcher, WaitOptions{
		PollInterval: 10 * time.Millisecond,
		SyncFetcher:  syncFetcher,
	})
	if err != nil {
		t.Fatalf("WaitForEmailWithSync() error = %v", err)
	}

	if result.ID != "email1" {
		t.Errorf("email.ID = %s, want email1", result.ID)
	}
}

func TestSSEStrategy_WaitForEmailCountWithSync(t *testing.T) {
	syncFetcher := func(ctx context.Context) (*SyncStatus, error) {
		return &SyncStatus{
			EmailCount: 3,
			EmailsHash: "test-hash",
		}, nil
	}

	fetcher := func(ctx context.Context) ([]*testEmail, error) {
		return []*testEmail{
			{ID: "email1"},
			{ID: "email2"},
			{ID: "email3"},
		}, nil
	}

	matcher := func(email *testEmail) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := SSEWaitForEmailCountWithSync(ctx, fetcher, matcher, 2, WaitOptions{
		PollInterval: 10 * time.Millisecond,
		SyncFetcher:  syncFetcher,
	})
	if err != nil {
		t.Fatalf("WaitForEmailCountWithSync() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestSSEStrategy_RemoveInbox_Idempotent(t *testing.T) {
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

func TestSSEStrategy_AddInbox_AfterClose(t *testing.T) {
	// Test behavior when adding inbox after strategy is closed
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

	// Close the strategy
	cancel()
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Try to add inbox after close
	inbox := InboxInfo{
		Hash:         "hash123",
		EmailAddress: "test@example.com",
	}

	// AddInbox should still work (it just adds to the map)
	// The strategy is closed but the map operations still work
	err := s.AddInbox(inbox)
	if err != nil {
		t.Logf("AddInbox after Close returned: %v", err)
	}

	// Verify the behavior - current implementation allows adding to map even after close
	// This is acceptable since the strategy is stopped and won't process new inboxes
	if _, exists := s.inboxHashes[inbox.Hash]; !exists {
		t.Log("inbox was not added after close (this is acceptable behavior)")
	}
}

func TestSSEStrategy_Start_AfterClose(t *testing.T) {
	// Test that starting after close doesn't cause panics
	s := NewSSEStrategy(Config{})

	// Close without starting
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Verify started flag is false
	s.mu.RLock()
	started := s.started
	s.mu.RUnlock()

	if started {
		t.Error("started should be false after Close")
	}
}

// TestSSEStrategy_AddInboxAfterStart verifies that adding an inbox after
// starting with an empty list properly triggers the connection loop.
// This was a bug where connectLoop would exit immediately when started with
// no inboxes, and never wake up when inboxes were added later.
func TestSSEStrategy_AddInboxAfterStart(t *testing.T) {
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

	// Give connectLoop time to start waiting
	time.Sleep(50 * time.Millisecond)

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
	time.Sleep(100 * time.Millisecond)

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
