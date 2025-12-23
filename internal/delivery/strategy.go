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

	// OnReconnect sets a callback that is invoked after each successful
	// connection/reconnection. For SSE, this is called after the EventSource
	// connects. For polling, this is a no-op since polling doesn't have
	// persistent connections. This can be used to sync emails that may have
	// arrived during the reconnection window.
	OnReconnect(fn func(ctx context.Context))
}

// Config holds configuration shared by all delivery strategies.
type Config struct {
	// APIClient is the API client used for making requests to the server.
	APIClient *api.Client

	// PollingInitialInterval is the starting interval between polls.
	// If zero, defaults to DefaultPollingInitialInterval.
	PollingInitialInterval time.Duration

	// PollingMaxBackoff is the maximum interval between polls.
	// If zero, defaults to DefaultPollingMaxBackoff.
	PollingMaxBackoff time.Duration

	// PollingBackoffMultiplier is the factor by which the interval
	// increases after each poll with no changes.
	// If zero, defaults to DefaultPollingBackoffMultiplier.
	PollingBackoffMultiplier float64

	// PollingJitterFactor is the maximum random jitter added to
	// poll intervals (as a fraction of the interval).
	// If zero, defaults to DefaultPollingJitterFactor.
	PollingJitterFactor float64

	// SSEConnectionTimeout is the maximum time to wait for an SSE connection
	// to be established before falling back to polling (when using auto mode).
	// If zero, defaults to DefaultSSEConnectionTimeout.
	SSEConnectionTimeout time.Duration
}

// Default polling configuration values.
const (
	DefaultPollingInitialInterval   = 2 * time.Second
	DefaultPollingMaxBackoff        = 30 * time.Second
	DefaultPollingBackoffMultiplier = 1.5
	DefaultPollingJitterFactor      = 0.3
	DefaultSSEConnectionTimeout     = 5 * time.Second
)

