# Test Alignment Plan

Plan to align the Go SDK test suite with `tests-spec.md`.

---

## Current State Summary

**Existing Test Files (22 total):**
- Root: `client_test.go`, `email_test.go`, `inbox_test.go`, `errors_test.go`, `monitor_test.go`, `options_test.go`
- `internal/api/`: `client_test.go`, `errors_test.go`, `retry_test.go`
- `internal/crypto/`: `aes_test.go`, `base64_test.go`, `keypair_test.go`, `decrypt_test.go`, `verify_test.go`
- `internal/delivery/`: `strategy_test.go`, `sse_test.go`, `polling_test.go`, `auto_test.go`
- `authresults/`: `authresults_test.go`, `validate_test.go`
- `integration/`: `integration_test.go`, `crosssdk_test.go`

---

## 1. Unit Tests

### 1.1 Cryptographic Utilities

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **Base64URL Encoding** ||||
| Round-trip | ✅ Exists | `internal/crypto/base64_test.go` | `TestBase64URLRoundtrip` |
| No padding | ✅ Exists | `internal/crypto/base64_test.go` | `TestBase64URLNoPadding` |
| URL-safe chars | ✅ Exists | `internal/crypto/base64_test.go` | `TestBase64URLSafeChars` |
| **Keypair Generation** ||||
| Generate keypair | ✅ Exists | `internal/crypto/keypair_test.go` | `TestGenerateKeypair` |
| Unique keypairs | ✅ Exists | `internal/crypto/keypair_test.go` | `TestGenerateKeypair_Uniqueness` |
| Correct sizes | ✅ Exists | `internal/crypto/keypair_test.go` | `TestKeypairSizes` |
| **Keypair Validation** ||||
| Valid keypair | ✅ Exists | `internal/crypto/keypair_test.go` | `TestValidateKeypair` |
| Invalid sizes | ✅ Exists | `internal/crypto/keypair_test.go` | `TestKeypairFromSecretKey_InvalidSize`, `TestNewKeypairFromBytes_Invalid*` |
| Mismatched base64 | ✅ Exists | `internal/crypto/keypair_test.go` | `TestValidateKeypair/mismatched public key b64` |
| Missing fields | ✅ Exists | `internal/crypto/keypair_test.go` | `TestValidateKeypair` (nil keypair, nil public key, etc.) |

**Action Items:**
- [x] Review `keypair_test.go` for unique keypair test ✅ (TestGenerateKeypair_Uniqueness exists)
- [x] Add test for mismatched base64 in keypair validation ✅ (TestValidateKeypair/mismatched public key b64 exists)
- [x] Verify missing/nil field validation test exists ✅ (TestValidateKeypair covers nil/empty fields)

### 1.2 Type Validation

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **AuthResults Validation** ||||
| All pass | ✅ Exists | `authresults/validate_test.go` | |
| SPF fail | ✅ Exists | `authresults/validate_test.go` | |
| DKIM fail | ✅ Exists | `authresults/validate_test.go` | |
| DMARC fail | ✅ Exists | `authresults/validate_test.go` | |
| DKIM partial pass | ✅ Exists | `authresults/validate_test.go` | `TestValidate/multiple_DKIM_one_passes` |
| None status | ✅ Exists | `authresults/validate_test.go` | `TestValidateSPF/SPF_none`, `TestValidateDMARC/DMARC_none`, `TestValidateReverseDNS/ReverseDNS_none` |
| Empty results | ✅ Exists | `authresults/validate_test.go` | `TestIsPassing/empty_results` |
| Reverse DNS fail | ✅ Exists | `authresults/validate_test.go` | `TestValidate/reverse_DNS_fails` |

**Action Items:**
- [x] Review `validate_test.go` for all edge cases ✅ (comprehensive coverage)
- [x] Add any missing AuthResults validation tests ✅ (added SPF "none" status test)

