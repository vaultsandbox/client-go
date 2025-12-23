package vaultsandbox

import (
	"context"
	"fmt"
)

// Watch returns a channel that receives emails as they arrive.
// The channel is not closed when the context is cancelled; use a select
// on ctx.Done() to detect cancellation.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
//	defer cancel()
//
//	ch := inbox.Watch(ctx)
//	for {
//	    select {
//	    case <-ctx.Done():
//	        return
//	    case email := <-ch:
//	        fmt.Printf("New email: %s\n", email.Subject)
//	    }
//	}
func (i *Inbox) Watch(ctx context.Context) <-chan *Email {
	ch := make(chan *Email, 16)

	// Subscribe with callback that sends to channel
	unsubscribe := i.client.subs.subscribe(i.inboxHash, func(email *Email) {
		select {
		case ch <- email:
		default:
			// Buffer full, drop (same behavior as before)
		}
	})

	// Cleanup goroutine: unsubscribe when context is cancelled.
	// We intentionally do not close(ch) to avoid a race where an
	// in-flight callback tries to send after close.
	go func() {
		<-ctx.Done()
		unsubscribe()
	}()

	return ch
}

// WatchFunc calls fn for each email as they arrive until the context is cancelled.
// This is a convenience wrapper around Watch for simpler use cases.
//
// Example:
//
//	inbox.WatchFunc(ctx, func(email *vaultsandbox.Email) {
//	    fmt.Printf("New email: %s\n", email.Subject)
//	})
func (i *Inbox) WatchFunc(ctx context.Context, fn func(*Email)) {
	emails := i.Watch(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case email := <-emails:
			if email != nil {
				fn(email)
			}
		}
	}
}

// WaitForEmail waits for an email matching the given criteria.
// It uses the client's callback infrastructure to receive instant notifications
// when SSE is active, or receives events when the polling handler fires.
func (i *Inbox) WaitForEmail(ctx context.Context, opts ...WaitOption) (*Email, error) {
	cfg := &waitConfig{
		timeout: defaultWaitTimeout,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	// 1. Start watching FIRST to avoid race condition
	emails := i.Watch(ctx)

	// 2. Check existing emails (handles already-arrived case)
	existing, err := i.GetEmails(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range existing {
		if cfg.Matches(e) {
			return e, nil
		}
	}

	// 3. Watch for new emails
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case email := <-emails:
			if email != nil && cfg.Matches(email) {
				return email, nil
			}
		}
	}
}

// WaitForEmailCount waits until at least count matching emails are found.
// It uses the client's callback infrastructure to receive instant notifications
// when SSE is active, or receives events when the polling handler fires.
func (i *Inbox) WaitForEmailCount(ctx context.Context, count int, opts ...WaitOption) ([]*Email, error) {
	if count < 0 {
		return nil, fmt.Errorf("count must be non-negative, got %d", count)
	}
	if count == 0 {
		return []*Email{}, nil
	}

	cfg := &waitConfig{
		timeout: defaultWaitTimeout,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	// Track seen email IDs to avoid duplicates
	seen := make(map[string]struct{})
	var results []*Email

	// Helper to add email if not seen
	addIfNew := func(e *Email) bool {
		if _, ok := seen[e.ID]; ok {
			return false
		}
		if cfg.Matches(e) {
			seen[e.ID] = struct{}{}
			results = append(results, e)
			return true
		}
		return false
	}

	// 1. Start watching FIRST to avoid race condition
	emails := i.Watch(ctx)

	// 2. Check existing emails (handles already-arrived case)
	existing, err := i.GetEmails(ctx)
	if err != nil {
		return nil, err
	}
	for _, e := range existing {
		addIfNew(e)
		if len(results) >= count {
			return results[:count], nil
		}
	}

	// 3. Watch for new emails
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case email := <-emails:
			if email != nil {
				addIfNew(email)
				if len(results) >= count {
					return results[:count], nil
				}
			}
		}
	}
}
