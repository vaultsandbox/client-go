package vaultsandbox

import (
	"sync"
	"testing"
	"time"
)

func TestInboxMonitor_OnEmail(t *testing.T) {
	// Create a mock client with minimal setup
	c := &Client{
		inboxes:        make(map[string]*Inbox),
		eventCallbacks: make(map[string][]emailEventCallback),
	}

	// Create a mock inbox
	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
		client:       c,
	}

	// Create monitor without starting it (we'll test callback registration only)
	monitor := &InboxMonitor{
		client:          c,
		inboxes:         []*Inbox{inbox},
		callbacks:       make([]EmailCallback, 0),
		callbackIndices: make(map[string]int),
	}

	// Track callback invocations
	var callbackCount int
	var mu sync.Mutex

	// Manually add callback without triggering startMonitoring
	monitor.mu.Lock()
	monitor.callbacks = append(monitor.callbacks, func(inbox *Inbox, email *Email) {
		mu.Lock()
		callbackCount++
		mu.Unlock()
	})
	monitor.mu.Unlock()

	// Verify callback registration works
	monitor.mu.RLock()
	if len(monitor.callbacks) != 1 {
		t.Errorf("callbacks length = %d, want 1", len(monitor.callbacks))
	}
	monitor.mu.RUnlock()
}

func TestInboxMonitor_MultipleCallbacks(t *testing.T) {
	c := &Client{
		inboxes:        make(map[string]*Inbox),
		eventCallbacks: make(map[string][]emailEventCallback),
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
		client:       c,
	}

	// Create monitor without starting (to avoid API calls)
	monitor := &InboxMonitor{
		client:          c,
		inboxes:         []*Inbox{inbox},
		callbacks:       make([]EmailCallback, 0),
		callbackIndices: make(map[string]int),
	}

	var count1, count2 int
	var mu sync.Mutex

	// Manually add callbacks without triggering startMonitoring
	monitor.mu.Lock()
	monitor.callbacks = append(monitor.callbacks, func(inbox *Inbox, email *Email) {
		mu.Lock()
		count1++
		mu.Unlock()
	})
	monitor.callbacks = append(monitor.callbacks, func(inbox *Inbox, email *Email) {
		mu.Lock()
		count2++
		mu.Unlock()
	})
	monitor.mu.Unlock()

	// Emit an email event
	email := &Email{ID: "email1", Subject: "Test"}
	monitor.emitEmail(inbox, email)

	// Wait for callbacks to execute
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if count1 != 1 {
		t.Errorf("callback1 count = %d, want 1", count1)
	}
	if count2 != 1 {
		t.Errorf("callback2 count = %d, want 1", count2)
	}
	mu.Unlock()
}

func TestInboxMonitor_Unsubscribe(t *testing.T) {
	c := &Client{
		inboxes:        make(map[string]*Inbox),
		eventCallbacks: make(map[string][]emailEventCallback),
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
		client:       c,
	}

	// Create monitor without starting (to avoid API calls)
	monitor := &InboxMonitor{
		client:          c,
		inboxes:         []*Inbox{inbox},
		callbacks:       make([]EmailCallback, 0),
		callbackIndices: make(map[string]int),
	}

	var called bool
	var mu sync.Mutex

	// Manually add callback
	monitor.mu.Lock()
	monitor.callbacks = append(monitor.callbacks, func(inbox *Inbox, email *Email) {
		mu.Lock()
		called = true
		mu.Unlock()
	})
	monitor.mu.Unlock()

	// Unsubscribe before emitting
	monitor.Unsubscribe()

	// Emit an email event after unsubscribe
	email := &Email{ID: "email1", Subject: "Test"}
	monitor.emitEmail(inbox, email)

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if called {
		t.Error("callback should not be called after Unsubscribe")
	}
	mu.Unlock()
}

