---
title: Inbox API
description: Complete API reference for the Inbox class and InboxMonitor
---

The `Inbox` class represents a single email inbox in VaultSandbox. It provides methods for managing emails, waiting for new messages, and monitoring in real-time.

## Properties

### emailAddress

```typescript
emailAddress: string;
```

The email address for this inbox. Use this address to send test emails.

#### Example

```javascript
const inbox = await client.createInbox();
console.log(`Send email to: ${inbox.emailAddress}`);

// Use in your application
await sendWelcomeEmail(inbox.emailAddress);
```

---

### inboxHash

```typescript
inboxHash: string;
```

Unique identifier for this inbox. Used internally for API operations.

#### Example

```javascript
console.log(`Inbox ID: ${inbox.inboxHash}`);
```

---

### expiresAt

```typescript
expiresAt: Date;
```

The date and time when this inbox will expire and be automatically deleted.

#### Example

```javascript
const inbox = await client.createInbox();
console.log(`Inbox expires at: ${inbox.expiresAt.toISOString()}`);

const timeUntilExpiry = inbox.expiresAt.getTime() - Date.now();
console.log(`Time remaining: ${Math.round(timeUntilExpiry / 1000)}s`);
```

## Methods

### listEmails()

Lists all emails in the inbox. Emails are automatically decrypted.

```typescript
listEmails(): Promise<Email[]>
```

#### Returns

`Promise<Email[]>` - Array of decrypted email objects, sorted by received time (newest first)

#### Example

```javascript
const emails = await inbox.listEmails();

console.log(`Inbox has ${emails.length} emails`);

emails.forEach((email) => {
	console.log(`- ${email.subject} from ${email.from}`);
});
```

---

### getEmail()

Retrieves a specific email by ID.

```typescript
getEmail(emailId: string): Promise<Email>
```

#### Parameters

- `emailId`: The unique identifier for the email

#### Returns

`Promise<Email>` - The decrypted email object

#### Example

```javascript
const emails = await inbox.listEmails();
const firstEmail = await inbox.getEmail(emails[0].id);

console.log(`Subject: ${firstEmail.subject}`);
console.log(`Body: ${firstEmail.text}`);
```

#### Errors

- `EmailNotFoundError` - Email does not exist

---

### waitForEmail()

Waits for an email matching specified criteria. This is the recommended way to handle email arrival in tests.

```typescript
waitForEmail(options: WaitOptions): Promise<Email>
```

#### Parameters

```typescript
interface WaitOptions {
	timeout?: number;
	pollInterval?: number;
	subject?: string | RegExp;
	from?: string | RegExp;
	predicate?: (email: Email) => boolean;
}
```

| Property       | Type                        | Default | Description                          |
| -------------- | --------------------------- | ------- | ------------------------------------ |
| `timeout`      | `number`                    | `30000` | Maximum time to wait in milliseconds |
| `pollInterval` | `number`                    | `2000`  | Polling interval in milliseconds     |
| `subject`      | `string \| RegExp`          | -       | Filter by email subject              |
| `from`         | `string \| RegExp`          | -       | Filter by sender address             |
| `predicate`    | `(email: Email) => boolean` | -       | Custom filter function               |

#### Returns

`Promise<Email>` - The first email matching the criteria

#### Examples

```javascript
// Wait for any email
const email = await inbox.waitForEmail({ timeout: 10000 });

// Wait for email with specific subject
const email = await inbox.waitForEmail({
	timeout: 10000,
	subject: /Password Reset/,
});

// Wait for email from specific sender
const email = await inbox.waitForEmail({
	timeout: 10000,
	from: 'noreply@example.com',
});

// Wait with custom predicate
const email = await inbox.waitForEmail({
	timeout: 15000,
	predicate: (email) => email.to.includes('user@example.com'),
});

// Combine multiple filters
const email = await inbox.waitForEmail({
	timeout: 10000,
	subject: /Welcome/,
	from: /noreply@/,
	predicate: (email) => email.links.length > 0,
});
```

#### Errors

- `TimeoutError` - No matching email received within timeout period

---

### waitForEmailCount()

Waits until the inbox has at least the specified number of emails. More efficient than using arbitrary timeouts when testing multiple emails.

```typescript
waitForEmailCount(count: number, options?: WaitForCountOptions): Promise<void>
```

#### Parameters

- `count`: Minimum number of emails to wait for

