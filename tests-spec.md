---
title: Test Specification
description: Language-agnostic specification for testing VaultSandbox SDK implementations.
---

# VaultSandbox SDK Test Specification

Language-agnostic specification for testing VaultSandbox SDK implementations.

---

## Overview

This document defines the minimum required tests for any VaultSandbox SDK implementation. Tests are categorized by their dependencies and purpose.

### Test Categories

| Category | Server Required | SMTP Required | Purpose |
|----------|-----------------|---------------|---------|
| Unit | No | No | Test isolated functions and classes |
| Integration | Yes | No | Test API interactions |
| E2E | Yes | Yes | Test complete email workflows |

### Conventions

- **[REQUIRED]** - Must be implemented
- **[RECOMMENDED]** - Should be implemented if feasible
- **[OPTIONAL]** - Nice to have

---

## 1. Unit Tests

Tests that require no external dependencies. Use mocks where necessary.

### 1.1 Cryptographic Utilities

#### Base64URL Encoding [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Round-trip | Encode then decode arbitrary bytes | Original bytes recovered |
| No padding | Encode data that would require padding | No `=` characters in output |
| URL-safe chars | Encode data that produces `+` and `/` in standard base64 | Uses `-` and `_` instead |

#### Keypair Generation [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Generate keypair | Call keypair generation | Returns public key (1184 bytes), secret key (2400 bytes), and base64 public key |
| Unique keypairs | Generate two keypairs | Different public keys |
| Correct sizes | Check ML-KEM-768 key sizes | Public: 1184 bytes, Secret: 2400 bytes |

#### Keypair Validation [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Valid keypair | Validate a correctly generated keypair | Returns true |
| Invalid sizes | Validate keypair with wrong key sizes | Returns false |
| Mismatched base64 | Validate keypair where base64 doesn't match public key | Returns false |
| Missing fields | Validate incomplete keypair object | Returns false |

### 1.2 Type Validation

