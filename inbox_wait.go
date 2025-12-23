package vaultsandbox

import (
	"context"
	"sync"
)

// InboxEmailCallback is called when a new email arrives in the inbox.
type InboxEmailCallback func(email *Email)

// WaitForEmail waits for an email matching the given criteria.
// It uses the client's callback infrastructure to receive instant notifications
// when SSE is active, or receives events when the polling handler fires.
func (i *Inbox) WaitForEmail(ctx context.Context, opts ...WaitOption) (*Email, error) {
	cfg := &waitConfig{
		timeout:      defaultWaitTimeout,
		pollInterval: defaultPollInterval,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	resultCh := make(chan *Email, 1)

	// 1. Subscribe FIRST to avoid race condition
	sub := i.OnNewEmail(func(email *Email) {
		if cfg.Matches(email) {
			select {
			case resultCh <- email:
			default: // already found
			}
		}
	})
	defer sub.Unsubscribe()

	// 2. Check existing emails (handles already-arrived case)
	emails, err := i.GetEmails(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range emails {
		if cfg.Matches(e) {
			return e, nil
		}
	}

	// 3. Wait for callback or timeout
	select {
	case email := <-resultCh:
		return email, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// WaitForEmailCount waits until at least count matching emails are found.
// It uses the client's callback infrastructure to receive instant notifications
// when SSE is active, or receives events when the polling handler fires.
func (i *Inbox) WaitForEmailCount(ctx context.Context, count int, opts ...WaitOption) ([]*Email, error) {
	cfg := &waitConfig{
		timeout:      defaultWaitTimeout,
		pollInterval: defaultPollInterval,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	// Use a mutex to protect concurrent access to the results slice
	var mu sync.Mutex
	var results []*Email
	doneCh := make(chan struct{})

	// 1. Subscribe FIRST to avoid race condition
	sub := i.OnNewEmail(func(email *Email) {
		if cfg.Matches(email) {
			mu.Lock()
			// Check if we already have this email (by ID)
			for _, e := range results {
				if e.ID == email.ID {
					mu.Unlock()
					return
				}
			}
			results = append(results, email)
			if len(results) >= count {
				select {
				case doneCh <- struct{}{}:
				default:
				}
			}
			mu.Unlock()
		}
	})
	defer sub.Unsubscribe()

	// 2. Check existing emails (handles already-arrived case)
	emails, err := i.GetEmails(ctx)
	if err != nil {
		return nil, err
	}
	mu.Lock()
	for _, e := range emails {
		if cfg.Matches(e) {
			results = append(results, e)
		}
	}
	if len(results) >= count {
		matched := results[:count]
		mu.Unlock()
		return matched, nil
	}
	mu.Unlock()

	// 3. Wait for callbacks or timeout
	for {
		select {
		case <-doneCh:
			mu.Lock()
			if len(results) >= count {
				matched := results[:count]
				mu.Unlock()
				return matched, nil
			}
			mu.Unlock()
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// OnNewEmail subscribes to new email notifications for this inbox.
// The callback is invoked whenever a new email arrives.
// Returns a Subscription that can be used to unsubscribe.
//
// This method uses the client's delivery strategy (SSE, polling, or auto)
// for real-time email notifications. With SSE enabled, emails are delivered
// instantly as push notifications.
//
// Example:
//
//	subscription := inbox.OnNewEmail(func(email *Email) {
//	    fmt.Printf("New email: %s\n", email.Subject)
//	})
//	defer subscription.Unsubscribe()
func (i *Inbox) OnNewEmail(callback InboxEmailCallback) Subscription {
	unsub := i.client.registerEmailCallback(i.inboxHash, func(inbox *Inbox, email *Email) {
		callback(email)
	})

	return &inboxEmailSubscription{unsubscribe: unsub}
}

// inboxEmailSubscription implements Subscription for single inbox monitoring.
type inboxEmailSubscription struct {
	unsubscribe func()
	once        sync.Once
}

func (s *inboxEmailSubscription) Unsubscribe() {
	s.once.Do(s.unsubscribe)
}