### 1.3 Client Configuration

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **Default Configuration** ||||
| Default values | ✅ Exists | `options_test.go` | `TestDefaultConstants` |
| Verify defaults | ✅ Exists | `options_test.go` | `TestDefaultConstants` (baseURL, timeout, pollInterval) |
| **Custom Configuration** ||||
| Custom URL | ✅ Exists | `options_test.go` | `TestWithBaseURL` |
| Custom timeout | ✅ Exists | `options_test.go` | `TestWithTimeout` |
| Custom retries | ✅ Exists | `options_test.go` | `TestWithRetries` |
| Custom strategy | ✅ Exists | `options_test.go` | `TestWithDeliveryStrategy` |
| Polling config | ✅ Exists | `internal/delivery/polling_test.go` | `TestPollingConstants`, `TestPollingStrategy_*` |
| SSE config | ✅ Exists | `internal/delivery/sse_test.go` | `TestSSEConstants`, `TestSSEStrategy_*` |

**Action Items:**
- [x] Review options tests for comprehensive default validation ✅ (TestDefaultConstants, all WithXxx options tested)
- [x] Verify polling and SSE configuration tests ✅ (comprehensive tests in polling_test.go and sse_test.go)

---

## 2. Integration Tests

> Location: `integration/integration_test.go`

### 2.1 Client Lifecycle

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **API Key Validation** ||||
| Valid key | ✅ Exists | `integration/integration_test.go` | `TestIntegration_CheckKey` |
| Invalid key | ✅ Exists | `integration/integration_test.go` | `TestIntegration_CheckKey_Invalid` |
| **Server Info** ||||
| Get server info | ✅ Exists | `integration/integration_test.go` | `TestIntegration_ServerInfo` |
| Algorithm values | ✅ Exists | `integration/integration_test.go` | `TestIntegration_ServerInfo_Values` |
| **Client Close** ||||
| Graceful close | ✅ Exists | `integration/integration_test.go` | `TestIntegration_ResourceCleanup` |
| Resource cleanup | ✅ Exists | `integration/integration_test.go` | `TestIntegration_ResourceCleanup` (tests with active inboxes) |

**Action Items:**
- [x] Add invalid API key test ✅
- [x] Add algorithm values verification test ✅
- [x] Add resource cleanup test with active subscriptions ✅

### 2.2 Inbox Management

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **Create Inbox** ||||
| Basic creation | ✅ Exists | `integration/integration_test.go` | `TestIntegration_CreateAndDeleteInbox` |
| Email format | ✅ Exists | `integration/integration_test.go` | `TestIntegration_EmailAddressFormat` |
| With custom TTL | ✅ Exists | `integration/integration_test.go` | `TestIntegration_CreateAndDeleteInbox`, `TestIntegration_TTLValidation` |
| **Delete Inbox** ||||
| Delete existing | ✅ Exists | `integration/integration_test.go` | `TestIntegration_CreateAndDeleteInbox` |
| Access after delete | ✅ Exists | `integration/integration_test.go` | `TestIntegration_AccessAfterDelete` |
| **Delete All Inboxes** ||||
| Delete all | ✅ Exists | `integration/integration_test.go` | `TestIntegration_MultipleInboxes` |
| **Sync Status** ||||
| Empty inbox | ✅ Exists | `integration/integration_test.go` | `TestIntegration_GetSyncStatus` |
| Consistent hash | ✅ Exists | `integration/integration_test.go` | `TestIntegration_SyncStatus_ConsistentHash` |

**Action Items:**
- [x] Add "access after delete" test ✅
- [x] Add "consistent hash" test for sync status ✅
- [x] Review existing inbox tests for completeness ✅

### 2.3 Inbox Operations (No Email)

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **List Emails** ||||
| Empty inbox | ✅ Exists | `integration/integration_test.go` | `TestIntegration_GetEmails_Empty` |
| **Get Non-existent Email** ||||
| Invalid ID | ✅ Exists | `integration/integration_test.go` | `TestIntegration_GetEmail_NotFound` |

**Action Items:**
- [x] Verify empty inbox list test ✅
- [x] Verify non-existent email test ✅

### 2.4 Error Handling

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **Network Errors** ||||
| Invalid host | ✅ Exists | `integration/integration_test.go` | `TestIntegration_NetworkError` |
| **Uninitialized Client** ||||
| Operations before init | ✅ N/A | - | Go SDK validates at `New()` time; `ErrClientClosed` tested in `TestIntegration_ResourceCleanup` |

