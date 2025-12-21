---
title: Error Handling
description: Complete guide to error handling and retry behavior in VaultSandbox Client
---

The VaultSandbox Client SDK provides comprehensive error handling with automatic retries for transient failures and specific error types for different failure scenarios.

## Error Hierarchy

All SDK errors extend from the base `VaultSandboxError` class, allowing you to catch all SDK-specific errors with a single catch block.

```typescript
VaultSandboxError (base class)
├── ApiError
├── NetworkError
├── TimeoutError
├── InboxNotFoundError
├── EmailNotFoundError
├── InboxAlreadyExistsError
├── InvalidImportDataError
├── DecryptionError
├── SignatureVerificationError
├── SSEError
└── StrategyError
```

## Automatic Retries

The SDK automatically retries failed HTTP requests for transient errors. This helps mitigate temporary network issues or server-side problems.

### Default Retry Behavior

By default, requests are retried for these HTTP status codes:

- `408` - Request Timeout
- `429` - Too Many Requests (Rate Limiting)
- `500` - Internal Server Error
- `502` - Bad Gateway
- `503` - Service Unavailable
- `504` - Gateway Timeout

### Configuration

Configure retry behavior when creating the client:

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

const client = new VaultSandboxClient({
	url: 'https://smtp.vaultsandbox.com',
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	maxRetries: 5, // Default: 3
	retryDelay: 2000, // Default: 1000ms
	retryOn: [408, 429, 500, 502, 503, 504], // Default status codes
});
```

### Retry Strategy

The SDK uses **exponential backoff** for retries:

- 1st retry: `retryDelay` ms
- 2nd retry: `retryDelay * 2` ms
- 3rd retry: `retryDelay * 4` ms
- And so on...

#### Example

```javascript
// With retryDelay: 1000ms and maxRetries: 3
// Retry schedule:
//   1st attempt: immediate
//   2nd attempt: after 1000ms
//   3rd attempt: after 2000ms
//   4th attempt: after 4000ms
//   Total time: up to 7 seconds + request time
```

## Error Types

### VaultSandboxError

Base class for all SDK errors. Use this to catch any SDK-specific error.

```typescript
class VaultSandboxError extends Error {
	name: string;
	message: string;
}
```

#### Example

```javascript
import { VaultSandboxError } from '@vaultsandbox/client';

try {
	const inbox = await client.createInbox();
	// Use inbox...
} catch (error) {
	if (error instanceof VaultSandboxError) {
		console.error('VaultSandbox error:', error.message);
	} else {
		console.error('Unexpected error:', error);
	}
}
```

---

### ApiError

Thrown for API-level errors such as invalid requests or permission denied.

```typescript
class ApiError extends VaultSandboxError {
	statusCode: number;
	message: string;
}
```

#### Properties

- `statusCode`: HTTP status code from the API
- `message`: Error message from the server

#### Example

```javascript
import { ApiError } from '@vaultsandbox/client';

try {
	const inbox = await client.createInbox();
} catch (error) {
	if (error instanceof ApiError) {
		console.error(`API Error (${error.statusCode}): ${error.message}`);

		if (error.statusCode === 401) {
			console.error('Invalid API key');
		} else if (error.statusCode === 403) {
			console.error('Permission denied');
		} else if (error.statusCode === 429) {
			console.error('Rate limit exceeded');
		}
	}
}
```

---

### NetworkError

Thrown when there is a network-level failure (e.g., cannot connect to server).

```typescript
class NetworkError extends VaultSandboxError {
	message: string;
}
```

#### Example

```javascript
import { NetworkError } from '@vaultsandbox/client';

try {
	const inbox = await client.createInbox();
} catch (error) {
	if (error instanceof NetworkError) {
		console.error('Network error:', error.message);
		console.error('Check your internet connection and server URL');
	}
}
```

---

### TimeoutError

Thrown by methods like `waitForEmail()` and `waitForEmailCount()` when the timeout is reached before the condition is met.

```typescript
class TimeoutError extends VaultSandboxError {
	message: string;
}
```

#### Example

```javascript
import { TimeoutError } from '@vaultsandbox/client';

