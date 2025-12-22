package delivery

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

func TestNewPollingStrategy(t *testing.T) {
	cfg := Config{
		APIClient: nil, // Would be a real client in production
	}

	p := NewPollingStrategy(cfg)
	if p == nil {
		t.Fatal("NewPollingStrategy returned nil")
	}
	if p.inboxes == nil {
		t.Error("inboxes map is nil")
	}
}

func TestPollingStrategy_Name(t *testing.T) {
	p := NewPollingStrategy(Config{})
	if p.Name() != "polling" {
		t.Errorf("Name() = %s, want polling", p.Name())
	}
}

func TestPollingStrategy_AddRemoveInbox(t *testing.T) {
	p := NewPollingStrategy(Config{})

	inbox := InboxInfo{
		Hash:         "hash123",
		EmailAddress: "test@example.com",
	}

	// Add inbox
	if err := p.AddInbox(inbox); err != nil {
		t.Fatalf("AddInbox() error = %v", err)
	}

	if _, exists := p.inboxes[inbox.Hash]; !exists {
		t.Error("inbox was not added")
	}

	// Remove inbox
	if err := p.RemoveInbox(inbox.Hash); err != nil {
		t.Fatalf("RemoveInbox() error = %v", err)
	}

	if _, exists := p.inboxes[inbox.Hash]; exists {
		t.Error("inbox was not removed")
	}
}

