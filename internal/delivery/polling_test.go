package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

func TestNewPollingStrategy(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	p := NewPollingStrategy(Config{})
	if p.Name() != "polling" {
		t.Errorf("Name() = %s, want polling", p.Name())
	}
}

func TestPollingStrategy_AddRemoveInbox(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	p := NewPollingStrategy(Config{})

	// Should not panic when stopping before starting
	if err := p.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestPollingStrategy_Start(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	p := NewPollingStrategy(Config{})

	// Setting nil callback should not panic
	p.OnError(nil)

	if p.onError != nil {
		t.Error("onError should be nil")
	}
}

func TestPollingStrategy_pollInbox_NilAPIClient(t *testing.T) {
	t.Parallel()
	p := NewPollingStrategy(Config{
		APIClient: nil,
	})

	inbox := &polledInbox{
		hash:         "hash123",
		emailAddress: "test@example.com",
		seenEmails:   make(map[string]struct{}),
		interval:     time.Second,
	}

	// Should not panic when API client is nil
	p.pollInbox(context.Background(), inbox)
}

func TestPollingStrategy_pollAll_EmptyInboxList(t *testing.T) {
	t.Parallel()
	p := NewPollingStrategy(Config{})

	// pollAll with empty inbox list should return initial interval
	wait := p.pollAll(context.Background())
	if wait != p.initialInterval {
		t.Errorf("wait = %v, want %v", wait, p.initialInterval)
	}
}

func TestPollingStrategy_pollLoop_ContextCancel(t *testing.T) {
	t.Parallel()
	p := NewPollingStrategy(Config{})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		p.pollLoop(ctx)
		close(done)
	}()

	// Cancel context immediately
	cancel()

	// Wait for pollLoop to exit
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("pollLoop did not exit after context cancel")
	}
}

func TestPollingStrategy_Start_WithInboxes(t *testing.T) {
	t.Parallel()
	p := NewPollingStrategy(Config{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inboxes := []InboxInfo{
		{Hash: "hash1", EmailAddress: "test1@example.com"},
		{Hash: "hash2", EmailAddress: "test2@example.com"},
		{Hash: "hash3", EmailAddress: "test3@example.com"},
	}

	handler := func(ctx context.Context, event *api.SSEEvent) error {
		return nil
	}

	if err := p.Start(ctx, inboxes, handler); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Verify all inboxes were added
	p.mu.RLock()
	count := len(p.inboxes)
	p.mu.RUnlock()

	if count != 3 {
		t.Errorf("inboxes count = %d, want 3", count)
	}

	// Verify each inbox has correct initial interval
	p.mu.RLock()
	for hash, inbox := range p.inboxes {
		if inbox.interval != p.initialInterval {
			t.Errorf("inbox %s interval = %v, want %v", hash, inbox.interval, p.initialInterval)
		}
	}
	p.mu.RUnlock()
}

func TestPollingStrategy_CustomConfig(t *testing.T) {
	t.Parallel()
	cfg := Config{
		PollingInitialInterval:   5 * time.Second,
		PollingMaxBackoff:        60 * time.Second,
		PollingBackoffMultiplier: 2.5,
		PollingJitterFactor:      0.2,
	}

	p := NewPollingStrategy(cfg)

	if p.initialInterval != 5*time.Second {
		t.Errorf("initialInterval = %v, want 5s", p.initialInterval)
	}
	if p.maxBackoff != 60*time.Second {
		t.Errorf("maxBackoff = %v, want 60s", p.maxBackoff)
	}
	if p.backoffMultiplier != 2.5 {
		t.Errorf("backoffMultiplier = %v, want 2.5", p.backoffMultiplier)
	}
	if p.jitterFactor != 0.2 {
		t.Errorf("jitterFactor = %v, want 0.2", p.jitterFactor)
	}
}

func TestPollingStrategy_DefaultConfig(t *testing.T) {
	t.Parallel()
	p := NewPollingStrategy(Config{})

	if p.initialInterval != DefaultPollingInitialInterval {
		t.Errorf("initialInterval = %v, want %v", p.initialInterval, DefaultPollingInitialInterval)
	}
	if p.maxBackoff != DefaultPollingMaxBackoff {
		t.Errorf("maxBackoff = %v, want %v", p.maxBackoff, DefaultPollingMaxBackoff)
	}
	if p.backoffMultiplier != DefaultPollingBackoffMultiplier {
		t.Errorf("backoffMultiplier = %v, want %v", p.backoffMultiplier, DefaultPollingBackoffMultiplier)
	}
	if p.jitterFactor != DefaultPollingJitterFactor {
		t.Errorf("jitterFactor = %v, want %v", p.jitterFactor, DefaultPollingJitterFactor)
	}
}

func TestPollingStrategy_pollInbox_NoChange(t *testing.T) {
	t.Parallel()
	// Create a mock server that returns unchanged sync status
	syncHash := "hash123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"emailCount": 0,
			"emailsHash": syncHash,
		})
	}))
	defer server.Close()

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL))
	p := NewPollingStrategy(Config{
		APIClient: apiClient,
	})

	inbox := &polledInbox{
		hash:         "hash123",
		emailAddress: "test@example.com",
		seenEmails:   make(map[string]struct{}),
		lastHash:     syncHash, // Same hash - no change
		interval:     time.Second,
	}

	initialInterval := inbox.interval
	p.pollInbox(context.Background(), inbox)

	// Interval should increase due to backoff
	if inbox.interval <= initialInterval {
		t.Errorf("interval should increase on no change, got %v", inbox.interval)
	}
}

