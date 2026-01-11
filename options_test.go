package vaultsandbox

import (
	"net/http"
	"regexp"
	"testing"
	"time"
)

func TestDeliveryStrategy_Constants(t *testing.T) {
	if StrategySSE != "sse" {
		t.Errorf("StrategySSE = %s, want sse", StrategySSE)
	}
	if StrategyPolling != "polling" {
		t.Errorf("StrategyPolling = %s, want polling", StrategyPolling)
	}
}

func TestDefaultConstants(t *testing.T) {
	if defaultBaseURL != "https://api.vaultsandbox.com" {
		t.Errorf("defaultBaseURL = %s, want https://api.vaultsandbox.com", defaultBaseURL)
	}
	if defaultWaitTimeout != 60*time.Second {
		t.Errorf("defaultWaitTimeout = %v, want 60s", defaultWaitTimeout)
	}
}

func TestWithBaseURL(t *testing.T) {
	cfg := &clientConfig{}
	WithBaseURL("https://custom.example.com")(cfg)
	if cfg.baseURL != "https://custom.example.com" {
		t.Errorf("baseURL = %s, want https://custom.example.com", cfg.baseURL)
	}
}

func TestWithHTTPClient(t *testing.T) {
	cfg := &clientConfig{}
	customClient := &http.Client{Timeout: 99 * time.Second}
	WithHTTPClient(customClient)(cfg)
	if cfg.httpClient != customClient {
		t.Error("httpClient was not set")
	}
}

func TestWithDeliveryStrategy(t *testing.T) {
	tests := []struct {
		strategy DeliveryStrategy
	}{
		{StrategySSE},
		{StrategyPolling},
	}

	for _, tt := range tests {
		t.Run(string(tt.strategy), func(t *testing.T) {
			cfg := &clientConfig{}
			WithDeliveryStrategy(tt.strategy)(cfg)
			if cfg.deliveryStrategy != tt.strategy {
				t.Errorf("deliveryStrategy = %s, want %s", cfg.deliveryStrategy, tt.strategy)
			}
		})
	}
}

func TestWithTimeout(t *testing.T) {
	cfg := &clientConfig{}
	WithTimeout(120 * time.Second)(cfg)
	if cfg.timeout != 120*time.Second {
		t.Errorf("timeout = %v, want 120s", cfg.timeout)
	}
}

func TestWithRetries(t *testing.T) {
	cfg := &clientConfig{}
	WithRetries(5)(cfg)
	if cfg.retries != 5 {
		t.Errorf("retries = %d, want 5", cfg.retries)
	}
}

func TestWithTTL(t *testing.T) {
	cfg := &inboxConfig{}
	WithTTL(30 * time.Minute)(cfg)
	if cfg.ttl != 30*time.Minute {
		t.Errorf("ttl = %v, want 30m", cfg.ttl)
	}
}

func TestWithEmailAddress(t *testing.T) {
	cfg := &inboxConfig{}
	WithEmailAddress("custom@example.com")(cfg)
	if cfg.emailAddress != "custom@example.com" {
		t.Errorf("emailAddress = %s, want custom@example.com", cfg.emailAddress)
	}
}

func TestWithSubject(t *testing.T) {
	cfg := &waitConfig{}
	WithSubject("Test Subject")(cfg)
	if cfg.subject != "Test Subject" {
		t.Errorf("subject = %s, want Test Subject", cfg.subject)
	}
}

func TestWithSubjectRegex(t *testing.T) {
	cfg := &waitConfig{}
	pattern := regexp.MustCompile("welcome.*")
	WithSubjectRegex(pattern)(cfg)
	if cfg.subjectRegex != pattern {
		t.Error("subjectRegex was not set")
	}
}

func TestWithFrom(t *testing.T) {
	cfg := &waitConfig{}
	WithFrom("sender@example.com")(cfg)
	if cfg.from != "sender@example.com" {
		t.Errorf("from = %s, want sender@example.com", cfg.from)
	}
}

func TestWithFromRegex(t *testing.T) {
	cfg := &waitConfig{}
	pattern := regexp.MustCompile(".*@example.com")
	WithFromRegex(pattern)(cfg)
	if cfg.fromRegex != pattern {
		t.Error("fromRegex was not set")
	}
}

func TestWithPredicate(t *testing.T) {
	cfg := &waitConfig{}
	predicate := func(e *Email) bool { return e.Subject == "Test" }
	WithPredicate(predicate)(cfg)
	if cfg.predicate == nil {
		t.Error("predicate was not set")
	}
}

func TestWithWaitTimeout(t *testing.T) {
	cfg := &waitConfig{}
	WithWaitTimeout(5 * time.Minute)(cfg)
	if cfg.timeout != 5*time.Minute {
		t.Errorf("timeout = %v, want 5m", cfg.timeout)
	}
}

