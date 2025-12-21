package delivery

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

func TestNewAutoStrategy(t *testing.T) {
	cfg := Config{
		APIClient: nil,
	}

	a := NewAutoStrategy(cfg)
	if a == nil {
		t.Fatal("NewAutoStrategy returned nil")
	}
}

func TestAutoStrategy_Name(t *testing.T) {
	a := NewAutoStrategy(Config{})

	// Without a current strategy
	if a.Name() != "auto" {
		t.Errorf("Name() = %s, want auto", a.Name())
	}
}

func TestAutoStrategy_Stop_NotStarted(t *testing.T) {
	a := NewAutoStrategy(Config{})

	// Should not panic when stopping without a current strategy
	if err := a.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestAutoStrategy_Close(t *testing.T) {
	a := NewAutoStrategy(Config{})

	if err := a.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestAutoStrategy_AddInbox_NoCurrent(t *testing.T) {
	a := NewAutoStrategy(Config{})

	// Should not panic when no current strategy
	err := a.AddInbox(InboxInfo{Hash: "hash1"})
	if err != nil {
		t.Errorf("AddInbox() error = %v", err)
	}
}

func TestAutoStrategy_RemoveInbox_NoCurrent(t *testing.T) {
	a := NewAutoStrategy(Config{})

	// Should not panic when no current strategy
	err := a.RemoveInbox("hash1")
	if err != nil {
		t.Errorf("RemoveInbox() error = %v", err)
	}
}

func TestAutoStrategy_WaitForEmail(t *testing.T) {
	a := NewAutoStrategy(Config{})

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

	result, err := a.WaitForEmail(ctx, "hash123", fetcher, matcher, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	email := result.(*testEmail)
	if email.ID != "email1" {
		t.Errorf("email.ID = %s, want email1", email.ID)
	}
}

func TestAutoStrategy_WaitForEmail_ImmediateMatch(t *testing.T) {
	a := NewAutoStrategy(Config{})

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
	result, err := a.WaitForEmail(ctx, "hash123", fetcher, matcher, time.Second)
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

func TestAutoStrategy_WaitForEmail_Timeout(t *testing.T) {
	a := NewAutoStrategy(Config{})

	fetcher := func(ctx context.Context) ([]interface{}, error) {
		return nil, nil
	}

	matcher := func(email interface{}) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := a.WaitForEmail(ctx, "hash123", fetcher, matcher, 10*time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Errorf("WaitForEmail() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestAutoStrategy_WaitForEmailCount(t *testing.T) {
	a := NewAutoStrategy(Config{})

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

	results, err := a.WaitForEmailCount(ctx, "hash123", fetcher, matcher, 2, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForEmailCount() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestAutoStrategy_WaitForEmailCount_ImmediateMatch(t *testing.T) {
	a := NewAutoStrategy(Config{})

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
	results, err := a.WaitForEmailCount(ctx, "hash123", fetcher, matcher, 2, time.Second)
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

func TestAutoStrategy_WaitForEmailCount_Timeout(t *testing.T) {
	a := NewAutoStrategy(Config{})

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

	_, err := a.WaitForEmailCount(ctx, "hash123", fetcher, matcher, 2, 10*time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Errorf("WaitForEmailCount() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestAutoSSETimeout(t *testing.T) {
	if AutoSSETimeout != 5*time.Second {
		t.Errorf("AutoSSETimeout = %v, want 5s", AutoSSETimeout)
	}
}

func TestAutoStrategy_Start_FallbackToPolling(t *testing.T) {
	// Without a real API client, SSE will fail and we should fall back to polling
	a := NewAutoStrategy(Config{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(event *api.SSEEvent) error {
		return nil
	}

	inboxes := []InboxInfo{
		{Hash: "hash1", EmailAddress: "test1@example.com"},
	}

	// Start should succeed by falling back to polling
	err := a.Start(ctx, inboxes, handler)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Current strategy should be polling
	if a.current != nil {
		t.Logf("current strategy: %s", a.current.Name())
	}

	a.Stop()
}

func TestAutoStrategy_AddRemoveInbox_WithCurrent(t *testing.T) {
	a := NewAutoStrategy(Config{})

	// Manually set a current strategy (polling)
	a.current = NewPollingStrategy(Config{})

	inbox := InboxInfo{Hash: "hash1", EmailAddress: "test@example.com"}

	// Add should delegate to current
	err := a.AddInbox(inbox)
	if err != nil {
		t.Errorf("AddInbox() error = %v", err)
	}

	// Remove should delegate to current
	err = a.RemoveInbox("hash1")
	if err != nil {
		t.Errorf("RemoveInbox() error = %v", err)
	}
}

func TestAutoStrategy_WaitForEmailWithSync(t *testing.T) {
	a := NewAutoStrategy(Config{})

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

	result, err := a.WaitForEmailWithSync(ctx, "hash123", fetcher, matcher, WaitOptions{
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

func TestAutoStrategy_WaitForEmailCountWithSync(t *testing.T) {
	a := NewAutoStrategy(Config{})

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

	results, err := a.WaitForEmailCountWithSync(ctx, "hash123", fetcher, matcher, 2, WaitOptions{
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
