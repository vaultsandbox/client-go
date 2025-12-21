package delivery

import (
	"context"
	"time"
)

// EmailFetcher is a function type for fetching emails from an inbox.
type EmailFetcher func(ctx context.Context) ([]interface{}, error)

// EmailMatcher is a function type for matching emails against criteria.
type EmailMatcher func(email interface{}) bool

// Strategy defines the interface for email delivery strategies.
type Strategy interface {
	// WaitForEmail waits for an email matching the criteria.
	WaitForEmail(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, pollInterval time.Duration) (interface{}, error)

	// WaitForEmailCount waits until the inbox has at least count emails.
	WaitForEmailCount(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, pollInterval time.Duration) ([]interface{}, error)

	// Close closes the strategy and releases resources.
	Close() error
}
