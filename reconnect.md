# SSE Reconnection System

This document explains how the Server-Sent Events (SSE) reconnection mechanism works, particularly when inboxes are added or removed.

## Overview

The system uses a single SSE connection to receive real-time email notifications for all tracked inboxes. When the inbox list changes (e.g., user creates or deletes an inbox), the SSE connection is closed and a new one is established with the updated inbox list. A hash-based synchronization mechanism ensures no emails are lost during reconnection.

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  InboxService   │────▶│ InboxSyncService │────▶│  VaultSandbox   │
│  (Operations)   │     │ (Orchestration)  │     │ (SSE Connection)│
└─────────────────┘     └──────────────────┘     └─────────────────┘
                               │
                               ▼
                        ┌──────────────────┐
                        │ VaultSandboxAPI  │
                        │ (HTTP Endpoints) │
                        └──────────────────┘
```

## SSE Connection

### Establishing Connection

The `VaultSandbox` service manages the SSE connection (`src/app/shared/services/vault-sandbox.ts`):

```typescript
this.eventSource = new EventSource(`${environment.apiUrl}/events?inboxes=${inboxHashes.join(',')}`, {
  fetch: (input, init) =>
    fetch(input, {
      ...init,
      headers: { ...init.headers, 'x-api-key': apiKey },
    }),
});
```

The connection:
- Connects to `/events` endpoint
- Passes all inbox hashes as comma-separated query parameters
- Includes the API key for authentication

### Receiving Events

When an email arrives, the server sends a `NewEmailEvent`:

```typescript
interface NewEmailEvent {
  inboxId: string;           // Inbox hash
  emailId: string;           // Email unique ID
  encryptedMetadata: EncryptedPayload;
}
```

## Reconnection Flow

### When a New Inbox is Created

1. **User creates inbox** via `InboxService.createInbox()`
2. **API call** to `POST /api/inboxes` creates the inbox on server
3. **State update** via `InboxStateService.addInbox()`
4. **SSE reconnection** triggered via `InboxSyncService.subscribeToAllInboxes()`

```
User Creates Inbox
       ↓
InboxService.createInbox()
       ↓
POST /api/inboxes
       ↓
InboxStateService.addInbox()
       ↓
sync.subscribeToAllInboxes()
       ↓
VaultSandbox.connectToEvents([...existingHashes, newHash])
       ↓
Close old EventSource → Create new EventSource
```

### When an Inbox is Deleted

The same reconnection flow occurs, but with the deleted inbox removed from the hash list:

```typescript
await this.sync.subscribeToAllInboxes(); // Reconnects with reduced inbox list
```

### The subscribeToAllInboxes Method

Located in `InboxSyncService` (`src/app/features/mail/services/inbox-sync.service.ts`):

```typescript
async subscribeToAllInboxes(): Promise<void> {
  const inboxHashes = this.state.getInboxHashes();

  if (inboxHashes.length === 0) {
    this.vaultSandbox.disconnectEvents();
    return;
  }

  this.vaultSandbox.connectToEvents(inboxHashes);

  // Sync all inboxes to ensure no emails were missed
  await Promise.all(
    inboxHashes.map((hash) => this.loadEmailsForInbox(hash))
  );
}
```

## Hash-Based Synchronization

### The Problem

During SSE reconnection, there's a window where emails could arrive at the server but not be delivered to the client. The hash-based sync mechanism solves this.

### The Sync Endpoint

The server provides a sync endpoint (`/inboxes/:emailAddress/sync`) that returns:

```typescript
{ emailsHash: string; emailCount: number }
```

The `emailsHash` is a hash of the complete email list on the server.

### Sync Logic

When reconnecting, `loadEmailsForInbox()` performs hash comparison:

```typescript
async loadEmailsForInbox(inboxHash: string): Promise<void> {
  const inbox = this.state.getInboxSnapshot(inboxHash);

  // Get server's current hash
  const syncStatus = await this.api.getInboxSyncStatus(inbox.emailAddress);

  // Compare with local hash
  if (inbox.emailsHash === syncStatus.emailsHash) {
    return; // No changes, skip full fetch
  }

  // Hashes differ - fetch full email list
  const result = await this.api.listEmails(inbox.emailAddress);

  // Update local state with new emails and new hash
  const updatedInbox = {
    ...inbox,
    emails: [...existingEmails, ...newEmails],
    emailsHash: syncStatus.emailsHash,
  };
  this.state.updateInbox(updatedInbox);
}
```

### Flow Diagram

```
SSE Reconnection
       ↓
Call /inboxes/{email}/sync
       ↓
Get server emailsHash
       ↓
Compare with local emailsHash
       ↓
┌──────┴──────┐
│             │
▼             ▼
Match       Different
│             │
▼             ▼
Skip        Fetch /inboxes/{email}/emails
            │
            ▼
            Decrypt & deduplicate
            │
            ▼
            Update local state with new hash
```

## Deduplication

Multiple mechanisms prevent duplicate emails:

### 1. Email ID Tracking

When adding emails from a sync:

```typescript
const existingEmailIds = new Set(inbox.emails.map((e) => e.id));
const newEmails = result.filter((email) => !existingEmailIds.has(email.id));
```

### 2. SSE Event Deduplication

When receiving SSE events:

```typescript
if (inbox.emails.some((email) => email.id === event.emailId)) {
  return; // Email already exists, ignore duplicate SSE event
}
```

## Automatic Reconnection on Network Failure

If the SSE connection fails due to network issues:

```typescript
private scheduleReconnect(): void {
  if (this.reconnectTimer || this.trackedInboxIds.length === 0) {
    return;
  }

  this.reconnectAttempts++;
  this.reconnectTimer = setTimeout(() => {
    this.reconnectTimer = null;
    if (this.trackedInboxIds.length > 0) {
      this.connectToEvents(this.trackedInboxIds);
    }
  }, 2000); // 2-second reconnection interval
}
```

## Complete Example: Adding a Second Inbox

1. User has one inbox (`inbox-hash-1`) with active SSE connection
2. User creates a new inbox
3. Server returns `inbox-hash-2` for the new inbox
4. `InboxSyncService.subscribeToAllInboxes()` is called
5. `VaultSandbox.connectToEvents(['inbox-hash-1', 'inbox-hash-2'])`:
   - Closes existing EventSource
   - Creates new EventSource: `/events?inboxes=inbox-hash-1,inbox-hash-2`
6. For each inbox, `loadEmailsForInbox()` is called:
   - Fetches `emailsHash` from `/inboxes/{email}/sync`
   - Compares with local hash
   - Fetches missing emails if hashes differ
7. New SSE connection receives events for both inboxes

## Key Files

| File | Purpose |
|------|---------|
| `src/app/shared/services/vault-sandbox.ts` | SSE connection management |
| `src/app/features/mail/services/inbox-sync.service.ts` | Sync orchestration |
| `src/app/features/mail/services/inbox.service.ts` | Inbox CRUD operations |
| `src/app/features/mail/services/inbox-state.service.ts` | Local state management |
| `src/app/features/mail/services/vault-sandbox-api.ts` | HTTP API client |
