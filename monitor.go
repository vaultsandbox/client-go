package vaultsandbox

import (
	"context"
	"sync"
	"time"
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
type InboxMonitor struct {
	client        *Client
	inboxes       []*Inbox
	callbacks     []EmailCallback
	subscriptions []Subscription
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
	started       bool
	pollInterval  time.Duration
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
	ctx, cancel := context.WithCancel(context.Background())
	return &InboxMonitor{
		client:       client,
		inboxes:      inboxes,
		callbacks:    make([]EmailCallback, 0),
		ctx:          ctx,
		cancel:       cancel,
		pollInterval: defaultPollInterval,
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

	// Cancel the context to stop all goroutines
	if m.cancel != nil {
		m.cancel()
	}

	// Clear all callbacks and subscriptions
	m.callbacks = nil
	m.subscriptions = nil
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

	// Start a goroutine for each inbox to poll for new emails
	for _, inbox := range m.inboxes {
		go m.monitorInbox(inbox)
	}
}

// monitorInbox polls a single inbox for new emails.
func (m *InboxMonitor) monitorInbox(inbox *Inbox) {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	// Track seen email IDs to detect new emails
	seenEmails := make(map[string]struct{})

	// Initial fetch to populate seen emails
	if emails, err := inbox.GetEmails(m.ctx); err == nil {
		for _, email := range emails {
			seenEmails[email.ID] = struct{}{}
		}
	}

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			emails, err := inbox.GetEmails(m.ctx)
			if err != nil {
				continue
			}

			for _, email := range emails {
				if _, seen := seenEmails[email.ID]; !seen {
					seenEmails[email.ID] = struct{}{}
					m.emitEmail(inbox, email)
				}
			}
		}
	}
}

// emitEmail calls all registered callbacks with the new email.
func (m *InboxMonitor) emitEmail(inbox *Inbox, email *Email) {
	m.mu.RLock()
	callbacks := make([]EmailCallback, len(m.callbacks))
	copy(callbacks, m.callbacks)
	m.mu.RUnlock()

	for _, callback := range callbacks {
		if callback != nil {
			// Call each callback in its own goroutine to prevent blocking
			go callback(inbox, email)
		}
	}
}