```typescript
interface WaitForCountOptions {
	timeout?: number;
}
```

| Property  | Type     | Default | Description                          |
| --------- | -------- | ------- | ------------------------------------ |
| `timeout` | `number` | `30000` | Maximum time to wait in milliseconds |

#### Returns

`Promise<void>` - Resolves when count is reached

#### Example

```javascript
// Trigger multiple emails
await sendMultipleNotifications(inbox.emailAddress, 3);

// Wait for all 3 to arrive
await inbox.waitForEmailCount(3, { timeout: 30000 });

// Now process all emails
const emails = await inbox.listEmails();
expect(emails.length).toBe(3);
```

#### Errors

- `TimeoutError` - Required count not reached within timeout

---

### onNewEmail()

Subscribes to new emails in real-time. Receives a callback for each new email that arrives.

```typescript
onNewEmail(callback: (email: Email) => void): Subscription
```

#### Parameters

- `callback`: Function called when a new email arrives

#### Returns

`Subscription` - Subscription object with `unsubscribe()` method

```typescript
interface Subscription {
	unsubscribe(): void;
}
```

#### Example

```javascript
const inbox = await client.createInbox();

console.log(`Monitoring: ${inbox.emailAddress}`);

// Subscribe to new emails
const subscription = inbox.onNewEmail((email) => {
	console.log(`New email: "${email.subject}"`);
	console.log(`From: ${email.from}`);

	// Process email...
});

// Later, stop monitoring
subscription.unsubscribe();
```

#### Best Practice

Always unsubscribe when done to avoid memory leaks:

```javascript
let subscription;

beforeEach(async () => {
	inbox = await client.createInbox();
	subscription = inbox.onNewEmail((email) => {
		// Handle email
	});
});

afterEach(async () => {
	if (subscription) {
		subscription.unsubscribe();
	}
	if (inbox) {
		await inbox.delete();
	}
});
```

---

### getSyncStatus()

Gets the current synchronization status of the inbox with the server.

```typescript
getSyncStatus(): Promise<SyncStatus>
```

#### Returns

`Promise<SyncStatus>` - Sync status information

```typescript
interface SyncStatus {
	emailCount: number;
	emailsHash: string;
}
```

#### Example

```javascript
const status = await inbox.getSyncStatus();
console.log(`Email count: ${status.emailCount}`);
console.log(`Emails hash: ${status.emailsHash}`);
```

---

### getRawEmail()

Gets the raw, decrypted source of a specific email (original MIME format).

```typescript
getRawEmail(emailId: string): Promise<RawEmail>
```

#### Parameters

- `emailId`: The unique identifier for the email

#### Returns

`Promise<RawEmail>` - Raw email source

```typescript
interface RawEmail {
	id: string;
	raw: string;
}
```

#### Example

```javascript
const emails = await inbox.listEmails();
const raw = await inbox.getRawEmail(emails[0].id);

console.log('Raw MIME source:');
console.log(raw.raw);

// Save to file for debugging
fs.writeFileSync('email.eml', raw.raw);
```

---

### markEmailAsRead()

Marks a specific email as read.

```typescript
markEmailAsRead(emailId: string): Promise<void>
```

#### Parameters

- `emailId`: The unique identifier for the email

#### Example

```javascript
const emails = await inbox.listEmails();
await inbox.markEmailAsRead(emails[0].id);

console.log('Email marked as read');
```

---

### deleteEmail()

Deletes a specific email from the inbox.

```typescript
deleteEmail(emailId: string): Promise<void>
```

#### Parameters

- `emailId`: The unique identifier for the email

#### Example

```javascript
const emails = await inbox.listEmails();

// Delete first email
await inbox.deleteEmail(emails[0].id);

console.log('Email deleted');

// Verify deletion
const updated = await inbox.listEmails();
expect(updated.length).toBe(emails.length - 1);
```

---

### delete()

Deletes this inbox and all its emails.

```typescript
delete(): Promise<void>
```

#### Example

```javascript
const inbox = await client.createInbox();

// Use inbox...

// Clean up
await inbox.delete();
console.log('Inbox deleted');
```

#### Best Practice

Always delete inboxes after tests:

```javascript
afterEach(async () => {
	if (inbox) {
		await inbox.delete();
	}
});
```

---

### export()

Exports inbox data and encryption keys for backup or sharing.

