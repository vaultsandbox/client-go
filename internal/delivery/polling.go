package delivery

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

// Polling backoff constants control the adaptive polling behavior.
// When no new emails are detected, the polling interval increases
// exponentially up to MaxBackoff. When changes are detected, the
// interval resets to InitialInterval.
const (
	// PollingInitialInterval is the starting interval between polls.
	PollingInitialInterval = 2 * time.Second

	// PollingMaxBackoff is the maximum interval between polls.
	PollingMaxBackoff = 30 * time.Second

	// PollingBackoffMultiplier is the factor by which the interval
	// increases after each poll with no changes.
	PollingBackoffMultiplier = 1.5

	// PollingJitterFactor is the maximum random jitter added to
	// poll intervals (as a fraction of the interval) to prevent
	// thundering herd problems when multiple clients poll.
	PollingJitterFactor = 0.3
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
	apiClient *api.Client            // API client for making requests.
	inboxes   map[string]*polledInbox // Active inboxes keyed by hash.
	handler   EventHandler            // Callback for new email events.
	cancel    context.CancelFunc      // Cancels the poll loop goroutine.
	mu        sync.RWMutex            // Protects inboxes and handler.
	started   bool                    // Whether polling is active.
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
	return &PollingStrategy{
		apiClient: cfg.APIClient,
		inboxes:   make(map[string]*polledInbox),
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
			interval:     PollingInitialInterval,
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
		interval:     PollingInitialInterval,
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
			minWait = PollingInitialInterval
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
		return PollingInitialInterval
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
		return
	}

	// No changes since last poll
	if sync.EmailsHash == inbox.lastHash {
		// Increase backoff
		newInterval := time.Duration(float64(inbox.interval) * PollingBackoffMultiplier)
		if newInterval > PollingMaxBackoff {
			newInterval = PollingMaxBackoff
		}
		inbox.interval = newInterval
		return
	}

	// Changes detected - fetch emails
	inbox.lastHash = sync.EmailsHash
	inbox.interval = PollingInitialInterval // Reset backoff

	emails, err := p.apiClient.GetEmailsNew(ctx, inbox.emailAddress)
	if err != nil {
		return
	}

	p.mu.RLock()
	handler := p.handler
	p.mu.RUnlock()

	// Find new emails
	for _, email := range emails {
		if _, seen := inbox.seenEmails[email.ID]; !seen {
			inbox.seenEmails[email.ID] = struct{}{}

			if handler != nil {
				handler(&api.SSEEvent{
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
	jitter := time.Duration(rand.Float64() * PollingJitterFactor * float64(inbox.interval))
	return inbox.interval + jitter
}

// Legacy interface implementation for backward compatibility.
// These methods provide the polling-based WaitForEmail API.

// WaitForEmail waits for an email matching the given criteria using simple polling.
// It blocks until a matching email is found or the context is canceled.
func (p *PollingStrategy) WaitForEmail(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, pollInterval time.Duration) (interface{}, error) {
	return p.WaitForEmailWithSync(ctx, inboxHash, fetcher, matcher, WaitOptions{
		PollInterval: pollInterval,
		SyncFetcher:  nil, // No sync fetcher, use simple polling
	})
}

// WaitForEmailWithSync waits for an email using sync-status-based change detection.
// If opts.SyncFetcher is provided, it uses smart polling that only fetches emails
// when the sync status hash changes, reducing API calls. If SyncFetcher is nil,
// it falls back to simple interval-based polling.
func (p *PollingStrategy) WaitForEmailWithSync(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, opts WaitOptions) (interface{}, error) {
	pollInterval := opts.PollInterval
	if pollInterval == 0 {
		pollInterval = PollingInitialInterval
	}

	// Check immediately first
	emails, err := fetcher(ctx)
	if err == nil {
		for _, email := range emails {
			if matcher(email) {
				return email, nil
			}
		}
	}

	// If no sync fetcher, use simple polling
	if opts.SyncFetcher == nil {
		return p.waitForEmailSimple(ctx, fetcher, matcher, pollInterval)
	}

	// Use sync-status-based smart polling with adaptive backoff
	var lastHash string
	currentBackoff := pollInterval

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Check sync status first (lightweight call)
		syncStatus, err := opts.SyncFetcher(ctx)
		if err != nil {
			// On error, wait and retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(currentBackoff):
			}
			continue
		}

		// Check if there are changes
		if lastHash == "" || syncStatus.EmailsHash != lastHash {
			lastHash = syncStatus.EmailsHash

			if syncStatus.EmailCount > 0 {
				// Changes detected - fetch and check emails
				emails, err := fetcher(ctx)
				if err == nil {
					for _, email := range emails {
						if matcher(email) {
							return email, nil
						}
					}
				}
			}
			// Reset backoff when changes detected
			currentBackoff = pollInterval
		} else {
			// No changes - increase backoff
			currentBackoff = time.Duration(float64(currentBackoff) * PollingBackoffMultiplier)
			if currentBackoff > PollingMaxBackoff {
				currentBackoff = PollingMaxBackoff
			}
		}

		// Apply jitter and wait
		jitter := time.Duration(rand.Float64() * PollingJitterFactor * float64(currentBackoff))
		waitTime := currentBackoff + jitter

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(waitTime):
		}
	}
}

// waitForEmailSimple polls at a fixed interval without sync-status optimization.
func (p *PollingStrategy) waitForEmailSimple(ctx context.Context, fetcher EmailFetcher, matcher EmailMatcher, pollInterval time.Duration) (interface{}, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			emails, err := fetcher(ctx)
			if err != nil {
				continue
			}

			for _, email := range emails {
				if matcher(email) {
					return email, nil
				}
			}
		}
	}
}

