# Codebase Cleanup Plan

This plan addresses over-engineering and dead code issues identified in the codebase.

---

## Phase 1: Remove Dead Code (High Priority)

### 1.1 Remove Dead Generic Polling Utilities
**Location:** `internal/delivery/polling.go:293-500`

**Issue:** `WaitForEmail[T any]` and `WaitForEmailCount[T any]` are never called. The public API uses channel-based `Watch()` mechanism instead.

**Action:**
- [ ] Remove `WaitForEmail[T any]` function (line 293)
- [ ] Remove `WaitForEmailCount[T any]` function (line 406)
- [ ] Remove supporting types: `EmailFetcher`, `EmailMatcher`, `WaitOptions`
- [ ] Remove any helper functions only used by these dead functions

**Impact:** ~200 lines of dead code removed

---

### 1.2 Remove Unused Constructor
**Location:** `internal/api/client.go:66`

**Issue:** `NewClient(cfg Config)` is never called - only `New(apiKey, opts...)` is used.

**Action:**
- [ ] Remove `NewClient()` function
- [ ] Remove `Config` struct if only used by `NewClient()`
- [ ] Verify no external packages depend on it

---

### 1.3 Remove Deprecated Constants
**Location:** `internal/delivery/polling.go:12-33`

**Issue:** Deprecated constants (`PollingInitialInterval`, `PollingMaxBackoff`, etc.) are still exported but never used internally.

**Action:**
- [ ] Remove deprecated constants
- [ ] Update any documentation referencing them

---

## Phase 2: Eliminate Redundant Abstractions (Medium Priority)

### 2.1 Consolidate SyncStatus Type
**Locations:**
- `internal/api/types.go:50-57`
- `inbox.go:21-27`
- `internal/delivery/strategy.go:117-127`

**Issue:** Same struct defined 3 times, forcing pointless field-by-field conversions.

**Action:**
- [ ] Keep `api.SyncStatus` as the single source of truth
- [ ] Use type alias in public package: `type SyncStatus = api.SyncStatus`
- [ ] Remove duplicate definition in `internal/delivery/strategy.go`
- [ ] Update `Inbox.GetSyncStatus()` to return `*api.SyncStatus` directly

---

### 2.2 Remove Unnecessary HTTP Wrapper
**Location:** `internal/api/client.go:294-296`

**Issue:** `do()` method just delegates to `Do()` with no added value.

**Action:**
- [ ] Replace all `c.do(...)` calls with `c.Do(...)`
- [ ] Remove `do()` method

---

### 2.3 Inline Trivial Builder Function
**Location:** `client.go:80-89`

**Issue:** `buildDeliveryConfig()` only copies fields - no logic.

**Action:**
- [ ] Inline the field mapping directly in `New()` constructor
- [ ] Remove `buildDeliveryConfig()` function

---

### 2.4 Remove Base64 Alias
**Location:** `internal/crypto/base64.go:31`

**Issue:** `EncodeBase64()` is just an alias to `ToBase64URL()`.

**Action:**
- [ ] Replace all `EncodeBase64()` calls with `ToBase64URL()`
- [ ] Remove `EncodeBase64()` function

---

## Phase 3: Interface Cleanup (Low Priority)

### 3.1 Address OnReconnect No-op
**Location:** `internal/delivery/polling.go:525-528`

**Issue:** `PollingStrategy.OnReconnect()` does nothing - polling doesn't reconnect.

**Options:**
- **Option A:** Remove from interface, use type assertion where needed
- **Option B:** Keep as-is (least disruptive, documents intent)

**Recommendation:** Option B - the no-op is harmless and documents that polling doesn't need reconnection handling.

---

## Phase 4: Configuration Simplification (Optional)

### 4.1 Review Polling Config Options
**Location:** `options.go:113-158`

**Issue:** 6 polling config functions may be excessive for most users.

**Possible Actions:**
- [ ] Document that defaults work for 99% of use cases
- [ ] Consider grouping into a single `WithPollingConfig(PollingConfig)` option
- [ ] Add examples showing when custom config is actually needed

**Note:** This is optional - current design is not wrong, just verbose.

---

## Execution Order

1. **Phase 1** first - removes dead code with no behavioral changes
2. **Phase 2** next - simplifies internals, minimal API impact
3. **Phase 3** as needed - low priority cleanup
4. **Phase 4** if time permits - nice-to-have improvements

---

## Testing Strategy

After each phase:
- [ ] Run `go build ./...` to verify compilation
- [ ] Run `go test ./...` to verify behavior
- [ ] Run integration tests if available

---

## Estimated Impact

| Phase | Lines Removed | Risk Level |
|-------|--------------|------------|
| 1.1   | ~200         | Low        |
| 1.2   | ~50          | Low        |
| 1.3   | ~25          | Low        |
| 2.1   | ~30          | Medium     |
| 2.2   | ~5           | Low        |
| 2.3   | ~15          | Low        |
| 2.4   | ~5           | Low        |

**Total:** ~330 lines of unnecessary code removed
