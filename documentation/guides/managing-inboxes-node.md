---
title: Managing Inboxes
description: Common operations for creating, using, and deleting inboxes
---

This guide covers common inbox management operations with practical examples.

## Creating Inboxes

### Basic Creation

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,
});

const inbox = await client.createInbox();
console.log(`Email address: ${inbox.emailAddress}`);
```

### With Custom TTL

```javascript
// Expire after 1 hour (good for CI/CD)
const inbox = await client.createInbox({ ttl: 3600 });

// Expire after 10 minutes (quick tests)
const inbox = await client.createInbox({ ttl: 600 });

// Expire after 7 days (long-running tests)
const inbox = await client.createInbox({ ttl: 604800 });
```

### Requesting Specific Address

```javascript
try {
	const inbox = await client.createInbox({
		emailAddress: 'test@mail.example.com',
	});
	console.log('Got requested address:', inbox.emailAddress);
} catch (error) {
	if (error instanceof InboxAlreadyExistsError) {
		console.log('Address already in use, using random address');
		const inbox = await client.createInbox();
	}
}
```

## Listing Emails

### List All Emails

```javascript
const emails = await inbox.listEmails();

console.log(`Inbox contains ${emails.length} emails`);
emails.forEach((email) => {
	console.log(`- ${email.from}: ${email.subject}`);
});
```

### Filtering Emails

```javascript
const emails = await inbox.listEmails();

// Filter by sender
const fromSupport = emails.filter((e) => e.from === 'support@example.com');

// Filter by subject
const passwordResets = emails.filter((e) => /reset/i.test(e.subject));

// Filter by date
const recentEmails = emails.filter(
	(e) => new Date() - e.receivedAt < 3600000 // Last hour
);
```

### Sorting Emails

```javascript
const emails = await inbox.listEmails();

// Sort by date (newest first)
const sortedByDate = emails.sort((a, b) => b.receivedAt - a.receivedAt);

// Sort by sender
const sortedBySender = emails.sort((a, b) => a.from.localeCompare(b.from));
```

## Getting Specific Emails

### By ID

```javascript
const emailId = 'email_abc123';
const email = await inbox.getEmail(emailId);

console.log(email.subject);
```

### With Error Handling

```javascript
try {
	const email = await inbox.getEmail(emailId);
	console.log('Found:', email.subject);
} catch (error) {
	if (error instanceof EmailNotFoundError) {
		console.error('Email not found');
	}
}
```

## Deleting Emails

### Delete Single Email

```javascript
// By ID
await inbox.deleteEmail('email_abc123');

// Via email object
const email = await inbox.getEmail('email_abc123');
await email.delete();
```

### Delete Multiple Emails

```javascript
const emails = await inbox.listEmails();

// Delete all emails
for (const email of emails) {
	await email.delete();
}

// Or in parallel
await Promise.all(emails.map((email) => email.delete()));
```

### Delete by Criteria

```javascript
const emails = await inbox.listEmails();

// Delete old emails
const oldEmails = emails.filter(
	(e) => new Date() - e.receivedAt > 86400000 // Older than 24h
);

await Promise.all(oldEmails.map((email) => email.delete()));
```

## Deleting Inboxes

### Delete Single Inbox

```javascript
await inbox.delete();

// Inbox and all emails are now deleted
```

### Delete All Inboxes

```javascript
// Delete all inboxes for this API key
const count = await client.deleteAllInboxes();
console.log(`Deleted ${count} inboxes`);
```

### Safe Deletion with Cleanup

```javascript
async function withInbox(callback) {
	const inbox = await client.createInbox();
	try {
		await callback(inbox);
	} finally {
		await inbox.delete();
	}
}

// Usage
await withInbox(async (inbox) => {
	await sendTestEmail(inbox.emailAddress);
	const email = await inbox.waitForEmail({ timeout: 10000 });
	expect(email.subject).toContain('Test');
});
```

## Checking Inbox Status

### Check if Inbox Exists

```javascript
try {
	const emails = await inbox.listEmails();
	console.log('Inbox exists');
} catch (error) {
	if (error instanceof InboxNotFoundError) {
		console.log('Inbox expired or deleted');
	}
}
```

### Check Expiration

```javascript
const now = new Date();
const expiresIn = inbox.expiresAt - now;

if (expiresIn < 300000) {
	// Less than 5 minutes
	console.warn('Inbox expiring soon!');
	console.log(`Time left: ${Math.floor(expiresIn / 60000)} minutes`);
}
```

### Get Sync Status

```javascript
const syncStatus = await inbox.getSyncStatus();

