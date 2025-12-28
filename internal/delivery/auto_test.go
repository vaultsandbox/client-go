package delivery

import (
	"context"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

func TestNewAutoStrategy(t *testing.T) {
	cfg := Config{
		APIClient:            nil,
		SSEConnectionTimeout: 10 * time.Second,
	}

	s := NewAutoStrategy(cfg)
	if s == nil {
		t.Fatal("NewAutoStrategy returned nil")
	}
	if s.timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", s.timeout)
	}
}

func TestNewAutoStrategy_DefaultTimeout(t *testing.T) {
	cfg := Config{
		APIClient: nil,
		// SSEConnectionTimeout not set, should use default
	}

	s := NewAutoStrategy(cfg)
	if s.timeout != DefaultSSEConnectionTimeout {
		t.Errorf("timeout = %v, want %v", s.timeout, DefaultSSEConnectionTimeout)
	}
}

func TestAutoStrategy_Name(t *testing.T) {
	s := NewAutoStrategy(Config{})

	// Before switching, should report SSE
	if s.Name() != "auto:sse" {
		t.Errorf("Name() = %s, want auto:sse", s.Name())
	}

	// After switching, should report polling
	s.mu.Lock()
	s.switched = true
	s.mu.Unlock()

	if s.Name() != "auto:polling" {
		t.Errorf("Name() = %s, want auto:polling", s.Name())
	}
}