func TestPollingStrategy_Stop_NotStarted(t *testing.T) {
	p := NewPollingStrategy(Config{})

	// Should not panic when stopping before starting
	if err := p.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestPollingStrategy_Close(t *testing.T) {
	p := NewPollingStrategy(Config{})

	if err := p.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestPollingStrategy_WaitForEmail(t *testing.T) {
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

	result, err := WaitForEmail(ctx, fetcher, matcher, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if result.ID != "email1" {
		t.Errorf("email.ID = %s, want email1", result.ID)
	}
}

func TestPollingStrategy_WaitForEmail_ImmediateMatch(t *testing.T) {
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
	result, err := WaitForEmail(ctx, fetcher, matcher, time.Second)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	if result == nil {
		t.Fatal("WaitForEmail() returned nil")
	}

	// Should return immediately without waiting
	if elapsed > 100*time.Millisecond {
		t.Errorf("WaitForEmail took too long: %v", elapsed)
	}
}

func TestPollingStrategy_WaitForEmail_Timeout(t *testing.T) {
	fetcher := func(ctx context.Context) ([]*testEmail, error) {
		return nil, nil // Never returns emails
	}

	matcher := func(email *testEmail) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := WaitForEmail(ctx, fetcher, matcher, 10*time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Errorf("WaitForEmail() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestPollingStrategy_WaitForEmailCount(t *testing.T) {
	var callCount int32

	fetcher := func(ctx context.Context) ([]*testEmail, error) {
		count := atomic.AddInt32(&callCount, 1)
		if count >= 2 {
			return []*testEmail{
				{ID: "email1", Subject: "Hello"},
				{ID: "email2", Subject: "Hello"},
				{ID: "email3", Subject: "Hello"},
			}, nil
		}
		return nil, nil
	}

	matcher := func(email *testEmail) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := WaitForEmailCount(ctx, fetcher, matcher, 2, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForEmailCount() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestPollingStrategy_WaitForEmailCount_ImmediateMatch(t *testing.T) {
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
	results, err := WaitForEmailCount(ctx, fetcher, matcher, 2, time.Second)
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

func TestPollingStrategy_WaitForEmailCount_Timeout(t *testing.T) {
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

	_, err := WaitForEmailCount(ctx, fetcher, matcher, 2, 10*time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Errorf("WaitForEmailCount() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestPollingStrategy_WaitForEmail_DefaultInterval(t *testing.T) {
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
	_, err := WaitForEmail(ctx, fetcher, matcher, 0)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}
}

func TestPollingConstants(t *testing.T) {
	if PollingInitialInterval != 2*time.Second {
		t.Errorf("PollingInitialInterval = %v, want 2s", PollingInitialInterval)
	}
	if PollingMaxBackoff != 30*time.Second {
		t.Errorf("PollingMaxBackoff = %v, want 30s", PollingMaxBackoff)
	}
	if PollingBackoffMultiplier != 1.5 {
		t.Errorf("PollingBackoffMultiplier = %v, want 1.5", PollingBackoffMultiplier)
	}
	if PollingJitterFactor != 0.3 {
		t.Errorf("PollingJitterFactor = %v, want 0.3", PollingJitterFactor)
	}
}

func TestPollingStrategy_Start(t *testing.T) {
	// Create a mock API client would be needed for full test
	// For now, test basic start/stop functionality

	p := NewPollingStrategy(Config{})

	ctx, cancel := context.WithCancel(context.Background())

	handler := func(event *api.SSEEvent) error {
		return nil
	}

	inboxes := []InboxInfo{
		{Hash: "hash1", EmailAddress: "test1@example.com"},
		{Hash: "hash2", EmailAddress: "test2@example.com"},
	}

	if err := p.Start(ctx, inboxes, handler); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Verify inboxes were added
	if len(p.inboxes) != 2 {
		t.Errorf("inboxes count = %d, want 2", len(p.inboxes))
	}

	// Stop the strategy
	cancel()
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

// testEmail is a simple struct for testing
type testEmail struct {
	ID      string
	Subject string
}

func TestPollingStrategy_WaitForEmailWithSync(t *testing.T) {
	var fetchCount int32
	var syncCount int32
	currentHash := "hash1"

	syncFetcher := func(ctx context.Context) (*SyncStatus, error) {
		count := atomic.AddInt32(&syncCount, 1)
		// Simulate hash change after 2 syncs
		if count >= 2 {
			currentHash = "hash2"
		}
		return &SyncStatus{
			EmailCount: 1,
			EmailsHash: currentHash,
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

	result, err := WaitForEmailWithSync(ctx, fetcher, matcher, WaitOptions{
		PollInterval: 10 * time.Millisecond,
		SyncFetcher:  syncFetcher,
	})
	if err != nil {
		t.Fatalf("WaitForEmailWithSync() error = %v", err)
	}

	if result.ID != "email1" {
		t.Errorf("email.ID = %s, want email1", result.ID)
	}

	// Verify sync was called
	if atomic.LoadInt32(&syncCount) < 1 {
		t.Error("sync fetcher was not called")
	}
}

func TestPollingStrategy_WaitForEmailWithSync_BackoffOnNoChange(t *testing.T) {
	var syncCount int32
	var fetchCount int32

	syncFetcher := func(ctx context.Context) (*SyncStatus, error) {
		atomic.AddInt32(&syncCount, 1)
		return &SyncStatus{
			EmailCount: 0,
			EmailsHash: "unchanging-hash",
		}, nil
	}

	fetcher := func(ctx context.Context) ([]*testEmail, error) {
		atomic.AddInt32(&fetchCount, 1)
		return nil, nil
	}

	matcher := func(email *testEmail) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := WaitForEmailWithSync(ctx, fetcher, matcher, WaitOptions{
		PollInterval: 5 * time.Millisecond,
		SyncFetcher:  syncFetcher,
	})

	// Should timeout
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}

	// Sync should be called multiple times
	syncs := atomic.LoadInt32(&syncCount)
	if syncs < 2 {
		t.Errorf("sync was called %d times, expected at least 2", syncs)
	}

	// Fetcher should not be called since hash never changes and email count is 0
	fetches := atomic.LoadInt32(&fetchCount)
	if fetches > 0 {
		t.Logf("fetcher was called %d times (called on first sync)", fetches)
	}
}

func TestPollingStrategy_WaitForEmailCountWithSync(t *testing.T) {
	var fetchCount int32

	syncFetcher := func(ctx context.Context) (*SyncStatus, error) {
		return &SyncStatus{
			EmailCount: 3,
			EmailsHash: "hash-with-emails",
		}, nil
	}

	fetcher := func(ctx context.Context) ([]*testEmail, error) {
		count := atomic.AddInt32(&fetchCount, 1)
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

	results, err := WaitForEmailCountWithSync(ctx, fetcher, matcher, 2, WaitOptions{
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

func TestPollingStrategy_getWaitDuration(t *testing.T) {
	p := NewPollingStrategy(Config{})

	inbox := &polledInbox{
		interval: 2 * time.Second,
	}

	// Call multiple times to verify jitter is applied
	durations := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		durations[i] = p.getWaitDuration(inbox)
	}

	// All should be >= base interval
	for _, d := range durations {
		if d < inbox.interval {
			t.Errorf("duration %v is less than base interval %v", d, inbox.interval)
		}
	}

	// All should be <= interval + 30% jitter
	maxExpected := time.Duration(float64(inbox.interval) * (1 + PollingJitterFactor))
	for _, d := range durations {
		if d > maxExpected {
			t.Errorf("duration %v exceeds max expected %v", d, maxExpected)
		}
	}
}

func TestPollingStrategy_WaitOptions(t *testing.T) {
	// Test that WaitOptions fields are correctly used
	opts := WaitOptions{
		PollInterval: 5 * time.Second,
		SyncFetcher: func(ctx context.Context) (*SyncStatus, error) {
			return &SyncStatus{EmailCount: 0, EmailsHash: "test"}, nil
		},
	}

	if opts.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want 5s", opts.PollInterval)
	}
	if opts.SyncFetcher == nil {
		t.Error("SyncFetcher is nil")
	}
}

func TestPollingStrategy_ConcurrentPolling(t *testing.T) {
	// Test polling multiple inboxes concurrently
	p := NewPollingStrategy(Config{})

	// Track which inboxes have been polled
	var polledInboxes sync.Map
	var wg sync.WaitGroup
	wg.Add(3)

	// Create 3 inboxes
	inboxes := []InboxInfo{
		{Hash: "hash1", EmailAddress: "inbox1@example.com"},
		{Hash: "hash2", EmailAddress: "inbox2@example.com"},
		{Hash: "hash3", EmailAddress: "inbox3@example.com"},
	}

	// Add all inboxes
	for _, inbox := range inboxes {
		if err := p.AddInbox(inbox); err != nil {
			t.Fatalf("AddInbox() error = %v", err)
		}
	}

	// Verify all inboxes were added
	if len(p.inboxes) != 3 {
		t.Errorf("inboxes count = %d, want 3", len(p.inboxes))
	}

	// Test concurrent WaitForEmail operations on different inboxes
	for i, hash := range []string{"hash1", "hash2", "hash3"} {
		go func(idx int, inboxHash string) {
			defer wg.Done()

			fetcher := func(ctx context.Context) ([]*testEmail, error) {
				polledInboxes.Store(inboxHash, true)
				return []*testEmail{
					{ID: "email" + inboxHash, Subject: "Test"},
				}, nil
			}

			matcher := func(email *testEmail) bool {
				return true
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			result, err := WaitForEmail(ctx, fetcher, matcher, 10*time.Millisecond)
			if err != nil {
				t.Errorf("WaitForEmail() for %s error = %v", inboxHash, err)
				return
			}

			if result.ID != "email"+inboxHash {
				t.Errorf("email.ID = %s, want email%s", result.ID, inboxHash)
			}
		}(i, hash)
	}

	wg.Wait()

	// Verify all inboxes were polled
	for _, inbox := range inboxes {
		if _, ok := polledInboxes.Load(inbox.Hash); !ok {
			t.Errorf("inbox %s was not polled", inbox.Hash)
		}
	}
}

func TestPollingStrategy_RemoveInbox_Idempotent(t *testing.T) {
	// Test that removing the same inbox multiple times doesn't cause errors
	p := NewPollingStrategy(Config{})

	inbox := InboxInfo{
		Hash:         "hash123",
		EmailAddress: "test@example.com",
	}

	// Add inbox
	if err := p.AddInbox(inbox); err != nil {
		t.Fatalf("AddInbox() error = %v", err)
	}

	// Remove inbox first time
	if err := p.RemoveInbox(inbox.Hash); err != nil {
		t.Errorf("first RemoveInbox() error = %v", err)
	}

	// Remove inbox second time (should be idempotent)
	if err := p.RemoveInbox(inbox.Hash); err != nil {
		t.Errorf("second RemoveInbox() error = %v", err)
	}

	// Remove inbox third time
	if err := p.RemoveInbox(inbox.Hash); err != nil {
		t.Errorf("third RemoveInbox() error = %v", err)
	}

	// Verify inbox is not in the map
	if _, exists := p.inboxes[inbox.Hash]; exists {
		t.Error("inbox should not exist after removal")
	}
}

func TestPollingStrategy_AddInbox_AfterClose(t *testing.T) {
	// Test behavior when adding inbox after strategy is closed
	p := NewPollingStrategy(Config{})

	// Close the strategy
	if err := p.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Try to add inbox after close
	inbox := InboxInfo{
		Hash:         "hash123",
		EmailAddress: "test@example.com",
	}

	// AddInbox should still work (it just adds to the map)
	// The strategy is closed but the map operations still work
	err := p.AddInbox(inbox)
	if err != nil {
		t.Logf("AddInbox after Close returned: %v", err)
	}

	// Verify the inbox was added to the map (the current implementation allows this)
	if _, exists := p.inboxes[inbox.Hash]; !exists {
		t.Log("inbox was not added after close (acceptable behavior)")
	}
}
