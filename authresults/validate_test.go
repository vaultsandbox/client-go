package authresults

import (
	"errors"
	"strings"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrSPFFailed", ErrSPFFailed},
		{"ErrDKIMFailed", ErrDKIMFailed},
		{"ErrDMARCFailed", ErrDMARCFailed},
		{"ErrReverseDNSFailed", ErrReverseDNSFailed},
		{"ErrNoAuthResults", ErrNoAuthResults},
	}

	for _, s := range sentinels {
		t.Run(s.name, func(t *testing.T) {
			if s.err == nil {
				t.Error("sentinel error is nil")
			}
			if s.err.Error() == "" {
				t.Error("sentinel error has empty message")
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		errors   []string
		expected string
	}{
		{
			name:     "empty errors",
			errors:   []string{},
			expected: "validation failed",
		},
		{
			name:     "single error",
			errors:   []string{"SPF did not pass"},
			expected: "SPF did not pass",
		},
		{
			name:     "multiple errors",
			errors:   []string{"SPF did not pass", "DKIM failed"},
			expected: "SPF did not pass; DKIM failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ValidationError{Errors: tt.errors}
			result := err.Error()
			if result != tt.expected {
				t.Errorf("Error() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		results *AuthResults
		wantErr bool
		errMsgs []string
	}{
		{
			name:    "nil results",
			results: nil,
			wantErr: true,
		},
		{
			name: "all passing",
			results: &AuthResults{
				SPF:   &SPFResult{Result: "pass"},
				DKIM:  []DKIMResult{{Result: "pass"}},
				DMARC: &DMARCResult{Result: "pass"},
			},
			wantErr: false,
		},
		{
			name: "all passing with reverse DNS",
			results: &AuthResults{
				SPF:        &SPFResult{Result: "pass"},
				DKIM:       []DKIMResult{{Result: "pass"}},
				DMARC:      &DMARCResult{Result: "pass"},
				ReverseDNS: &ReverseDNSResult{Result: "pass"},
			},
			wantErr: false,
		},
		{
			name: "SPF fails",
			results: &AuthResults{
				SPF:   &SPFResult{Result: "fail"},
				DKIM:  []DKIMResult{{Result: "pass"}},
				DMARC: &DMARCResult{Result: "pass"},
			},
			wantErr: true,
			errMsgs: []string{"SPF"},
		},
		{
			name: "SPF missing",
			results: &AuthResults{
				SPF:   nil,
				DKIM:  []DKIMResult{{Result: "pass"}},
				DMARC: &DMARCResult{Result: "pass"},
			},
			wantErr: true,
			errMsgs: []string{"SPF"},
		},
		{
			name: "DKIM fails",
			results: &AuthResults{
				SPF:   &SPFResult{Result: "pass"},
				DKIM:  []DKIMResult{{Result: "fail"}},
				DMARC: &DMARCResult{Result: "pass"},
			},
			wantErr: true,
			errMsgs: []string{"DKIM"},
		},
		{
			name: "DKIM missing",
			results: &AuthResults{
				SPF:   &SPFResult{Result: "pass"},
				DKIM:  nil,
				DMARC: &DMARCResult{Result: "pass"},
			},
			wantErr: true,
			errMsgs: []string{"DKIM"},
		},
		{
			name: "multiple DKIM one passes",
			results: &AuthResults{
				SPF: &SPFResult{Result: "pass"},
				DKIM: []DKIMResult{
					{Result: "fail"},
					{Result: "pass"},
					{Result: "fail"},
				},
				DMARC: &DMARCResult{Result: "pass"},
			},
			wantErr: false,
		},
		{
			name: "DMARC fails",
			results: &AuthResults{
				SPF:   &SPFResult{Result: "pass"},
				DKIM:  []DKIMResult{{Result: "pass"}},
				DMARC: &DMARCResult{Result: "fail"},
			},
			wantErr: true,
			errMsgs: []string{"DMARC"},
		},
		{
			name: "DMARC missing",
			results: &AuthResults{
				SPF:   &SPFResult{Result: "pass"},
				DKIM:  []DKIMResult{{Result: "pass"}},
				DMARC: nil,
			},
			wantErr: true,
			errMsgs: []string{"DMARC"},
		},
		{
			name: "reverse DNS fails",
			results: &AuthResults{
				SPF:        &SPFResult{Result: "pass"},
				DKIM:       []DKIMResult{{Result: "pass"}},
				DMARC:      &DMARCResult{Result: "pass"},
				ReverseDNS: &ReverseDNSResult{Result: "fail"},
			},
			wantErr: true,
			errMsgs: []string{"reverse DNS"},
		},
		{
			name: "multiple failures",
			results: &AuthResults{
				SPF:   &SPFResult{Result: "fail"},
				DKIM:  []DKIMResult{{Result: "fail"}},
				DMARC: &DMARCResult{Result: "fail"},
			},
			wantErr: true,
			errMsgs: []string{"SPF", "DKIM", "DMARC"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.results)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil && len(tt.errMsgs) > 0 {
				errStr := err.Error()
				for _, msg := range tt.errMsgs {
					if !strings.Contains(errStr, msg) {
						t.Errorf("error message %q does not contain %q", errStr, msg)
					}
				}
			}
		})
	}
}

func TestValidateSPF(t *testing.T) {
	tests := []struct {
		name    string
		results *AuthResults
		wantErr error
	}{
		{
			name:    "nil results",
			results: nil,
			wantErr: ErrNoAuthResults,
		},
		{
			name:    "nil SPF",
			results: &AuthResults{SPF: nil},
			wantErr: ErrNoAuthResults,
		},
		{
			name:    "SPF pass",
			results: &AuthResults{SPF: &SPFResult{Result: "pass"}},
			wantErr: nil,
		},
		{
			name:    "SPF fail",
			results: &AuthResults{SPF: &SPFResult{Result: "fail"}},
			wantErr: ErrSPFFailed,
		},
		{
			name:    "SPF softfail",
			results: &AuthResults{SPF: &SPFResult{Result: "softfail"}},
			wantErr: ErrSPFFailed,
		},
		{
			name:    "SPF neutral",
			results: &AuthResults{SPF: &SPFResult{Result: "neutral"}},
			wantErr: ErrSPFFailed,
		},
		{
			name:    "SPF none",
			results: &AuthResults{SPF: &SPFResult{Result: "none"}},
			wantErr: ErrSPFFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSPF(tt.results)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateSPF() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDKIM(t *testing.T) {
	tests := []struct {
		name    string
		results *AuthResults
		wantErr error
	}{
		{
			name:    "nil results",
			results: nil,
			wantErr: ErrNoAuthResults,
		},
		{
			name:    "empty DKIM",
			results: &AuthResults{DKIM: []DKIMResult{}},
			wantErr: ErrNoAuthResults,
		},
		{
			name:    "nil DKIM",
			results: &AuthResults{DKIM: nil},
			wantErr: ErrNoAuthResults,
		},
		{
			name:    "DKIM pass",
			results: &AuthResults{DKIM: []DKIMResult{{Result: "pass"}}},
			wantErr: nil,
		},
		{
			name:    "DKIM fail",
			results: &AuthResults{DKIM: []DKIMResult{{Result: "fail"}}},
			wantErr: ErrDKIMFailed,
		},
		{
			name: "multiple DKIM one passes",
			results: &AuthResults{DKIM: []DKIMResult{
				{Result: "fail"},
				{Result: "pass"},
			}},
			wantErr: nil,
		},
		{
			name: "multiple DKIM all fail",
			results: &AuthResults{DKIM: []DKIMResult{
				{Result: "fail"},
				{Result: "fail"},
			}},
			wantErr: ErrDKIMFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDKIM(tt.results)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateDKIM() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDMARC(t *testing.T) {
	tests := []struct {
		name    string
		results *AuthResults
		wantErr error
	}{
		{
			name:    "nil results",
			results: nil,
			wantErr: ErrNoAuthResults,
		},
		{
			name:    "nil DMARC",
			results: &AuthResults{DMARC: nil},
			wantErr: ErrNoAuthResults,
		},
		{
			name:    "DMARC pass",
			results: &AuthResults{DMARC: &DMARCResult{Result: "pass"}},
			wantErr: nil,
		},
		{
			name:    "DMARC fail",
			results: &AuthResults{DMARC: &DMARCResult{Result: "fail"}},
			wantErr: ErrDMARCFailed,
		},
		{
			name:    "DMARC none",
			results: &AuthResults{DMARC: &DMARCResult{Result: "none"}},
			wantErr: ErrDMARCFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDMARC(tt.results)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateDMARC() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateReverseDNS(t *testing.T) {
	tests := []struct {
		name    string
		results *AuthResults
		wantErr error
	}{
		{
			name:    "nil results",
			results: nil,
			wantErr: ErrNoAuthResults,
		},
		{
			name:    "nil ReverseDNS",
			results: &AuthResults{ReverseDNS: nil},
			wantErr: ErrNoAuthResults,
		},
		{
			name:    "ReverseDNS pass",
			results: &AuthResults{ReverseDNS: &ReverseDNSResult{Result: "pass"}},
			wantErr: nil,
		},
		{
			name:    "ReverseDNS fail",
			results: &AuthResults{ReverseDNS: &ReverseDNSResult{Result: "fail"}},
			wantErr: ErrReverseDNSFailed,
		},
		{
			name:    "ReverseDNS none",
			results: &AuthResults{ReverseDNS: &ReverseDNSResult{Result: "none"}},
			wantErr: ErrReverseDNSFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReverseDNS(tt.results)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateReverseDNS() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsPassing(t *testing.T) {
	tests := []struct {
		name     string
		results  *AuthResults
		expected bool
	}{
		{
			name:     "nil results",
			results:  nil,
			expected: false,
		},
		{
			name:     "empty results",
			results:  &AuthResults{},
			expected: false, // all three checks (SPF, DKIM, DMARC) must pass
		},
		{
			name: "all passing",
			results: &AuthResults{
				SPF:        &SPFResult{Result: "pass"},
				DKIM:       []DKIMResult{{Result: "pass"}},
				DMARC:      &DMARCResult{Result: "pass"},
				ReverseDNS: &ReverseDNSResult{Result: "pass"},
			},
			expected: true,
		},
		{
			name: "SPF fails",
			results: &AuthResults{
				SPF: &SPFResult{Result: "fail"},
			},
			expected: false,
		},
		{
			name: "DKIM all fail",
			results: &AuthResults{
				DKIM: []DKIMResult{{Result: "fail"}},
			},
			expected: false,
		},
		{
			name: "DMARC fails",
			results: &AuthResults{
				DMARC: &DMARCResult{Result: "fail"},
			},
			expected: false,
		},
		{
			name: "ReverseDNS fails",
			results: &AuthResults{
				ReverseDNS: &ReverseDNSResult{Result: "fail"},
			},
			expected: false,
		},
		{
			name: "only SPF present and passes",
			results: &AuthResults{
				SPF: &SPFResult{Result: "pass"},
			},
			expected: false, // DKIM and DMARC also required
		},
		{
			name: "multiple DKIM one passes",
			results: &AuthResults{
				DKIM: []DKIMResult{
					{Result: "fail"},
					{Result: "pass"},
				},
			},
			expected: false, // SPF and DMARC also required
		},
		{
			name: "multiple DKIM one passes with all checks",
			results: &AuthResults{
				SPF: &SPFResult{Result: "pass"},
				DKIM: []DKIMResult{
					{Result: "fail"},
					{Result: "pass"},
				},
				DMARC: &DMARCResult{Result: "pass"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.results.IsPassing()
			if result != tt.expected {
				t.Errorf("IsPassing() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestResultTypes_Fields(t *testing.T) {
	t.Run("SPFResult", func(t *testing.T) {
		spf := &SPFResult{
			Result:  "pass",
			Domain:  "example.com",
			IP:      "1.2.3.4",
			Details: "SPF record found",
		}

		if spf.Result != "pass" {
			t.Errorf("Result = %s, want pass", spf.Result)
		}
		if spf.Domain != "example.com" {
			t.Errorf("Domain = %s, want example.com", spf.Domain)
		}
		if spf.IP != "1.2.3.4" {
			t.Errorf("IP = %s, want 1.2.3.4", spf.IP)
		}
	})

	t.Run("DKIMResult", func(t *testing.T) {
		dkim := DKIMResult{
			Result:    "pass",
			Domain:    "example.com",
			Selector:  "selector1",
			Signature: "DKIM verified",
		}

		if dkim.Result != "pass" {
			t.Errorf("Result = %s, want pass", dkim.Result)
		}
		if dkim.Selector != "selector1" {
			t.Errorf("Selector = %s, want selector1", dkim.Selector)
		}
	})

	t.Run("DMARCResult", func(t *testing.T) {
		dmarc := &DMARCResult{
			Result:  "pass",
			Policy:  "reject",
			Aligned: true,
			Domain:  "example.com",
		}

		if dmarc.Result != "pass" {
			t.Errorf("Result = %s, want pass", dmarc.Result)
		}
		if dmarc.Policy != "reject" {
			t.Errorf("Policy = %s, want reject", dmarc.Policy)
		}
		if !dmarc.Aligned {
			t.Error("Aligned = false, want true")
		}
	})

	t.Run("ReverseDNSResult", func(t *testing.T) {
		rdns := &ReverseDNSResult{
			Result:   "pass",
			IP:       "1.2.3.4",
			Hostname: "mail.example.com",
		}

		if rdns.Result != "pass" {
			t.Errorf("Result = %s, want pass", rdns.Result)
		}
		if rdns.Hostname != "mail.example.com" {
			t.Errorf("Hostname = %s, want mail.example.com", rdns.Hostname)
		}
	})
}
