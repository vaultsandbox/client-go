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
// The handler receives the connection context and an SSE event containing
// the inbox ID, email ID, and encrypted metadata. Return an error to signal
// processing failure (currently errors are not propagated, but this may change).
type EventHandler func(ctx context.Context, event *api.SSEEvent) error

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

// EmailFetcher is a generic function that retrieves emails from an inbox.
// Used by WaitForEmail methods to check for new emails.
// The type parameter T represents the email type being fetched.
type EmailFetcher[T any] func(ctx context.Context) ([]T, error)

// EmailMatcher is a generic predicate function that determines whether an email
// matches the desired criteria. Used by WaitForEmail methods to filter emails.
// The type parameter T represents the email type being matched.
type EmailMatcher[T any] func(email T) bool

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

// FullStrategy extends the Strategy interface with a Close method.
// This interface is implemented by all concrete strategies
// (PollingStrategy, SSEStrategy, AutoStrategy).
//
// Note: Email waiting is handled at the Inbox level using callbacks,
// which leverages SSE for instant notifications when available.
type FullStrategy interface {
	Strategy

	// Close releases resources and stops the strategy.
	// It is equivalent to calling Stop.
	Close() error

	// OnReconnect sets a callback that is invoked after each successful
	// connection/reconnection. For SSE, this is called after the EventSource
	// connects. For polling, this is a no-op since polling doesn't have
	// persistent connections. This can be used to sync emails that may have
	// arrived during the reconnection window.
	OnReconnect(fn func(ctx context.Context))
}
