package authresults

import (
	"errors"
	"strings"
)

var (
	// ErrSPFFailed is returned when SPF check failed.
	ErrSPFFailed = errors.New("SPF check failed")

	// ErrDKIMFailed is returned when DKIM check failed.
	ErrDKIMFailed = errors.New("DKIM check failed")

	// ErrDMARCFailed is returned when DMARC check failed.
	ErrDMARCFailed = errors.New("DMARC check failed")

	// ErrReverseDNSFailed is returned when reverse DNS check failed.
	ErrReverseDNSFailed = errors.New("reverse DNS check failed")

	// ErrNoAuthResults is returned when no auth results are available.
	ErrNoAuthResults = errors.New("no authentication results available")
)

// ValidationError contains details about validation failures.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	return strings.Join(e.Errors, "; ")
}

// Validate checks that all authentication results pass.
// Results with status "skipped" are treated as passed.
func Validate(results *AuthResults) error {
	if results == nil {
		return ErrNoAuthResults
	}

	var errs []string

	// SPF must pass or be skipped
	if results.SPF == nil || (results.SPF.Result != "pass" && results.SPF.Result != "skipped") {
		errs = append(errs, "SPF did not pass")
	}

	// At least one DKIM must pass, or all must be skipped
	dkimPassed := false
	allSkipped := true
	for _, dkim := range results.DKIM {
		if dkim.Result == "pass" {
			dkimPassed = true
			break
		}
		if dkim.Result != "skipped" {
			allSkipped = false
		}
	}
	if !dkimPassed && !(len(results.DKIM) > 0 && allSkipped) {
		errs = append(errs, "no DKIM signature passed")
	}

	// DMARC must pass or be skipped
	if results.DMARC == nil || (results.DMARC.Result != "pass" && results.DMARC.Result != "skipped") {
		errs = append(errs, "DMARC did not pass")
	}

	// ReverseDNS must pass or be skipped if present
	if results.ReverseDNS != nil && results.ReverseDNS.Result != "pass" && results.ReverseDNS.Result != "skipped" {
		errs = append(errs, "reverse DNS did not pass")
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}

	return nil
}

// ValidateSPF validates only SPF results.
// Results with status "skipped" are treated as passed.
func ValidateSPF(results *AuthResults) error {
	if results == nil || results.SPF == nil {
		return ErrNoAuthResults
	}
	if results.SPF.Result != "pass" && results.SPF.Result != "skipped" {
		return ErrSPFFailed
	}
	return nil
}

// ValidateDKIM validates only DKIM results.
// Returns nil if at least one DKIM signature passes, or all are skipped.
func ValidateDKIM(results *AuthResults) error {
	if results == nil || len(results.DKIM) == 0 {
		return ErrNoAuthResults
	}
	allSkipped := true
	for _, dkim := range results.DKIM {
		if dkim.Result == "pass" {
			return nil
		}
		if dkim.Result != "skipped" {
			allSkipped = false
		}
	}
	if allSkipped {
		return nil
	}
	return ErrDKIMFailed
}

// ValidateDMARC validates only DMARC results.
// Results with status "skipped" are treated as passed.
func ValidateDMARC(results *AuthResults) error {
	if results == nil || results.DMARC == nil {
		return ErrNoAuthResults
	}
	if results.DMARC.Result != "pass" && results.DMARC.Result != "skipped" {
		return ErrDMARCFailed
	}
	return nil
}

// ValidateReverseDNS validates only reverse DNS results.
// Results with status "skipped" are treated as passed.
func ValidateReverseDNS(results *AuthResults) error {
	if results == nil || results.ReverseDNS == nil {
		return ErrNoAuthResults
	}
	if results.ReverseDNS.Result != "pass" && results.ReverseDNS.Result != "skipped" {
		return ErrReverseDNSFailed
	}
	return nil
}
