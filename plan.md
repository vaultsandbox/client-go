# Godoc Improvement Plan

## Overview

This plan outlines improvements to Go documentation comments across the codebase. The public API is already well-documented; focus is on internal packages and consistency.

## Priority Levels

- **P0**: Critical - security-related or public API gaps
- **P1**: High - internal packages used frequently
- **P2**: Medium - utility functions and types
- **P3**: Low - nice-to-have improvements

---

## Phase 1: Internal API Package

### `internal/api/client.go`

- [ ] Add package-level doc comment for `internal/api`
- [ ] Document `Client` struct fields
- [ ] Document `New()` and `NewClient()` with parameter explanations
- [ ] Document HTTP methods (`Get`, `Post`, `Delete`, etc.)
- [ ] Document `do()` internal method for maintainability

### `internal/api/types.go`

- [ ] Review and enhance struct field comments
- [ ] Add JSON tag explanations where non-obvious

---

## Phase 2: Internal Crypto Package

### `internal/crypto/decrypt.go`

- [ ] Document `DecryptedMetadata` struct and fields
- [ ] Document `DecryptedParsed` struct and fields
- [ ] Document `DecryptedEmail` struct and fields
- [ ] Document `DecryptedAttachment` struct and fields
- [ ] Document `DecryptEmail()` function with security notes
- [ ] Document `DecryptAttachment()` function

### `internal/crypto/aes.go`

- [ ] Document `DecryptAES()` with algorithm details (AES-256-GCM)
- [ ] Document `EncryptAES()` with nonce handling explanation
- [ ] Add security notes about IV/nonce uniqueness

### `internal/crypto/base64.go`

- [ ] Document base64 utility functions
- [ ] Explain URL-safe vs standard encoding choices

---

## Phase 3: Internal Delivery Package ✓

### `internal/delivery/polling.go`

- [x] Add package-level doc comment for `internal/delivery`
- [x] Document `PollingStrategy` struct
- [x] Document `Start()` method with lifecycle details
- [x] Document `Stop()` method with cleanup behavior
- [x] Document `AddInbox()` / `RemoveInbox()` methods
- [x] Document backoff/retry behavior

### `internal/delivery/sse.go`

- [x] Document `SSEStrategy` struct
- [x] Document SSE connection lifecycle
- [x] Document reconnection behavior
- [x] Document event parsing logic
- [x] Add notes about server-sent events protocol

### `internal/delivery/auto.go`

- [x] Document `AutoStrategy` struct
- [x] Document strategy selection logic (SSE vs polling fallback)
- [x] Document failover behavior

### `internal/delivery/strategy.go`

- [x] Enhance interface method documentation
- [x] Add usage examples in interface docs

---

## Phase 4: Consistency Improvements ✓

### All Files

- [x] Ensure all exported items start with their name (Go convention)
- [x] Add period at end of all doc comments
- [x] Review for consistent terminology across packages

### Add Examples

- [x] Add `Example` functions in `_test.go` files for key APIs
- [x] These appear in `go doc` output and pkg.go.dev

---

## Phase 5: Package-Level Documentation ✓

### Missing `doc.go` Files

- [x] Create `internal/api/doc.go` with package overview
- [x] Create `internal/crypto/doc.go` with security overview
- [x] Create `internal/delivery/doc.go` with strategy explanation

---

## Validation

After completing improvements:

1. Run `go doc ./...` to verify all packages have documentation
2. Run `golint ./...` to catch missing doc comments
3. Review generated docs with `pkgsite` locally

---

## Notes

- Internal packages are lower priority for end users but improve maintainability
- Security-related functions should always have clear warnings
- Examples in doc comments are rendered by `go doc` and pkg.go.dev
