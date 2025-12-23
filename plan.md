# Test Coverage Improvement Plan (Integration-Focused)

**Current Coverage:** ~53%
**Target Coverage:** 80%
**Strategy:** Expand integration tests using existing SMTP infrastructure

---

## Current State (Better Than Expected)

### Existing Infrastructure
- **SMTP helpers already exist** in `integration/readme_examples_test.go`:
  - `sendTestEmail(t, to, subject, body)`
  - `sendTestHTMLEmail(t, to, subject, textBody, htmlBody)`
  - `sendTestEmailWithAttachment(t, to, subject, body, filename, content)`
  - `getSMTPConfig()` / `skipIfNoSMTP(t)`

### Existing Automated Tests
`readme_examples_test.go` already has **25+ automated tests** covering:
- Email delivery and decryption (QuickStart, PasswordReset, etc.)
- Filters: Subject, From, Predicate
- HTML emails, attachments, links
- Real-time: `Watch`, `WatchInboxes`
- Multiple emails: `WaitForEmailCount`
- Export/Import
- All delivery strategies (Auto, SSE, Polling)
- Error handling
- Client/Inbox options

### The `MANUAL_TEST` Tests Are Redundant
The tests in `integration_test.go` marked with `MANUAL_TEST=1` duplicate functionality already covered by automated tests in `readme_examples_test.go`.

---

## Why Is Coverage Still ~53%?

Possible reasons:
1. **Tests aren't running** - integration tests require `-tags=integration`
2. **SMTP not configured** - tests skip without `SMTP_HOST`
3. **Missing edge cases** - some code paths not exercised
4. **Internal packages** - may need more direct testing

Let's verify current coverage with integration tests:

```bash
# Run ALL tests including integration with coverage
VAULTSANDBOX_API_KEY="..." \
VAULTSANDBOX_URL="..." \
SMTP_HOST="..." \
SMTP_PORT=25 \
go test -tags=integration -coverprofile=coverage.out -covermode=atomic ./...

# Check actual coverage
go tool cover -func=coverage.out | grep total
```

---

## Phase 1: Verify and Measure (Do First)

### 1.1 Run Integration Tests with Coverage
```bash
# From project root with .env loaded
source .env 2>/dev/null || true
go test -tags=integration -v -coverprofile=coverage-full.out -covermode=atomic ./...
go tool cover -func=coverage-full.out | grep total
```

### 1.2 Identify Actual Gaps
```bash
# Generate HTML report
go tool cover -html=coverage-full.out -o coverage.html

# Find uncovered functions
go tool cover -func=coverage-full.out | grep -E "^\S+.*0\.0%"
```

---

## Phase 2: Fill Coverage Gaps

Based on the coverage analysis, add tests for uncovered code. Likely gaps:

### 2.1 Options Not Tested
```go
// Add to readme_examples_test.go or integration_test.go

func TestIntegration_PollingConfigOptions(t *testing.T) {
    client, err := vaultsandbox.New(apiKey,
        vaultsandbox.WithBaseURL(baseURL),
        vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
        vaultsandbox.WithPollingInitialInterval(1*time.Second),
        vaultsandbox.WithPollingMaxBackoff(10*time.Second),
        vaultsandbox.WithPollingBackoffMultiplier(1.5),
        vaultsandbox.WithPollingJitterFactor(0.1),
    )
    if err != nil {
        t.Fatal(err)
    }
    defer client.Close()

    // Create inbox and verify it works
    ctx := context.Background()
    inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
    if err != nil {
        t.Fatal(err)
    }
    defer inbox.Delete(ctx)

    t.Logf("Created inbox with custom polling config: %s", inbox.EmailAddress())
}

func TestIntegration_SSEConfigOptions(t *testing.T) {
    client, err := vaultsandbox.New(apiKey,
        vaultsandbox.WithBaseURL(baseURL),
        vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),
        vaultsandbox.WithSSEConnectionTimeout(45*time.Second),
    )
    if err != nil {
        t.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()
    inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
    if err != nil {
        t.Fatal(err)
    }
    defer inbox.Delete(ctx)

    t.Logf("Created inbox with SSE config: %s", inbox.EmailAddress())
}
```

