package delivery

import (
	"context"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

// InboxInfo contains the information needed to monitor an inbox.
type InboxInfo struct {
	Hash         string // SHA-256 hash of public key (used for SSE)
	EmailAddress string // Email address (used for polling API calls)
}

// EventHandler is called when a new email arrives.
type EventHandler func(event *api.SSEEvent) error

// Strategy defines the interface for email delivery mechanisms.
type Strategy interface {
	// Start begins listening for emails on the given inboxes.
	Start(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error

	// Stop gracefully shuts down the strategy.
	Stop() error

	// AddInbox adds an inbox to monitor (for SSE, updates connection).
	AddInbox(inbox InboxInfo) error

	// RemoveInbox removes an inbox from monitoring.
	RemoveInbox(inboxHash string) error

	// Name returns the strategy name for logging/debugging.
	Name() string
}

// Config holds common strategy configuration.
type Config struct {
	APIClient *api.Client
}

// EmailFetcher is a function type for fetching emails from an inbox.
type EmailFetcher func(ctx context.Context) ([]interface{}, error)

// EmailMatcher is a function type for matching emails against criteria.
type EmailMatcher func(email interface{}) bool

// FullStrategy combines the event-driven Strategy interface with the
// polling-based methods for backward compatibility.
type FullStrategy interface {
	Strategy

	// WaitForEmail waits for an email matching the criteria using polling.
	WaitForEmail(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, pollInterval time.Duration) (interface{}, error)

	// WaitForEmailCount waits until at least count emails match the criteria.
	WaitForEmailCount(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, pollInterval time.Duration) ([]interface{}, error)

	// Close closes the strategy and releases resources.
	Close() error
}
