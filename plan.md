# Go SDK Alignment Plan with Node SDK

This document outlines a phased approach to verify and align the Go SDK (`client-go`) with the Node SDK (`client-node`) to ensure feature parity and consistency.

**Reference:** `/home/vs/Desktop/dev/client-node`

---

## Phase 1: Core Cryptography Verification ✅ COMPLETED

Ensure cryptographic operations are identical between both SDKs.

### 1.1 Keypair Generation
- [x] Verify ML-KEM-768 key sizes match (public: 1184B, secret: 2400B)
- [x] Verify `GenerateKeypair()` produces valid keys
- [x] Implement `DerivePublicKeyFromSecret()` - Added to keypair.go
- [x] Add `ValidateKeypair()` function - Added to keypair.go

### 1.2 Signature Verification
- [x] Verify ML-DSA-65 public key size (1952B)
- [x] Verify transcript building matches Node SDK:
  - Version byte (1)
  - Algorithm ciphersuite string (`ML-KEM-768:ML-DSA-65:AES-256-GCM:HKDF-SHA-512`)
  - Context string (`vaultsandbox:email:v1`)
  - KEM ciphertext
  - Nonce
  - AAD
  - Ciphertext
  - Server public key
- [x] Add `VerifySignatureSafe()` non-throwing variant - Added to verify.go
- [x] Add `ValidateServerPublicKey()` function - Added to verify.go

### 1.3 Decryption Pipeline
- [x] Verify HKDF-SHA-512 key derivation matches:
  - Salt = SHA-256(ctKem)
  - Info = context || aad_length (4 bytes BE) || aad
- [x] Verify AES-256-GCM parameters (12-byte nonce, 128-bit tag)
- [x] Ensure signature verification happens BEFORE decryption

### 1.4 Base64 Encoding
- [x] Verify Base64URL encoding/decoding (no padding)
- [x] Add standard Base64 encoding (`ToBase64`/`FromBase64`) for attachment content
- [x] Lenient decoding already exists (`DecodeBase64` tries multiple formats)

**Note:** The `DerivePublicKeyFromSecret` offset differs between Go (1152) and Node (1216) due to library-specific ML-KEM-768 secret key formats. This doesn't affect cross-SDK compatibility because export/import includes both public and secret keys explicitly.

---

## Phase 2: API Client Alignment ✅ COMPLETED

Ensure HTTP client behavior matches Node SDK.

### 2.1 Configuration Options
| Option | Node SDK | Go SDK | Status |
|--------|----------|--------|--------|
| `url` / `baseURL` | Required | `WithBaseURL()` | ✅ |
| `apiKey` | Required | Constructor param | ✅ |
| `strategy` | `'sse'|'polling'|'auto'` | `WithDeliveryStrategy()` | ✅ |
| `pollingInterval` | 2000ms default | `defaultPollInterval` | ✅ |
| `maxRetries` | 3 default | `WithRetries()` | ✅ |
| `retryDelay` | 1000ms default | `DefaultRetryDelay` | ✅ |
| `retryOn` | `[408,429,500,502,503,504]` | `WithRetryOn()` | ✅ |
| `sseReconnectInterval` | 5000ms | `SSEReconnectInterval` | ✅ |
| `sseMaxReconnectAttempts` | 10 | `SSEMaxReconnectAttempts` | ✅ |

### 2.2 API Endpoints
- [x] `GET /api/check-key` - Validate API key
- [x] `GET /api/server-info` - Get server capabilities
- [x] `POST /api/inboxes` - Create inbox
- [x] `DELETE /api/inboxes` - Delete all inboxes
- [x] `DELETE /api/inboxes/{email}` - Delete specific inbox
- [x] `GET /api/inboxes/{email}/sync` - Get sync status
- [x] `GET /api/inboxes/{email}/emails` - List emails
- [x] `GET /api/inboxes/{email}/emails/{id}` - Get email
- [x] `GET /api/inboxes/{email}/emails/{id}/raw` - Get raw email
- [x] `PATCH /api/inboxes/{email}/emails/{id}/read` - Mark as read
- [x] `DELETE /api/inboxes/{email}/emails/{id}` - Delete email
- [x] `GET /api/events?inboxes=...` - SSE stream

