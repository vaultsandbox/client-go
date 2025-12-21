---
title: Inboxes
description: Understanding VaultSandbox inboxes and how to work with them
---

Inboxes are the core concept in VaultSandbox. Each inbox is an isolated, encrypted email destination with its own unique address and encryption keys.

## What is an Inbox?

An inbox is a temporary, encrypted email destination that:

- Has a **unique email address** (e.g., `a1b2c3d4@mail.example.com`)
- Uses **client-side encryption** (ML-KEM-768 keypair)
- **Expires automatically** after a configurable time-to-live (TTL)
- Is **isolated** from other inboxes
- Stores emails **in memory** on the gateway

## Creating Inboxes

### Basic Creation

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

const client = new VaultSandboxClient({ url, apiKey });
const inbox = await client.createInbox();

console.log(inbox.emailAddress); // "a1b2c3d4@mail.example.com"
console.log(inbox.inboxHash); // "a1b2c3d4"
console.log(inbox.expiresAt); // Date object
```

### With Options

```javascript
const inbox = await client.createInbox({
	ttl: 3600, // 1 hour (default: 24 hours)
	emailAddress: 'test@mail.example.com', // Request specific address
});
```

**Note**: Requesting a specific email address may fail if it's already in use. The server will return an error.

## Inbox Properties

### emailAddress

**Type**: `string`

The full email address for this inbox.

```javascript
console.log(inbox.emailAddress);
// "a1b2c3d4@mail.example.com"
```

Send emails to this address to have them appear in the inbox.

### inboxHash

**Type**: `string`

A unique cryptographic hash identifier for the inbox. This is used internally for encryption and identification purposes.

```javascript
console.log(inbox.inboxHash);
// "Rr02MLnP7F0pRVC6QdcpSIeyklqu3PDkYglvsfN7Oss"
```

**Note**: This is not the same as the local part of the email address. The email address local part (e.g., `a1b2c3d4` in `a1b2c3d4@mail.example.com`) is different from the `inboxHash`.

### expiresAt

**Type**: `Date`

When the inbox will automatically expire and be deleted.

```javascript
console.log(inbox.expiresAt);
// Date: 2024-01-16T12:00:00.000Z

// Check if inbox is expiring soon
const hoursUntilExpiry = (inbox.expiresAt - new Date()) / 1000 / 60 / 60;
console.log(`Expires in ${hoursUntilExpiry} hours`);
```

## Inbox Lifecycle

```
┌─────────────────────────────────────────────────────────┐
│                  Inbox Lifecycle                        │
└─────────────────────────────────────────────────────────┘

1. Creation
   client.createInbox() → Inbox object
   ↓
   - Keypair generated client-side
   - Public key sent to server
   - Unique email address assigned
   - TTL timer starts

2. Active
   ↓
   - Receive emails
   - List/read emails
   - Wait for emails
   - Monitor for new emails

3. Expiration (TTL reached) or Manual Deletion
   ↓
   inbox.delete() or TTL expires
   - All emails deleted
   - Inbox address freed
   - Keypair destroyed
```

## Working with Inboxes

### Listing Emails

```javascript
const emails = await inbox.listEmails();

console.log(`${emails.length} emails in inbox`);
emails.forEach((email) => {
	console.log(`${email.from}: ${email.subject}`);
});
```

### Getting a Specific Email

```javascript
const email = await inbox.getEmail('email-id-123');

console.log(email.subject);
console.log(email.text);
```

### Waiting for Emails

```javascript
// Wait for any email
const email = await inbox.waitForEmail({ timeout: 30000 });

// Wait for specific email
const email = await inbox.waitForEmail({
	timeout: 30000,
	subject: /Password Reset/,
	from: 'noreply@example.com',
});
```

### Deleting Emails

```javascript
// Delete specific email
await inbox.deleteEmail('email-id-123');

// Or via email object
await email.delete();
```

### Deleting Inbox

```javascript
// Delete inbox and all its emails
await inbox.delete();
```

## Inbox Isolation

Each inbox is completely isolated:

```javascript
const inbox1 = await client.createInbox();
const inbox2 = await client.createInbox();

// inbox1 cannot access inbox2's emails
// inbox2 cannot access inbox1's emails

// Each has its own:
// - Email address
// - Encryption keys
// - Email storage
// - Expiration time
```

## Time-to-Live (TTL)

Inboxes automatically expire after their TTL:

### Default TTL

```javascript
// Uses server's DEFAULT_INBOX_TTL (typically 24 hours)
const inbox = await client.createInbox();
```

### Custom TTL

```javascript
// Expire after 1 hour
const inbox = await client.createInbox({ ttl: 3600 });