console.log('Email count:', syncStatus.emailCount);
console.log('Emails hash:', syncStatus.emailsHash);
```

## Bulk Operations

### Create Multiple Inboxes

```javascript
const inboxes = await Promise.all([client.createInbox(), client.createInbox(), client.createInbox()]);

console.log(`Created ${inboxes.length} inboxes`);
inboxes.forEach((inbox) => {
	console.log(`- ${inbox.emailAddress}`);
});
```

### Clean Up Multiple Inboxes

```javascript
// Delete all
await Promise.all(inboxes.map((inbox) => inbox.delete()));

// Or use convenience method
await client.deleteAllInboxes();
```

## Testing Patterns

### Jest Setup/Teardown

```javascript
describe('Email Tests', () => {
	let client, inbox;

	beforeAll(() => {
		client = new VaultSandboxClient({
			url: process.env.VAULTSANDBOX_URL,
			apiKey: process.env.VAULTSANDBOX_API_KEY,
		});
	});

	beforeEach(async () => {
		inbox = await client.createInbox();
	});

	afterEach(async () => {
		await inbox.delete();
	});

	afterAll(async () => {
		await client.close();
	});

	test('receives email', async () => {
		await sendEmail(inbox.emailAddress);
		const email = await inbox.waitForEmail({ timeout: 10000 });
		expect(email).toBeDefined();
	});
});
```

### Shared Inbox Pattern

```javascript
describe('Email Suite', () => {
	let client, inbox;

	beforeAll(async () => {
		client = new VaultSandboxClient({ url, apiKey });
		inbox = await client.createInbox({ ttl: 7200 }); // 2 hours
	});

	afterAll(async () => {
		await inbox.delete();
		await client.close();
	});

	test('test 1', async () => {
		// Use shared inbox
	});

	test('test 2', async () => {
		// Use shared inbox
	});
});
```

### Inbox Pool Pattern

```javascript
class InboxPool {
	constructor(client, size = 5) {
		this.client = client;
		this.size = size;
		this.available = [];
		this.inUse = new Set();
	}

	async initialize() {
		const promises = Array(this.size)
			.fill()
			.map(() => this.client.createInbox());
		this.available = await Promise.all(promises);
	}

	acquire() {
		if (this.available.length === 0) {
			throw new Error('No inboxes available');
		}
		const inbox = this.available.shift();
		this.inUse.add(inbox);
		return inbox;
	}

	release(inbox) {
		this.inUse.delete(inbox);
		this.available.push(inbox);
	}

	async cleanup() {
		const all = [...this.available, ...this.inUse];
		await Promise.all(all.map((inbox) => inbox.delete()));
	}
}

// Usage
const pool = new InboxPool(client, 5);
await pool.initialize();

const inbox = pool.acquire();
// Use inbox
pool.release(inbox);

await pool.cleanup();
```

## Error Handling

### Handling Expired Inboxes

```javascript
try {
	const emails = await inbox.listEmails();
} catch (error) {
	if (error instanceof InboxNotFoundError) {
		console.log('Inbox expired, creating new one');
		inbox = await client.createInbox();
	} else {
		throw error;
	}
}
```

### Handling Creation Errors

```javascript
try {
	const inbox = await client.createInbox();
} catch (error) {
	if (error instanceof ApiError) {
		console.error('API error:', error.statusCode, error.message);
	} else if (error instanceof NetworkError) {
		console.error('Network error:', error.message);
	} else {
		throw error;
	}
}
```

## Best Practices

### Always Clean Up

```javascript
// ✅ Good: Cleanup in finally block
const inbox = await client.createInbox();
try {
	// Use inbox
} finally {
	await inbox.delete();
}

// ❌ Bad: No cleanup
const inbox = await client.createInbox();
// Use inbox
// Inbox never deleted
```

### Use Appropriate TTL

```javascript
// ✅ Good: Short TTL for CI/CD
const inbox = await client.createInbox({ ttl: 3600 }); // 1 hour

// ❌ Bad: Long TTL wastes resources
const inbox = await client.createInbox({ ttl: 604800 }); // 7 days for quick test
```

### Handle Cleanup Errors

```javascript
async function safeDelete(inbox) {
	try {
		await inbox.delete();
	} catch (error) {
		// Inbox may have already expired
		if (!(error instanceof InboxNotFoundError)) {
			console.error('Error deleting inbox:', error);
		}
	}
}
```

## Next Steps

- **[Waiting for Emails](/client-node/guides/waiting-for-emails/)** - Learn about email waiting strategies
- **[Real-time Monitoring](/client-node/guides/real-time/)** - Subscribe to new emails
- **[API Reference: Inbox](/client-node/api/inbox/)** - Complete inbox API documentation
- **[Core Concepts: Inboxes](/client-node/concepts/inboxes/)** - Deep dive into inbox concepts
