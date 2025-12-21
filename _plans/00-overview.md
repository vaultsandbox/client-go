# VaultSandbox Go Client SDK - Implementation Plan

## Overview

This plan outlines the implementation of a Go client SDK for VaultSandbox, a secure receive-only SMTP server for QA/testing environments. The SDK enables creating temporary email inboxes with quantum-safe encryption.

## Architecture Layers

```
┌────────────────────────────────────────────────────────────┐
│                     User-Facing API                        │
│         (Client, Inbox, Email types)                       │
├────────────────────────────────────────────────────────────┤
│                   Delivery Strategy Layer                  │
│              (SSE Strategy / Polling Strategy)             │
├────────────────────────────────────────────────────────────┤
│                       HTTP Layer                           │
│           (API Client with retry logic)                    │
├────────────────────────────────────────────────────────────┤
│                      Crypto Layer                          │
│    (ML-KEM-768, ML-DSA-65, AES-256-GCM, HKDF-SHA-512)      │
└────────────────────────────────────────────────────────────┘
```

## Plan Files

| File | Description |
|------|-------------|
| [01-project-structure.md](01-project-structure.md) | Package layout and module organization |
| [02-crypto-layer.md](02-crypto-layer.md) | Post-quantum cryptography implementation |
| [03-http-layer.md](03-http-layer.md) | HTTP client with retry logic |
| [04-data-types.md](04-data-types.md) | Core data structures and models |
| [05-delivery-strategies.md](05-delivery-strategies.md) | SSE and polling implementations |
| [06-client-api.md](06-client-api.md) | Main client interface |
| [07-error-handling.md](07-error-handling.md) | Error types and handling |
| [08-testing.md](08-testing.md) | Testing strategy |

## Implementation Order

1. **Phase 1: Foundation**
   - Project structure setup
   - Crypto layer (ML-KEM-768, ML-DSA-65, AES-256-GCM, HKDF)
   - Base64url utilities

2. **Phase 2: Core Infrastructure**
   - Error types
   - Data structures
   - HTTP client with retry logic

3. **Phase 3: API Implementation**
   - Inbox management
   - Email operations
   - Decryption pipeline

4. **Phase 4: Delivery Strategies**
   - SSE strategy with reconnection
   - Polling strategy with backoff
   - Auto strategy selection

5. **Phase 5: Advanced Features**
   - Export/import inbox
   - Email filtering
   - Authentication results validation

6. **Phase 6: Testing & Polish**
   - Unit tests
   - Integration tests
   - Documentation

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/cloudflare/circl` | ML-KEM-768 and ML-DSA-65 |
| `golang.org/x/crypto` | HKDF-SHA-512 |
| `github.com/joho/godotenv` | Load .env files for integration tests |
| Standard library | AES-256-GCM, HTTP, SSE |

## Key Design Decisions

1. **Context-based API**: All operations accept `context.Context` for cancellation
2. **Functional options**: Use options pattern for configuration
3. **Interface-based strategies**: Allow custom delivery strategy implementations
4. **Immutable emails**: Email objects are read-only after creation
5. **Thread-safe client**: Support concurrent inbox operations
