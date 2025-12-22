# Phase 4: Clean Email Struct (Go-Idiomatic Refactor)

## Goal

Make `Email` a pure data struct with no hidden dependencies, following Go SDK conventions.

## Current State

```go
// email.go
type Email struct {
    ID          string
    // ... data fields
    inbox *Inbox  // Hidden coupling
}

func (e *Email) GetRaw(ctx context.Context) (string, error) {
    return e.inbox.client.apiClient.GetEmailRaw(...)  // 3 levels deep
}
func (e *Email) MarkAsRead(ctx context.Context) error { ... }
func (e *Email) Delete(ctx context.Context) error { ... }
```

**Problems:**
1. `Email` has hidden `inbox *Inbox` field (unexported coupling)
2. Methods reach through 3 levels: `email.inbox.client.apiClient`
3. `Email` cannot be created/tested in isolation
4. Cannot serialize/deserialize `Email` without losing the inbox reference

## Target State

```go
// email.go - pure data struct
type Email struct {
    ID          string
    From        string
    To          []string
    Subject     string
    Text        string
    HTML        string
    ReceivedAt  time.Time
    Headers     map[string]string
    Attachments []Attachment
    Links       []string
    AuthResults *authresults.AuthResults
    IsRead      bool
    // NO inbox reference
}

// No methods on Email that require API calls
```

Operations move to `Inbox` (which already has them per README):
```go
// inbox.go - already exists
func (i *Inbox) GetRawEmail(ctx context.Context, emailID string) (string, error)
func (i *Inbox) MarkEmailAsRead(ctx context.Context, emailID string) error
func (i *Inbox) DeleteEmail(ctx context.Context, emailID string) error
```

## Why This Is Go-Idiomatic

| Pattern | Example | Our Equivalent |
|---------|---------|----------------|
| `db.Query()` returns `Rows` (data) | database/sql | `inbox.GetEmails()` returns `[]*Email` (data) |
| `client.Do(request)` | net/http | `inbox.DeleteEmail(ctx, id)` |
| `s3Client.DeleteObject()` | aws-sdk-go | `inbox.DeleteEmail(ctx, id)` |
| `client.Issues.Create()` | go-github | `inbox.MarkEmailAsRead(ctx, id)` |

**Rule:** Data structs are dumb. Service/client types perform operations.

---

## Implementation Steps

### Step 1: Update `email.go`

**Remove:**
- The `inbox *Inbox` field
- `GetRaw()` method
- `MarkAsRead()` method
- `Delete()` method

**After:**
```go
package vaultsandbox

import (
    "time"

    "github.com/vaultsandbox/client-go/authresults"
)

// Email represents a decrypted email.
type Email struct {
    ID          string
    From        string
    To          []string
    Subject     string
    Text        string
    HTML        string
    ReceivedAt  time.Time
    Headers     map[string]string
    Attachments []Attachment
    Links       []string
    AuthResults *authresults.AuthResults
    IsRead      bool
}

// Attachment represents an email attachment.
type Attachment struct {
    Filename           string
    ContentType        string
    Size               int
    ContentID          string
    ContentDisposition string
    Content            []byte
    Checksum           string
}
```

### Step 2: Update `inbox.go`

Find all places where `Email` structs are created and remove the `inbox: i` assignment.

**Before:**
```go
return &Email{
    ID:      emailData.ID,
    From:    metadata.From,
    // ...
    inbox:   i,
}
```

**After:**
```go
return &Email{
    ID:      emailData.ID,
    From:    metadata.From,
    // ...
}
```

### Step 3: Verify Inbox Methods Exist

Confirm these methods exist on `Inbox` (per README they should):
- `GetRawEmail(ctx, emailID string) (string, error)`
- `MarkEmailAsRead(ctx, emailID string) error`
- `DeleteEmail(ctx, emailID string) error`

### Step 4: Update Tests

Update any tests that use the old `Email` methods:

**Before:**
```go
email, _ := inbox.WaitForEmail(ctx)
raw, err := email.GetRaw(ctx)
email.MarkAsRead(ctx)
email.Delete(ctx)
```

**After:**
```go
email, _ := inbox.WaitForEmail(ctx)
raw, err := inbox.GetRawEmail(ctx, email.ID)
inbox.MarkEmailAsRead(ctx, email.ID)
inbox.DeleteEmail(ctx, email.ID)
```

---

## Documentation Updates

### README.md Changes

#### Section: "Email" (lines 552-579)

**Remove from "Methods" section (lines 574-579):**
```markdown
#### Methods

- `MarkAsRead(ctx) error` — Marks this email as read
- `Delete(ctx) error` — Deletes this email
- `GetRaw(ctx) (string, error)` — Gets raw email source (RFC 5322 format)
```

**Replace with:**
```markdown
`Email` is a pure data struct with no methods. Use `Inbox` methods to perform operations on emails:

- `inbox.GetRawEmail(ctx, emailID)` — Gets raw email source
- `inbox.MarkEmailAsRead(ctx, emailID)` — Marks email as read
- `inbox.DeleteEmail(ctx, emailID)` — Deletes an email
```

#### Update Examples That Use Old API

**Quick Start example (if applicable):**
No changes needed - doesn't use Email methods.

**Any examples using `email.Delete()`, `email.MarkAsRead()`, `email.GetRaw()`:**
Change to use `inbox.DeleteEmail()`, `inbox.MarkEmailAsRead()`, `inbox.GetRawEmail()`.

### Documentation Folder

Check and update files in `documentation/` that reference:
- `email.GetRaw()`
- `email.MarkAsRead()`
- `email.Delete()`

Files to check:
- `documentation/api/` - API reference docs
- `documentation/guides/` - Usage guides
- `documentation/concepts/` - Conceptual docs

---

## Verification

```bash
# Ensure it compiles
go build ./...

# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Check for any remaining references to removed methods
grep -r "\.GetRaw\|\.MarkAsRead\|\.Delete" --include="*.go" .
# Should only find Inbox methods, not Email methods

# Verify documentation has no stale references
grep -r "email\.GetRaw\|email\.MarkAsRead\|email\.Delete" documentation/
```

---

## Files to Modify

| File | Changes |
|------|---------|
| `email.go` | Remove `inbox` field and 3 methods |
| `inbox.go` | Remove `inbox: i` from Email struct literals |
| `*_test.go` | Update tests using old Email methods |
| `README.md` | Update Email section, remove methods |
| `documentation/api/*` | Update API reference |
| `documentation/guides/*` | Update any affected guides |

---

## Rollback

Single commit, easily reversible:
```bash
git revert <commit-hash>
```