// Expire after 10 minutes (useful for quick tests)
const inbox = await client.createInbox({ ttl: 600 });

// Expire after 7 days
const inbox = await client.createInbox({ ttl: 604800 });
```

### Checking Expiration

```javascript
const minutesLeft = (inbox.expiresAt - new Date()) / 1000 / 60;

if (minutesLeft < 5) {
	console.warn('Inbox expiring soon!');
}
```

## Import and Export

Inboxes can be exported and imported for:

- Test reproducibility
- Sharing between environments
- Backup and restore

### Export

```javascript
const exportData = inbox.export();

// Save to file
fs.writeFileSync('inbox.json', JSON.stringify(exportData));
```

### Import

```javascript
const exportData = JSON.parse(fs.readFileSync('inbox.json', 'utf8'));
const inbox = await client.importInbox(exportData);

// Inbox restored with all encryption keys
```

**Security Warning**: Exported data contains private keys. Treat as sensitive.

## Best Practices

### CI/CD Pipelines

**Short TTL for fast cleanup**:

```javascript
const inbox = await client.createInbox({ ttl: 3600 }); // 1 hour
```

**Always clean up**:

```javascript
try {
	const inbox = await client.createInbox();
	// Run tests
} finally {
	await inbox.delete();
}
```

### Manual Testing

**Longer TTL for convenience**:

```javascript
const inbox = await client.createInbox({ ttl: 86400 }); // 24 hours
```

**Export for reuse**:

```javascript
// Export after creating
const exportData = inbox.export();
fs.writeFileSync('test-inbox.json', JSON.stringify(exportData));

// Reuse in later sessions
const inbox = await client.importInbox(exportData);
```

### Production Monitoring

**Monitor expiration**:

```javascript
setInterval(() => {
	const minutesLeft = (inbox.expiresAt - new Date()) / 1000 / 60;
	if (minutesLeft < 10) {
		console.warn(`Inbox ${inbox.emailAddress} expiring in ${minutesLeft} minutes`);
	}
}, 60000); // Check every minute
```

## Common Patterns

### Dedicated Test Inbox

```javascript
let testInbox;

beforeAll(async () => {
	testInbox = await client.createInbox({ ttl: 7200 }); // 2 hours
});

afterAll(async () => {
	await testInbox.delete();
});

test('password reset', async () => {
	await triggerPasswordReset(testInbox.emailAddress);
	const email = await testInbox.waitForEmail({ timeout: 10000 });
	// ...
});
```

### Multiple Inboxes

```javascript
const user1Inbox = await client.createInbox();
const user2Inbox = await client.createInbox();
const adminInbox = await client.createInbox();

// Each inbox receives emails independently
await sendWelcomeEmail(user1Inbox.emailAddress);
await sendWelcomeEmail(user2Inbox.emailAddress);
await sendAdminReport(adminInbox.emailAddress);
```

### Inbox Pool

```javascript
class InboxPool {
	constructor(client, size = 5) {
		this.client = client;
		this.pool = [];
		this.size = size;
	}

	async initialize() {
		for (let i = 0; i < this.size; i++) {
			const inbox = await this.client.createInbox();
			this.pool.push(inbox);
		}
	}

	get() {
		return this.pool.shift();
	}

	async cleanup() {
		await Promise.all(this.pool.map((inbox) => inbox.delete()));
	}
}
```

## Troubleshooting

### Inbox Not Receiving Emails

**Check**:

1. Email is sent to correct address
2. Inbox hasn't expired
3. DNS/MX records configured correctly
4. SMTP connection successful

```javascript
// Verify inbox still exists
const emails = await inbox.listEmails(); // Will error if inbox expired
```

### Inbox Already Exists Error

When requesting a specific email address:

```javascript
try {
	const inbox = await client.createInbox({
		emailAddress: 'test@mail.example.com',
	});
} catch (error) {
	if (error instanceof InboxAlreadyExistsError) {
		// Address already in use, generate random instead
		const inbox = await client.createInbox();
	}
}
```

### Inbox Expired

```javascript
try {
	const emails = await inbox.listEmails();
} catch (error) {
	if (error instanceof InboxNotFoundError) {
		console.error('Inbox has expired');
		// Create new inbox
		const newInbox = await client.createInbox();
	}
}
```

## Next Steps

- **[Email Objects](/client-node/concepts/emails/)** - Learn about email structure
- **[Managing Inboxes](/client-node/guides/managing-inboxes/)** - Common inbox operations
- **[Import/Export](/client-node/advanced/import-export/)** - Advanced inbox persistence
- **[API Reference: Inbox](/client-node/api/inbox/)** - Complete API documentation
