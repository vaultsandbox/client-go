# Refactoring Plan: Reduce Over-Engineering

This plan addresses unnecessary complexity identified in the codebase.

## Priority 1: Remove Dead Code

### 1.1 Delete RetryConfig (internal/api/retry.go)

**Problem**: Full retry system (82 lines) with 254 lines of tests, but never used in production. Actual retry logic is reimplemented in `client.go:217-289`.

**Files to modify**:
- Delete `internal/api/retry.go`
- Delete `internal/api/retry_test.go`

**Risk**: None - code is not called anywhere.

---

## Priority 2: Consolidate Interfaces

### 2.1 Merge Strategy and FullStrategy interfaces

**Problem**: Two interfaces where one suffices. All implementations implement both. `Close()` is identical to `Stop()` in all implementations. `OnReconnect` is mostly a no-op.

**Current state** (`internal/delivery/strategy.go`):
```go
type Strategy interface {
    Start(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error
    Stop() error
    AddInbox(inbox InboxInfo) error
    RemoveInbox(inboxHash string) error
    Name() string
}

type FullStrategy interface {
    Strategy
    Close() error           // Always delegates to Stop()
    OnReconnect(fn func(ctx context.Context))  // No-op in polling
}
```

**Target state**:
```go
type Strategy interface {
    Start(ctx context.Context, inboxes []InboxInfo, handler EventHandler) error
    Stop() error
    AddInbox(inbox InboxInfo) error
    RemoveInbox(inboxHash string) error
    Name() string
    OnReconnect(fn func(ctx context.Context))
}
```

**Files to modify**:
- `internal/delivery/strategy.go` - merge interfaces, remove `FullStrategy`
- `internal/delivery/polling.go` - remove `Close()` method
- `internal/delivery/sse.go` - remove `Close()` method
- `internal/delivery/auto.go` - remove `Close()` method, simplify type assertions
- `internal/delivery/strategy_test.go` - remove `FullStrategy` checks
- `client.go` - change `FullStrategy` references to `Strategy`

**Risk**: Low - internal change, no public API impact.

---

## Priority 3: Remove Wrapper Functions

### 3.1 Remove SSE WaitForEmail wrappers

**Problem**: 4 SSE-prefixed functions that just delegate to the generic versions (`internal/delivery/sse.go:349-375`):
```go
func SSEWaitForEmail[T any](...) { return WaitForEmail(...) }
func SSEWaitForEmailWithSync[T any](...) { return WaitForEmailWithSync(...) }
func SSEWaitForEmailCount[T any](...) { return WaitForEmailCount(...) }
func SSEWaitForEmailCountWithSync[T any](...) { return WaitForEmailCountWithSync(...) }
```

**Note**: The `SSEStrategy` itself (lines 47-347) is legitimate - it uses real SSE via `OpenEventStream()`. Only these wrapper functions are redundant because `WaitForEmail` is strategy-agnostic.

**Action**: Delete these wrapper functions and update any callers to use `WaitForEmail` directly.

**Files to modify**:
- `internal/delivery/sse.go` - delete lines 349-387
- `internal/delivery/sse_test.go` - update tests to use base functions

**Risk**: Low - functions are just pass-through.

### 3.2 Remove redundant WaitForEmail wrappers

**Problem**: 4 wrapper functions in `internal/delivery/polling.go:256-270` that just delegate:
```go
func WaitForEmail[T any](...) { return WaitForEmailWithSync(...) }
func WaitForEmailCount[T any](...) { return WaitForEmailCountWithSync(...) }
```

**Action**: Keep only the `WithSync` versions, rename them to remove the suffix.

**Files to modify**:
- `internal/delivery/polling.go` - consolidate functions
- Update all callers

**Risk**: Medium - need to update callers carefully.

---

## Priority 4: Consolidate Error Handling

### 4.1 Unify error hierarchies