func TestWaitConfig_Matches(t *testing.T) {
	tests := []struct {
		name     string
		config   waitConfig
		email    *Email
		expected bool
	}{
		{
			name:     "empty config matches all",
			config:   waitConfig{},
			email:    &Email{Subject: "Test", From: "sender@example.com"},
			expected: true,
		},
		{
			name:     "subject match",
			config:   waitConfig{subject: "Test"},
			email:    &Email{Subject: "Test"},
			expected: true,
		},
		{
			name:     "subject mismatch",
			config:   waitConfig{subject: "Test"},
			email:    &Email{Subject: "Different"},
			expected: false,
		},
		{
			name:     "subject regex match",
			config:   waitConfig{subjectRegex: regexp.MustCompile("welcome.*")},
			email:    &Email{Subject: "welcome to the system"},
			expected: true,
		},
		{
			name:     "subject regex mismatch",
			config:   waitConfig{subjectRegex: regexp.MustCompile("welcome.*")},
			email:    &Email{Subject: "goodbye"},
			expected: false,
		},
		{
			name:     "from match",
			config:   waitConfig{from: "sender@example.com"},
			email:    &Email{From: "sender@example.com"},
			expected: true,
		},
		{
			name:     "from mismatch",
			config:   waitConfig{from: "sender@example.com"},
			email:    &Email{From: "other@example.com"},
			expected: false,
		},
		{
			name:     "from regex match",
			config:   waitConfig{fromRegex: regexp.MustCompile(".*@example.com")},
			email:    &Email{From: "sender@example.com"},
			expected: true,
		},
		{
			name:     "from regex mismatch",
			config:   waitConfig{fromRegex: regexp.MustCompile(".*@example.com")},
			email:    &Email{From: "sender@other.com"},
			expected: false,
		},
		{
			name:   "predicate match",
			config: waitConfig{predicate: func(e *Email) bool { return e.Subject == "Test" }},
			email:  &Email{Subject: "Test"},
			expected: true,
		},
		{
			name:     "predicate mismatch",
			config:   waitConfig{predicate: func(e *Email) bool { return e.Subject == "Test" }},
			email:    &Email{Subject: "Other"},
			expected: false,
		},
		{
			name: "all conditions match",
			config: waitConfig{
				subject: "Test",
				from:    "sender@example.com",
			},
			email:    &Email{Subject: "Test", From: "sender@example.com"},
			expected: true,
		},
		{
			name: "one condition fails",
			config: waitConfig{
				subject: "Test",
				from:    "sender@example.com",
			},
			email:    &Email{Subject: "Test", From: "other@example.com"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.Matches(tt.email)
			if result != tt.expected {
				t.Errorf("Matches() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestTTLConstants(t *testing.T) {
	if MinTTL != 60*time.Second {
		t.Errorf("MinTTL = %v, want 60s", MinTTL)
	}
	if MaxTTL != 604800*time.Second {
		t.Errorf("MaxTTL = %v, want 7 days", MaxTTL)
	}
}

func TestWithOnSyncError(t *testing.T) {
	cfg := &clientConfig{}

	var called bool
	callback := func(err error) {
		called = true
	}

	WithOnSyncError(callback)(cfg)

	if cfg.onSyncError == nil {
		t.Fatal("onSyncError was not set")
	}

	// Verify the callback is the one we set
	cfg.onSyncError(nil)
	if !called {
		t.Error("callback was not invoked")
	}
}

func TestWithPollingConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   PollingConfig
		validate func(t *testing.T, cfg *clientConfig)
	}{
		{
			name: "all fields set",
			config: PollingConfig{
				InitialInterval:   1 * time.Second,
				MaxBackoff:        10 * time.Second,
				BackoffMultiplier: 2.0,
				JitterFactor:      0.5,
			},
			validate: func(t *testing.T, cfg *clientConfig) {
				if cfg.pollingInitialInterval != 1*time.Second {
					t.Errorf("pollingInitialInterval = %v, want 1s", cfg.pollingInitialInterval)
				}
				if cfg.pollingMaxBackoff != 10*time.Second {
					t.Errorf("pollingMaxBackoff = %v, want 10s", cfg.pollingMaxBackoff)
				}
				if cfg.pollingBackoffMultiplier != 2.0 {
					t.Errorf("pollingBackoffMultiplier = %v, want 2.0", cfg.pollingBackoffMultiplier)
				}
				if cfg.pollingJitterFactor != 0.5 {
					t.Errorf("pollingJitterFactor = %v, want 0.5", cfg.pollingJitterFactor)
				}
			},
		},
		{
			name: "partial fields - only InitialInterval",
			config: PollingConfig{
				InitialInterval: 5 * time.Second,
			},
			validate: func(t *testing.T, cfg *clientConfig) {
				if cfg.pollingInitialInterval != 5*time.Second {
					t.Errorf("pollingInitialInterval = %v, want 5s", cfg.pollingInitialInterval)
				}
				// Other fields should remain zero
				if cfg.pollingMaxBackoff != 0 {
					t.Errorf("pollingMaxBackoff = %v, want 0", cfg.pollingMaxBackoff)
				}
			},
		},
		{
			name: "zero values ignored",
			config: PollingConfig{
				InitialInterval:   0,
				MaxBackoff:        0,
				BackoffMultiplier: 0,
				JitterFactor:      0,
			},
			validate: func(t *testing.T, cfg *clientConfig) {
				// All fields should remain zero since zero values are ignored
				if cfg.pollingInitialInterval != 0 {
					t.Errorf("pollingInitialInterval = %v, want 0", cfg.pollingInitialInterval)
				}
				if cfg.pollingMaxBackoff != 0 {
					t.Errorf("pollingMaxBackoff = %v, want 0", cfg.pollingMaxBackoff)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &clientConfig{}
			WithPollingConfig(tt.config)(cfg)
			tt.validate(t, cfg)
		})
	}
}

func TestWithRetryOn(t *testing.T) {
	cfg := &clientConfig{}
	codes := []int{500, 502, 503}
	WithRetryOn(codes)(cfg)

	if len(cfg.retryOn) != 3 {
		t.Errorf("retryOn length = %d, want 3", len(cfg.retryOn))
	}
	for i, code := range codes {
		if cfg.retryOn[i] != code {
			t.Errorf("retryOn[%d] = %d, want %d", i, cfg.retryOn[i], code)
		}
	}
}
