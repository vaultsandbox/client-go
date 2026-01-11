# Changelog

All notable changes to this project will be documented in this file.

The format is inspired by [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