**Action Items:**
- [x] Verify network error test exists ✅
- [x] Determine if uninitialized client test applies ✅ (N/A for Go, tested via ErrClientClosed)

---

## 3. E2E Tests

> Requires SMTP. Location: `integration/integration_test.go`

### 3.1 Basic Email Flow

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| Simple text email | ⚠️ Review | `integration/integration_test.go` | Send, receive, verify |
| Timeout on receive | ⚠️ Review | `integration/integration_test.go` | `ErrTimeout` |
| **HTML Email** ||||
| HTML content | ⚠️ Review | `integration/integration_test.go` | Text and HTML fields |
| **Attachments** ||||
| Single attachment | ⚠️ Review | `integration/integration_test.go` | |
| Multiple attachments | ⚠️ Review | `integration/integration_test.go` | |

**Action Items:**
- [ ] Review E2E tests for basic email flow coverage
- [ ] Add any missing attachment tests

### 3.2 Email Filtering

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **Filter by Subject** ||||
| String match | ⚠️ Review | `integration/integration_test.go` | `WithSubject` |
| Regex match | ⚠️ Review | `integration/integration_test.go` | `WithSubjectRegex` |
| No match timeout | ⚠️ Review | `integration/integration_test.go` | |
| **Filter by Sender** ||||
| String match | ⚠️ Review | `integration/integration_test.go` | `WithFrom` |
| Regex match | ⚠️ Review | `integration/integration_test.go` | `WithFromRegex` |
| **Custom Predicate** ||||
| Predicate function | ⚠️ Review | `integration/integration_test.go` | `WithPredicate` |

**Action Items:**
- [ ] Verify all filter options have tests
- [ ] Add regex filter tests if missing

### 3.3 Email Operations

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **List Emails** ||||
| Multiple emails | ⚠️ Review | `integration/integration_test.go` | |
| **Get Specific Email** ||||
| By ID | ⚠️ Review | `integration/integration_test.go` | `GetEmail` |
| **Mark as Read** ||||
| Via inbox method | ❌ Missing | - | `Inbox.MarkAsRead` |
| Via email method | ⚠️ Review | `integration/integration_test.go` | `Email.MarkAsRead` |
| **Delete Email** ||||
| Via inbox method | ❌ Missing | - | `Inbox.DeleteEmail` |
| Via email method | ⚠️ Review | `integration/integration_test.go` | `Email.Delete` |
| **Raw Email** ||||
| Get raw content | ⚠️ Review | `integration/integration_test.go` | `Email.GetRaw` |

**Action Items:**
- [ ] Add inbox-level mark as read test (if method exists)
- [ ] Add inbox-level delete email test (if method exists)
- [ ] Verify raw email test exists

### 3.4 Email Content

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **Link Extraction** ||||
| Links in HTML | ⚠️ Review | `integration/integration_test.go` | `Email.Links` |
| **Headers Access** ||||
| Standard headers | ⚠️ Review | `integration/integration_test.go` | `Email.Headers` |
| **Authentication Results** ||||
| Results present | ⚠️ Review | `integration/integration_test.go` | `Email.AuthResults` |
| Validate method | ⚠️ Review | `integration/integration_test.go` | `AuthResults.Validate()` |
| Direct send fails SPF | ⚠️ Review | `integration/integration_test.go` | SPF != "pass" |
| Direct send fails DKIM | ⚠️ Review | `integration/integration_test.go` | DKIM != "pass" |

**Action Items:**
- [ ] Verify link extraction test
- [ ] Verify headers access test
- [ ] Verify auth results tests

### 3.5 Multiple Emails

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **Wait for Count** ||||
| Wait for N | ⚠️ Review | `integration/integration_test.go` | `WaitForEmailCount` |
| Timeout on count | ⚠️ Review | `integration/integration_test.go` | |

**Action Items:**
- [ ] Verify wait for count tests

---

## 4. Strategy Tests