### 2.3 Error Handling
- [x] Map HTTP 404 + "inbox" → `ErrInboxNotFound`
- [x] Map HTTP 404 + "email" → `ErrEmailNotFound`
- [x] Map HTTP 401 → `ErrUnauthorized`
- [x] Map HTTP 429 → `ErrRateLimited`
- [x] Include RequestID in error messages

### 2.4 Retry Logic
- [x] Exponential backoff: `delay * 2^retryCount`
- [x] Verify retryable status codes
- [x] Add configurable `retryOn` status codes

---

## Phase 3: Client API Alignment ✅ COMPLETED

Ensure public Client API matches Node SDK.

### 3.1 Client Methods
| Method | Node SDK | Go SDK | Status |
|--------|----------|--------|--------|
| Constructor | `new VaultSandboxClient(config)` | `New(apiKey, ...opts)` | ✅ |
| `createInbox()` | ✅ | `CreateInbox()` | ✅ |
| `deleteAllInboxes()` | ✅ | `DeleteAllInboxes()` | ✅ |
| `exportInbox()` | ✅ | Via `inbox.Export()` | ✅ |
| `importInbox()` | ✅ | `ImportInbox()` | ✅ |
| `exportInboxToFile()` | ✅ | `ExportInboxToFile()` | ✅ |
| `importInboxFromFile()` | ✅ | `ImportInboxFromFile()` | ✅ |
| `monitorInboxes()` | ✅ | `MonitorInboxes()` | ✅ |
| `getServerInfo()` | ✅ | `ServerInfo()` | ✅ |
| `checkKey()` | ✅ | `CheckKey()` | ✅ |
| `close()` | ✅ | `Close()` | ✅ |

### 3.2 Missing Client Features
- [x] Add `CheckKey() error` public method
- [x] Add `ExportInboxToFile(inbox, path) error`
- [x] Add `ImportInboxFromFile(path) (*Inbox, error)`
- [x] Add `MonitorInboxes(inboxes) *InboxMonitor` with EventEmitter pattern

---

## Phase 4: Inbox API Alignment ✅ COMPLETED

Ensure Inbox operations match Node SDK.

### 4.1 Inbox Methods
| Method | Node SDK | Go SDK | Status |
|--------|----------|--------|--------|
| `emailAddress` | Property | `EmailAddress()` | ✅ |
| `expiresAt` | Property | `ExpiresAt()` | ✅ |
| `inboxHash` | Property | `InboxHash()` | ✅ |
| `listEmails()` | ✅ | `GetEmails()` | ✅ |
| `getEmail(id)` | ✅ | `GetEmail()` | ✅ |
| `getRawEmail(id)` | ✅ | Via `email.GetRaw()` | ✅ |
| `waitForEmail()` | ✅ | `WaitForEmail()` | ✅ |
| `waitForEmailCount()` | ✅ | `WaitForEmailCount()` | ✅ |
| `onNewEmail(cb)` | ✅ | `OnNewEmail()` | ✅ |
| `markEmailAsRead(id)` | ✅ | Via `email.MarkAsRead()` | ✅ |
| `deleteEmail(id)` | ✅ | Via `email.Delete()` | ✅ |
| `delete()` | ✅ | `Delete()` | ✅ |
| `getSyncStatus()` | ✅ | `GetSyncStatus()` | ✅ |
| `export()` | ✅ | `Export()` | ✅ |

### 4.2 Missing Inbox Features
- [x] Add `GetSyncStatus() (*SyncStatus, error)`
- [x] Add `OnNewEmail(callback) Subscription` with unsubscribe

---

## Phase 5: Email API Alignment ✅ COMPLETED

Ensure Email properties and methods match.

