# Changelog

All notable changes to this project will be documented in this file.

The format is inspired by [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [0.9.2] - 2026-01-29

### Changed

- Tests now run in parallel for faster execution
- Replaced unnecessary `time.Sleep` calls with proper synchronization

## [0.9.1] - 2026-01-27

### Fixed

- Polling strategy now reports handler errors via `OnError` callback
- `DeleteAllInboxes` state consistency on API failure
- Race conditions in `DeleteInbox` and `ImportInbox`
- Nil pointer dereference in `verifyAndDecrypt` for plain inboxes
- SSE strategy state not resetting on reuse
- Memory leak in polling strategy `seenEmails`
- Silently ignored errors in crypto operations and import
- SSE event processing errors now reported via `OnSyncError` callback
- `ErrChaosDisabled` mapping for 403 responses

### Added

- `AuthResultsError` and `SpamAnalysisError` fields on `Email` struct


## [0.9.0] - 2026-01-22

### Added

- Chaos Engineering

## [0.8.5] - 2026-01-20

### Added

- Spam analysis support (Rspamd integration)
- Inbox creation option to enable/disable spam analysis
- Server capability detection for spam analysis

## [0.8.0] - 2026-01-16

### Added

- Webhooks support

## [0.7.0] - 2026-01-13

### Added

- Optional encryption support with `encryptionPolicy` option
- Optional email authentication feature

### Changed

- Updated ReverseDNS structure
- License changed from MIT to Apache 2.0

## [0.6.1] - 2026-01-11

### Removed

- `StrategyAuto` delivery strategy (use `StrategySSE` or `StrategyPolling` explicitly)
- `SSEConnectionTimeout` from `PollingConfig`

## [0.6.0] - 2026-01-04

### Added

- `GetEmailsMetadataOnly()` method for efficient email listing without full content
- `EmailMetadata` type with ID, From, Subject, ReceivedAt, and IsRead fields

### Changed

- `GetEmails()` now returns full email content (uses `?includeContent=true`)
- Export format updated to VaultSandbox spec:
  - Added `version` field (must be 1)
  - Renamed `secretKeyB64` to `secretKey`
  - Removed `publicKeyB64` (public key is now derived from secret key per spec Section 4.2)
  - `exportedAt` now uses UTC
- Enhanced validation per VaultSandbox spec Section 10 with detailed error messages

### Added

- `ExportVersion` constant for export format versioning
- `KeypairFromSecretKey` function to derive ML-KEM public key from secret key
- Public beta notice in README

## [0.5.1] - 2025-12-31

### Changed

- Standardized email authentication result structs to match wire format and other SDKs

### Added

- End-to-end integration tests for email authentication results using the test email API

## [0.5.0] - 2025-12-28

### Initial release

- Quantum-safe email testing SDK with ML-KEM-768 encryption
- Automatic keypair generation and management
- Support for both polling and real-time (SSE) email delivery
- Full email content access including attachments and headers
- Built-in SPF/DKIM/DMARC authentication validation
- Inbox import/export functionality for test reproducibility
- Comprehensive error handling with automatic retries