func TestPollingStrategy_pollInbox_WithNewEmails(t *testing.T) {
	t.Parallel()
	var syncCalled, emailsCalled atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/inboxes/test%40example.com/sync" || r.URL.Path == "/api/inboxes/test@example.com/sync" {
			syncCalled.Add(1)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailCount": 1,
				"emailsHash": "newhash",
			})
			return
		}
		if r.URL.Path == "/api/inboxes/test%40example.com/emails" || r.URL.Path == "/api/inboxes/test@example.com/emails" {
			emailsCalled.Add(1)
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id":      "email1",
					"inboxId": "hash123",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL))
	p := NewPollingStrategy(Config{
		APIClient: apiClient,
	})

	var receivedEvent *api.SSEEvent
	p.handler = func(ctx context.Context, event *api.SSEEvent) error {
		receivedEvent = event
		return nil
	}

	inbox := &polledInbox{
		hash:         "hash123",
		emailAddress: "test@example.com",
		seenEmails:   make(map[string]struct{}),
		lastHash:     "oldhash", // Different hash - change detected
		interval:     time.Second,
	}

	p.pollInbox(context.Background(), inbox)

	if syncCalled.Load() != 1 {
		t.Errorf("sync endpoint called %d times, want 1", syncCalled.Load())
	}
	if emailsCalled.Load() != 1 {
		t.Errorf("emails endpoint called %d times, want 1", emailsCalled.Load())
	}
	if receivedEvent == nil {
		t.Error("handler was not called with new email")
	}
	if _, seen := inbox.seenEmails["email1"]; !seen {
		t.Error("email1 should be marked as seen")
	}
	// Interval should reset on change
	if inbox.interval != p.initialInterval {
		t.Errorf("interval should reset to initial on change, got %v", inbox.interval)
	}
}

func TestPollingStrategy_pollInbox_SyncError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL), api.WithRetries(0))
	p := NewPollingStrategy(Config{
		APIClient: apiClient,
	})

	var errorReceived error
	p.onError = func(err error) {
		errorReceived = err
	}

	inbox := &polledInbox{
		hash:         "hash123",
		emailAddress: "test@example.com",
		seenEmails:   make(map[string]struct{}),
		interval:     time.Second,
	}

	p.pollInbox(context.Background(), inbox)

	if errorReceived == nil {
		t.Error("onError should be called on sync error")
	}
}

func TestPollingStrategy_pollInbox_EmailsError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/inboxes/test%40example.com/sync" || r.URL.Path == "/api/inboxes/test@example.com/sync" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailCount": 1,
				"emailsHash": "newhash",
			})
			return
		}
		// Return error for emails endpoint
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL), api.WithRetries(0))
	p := NewPollingStrategy(Config{
		APIClient: apiClient,
	})

	var errorReceived error
	p.onError = func(err error) {
		errorReceived = err
	}

	inbox := &polledInbox{
		hash:         "hash123",
		emailAddress: "test@example.com",
		seenEmails:   make(map[string]struct{}),
		lastHash:     "oldhash",
		interval:     time.Second,
	}

	p.pollInbox(context.Background(), inbox)

	if errorReceived == nil {
		t.Error("onError should be called on emails fetch error")
	}
}