func TestInboxMonitor_SingleSubscriptionUnsubscribe(t *testing.T) {
	c := &Client{
		inboxes:        make(map[string]*Inbox),
		eventCallbacks: make(map[string][]emailEventCallback),
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
		client:       c,
	}

	// Create monitor without starting (to avoid API calls)
	monitor := &InboxMonitor{
		client:          c,
		inboxes:         []*Inbox{inbox},
		callbacks:       make([]EmailCallback, 0),
		subscriptions:   make([]Subscription, 0),
		callbackIndices: make(map[string]int),
	}

	var count1, count2 int
	var mu sync.Mutex

	// Manually add callbacks with subscription tracking
	sub1 := &internalSubscription{
		cancel: func() {
			monitor.mu.Lock()
			defer monitor.mu.Unlock()
			if len(monitor.callbacks) > 0 {
				monitor.callbacks[0] = nil
			}
		},
	}

	monitor.mu.Lock()
	monitor.callbacks = append(monitor.callbacks, func(inbox *Inbox, email *Email) {
		mu.Lock()
		count1++
		mu.Unlock()
	})
	monitor.callbacks = append(monitor.callbacks, func(inbox *Inbox, email *Email) {
		mu.Lock()
		count2++
		mu.Unlock()
	})
	monitor.subscriptions = append(monitor.subscriptions, sub1)
	monitor.mu.Unlock()

	// Unsubscribe only the first callback
	sub1.Unsubscribe()

	// Emit an email event
	email := &Email{ID: "email1", Subject: "Test"}
	monitor.emitEmail(inbox, email)

	// Wait for callbacks to execute
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if count1 != 0 {
		t.Errorf("callback1 count = %d, want 0 (should be unsubscribed)", count1)
	}
	if count2 != 1 {
		t.Errorf("callback2 count = %d, want 1", count2)
	}
	mu.Unlock()
}

func TestSubscription_Interface(t *testing.T) {
	// Verify internalSubscription implements Subscription
	var _ Subscription = &internalSubscription{}
}

func TestMonitorInboxes_EmptyInboxes(t *testing.T) {
	c := &Client{
		inboxes: make(map[string]*Inbox),
	}

	// Should return error for empty inboxes slice
	_, err := c.MonitorInboxes([]*Inbox{})
	if err == nil {
		t.Error("MonitorInboxes should return error for empty inboxes")
	}
}

func TestMonitorInboxes_ClosedClient(t *testing.T) {
	c := &Client{
		inboxes: make(map[string]*Inbox),
		closed:  true,
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		client:       c,
	}

	_, err := c.MonitorInboxes([]*Inbox{inbox})
	if err != ErrClientClosed {
		t.Errorf("MonitorInboxes error = %v, want ErrClientClosed", err)
	}
}