### 2.2 Error Paths
```go
func TestIntegration_RetryBehavior(t *testing.T) {
    // Test with retry configuration
    client, err := vaultsandbox.New(apiKey,
        vaultsandbox.WithBaseURL(baseURL),
        vaultsandbox.WithRetries(5),
        vaultsandbox.WithRetryOn([]int{408, 429, 500, 502, 503, 504}),
    )
    if err != nil {
        t.Fatal(err)
    }
    defer client.Close()

    // Operations that might trigger retries
    ctx := context.Background()
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }
    defer inbox.Delete(ctx)
}
```

### 2.3 Sync Operations
```go
func TestIntegration_ManualSync(t *testing.T) {
    skipIfNoSMTP(t)

    client := newClient(t)
    ctx := context.Background()

    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        t.Fatal(err)
    }
    defer inbox.Delete(ctx)

    // Send email
    sendTestEmail(t, inbox.EmailAddress(), "Sync Test", "Testing sync")

    // Wait a bit then sync
    time.Sleep(2 * time.Second)

    // Get emails (triggers sync internally)
    emails, err := inbox.GetEmails(ctx)
    if err != nil {
        t.Fatal(err)
    }

    if len(emails) == 0 {
        t.Error("expected at least 1 email after sync")
    }
}
```

### 2.4 Concurrent Operations
```go
func TestIntegration_ConcurrentInboxCreation(t *testing.T) {
    client := newClient(t)
    ctx := context.Background()

    const numInboxes = 5
    var wg sync.WaitGroup
    inboxes := make(chan *vaultsandbox.Inbox, numInboxes)
    errors := make(chan error, numInboxes)

    for i := 0; i < numInboxes; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
            if err != nil {
                errors <- err
                return
            }
            inboxes <- inbox
        }()
    }

    wg.Wait()
    close(inboxes)
    close(errors)

    // Check for errors
    for err := range errors {
        t.Errorf("concurrent CreateInbox error: %v", err)
    }

    // Cleanup
    for inbox := range inboxes {
        inbox.Delete(ctx)
    }

    t.Logf("Successfully created %d inboxes concurrently", numInboxes)
}
```

---

## Phase 3: Remove Redundant Manual Tests

The `MANUAL_TEST` guarded tests in `integration_test.go` can be removed or converted since `readme_examples_test.go` covers them automatically:

| Manual Test | Covered By |
|-------------|------------|
| `WaitForEmail_Receive` | `TestREADME_QuickStart` |
| `SSEDelivery` | `TestREADME_DeliveryStrategy_SSE` |
| `PollingDelivery` | `TestREADME_DeliveryStrategy_Polling` |
| `EmailOperations` | `TestREADME_EmailMethods` |
| `WaitForEmail_WithFilters` | `TestREADME_WaitForEmail_SubjectFilter` |
| `WaitForEmail_WithPredicate` | `TestREADME_WaitOptionPredicate` |
| `WaitForEmailCount` | `TestREADME_WaitForEmailCount` |
| `AuthResults` | `TestREADME_EmailAuthentication` |

---

## Implementation Checklist

### Immediate Actions
- [ ] Run integration tests with coverage and measure actual baseline
- [ ] Generate coverage report to identify real gaps
- [ ] Verify SMTP is working: `go test -tags=integration -run=QuickStart -v ./integration/...`

### Add Missing Tests
- [ ] Polling config options (`WithPollingInitialInterval`, etc.)
- [ ] SSE config options (`WithSSEConnectionTimeout`)
- [ ] Retry configuration tests
- [ ] Concurrent operations
- [ ] Any gaps identified in coverage report

### Cleanup
- [ ] Remove or merge redundant `MANUAL_TEST` tests
- [ ] Consolidate test helpers if needed

---

## Commands

```bash
# Quick smoke test (no SMTP needed)
go test -tags=integration -run=ServerInfo -v ./integration/...

# Test with SMTP (full coverage)
go test -tags=integration -v ./integration/...

# Full coverage measurement
go test -tags=integration -coverprofile=coverage.out -covermode=atomic ./...
go tool cover -func=coverage.out | tail -1

# Coverage report
go tool cover -html=coverage.out -o coverage.html
```

---

## Expected Outcome

After running integration tests with SMTP configured, coverage should already be significantly higher than 53%. The remaining work is:

1. **Verify** - Run tests and measure actual coverage
2. **Fill gaps** - Add tests for uncovered options/paths
3. **Clean up** - Remove redundant manual tests

If integration tests are already covering the code but not being counted, ensure:
- Tests run with `-tags=integration`
- SMTP environment variables are set
- Coverage includes all packages: `./...`
