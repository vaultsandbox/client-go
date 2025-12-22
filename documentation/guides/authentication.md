---
title: Email Authentication Testing
description: Test SPF, DKIM, and DMARC email authentication
---

VaultSandbox validates SPF, DKIM, DMARC, and reverse DNS for every email, helping you catch authentication issues before production.

## Why Test Email Authentication?

Email authentication prevents:

- Emails being marked as spam
- Emails being rejected by receivers
- Domain spoofing and phishing
- Delivery failures

Testing authentication ensures your emails will be trusted by Gmail, Outlook, and other providers.

## Basic Authentication Check

### Using Validate()

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
	log.Fatal(err)
}

validation := email.AuthResults.Validate()

if validation.Passed {
	fmt.Println("All authentication checks passed")
} else {
	fmt.Println("Authentication failures:")
	for _, f := range validation.Failures {
		fmt.Printf("  - %s\n", f)
	}
}
```

### Checking Individual Results

```go
auth := email.AuthResults

if auth.SPF != nil {
	fmt.Printf("SPF: %s\n", auth.SPF.Status)
}
if len(auth.DKIM) > 0 {
	fmt.Printf("DKIM: %s\n", auth.DKIM[0].Status)
}
if auth.DMARC != nil {
	fmt.Printf("DMARC: %s\n", auth.DMARC.Status)
}
if auth.ReverseDNS != nil {
	fmt.Printf("Reverse DNS: %s\n", auth.ReverseDNS.Status)
}
```

## Testing SPF

### Basic SPF Test

```go
func TestEmailPassesSPFCheck(t *testing.T) {
	sendEmail(inbox.EmailAddress())

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	if email.AuthResults.SPF == nil {
		t.Fatal("expected SPF results")
	}
	if email.AuthResults.SPF.Status != "pass" {
		t.Errorf("expected SPF pass, got %s", email.AuthResults.SPF.Status)
	}
}
```

### Detailed SPF Validation

```go
func TestSPFValidationDetails(t *testing.T) {
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	spf := email.AuthResults.SPF
	if spf != nil {
		if spf.Status != "pass" {
			t.Errorf("expected SPF pass, got %s", spf.Status)
		}
		if spf.Domain != "example.com" {
			t.Errorf("expected domain example.com, got %s", spf.Domain)
		}

		t.Logf("SPF %s for %s", spf.Status, spf.Domain)
		t.Logf("Info: %s", spf.Info)
	}
}
```

### Handling SPF Failures

```go
func TestHandlesSPFFailure(t *testing.T) {
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	spf := email.AuthResults.SPF
	if spf != nil && spf.Status != "pass" {
		t.Logf("SPF %s: %s", spf.Status, spf.Info)

		// Common failures
		switch spf.Status {
		case "fail":
			t.Log("Server IP not authorized in SPF record")
			t.Log("Action: Add server IP to SPF record")
		case "softfail":
			t.Log("Server probably not authorized (~all in SPF)")
		case "none":
			t.Log("No SPF record found")
			t.Log("Action: Add SPF record to DNS")
		}
	}
}
```

## Testing DKIM

### Basic DKIM Test

```go
func TestEmailHasValidDKIMSignature(t *testing.T) {
	sendEmail(inbox.EmailAddress())

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	if len(email.AuthResults.DKIM) == 0 {
		t.Fatal("expected DKIM results")
	}
	if email.AuthResults.DKIM[0].Status != "pass" {
		t.Errorf("expected DKIM pass, got %s", email.AuthResults.DKIM[0].Status)
	}
}
```

### Multiple DKIM Signatures

```go
func TestValidatesAllDKIMSignatures(t *testing.T) {
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	dkim := email.AuthResults.DKIM
	if len(dkim) > 0 {
		t.Logf("Email has %d DKIM signature(s)", len(dkim))

		for i, sig := range dkim {
			t.Logf("Signature %d: status=%s, domain=%s, selector=%s",
				i+1, sig.Status, sig.Domain, sig.Selector)

			if sig.Status != "pass" {
				t.Errorf("signature %d failed: %s", i+1, sig.Status)
			}
		}

		// At least one signature should pass
		anyPassed := false
		for _, sig := range dkim {
			if sig.Status == "pass" {
				anyPassed = true
				break
			}
		}
		if !anyPassed {
			t.Error("expected at least one DKIM signature to pass")
		}
	}
}
```

### DKIM Selector Verification

```go
func TestDKIMUsesCorrectSelector(t *testing.T) {
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	if len(email.AuthResults.DKIM) == 0 {
		t.Fatal("expected DKIM results")
	}

	dkim := email.AuthResults.DKIM[0]

	if dkim.Selector != "default" { // Or your expected selector
		t.Errorf("expected selector 'default', got %s", dkim.Selector)
	}
	if dkim.Domain != "example.com" {
		t.Errorf("expected domain example.com, got %s", dkim.Domain)
	}

	// DKIM DNS record should exist at:
	// {selector}._domainkey.{domain}
	t.Logf("DKIM key at: %s._domainkey.%s", dkim.Selector, dkim.Domain)
}
```

## Testing DMARC

### Basic DMARC Test

```go
func TestEmailPassesDMARC(t *testing.T) {
	sendEmail(inbox.EmailAddress())

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	if email.AuthResults.DMARC == nil {
		t.Fatal("expected DMARC results")
	}
	if email.AuthResults.DMARC.Status != "pass" {
		t.Errorf("expected DMARC pass, got %s", email.AuthResults.DMARC.Status)
	}
}
```

### DMARC Policy Verification

```go
func TestDMARCPolicyIsEnforced(t *testing.T) {
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	dmarc := email.AuthResults.DMARC
	if dmarc != nil {
		t.Logf("DMARC status: %s", dmarc.Status)
		t.Logf("DMARC policy: %s", dmarc.Policy)

		// Policy should be restrictive in production
		if dmarc.Policy != "quarantine" && dmarc.Policy != "reject" {
			t.Errorf("expected restrictive policy, got %s", dmarc.Policy)
		}
	}
}
```

### DMARC Alignment Check

```go
func TestDMARCAlignmentRequirements(t *testing.T) {
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	// DMARC requires either SPF or DKIM to align with From domain
	validation := email.AuthResults.Validate()

	if !validation.DMARCPassed {
		t.Log("DMARC failed. Checking alignment:")
		t.Logf("SPF passed: %v", validation.SPFPassed)
		t.Logf("DKIM passed: %v", validation.DKIMPassed)

		// At least one should pass for DMARC to pass
		if !validation.SPFPassed && !validation.DKIMPassed {
			t.Error("neither SPF nor DKIM passed - DMARC cannot align")
		}
	}
}
```

## Testing Reverse DNS

### Basic Reverse DNS Test

```go
func TestServerHasValidReverseDNS(t *testing.T) {
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	rdns := email.AuthResults.ReverseDNS
	if rdns != nil {
		if rdns.Status != "pass" {
			t.Errorf("expected reverse DNS pass, got %s", rdns.Status)
		}
		if rdns.Hostname == "" {
			t.Error("expected hostname to be set")
		}

		t.Logf("Reverse DNS: %s -> %s", rdns.IP, rdns.Hostname)
		t.Logf("Status: %s", rdns.Status)
	}
}
```

## Complete Authentication Test

### All Checks Pass

```go
func TestEmailPassesAllAuthenticationChecks(t *testing.T) {
	app.SendProductionEmail(inbox.EmailAddress())

	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	validation := email.AuthResults.Validate()

	// All checks should pass in production
	if !validation.Passed {
		t.Error("expected all authentication checks to pass")
	}
	if !validation.SPFPassed {
		t.Error("expected SPF to pass")
	}
	if !validation.DKIMPassed {
		t.Error("expected DKIM to pass")
	}
	if !validation.DMARCPassed {
		t.Error("expected DMARC to pass")
	}

	// Log results
	t.Log("Authentication Results:")
	t.Logf("  SPF: %v", validation.SPFPassed)
	t.Logf("  DKIM: %v", validation.DKIMPassed)
	t.Logf("  DMARC: %v", validation.DMARCPassed)
	t.Logf("  Reverse DNS: %v", validation.ReverseDNSPassed)
}
```

### Graceful Failure Handling

```go
func TestHandlesAuthenticationFailuresGracefully(t *testing.T) {
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	validation := email.AuthResults.Validate()

	// Log failures without failing test (for non-production)
	if !validation.Passed {
		t.Log("Authentication issues detected:")
		for _, failure := range validation.Failures {
			t.Logf("  - %s", failure)
		}

		// Provide remediation steps
		if !validation.SPFPassed {
			t.Log("")
			t.Log("To fix SPF:")
			t.Log("  Add to DNS: v=spf1 ip4:YOUR_SERVER_IP -all")
		}

		if !validation.DKIMPassed {
			t.Log("")
			t.Log("To fix DKIM:")
			t.Log("  1. Generate DKIM keys")
			t.Log("  2. Add public key to DNS")
			t.Log("  3. Configure mail server to sign emails")
		}

		if !validation.DMARCPassed {
			t.Log("")
			t.Log("To fix DMARC:")
			t.Log("  Add to DNS: v=DMARC1; p=none; rua=mailto:dmarc@example.com")
		}
	}

	// In production, this should be: if !validation.Passed { t.Fail() }
}
```

## Real-World Testing Patterns

### Pre-Production Validation

```go
func TestValidatesStagingEnvironmentEmailAuth(t *testing.T) {
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	auth := email.AuthResults

	// SPF should be configured
	if auth.SPF != nil {
		if auth.SPF.Status != "pass" && auth.SPF.Status != "neutral" {
			t.Logf("SPF not configured correctly: %s", auth.SPF.Status)
			t.Logf("Info: %s", auth.SPF.Info)
		}
	}

	// DKIM should be present
	if len(auth.DKIM) > 0 {
		anyValid := false
		for _, d := range auth.DKIM {
			if d.Status == "pass" {
				anyValid = true
				break
			}
		}
		if !anyValid {
			t.Log("No valid DKIM signatures")
			t.Log("Fix: Configure DKIM signing in mail server")
		}
	}
}
```

### Production Readiness Check

```go
func TestProductionEmailConfiguration(t *testing.T) {
	email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}

	validation := email.AuthResults.Validate()

	// Production requirements
	t.Log("Production Readiness:")
	t.Logf("  SPF:      %v", validation.SPFPassed)
	t.Logf("  DKIM:     %v", validation.DKIMPassed)
	t.Logf("  DMARC:    %v", validation.DMARCPassed)
	t.Logf("  All:      %v", validation.Passed)

	// Fail if not production-ready
	if !validation.Passed {
		t.Log("")
		t.Log("Email not production-ready:")
		for _, f := range validation.Failures {
			t.Logf("  - %s", f)
		}
		t.Log("")
		t.Log("Fix these issues before deploying to production.")
		t.Fail()
	}
}
```

## Using Package-Level Validation Functions

The `authresults` package provides functions that return errors for validation:

```go
import "github.com/vaultsandbox/client-go/authresults"

