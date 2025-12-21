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

## Phase 3: Internal Delivery Package

### `internal/delivery/polling.go`

- [ ] Add package-level doc comment for `internal/delivery`
- [ ] Document `PollingStrategy` struct
- [ ] Document `Start()` method with lifecycle details
- [ ] Document `Stop()` method with cleanup behavior
- [ ] Document `AddInbox()` / `RemoveInbox()` methods
- [ ] Document backoff/retry behavior

### `internal/delivery/sse.go`

- [ ] Document `SSEStrategy` struct
- [ ] Document SSE connection lifecycle
- [ ] Document reconnection behavior
- [ ] Document event parsing logic
- [ ] Add notes about server-sent events protocol

### `internal/delivery/auto.go`

- [ ] Document `AutoStrategy` struct
- [ ] Document strategy selection logic (SSE vs polling fallback)
- [ ] Document failover behavior

### `internal/delivery/strategy.go`

- [ ] Enhance interface method documentation
- [ ] Add usage examples in interface docs

---

## Phase 4: Consistency Improvements

### All Files

- [ ] Ensure all exported items start with their name (Go convention)
- [ ] Add period at end of all doc comments
- [ ] Review for consistent terminology across packages

### Add Examples

- [ ] Add `Example` functions in `_test.go` files for key APIs
- [ ] These appear in `go doc` output and pkg.go.dev

---

## Phase 5: Package-Level Documentation

### Missing `doc.go` Files

- [ ] Create `internal/api/doc.go` with package overview
- [ ] Create `internal/crypto/doc.go` with security overview
- [ ] Create `internal/delivery/doc.go` with strategy explanation

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
