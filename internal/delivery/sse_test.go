package delivery

import (
	"context"
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
	s := NewSSEStrategy(Config{})

	var callCount int32

	fetcher := func(ctx context.Context) ([]interface{}, error) {
		count := atomic.AddInt32(&callCount, 1)
		if count >= 3 {
			return []interface{}{
				&testEmail{ID: "email1", Subject: "Hello"},
			}, nil
		}
		return nil, nil
	}

	matcher := func(email interface{}) bool {
		e := email.(*testEmail)
		return e.Subject == "Hello"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := s.WaitForEmail(ctx, "hash123", fetcher, matcher, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	email := result.(*testEmail)
	if email.ID != "email1" {
		t.Errorf("email.ID = %s, want email1", email.ID)
	}
}

func TestSSEStrategy_WaitForEmail_ImmediateMatch(t *testing.T) {
	s := NewSSEStrategy(Config{})

	fetcher := func(ctx context.Context) ([]interface{}, error) {
		return []interface{}{
			&testEmail{ID: "email1", Subject: "Hello"},
		}, nil
	}

	matcher := func(email interface{}) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	result, err := s.WaitForEmail(ctx, "hash123", fetcher, matcher, time.Second)
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
	s := NewSSEStrategy(Config{})

	fetcher := func(ctx context.Context) ([]interface{}, error) {
		return nil, nil
	}

	matcher := func(email interface{}) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := s.WaitForEmail(ctx, "hash123", fetcher, matcher, 10*time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Errorf("WaitForEmail() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestSSEStrategy_WaitForEmailCount(t *testing.T) {
	s := NewSSEStrategy(Config{})

	var callCount int32

	fetcher := func(ctx context.Context) ([]interface{}, error) {
		count := atomic.AddInt32(&callCount, 1)
		if count >= 2 {
			return []interface{}{
				&testEmail{ID: "email1"},
				&testEmail{ID: "email2"},
				&testEmail{ID: "email3"},
			}, nil
		}
		return nil, nil
	}

	matcher := func(email interface{}) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := s.WaitForEmailCount(ctx, "hash123", fetcher, matcher, 2, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForEmailCount() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestSSEStrategy_WaitForEmailCount_ImmediateMatch(t *testing.T) {
	s := NewSSEStrategy(Config{})

	fetcher := func(ctx context.Context) ([]interface{}, error) {
		return []interface{}{
			&testEmail{ID: "email1"},
			&testEmail{ID: "email2"},
		}, nil
	}

	matcher := func(email interface{}) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	results, err := s.WaitForEmailCount(ctx, "hash123", fetcher, matcher, 2, time.Second)
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
	s := NewSSEStrategy(Config{})

	fetcher := func(ctx context.Context) ([]interface{}, error) {
		return []interface{}{
			&testEmail{ID: "email1"},
		}, nil // Only 1 email, but we want 2
	}

	matcher := func(email interface{}) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := s.WaitForEmailCount(ctx, "hash123", fetcher, matcher, 2, 10*time.Millisecond)
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

	handler := func(event *api.SSEEvent) error {
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
	s := NewSSEStrategy(Config{})

	fetchCount := 0
	fetcher := func(ctx context.Context) ([]interface{}, error) {
		fetchCount++
		if fetchCount >= 2 {
			return []interface{}{&testEmail{ID: "email1"}}, nil
		}
		return nil, nil
	}

	matcher := func(email interface{}) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Pass 0 for pollInterval to use default
	_, err := s.WaitForEmail(ctx, "hash123", fetcher, matcher, 0)
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
	s := NewSSEStrategy(Config{})

	var fetchCount int32

	syncFetcher := func(ctx context.Context) (*SyncStatus, error) {
		return &SyncStatus{
			EmailCount: 1,
			EmailsHash: "test-hash",
		}, nil
	}

	fetcher := func(ctx context.Context) ([]interface{}, error) {
		count := atomic.AddInt32(&fetchCount, 1)
		if count >= 2 {
			return []interface{}{
				&testEmail{ID: "email1", Subject: "Hello"},
			}, nil
		}
		return nil, nil
	}

	matcher := func(email interface{}) bool {
		return email.(*testEmail).Subject == "Hello"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := s.WaitForEmailWithSync(ctx, "hash123", fetcher, matcher, WaitOptions{
		PollInterval: 10 * time.Millisecond,
		SyncFetcher:  syncFetcher,
	})
	if err != nil {
		t.Fatalf("WaitForEmailWithSync() error = %v", err)
	}

	email := result.(*testEmail)
	if email.ID != "email1" {
		t.Errorf("email.ID = %s, want email1", email.ID)
	}
}

func TestSSEStrategy_WaitForEmailCountWithSync(t *testing.T) {
	s := NewSSEStrategy(Config{})

	syncFetcher := func(ctx context.Context) (*SyncStatus, error) {
		return &SyncStatus{
			EmailCount: 3,
			EmailsHash: "test-hash",
		}, nil
	}

	fetcher := func(ctx context.Context) ([]interface{}, error) {
		return []interface{}{
			&testEmail{ID: "email1"},
			&testEmail{ID: "email2"},
			&testEmail{ID: "email3"},
		}, nil
	}

	matcher := func(email interface{}) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := s.WaitForEmailCountWithSync(ctx, "hash123", fetcher, matcher, 2, WaitOptions{
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