try {
	const email = await inbox.waitForEmail({
		timeout: 5000,
		subject: /Welcome/,
	});
} catch (error) {
	if (error instanceof TimeoutError) {
		console.error('Timed out waiting for email');
		console.error('Email may not have been sent or took too long to deliver');

		// Check what emails did arrive
		const emails = await inbox.listEmails();
		console.log(`Found ${emails.length} emails:`);
		emails.forEach((e) => console.log(`  - ${e.subject}`));
	}
}
```

---

### InboxNotFoundError

Represents an error when an inbox does not exist. This error class is exported for type checking, but in practice, a missing inbox will throw an `ApiError` with `statusCode: 404`.

```typescript
class InboxNotFoundError extends VaultSandboxError {
	message: string;
}
```

#### Example

```javascript
import { ApiError } from '@vaultsandbox/client';

try {
	const emails = await inbox.listEmails();
} catch (error) {
	if (error instanceof ApiError && error.statusCode === 404) {
		console.error('Inbox no longer exists');
		console.error('It may have expired or been deleted');
	}
}
```

---

### EmailNotFoundError

Represents an error when an email does not exist. This error class is exported for type checking, but in practice, a missing email will throw an `ApiError` with `statusCode: 404`.

```typescript
class EmailNotFoundError extends VaultSandboxError {
	message: string;
}
```

#### Example

```javascript
import { ApiError } from '@vaultsandbox/client';

try {
	const email = await inbox.getEmail('non-existent-id');
} catch (error) {
	if (error instanceof ApiError && error.statusCode === 404) {
		console.error('Email not found');
		console.error('It may have been deleted');
	}
}
```

---

### InboxAlreadyExistsError

Thrown when attempting to import an inbox that already exists in the client.

```typescript
class InboxAlreadyExistsError extends VaultSandboxError {
	message: string;
}
```

#### Example

```javascript
import { InboxAlreadyExistsError } from '@vaultsandbox/client';

try {
	const inbox = await client.importInbox(exportedData);
} catch (error) {
	if (error instanceof InboxAlreadyExistsError) {
		console.error('Inbox already imported in this client');
		console.error('Use a new client instance or delete the existing inbox');
	}
}
```

---

### InvalidImportDataError

Thrown when imported inbox data fails validation (missing fields, invalid keys, server mismatch, etc.).

```typescript
class InvalidImportDataError extends VaultSandboxError {
	message: string;
}
```

#### Example

```javascript
import { InvalidImportDataError } from '@vaultsandbox/client';

try {
	const corruptedData = JSON.parse(corruptedJson);
	const inbox = await client.importInbox(corruptedData);
} catch (error) {
	if (error instanceof InvalidImportDataError) {
		console.error('Invalid import data:', error.message);
		console.error('The exported data may be corrupted or from a different server');
	}
}
```

---

### DecryptionError

Thrown if the client fails to decrypt an email. This is rare and may indicate data corruption or a bug.

```typescript
class DecryptionError extends VaultSandboxError {
	message: string;
}
```

#### Example

```javascript
import { DecryptionError } from '@vaultsandbox/client';

try {
	const emails = await inbox.listEmails();
} catch (error) {
	if (error instanceof DecryptionError) {
		console.error('Failed to decrypt email:', error.message);
		console.error('This is a critical error - please report it');

		// Log for investigation
		console.error('Inbox:', inbox.emailAddress);
		console.error('Time:', new Date().toISOString());
	}
}
```

#### Handling

Decryption errors should **always** be logged and investigated as they may indicate:

- Data corruption
- SDK bug
- MITM attack (rare)
- Server-side encryption issue

---

### SignatureVerificationError

Thrown if the cryptographic signature of a message from the server cannot be verified. This is a **critical security error** that may indicate a man-in-the-middle (MITM) attack.

```typescript
class SignatureVerificationError extends VaultSandboxError {
	message: string;
}
```

#### Example

```javascript
import { SignatureVerificationError } from '@vaultsandbox/client';