func TestAutoStrategy_AddRemoveInbox(t *testing.T) {
	s := NewAutoStrategy(Config{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(ctx context.Context, event *api.SSEEvent) error {
		return nil
	}

	if err := s.Start(ctx, nil, handler); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	inbox := InboxInfo{
		Hash:         "hash123",
		EmailAddress: "test@example.com",
	}

	// Add inbox
	if err := s.AddInbox(inbox); err != nil {
		t.Fatalf("AddInbox() error = %v", err)
	}

	// Verify tracked in SSE strategy
	s.sse.mu.RLock()
	_, found := s.sse.inboxHashes[inbox.Hash]
	s.sse.mu.RUnlock()

	if !found {
		t.Error("inbox was not added to SSE strategy")
	}

	// Remove inbox
	if err := s.RemoveInbox(inbox.Hash); err != nil {
		t.Fatalf("RemoveInbox() error = %v", err)
	}

	// Verify removed from SSE strategy
	s.sse.mu.RLock()
	_, found = s.sse.inboxHashes[inbox.Hash]
	s.sse.mu.RUnlock()

	if found {
		t.Error("inbox was not removed from SSE strategy")
	}
}

func TestAutoStrategy_Stop_NotStarted(t *testing.T) {
	s := NewAutoStrategy(Config{})

	// Should not panic when stopping before starting
	if err := s.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestAutoStrategy_Start(t *testing.T) {
	s := NewAutoStrategy(Config{})

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

	// Verify inboxes were tracked in SSE strategy
	s.sse.mu.RLock()
	if len(s.sse.inboxHashes) != 2 {
		t.Errorf("inboxes count = %d, want 2", len(s.sse.inboxHashes))
	}
	s.sse.mu.RUnlock()

	// Stop the strategy
	cancel()
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestAutoStrategy_Connected(t *testing.T) {
	s := NewAutoStrategy(Config{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(ctx context.Context, event *api.SSEEvent) error {
		return nil
	}

	if err := s.Start(ctx, nil, handler); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Channel should not be closed initially
	select {
	case <-s.Connected():
		t.Error("connected channel should not be closed initially")
	default:
		// Expected
	}
}

func TestAutoStrategy_Switched(t *testing.T) {
	s := NewAutoStrategy(Config{})

	// Initially should not be switched
	if s.Switched() {
		t.Error("Switched() should be false initially")
	}

	// After switching, should return true
	s.mu.Lock()
	s.switched = true
	s.mu.Unlock()

	if !s.Switched() {
		t.Error("Switched() should be true after switching")
	}
}

func TestAutoStrategy_OnReconnect(t *testing.T) {
	s := NewAutoStrategy(Config{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(ctx context.Context, event *api.SSEEvent) error {
		return nil
	}

	if err := s.Start(ctx, nil, handler); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	s.OnReconnect(func(ctx context.Context) {
		// Callback for testing
	})

	// Verify callback was registered
	s.mu.RLock()
	if s.onReconnect == nil {
		t.Error("onReconnect callback was not set")
	}
	s.mu.RUnlock()
}

func TestAutoStrategy_FallbackToPolling(t *testing.T) {
	// Use a very short timeout to trigger fallback quickly
	cfg := Config{
		SSEConnectionTimeout: 50 * time.Millisecond,
	}
	s := NewAutoStrategy(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(ctx context.Context, event *api.SSEEvent) error {
		return nil
	}

	inboxes := []InboxInfo{
		{Hash: "hash1", EmailAddress: "test1@example.com"},
	}

	if err := s.Start(ctx, inboxes, handler); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Add an inbox to trigger connection attempt
	inbox := InboxInfo{Hash: "hash2", EmailAddress: "test2@example.com"}
	if err := s.AddInbox(inbox); err != nil {
		t.Fatalf("AddInbox() error = %v", err)
	}

	// Wait for fallback to occur (timeout + some buffer)
	time.Sleep(150 * time.Millisecond)

	// Verify switched to polling
	if !s.Switched() {
		t.Error("Expected strategy to switch to polling after SSE timeout")
	}

	if s.Name() != "auto:polling" {
		t.Errorf("Name() = %s, want auto:polling", s.Name())
	}
}

func TestAutoStrategy_RemoveInbox_Idempotent(t *testing.T) {
	s := NewAutoStrategy(Config{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(ctx context.Context, event *api.SSEEvent) error {
		return nil
	}

	if err := s.Start(ctx, nil, handler); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	inbox := InboxInfo{
		Hash:         "hash123",
		EmailAddress: "test@example.com",
	}

	// Add inbox
	if err := s.AddInbox(inbox); err != nil {
		t.Fatalf("AddInbox() error = %v", err)
	}

	// Remove inbox first time
	if err := s.RemoveInbox(inbox.Hash); err != nil {
		t.Errorf("first RemoveInbox() error = %v", err)
	}

	// Remove inbox second time (should be idempotent)
	if err := s.RemoveInbox(inbox.Hash); err != nil {
		t.Errorf("second RemoveInbox() error = %v", err)
	}

	// Verify inbox is not in SSE strategy
	s.sse.mu.RLock()
	_, exists := s.sse.inboxHashes[inbox.Hash]
	s.sse.mu.RUnlock()

	if exists {
		t.Error("inbox should not exist after removal")
	}
}

func TestAutoStrategy_ActiveStrategyDelegation(t *testing.T) {
	s := NewAutoStrategy(Config{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler := func(ctx context.Context, event *api.SSEEvent) error {
		return nil
	}

	inboxes := []InboxInfo{
		{Hash: "hash1", EmailAddress: "test1@example.com"},
	}

	if err := s.Start(ctx, inboxes, handler); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Verify active strategy is SSE initially
	s.mu.RLock()
	if s.active != s.sse {
		t.Error("active strategy should be SSE initially")
	}
	s.mu.RUnlock()

	// Add inbox should delegate to active strategy
	inbox := InboxInfo{Hash: "hash2", EmailAddress: "test2@example.com"}
	if err := s.AddInbox(inbox); err != nil {
		t.Fatalf("AddInbox() error = %v", err)
	}

	// Verify inbox was added to SSE strategy
	s.sse.mu.RLock()
	_, exists := s.sse.inboxHashes[inbox.Hash]
	s.sse.mu.RUnlock()

	if !exists {
		t.Error("inbox should have been delegated to SSE strategy")
	}
}
