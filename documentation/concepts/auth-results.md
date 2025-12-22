---
title: Authentication Results
description: Understanding email authentication (SPF, DKIM, DMARC) in VaultSandbox
---

VaultSandbox validates email authentication for every received email, providing detailed SPF, DKIM, DMARC, and reverse DNS results.

## What is Email Authentication?

Email authentication helps verify that an email:

- Came from the claimed sender domain (**SPF**)
- Wasn't modified in transit (**DKIM**)
- Complies with the domain's policy (**DMARC**)
- Came from a legitimate mail server (**Reverse DNS**)

## AuthResults Struct

Every email has an `AuthResults` field:

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    log.Fatal(err)
}

auth := email.AuthResults

fmt.Printf("SPF: %+v\n", auth.SPF)
fmt.Printf("DKIM: %+v\n", auth.DKIM)
fmt.Printf("DMARC: %+v\n", auth.DMARC)
fmt.Printf("ReverseDNS: %+v\n", auth.ReverseDNS)
```

The `AuthResults` struct is defined in the `authresults` package:

```go
import "github.com/vaultsandbox/client-go/authresults"

type AuthResults struct {
    SPF        *SPFResult
    DKIM       []DKIMResult
    DMARC      *DMARCResult
    ReverseDNS *ReverseDNSResult
}
```

## SPF (Sender Policy Framework)

Verifies the sending server is authorized to send from the sender's domain.

### SPFResult Struct

```go
type SPFResult struct {
    Status string // "pass", "fail", "softfail", "neutral", "none", "temperror", "permerror"
    Domain string
    IP     string
    Info   string
}
```

```go
if email.AuthResults.SPF != nil {
    spf := email.AuthResults.SPF
    fmt.Printf("Status: %s\n", spf.Status)
    fmt.Printf("Domain: %s\n", spf.Domain)
    fmt.Printf("Info: %s\n", spf.Info)
}
```

### SPF Status Values

| Status      | Meaning                                    |
| ----------- | ------------------------------------------ |
| `pass`      | Sending server is authorized               |
| `fail`      | Sending server is NOT authorized           |
| `softfail`  | Probably not authorized (policy says ~all) |
| `neutral`   | Domain makes no assertion                  |
| `temperror` | Temporary error during check               |
| `permerror` | Permanent error in SPF record              |
| `none`      | No SPF record found                        |

### SPF Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    t.Fatal(err)
}

if email.AuthResults.SPF != nil {
    spf := email.AuthResults.SPF

    if spf.Status != "pass" {
        t.Errorf("expected SPF pass, got %s", spf.Status)
    }
    if spf.Domain != "example.com" {
        t.Errorf("expected domain example.com, got %s", spf.Domain)
    }

    fmt.Printf("SPF %s for %s\n", spf.Status, spf.Domain)
}
```

## DKIM (DomainKeys Identified Mail)

Cryptographically verifies the email hasn't been modified and came from the claimed domain.

### DKIMResult Struct

```go
type DKIMResult struct {
    Status   string // "pass", "fail", "none"
    Domain   string
    Selector string
    Info     string
}
```

```go
dkim := email.AuthResults.DKIM // []DKIMResult

if len(dkim) > 0 {
    for _, result := range dkim {
        fmt.Printf("Status: %s\n", result.Status)
        fmt.Printf("Domain: %s\n", result.Domain)
        fmt.Printf("Selector: %s\n", result.Selector)
        fmt.Printf("Info: %s\n", result.Info)
    }
}
```

**Note**: An email can have multiple DKIM signatures (one per signing domain).

### DKIM Status Values

| Status | Meaning                 |
| ------ | ----------------------- |
| `pass` | Signature is valid      |
| `fail` | Signature is invalid    |
| `none` | No DKIM signature found |

### DKIM Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    t.Fatal(err)
}

