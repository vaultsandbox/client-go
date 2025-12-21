package delivery

import (
	"context"
	"time"
)

// PollingStrategy implements email delivery via polling.
type PollingStrategy struct {
	apiClient interface{}
}

// NewPollingStrategy creates a new polling strategy.
func NewPollingStrategy(apiClient interface{}) *PollingStrategy {
	return &PollingStrategy{
		apiClient: apiClient,
	}
}

// WaitForEmail waits for an email using polling.
func (p *PollingStrategy) WaitForEmail(ctx context.Context, inboxHash string, fetcher EmailFetcher, matcher EmailMatcher, pollInterval time.Duration) (interface{}, error) {
	if pollInterval == 0 {
		pollInterval = 2 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Check immediately first
	emails, err := fetcher(ctx)
	if err == nil {
		for _, email := range emails {
			if matcher(email) {
				return email, nil
			}
		}
	}

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
	if pollInterval == 0 {
		pollInterval = 2 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

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
	return nil
}