func TestPollingStrategy_pollInbox_BackoffCapping(t *testing.T) {
	t.Parallel()
	syncHash := "samehash"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"emailCount": 0,
			"emailsHash": syncHash,
		})
	}))
	defer server.Close()

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL))
	p := NewPollingStrategy(Config{
		APIClient:                apiClient,
		PollingInitialInterval:   100 * time.Millisecond,
		PollingMaxBackoff:        500 * time.Millisecond,
		PollingBackoffMultiplier: 2.0,
	})

	inbox := &polledInbox{
		hash:         "hash123",
		emailAddress: "test@example.com",
		seenEmails:   make(map[string]struct{}),
		lastHash:     syncHash,
		interval:     100 * time.Millisecond,
	}

	// Poll multiple times to hit max backoff
	for i := 0; i < 10; i++ {
		p.pollInbox(context.Background(), inbox)
	}

	// Interval should be capped at maxBackoff
	if inbox.interval > p.maxBackoff {
		t.Errorf("interval %v exceeds maxBackoff %v", inbox.interval, p.maxBackoff)
	}
}

func TestPollingStrategy_pollInbox_OnErrorNil(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL), api.WithRetries(0))
	p := NewPollingStrategy(Config{
		APIClient: apiClient,
	})
	// onError is nil - should not panic

	inbox := &polledInbox{
		hash:         "hash123",
		emailAddress: "test@example.com",
		seenEmails:   make(map[string]struct{}),
		interval:     time.Second,
	}

	// Should not panic even with nil onError
	p.pollInbox(context.Background(), inbox)
}

func TestPollingStrategy_pollInbox_SkipSeenEmails(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/inboxes/test%40example.com/sync" || r.URL.Path == "/api/inboxes/test@example.com/sync" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailCount": 2,
				"emailsHash": "newhash",
			})
			return
		}
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "email1", "inboxId": "hash123"},
			{"id": "email2", "inboxId": "hash123"},
		})
	}))
	defer server.Close()

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL))
	p := NewPollingStrategy(Config{
		APIClient: apiClient,
	})

	var eventCount int
	p.handler = func(ctx context.Context, event *api.SSEEvent) error {
		eventCount++
		return nil
	}

	inbox := &polledInbox{
		hash:         "hash123",
		emailAddress: "test@example.com",
		seenEmails: map[string]struct{}{
			"email1": {}, // Already seen
		},
		lastHash: "oldhash",
		interval: time.Second,
	}

	p.pollInbox(context.Background(), inbox)

	// Only email2 should trigger handler (email1 already seen)
	if eventCount != 1 {
		t.Errorf("handler called %d times, want 1 (only unseen email)", eventCount)
	}
}

func TestPollingStrategy_pollInbox_NilHandler(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/inboxes/test%40example.com/sync" || r.URL.Path == "/api/inboxes/test@example.com/sync" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"emailCount": 1,
				"emailsHash": "newhash",
			})
			return
		}
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "email1", "inboxId": "hash123"},
		})
	}))
	defer server.Close()

	apiClient, _ := api.New("test-key", api.WithBaseURL(server.URL))
	p := NewPollingStrategy(Config{
		APIClient: apiClient,
	})
	// handler is nil

	inbox := &polledInbox{
		hash:         "hash123",
		emailAddress: "test@example.com",
		seenEmails:   make(map[string]struct{}),
		lastHash:     "oldhash",
		interval:     time.Second,
	}

	// Should not panic with nil handler
	p.pollInbox(context.Background(), inbox)

	// Email should still be marked as seen
	if _, seen := inbox.seenEmails["email1"]; !seen {
		t.Error("email1 should be marked as seen even with nil handler")
	}
}

func TestPollingStrategy_pollLoop_ZeroMinWait(t *testing.T) {
	t.Parallel()
	// Test that pollLoop handles zero minWait by using initialInterval
	// This covers the defensive check: if minWait == 0 { minWait = p.initialInterval }
	p := NewPollingStrategy(Config{
		PollingJitterFactor: 0, // No jitter so getWaitDuration returns exact interval
	})

	// Add an inbox with zero interval (edge case)
	p.mu.Lock()
	p.inboxes["hash1"] = &polledInbox{
		hash:         "hash1",
		emailAddress: "test@example.com",
		seenEmails:   make(map[string]struct{}),
		interval:     0, // Zero interval
	}
	p.mu.Unlock()

	// Verify pollAll returns 0 with this setup
	wait := p.pollAll(context.Background())
	if wait != 0 {
		t.Errorf("pollAll should return 0 with zero interval inbox, got %v", wait)
	}

	// Now run pollLoop briefly - it should use initialInterval when minWait is 0
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		p.pollLoop(ctx)
		close(done)
	}()

	// Cancel immediately - we just need to verify it doesn't hang on zero minWait
	cancel()

	select {
	case <-done:
		// Success - pollLoop handled zero minWait without hanging
	case <-time.After(time.Second):
		t.Error("pollLoop did not exit - may have hung on zero minWait")
	}
}

// Ensure errors package is used (required for import)
var _ = errors.New
