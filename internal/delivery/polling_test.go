package delivery

import (
	"context"
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
	p := NewPollingStrategy(Config{})

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

	result, err := p.WaitForEmail(ctx, "hash123", fetcher, matcher, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForEmail() error = %v", err)
	}

	email := result.(*testEmail)
	if email.ID != "email1" {
		t.Errorf("email.ID = %s, want email1", email.ID)
	}
}

func TestPollingStrategy_WaitForEmail_ImmediateMatch(t *testing.T) {
	p := NewPollingStrategy(Config{})

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
	result, err := p.WaitForEmail(ctx, "hash123", fetcher, matcher, time.Second)
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
	p := NewPollingStrategy(Config{})

	fetcher := func(ctx context.Context) ([]interface{}, error) {
		return nil, nil // Never returns emails
	}

	matcher := func(email interface{}) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := p.WaitForEmail(ctx, "hash123", fetcher, matcher, 10*time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Errorf("WaitForEmail() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestPollingStrategy_WaitForEmailCount(t *testing.T) {
	p := NewPollingStrategy(Config{})

	var callCount int32

	fetcher := func(ctx context.Context) ([]interface{}, error) {
		count := atomic.AddInt32(&callCount, 1)
		if count >= 2 {
			return []interface{}{
				&testEmail{ID: "email1", Subject: "Hello"},
				&testEmail{ID: "email2", Subject: "Hello"},
				&testEmail{ID: "email3", Subject: "Hello"},
			}, nil
		}
		return nil, nil
	}

	matcher := func(email interface{}) bool {
		return true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := p.WaitForEmailCount(ctx, "hash123", fetcher, matcher, 2, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForEmailCount() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestPollingStrategy_WaitForEmailCount_ImmediateMatch(t *testing.T) {
	p := NewPollingStrategy(Config{})

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
	results, err := p.WaitForEmailCount(ctx, "hash123", fetcher, matcher, 2, time.Second)
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
	p := NewPollingStrategy(Config{})

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

	_, err := p.WaitForEmailCount(ctx, "hash123", fetcher, matcher, 2, 10*time.Millisecond)
	if err != context.DeadlineExceeded {
		t.Errorf("WaitForEmailCount() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestPollingStrategy_WaitForEmail_DefaultInterval(t *testing.T) {
	p := NewPollingStrategy(Config{})

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
	_, err := p.WaitForEmail(ctx, "hash123", fetcher, matcher, 0)
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