**Problem**: Nearly identical error types in two packages:
- `errors.go` - public errors
- `internal/api/errors.go` - internal errors

Plus `wrapError()` called 37+ times to convert between them.

**Options**:

**Option A (Recommended)**: Keep public errors, make internal code return public errors directly.
- Delete `internal/api/errors.go`
- Update `internal/api/client.go` to return public error types
- Remove all `wrapError()` calls

**Option B**: Keep separation but reduce duplication with embedding.

**Files to modify**:
- `internal/api/errors.go` - delete or refactor
- `internal/api/client.go` - return public errors
- `errors.go` - keep as-is
- All files calling `wrapError()` - remove wrapper calls

**Risk**: Medium - errors are part of public API, need careful testing.

---

## Priority 5: Simplify Configuration

### 5.1 Make polling constants configurable

**Problem**: Hardcoded constants that users cannot override:
```go
// internal/delivery/polling.go
const (
    PollingInitialInterval   = 2 * time.Second
    PollingMaxBackoff        = 30 * time.Second
    PollingBackoffMultiplier = 1.5
    PollingJitterFactor      = 0.3
)
```

**Action**: Add these to the `Config` struct with defaults.

**Files to modify**:
- `internal/delivery/strategy.go` - expand `Config` struct
- `internal/delivery/polling.go` - use config values instead of constants
- `options.go` - add client options to set these values

**Risk**: Low - additive change, backwards compatible.

### 5.2 Replace AutoStrategy with config flag

**Problem**: `AutoStrategy` (`internal/delivery/auto.go`) is a wrapper that adds indirection for simple SSE→polling fallback.

**Action**: Add a `DeliveryMode` option to client config:
```go
type DeliveryMode int
const (
    DeliveryModeAuto DeliveryMode = iota  // Try SSE, fall back to polling
    DeliveryModeSSE
    DeliveryModePolling
)
```

Then handle the fallback logic in client initialization, not a wrapper strategy.

**Files to modify**:
- `options.go` - add `DeliveryMode` type and option
- `client.go` - handle mode selection in `createDeliveryStrategy()`
- `internal/delivery/auto.go` - delete file
- `internal/delivery/auto_test.go` - delete or convert to integration test

**Risk**: Medium - changes initialization logic.

---

## Priority 6: Flatten Indirection (Optional)

### 6.1 Simplify email decryption pipeline

**Problem**: 7 functions for decrypt flow in `inbox_crypto.go`:
1. `decryptEmail()`
2. `verifyAndDecrypt()`
3. `parseMetadata()`
4. `buildDecryptedEmail()`
5. `applyParsedContent()`
6. `convertDecryptedEmail()`
7. crypto package calls

**Action**: Consolidate into 2-3 functions with clear responsibilities:
1. `decryptAndVerify()` - crypto operations
2. `parseDecryptedEmail()` - JSON parsing and type conversion

**Risk**: Medium - core functionality, needs thorough testing.

---

## Execution Order

1. **Phase 1** (Safe deletions):
   - 1.1 Delete RetryConfig
   - 3.1 Remove SSE delegating functions

2. **Phase 2** (Interface cleanup):
   - 2.1 Merge Strategy interfaces
   - 3.2 Consolidate WaitForEmail functions

3. **Phase 3** (Error handling):
   - 4.1 Unify error hierarchies

4. **Phase 4** (Configuration):
   - 5.1 Make constants configurable
   - 5.2 Replace AutoStrategy

5. **Phase 5** (Optional):
   - 6.1 Flatten decryption pipeline

---

## Testing Strategy

After each phase:
1. Run `go build ./...`
2. Run `go test ./...`
3. Run integration tests if available
4. Verify no public API changes (unless intentional)

---

## Not Changing

The following were initially flagged but are justified:

- **Generic type parameters** (`WaitForEmail[T any]`) - Used in tests with `testEmail` struct for simpler test fixtures. Provides real value.