// WaitForEmailCount waits until at least count emails match the criteria.
// It blocks until enough matching emails are found or the context is canceled.
func (p *PollingStrategy) WaitForEmailCount(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, pollInterval time.Duration) ([]interface{}, error) {
	return p.WaitForEmailCountWithSync(ctx, inboxHash, fetcher, matcher, count, WaitOptions{
		PollInterval: pollInterval,
		SyncFetcher:  nil,
	})
}

// WaitForEmailCountWithSync waits for multiple emails using sync-status-based
// change detection. Like WaitForEmailWithSync, it uses smart polling when a
// SyncFetcher is provided.
func (p *PollingStrategy) WaitForEmailCountWithSync(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, opts WaitOptions) ([]interface{}, error) {
	pollInterval := opts.PollInterval
	if pollInterval == 0 {
		pollInterval = PollingInitialInterval
	}

	// Check immediately first
	emails, err := fetcher(ctx)
	if err == nil {
		var matching []interface{}
		for _, email := range emails {
			if matcher(email) {
				matching = append(matching, email)
			}
		}
		if len(matching) >= count {
			return matching[:count], nil
		}
	}

	// If no sync fetcher, use simple polling
	if opts.SyncFetcher == nil {
		return p.waitForEmailCountSimple(ctx, fetcher, matcher, count, pollInterval)
	}

	// Use sync-status-based smart polling with adaptive backoff
	var lastHash string
	currentBackoff := pollInterval

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Check sync status first (lightweight call)
		syncStatus, err := opts.SyncFetcher(ctx)
		if err != nil {
			// On error, wait and retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(currentBackoff):
			}
			continue
		}

		// Check if there are changes
		if lastHash == "" || syncStatus.EmailsHash != lastHash {
			lastHash = syncStatus.EmailsHash

			if syncStatus.EmailCount > 0 {
				// Changes detected - fetch and check emails
				emails, err := fetcher(ctx)
				if err == nil {
					var matching []interface{}
					for _, email := range emails {
						if matcher(email) {
							matching = append(matching, email)
						}
					}
					if len(matching) >= count {
						return matching[:count], nil
					}
				}
			}
			// Reset backoff when changes detected
			currentBackoff = pollInterval
		} else {
			// No changes - increase backoff
			currentBackoff = time.Duration(float64(currentBackoff) * PollingBackoffMultiplier)
			if currentBackoff > PollingMaxBackoff {
				currentBackoff = PollingMaxBackoff
			}
		}

		// Apply jitter and wait
		jitter := time.Duration(rand.Float64() * PollingJitterFactor * float64(currentBackoff))
		waitTime := currentBackoff + jitter

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(waitTime):
		}
	}
}

// waitForEmailCountSimple polls at a fixed interval without sync-status optimization.
func (p *PollingStrategy) waitForEmailCountSimple(ctx context.Context, fetcher EmailFetcher, matcher EmailMatcher, count int, pollInterval time.Duration) ([]interface{}, error) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			emails, err := fetcher(ctx)
			if err != nil {
				continue
			}

			var matching []interface{}
			for _, email := range emails {
				if matcher(email) {
					matching = append(matching, email)
				}
			}

			if len(matching) >= count {
				return matching[:count], nil
			}
		}
	}
}

// Close releases resources and stops the polling strategy.
// It is equivalent to calling Stop.
func (p *PollingStrategy) Close() error {
	return p.Stop()
}
