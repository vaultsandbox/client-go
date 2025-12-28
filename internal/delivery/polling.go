package delivery

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

// PollingStrategy implements email delivery via periodic API polling.
// It uses sync-status-based change detection to minimize API calls:
// the strategy first checks a lightweight sync endpoint for changes
// before fetching full email lists.
//
// The strategy maintains per-inbox adaptive backoff. When no new emails
// arrive, polling intervals gradually increase. When changes are detected,
// intervals reset to the initial value for responsive delivery.
type PollingStrategy struct {
	apiClient *api.Client             // API client for making requests.
	inboxes   map[string]*polledInbox // Active inboxes keyed by hash.
	handler   EventHandler            // Callback for new email events.
	onError   func(error)             // Callback for polling errors.
	cancel    context.CancelFunc      // Cancels the poll loop goroutine.
	mu        sync.RWMutex            // Protects inboxes, handler, and onError.
	started   bool                    // Whether polling is active.

	// Configurable polling parameters
	initialInterval   time.Duration
	maxBackoff        time.Duration
	backoffMultiplier float64
	jitterFactor      float64
}

// polledInbox tracks the state of a single inbox being polled.
type polledInbox struct {
	hash         string                 // SHA-256 hash of the inbox public key.
	emailAddress string                 // Email address for API requests.
	lastHash     string                 // Last seen emails hash for change detection.
	seenEmails   map[string]struct{}    // Set of email IDs already delivered.
	interval     time.Duration          // Current adaptive polling interval.
}

// NewPollingStrategy creates a new polling strategy with the given configuration.
// The strategy is created in a stopped state; call Start to begin polling.
func NewPollingStrategy(cfg Config) *PollingStrategy {
	initialInterval := cfg.PollingInitialInterval
	if initialInterval == 0 {
		initialInterval = DefaultPollingInitialInterval
	}

	maxBackoff := cfg.PollingMaxBackoff
	if maxBackoff == 0 {
		maxBackoff = DefaultPollingMaxBackoff
	}

	backoffMultiplier := cfg.PollingBackoffMultiplier
	if backoffMultiplier == 0 {
		backoffMultiplier = DefaultPollingBackoffMultiplier
	}

	jitterFactor := cfg.PollingJitterFactor
	if jitterFactor == 0 {
		jitterFactor = DefaultPollingJitterFactor
	}

	return &PollingStrategy{
		apiClient:         cfg.APIClient,
		inboxes:           make(map[string]*polledInbox),
		initialInterval:   initialInterval,
		maxBackoff:        maxBackoff,
		backoffMultiplier: backoffMultiplier,
		jitterFactor:      jitterFactor,
	}
}

// Name returns the strategy name for logging and debugging.
func (p *PollingStrategy) Name() string {
	return "polling"
}

// Start begins polling for emails on the given inboxes. It spawns a background
// goroutine that periodically checks each inbox for new emails and calls the
// handler for each new email found.
//
// The context controls the lifetime of the polling loop. When the context is
// canceled, polling stops. Alternatively, call Stop to gracefully shut down.
//
// Start returns immediately; the actual polling happens asynchronously.
func (p *PollingStrategy) Start(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error {
	p.mu.Lock()
	p.handler = handler
	for _, inbox := range inboxes {
		p.inboxes[inbox.Hash] = &polledInbox{
			hash:         inbox.Hash,
			emailAddress: inbox.EmailAddress,
			seenEmails:   make(map[string]struct{}),
			interval:     p.initialInterval,
		}
	}
	p.started = true
	p.mu.Unlock()

	ctx, p.cancel = context.WithCancel(ctx)
	go p.pollLoop(ctx)
	return nil
}

// Stop gracefully shuts down the polling strategy. It cancels the polling
// goroutine and marks the strategy as stopped. Stop is idempotent and safe
// to call multiple times. After Stop returns, no more events will be delivered.
func (p *PollingStrategy) Stop() error {
	p.mu.Lock()
	p.started = false
	p.mu.Unlock()

	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

// AddInbox adds an inbox to be monitored. The inbox will be included in the
// next polling cycle. This method is safe to call while polling is active.
func (p *PollingStrategy) AddInbox(inbox InboxInfo) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.inboxes[inbox.Hash] = &polledInbox{
		hash:         inbox.Hash,
		emailAddress: inbox.EmailAddress,
		seenEmails:   make(map[string]struct{}),
		interval:     p.initialInterval,
	}
	return nil
}

// RemoveInbox removes an inbox from monitoring. The inbox will no longer
// be polled after the current cycle completes. This method is safe to call
// while polling is active.
func (p *PollingStrategy) RemoveInbox(inboxHash string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.inboxes, inboxHash)
	return nil
}

