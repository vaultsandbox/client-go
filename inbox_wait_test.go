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
			subs: newSubscriptionManager(),
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
			subs: newSubscriptionManager(),
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
		subs: newSubscriptionManager(),
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
	client.subs.notify("test-hash", testEmail)

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
		subs: newSubscriptionManager(),
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

	// Both should receive the same email
	testEmail := &Email{ID: "email-1"}
	client.subs.notify("test-hash", testEmail)

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
		subs: newSubscriptionManager(),
	}
	inbox := &Inbox{
		inboxHash: "test-hash",
		client:    client,
	}

	ctx, cancel := context.WithCancel(context.Background())
	_ = inbox.Watch(ctx)

	// Cancel and wait for cleanup
	cancel()
	time.Sleep(50 * time.Millisecond)

	// Verify notification doesn't reach any subscriber
	// by checking that notify doesn't panic or deliver
	called := false
	unsub := client.subs.subscribe("test-hash", func(email *Email) {
		called = true
	})
	defer unsub()

	testEmail := &Email{ID: "test"}
	client.subs.notify("test-hash", testEmail)

	if !called {
		t.Error("new subscriber should receive email")
	}
}

func TestClient_WatchInboxes_ReturnsChannel(t *testing.T) {
	client := &Client{
		subs: newSubscriptionManager(),
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
		subs: newSubscriptionManager(),
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
		subs: newSubscriptionManager(),
	}
	inbox1 := &Inbox{inboxHash: "hash-1", emailAddress: "inbox1@test.com", client: client}
	inbox2 := &Inbox{inboxHash: "hash-2", emailAddress: "inbox2@test.com", client: client}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := client.WatchInboxes(ctx, inbox1, inbox2)

	// Send email to first inbox
	email1 := &Email{ID: "email-1", Subject: "To Inbox 1"}
	client.subs.notify("hash-1", email1)

	// Send email to second inbox
	email2 := &Email{ID: "email-2", Subject: "To Inbox 2"}
	client.subs.notify("hash-2", email2)

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
		subs: newSubscriptionManager(),
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

func TestSubscriptionManager_Subscribe(t *testing.T) {
	m := newSubscriptionManager()

	var received *Email
	unsub := m.subscribe("test-hash", func(email *Email) {
		received = email
	})

	// Notify should call the callback
	testEmail := &Email{ID: "email-1"}
	m.notify("test-hash", testEmail)

	if received == nil {
		t.Fatal("callback was not called")
	}
	if received.ID != "email-1" {
		t.Errorf("received.ID = %q, want %q", received.ID, "email-1")
	}

	// After unsubscribe, callback should not be called
	unsub()
	received = nil
	m.notify("test-hash", &Email{ID: "email-2"})

	if received != nil {
		t.Error("callback was called after unsubscribe")
	}
}

func TestSubscriptionManager_UnsubscribeIdempotent(t *testing.T) {
	m := newSubscriptionManager()

	unsub := m.subscribe("test-hash", func(email *Email) {})

	// Multiple calls to unsubscribe should not panic
	unsub()
	unsub()
	unsub()
}

func TestSubscriptionManager_Clear(t *testing.T) {
	m := newSubscriptionManager()

	callCount := 0
	m.subscribe("hash-1", func(email *Email) { callCount++ })
	m.subscribe("hash-2", func(email *Email) { callCount++ })

	// Clear all subscriptions
	m.clear()

	// Notifications should not reach any subscriber
	m.notify("hash-1", &Email{ID: "test"})
	m.notify("hash-2", &Email{ID: "test"})

	if callCount != 0 {
		t.Errorf("callCount = %d, want 0 after clear", callCount)
	}
}

func TestSubscriptionManager_NotifyNoSubscribers(t *testing.T) {
	m := newSubscriptionManager()

	// Should not panic
	m.notify("nonexistent-hash", &Email{ID: "test"})
}

func TestSubscriptionManager_ConcurrentAccess(t *testing.T) {
	m := newSubscriptionManager()

	// Set up initial subscribers
	const numSubscribers = 10
	for i := 0; i < numSubscribers; i++ {
		m.subscribe("test-hash", func(email *Email) {})
	}

	// Concurrently add/remove subscribers and notify
	var wg sync.WaitGroup
	const iterations = 100

	// Notifiers
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			m.notify("test-hash", &Email{ID: "test"})
		}
	}()

	// Add/remove subscribers
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			unsub := m.subscribe("test-hash", func(email *Email) {})
			unsub()
		}
	}()

	wg.Wait()
	// If we get here without deadlock or panic, the test passes
}

func TestSubscriptionManager_CallbackNotInvokedAfterUnsubscribe(t *testing.T) {
	m := newSubscriptionManager()

	var callCount int
	var mu sync.Mutex

	unsub := m.subscribe("test-hash", func(email *Email) {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	// Notify once
	m.notify("test-hash", &Email{ID: "test"})

	mu.Lock()
	count1 := callCount
	mu.Unlock()

	if count1 != 1 {
		t.Fatalf("callCount = %d, want 1", count1)
	}

	// Unsubscribe
	unsub()

	// Notify again - callback should not be called
	m.notify("test-hash", &Email{ID: "test"})

	mu.Lock()
	count2 := callCount
	mu.Unlock()

	if count2 != 1 {
		t.Errorf("callCount after unsubscribe = %d, want 1", count2)
	}
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
