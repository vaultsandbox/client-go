package authresults

import (
	"testing"
)

func TestValidate_AllPassing(t *testing.T) {
	t.Parallel()
	ar := &AuthResults{
		SPF:   &SPFResult{Result: "pass", Domain: "example.com"},
		DKIM:  []DKIMResult{{Result: "pass", Domain: "example.com"}},
		DMARC: &DMARCResult{Result: "pass", Domain: "example.com"},
		ReverseDNS: &ReverseDNSResult{Result: "pass", Hostname: "mail.example.com"},
	}

	v := ar.Validate()

	if !v.Passed {
		t.Error("expected Passed to be true when all checks pass")
	}
	if !v.SPFPassed {
		t.Error("expected SPFPassed to be true")
	}
	if !v.DKIMPassed {
		t.Error("expected DKIMPassed to be true")
	}
	if !v.DMARCPassed {
		t.Error("expected DMARCPassed to be true")
	}
	if !v.ReverseDNSPassed {
		t.Error("expected ReverseDNSPassed to be true")
	}
	if len(v.Failures) != 0 {
		t.Errorf("expected no failures, got %v", v.Failures)
	}
}

func TestValidate_SPFFailed(t *testing.T) {
	t.Parallel()
	ar := &AuthResults{
		SPF:   &SPFResult{Result: "fail", Domain: "example.com"},
		DKIM:  []DKIMResult{{Result: "pass", Domain: "example.com"}},
		DMARC: &DMARCResult{Result: "pass", Domain: "example.com"},
	}

	v := ar.Validate()

	if v.Passed {
		t.Error("expected Passed to be false when SPF fails")
	}
	if v.SPFPassed {
		t.Error("expected SPFPassed to be false")
	}
	if len(v.Failures) != 1 {
		t.Errorf("expected 1 failure, got %d", len(v.Failures))
	}
	if v.Failures[0] != "SPF check failed: fail (domain: example.com)" {
		t.Errorf("unexpected failure message: %s", v.Failures[0])
	}
}

func TestValidate_DKIMFailed(t *testing.T) {
	t.Parallel()
	ar := &AuthResults{
		SPF:   &SPFResult{Result: "pass", Domain: "example.com"},
		DKIM:  []DKIMResult{{Result: "fail", Domain: "example.com"}},
		DMARC: &DMARCResult{Result: "pass", Domain: "example.com"},
	}

	v := ar.Validate()

	if v.Passed {
		t.Error("expected Passed to be false when DKIM fails")
	}
	if v.DKIMPassed {
		t.Error("expected DKIMPassed to be false")
	}
	if len(v.Failures) != 1 {
		t.Errorf("expected 1 failure, got %d", len(v.Failures))
	}
}

func TestValidate_DKIMOnePassing(t *testing.T) {
	t.Parallel()
	ar := &AuthResults{
		SPF: &SPFResult{Result: "pass", Domain: "example.com"},
		DKIM: []DKIMResult{
			{Result: "fail", Domain: "bad.com"},
			{Result: "pass", Domain: "example.com"},
		},
		DMARC: &DMARCResult{Result: "pass", Domain: "example.com"},
	}

	v := ar.Validate()

	if !v.Passed {
		t.Error("expected Passed to be true when at least one DKIM passes")
	}
	if !v.DKIMPassed {
		t.Error("expected DKIMPassed to be true when at least one signature passes")
	}
}

func TestValidate_DMARCFailed(t *testing.T) {
	t.Parallel()
	ar := &AuthResults{
		SPF:   &SPFResult{Result: "pass", Domain: "example.com"},
		DKIM:  []DKIMResult{{Result: "pass", Domain: "example.com"}},
		DMARC: &DMARCResult{Result: "fail", Policy: "reject", Domain: "example.com"},
	}

	v := ar.Validate()

	if v.Passed {
		t.Error("expected Passed to be false when DMARC fails")
	}
	if v.DMARCPassed {
		t.Error("expected DMARCPassed to be false")
	}
	if len(v.Failures) != 1 {
		t.Errorf("expected 1 failure, got %d", len(v.Failures))
	}
	if v.Failures[0] != "DMARC policy: fail (policy: reject)" {
		t.Errorf("unexpected failure message: %s", v.Failures[0])
	}
}

