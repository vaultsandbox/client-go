package authresults

import (
	"errors"
	"fmt"
)

var (
	// ErrSPFFailed is returned when SPF check failed.
	ErrSPFFailed = errors.New("SPF check failed")

	// ErrDKIMFailed is returned when DKIM check failed.
	ErrDKIMFailed = errors.New("DKIM check failed")

	// ErrDMARCFailed is returned when DMARC check failed.
	ErrDMARCFailed = errors.New("DMARC check failed")

	// ErrNoAuthResults is returned when no auth results are available.
	ErrNoAuthResults = errors.New("no authentication results available")
)

// ValidationError contains details about a validation failure.
type ValidationError struct {
	SPF   error
	DKIM  error
	DMARC error
}

func (e *ValidationError) Error() string {
	var msg string
	if e.SPF != nil {
		msg += fmt.Sprintf("SPF: %v; ", e.SPF)
	}
	if e.DKIM != nil {
		msg += fmt.Sprintf("DKIM: %v; ", e.DKIM)
	}
	if e.DMARC != nil {
		msg += fmt.Sprintf("DMARC: %v; ", e.DMARC)
	}
	if msg == "" {
		return "validation failed"
	}
	return msg[:len(msg)-2]
}

// Validate checks if authentication results are valid.
func Validate(results *AuthResults) error {
	if results == nil {
		return ErrNoAuthResults
	}

	var validationErr ValidationError
	hasError := false

	if results.SPF != nil && results.SPF.Result != "pass" {
		validationErr.SPF = fmt.Errorf("%w: %s", ErrSPFFailed, results.SPF.Result)
		hasError = true
	}

	if results.DKIM != nil && results.DKIM.Result != "pass" {
		validationErr.DKIM = fmt.Errorf("%w: %s", ErrDKIMFailed, results.DKIM.Result)
		hasError = true
	}

	if results.DMARC != nil && results.DMARC.Result != "pass" {
		validationErr.DMARC = fmt.Errorf("%w: %s", ErrDMARCFailed, results.DMARC.Result)
		hasError = true
	}

	if hasError {
		return &validationErr
	}

	return nil
}

// ValidateSPF validates only SPF results.
func ValidateSPF(results *AuthResults) error {
	if results == nil || results.SPF == nil {
		return ErrNoAuthResults
	}
	if results.SPF.Result != "pass" {
		return fmt.Errorf("%w: %s", ErrSPFFailed, results.SPF.Result)
	}
	return nil
}

// ValidateDKIM validates only DKIM results.
func ValidateDKIM(results *AuthResults) error {
	if results == nil || results.DKIM == nil {
		return ErrNoAuthResults
	}
	if results.DKIM.Result != "pass" {
		return fmt.Errorf("%w: %s", ErrDKIMFailed, results.DKIM.Result)
	}
	return nil
}

// ValidateDMARC validates only DMARC results.
func ValidateDMARC(results *AuthResults) error {
	if results == nil || results.DMARC == nil {
		return ErrNoAuthResults
	}
	if results.DMARC.Result != "pass" {
		return fmt.Errorf("%w: %s", ErrDMARCFailed, results.DMARC.Result)
	}
	return nil
}