if len(email.AuthResults.DKIM) > 0 {
    dkim := email.AuthResults.DKIM[0]

    if dkim.Status != "pass" {
        t.Errorf("expected DKIM pass, got %s", dkim.Status)
    }
    if dkim.Domain != "example.com" {
        t.Errorf("expected domain example.com, got %s", dkim.Domain)
    }

    fmt.Printf("DKIM %s (%s._domainkey.%s)\n", dkim.Status, dkim.Selector, dkim.Domain)
}
```

## DMARC (Domain-based Message Authentication)

Checks that SPF or DKIM align with the From address and enforces the domain's policy.

### DMARCResult Struct

```go
type DMARCResult struct {
    Status  string // "pass", "fail", "none"
    Policy  string // "none", "quarantine", "reject"
    Aligned bool
    Domain  string
    Info    string
}
```

```go
if email.AuthResults.DMARC != nil {
    dmarc := email.AuthResults.DMARC
    fmt.Printf("Status: %s\n", dmarc.Status)
    fmt.Printf("Domain: %s\n", dmarc.Domain)
    fmt.Printf("Policy: %s\n", dmarc.Policy)
    fmt.Printf("Aligned: %v\n", dmarc.Aligned)
    fmt.Printf("Info: %s\n", dmarc.Info)
}
```

### DMARC Status Values

| Status | Meaning                                  |
| ------ | ---------------------------------------- |
| `pass` | DMARC check passed (SPF or DKIM aligned) |
| `fail` | DMARC check failed                       |
| `none` | No DMARC policy found                    |

### DMARC Policies

| Policy       | Meaning                         |
| ------------ | ------------------------------- |
| `none`       | No action (monitoring only)     |
| `quarantine` | Treat suspicious emails as spam |
| `reject`     | Reject emails that fail DMARC   |

### DMARC Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    t.Fatal(err)
}

if email.AuthResults.DMARC != nil {
    dmarc := email.AuthResults.DMARC

    if dmarc.Status != "pass" {
        t.Errorf("expected DMARC pass, got %s", dmarc.Status)
    }
    if dmarc.Domain != "example.com" {
        t.Errorf("expected domain example.com, got %s", dmarc.Domain)
    }

    fmt.Printf("DMARC %s (policy: %s)\n", dmarc.Status, dmarc.Policy)
}
```

## Reverse DNS

Verifies the sending server's IP resolves to a hostname that matches the sending domain.

### ReverseDNSResult Struct

```go
type ReverseDNSResult struct {
    Status   string // "pass", "fail", "none"
    IP       string
    Hostname string
    Info     string
}
```

```go
if email.AuthResults.ReverseDNS != nil {
    rdns := email.AuthResults.ReverseDNS
    fmt.Printf("Status: %s\n", rdns.Status)
    fmt.Printf("IP: %s\n", rdns.IP)
    fmt.Printf("Hostname: %s\n", rdns.Hostname)
    fmt.Printf("Info: %s\n", rdns.Info)
}
```

### Reverse DNS Status Values

| Status | Meaning              |
| ------ | -------------------- |
| `pass` | Reverse DNS verified |
| `fail` | Reverse DNS failed   |
| `none` | No PTR record        |

### Reverse DNS Example

```go
email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    t.Fatal(err)
}

if email.AuthResults.ReverseDNS != nil {
    rdns := email.AuthResults.ReverseDNS

    fmt.Printf("Reverse DNS: %s -> %s\n", rdns.IP, rdns.Hostname)
    fmt.Printf("Status: %s\n", rdns.Status)
}
```

## Validation Methods

### Validate Method

The `Validate()` method provides a summary of all authentication checks:

```go
validation := email.AuthResults.Validate()

fmt.Printf("Passed: %v\n", validation.Passed)
fmt.Printf("SPF Passed: %v\n", validation.SPFPassed)
fmt.Printf("DKIM Passed: %v\n", validation.DKIMPassed)
fmt.Printf("DMARC Passed: %v\n", validation.DMARCPassed)
fmt.Printf("ReverseDNS Passed: %v\n", validation.ReverseDNSPassed)
fmt.Printf("Failures: %v\n", validation.Failures)
```

### AuthValidation Struct

```go
type AuthValidation struct {
    Passed           bool     // True if SPF, DKIM, and DMARC all passed
    SPFPassed        bool     // SPF passed
    DKIMPassed       bool     // At least one DKIM passed
    DMARCPassed      bool     // DMARC passed
    ReverseDNSPassed bool     // Reverse DNS passed
    Failures         []string // Slice of failure descriptions
}
```

### Validation Examples

**All checks pass**:

```go
validation := email.AuthResults.Validate()

// AuthValidation{
//     Passed:           true,
//     SPFPassed:        true,
//     DKIMPassed:       true,
//     DMARCPassed:      true,
//     ReverseDNSPassed: true,
//     Failures:         [],
// }

if !validation.Passed {
    t.Error("expected all authentication checks to pass")
}
```

**Some checks fail**:

```go
validation := email.AuthResults.Validate()

// AuthValidation{
//     Passed:           false,
//     SPFPassed:        false,
//     DKIMPassed:       true,
//     DMARCPassed:      false,
//     ReverseDNSPassed: true,
//     Failures: []string{
//         "SPF check failed: fail (domain: example.com)",
//         "DMARC policy: fail (policy: reject)",
//     },
// }

if !validation.Passed {
    fmt.Println("Authentication failures:")
    for _, failure := range validation.Failures {
        fmt.Printf("  - %s\n", failure)
    }
}
```

### IsPassing Method

Quick check if all primary authentication checks passed:

```go
if email.AuthResults.IsPassing() {
    fmt.Println("All authentication checks passed")
} else {
    fmt.Println("Some authentication checks failed")
}
```

**Note**: `IsPassing()` is equivalent to `Validate().Passed` and checks SPF, DKIM, and DMARC. Reverse DNS is not included in this check.

## Package-Level Validation Functions

The `authresults` package also provides functions that return errors for validation:

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

### Sentinel Errors

```go
var (
    ErrSPFFailed        = errors.New("SPF check failed")
    ErrDKIMFailed       = errors.New("DKIM check failed")
    ErrDMARCFailed      = errors.New("DMARC check failed")
    ErrReverseDNSFailed = errors.New("reverse DNS check failed")
    ErrNoAuthResults    = errors.New("no authentication results available")
)
```

Use `errors.Is` to check for specific failures with individual validation functions:

```go
if err := authresults.ValidateSPF(email.AuthResults); err != nil {
    if errors.Is(err, authresults.ErrSPFFailed) {
        fmt.Println("SPF check failed - check your SPF record")
    } else if errors.Is(err, authresults.ErrNoAuthResults) {
        fmt.Println("No SPF results available")
    }
}
```

### ValidationError Type

The package-level `Validate()` function returns a `*ValidationError` when validation fails, which contains all individual error messages:

```go
type ValidationError struct {
    Errors []string
}

func (e *ValidationError) Error() string // Returns errors joined with "; "
```

This allows you to access individual failure messages:

```go
if err := authresults.Validate(email.AuthResults); err != nil {
    // Type assert to access individual errors
    if ve, ok := err.(*authresults.ValidationError); ok {
        for _, msg := range ve.Errors {
            fmt.Printf("- %s\n", msg)
        }
    }
}
```

**Note**: The package-level `Validate()` function returns `*ValidationError` or `ErrNoAuthResults`, while the individual functions (`ValidateSPF`, `ValidateDKIM`, etc.) return their respective sentinel errors.

## Testing Patterns

### Strict Authentication

```go
func TestEmailPassesAllAuthenticationChecks(t *testing.T) {
    sendEmail(inbox.EmailAddress())

    email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
    if err != nil {
        t.Fatal(err)
    }

    validation := email.AuthResults.Validate()

    if !validation.Passed {
        t.Errorf("expected all checks to pass, failures: %v", validation.Failures)
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
}
```

### Lenient Authentication

```go
func TestEmailHasValidDKIMSignature(t *testing.T) {
    sendEmail(inbox.EmailAddress())

    email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
    if err != nil {
        t.Fatal(err)
    }

    // Only check DKIM (most reliable)
    if len(email.AuthResults.DKIM) == 0 {
        t.Fatal("expected DKIM results")
    }
    if email.AuthResults.DKIM[0].Status != "pass" {
        t.Errorf("expected DKIM pass, got %s", email.AuthResults.DKIM[0].Status)
    }
}
```

### Handling Missing Authentication

```go
func TestHandlesEmailsWithoutAuthentication(t *testing.T) {
    email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
    if err != nil {
        t.Fatal(err)
    }

    // Some senders don't have SPF/DKIM configured
    validation := email.AuthResults.Validate()

    // Log results for debugging
    if !validation.Passed {
        t.Logf("Auth failures (expected for test emails): %v", validation.Failures)
    }
}
```

