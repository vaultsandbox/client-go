package vaultsandbox

import (
	"sync"
	"testing"
	"time"
)

func TestInboxMonitor_OnEmail(t *testing.T) {
	// Create a mock client with minimal setup
	c := &Client{
		inboxes: make(map[string]*Inbox),
	}

	// Create a mock inbox
	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
		client:       c,
	}

	// Create monitor without starting it (we'll test callback registration only)
	monitor := &InboxMonitor{
		client:       c,
		inboxes:      []*Inbox{inbox},
		callbacks:    make([]EmailCallback, 0),
		pollInterval: time.Second,
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
		inboxes: make(map[string]*Inbox),
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
		client:       c,
	}

	// Create monitor without starting (to avoid API calls)
	monitor := &InboxMonitor{
		client:       c,
		inboxes:      []*Inbox{inbox},
		callbacks:    make([]EmailCallback, 0),
		pollInterval: time.Second,
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
		inboxes: make(map[string]*Inbox),
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
		client:       c,
	}

	// Create monitor without starting (to avoid API calls)
	monitor := &InboxMonitor{
		client:       c,
		inboxes:      []*Inbox{inbox},
		callbacks:    make([]EmailCallback, 0),
		pollInterval: time.Second,
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
		inboxes: make(map[string]*Inbox),
	}

	inbox := &Inbox{
		emailAddress: "test@example.com",
		inboxHash:    "hash123",
		client:       c,
	}

	// Create monitor without starting (to avoid API calls)
	monitor := &InboxMonitor{
		client:        c,
		inboxes:       []*Inbox{inbox},
		callbacks:     make([]EmailCallback, 0),
		subscriptions: make([]Subscription, 0),
		pollInterval:  time.Second,
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