#### AuthResults Validation [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| All pass | SPF=pass, DKIM=pass, DMARC=pass | `passed=true`, no failures |
| SPF fail | SPF=fail, others pass | `passed=false`, `spf_passed=false`, failure message |
| DKIM fail | DKIM=fail, others pass | `passed=false`, `dkim_passed=false`, failure message |
| DMARC fail | DMARC=fail, others pass | `passed=false`, `dmarc_passed=false`, failure message |
| DKIM partial pass | Multiple DKIM results, at least one passes | `dkim_passed=true` |
| None status | All statuses are "none" | `passed=false` (requires explicit "pass") |
| Empty results | No auth data present | `passed=false`, all checks false |
| Reverse DNS fail | Reverse DNS fails, others pass | `passed=true` (reverse DNS doesn't affect overall) |

### 1.3 Client Configuration

#### Default Configuration [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Default values | Create client with only API key | Uses default URL, timeout, retries, strategy |
| Verify defaults | Check specific default values | Timeout=30000ms, retries=3, strategy=auto |

#### Custom Configuration [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Custom URL | Provide custom base URL | Client uses provided URL |
| Custom timeout | Provide custom timeout | Client uses provided timeout |
| Custom retries | Provide custom retry settings | Client uses provided settings |
| Custom strategy | Specify polling/SSE/auto | Client uses specified strategy |
| Polling config | Custom polling interval and backoff | Strategy uses provided config |
| SSE config | Custom reconnect interval and max attempts | Strategy uses provided config |

---

## 2. Integration Tests

Tests that require a running VaultSandbox server but no SMTP.

> **Skip condition**: Skip all integration tests if API key is not configured or equals a placeholder value.

### 2.1 Client Lifecycle

#### API Key Validation [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Valid key | Call check_key with valid API key | Returns true |
| Invalid key | Call check_key with invalid API key | Returns false or throws ApiError(401) |

#### Server Info [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Get server info | Fetch server capabilities | Returns server_sig_pk, algorithms, max_ttl, default_ttl, allowed_domains |
| Algorithm values | Check returned algorithms | kem=ML-KEM-768, sig=ML-DSA-65, aead=AES-256-GCM, kdf=HKDF-SHA-512 |

#### Client Close [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Graceful close | Close client after operations | No errors thrown |
| Resource cleanup | Close with active subscriptions | Subscriptions cleaned up |

### 2.2 Inbox Management

#### Create Inbox [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Basic creation | Create inbox with defaults | Returns email_address, inbox_hash, expires_at |
| Email format | Check created email address | Contains `@` symbol |
| With custom TTL | Create inbox with TTL parameter | Inbox created with specified TTL |

#### Delete Inbox [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Delete existing | Delete a created inbox | No error |
| Access after delete | Try to access deleted inbox | Throws InboxNotFoundError or similar |

#### Delete All Inboxes [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Delete all | Create multiple inboxes, delete all | Returns count >= created count |

#### Sync Status [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Empty inbox | Get sync status of new inbox | email_count=0, emails_hash defined |
| Consistent hash | Multiple calls without changes | Same emails_hash returned |

### 2.3 Inbox Operations (No Email)

#### List Emails [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Empty inbox | List emails in new inbox | Returns empty array |

#### Get Non-existent Email [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Invalid ID | Get email with fake ID | Throws EmailNotFoundError |

### 2.4 Error Handling

#### Network Errors [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Invalid host | Connect to non-existent server | Throws NetworkError |

#### Uninitialized Client [RECOMMENDED]

| Test | Description | Expected |
|------|-------------|----------|
| Operations before init | Call methods before initialization | Throws RuntimeError or similar |

---

## 3. E2E Tests

Tests that require both a VaultSandbox server and SMTP access.

> **Skip condition**: Skip if SMTP host is not configured.

### 3.1 Basic Email Flow

#### Send and Receive [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Simple text email | Send text email via SMTP, wait for it | Email received with correct subject, body, from |
| Timeout on receive | Wait for email without sending | Throws TimeoutError after specified timeout |

#### HTML Email [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| HTML content | Send email with HTML body | Email has both text and html fields populated |

#### Email with Attachments [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Single attachment | Send email with one attachment | Attachment accessible with filename, content_type, size, content |
| Multiple attachments | Send email with multiple attachments | All attachments accessible |

### 3.2 Email Filtering

#### Filter by Subject [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| String match | Wait for email with subject containing string | Correct email returned |
| Regex match | Wait for email with subject matching pattern | Correct email returned |
| No match timeout | Wait for subject that doesn't exist | Throws TimeoutError |

#### Filter by Sender [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| String match | Wait for email from specific address | Correct email returned |
| Regex match | Wait for email from address matching pattern | Correct email returned |

#### Custom Predicate [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Predicate function | Wait with custom filter function | Returns email matching predicate |

### 3.3 Email Operations

#### List Emails [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Multiple emails | Send N emails, list all | Returns all N emails decrypted |

#### Get Specific Email [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| By ID | Get email by its ID | Returns same email as wait_for_email |

#### Mark as Read [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Via inbox method | Mark email as read via inbox | is_read changes to true |
| Via email method | Mark email as read via email object | is_read changes to true |

#### Delete Email [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Via inbox method | Delete email via inbox | Email no longer in list |
| Via email method | Delete email via email object | Email no longer in list |

#### Raw Email [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Get raw content | Fetch raw MIME source | Returns raw string containing headers and subject |

### 3.4 Email Content

#### Link Extraction [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Links in HTML | Send email with links in HTML | links array contains URLs |

#### Headers Access [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Standard headers | Access email headers | from, subject, date headers accessible |

#### Authentication Results [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Results present | Receive email, check auth_results | auth_results object exists |
| Validate method | Call auth_results.validate() | Returns validation object with passed, failures |
| Direct send fails SPF | Send directly without authorization | SPF status is NOT "pass" |
| Direct send fails DKIM | Send without signing | DKIM status is NOT "pass" |

### 3.5 Multiple Emails

#### Wait for Count [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Wait for N | Send N emails, wait for count N | Returns when count reached |
| Timeout on count | Wait for more emails than sent | Throws TimeoutError |

---

## 4. Strategy Tests

### 4.1 Polling Strategy

#### Configuration [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Default config | Create strategy without config | Uses default interval, backoff, jitter |
| Custom config | Create strategy with custom config | Uses provided values |

#### Behavior [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Timeout with backoff | Wait without email | Times out near specified timeout |
| Custom interval | Specify poll interval | Respects provided interval |
| Concurrent polling | Poll multiple inboxes concurrently | All eventually timeout |

#### Subscription Management [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Subscribe | Subscribe to inbox | Returns subscription object |
| Unsubscribe | Unsubscribe from inbox | Stops polling for that inbox |
| Close | Close strategy | All polling tasks cancelled |

### 4.2 SSE Strategy

#### Configuration [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Default config | Create strategy without config | Uses default reconnect settings |
| Custom config | Create strategy with custom config | Uses provided values |

#### Subscription Management [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Subscribe | Subscribe to inbox | Returns subscription object |
| Unsubscribe | Unsubscribe from inbox | Removes subscription |
| Multiple unsubscribe | Call unsubscribe multiple times | No error (idempotent) |
| Close | Close strategy | All subscriptions cleaned up |

#### Connection Handling [RECOMMENDED]

| Test | Description | Expected |
|------|-------------|----------|
| Connection error | Connect to invalid URL | Handles error gracefully |
| No connect when closing | Subscribe after close | Does not attempt connection |

### 4.3 Real-time Monitoring

#### on_new_email [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Receive via callback | Subscribe, send email | Callback invoked with email |
| Unsubscribe stops callback | Unsubscribe, send email | Callback not invoked |

#### monitor_inboxes [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Multiple inboxes | Monitor 2+ inboxes | Events received for all |
| Unsubscribe all | Unsubscribe from monitor | All subscriptions stopped |

---

## 5. Import/Export Tests

### 5.1 Export [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Export to object | Export inbox | Returns all required fields |
| Required fields | Check exported data | Has email_address, inbox_hash, expires_at, server_sig_pk, public_key_b64, secret_key_b64, exported_at |
| Valid timestamps | Check timestamp fields | Valid ISO 8601 format |
| Valid base64 keys | Check key fields | Valid base64 encoding |
| Export by address | Export using email address string | Works same as inbox object |
| Not found error | Export non-existent inbox | Throws InboxNotFoundError |

### 5.2 Import [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Import valid data | Import previously exported inbox | Returns functional inbox |
| Access emails | List emails from imported inbox | Returns emails that were sent before export |
| Missing fields | Import with missing required fields | Throws InvalidImportDataError |
| Empty fields | Import with empty required fields | Throws InvalidImportDataError |
| Invalid timestamp | Import with invalid timestamp | Throws InvalidImportDataError |
| Invalid base64 | Import with invalid base64 keys | Throws InvalidImportDataError |
| Wrong key length | Import with incorrect key sizes | Throws InvalidImportDataError |
| Server mismatch | Import with different server_sig_pk | Throws InvalidImportDataError |
| Already exists | Import inbox that's already loaded | Throws InboxAlreadyExistsError |

### 5.3 File Operations [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Export to file | Export inbox to JSON file | File created with valid JSON |
| Import from file | Import inbox from JSON file | Returns functional inbox |
| Invalid JSON file | Import from file with invalid JSON | Throws InvalidImportDataError |
| Non-existent file | Import from missing file | Throws error |
| Formatted JSON | Check exported file format | JSON is indented/formatted |

---

## 6. Edge Cases

### 6.1 Error Handling [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Timeout value 0 | Wait with timeout=0 | Returns immediately with timeout error |
| Deleted inbox during wait | Delete inbox while waiting | Throws appropriate error |
| Empty inbox array | Monitor empty array | No crash, returns valid monitor |

### 6.2 Retry Logic [RECOMMENDED]

| Test | Description | Expected |
|------|-------------|----------|
| Retry on 5xx | Server returns 500, then 200 | Eventually succeeds |
| Max retries exceeded | Server always returns 500 | Throws error after max retries |
| No retry on 4xx | Server returns 400 | Fails immediately (no retry) |

### 6.3 Specific Error Types [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| 404 inbox | Access non-existent inbox | Throws InboxNotFoundError |
| 404 email | Access non-existent email | Throws EmailNotFoundError |

---

## 7. README Examples Tests

Ensure all code examples in README documentation work correctly.

### 7.1 Quick Start [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Basic flow | Create inbox, send email, receive, verify | All steps succeed |

### 7.2 Configuration Examples [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| All client options | Use all documented options | Client initializes correctly |
| Environment variables | Load config from env vars | Works as documented |

### 7.3 Feature Examples [REQUIRED]

| Test | Description | Expected |
|------|-------------|----------|
| Filter examples | All documented filter patterns | Work as documented |
| Attachment example | Documented attachment access | Works as documented |
| Auth results example | Documented auth checking | Works as documented |
| Monitor example | Documented monitoring pattern | Works as documented |
| Export/import example | Documented import/export | Works as documented |
| Error handling example | Documented error catching | Catches expected errors |

---

## 8. Test Utilities

### 8.1 Required Helpers

Each implementation should provide:

| Utility | Purpose |
|---------|---------|
| SMTP client | Send test emails to inboxes |
| Cleanup hooks | Delete created inboxes after tests |
| Skip conditions | Skip integration tests when server unavailable |
| Timeout helpers | Reasonable timeouts for async operations |

### 8.2 Environment Variables

Tests should respect these environment variables:

| Variable | Purpose | Default |
|----------|---------|---------|
| `VAULTSANDBOX_URL` | Server URL | Implementation default |
| `VAULTSANDBOX_API_KEY` | API key for testing | None (skip if missing) |
| `SMTP_HOST` | SMTP server hostname | localhost |
| `SMTP_PORT` | SMTP server port | 25 |

---

## Appendix: Test Counts

Minimum required tests by category:

| Category | Required Tests |
|----------|----------------|
| Unit - Crypto | 9 |
| Unit - Types | 8 |
| Unit - Config | 6 |
| Integration - Client | 6 |
| Integration - Inbox | 7 |
| Integration - Errors | 2 |
| E2E - Basic Flow | 4 |
| E2E - Filtering | 6 |
| E2E - Operations | 8 |
| E2E - Content | 6 |
| E2E - Multiple | 2 |
| Strategy - Polling | 6 |
| Strategy - SSE | 6 |
| Strategy - Monitoring | 4 |
| Import/Export | 15 |
| Edge Cases | 5 |
| README Examples | 8 |
| **Total** | **~108** |

---

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2025-12-17 | Initial specification based on Node.js and Python implementations |