func TestValidate_ReverseDNSFailedDoesNotAffectPassed(t *testing.T) {
	t.Parallel()
	ar := &AuthResults{
		SPF:        &SPFResult{Result: "pass", Domain: "example.com"},
		DKIM:       []DKIMResult{{Result: "pass", Domain: "example.com"}},
		DMARC:      &DMARCResult{Result: "pass", Domain: "example.com"},
		ReverseDNS: &ReverseDNSResult{Result: "fail", Hostname: "bad.example.com"},
	}

	v := ar.Validate()

	// passed = spfPassed && dkimPassed && dmarcPassed (NOT reverseDnsPassed)
	if !v.Passed {
		t.Error("expected Passed to be true even when ReverseDNS fails")
	}
	if v.ReverseDNSPassed {
		t.Error("expected ReverseDNSPassed to be false")
	}
	if len(v.Failures) != 1 {
		t.Errorf("expected 1 failure for ReverseDNS, got %d", len(v.Failures))
	}
}

func TestValidate_NilAuthResults(t *testing.T) {
	t.Parallel()
	var ar *AuthResults

	v := ar.Validate()

	if v.Passed {
		t.Error("expected Passed to be false for nil AuthResults")
	}
	if len(v.Failures) != 1 {
		t.Errorf("expected 1 failure, got %d", len(v.Failures))
	}
	if v.Failures[0] != "no authentication results available" {
		t.Errorf("unexpected failure message: %s", v.Failures[0])
	}
}

func TestValidate_EmptyAuthResults(t *testing.T) {
	t.Parallel()
	ar := &AuthResults{}

	v := ar.Validate()

	// With no checks present, all individual checks are false
	// But with no DKIM entries, dkimPassed stays false (no signatures to validate)
	if v.SPFPassed {
		t.Error("expected SPFPassed to be false when SPF is nil")
	}
	if v.DKIMPassed {
		t.Error("expected DKIMPassed to be false when DKIM is empty")
	}
	if v.DMARCPassed {
		t.Error("expected DMARCPassed to be false when DMARC is nil")
	}
	if v.ReverseDNSPassed {
		t.Error("expected ReverseDNSPassed to be false when ReverseDNS is nil")
	}
	// Failures should be empty since there are no checks to fail
	if len(v.Failures) != 0 {
		t.Errorf("expected empty failures for empty AuthResults, got %v", v.Failures)
	}
}

func TestValidate_FailuresIsNeverNil(t *testing.T) {
	t.Parallel()
	ar := &AuthResults{
		SPF:   &SPFResult{Result: "pass"},
		DKIM:  []DKIMResult{{Result: "pass"}},
		DMARC: &DMARCResult{Result: "pass"},
	}

	v := ar.Validate()

	if v.Failures == nil {
		t.Error("expected Failures to be non-nil empty slice, got nil")
	}
}

func TestIsPassing_MatchesValidatePassed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		ar       *AuthResults
		expected bool
	}{
		{
			name: "all passing",
			ar: &AuthResults{
				SPF:   &SPFResult{Result: "pass"},
				DKIM:  []DKIMResult{{Result: "pass"}},
				DMARC: &DMARCResult{Result: "pass"},
			},
			expected: true,
		},
		{
			name: "SPF fail",
			ar: &AuthResults{
				SPF:   &SPFResult{Result: "fail"},
				DKIM:  []DKIMResult{{Result: "pass"}},
				DMARC: &DMARCResult{Result: "pass"},
			},
			expected: false,
		},
		{
			name: "ReverseDNS fail does not affect",
			ar: &AuthResults{
				SPF:        &SPFResult{Result: "pass"},
				DKIM:       []DKIMResult{{Result: "pass"}},
				DMARC:      &DMARCResult{Result: "pass"},
				ReverseDNS: &ReverseDNSResult{Result: "fail"},
			},
			expected: true,
		},
		{
			name:     "nil AuthResults",
			ar:       nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ar.IsPassing()
			if got != tt.expected {
				t.Errorf("IsPassing() = %v, want %v", got, tt.expected)
			}
			// Verify it matches Validate().Passed
			if tt.ar != nil && got != tt.ar.Validate().Passed {
				t.Error("IsPassing() does not match Validate().Passed")
			}
		})
	}
}

func TestJoinStrings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		strs     []string
		sep      string
		expected string
	}{
		{[]string{}, ", ", ""},
		{[]string{"a"}, ", ", "a"},
		{[]string{"a", "b"}, ", ", "a, b"},
		{[]string{"a", "b", "c"}, "-", "a-b-c"},
	}

	for _, tt := range tests {
		got := joinStrings(tt.strs, tt.sep)
		if got != tt.expected {
			t.Errorf("joinStrings(%v, %q) = %q, want %q", tt.strs, tt.sep, got, tt.expected)
		}
	}
}