try {
	const inbox = await client.createInbox();
} catch (error) {
	if (error instanceof SignatureVerificationError) {
		console.error('CRITICAL: Signature verification failed!');
		console.error('This may indicate a MITM attack');
		console.error('Message:', error.message);

		// Alert security team
		alertSecurityTeam({
			error: error.message,
			timestamp: new Date().toISOString(),
			serverUrl: client.config.url,
		});

		throw error; // Do not continue
	}
}
```

#### Handling

Signature verification errors should **never** be ignored:

1. **Log immediately** with full context
2. **Alert security/operations team**
3. **Stop processing** - do not continue with the operation
4. **Investigate** - check for network issues, proxy problems, or actual attacks

---

### SSEError

Thrown for errors related to the Server-Sent Events (SSE) connection.

```typescript
class SSEError extends VaultSandboxError {
	message: string;
}
```

#### Example

```javascript
import { SSEError } from '@vaultsandbox/client';

const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'sse',
});

try {
	const inbox = await client.createInbox();
	const subscription = inbox.onNewEmail((email) => {
		console.log('New email:', email.subject);
	});
} catch (error) {
	if (error instanceof SSEError) {
		console.error('SSE connection error:', error.message);
		console.error('Falling back to polling strategy');

		// Recreate client with polling
		const pollingClient = new VaultSandboxClient({
			url: process.env.VAULTSANDBOX_URL,
			apiKey: process.env.VAULTSANDBOX_API_KEY,
			strategy: 'polling',
		});
	}
}
```

---

### StrategyError

Thrown when a delivery strategy is not set or is invalid.

```typescript
class StrategyError extends VaultSandboxError {
	message: string;
}
```

#### Example

```javascript
import { StrategyError } from '@vaultsandbox/client';

try {
	const inbox = await client.createInbox();
	const subscription = inbox.onNewEmail((email) => {
		console.log('New email:', email.subject);
	});
} catch (error) {
	if (error instanceof StrategyError) {
		console.error('Strategy error:', error.message);
		console.error('This may indicate the delivery strategy is not properly configured');
	}
}
```

## Error Handling Patterns

### Basic Error Handling

```javascript
import { VaultSandboxClient, ApiError, TimeoutError, NetworkError, VaultSandboxError } from '@vaultsandbox/client';

const client = new VaultSandboxClient({ url, apiKey });

try {
	const inbox = await client.createInbox();
	console.log(`Send email to: ${inbox.emailAddress}`);

	const email = await inbox.waitForEmail({ timeout: 10000 });
	console.log('Email received:', email.subject);

	await inbox.delete();
} catch (error) {
	if (error instanceof TimeoutError) {
		console.error('Timed out waiting for email');
	} else if (error instanceof ApiError) {
		console.error(`API Error (${error.statusCode}):`, error.message);
	} else if (error instanceof NetworkError) {
		console.error('Network error:', error.message);
	} else if (error instanceof VaultSandboxError) {
		console.error('VaultSandbox error:', error.message);
	} else {
		console.error('Unexpected error:', error);
	}
}
```

### Retry with Custom Logic

```javascript
async function waitForEmailWithRetry(inbox, options, maxAttempts = 3) {
	let lastError;

	for (let attempt = 1; attempt <= maxAttempts; attempt++) {
		try {
			return await inbox.waitForEmail(options);
		} catch (error) {
			lastError = error;

			if (error instanceof TimeoutError) {
				console.log(`Attempt ${attempt}/${maxAttempts} timed out`);

				if (attempt < maxAttempts) {
					console.log('Retrying...');
					await new Promise((resolve) => setTimeout(resolve, 2000));
				}
			} else {
				// Non-timeout error, don't retry
				throw error;
			}
		}
	}

	throw lastError;
}

