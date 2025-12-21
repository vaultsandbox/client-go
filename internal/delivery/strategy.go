package delivery

import (
	"context"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

// InboxInfo contains the information needed to monitor an inbox for new emails.
type InboxInfo struct {
	// Hash is the SHA-256 hash of the inbox's public key.
	// Used to identify the inbox in SSE connections.
	Hash string

	// EmailAddress is the full email address of the inbox.
	// Used for polling API endpoints that require the email address.
	EmailAddress string
}

// EventHandler is a callback function invoked when a new email arrives.
// The handler receives an SSE event containing the inbox ID, email ID,
// and encrypted metadata. Return an error to signal processing failure
// (currently errors are not propagated, but this may change).
type EventHandler func(event *api.SSEEvent) error

// Strategy defines the interface for email delivery mechanisms.
// Implementations include PollingStrategy, SSEStrategy, and AutoStrategy.
//
// The typical lifecycle is:
//  1. Create a strategy with NewXxxStrategy(cfg)
//  2. Call Start(ctx, inboxes, handler) to begin receiving events
//  3. Optionally call AddInbox/RemoveInbox to modify monitored inboxes
//  4. Call Stop() when done to release resources
//
// All implementations are safe for concurrent use.
type Strategy interface {
	// Start begins listening for emails on the given inboxes.
	// The handler is called for each new email that arrives.
	// Start returns immediately; event delivery is asynchronous.
	Start(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error

	// Stop gracefully shuts down the strategy and releases resources.
	// After Stop returns, no more events will be delivered.
	// Stop is idempotent and safe to call multiple times.
	Stop() error

	// AddInbox adds an inbox to monitor. The inbox will begin receiving
	// events according to the strategy's behavior (immediately for polling,
	// on next reconnection for SSE).
	AddInbox(inbox InboxInfo) error

	// RemoveInbox removes an inbox from monitoring. The inbox will stop
	// receiving events after the current processing cycle completes.
	RemoveInbox(inboxHash string) error

	// Name returns the strategy name for logging and debugging.
	// Examples: "polling", "sse", "auto:sse", "auto:polling"
	Name() string
}

// Config holds configuration shared by all delivery strategies.
type Config struct {
	// APIClient is the API client used for making requests to the server.
	APIClient *api.Client
}

// EmailFetcher is a function that retrieves emails from an inbox.
// Used by WaitForEmail methods to check for new emails.
type EmailFetcher func(ctx context.Context) ([]interface{}, error)

// EmailMatcher is a predicate function that determines whether an email
// matches the desired criteria. Used by WaitForEmail methods to filter
// emails.
type EmailMatcher func(email interface{}) bool

// SyncStatus represents the synchronization status of an inbox.
// Used for efficient change detection in smart polling.
type SyncStatus struct {
	// EmailCount is the total number of emails in the inbox.
	EmailCount int

	// EmailsHash is a hash of the inbox contents that changes when
	// emails are added, deleted, or modified. Used to detect changes
	// without fetching the full email list.
	EmailsHash string
}

// SyncFetcher is a function that retrieves the sync status of an inbox.
// Used for smart polling to detect changes without fetching full email lists.
type SyncFetcher func(ctx context.Context) (*SyncStatus, error)

// WaitOptions contains options for WaitForEmail and WaitForEmailCount operations.
type WaitOptions struct {
	// PollInterval is the base interval between polls.
	// If zero, defaults to PollingInitialInterval.
	PollInterval time.Duration

	// SyncFetcher enables smart polling with hash-based change detection.
	// When set, the strategy first checks the sync status before fetching
	// emails, reducing API calls when no changes have occurred.
	// If nil, simple interval-based polling is used.
	SyncFetcher SyncFetcher
}

// FullStrategy combines the event-driven Strategy interface with blocking
// WaitForEmail methods. This interface is implemented by all concrete
// strategies (PollingStrategy, SSEStrategy, AutoStrategy).
//
// The WaitForEmail methods provide a simpler API for waiting for specific
// emails without managing event handlers. They block until a matching email
// is found or the context is canceled.
type FullStrategy interface {
	Strategy

	// WaitForEmail blocks until an email matching the criteria is found.
	// The fetcher retrieves emails, and the matcher determines if each
	// email matches. Returns the first matching email or an error if
	// the context is canceled.
	WaitForEmail(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, pollInterval time.Duration) (interface{}, error)

	// WaitForEmailWithSync is like WaitForEmail but uses sync-status-based
	// change detection when opts.SyncFetcher is provided. This reduces API
	// calls by checking for changes before fetching the full email list.
	WaitForEmailWithSync(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, opts WaitOptions) (interface{}, error)

	// WaitForEmailCount blocks until at least count matching emails are found.
	// Returns a slice of exactly count matching emails.
	WaitForEmailCount(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, pollInterval time.Duration) ([]interface{}, error)

	// WaitForEmailCountWithSync is like WaitForEmailCount but uses
	// sync-status-based change detection for efficiency.
	WaitForEmailCountWithSync(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, opts WaitOptions) ([]interface{}, error)

	// Close releases resources and stops the strategy.
	// It is equivalent to calling Stop.
	Close() error
}
