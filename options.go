package vaultsandbox

import (
	"net/http"
	"regexp"
	"time"

	"github.com/vaultsandbox/client-go/internal/delivery"
)

const (
	defaultBaseURL      = "https://api.vaultsandbox.com"
	defaultWaitTimeout  = 60 * time.Second
	defaultPollInterval = 2 * time.Second
)

// clientConfig holds configuration for the client.
type clientConfig struct {
	baseURL    string
	httpClient *http.Client
	strategy   delivery.Strategy
	timeout    time.Duration
	retries    int
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

// WithStrategy sets the delivery strategy.
func WithStrategy(strategy delivery.Strategy) Option {
	return func(c *clientConfig) {
		c.strategy = strategy
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