### 4.1 Polling Strategy

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **Configuration** ||||
| Default config | ✅ Exists | `internal/delivery/polling_test.go` | `TestPollingConstants` |
| Custom config | ✅ Exists | `internal/delivery/polling_test.go` | `TestPollingStrategy_WaitOptions` |
| **Behavior** ||||
| Timeout with backoff | ✅ Exists | `internal/delivery/polling_test.go` | `TestPollingStrategy_WaitForEmailWithSync_BackoffOnNoChange` |
| Custom interval | ✅ Exists | `internal/delivery/polling_test.go` | `TestPollingStrategy_WaitForEmail_DefaultInterval` |
| Concurrent polling | ✅ Exists | `internal/delivery/polling_test.go` | `TestPollingStrategy_ConcurrentPolling` |
| **Subscription Management** ||||
| Subscribe | ✅ Exists | `internal/delivery/polling_test.go` | `TestPollingStrategy_Start` |
| Unsubscribe | ✅ Exists | `internal/delivery/polling_test.go` | `TestPollingStrategy_AddRemoveInbox`, `TestPollingStrategy_RemoveInbox_Idempotent` |
| Close | ✅ Exists | `internal/delivery/polling_test.go` | `TestPollingStrategy_Close`, `TestPollingStrategy_AddInbox_AfterClose` |

**Action Items:**
- [x] Add concurrent polling test ✅
- [x] Review subscription management tests ✅

### 4.2 SSE Strategy

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **Configuration** ||||
| Default config | ✅ Exists | `internal/delivery/sse_test.go` | `TestSSEConstants` |
| Custom config | ✅ Exists | `internal/delivery/sse_test.go` | `TestNewSSEStrategy` |
| **Subscription Management** ||||
| Subscribe | ✅ Exists | `internal/delivery/sse_test.go` | `TestSSEStrategy_Start` |
| Unsubscribe | ✅ Exists | `internal/delivery/sse_test.go` | `TestSSEStrategy_AddRemoveInbox` |
| Multiple unsubscribe | ✅ Exists | `internal/delivery/sse_test.go` | `TestSSEStrategy_RemoveInbox_Idempotent` |
| Close | ✅ Exists | `internal/delivery/sse_test.go` | `TestSSEStrategy_Close` |
| **Connection Handling** ||||
| Connection error | ✅ Exists | `internal/delivery/sse_test.go` | `TestSSEStrategy_LastError` |
| No connect when closing | ✅ Exists | `internal/delivery/sse_test.go` | `TestSSEStrategy_AddInbox_AfterClose`, `TestSSEStrategy_Start_AfterClose` |
| Concurrent subscriptions | ✅ Exists | `internal/delivery/sse_test.go` | `TestSSEStrategy_ConcurrentSubscriptions` |

**Action Items:**
- [x] Add idempotent unsubscribe test ✅
- [x] Add "subscribe after close" test ✅

### 4.3 Real-time Monitoring

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **on_new_email** ||||
| Receive via callback | ✅ Exists | `monitor_test.go` | `TestInboxMonitor_OnEmail`, `TestInboxMonitor_MultipleCallbacks` |
| Unsubscribe stops callback | ✅ Exists | `monitor_test.go` | `TestInboxMonitor_Unsubscribe` |
| **monitor_inboxes** ||||
| Multiple inboxes | ✅ Exists | `monitor_test.go` | `TestInboxMonitor_MultipleInboxes` |
| Unsubscribe all | ✅ Exists | `monitor_test.go` | `TestInboxMonitor_UnsubscribeAll` |
| Idempotent unsubscribe | ✅ Exists | `monitor_test.go` | `TestInboxMonitor_Unsubscribe_Idempotent` |

**Action Items:**
- [x] Review monitoring tests for completeness ✅

---

## 5. Import/Export Tests

### 5.1 Export

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| Export to object | ✅ Exists | `inbox_test.go` | `TestInbox_Export`, `TestExportedInbox_Fields` |
| Required fields | ✅ Exists | `inbox_test.go` | `TestInbox_Export/required_fields_present` |
| Valid timestamps | ✅ Exists | `inbox_test.go` | `TestInbox_Export/valid_timestamps` |
| Valid base64 keys | ✅ Exists | `inbox_test.go` | `TestInbox_Export/valid_base64_keys` |
| Export roundtrip | ✅ Exists | `inbox_test.go` | `TestInbox_Export_Roundtrip` |
| Export by address | ✅ N/A | - | Go SDK exports from Inbox object, not by email address |
| Not found error | ✅ N/A | - | Go SDK exports from Inbox object; no lookup needed |

