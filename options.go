package vaultsandbox

import (
	"net/http"
	"regexp"
	"time"
)

// DeliveryStrategy specifies how the client receives new emails.
type DeliveryStrategy string

const (
	// StrategySSE uses Server-Sent Events for real-time push notifications.
	StrategySSE DeliveryStrategy = "sse"
	// StrategyPolling uses periodic API calls with exponential backoff.
	StrategyPolling DeliveryStrategy = "polling"
)

const (
	defaultBaseURL     = "https://api.vaultsandbox.com"
	defaultWaitTimeout = 60 * time.Second
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

	// Error callback for background sync failures
	onSyncError func(error)
}

// EncryptionMode specifies the desired encryption mode for an inbox.
type EncryptionMode string

const (
	// EncryptionModeDefault uses the server's default encryption setting.
	EncryptionModeDefault EncryptionMode = ""
	// EncryptionModeEncrypted requests an encrypted inbox.
	EncryptionModeEncrypted EncryptionMode = "encrypted"
	// EncryptionModePlain requests a plain (unencrypted) inbox.
	EncryptionModePlain EncryptionMode = "plain"
)

// PersistenceMode specifies the desired persistence mode for an inbox.
type PersistenceMode string

const (
	// PersistenceModeDefault uses the server's default persistence setting.
	PersistenceModeDefault PersistenceMode = ""
	// PersistenceModePersistent requests a persistent inbox.
	PersistenceModePersistent PersistenceMode = "persistent"
	// PersistenceModeEphemeral requests an ephemeral inbox.
	PersistenceModeEphemeral PersistenceMode = "ephemeral"
)

// inboxConfig holds configuration for inbox creation.
type inboxConfig struct {
	ttl          time.Duration
	emailAddress string
	emailAuth    *bool
	encryption   EncryptionMode
	persistence  PersistenceMode
	spamAnalysis *bool
}

// waitConfig holds configuration for waiting on emails.
type waitConfig struct {
	subject      string
	subjectRegex *regexp.Regexp
	from         string
	fromRegex    *regexp.Regexp
	predicate    func(*Email) bool
	timeout      time.Duration
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

// WithOnSyncError sets a callback for errors during background sync.
// This is called when syncInbox fails to fetch emails after an SSE reconnection.
func WithOnSyncError(fn func(error)) Option {
	return func(c *clientConfig) {
		c.onSyncError = fn
	}
}

// PollingConfig holds all polling-related configuration options.
// The defaults work well for most use cases. Only customize these if you have
// specific requirements around polling frequency or backoff behavior.
//
// Example use case for customization: high-frequency testing scenarios where
// you need faster polling, or low-priority background checks where you want
// longer intervals to reduce API calls.
type PollingConfig struct {
	// InitialInterval is the starting polling interval.
	// Default: 2 seconds
	InitialInterval time.Duration

	// MaxBackoff is the maximum polling interval after backoff.
	// Default: 30 seconds
	MaxBackoff time.Duration

	// BackoffMultiplier increases the interval after each poll with no changes.
	// Default: 1.5
	BackoffMultiplier float64

	// JitterFactor adds randomness to prevent synchronized polling.
	// Default: 0.3 (30%)
	JitterFactor float64
}

// WithPollingConfig sets all polling-related options at once.
// This is the recommended way to customize polling behavior when defaults
// don't meet your needs. For most use cases, the defaults work well and
// no configuration is necessary.
//
// Example:
//
//	client := vaultsandbox.New(apiKey, vaultsandbox.WithPollingConfig(vaultsandbox.PollingConfig{
//	    InitialInterval: 1 * time.Second,  // Faster initial polling
//	    MaxBackoff:      10 * time.Second, // Lower max backoff
//	}))
func WithPollingConfig(cfg PollingConfig) Option {
	return func(c *clientConfig) {
		if cfg.InitialInterval > 0 {
			c.pollingInitialInterval = cfg.InitialInterval
		}
		if cfg.MaxBackoff > 0 {
			c.pollingMaxBackoff = cfg.MaxBackoff
		}
		if cfg.BackoffMultiplier > 0 {
			c.pollingBackoffMultiplier = cfg.BackoffMultiplier
		}
		if cfg.JitterFactor > 0 {
			c.pollingJitterFactor = cfg.JitterFactor
		}
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

// WithEmailAuth controls email authentication (SPF, DKIM, DMARC, PTR) for the inbox.
// When enabled, incoming emails are validated and results are available in AuthResults.
// When disabled, authentication checks are skipped and results have status "skipped".
// If not specified, the server default is used.
func WithEmailAuth(enabled bool) InboxOption {
	return func(c *inboxConfig) {
		c.emailAuth = &enabled
	}
}

// WithEncryption sets the encryption mode for the inbox.
// Use [EncryptionModeEncrypted] to create an encrypted inbox, or [EncryptionModePlain]
// for a plain inbox. If not specified, the server's default is used based on its
// encryption policy.
//
// Note: The server's encryption policy may not allow overrides. Use
// [ServerInfo.EncryptionPolicy.CanOverride] to check if the server allows
// per-inbox encryption settings.
func WithEncryption(mode EncryptionMode) InboxOption {
	return func(c *inboxConfig) {
		c.encryption = mode
	}
}

// WithPersistence sets the persistence mode for the inbox.
// Use [PersistenceModePersistent] to create a persistent inbox, or [PersistenceModeEphemeral]
// for an ephemeral inbox. If not specified, the server's default is used based on its
// persistence policy.
//
// Note: The server's persistence policy may not allow overrides. Use
// [ServerInfo.PersistencePolicy.CanOverride] to check if the server allows
// per-inbox persistence settings.
func WithPersistence(mode PersistenceMode) InboxOption {
	return func(c *inboxConfig) {
		c.persistence = mode
	}
}

// WithSpamAnalysis controls spam analysis (Rspamd) for the inbox.
// When enabled, incoming emails are analyzed for spam and results are available
// in [Email.SpamAnalysis]. When disabled, spam analysis is skipped and results
// have status "skipped". If not specified, the server default is used.
//
// Note: This option has no effect if spam analysis is disabled globally on the server.
// Use [ServerInfo.SpamAnalysisEnabled] to check if spam analysis is available.
func WithSpamAnalysis(enabled bool) InboxOption {
	return func(c *inboxConfig) {
		c.spamAnalysis = &enabled
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