### 5.1 Email Properties
| Property | Node SDK | Go SDK | Status |
|----------|----------|--------|--------|
| `id` | ✅ | `ID` | ✅ |
| `from` | ✅ | `From` | ✅ |
| `to` | `string[]` | `[]string` | ✅ |
| `subject` | ✅ | `Subject` | ✅ |
| `receivedAt` | `Date` | `time.Time` | ✅ |
| `isRead` | ✅ | `IsRead` | ✅ |
| `text` | `string|null` | `string` | ✅ |
| `html` | `string|null` | `string` | ✅ |
| `attachments` | ✅ | `[]Attachment` | ✅ |
| `links` | `string[]` | `[]string` | ✅ |
| `headers` | `Record<string,unknown>` | `map[string]string` | ✅ (intentional) |
| `authResults` | `AuthResults` | `*authresults.AuthResults` | ✅ |

**Note:** The `headers` field uses `map[string]string` in Go for type safety, while the Node SDK uses `Record<string, unknown>`. Non-string header values from the server are omitted during parsing. This is intentional as email headers are typically string values.

### 5.2 Email Methods
| Method | Node SDK | Go SDK | Status |
|--------|----------|--------|--------|
| `markAsRead()` | ✅ | `MarkAsRead()` | ✅ |
| `delete()` | ✅ | `Delete()` | ✅ |
| `getRaw()` | ✅ | `GetRaw()` | ✅ |

### 5.3 Attachment Structure
- [x] Verify `Filename`, `ContentType`, `Size` fields - JSON tags updated to camelCase
- [x] Verify `ContentID`, `ContentDisposition` for inline attachments - JSON tags updated
- [x] Verify `Content` ([]byte) and `Checksum` fields - Using `Base64Bytes` type
- [x] Handle base64-encoded content from server - `Base64Bytes` custom unmarshaler handles base64 strings

---

## Phase 6: Authentication Results Alignment ✅ COMPLETED

Ensure email authentication validation matches.

### 6.1 AuthResults Structure
| Component | Node SDK | Go SDK | Status |
|-----------|----------|--------|--------|
| `spf` | `SPFResult` | `*SPFResult` | ✅ |
| `dkim` | `DKIMResult[]` | `[]DKIMResult` | ✅ |
| `dmarc` | `DMARCResult` | `*DMARCResult` | ✅ |
| `reverseDns` | `ReverseDNSResult` | `*ReverseDNSResult` | ✅ |

### 6.2 Validation Method
- [x] Implement `Validate() AuthValidation` matching Node SDK:
  ```go
  type AuthValidation struct {
      Passed           bool
      SPFPassed        bool
      DKIMPassed       bool
      DMARCPassed      bool
      ReverseDNSPassed bool
      Failures         []string
  }
  ```
- [x] Add `IsPassing() bool` convenience method

**Note:** The `Passed` field (and `IsPassing()`) only checks SPF, DKIM, and DMARC to match Node SDK behavior. Reverse DNS is reported separately but does not affect the overall pass status.

---

## Phase 7: Delivery Strategies Alignment ✅ COMPLETED

Ensure SSE and Polling strategies match Node SDK behavior.

### 7.1 SSE Strategy
- [x] Verify SSE endpoint URL format: `/api/events?inboxes={hash1},{hash2},...`
- [x] Verify `X-API-Key` header authentication
- [x] Implement reconnection with exponential backoff
- [x] Match reconnection parameters (5s interval, 10 max attempts, 2x multiplier)
- [x] Handle SSE event parsing (JSON payload)

### 7.2 Polling Strategy
- [x] Verify polling uses sync status (hash-based change detection)
- [x] Implement exponential backoff with jitter:
  - Initial: 2s
  - Max: 30s
  - Multiplier: 1.5
  - Jitter: 0.3
- [x] Reset backoff when changes detected

### 7.3 Auto Strategy
- [x] Try SSE first with timeout (5s)
- [x] Fall back to polling on SSE failure
- [x] Use polling for `WaitForEmail`/`WaitForEmailCount` (backward compat)

