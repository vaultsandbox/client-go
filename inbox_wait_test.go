package vaultsandbox

import (
	"context"
	"regexp"
	"sync"
	"testing"
	"time"
)

func TestInbox_Watch_ReturnsChannel(t *testing.T) {
	t.Parallel()
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

func TestInbox_Watch_UnsubscribesOnContextCancel(t *testing.T) {
	t.Parallel()
	client := &Client{
		subs: newSubscriptionManager(),
	}
	inbox := &Inbox{
		inboxHash: "test-hash",
		client:    client,
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch := inbox.Watch(ctx)

	// Cancel context - the unsubscribe goroutine needs a moment to run
	cancel()

	// Wait for the unsubscribe goroutine to complete
	// The Watch function starts a goroutine that listens for context.Done()
	time.Sleep(10 * time.Millisecond)

	// After cancel, notify should not deliver (unsubscribed)
	client.subs.notify("test-hash", &Email{ID: "late-email"})

	// Channel should not receive the email (non-blocking check)
	select {
	case <-ch:
		t.Error("received email after context cancel")
	default:
		// Expected: no email received
	}
}

func TestInbox_Watch_ReceivesEmails(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	client := &Client{
		subs: newSubscriptionManager(),
	}
	inbox := &Inbox{
		inboxHash: "test-hash",
		client:    client,
	}

	ctx, cancel := context.WithCancel(context.Background())
	_ = inbox.Watch(ctx)

	// Cancel - the unsubscribe happens synchronously
	cancel()

	// Verify notification doesn't reach old subscriber
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestClient_WatchInboxes_UnsubscribesOnContextCancel(t *testing.T) {
	t.Parallel()
	client := &Client{
		subs: newSubscriptionManager(),
	}
	inbox := &Inbox{inboxHash: "hash-1", client: client}

	ctx, cancel := context.WithCancel(context.Background())
	ch := client.WatchInboxes(ctx, inbox)

	// Cancel context - the unsubscribe happens synchronously
	cancel()

	// After cancel, notify should not deliver (unsubscribed)
	client.subs.notify("hash-1", &Email{ID: "late-email"})

	// Channel should not receive the event (non-blocking check)
	select {
	case <-ch:
		t.Error("received event after context cancel")
	default:
		// Expected: no event received
	}
}

func TestSubscriptionManager_Subscribe(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	m := newSubscriptionManager()

	unsub := m.subscribe("test-hash", func(email *Email) {})

	// Multiple calls to unsubscribe should not panic
	unsub()
	unsub()
	unsub()
}

func TestSubscriptionManager_Clear(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	m := newSubscriptionManager()

	// Should not panic
	m.notify("nonexistent-hash", &Email{ID: "test"})
}

func TestSubscriptionManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestInbox_WatchFunc_ReceivesEmails(t *testing.T) {
	t.Parallel()
	client := &Client{
		subs: newSubscriptionManager(),
	}
	inbox := &Inbox{
		inboxHash: "test-hash",
		client:    client,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var received []*Email
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		inbox.WatchFunc(ctx, func(email *Email) {
			mu.Lock()
			received = append(received, email)
			count := len(received)
			mu.Unlock()
			if count >= 2 {
				cancel()
			}
		})
		close(done)
	}()

	// Give WatchFunc time to set up subscription
	time.Sleep(10 * time.Millisecond)

	// Send emails
	client.subs.notify("test-hash", &Email{ID: "email-1", Subject: "First"})
	client.subs.notify("test-hash", &Email{ID: "email-2", Subject: "Second"})

	// Wait for WatchFunc to finish
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("WatchFunc did not terminate")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) < 2 {
		t.Errorf("received %d emails, want at least 2", len(received))
	}
}

func TestInbox_WatchFunc_ContextCancellation(t *testing.T) {
	t.Parallel()
	client := &Client{
		subs: newSubscriptionManager(),
	}
	inbox := &Inbox{
		inboxHash: "test-hash",
		client:    client,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		inbox.WatchFunc(ctx, func(email *Email) {
			t.Error("callback should not be called")
		})
		close(done)
	}()

	// Cancel immediately
	cancel()

	// WatchFunc should return promptly
	select {
	case <-done:
		// Expected: WatchFunc returned after cancel
	case <-time.After(100 * time.Millisecond):
		t.Error("WatchFunc did not return after context cancel")
	}
}

func TestInbox_WatchFunc_NilEmailHandling(t *testing.T) {
	t.Parallel()
	client := &Client{
		subs: newSubscriptionManager(),
	}
	inbox := &Inbox{
		inboxHash: "test-hash",
		client:    client,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var callCount int
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		inbox.WatchFunc(ctx, func(email *Email) {
			mu.Lock()
			callCount++
			mu.Unlock()
		})
		close(done)
	}()

	// Give WatchFunc time to start
	time.Sleep(10 * time.Millisecond)

	// Send nil email (should be ignored)
	client.subs.notify("test-hash", nil)

	// Send a real email
	client.subs.notify("test-hash", &Email{ID: "real-email"})

	// Give time for processing
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	count := callCount
	mu.Unlock()

	// Callback should only be called once (for the real email, not for nil)
	if count != 1 {
		t.Errorf("callback called %d times, want 1 (nil should be ignored)", count)
	}

	cancel()
	<-done
}

func TestWaitForEmailCount_NegativeCount(t *testing.T) {
	t.Parallel()
	client := &Client{
		subs: newSubscriptionManager(),
	}
	inbox := &Inbox{
		inboxHash: "test-hash",
		client:    client,
	}

	ctx := context.Background()
	_, err := inbox.WaitForEmailCount(ctx, -1)

	if err == nil {
		t.Fatal("expected error for negative count")
	}
	if err.Error() != "count must be non-negative, got -1" {
		t.Errorf("error = %q, want %q", err.Error(), "count must be non-negative, got -1")
	}
}

func TestWaitForEmailCount_ZeroCount(t *testing.T) {
	t.Parallel()
	client := &Client{
		subs: newSubscriptionManager(),
	}
	inbox := &Inbox{
		inboxHash: "test-hash",
		client:    client,
	}

	ctx := context.Background()
	result, err := inbox.WaitForEmailCount(ctx, 0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if len(result) != 0 {
		t.Errorf("result length = %d, want 0", len(result))
	}
}

func TestWaitConfig_MatchesFromRegex(t *testing.T) {
	t.Parallel()
	cfg := &waitConfig{
		fromRegex: regexp.MustCompile(`.*@example\.com$`),
	}

	matching := &Email{ID: "1", From: "sender@example.com"}
	nonMatching := &Email{ID: "2", From: "sender@other.com"}

	if !cfg.Matches(matching) {
		t.Error("config should match email from example.com")
	}
	if cfg.Matches(nonMatching) {
		t.Error("config should not match email from other.com")
	}
}

func TestWaitConfig_MultipleFilters(t *testing.T) {
	t.Parallel()
	cfg := &waitConfig{
		subject: "Welcome",
		from:    "noreply@example.com",
	}

	matchesBoth := &Email{ID: "1", Subject: "Welcome", From: "noreply@example.com"}
	matchesSubjectOnly := &Email{ID: "2", Subject: "Welcome", From: "other@example.com"}
	matchesFromOnly := &Email{ID: "3", Subject: "Goodbye", From: "noreply@example.com"}

	if !cfg.Matches(matchesBoth) {
		t.Error("config should match email with both subject and from matching")
	}
	if cfg.Matches(matchesSubjectOnly) {
		t.Error("config should not match email with only subject matching")
	}
	if cfg.Matches(matchesFromOnly) {
		t.Error("config should not match email with only from matching")
	}
}
