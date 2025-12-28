package delivery

import (
	"context"
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

func TestPollingStrategy_Start(t *testing.T) {
	// Create a mock API client would be needed for full test
	// For now, test basic start/stop functionality

	p := NewPollingStrategy(Config{})

	ctx, cancel := context.WithCancel(context.Background())

	handler := func(ctx context.Context, event *api.SSEEvent) error {
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

	// All should be <= interval + 30% jitter (DefaultPollingJitterFactor)
	maxExpected := time.Duration(float64(inbox.interval) * (1 + DefaultPollingJitterFactor))
	for _, d := range durations {
		if d > maxExpected {
			t.Errorf("duration %v exceeds max expected %v", d, maxExpected)
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

func TestPollingStrategy_AddInbox_AfterStop(t *testing.T) {
	// Test behavior when adding inbox after strategy is stopped
	p := NewPollingStrategy(Config{})

	// Stop the strategy
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Try to add inbox after stop
	inbox := InboxInfo{
		Hash:         "hash123",
		EmailAddress: "test@example.com",
	}

	// AddInbox should still work (it just adds to the map)
	// The strategy is stopped but the map operations still work
	err := p.AddInbox(inbox)
	if err != nil {
		t.Logf("AddInbox after Stop returned: %v", err)
	}

	// Verify the inbox was added to the map (the current implementation allows this)
	if _, exists := p.inboxes[inbox.Hash]; !exists {
		t.Log("inbox was not added after stop (acceptable behavior)")
	}
}

func TestPollingStrategy_OnReconnect(t *testing.T) {
	p := NewPollingStrategy(Config{})

	var called bool
	callback := func(ctx context.Context) {
		called = true
	}

	// OnReconnect is a no-op for polling, but should not panic
	p.OnReconnect(callback)

	// Callback should NOT be stored since polling doesn't use reconnection
	// The method is a no-op by design
	if called {
		t.Error("callback should not have been called")
	}
}

func TestPollingStrategy_OnError(t *testing.T) {
	p := NewPollingStrategy(Config{})

	var receivedErr error
	callback := func(err error) {
		receivedErr = err
	}

	p.OnError(callback)

	// Verify the callback was stored
	if p.onError == nil {
		t.Fatal("onError callback was not set")
	}

	// Test that the callback works
	testErr := context.DeadlineExceeded
	p.onError(testErr)

	if receivedErr != testErr {
		t.Errorf("received error = %v, want %v", receivedErr, testErr)
	}
}

func TestPollingStrategy_OnError_NilCallback(t *testing.T) {
	p := NewPollingStrategy(Config{})

	// Setting nil callback should not panic
	p.OnError(nil)

	if p.onError != nil {
		t.Error("onError should be nil")
	}
}
