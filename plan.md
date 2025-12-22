# Findings

- Medium: nil deref on import. `Client.ImportInbox` dereferences `data` without a nil check, which will panic on nil input.
  - File: `client.go` (ImportInbox)
- Medium (security/compat): SSE ignores custom HTTP client settings. `OpenEventStream` creates a new `http.Client` with default transport, bypassing user TLS/proxy/mTLS settings.
  - File: `internal/api/endpoints.go` (OpenEventStream)
- Medium (reliability): SSE parser uses default `bufio.Scanner` token size (64K); large events can fail with `ErrTooLong`, dropping events/connection.
  - File: `internal/delivery/sse.go` (connect)
- Low (availability/optimization): jitter uses `math/rand` without seeding, so clients share identical sequences; reduces thundering-herd protection.
  - Files: `internal/delivery/polling.go`, `internal/api/retry.go`
- Low (optimization): `handleSSEEvent` does O(n) scan over inboxes to match `InboxID`; consider map for O(1).
  - File: `client.go` (handleSSEEvent)
