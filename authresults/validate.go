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
func Validate(results *AuthResults) error {
	if results == nil {
		return ErrNoAuthResults
	}

	var errs []string

	// SPF must pass
	if results.SPF == nil || results.SPF.Status != "pass" {
		errs = append(errs, "SPF did not pass")
	}

	// At least one DKIM must pass
	dkimPassed := false
	for _, dkim := range results.DKIM {
		if dkim.Status == "pass" {
			dkimPassed = true
			break
		}
	}
	if !dkimPassed {
		errs = append(errs, "no DKIM signature passed")
	}

	// DMARC must pass
	if results.DMARC == nil || results.DMARC.Status != "pass" {
		errs = append(errs, "DMARC did not pass")
	}

	// ReverseDNS must pass if present
	if results.ReverseDNS != nil && results.ReverseDNS.Status() != "pass" {
		errs = append(errs, "reverse DNS did not pass")
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}

	return nil
}

// ValidateSPF validates only SPF results.
func ValidateSPF(results *AuthResults) error {
	if results == nil || results.SPF == nil {
		return ErrNoAuthResults
	}
	if results.SPF.Status != "pass" {
		return ErrSPFFailed
	}
	return nil
}

// ValidateDKIM validates only DKIM results.
// Returns nil if at least one DKIM signature passes.
func ValidateDKIM(results *AuthResults) error {
	if results == nil || len(results.DKIM) == 0 {
		return ErrNoAuthResults
	}
	for _, dkim := range results.DKIM {
		if dkim.Status == "pass" {
			return nil
		}
	}
	return ErrDKIMFailed
}

// ValidateDMARC validates only DMARC results.
func ValidateDMARC(results *AuthResults) error {
	if results == nil || results.DMARC == nil {
		return ErrNoAuthResults
	}
	if results.DMARC.Status != "pass" {
		return ErrDMARCFailed
	}
	return nil
}

// ValidateReverseDNS validates only reverse DNS results.
func ValidateReverseDNS(results *AuthResults) error {
	if results == nil || results.ReverseDNS == nil {
		return ErrNoAuthResults
	}
	if results.ReverseDNS.Status() != "pass" {
		return ErrReverseDNSFailed
	}
	return nil
}
