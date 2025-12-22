package vaultsandbox

import (
	"sync"
)

// Subscription represents an active subscription that can be unsubscribed.
type Subscription interface {
	// Unsubscribe stops the subscription and releases resources.
	Unsubscribe()
}

// EmailCallback is called when a new email arrives.
type EmailCallback func(inbox *Inbox, email *Email)

// InboxMonitor monitors multiple inboxes for new emails.
// It provides an event-emitter like pattern for receiving email notifications.
//
// InboxMonitor uses the client's delivery strategy (SSE, polling, or auto)
// for real-time email notifications. With SSE enabled, emails are delivered
// instantly as push notifications.
type InboxMonitor struct {
	client        *Client
	inboxes       []*Inbox
	callbacks     []EmailCallback
	subscriptions []Subscription
	mu            sync.RWMutex
	started       bool
	unsubscribers map[string]func() // inboxHash -> unsubscribe function
}

// internalSubscription implements the Subscription interface.
type internalSubscription struct {
	cancel func()
}

func (s *internalSubscription) Unsubscribe() {
	if s.cancel != nil {
		s.cancel()
	}
}

// newInboxMonitor creates a new inbox monitor for the given inboxes.
func newInboxMonitor(client *Client, inboxes []*Inbox) *InboxMonitor {
	return &InboxMonitor{
		client:        client,
		inboxes:       inboxes,
		callbacks:     make([]EmailCallback, 0),
		unsubscribers: make(map[string]func()),
	}
}

// OnEmail registers a callback to be called when a new email arrives in any monitored inbox.
// Returns a Subscription that can be used to unsubscribe this specific callback.
func (m *InboxMonitor) OnEmail(callback EmailCallback) Subscription {
	m.mu.Lock()
	m.callbacks = append(m.callbacks, callback)
	callbackIndex := len(m.callbacks) - 1
	m.mu.Unlock()

	// Start monitoring if not already started
	m.startMonitoring()

	sub := &internalSubscription{
		cancel: func() {
			m.mu.Lock()
			defer m.mu.Unlock()
			// Mark this callback as nil (don't remove to preserve indices)
			if callbackIndex < len(m.callbacks) {
				m.callbacks[callbackIndex] = nil
			}
		},
	}

	m.mu.Lock()
	m.subscriptions = append(m.subscriptions, sub)
	m.mu.Unlock()

	return sub
}

// Unsubscribe stops monitoring all inboxes and releases all resources.
func (m *InboxMonitor) Unsubscribe() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Unregister callbacks from client's event system
	for _, unsub := range m.unsubscribers {
		unsub()
	}

	// Clear all callbacks and subscriptions
	m.callbacks = nil
	m.subscriptions = nil
	m.unsubscribers = make(map[string]func())
	m.started = false
}

// startMonitoring begins the monitoring process if not already started.
func (m *InboxMonitor) startMonitoring() {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()

	// Register a callback with the client's event system for each inbox
	for _, inbox := range m.inboxes {
		inboxRef := inbox // capture for closure
		unsub := m.client.registerEmailCallback(inbox.inboxHash, func(inbox *Inbox, email *Email) {
			m.emitEmail(inboxRef, email)
		})
		m.mu.Lock()
		m.unsubscribers[inbox.inboxHash] = unsub
		m.mu.Unlock()
	}
}

// emitEmail calls all registered callbacks with the new email.
func (m *InboxMonitor) emitEmail(inbox *Inbox, email *Email) {
	m.mu.RLock()
	callbacks := make([]EmailCallback, len(m.callbacks))
	copy(callbacks, m.callbacks)
	m.mu.RUnlock()

	// Low volume expected; spawning per-email is fine.
	for _, callback := range callbacks {
		if callback != nil {
			go callback(inbox, email)
		}
	}
}
