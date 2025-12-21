package delivery

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/vaultsandbox/client-go/internal/api"
)

const (
	PollingInitialInterval   = 2 * time.Second
	PollingMaxBackoff        = 30 * time.Second
	PollingBackoffMultiplier = 1.5
	PollingJitterFactor      = 0.3
)

// PollingStrategy implements email delivery via polling.
type PollingStrategy struct {
	apiClient *api.Client
	inboxes   map[string]*polledInbox // keyed by hash
	handler   EventHandler
	cancel    context.CancelFunc
	mu        sync.RWMutex
	started   bool
}

type polledInbox struct {
	hash         string
	emailAddress string // Required for polling API endpoints
	lastHash     string
	seenEmails   map[string]struct{}
	interval     time.Duration
}

// NewPollingStrategy creates a new polling strategy.
func NewPollingStrategy(cfg Config) *PollingStrategy {
	return &PollingStrategy{
		apiClient: cfg.APIClient,
		inboxes:   make(map[string]*polledInbox),
	}
}

// Name returns the strategy name.
func (p *PollingStrategy) Name() string {
	return "polling"
}

// Start begins listening for emails on the given inboxes.
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

// Stop gracefully shuts down the strategy.
func (p *PollingStrategy) Stop() error {
	p.mu.Lock()
	p.started = false
	p.mu.Unlock()

	if p.cancel != nil {
		p.cancel()
	}
	return nil
}

// AddInbox adds an inbox to monitor.
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

// RemoveInbox removes an inbox from monitoring.
func (p *PollingStrategy) RemoveInbox(inboxHash string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.inboxes, inboxHash)
	return nil
}

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

func (p *PollingStrategy) getWaitDuration(inbox *polledInbox) time.Duration {
	// Add jitter to prevent thundering herd
	jitter := time.Duration(rand.Float64() * PollingJitterFactor * float64(inbox.interval))
	return inbox.interval + jitter
}

// Legacy interface implementation for backward compatibility.

// WaitForEmail waits for an email using polling.
func (p *PollingStrategy) WaitForEmail(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, pollInterval time.Duration) (interface{}, error) {
	return p.WaitForEmailWithSync(ctx, inboxHash, fetcher, matcher, WaitOptions{
		PollInterval: pollInterval,
		SyncFetcher:  nil, // No sync fetcher, use simple polling
	})
}

// WaitForEmailWithSync waits for an email using sync-status-based change detection.
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

// WaitForEmailCount waits for multiple emails using polling.
func (p *PollingStrategy) WaitForEmailCount(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, count int, pollInterval time.Duration) ([]interface{}, error) {
	return p.WaitForEmailCountWithSync(ctx, inboxHash, fetcher, matcher, count, WaitOptions{
		PollInterval: pollInterval,
		SyncFetcher:  nil,
	})
}

// WaitForEmailCountWithSync waits for multiple emails using sync-status-based change detection.
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

// Close closes the polling strategy.
func (p *PollingStrategy) Close() error {
	return p.Stop()
}
