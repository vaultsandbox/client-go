package vaultsandbox

import (
	"net/http"
	"regexp"
	"testing"
	"time"
)

func TestDeliveryStrategy_Constants(t *testing.T) {
	if StrategyAuto != "auto" {
		t.Errorf("StrategyAuto = %s, want auto", StrategyAuto)
	}
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
	if defaultPollInterval != 2*time.Second {
		t.Errorf("defaultPollInterval = %v, want 2s", defaultPollInterval)
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
		{StrategyAuto},
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

func TestWithPollInterval(t *testing.T) {
	cfg := &waitConfig{}
	WithPollInterval(10 * time.Second)(cfg)
	if cfg.pollInterval != 10*time.Second {
		t.Errorf("pollInterval = %v, want 10s", cfg.pollInterval)
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
