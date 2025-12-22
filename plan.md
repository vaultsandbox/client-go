# Findings

## 1. Medium: nil deref on ImportInbox
`Client.ImportInbox` dereferences `data` without a nil check, which will panic on nil input.

**File:** `client.go` (ImportInbox, line ~194)

**Fix:** Add nil check before acquiring lock:
```go
func (c *Client) ImportInbox(ctx context.Context, data *ExportedInbox) (*Inbox, error) {
    if data == nil {
        return nil, errors.New("exported inbox data cannot be nil")
    }

    c.mu.Lock()
    // ... rest of function
}
```

---

## 2. Medium (security/compat): SSE ignores custom HTTP client settings
`OpenEventStream` creates a new `http.Client` with default transport, bypassing user TLS/proxy/mTLS settings.

**File:** `internal/api/endpoints.go` (OpenEventStream, line ~146)

**Fix:** Clone the existing client's Transport, only overriding the timeout:
```go
func (c *Client) OpenEventStream(ctx context.Context, inboxHashes []string) (*http.Response, error) {
    // ... request setup ...

    // Clone transport from existing client, but disable timeout for SSE
    sseClient := &http.Client{
        Transport: c.httpClient.Transport,
        Timeout:   0,
    }
    return sseClient.Do(req)
}
```

This preserves:
- Custom TLS configuration (root CAs, client certs for mTLS)
- Proxy settings
- Custom dialers
- Connection pooling settings

---

## 3. Medium (reliability): SSE parser buffer size limit
SSE parser uses default `bufio.Scanner` token size (64KB); large events can fail with `ErrTooLong`, dropping events/connection.

**File:** `internal/delivery/sse.go` (connect, line ~220)

**Fix:** Configure scanner with larger buffer:
```go
scanner := bufio.NewScanner(resp.Body)
// Allow lines up to 1MB (default is 64KB)
scanner.Buffer(make([]byte, 64*1024), 1024*1024)

for scanner.Scan() {
    // ...
}
```

Consider making the max size configurable via `SSEConfig` if users may have exceptionally large events.

---

## 4. Low (optimization): O(n) inbox lookup in handleSSEEvent
`handleSSEEvent` does O(n) scan over inboxes to match `InboxID`; could use map for O(1).

**File:** `client.go` (handleSSEEvent, line ~382)

**Fix:** Add secondary index map to Client struct:
```go
type Client struct {
    // ... existing fields ...
    inboxes         map[string]*Inbox  // keyed by email address
    inboxesByHash   map[string]*Inbox  // keyed by inbox hash (new)
}
```

Update handleSSEEvent to use direct lookup:
```go
func (c *Client) handleSSEEvent(event *api.SSEEvent) error {
    // ...
    c.mu.RLock()
    inbox := c.inboxesByHash[event.InboxID]
    c.mu.RUnlock()

    if inbox == nil {
        return nil
    }
    // ...
}
```

Maintain the secondary map in `CreateInbox`, `ImportInbox`, and inbox removal paths.