// Usage
try {
	const email = await waitForEmailWithRetry(inbox, { timeout: 10000, subject: /Welcome/ }, 3);
	console.log('Email received:', email.subject);
} catch (error) {
	console.error('Failed after retries:', error.message);
}
```

### Graceful Degradation

```javascript
async function getEmailsWithFallback(inbox) {
	try {
		// Try to wait for new email
		return [await inbox.waitForEmail({ timeout: 5000 })];
	} catch (error) {
		if (error instanceof TimeoutError) {
			console.log('No new emails, checking existing...');
			// Fall back to listing existing emails
			return await inbox.listEmails();
		}
		throw error;
	}
}
```

### Test Cleanup with Error Handling

```javascript
describe('Email Tests', () => {
	let client;
	let inbox;

	beforeEach(async () => {
		client = new VaultSandboxClient({ url, apiKey });
		inbox = await client.createInbox();
	});

	afterEach(async () => {
		// Always clean up, even if test failed
		if (inbox) {
			try {
				await inbox.delete();
			} catch (error) {
				if (error instanceof ApiError && error.statusCode === 404) {
					// Inbox already deleted, that's fine
					console.log('Inbox already deleted');
				} else {
					// Log but don't fail the test
					console.error('Failed to delete inbox:', error.message);
				}
			}
		}
	});

	test('should receive email', async () => {
		await sendEmail(inbox.emailAddress);

		const email = await inbox.waitForEmail({
			timeout: 10000,
			subject: /Test/,
		});

		expect(email.subject).toContain('Test');
	});
});
```

## Best Practices

### 1. Always Handle TimeoutError

Timeouts are common in email testing. Always handle them explicitly:

```javascript
try {
	const email = await inbox.waitForEmail({ timeout: 10000 });
} catch (error) {
	if (error instanceof TimeoutError) {
		// List what emails did arrive
		const emails = await inbox.listEmails();
		console.log(`Expected email not found. Received ${emails.length} emails:`);
		emails.forEach((e) => console.log(`  - "${e.subject}" from ${e.from}`));
	}
	throw error;
}
```

### 2. Log Critical Errors

Always log signature verification and decryption errors:

```javascript
try {
	const inbox = await client.createInbox();
} catch (error) {
	if (error instanceof SignatureVerificationError || error instanceof DecryptionError) {
		// Critical security/integrity error
		logger.critical({
			error: error.message,
			type: error.constructor.name,
			timestamp: new Date().toISOString(),
			context: { serverUrl: client.config.url },
		});

		// Alert operations team
		alertOps(error);

		throw error;
	}
}
```

### 3. Use Specific Error Types

Catch specific errors before generic ones:

```javascript
// Good: Specific to general
try {
	// ...
} catch (error) {
	if (error instanceof ApiError && error.statusCode === 404) {
		// Handle not found case (inbox or email)
	} else if (error instanceof ApiError) {
		// Handle other API errors
	} else if (error instanceof TimeoutError) {
		// Handle timeout case
	} else if (error instanceof VaultSandboxError) {
		// Handle any other SDK error
	} else {
		// Handle unexpected errors
	}
}

// Avoid: Too generic
try {
	// ...
} catch (error) {
	if (error instanceof VaultSandboxError) {
		// Can't differentiate between error types
	}
}
```

### 4. Clean Up Resources

Always clean up, even when errors occur:

```javascript
const client = new VaultSandboxClient({ url, apiKey });

try {
	const inbox = await client.createInbox();
	// Use inbox...
} catch (error) {
	console.error('Error:', error.message);
	throw error;
} finally {
	await client.close();
}
```

## Next Steps

- [CI/CD Integration](/client-node/testing/cicd) - Error handling in CI
- [VaultSandboxClient API](/client-node/api/client) - Client configuration