// Validate all checks
if err := authresults.Validate(email.AuthResults); err != nil {
	log.Printf("validation failed: %v", err)
}

// Validate individual checks
if err := authresults.ValidateSPF(email.AuthResults); err != nil {
	log.Printf("SPF failed: %v", err)
}

if err := authresults.ValidateDKIM(email.AuthResults); err != nil {
	log.Printf("DKIM failed: %v", err)
}

if err := authresults.ValidateDMARC(email.AuthResults); err != nil {
	log.Printf("DMARC failed: %v", err)
}

if err := authresults.ValidateReverseDNS(email.AuthResults); err != nil {
	log.Printf("Reverse DNS failed: %v", err)
}
```

### Using Sentinel Errors

```go
import (
	"errors"

	"github.com/vaultsandbox/client-go/authresults"
)

if err := authresults.ValidateSPF(email.AuthResults); err != nil {
	switch {
	case errors.Is(err, authresults.ErrSPFFailed):
		fmt.Println("SPF check failed - check your SPF record")
	case errors.Is(err, authresults.ErrNoAuthResults):
		fmt.Println("No SPF results available")
	default:
		fmt.Printf("unexpected error: %v\n", err)
	}
}
```

## Debugging Authentication Issues

### Verbose Logging

```go
func logAuthenticationDetails(email *vaultsandbox.Email) {
	auth := email.AuthResults

	fmt.Println()
	fmt.Println("=== Email Authentication Details ===")
	fmt.Println()

	// SPF
	if auth.SPF != nil {
		fmt.Println("SPF:")
		fmt.Printf("  Status: %s\n", auth.SPF.Status)
		fmt.Printf("  Domain: %s\n", auth.SPF.Domain)
		fmt.Printf("  Info: %s\n", auth.SPF.Info)
	} else {
		fmt.Println("SPF: No result")
	}

	// DKIM
	if len(auth.DKIM) > 0 {
		fmt.Println()
		fmt.Println("DKIM:")
		for i, sig := range auth.DKIM {
			fmt.Printf("  Signature %d:\n", i+1)
			fmt.Printf("    Status: %s\n", sig.Status)
			fmt.Printf("    Domain: %s\n", sig.Domain)
			fmt.Printf("    Selector: %s\n", sig.Selector)
			fmt.Printf("    Info: %s\n", sig.Info)
		}
	} else {
		fmt.Println()
		fmt.Println("DKIM: No signatures")
	}

	// DMARC
	if auth.DMARC != nil {
		fmt.Println()
		fmt.Println("DMARC:")
		fmt.Printf("  Status: %s\n", auth.DMARC.Status)
		fmt.Printf("  Domain: %s\n", auth.DMARC.Domain)
		fmt.Printf("  Policy: %s\n", auth.DMARC.Policy)
	} else {
		fmt.Println()
		fmt.Println("DMARC: No result")
	}

	// Reverse DNS
	if auth.ReverseDNS != nil {
		fmt.Println()
		fmt.Println("Reverse DNS:")
		fmt.Printf("  Status: %s\n", auth.ReverseDNS.Status)
		fmt.Printf("  IP: %s\n", auth.ReverseDNS.IP)
		fmt.Printf("  Hostname: %s\n", auth.ReverseDNS.Hostname)
	}

	// Validation Summary
	validation := auth.Validate()
	fmt.Println()
	fmt.Println("Validation Summary:")
	if validation.Passed {
		fmt.Println("  Overall: PASS")
	} else {
		fmt.Println("  Overall: FAIL")
		fmt.Println("  Failures:")
		for _, f := range validation.Failures {
			fmt.Printf("    - %s\n", f)
		}
	}
}

// Usage
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
	log.Fatal(err)
}
logAuthenticationDetails(email)
```

## Quick Check with IsPassing()

For a simple pass/fail check:

```go
if email.AuthResults.IsPassing() {
	fmt.Println("All authentication checks passed")
} else {
	fmt.Println("Some authentication checks failed")
}
```

**Note**: `IsPassing()` is equivalent to `Validate().Passed` and checks SPF, DKIM, and DMARC. Reverse DNS is not included in this check.

## Next Steps

- **[Authentication Results](/client-go/concepts/auth-results/)** - Deep dive into auth results
- **[Email Objects](/client-go/concepts/emails/)** - Understanding email structure
- **[Testing Patterns](/client-go/testing/password-reset/)** - Real-world test examples
- **[Gateway Security](/gateway/security/)** - Understanding the security model
