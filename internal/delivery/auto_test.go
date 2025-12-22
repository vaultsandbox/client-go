package delivery

import (
	"context"
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