```typescript
export(): ExportedInboxData
```

#### Returns

`ExportedInboxData` - Serializable inbox data including sensitive keys

```typescript
interface ExportedInboxData {
	emailAddress: string;
	inboxHash: string;
	expiresAt: string;
	serverSigPk: string;
	publicKeyB64: string;
	secretKeyB64: string;
	exportedAt: string;
}
```

#### Example

```javascript
const inbox = await client.createInbox();
const data = inbox.export();

// Save for later
fs.writeFileSync('inbox-backup.json', JSON.stringify(data, null, 2));
```

#### Security Warning

Exported data contains private encryption keys. Store securely!

## InboxMonitor

The `InboxMonitor` class allows you to monitor multiple inboxes simultaneously.

### Creating a Monitor

```javascript
const inbox1 = await client.createInbox();
const inbox2 = await client.createInbox();

const monitor = client.monitorInboxes([inbox1, inbox2]);
```

### Events

#### email

Emitted when a new email arrives in any monitored inbox.

```typescript
on(event: 'email', listener: (inbox: Inbox, email: Email) => void): this
```

##### Parameters

- `inbox`: The inbox that received the email
- `email`: The email that was received

##### Example

```javascript
monitor.on('email', (inbox, email) => {
	console.log(`Email received in ${inbox.emailAddress}`);
	console.log(`Subject: ${email.subject}`);
});
```

### Methods

#### unsubscribe()

Stops monitoring all inboxes and cleans up resources.

```typescript
unsubscribe(): void
```

##### Example

```javascript
const monitor = client.monitorInboxes([inbox1, inbox2]);

// Use monitor...

// Stop monitoring
monitor.unsubscribe();
```

### Complete Example

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

async function monitorMultipleInboxes() {
	const client = new VaultSandboxClient({ url, apiKey });

	// Create multiple inboxes
	const inbox1 = await client.createInbox();
	const inbox2 = await client.createInbox();

	console.log(`Inbox 1: ${inbox1.emailAddress}`);
	console.log(`Inbox 2: ${inbox2.emailAddress}`);

	// Monitor both inboxes
	const monitor = client.monitorInboxes([inbox1, inbox2]);

	monitor.on('email', (inbox, email) => {
		console.log(`\nNew email in ${inbox.emailAddress}:`);
		console.log(`  Subject: ${email.subject}`);
		console.log(`  From: ${email.from}`);
	});

	// Wait for emails to arrive...
	await new Promise((resolve) => setTimeout(resolve, 60000));

	// Clean up
	monitor.unsubscribe();
	await inbox1.delete();
	await inbox2.delete();
}

monitorMultipleInboxes().catch(console.error);
```

## Complete Inbox Example

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

async function completeInboxExample() {
	const client = new VaultSandboxClient({
		url: process.env.VAULTSANDBOX_URL,
		apiKey: process.env.VAULTSANDBOX_API_KEY,
	});

	try {
		// Create inbox
		const inbox = await client.createInbox();
		console.log(`Created: ${inbox.emailAddress}`);
		console.log(`Expires: ${inbox.expiresAt.toISOString()}`);

		// Subscribe to new emails
		const subscription = inbox.onNewEmail((email) => {
			console.log(`Received: ${email.subject}`);
		});

		// Trigger test email
		await sendTestEmail(inbox.emailAddress);

		// Wait for specific email
		const email = await inbox.waitForEmail({
			timeout: 10000,
			subject: /Test/,
		});

		console.log(`Found email: ${email.subject}`);
		console.log(`Body: ${email.text}`);

		// Mark as read
		await inbox.markEmailAsRead(email.id);

		// Get all emails
		const allEmails = await inbox.listEmails();
		console.log(`Total emails: ${allEmails.length}`);

		// Export inbox
		const exportData = inbox.export();
		fs.writeFileSync('inbox.json', JSON.stringify(exportData));

		// Clean up
		subscription.unsubscribe();
		await inbox.delete();
	} finally {
		await client.close();
	}
}

completeInboxExample().catch(console.error);
```

## Next Steps

- [Email API Reference](/client-node/api/email) - Work with email objects
- [VaultSandboxClient API](/client-node/api/client) - Learn about client methods
- [Waiting for Emails Guide](/client-node/guides/waiting-for-emails) - Best practices
- [Real-time Monitoring Guide](/client-node/guides/real-time) - Advanced monitoring patterns