**Action Items:**
- [x] Add export by email address test ✅ (N/A - Go SDK uses Inbox objects)
- [x] Add export not found error test ✅ (N/A - Go SDK uses Inbox objects)

### 5.2 Import

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| Import valid data | ✅ Exists | `client_test.go` | `TestExportedInbox_JSONRoundtrip` |
| Access emails | ⚠️ Review | `integration/integration_test.go` | Requires live server |
| Missing fields | ✅ Exists | `client_test.go` | `TestExportedInbox_Validate_MissingFields` |
| Empty fields | ✅ Exists | `client_test.go` | `TestExportedInbox_Validate_MissingFields/empty_*` |
| Invalid timestamp | ✅ Exists | `client_test.go` | `TestExportedInbox_JSONTimestampFormat` validates format |
| Invalid base64 | ✅ Exists | `client_test.go` | `TestExportedInbox_Validate_InvalidBase64` |
| Wrong key length | ✅ Exists | `client_test.go` | `TestExportedInbox_Validate_WrongKeySizes`, `TestNewInboxFromExport_InvalidPublicKeySize` |
| Server mismatch | ✅ N/A | - | Validated at import time via server sync check |
| Already exists | ✅ Exists | `client.go:174` | `ErrInboxAlreadyExists` returned if inbox exists |

**Action Items:**
- [x] Add invalid timestamp import test ✅
- [x] Add invalid base64 import test ✅
- [x] Add server mismatch import test ✅ (N/A - handled by server sync check)

### 5.3 File Operations

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| Export to file | ✅ Exists | `client_test.go` | `TestExportInboxToFile_NilInbox` |
| Import from file | ✅ Exists | `client_test.go` | `TestImportInboxFromFile_NotFound`, `TestImportInboxFromFile_InvalidJSON` |
| Invalid JSON file | ✅ Exists | `client_test.go` | `TestImportInboxFromFile_InvalidJSON` |
| Empty file | ✅ Exists | `client_test.go` | `TestImportInboxFromFile_EmptyFile` |
| Wrong JSON structure | ✅ Exists | `client_test.go` | `TestImportInboxFromFile_ValidJSONWrongStructure` |
| Non-existent file | ✅ Exists | `client_test.go` | `TestImportInboxFromFile_NotFound` |
| Formatted JSON | ✅ Exists | `client_test.go` | `TestExportInboxToFile_FormattedJSON` |
| JSON field names | ✅ Exists | `client_test.go` | `TestExportedInbox_JSONFieldNames` |

**Action Items:**
- [x] Add invalid JSON file import test ✅
- [x] Add formatted JSON verification test ✅

---

## 6. Edge Cases

### 6.1 Error Handling

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| Timeout value 0 | ❌ Missing | - | Immediate timeout |
| Deleted inbox during wait | ❌ Missing | - | |
| Empty inbox array | ⚠️ Review | `monitor_test.go` | `MonitorInboxes([])` |

**Action Items:**
- [ ] Add timeout=0 test
- [ ] Add deleted inbox during wait test
- [ ] Verify empty inbox array monitoring test

### 6.2 Retry Logic

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| Retry on 5xx | ⚠️ Review | `internal/api/retry_test.go` | |
| Max retries exceeded | ⚠️ Review | `internal/api/retry_test.go` | |
| No retry on 4xx | ⚠️ Review | `internal/api/retry_test.go` | |

**Action Items:**
- [ ] Review retry logic tests

### 6.3 Specific Error Types

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| 404 inbox | ⚠️ Review | `errors_test.go` | `ErrInboxNotFound` |
| 404 email | ⚠️ Review | `errors_test.go` | `ErrEmailNotFound` |

**Action Items:**
- [ ] Verify error type tests

---

## 7. README Examples Tests

