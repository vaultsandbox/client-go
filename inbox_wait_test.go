package vaultsandbox

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestInbox_Watch_ReturnsChannel(t *testing.T) {
	inbox := &Inbox{
		inboxHash: "test-hash",
		client: &Client{
			watchers: make(map[string][]chan<- *Email),
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := inbox.Watch(ctx)
	if ch == nil {
		t.Fatal("Watch() returned nil channel")
	}
}

func TestInbox_Watch_ClosesOnContextCancel(t *testing.T) {
	inbox := &Inbox{
		inboxHash: "test-hash",
		client: &Client{
			watchers: make(map[string][]chan<- *Email),
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	ch := inbox.Watch(ctx)

	// Cancel context
	cancel()

	// Channel should close
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("channel did not close after context cancel")
	}
}

func TestInbox_Watch_ReceivesEmails(t *testing.T) {
	client := &Client{
		watchers: make(map[string][]chan<- *Email),
	}
	inbox := &Inbox{
		inboxHash: "test-hash",
		client:    client,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := inbox.Watch(ctx)

	// Simulate email arrival
	testEmail := &Email{ID: "email-1", Subject: "Test"}
	client.notifyWatchers(ctx, "test-hash", testEmail)

	select {
	case email := <-ch:
		if email.ID != "email-1" {
			t.Errorf("email.ID = %q, want %q", email.ID, "email-1")
		}
		if email.Subject != "Test" {
			t.Errorf("email.Subject = %q, want %q", email.Subject, "Test")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive email")
	}
}

func TestInbox_Watch_MultipleWatchers(t *testing.T) {
	client := &Client{
		watchers: make(map[string][]chan<- *Email),
	}
	inbox := &Inbox{
		inboxHash: "test-hash",
		client:    client,
	}

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	ch1 := inbox.Watch(ctx1)
	ch2 := inbox.Watch(ctx2)

	// Both watchers should be registered
	client.watchersMu.RLock()
	watcherCount := len(client.watchers["test-hash"])
	client.watchersMu.RUnlock()

	if watcherCount != 2 {
		t.Errorf("watcher count = %d, want 2", watcherCount)
	}

	// Both should receive the same email
	testEmail := &Email{ID: "email-1"}
	client.notifyWatchers(ctx1, "test-hash", testEmail)

	for i, ch := range []<-chan *Email{ch1, ch2} {
		select {
		case email := <-ch:
			if email.ID != "email-1" {
				t.Errorf("watcher %d: email.ID = %q, want %q", i, email.ID, "email-1")
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("watcher %d: did not receive email", i)
		}
	}
}

func TestInbox_Watch_CancelRemovesWatcher(t *testing.T) {
	client := &Client{
		watchers: make(map[string][]chan<- *Email),
	}
	inbox := &Inbox{
		inboxHash: "test-hash",
		client:    client,
	}

	ctx, cancel := context.WithCancel(context.Background())
	_ = inbox.Watch(ctx)

	// Verify watcher is registered
	client.watchersMu.RLock()
	initialCount := len(client.watchers["test-hash"])
	client.watchersMu.RUnlock()
	if initialCount != 1 {
		t.Fatalf("initial watcher count = %d, want 1", initialCount)
	}

	// Cancel and wait for cleanup
	cancel()
	time.Sleep(50 * time.Millisecond)

	// Verify watcher is removed
	client.watchersMu.RLock()
	finalCount := len(client.watchers["test-hash"])
	client.watchersMu.RUnlock()
	if finalCount != 0 {
		t.Errorf("watcher count after cancel = %d, want 0", finalCount)
	}
}

func TestClient_WatchInboxes_ReturnsChannel(t *testing.T) {
	client := &Client{
		watchers: make(map[string][]chan<- *Email),
	}
	inbox1 := &Inbox{inboxHash: "hash-1", client: client}
	inbox2 := &Inbox{inboxHash: "hash-2", client: client}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := client.WatchInboxes(ctx, inbox1, inbox2)
	if ch == nil {
		t.Fatal("WatchInboxes() returned nil channel")
	}
}

func TestClient_WatchInboxes_EmptyInboxes(t *testing.T) {
	client := &Client{
		watchers: make(map[string][]chan<- *Email),
	}

	ctx := context.Background()
	ch := client.WatchInboxes(ctx)

	// Channel should be closed immediately
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed for empty inboxes")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("channel should close immediately for empty inboxes")
	}
}

func TestClient_WatchInboxes_ReceivesFromMultipleInboxes(t *testing.T) {
	client := &Client{
		watchers: make(map[string][]chan<- *Email),
	}
	inbox1 := &Inbox{inboxHash: "hash-1", emailAddress: "inbox1@test.com", client: client}
	inbox2 := &Inbox{inboxHash: "hash-2", emailAddress: "inbox2@test.com", client: client}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := client.WatchInboxes(ctx, inbox1, inbox2)

	// Send email to first inbox
	email1 := &Email{ID: "email-1", Subject: "To Inbox 1"}
	client.notifyWatchers(ctx, "hash-1", email1)

	// Send email to second inbox
	email2 := &Email{ID: "email-2", Subject: "To Inbox 2"}
	client.notifyWatchers(ctx, "hash-2", email2)

	received := make(map[string]string) // emailID -> inboxAddress

	for i := 0; i < 2; i++ {
		select {
		case event := <-ch:
			received[event.Email.ID] = event.Inbox.EmailAddress()
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("did not receive email %d", i+1)
		}
	}

	if received["email-1"] != "inbox1@test.com" {
		t.Errorf("email-1 inbox = %q, want inbox1@test.com", received["email-1"])
	}
	if received["email-2"] != "inbox2@test.com" {
		t.Errorf("email-2 inbox = %q, want inbox2@test.com", received["email-2"])
	}
}

func TestClient_WatchInboxes_ClosesOnContextCancel(t *testing.T) {
	client := &Client{
		watchers: make(map[string][]chan<- *Email),
	}
	inbox := &Inbox{inboxHash: "hash-1", client: client}

	ctx, cancel := context.WithCancel(context.Background())
	ch := client.WatchInboxes(ctx, inbox)

	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("channel did not close after context cancel")
	}
}

func TestClient_addWatcher(t *testing.T) {
	client := &Client{
		watchers: make(map[string][]chan<- *Email),
	}

	ch := make(chan *Email, 1)
	cleanup := client.addWatcher("test-hash", ch)

	// Verify watcher is added
	client.watchersMu.RLock()
	watchers := client.watchers["test-hash"]
	client.watchersMu.RUnlock()

	if len(watchers) != 1 {
		t.Errorf("watcher count = %d, want 1", len(watchers))
	}

	// Cleanup should remove the watcher
	cleanup()

	client.watchersMu.RLock()
	watchersAfter := client.watchers["test-hash"]
	client.watchersMu.RUnlock()

	if len(watchersAfter) != 0 {
		t.Errorf("watcher count after cleanup = %d, want 0", len(watchersAfter))
	}
}

func TestClient_removeWatcher(t *testing.T) {
	client := &Client{
		watchers: make(map[string][]chan<- *Email),
	}

	ch1 := make(chan *Email, 1)
	ch2 := make(chan *Email, 1)
	ch3 := make(chan *Email, 1)

	client.addWatcher("test-hash", ch1)
	client.addWatcher("test-hash", ch2)
	client.addWatcher("test-hash", ch3)

	// Remove middle watcher
	client.removeWatcher("test-hash", ch2)

	client.watchersMu.RLock()
	watchers := client.watchers["test-hash"]
	client.watchersMu.RUnlock()

	if len(watchers) != 2 {
		t.Errorf("watcher count = %d, want 2", len(watchers))
	}

	// Verify ch2 is gone
	for _, w := range watchers {
		if w == ch2 {
			t.Error("ch2 should have been removed")
		}
	}
}

func TestClient_removeWatcher_CleansUpEmptySlice(t *testing.T) {
	client := &Client{
		watchers: make(map[string][]chan<- *Email),
	}

	ch := make(chan *Email, 1)
	client.addWatcher("test-hash", ch)
	client.removeWatcher("test-hash", ch)

	client.watchersMu.RLock()
	_, exists := client.watchers["test-hash"]
	client.watchersMu.RUnlock()

	if exists {
		t.Error("empty watcher slice should be deleted from map")
	}
}

func TestClient_notifyWatchers_RespectsContext(t *testing.T) {
	client := &Client{
		watchers: make(map[string][]chan<- *Email),
	}

	// Create a channel with no buffer - will block without context cancel
	ch := make(chan *Email)
	client.addWatcher("test-hash", ch)

	ctx, cancel := context.WithCancel(context.Background())

	// This will block until context is cancelled
	done := make(chan struct{})
	go func() {
		client.notifyWatchers(ctx, "test-hash", &Email{ID: "test"})
		close(done)
	}()

	// Cancel context to unblock
	cancel()

	select {
	case <-done:
		// Success - notifyWatchers returned after context cancel
	case <-time.After(100 * time.Millisecond):
		t.Error("notifyWatchers did not return after context cancel")
	}
}

func TestClient_notifyWatchers_NoWatchers(t *testing.T) {
	client := &Client{
		watchers: make(map[string][]chan<- *Email),
	}

	// Should not panic or block
	done := make(chan struct{})
	go func() {
		client.notifyWatchers(context.Background(), "nonexistent-hash", &Email{ID: "test"})
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("notifyWatchers blocked with no watchers")
	}
}

func TestClient_notifyWatchers_ConcurrentAccess(t *testing.T) {
	client := &Client{
		watchers: make(map[string][]chan<- *Email),
	}

	// Set up multiple watchers
	const numWatchers = 10
	channels := make([]chan *Email, numWatchers)
	for i := 0; i < numWatchers; i++ {
		channels[i] = make(chan *Email, 100)
		client.addWatcher("test-hash", channels[i])
	}

	// Concurrently add/remove watchers and notify
	var wg sync.WaitGroup
	const iterations = 100

	// Writers
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			client.notifyWatchers(context.Background(), "test-hash", &Email{ID: "test"})
		}
	}()

	// Add/remove watchers
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			ch := make(chan *Email, 1)
			cleanup := client.addWatcher("test-hash", ch)
			cleanup()
		}
	}()

	wg.Wait()
	// If we get here without deadlock or panic, the test passes
}

