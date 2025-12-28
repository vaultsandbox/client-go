package vaultsandbox

import (
	"strconv"
	"sync"
	"sync/atomic"
)

// subscription represents an active email subscription.
type subscription struct {
	id        string
	inboxHash string
	callback  func(*Email)
	active    atomic.Bool
}

// subscriptionManager handles email subscriptions with safe lifecycle management.
// It ensures callbacks are never invoked after unsubscription completes.
type subscriptionManager struct {
	mu     sync.RWMutex
	subs   map[string]map[string]*subscription // inboxHash -> subID -> subscription
	nextID atomic.Uint64
}

// newSubscriptionManager creates a new subscription manager.
func newSubscriptionManager() *subscriptionManager {
	return &subscriptionManager{
		subs: make(map[string]map[string]*subscription),
	}
}

// subscribe registers a callback for emails arriving at the given inbox.
// The callback will be invoked synchronously when emails arrive.
// Returns an unsubscribe function that must be called to clean up.
func (m *subscriptionManager) subscribe(inboxHash string, callback func(*Email)) func() {
	id := strconv.FormatUint(m.nextID.Add(1), 10)

	sub := &subscription{
		id:        id,
		inboxHash: inboxHash,
		callback:  callback,
	}
	sub.active.Store(true)

	m.mu.Lock()
	if m.subs[inboxHash] == nil {
		m.subs[inboxHash] = make(map[string]*subscription)
	}
	m.subs[inboxHash][id] = sub
	m.mu.Unlock()

	return func() {
		m.unsubscribe(inboxHash, id)
	}
}

// unsubscribe removes a subscription. Safe to call multiple times.
func (m *subscriptionManager) unsubscribe(inboxHash, subID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if inboxSubs, ok := m.subs[inboxHash]; ok {
		if sub, ok := inboxSubs[subID]; ok {
			sub.active.Store(false) // Mark inactive before removing
			delete(inboxSubs, subID)
			if len(inboxSubs) == 0 {
				delete(m.subs, inboxHash)
			}
		}
	}
}

// notify calls all registered callbacks for the given inbox.
// Callbacks are invoked synchronously after releasing the read lock.
// The active flag is checked before invoking to prevent calls after unsubscribe.
func (m *subscriptionManager) notify(inboxHash string, email *Email) {
	m.mu.RLock()
	inboxSubs := m.subs[inboxHash]
	if len(inboxSubs) == 0 {
		m.mu.RUnlock()
		return
	}

	// Copy subscriptions to avoid holding lock during callbacks
	subs := make([]*subscription, 0, len(inboxSubs))
	for _, sub := range inboxSubs {
		subs = append(subs, sub)
	}
	m.mu.RUnlock()

	for _, sub := range subs {
		if sub.active.Load() {
			sub.callback(email)
		}
	}
}

// clear removes all subscriptions. Called during Client.Close().
func (m *subscriptionManager) clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, inboxSubs := range m.subs {
		for _, sub := range inboxSubs {
			sub.active.Store(false)
		}
	}
	m.subs = make(map[string]map[string]*subscription)
}
