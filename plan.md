# Phased Refactoring Plan

## Validation Summary

After reviewing the code, here's what I found:

| Issue | Status | Severity | Risk |
|-------|--------|----------|------|
| Code duplication in `inbox.go` | **Confirmed** | Medium | Low |
| Complex `decryptEmailWithContext()` | **Confirmed** | Medium | Medium |
| Complex `New()` in `client.go` | **Partially confirmed** | Low | Low |
| Feature envy in `Email` methods | **Confirmed** | Medium | High (API breaking) |
| Inconsistent error handling | **Confirmed** | Low | Low |

## Guiding Principles

1. **Each phase is independently shippable** - The codebase works after each phase
2. **Tests first** - Write/update tests before refactoring
3. **No API breaks until Phase 4** - Internal changes only in early phases
4. **Run full test suite after each change** - `go test ./...`

---

## Phase 1: Extract Duplicate Helpers (Low Risk)

**Goal:** Remove duplicate `fetcher` and `matcher` functions in `inbox.go`

### Current State

`WaitForEmail` (lines 138-157) and `WaitForEmailCount` (lines 186-205) both define identical:

```go
fetcher := func(ctx context.Context) ([]interface{}, error) {
    emails, err := i.GetEmails(ctx)
    if err != nil {
        return nil, err
    }
    result := make([]interface{}, len(emails))
    for j, e := range emails {
        result[j] = e
    }
    return result, nil
}

matcher := func(email interface{}) bool {
    e, ok := email.(*Email)
    if !ok {
        return false
    }
    return cfg.Matches(e)
}
```

### Refactoring

Extract to private methods on `Inbox`:

```go
func (i *Inbox) emailFetcher() func(ctx context.Context) ([]interface{}, error) {
    return func(ctx context.Context) ([]interface{}, error) {
        emails, err := i.GetEmails(ctx)
        if err != nil {
            return nil, err
        }
        result := make([]interface{}, len(emails))
        for j, e := range emails {
            result[j] = e
        }
        return result, nil
    }
}

func emailMatcher(cfg *waitConfig) func(interface{}) bool {
    return func(email interface{}) bool {
        e, ok := email.(*Email)
        if !ok {
            return false
        }
        return cfg.Matches(e)
    }
}
```

### Tests to Add

```go
// inbox_helpers_test.go
func TestEmailFetcher_ReturnsEmails(t *testing.T)
func TestEmailFetcher_PropagatesErrors(t *testing.T)
func TestEmailMatcher_MatchesCorrectly(t *testing.T)
func TestEmailMatcher_HandlesNonEmailType(t *testing.T)
```

### Verification

```bash
go test ./... -v
go test -race ./...
```

---

## Phase 2: Simplify `decryptEmailWithContext` (Medium Risk)

**Goal:** Break down the 90-line function into focused helpers

### Current Responsibilities (inbox.go:333-422)

1. Validate input has encrypted metadata
2. Fetch full email if needed
3. Verify signature on metadata
4. Decrypt metadata
5. Parse metadata JSON
6. Build DecryptedEmail struct
7. Parse receivedAt timestamp
8. Verify signature on parsed content
9. Decrypt parsed content
10. Parse parsed JSON
11. Convert headers
12. Convert to Email struct

### Proposed Extraction

```go
// verifyAndDecrypt handles signature verification and decryption
func (i *Inbox) verifyAndDecrypt(data []byte) ([]byte, error)

// parseMetadata unmarshals and validates decrypted metadata
func parseMetadata(plaintext []byte) (*crypto.DecryptedMetadata, error)

// parseParsedContent unmarshals decrypted parsed content
func parseParsedContent(plaintext []byte) (*crypto.DecryptedParsed, error)

// buildDecryptedEmail constructs DecryptedEmail from metadata
func buildDecryptedEmail(emailData *api.RawEmail, metadata *crypto.DecryptedMetadata) *crypto.DecryptedEmail
```

### Tests to Add

```go
// inbox_decrypt_test.go
func TestVerifyAndDecrypt_ValidSignature(t *testing.T)
func TestVerifyAndDecrypt_InvalidSignature(t *testing.T)
func TestVerifyAndDecrypt_KeyMismatch(t *testing.T)
func TestParseMetadata_Valid(t *testing.T)
func TestParseMetadata_InvalidJSON(t *testing.T)
func TestParseParsedContent_WithHeaders(t *testing.T)
func TestParseParsedContent_NonStringHeaders(t *testing.T)
func TestBuildDecryptedEmail_ReceivedAtFallback(t *testing.T)
```

### Verification

```bash
go test ./... -v
go test -race ./...
go test -cover ./...  # Ensure coverage doesn't drop
```

---

## Phase 3: Standardize Error Handling (Low Risk)

**Goal:** Consistent error wrapping throughout the codebase

### Current Issues

Mixed usage of:
- `wrapError(err)` - converts internal API errors to public errors
- `fmt.Errorf("message: %w", err)` - standard Go wrapping
- Direct returns - `return nil, err`

### Rules to Apply

1. **API boundary errors**: Use `wrapError()` when calling `apiClient` methods
2. **Internal errors**: Use `fmt.Errorf` with `%w` for context
3. **Crypto errors**: Use `wrapCryptoError()` for signature/decryption failures
4. **Never lose context**: Always wrap, never return bare errors from internal calls