func TestInboxMonitor_MultipleInboxes(t *testing.T) {
	c := &Client{
		inboxes:        make(map[string]*Inbox),
		eventCallbacks: make(map[string][]emailEventCallback),
	}

	// Create multiple inboxes
	inbox1 := &Inbox{
		emailAddress: "test1@example.com",
		inboxHash:    "hash1",
		client:       c,
	}
	inbox2 := &Inbox{
		emailAddress: "test2@example.com",
		inboxHash:    "hash2",
		client:       c,
	}
	inbox3 := &Inbox{
		emailAddress: "test3@example.com",
		inboxHash:    "hash3",
		client:       c,
	}

	// Create monitor without starting it
	monitor := &InboxMonitor{
		client:          c,
		inboxes:         []*Inbox{inbox1, inbox2, inbox3},
		callbacks:       make([]EmailCallback, 0),
		callbackIndices: make(map[string]int),
	}

	// Track callback invocations per inbox
	var invocations sync.Map
	var mu sync.Mutex

	// Add a callback that tracks which inbox received the email
	monitor.mu.Lock()
	monitor.callbacks = append(monitor.callbacks, func(inbox *Inbox, email *Email) {
		mu.Lock()
		defer mu.Unlock()
		count, _ := invocations.LoadOrStore(inbox.inboxHash, 0)
		invocations.Store(inbox.inboxHash, count.(int)+1)
	})
	monitor.mu.Unlock()

	// Emit emails from different inboxes
	monitor.emitEmail(inbox1, &Email{ID: "email1", Subject: "Test1"})
	monitor.emitEmail(inbox2, &Email{ID: "email2", Subject: "Test2"})
	monitor.emitEmail(inbox3, &Email{ID: "email3", Subject: "Test3"})
	monitor.emitEmail(inbox1, &Email{ID: "email4", Subject: "Test4"})

	// Wait for callbacks to execute
	time.Sleep(100 * time.Millisecond)

	// Verify each inbox received the correct number of invocations
	mu.Lock()
	defer mu.Unlock()

	count1, _ := invocations.Load("hash1")
	if count1 != 2 {
		t.Errorf("inbox1 received %v emails, want 2", count1)
	}

	count2, _ := invocations.Load("hash2")
	if count2 != 1 {
		t.Errorf("inbox2 received %v email, want 1", count2)
	}

	count3, _ := invocations.Load("hash3")
	if count3 != 1 {
		t.Errorf("inbox3 received %v email, want 1", count3)
	}
}

func TestInboxMonitor_UnsubscribeAll(t *testing.T) {
	c := &Client{
		inboxes:        make(map[string]*Inbox),
		eventCallbacks: make(map[string][]emailEventCallback),
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
		client:       c,
	}

	// Create monitor without starting (to avoid API calls)
	monitor := &InboxMonitor{
		client:          c,
		inboxes:         []*Inbox{inbox},
		callbacks:       make([]EmailCallback, 0),
		callbackIndices: make(map[string]int),
	}

	var count1, count2, count3 int
	var mu sync.Mutex

	// Manually add multiple callbacks
	monitor.mu.Lock()
	monitor.callbacks = append(monitor.callbacks, func(inbox *Inbox, email *Email) {
		mu.Lock()
		count1++
		mu.Unlock()
	})
	monitor.callbacks = append(monitor.callbacks, func(inbox *Inbox, email *Email) {
		mu.Lock()
		count2++
		mu.Unlock()
	})
	monitor.callbacks = append(monitor.callbacks, func(inbox *Inbox, email *Email) {
		mu.Lock()
		count3++
		mu.Unlock()
	})
	monitor.mu.Unlock()

	// Emit an email event before unsubscribe
	email := &Email{ID: "email1", Subject: "Test"}
	monitor.emitEmail(inbox, email)

	// Wait for callbacks to execute
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if count1 != 1 || count2 != 1 || count3 != 1 {
		t.Errorf("callbacks before unsubscribe: count1=%d, count2=%d, count3=%d, want all 1", count1, count2, count3)
	}
	mu.Unlock()

	// Unsubscribe all
	monitor.Unsubscribe()

	// Emit an email event after unsubscribe
	monitor.emitEmail(inbox, &Email{ID: "email2", Subject: "Test2"})

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if count1 != 1 || count2 != 1 || count3 != 1 {
		t.Errorf("callbacks after unsubscribe: count1=%d, count2=%d, count3=%d, want all still 1", count1, count2, count3)
	}
	mu.Unlock()
}

func TestInboxMonitor_Unsubscribe_Idempotent(t *testing.T) {
	c := &Client{
		inboxes:        make(map[string]*Inbox),
		eventCallbacks: make(map[string][]emailEventCallback),
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
		client:       c,
	}

	// Create monitor without starting
	monitor := &InboxMonitor{
		client:          c,
		inboxes:         []*Inbox{inbox},
		callbacks:       make([]EmailCallback, 0),
		callbackIndices: make(map[string]int),
	}

	// Unsubscribe multiple times should not panic
	monitor.Unsubscribe()
	monitor.Unsubscribe()
	monitor.Unsubscribe()
}