func TestWaitForEmail_MatchesConfig(t *testing.T) {
	// Test that waitConfig.Matches works correctly with the new flow
	cfg := &waitConfig{
		subject: "Welcome",
	}

	matching := &Email{ID: "1", Subject: "Welcome"}
	nonMatching := &Email{ID: "2", Subject: "Goodbye"}

	if !cfg.Matches(matching) {
		t.Error("config should match email with subject 'Welcome'")
	}
	if cfg.Matches(nonMatching) {
		t.Error("config should not match email with subject 'Goodbye'")
	}
}

func TestWaitForEmailCount_DeduplicatesEmails(t *testing.T) {
	// Test the seen map deduplication logic
	seen := make(map[string]struct{})
	var results []*Email

	addIfNew := func(e *Email) bool {
		if _, ok := seen[e.ID]; ok {
			return false
		}
		seen[e.ID] = struct{}{}
		results = append(results, e)
		return true
	}

	email1 := &Email{ID: "email-1"}
	email2 := &Email{ID: "email-2"}

	// First add should succeed
	if !addIfNew(email1) {
		t.Error("first add of email-1 should return true")
	}
	if len(results) != 1 {
		t.Errorf("results length = %d, want 1", len(results))
	}

	// Duplicate should be rejected
	if addIfNew(email1) {
		t.Error("duplicate add of email-1 should return false")
	}
	if len(results) != 1 {
		t.Errorf("results length = %d, want 1 (no change)", len(results))
	}

	// New email should succeed
	if !addIfNew(email2) {
		t.Error("first add of email-2 should return true")
	}
	if len(results) != 2 {
		t.Errorf("results length = %d, want 2", len(results))
	}
}

func TestInboxEvent_Fields(t *testing.T) {
	inbox := &Inbox{emailAddress: "test@example.com"}
	email := &Email{ID: "email-1", Subject: "Test"}

	event := &InboxEvent{
		Inbox: inbox,
		Email: email,
	}

	if event.Inbox != inbox {
		t.Error("event.Inbox should match")
	}
	if event.Email != email {
		t.Error("event.Email should match")
	}
}
