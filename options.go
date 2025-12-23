package vaultsandbox

import (
	"net/http"
	"regexp"
	"time"
)

// DeliveryStrategy specifies how the client receives new emails.
type DeliveryStrategy string

const (
	// StrategyAuto tries SSE first, falls back to polling.
	StrategyAuto DeliveryStrategy = "auto"
	// StrategySSE uses Server-Sent Events for real-time push notifications.
	StrategySSE DeliveryStrategy = "sse"
	// StrategyPolling uses periodic API calls with exponential backoff.
	StrategyPolling DeliveryStrategy = "polling"
)

const (
	defaultBaseURL      = "https://api.vaultsandbox.com"
	defaultWaitTimeout  = 60 * time.Second
	defaultPollInterval = 2 * time.Second
)

// clientConfig holds configuration for the client.
type clientConfig struct {
	baseURL          string
	httpClient       *http.Client
	deliveryStrategy DeliveryStrategy
	timeout          time.Duration
	retries          int
	retryOn          []int

	// Polling configuration
	pollingInitialInterval   time.Duration
	pollingMaxBackoff        time.Duration
	pollingBackoffMultiplier float64
	pollingJitterFactor      float64
	sseConnectionTimeout     time.Duration
}

// inboxConfig holds configuration for inbox creation.
type inboxConfig struct {
	ttl          time.Duration
	emailAddress string
}

// waitConfig holds configuration for waiting on emails.
type waitConfig struct {
	subject      string
	subjectRegex *regexp.Regexp
	from         string
	fromRegex    *regexp.Regexp
	predicate    func(*Email) bool
	timeout      time.Duration
	pollInterval time.Duration
}

// Option configures the client.
type Option func(*clientConfig)

// InboxOption configures inbox creation.
type InboxOption func(*inboxConfig)

// WaitOption configures email waiting.
type WaitOption func(*waitConfig)

// WithBaseURL sets the API base URL.
func WithBaseURL(url string) Option {
	return func(c *clientConfig) {
		c.baseURL = url
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientConfig) {
		c.httpClient = client
	}
}

// WithDeliveryStrategy sets the delivery strategy.
func WithDeliveryStrategy(strategy DeliveryStrategy) Option {
	return func(c *clientConfig) {
		c.deliveryStrategy = strategy
	}
}

// WithTimeout sets the default timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *clientConfig) {
		c.timeout = timeout
	}
}

// WithRetries sets the number of retries for API calls.
func WithRetries(count int) Option {
	return func(c *clientConfig) {
		c.retries = count
	}
}

// WithRetryOn sets the HTTP status codes that trigger a retry.
// Default: [408, 429, 500, 502, 503, 504]
func WithRetryOn(statusCodes []int) Option {
	return func(c *clientConfig) {
		c.retryOn = statusCodes
	}
}

// WithPollingInitialInterval sets the initial polling interval.
// This is the interval used when emails are actively being received.
// Default: 2 seconds
func WithPollingInitialInterval(interval time.Duration) Option {
	return func(c *clientConfig) {
		c.pollingInitialInterval = interval
	}
}

// WithPollingMaxBackoff sets the maximum polling backoff interval.
// When no new emails arrive, the polling interval increases up to this maximum.
// Default: 30 seconds
func WithPollingMaxBackoff(maxBackoff time.Duration) Option {
	return func(c *clientConfig) {
		c.pollingMaxBackoff = maxBackoff
	}
}

// WithPollingBackoffMultiplier sets the backoff multiplier for polling.
// After each poll with no changes, the interval is multiplied by this factor.
// Default: 1.5
func WithPollingBackoffMultiplier(multiplier float64) Option {
	return func(c *clientConfig) {
		c.pollingBackoffMultiplier = multiplier
	}
}

// WithPollingJitterFactor sets the jitter factor for polling intervals.
// Random jitter up to this fraction of the interval is added to prevent
// synchronized polling across multiple clients.
// Default: 0.3 (30%)
func WithPollingJitterFactor(factor float64) Option {
	return func(c *clientConfig) {
		c.pollingJitterFactor = factor
	}
}

// WithSSEConnectionTimeout sets the timeout for SSE connection establishment.
// When using StrategyAuto, if the SSE connection is not established within
// this timeout, the client falls back to polling.
// Default: 5 seconds
func WithSSEConnectionTimeout(timeout time.Duration) Option {
	return func(c *clientConfig) {
		c.sseConnectionTimeout = timeout
	}
}

// WithTTL sets the inbox time-to-live.
func WithTTL(ttl time.Duration) InboxOption {
	return func(c *inboxConfig) {
		c.ttl = ttl
	}
}

// WithEmailAddress sets a custom email address for the inbox.
func WithEmailAddress(email string) InboxOption {
	return func(c *inboxConfig) {
		c.emailAddress = email
	}
}

// WithSubject filters emails by exact subject match.
func WithSubject(subject string) WaitOption {
	return func(c *waitConfig) {
		c.subject = subject
	}
}

// WithSubjectRegex filters emails by subject regex.
func WithSubjectRegex(pattern *regexp.Regexp) WaitOption {
	return func(c *waitConfig) {
		c.subjectRegex = pattern
	}
}

// WithFrom filters emails by exact sender match.
func WithFrom(from string) WaitOption {
	return func(c *waitConfig) {
		c.from = from
	}
}

// WithFromRegex filters emails by sender regex.
func WithFromRegex(pattern *regexp.Regexp) WaitOption {
	return func(c *waitConfig) {
		c.fromRegex = pattern
	}
}

// WithPredicate filters emails by custom predicate.
func WithPredicate(fn func(*Email) bool) WaitOption {
	return func(c *waitConfig) {
		c.predicate = fn
	}
}

// WithWaitTimeout sets the timeout for waiting.
func WithWaitTimeout(timeout time.Duration) WaitOption {
	return func(c *waitConfig) {
		c.timeout = timeout
	}
}

// WithPollInterval sets the polling interval.
func WithPollInterval(interval time.Duration) WaitOption {
	return func(c *waitConfig) {
		c.pollInterval = interval
	}
}

// Matches checks if an email matches the wait criteria.
func (w *waitConfig) Matches(e *Email) bool {
	if w.subject != "" && e.Subject != w.subject {
		return false
	}
	if w.subjectRegex != nil && !w.subjectRegex.MatchString(e.Subject) {
		return false
	}
	if w.from != "" && e.From != w.from {
		return false
	}
	if w.fromRegex != nil && !w.fromRegex.MatchString(e.From) {
		return false
	}
	if w.predicate != nil && !w.predicate(e) {
		return false
	}
	return true
}