### Changes

```go
// Before (inbox.go:343)
return nil, fmt.Errorf("failed to fetch full email: %w", err)

// After
return nil, fmt.Errorf("fetch full email: %w", wrapError(err))
```

### Tests to Add

```go
// errors_wrapping_test.go
func TestWrapError_PreservesAPIError(t *testing.T)
func TestWrapError_PreservesNetworkError(t *testing.T)
func TestWrapError_PassesThroughOther(t *testing.T)
func TestErrorChain_CanUnwrapToSentinel(t *testing.T)
```

### Verification

```bash
go test ./... -v
# Verify errors.Is() still works for all sentinel errors
go test -run TestError ./...
```

---

## Phase 4: Resolve Feature Envy (High Risk - API Breaking)

**Goal:** Move email operations from `Email` to `Inbox`

### Current State (email.go)

```go
func (e *Email) GetRaw(ctx context.Context) (string, error) {
    return e.inbox.client.apiClient.GetEmailRaw(ctx, e.inbox.emailAddress, e.ID)
}

func (e *Email) MarkAsRead(ctx context.Context) error {
    if err := e.inbox.client.apiClient.MarkEmailAsRead(ctx, e.inbox.emailAddress, e.ID); err != nil {
        return err
    }
    e.IsRead = true
    return nil
}

func (e *Email) Delete(ctx context.Context) error {
    return e.inbox.client.apiClient.DeleteEmail(ctx, e.inbox.emailAddress, e.ID)
}
```

Problems:
- `Email` reaches through 3 levels: `email.inbox.client.apiClient`
- `Email` must hold reference to `Inbox` (hidden coupling)
- Makes `Email` harder to test in isolation

### Proposed Changes

**Option A: Move methods to Inbox (Recommended)**

```go
// inbox.go - new methods
func (i *Inbox) GetEmailRaw(ctx context.Context, emailID string) (string, error)
func (i *Inbox) MarkEmailAsRead(ctx context.Context, emailID string) error
func (i *Inbox) DeleteEmail(ctx context.Context, emailID string) error

// email.go - deprecate old methods
// Deprecated: Use Inbox.GetEmailRaw instead
func (e *Email) GetRaw(ctx context.Context) (string, error)
```

**Option B: Keep Email methods but simplify (Less disruptive)**

```go
// email.go - Email holds inbox reference for convenience methods
// but can also work standalone without it
func (e *Email) GetRaw(ctx context.Context) (string, error) {
    if e.inbox == nil {
        return "", errors.New("email not associated with inbox")
    }
    return e.inbox.GetEmailRaw(ctx, e.ID)
}
```

### Migration Path

1. Add new `Inbox` methods
2. Mark `Email` methods as deprecated
3. Update examples and documentation
4. Remove deprecated methods in next major version

### Tests to Add

```go
// inbox_email_ops_test.go
func TestInbox_GetEmailRaw(t *testing.T)
func TestInbox_MarkEmailAsRead(t *testing.T)
func TestInbox_DeleteEmail(t *testing.T)
func TestInbox_DeleteEmail_NotFound(t *testing.T)

// Deprecation tests
func TestEmail_GetRaw_Deprecated(t *testing.T)
func TestEmail_StillWorksWithoutInbox(t *testing.T)  // if Option B
```

### Verification

```bash
go test ./... -v
go test ./integration/... -v  # Critical: run integration tests
```

---

## Phase 5: Simplify `New()` Constructor (Optional)

**Goal:** Make `New()` in `client.go` more readable

### Assessment

The `New()` function is ~85 lines but is relatively linear:
1. Validate API key
2. Apply options
3. Build API client
4. Validate key with server
5. Fetch server info
6. Create delivery strategy
7. Initialize client struct
8. Start strategy

This is acceptable complexity for a constructor. However, we can extract:

```go
func buildAPIClient(apiKey string, cfg *clientConfig) (*api.Client, error)
func createDeliveryStrategy(cfg *clientConfig, apiClient *api.Client) delivery.FullStrategy
```

### Decision

**Skip unless codebase grows.** The current `New()` is readable enough and extraction would scatter related initialization logic.

---

## Execution Order

```
Phase 1 (1-2 hours)  ─┬─► Phase 2 (2-3 hours) ─┬─► Phase 3 (1 hour)
                      │                         │
                      └─────────────────────────┴─► Phase 4 (3-4 hours)
```

- Phases 1-3 can be done in any order (independent)
- Phase 4 depends on phases being stable (it's the riskiest)
- Phase 5 is optional and independent

## Test Commands

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...

# Run integration tests (requires API key)
VS_API_KEY=xxx go test ./integration/...

# Run specific phase tests
go test -run TestEmailFetcher ./...      # Phase 1
go test -run TestVerifyAndDecrypt ./...  # Phase 2
go test -run TestWrapError ./...         # Phase 3
go test -run TestInbox_.*Email ./...     # Phase 4
```

## Rollback Plan

Each phase is a separate commit. If issues arise:

```bash
git revert <commit-hash>
```

Keep commits small and focused on one change at a time.