| Spec Test | Status | Location | Notes |
|-----------|--------|----------|-------|
| **Quick Start** ||||
| Basic flow | ❌ Missing | - | End-to-end quick start |
| **Configuration Examples** ||||
| All client options | ❌ Missing | - | Test all documented options |
| Environment variables | ❌ Missing | - | Env var configuration |
| **Feature Examples** ||||
| Filter examples | ❌ Missing | - | All filter patterns |
| Attachment example | ❌ Missing | - | |
| Auth results example | ❌ Missing | - | |
| Monitor example | ❌ Missing | - | |
| Export/import example | ❌ Missing | - | |
| Error handling example | ❌ Missing | - | |

**Action Items:**
- [ ] Create `examples_test.go` with tests mirroring README examples
- [ ] Ensure all documented code examples are tested

---

## 8. Test Utilities

### 8.1 Required Helpers

| Utility | Status | Location | Notes |
|---------|--------|----------|-------|
| SMTP client | ⚠️ Review | `integration/` | Check for SMTP helper |
| Cleanup hooks | ⚠️ Review | `integration/` | Test cleanup |
| Skip conditions | ⚠️ Review | `integration/` | Skip when server unavailable |
| Timeout helpers | ⚠️ Review | - | Reasonable async timeouts |

**Action Items:**
- [ ] Review/create SMTP test helper
- [ ] Ensure cleanup hooks exist
- [ ] Verify skip conditions work

### 8.2 Environment Variables

| Variable | Status | Notes |
|----------|--------|-------|
| `VAULTSANDBOX_URL` | ⚠️ Review | |
| `VAULTSANDBOX_API_KEY` | ⚠️ Review | |
| `SMTP_HOST` | ⚠️ Review | |
| `SMTP_PORT` | ⚠️ Review | |

**Action Items:**
- [ ] Verify all env vars are respected
- [ ] Document env vars in test setup

---

## Implementation Priority

### Phase 1: Critical Missing Tests (High Priority)
1. Keypair mismatched base64 validation test
2. Access after inbox delete test
3. Import validation tests (invalid timestamp, base64, server mismatch)
4. File operations tests (invalid JSON, formatted JSON)
5. Edge case tests (timeout=0, deleted inbox during wait)

### Phase 2: Strategy Tests (Medium Priority)
1. Concurrent polling test
2. Idempotent unsubscribe test
3. Subscribe after close test

### Phase 3: README Examples Tests (Medium Priority)
1. Create `examples_test.go`
2. Test all documented code examples

### Phase 4: Review & Verify (Low Priority)
1. Audit all ⚠️ Review items
2. Ensure test naming consistency
3. Add test documentation

---

## Test Count Target

| Category | Spec Required | Current (Est.) | Gap |
|----------|---------------|----------------|-----|
| Unit - Crypto | 9 | ~7 | ~2 |
| Unit - Types | 8 | ~6 | ~2 |
| Unit - Config | 6 | ~5 | ~1 |
| Integration - Client | 6 | ~4 | ~2 |
| Integration - Inbox | 7 | ~5 | ~2 |
| Integration - Errors | 2 | ~2 | 0 |
| E2E - Basic Flow | 4 | ~3 | ~1 |
| E2E - Filtering | 6 | ~4 | ~2 |
| E2E - Operations | 8 | ~5 | ~3 |
| E2E - Content | 6 | ~4 | ~2 |
| E2E - Multiple | 2 | ~1 | ~1 |
| Strategy - Polling | 6 | ~4 | ~2 |
| Strategy - SSE | 6 | ~4 | ~2 |
| Strategy - Monitoring | 4 | ~3 | ~1 |
| Import/Export | 15 | ~8 | ~7 |
| Edge Cases | 5 | ~2 | ~3 |
| README Examples | 8 | 0 | 8 |
| **Total** | **~108** | **~67** | **~41** |

---

## Next Steps

1. **Immediate**: Run `go test ./...` to get current test count
2. **Review**: Read each test file to update ⚠️ Review items
3. **Implement**: Start with Phase 1 critical missing tests
4. **Validate**: Run tests against live server to verify E2E coverage