### Testing Specific Checks

```go
func TestEmailAuthentication(t *testing.T) {
    ctx := context.Background()
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }
    defer inbox.Delete(ctx)

    sendEmail(inbox.EmailAddress())

    email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
    if err != nil {
        t.Fatal(err)
    }

    t.Run("SPF check", func(t *testing.T) {
        if email.AuthResults.SPF != nil {
            status := email.AuthResults.SPF.Status
            if status != "pass" && status != "neutral" && status != "softfail" {
                t.Errorf("unexpected SPF status: %s", status)
            }
        }
    })

    t.Run("DKIM check", func(t *testing.T) {
        if len(email.AuthResults.DKIM) > 0 {
            anyPassed := false
            for _, d := range email.AuthResults.DKIM {
                if d.Status == "pass" {
                    anyPassed = true
                    break
                }
            }
            if !anyPassed {
                t.Error("expected at least one DKIM signature to pass")
            }
        }
    })

    t.Run("DMARC check", func(t *testing.T) {
        if email.AuthResults.DMARC != nil {
            status := email.AuthResults.DMARC.Status
            if status != "pass" && status != "none" {
                t.Errorf("unexpected DMARC status: %s", status)
            }
        }
    })
}
```

## Why Authentication Matters

### Production Readiness

Testing authentication catches issues like:

- **Misconfigured SPF records** - emails rejected by Gmail/Outlook
- **Missing DKIM signatures** - reduced deliverability
- **DMARC failures** - emails sent to spam
- **Reverse DNS mismatches** - flagged as suspicious

### Real-World Example

```go
func TestProductionEmailConfiguration(t *testing.T) {
    app.SendWelcomeEmail(inbox.EmailAddress())

    email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
    if err != nil {
        t.Fatal(err)
    }

    validation := email.AuthResults.Validate()

    // In production, these should all pass
    if !validation.Passed {
        t.Log("Email authentication issues detected:")
        for _, f := range validation.Failures {
            t.Logf("  %s", f)
        }
        t.Log("")
        t.Log("Action required:")

        if !validation.SPFPassed {
            t.Log("- Fix SPF record for your domain")
        }
        if !validation.DKIMPassed {
            t.Log("- Configure DKIM signing in your email service")
        }
        if !validation.DMARCPassed {
            t.Log("- Add/fix DMARC policy")
        }

        t.Fail()
    }
}
```

## Troubleshooting

### No Authentication Results

```go
if email.AuthResults.SPF == nil &&
   len(email.AuthResults.DKIM) == 0 &&
   email.AuthResults.DMARC == nil {
    fmt.Println("No authentication performed")
    fmt.Println("This may happen for:")
    fmt.Println("- Emails sent from localhost/internal servers")
    fmt.Println("- Test SMTP servers without authentication")
}
```

### All Checks Fail

```go
validation := email.AuthResults.Validate()

if !validation.Passed {
    fmt.Printf("Authentication failed: %v\n", validation.Failures)

    // Common causes:
    // 1. No SPF record: Add "v=spf1 ip4:YOUR_IP -all" to DNS
    // 2. No DKIM: Configure your mail server to sign emails
    // 3. No DMARC: Add "v=DMARC1; p=none" to DNS
    // 4. Wrong IP: Update SPF record with correct server IP
}
```

### Understanding Failure Reasons

```go
validation := email.AuthResults.Validate()

for _, failure := range validation.Failures {
    switch {
    case strings.Contains(failure, "SPF"):
        fmt.Println("Fix SPF: Update DNS TXT record for your domain")
    case strings.Contains(failure, "DKIM"):
        fmt.Println("Fix DKIM: Enable DKIM signing in your email service")
    case strings.Contains(failure, "DMARC"):
        fmt.Println("Fix DMARC: Add DMARC policy to DNS")
    }
}
```

## Next Steps

- **[Email Authentication Guide](/client-go/guides/authentication/)** - Testing authentication in depth
- **[Email API Reference](/client-go/api/email/)** - Understanding email structure
- **[Testing Patterns](/client-go/testing/password-reset/)** - Real-world testing examples
- **[Gateway Security](/gateway/security/)** - Understanding the security model
