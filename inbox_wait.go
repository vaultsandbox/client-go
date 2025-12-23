package vaultsandbox

import (
	"context"
)

// Watch returns a channel that receives emails as they arrive.
// The channel closes when the context is cancelled.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
//	defer cancel()
//
//	for email := range inbox.Watch(ctx) {
//	    fmt.Printf("New email: %s\n", email.Subject)
//	}
func (i *Inbox) Watch(ctx context.Context) <-chan *Email {
	ch := make(chan *Email, 16)

	cleanup := i.client.addWatcher(i.inboxHash, ch)

	go func() {
		<-ctx.Done()
		cleanup()
		close(ch)
	}()

	return ch
}

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
	for email := range emails {
		if cfg.Matches(email) {
			return email, nil
		}
	}

	return nil, ctx.Err()
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
	for email := range emails {
		addIfNew(email)
		if len(results) >= count {
			return results[:count], nil
		}
	}

	return nil, ctx.Err()
}