// pollLoop is the main polling goroutine. It continuously polls all inboxes
// and sleeps for the minimum wait duration across all inboxes.
func (p *PollingStrategy) pollLoop(ctx context.Context) {
	// Use adaptive polling with per-inbox intervals
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Get minimum wait duration across all inboxes
		minWait := p.pollAll(ctx)
		if minWait == 0 {
			minWait = p.initialInterval
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(minWait):
		}
	}
}

// pollAll polls all inboxes once and returns the minimum wait duration
// for the next poll cycle. The wait duration is determined by the inbox
// with the shortest adaptive interval.
func (p *PollingStrategy) pollAll(ctx context.Context) time.Duration {
	p.mu.RLock()
	inboxList := make([]*polledInbox, 0, len(p.inboxes))
	for _, inbox := range p.inboxes {
		inboxList = append(inboxList, inbox)
	}
	p.mu.RUnlock()

	if len(inboxList) == 0 {
		return p.initialInterval
	}

	for _, inbox := range inboxList {
		p.pollInbox(ctx, inbox)
	}

	// Return minimum wait duration with jitter
	var minWait time.Duration
	for _, inbox := range inboxList {
		wait := p.getWaitDuration(inbox)
		if minWait == 0 || wait < minWait {
			minWait = wait
		}
	}
	return minWait
}

// pollInbox polls a single inbox for new emails. It first checks the sync
// status to detect changes, then fetches emails only if changes are detected.
// This minimizes API calls when no new emails have arrived.
func (p *PollingStrategy) pollInbox(ctx context.Context, inbox *polledInbox) {
	// Check for nil API client
	if p.apiClient == nil {
		return
	}

	// Check sync status first
	sync, err := p.apiClient.GetInboxSync(ctx, inbox.emailAddress)
	if err != nil {
		p.mu.RLock()
		onError := p.onError
		p.mu.RUnlock()
		if onError != nil {
			onError(err)
		}
		return
	}

	// No changes since last poll
	if sync.EmailsHash == inbox.lastHash {
		// Increase backoff
		newInterval := time.Duration(float64(inbox.interval) * p.backoffMultiplier)
		if newInterval > p.maxBackoff {
			newInterval = p.maxBackoff
		}
		inbox.interval = newInterval
		return
	}

	// Changes detected - fetch emails
	inbox.lastHash = sync.EmailsHash
	inbox.interval = p.initialInterval // Reset backoff

	resp, err := p.apiClient.GetEmails(ctx, inbox.emailAddress)
	if err != nil {
		p.mu.RLock()
		onError := p.onError
		p.mu.RUnlock()
		if onError != nil {
			onError(err)
		}
		return
	}

	p.mu.RLock()
	handler := p.handler
	p.mu.RUnlock()

	// Find new emails
	for _, email := range resp.Emails {
		if _, seen := inbox.seenEmails[email.ID]; !seen {
			inbox.seenEmails[email.ID] = struct{}{}

			if handler != nil {
				handler(ctx, &api.SSEEvent{
					InboxID:           inbox.hash,
					EmailID:           email.ID,
					EncryptedMetadata: email.EncryptedMetadata,
				})
			}
		}
	}
}

// getWaitDuration calculates the wait duration for an inbox, adding random
// jitter to the base interval to prevent synchronized polling across clients.
func (p *PollingStrategy) getWaitDuration(inbox *polledInbox) time.Duration {
	// Add jitter to prevent thundering herd
	jitter := time.Duration(rand.Float64() * p.jitterFactor * float64(inbox.interval))
	return inbox.interval + jitter
}

// OnReconnect is a no-op for polling strategy since polling doesn't have
// persistent connections. Polling already checks all inboxes each cycle,
// so no special reconnection handling is needed.
func (p *PollingStrategy) OnReconnect(fn func(ctx context.Context)) {
	// No-op: polling doesn't need reconnection handling
}

// OnError sets a callback that is invoked when a polling error occurs.
// This includes errors from sync status checks and email fetches.
func (p *PollingStrategy) OnError(fn func(error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onError = fn
}