**Implementation Notes:**
- Added `WaitForEmailWithSync` and `WaitForEmailCountWithSync` methods with optional `SyncFetcher` for hash-based change detection
- Polling loop now uses adaptive per-inbox intervals with jitter
- SSE strategy signals connection establishment via `Connected()` channel
- Auto strategy properly waits for SSE connection before declaring success

---

## Phase 8: Error Types Alignment

Ensure error types match Node SDK.

### 8.1 Error Classes
| Node SDK Error | Go SDK Error | Status |
|----------------|--------------|--------|
| `VaultSandboxError` | Base interface | ✅ |
| `ApiError` | `APIError` | ✅ |
| `NetworkError` | `NetworkError` | ✅ |
| `TimeoutError` | `TimeoutError` | ✅ |
| `DecryptionError` | `DecryptionError` | ✅ |
| `SignatureVerificationError` | `SignatureVerificationError` | ✅ |
| `InboxNotFoundError` | `ErrInboxNotFound` | ✅ |
| `EmailNotFoundError` | `ErrEmailNotFound` | ✅ |
| `SSEError` | `ErrSSEConnection` | ✅ |
| `InboxAlreadyExistsError` | `ErrInboxAlreadyExists` | ✅ |
| `InvalidImportDataError` | `ErrInvalidImportData` | ✅ |
| `StrategyError` | `StrategyError` | ✅ |

### 8.2 Error Enhancements
- [x] Add `StrategyError` type
- [ ] Ensure all errors implement `VaultSandboxError` interface
- [ ] Add `Unwrap()` for error chaining

---

## Phase 9: Testing & Validation

Verify implementations with test cases.

### 9.1 Unit Tests
- [ ] Crypto operations (keypair, signature, decrypt)
- [ ] Base64 encoding/decoding edge cases
- [ ] Error type assertions
- [ ] Option builders

### 9.2 Integration Tests
- [ ] Create inbox, send email, receive email
- [ ] Export/import inbox round-trip
- [ ] Email filtering (subject, from, predicate)
- [ ] SSE real-time delivery
- [ ] Polling fallback
- [ ] Authentication results parsing

### 9.3 Cross-SDK Validation
- [ ] Export from Node, import in Go
- [ ] Export from Go, import in Node
- [ ] Same inbox, same emails, verify decrypt works
- [ ] Compare decrypted content byte-for-byte

---

## Phase 10: Documentation & Examples

Ensure documentation and examples are complete.

### 10.1 Examples
- [x] Basic inbox creation (`examples/basic`)
- [x] Wait for email (`examples/waitforemail`)
- [x] Export/import (`examples/export`)
- [x] SMTP integration (`examples/smtp`)
- [ ] Monitor multiple inboxes
- [ ] Authentication validation
- [ ] Custom HTTP client
- [ ] Error handling patterns

### 10.2 Documentation
- [ ] README with quick start
- [ ] API reference (godoc)
- [ ] Configuration options
- [ ] Error handling guide
- [ ] Security considerations

---

## Summary: Priority Tasks

### High Priority (Breaking/Security)
1. Verify signature transcript matches exactly
2. Verify HKDF key derivation matches exactly
3. Verify decryption produces identical output

### Medium Priority (Feature Parity)
1. ~~Add `MonitorInboxes()` with event subscription~~ ✅
2. ~~Add `OnNewEmail()` subscription on inbox~~ ✅
3. ~~Add `GetSyncStatus()` on inbox~~ ✅
4. ~~Add file-based export/import helpers~~ ✅
5. ~~Expose `CheckKey()` publicly~~ ✅

### Low Priority (Polish)
1. Add configurable retry status codes
2. Add `StrategyError` type
3. Improve documentation
4. Add more examples

---

## Verification Checklist

Before marking a phase complete:

- [ ] Code compiles without warnings
- [ ] All unit tests pass
- [ ] Integration tests pass against real server
- [ ] Behavior matches Node SDK (manual verification)
- [ ] No security regressions (signature before decrypt)
