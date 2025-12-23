# Plan: Refactor `inbox.go`

## Objective
Split the `inbox.go` file into smaller, logically grouped files to improve maintainability and readability. Currently, it mixes public API surface, business logic, and complex decryption/parsing logic.

## Proposed Structure

### 1. `inbox.go` (Public API)
Keep the core struct and public-facing methods here.
- **Symbols**: `Inbox` struct, `EmailAddress()`, `ExpiresAt()`, `InboxHash()`, `IsExpired()`, `Delete()`.
- **Sync/Status**: `GetSyncStatus()`, `SyncStatus` struct.

### 2. `inbox_actions.go` (Operations)
Move methods that perform active operations or fetch data from the API.
- **Methods**: `GetEmails()`, `GetEmail()`, `GetRawEmail()`, `MarkEmailAsRead()`, `DeleteEmail()`.

### 3. `inbox_wait.go` (Waiting & Events)
Move the complex concurrency logic for waiting and event subscriptions.
- **Methods**: `WaitForEmail()`, `WaitForEmailCount()`, `OnNewEmail()`.
- **Helpers**: `inboxEmailSubscription` struct.

### 4. `inbox_crypto.go` (Internal Decryption)
Move all unexported logic related to signature verification, decryption, and JSON parsing of encrypted payloads.
- **Methods**: `decryptEmail()`, `verifyAndDecrypt()`, `applyParsedContent()`, `convertDecryptedEmail()`.
- **Helpers**: `parseMetadata()`, `parseParsedContent()`, `buildDecryptedEmail()`, `wrapCryptoError()`.

### 5. `inbox_export.go` (Import/Export)
Move logic for serializing and restoring inboxes.
- **Methods**: `Export()`, `newInboxFromExport()`.
- **Data**: `ExportedInbox` struct and its `Validate()` method.

**Note**: `newInboxFromResult()` is internal and should stay with the client code that calls it (likely in the client package).

## Benefits
- **Clearer Boundaries**: Separates I/O logic from crypto logic.
- **Easier Testing**: Allows focusing tests on specific areas (e.g., crypto vs. wait logic).
- **Reduced Cognitive Load**: No longer need to scroll through 400+ lines of mixed concerns.
