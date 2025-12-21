---
title: Email Objects
description: Understanding email objects and their properties in VaultSandbox
---

Email objects in VaultSandbox represent decrypted emails with all their content, headers, and metadata.

## Email Structure

```javascript
const email = await inbox.waitForEmail({ timeout: 30000 });

console.log(email.id); // "email_abc123"
console.log(email.from); // "sender@example.com"
console.log(email.to); // ["recipient@mail.example.com"]
console.log(email.subject); // "Welcome to our service"
console.log(email.text); // Plain text content
console.log(email.html); // HTML content
console.log(email.receivedAt); // Date object
console.log(email.isRead); // false
console.log(email.links); // ["https://example.com/verify"]
console.log(email.attachments); // Array of attachments
console.log(email.authResults); // SPF/DKIM/DMARC results
```

## Core Properties

### id

**Type**: `string`

Unique identifier for the email.

```javascript
const emailId = email.id;
// Later...
const sameEmail = await inbox.getEmail(emailId);
```

### from

**Type**: `string`

Sender's email address (from the `From` header).

```javascript
console.log(email.from); // "noreply@example.com"

// Use in assertions
expect(email.from).toBe('support@example.com');
```

### to

**Type**: `string[]`

Array of recipient email addresses.

```javascript
console.log(email.to); // ["user@mail.example.com"]

// Multiple recipients
console.log(email.to); // ["user1@mail.example.com", "user2@mail.example.com"]

// Check if sent to specific address
expect(email.to).toContain(inbox.emailAddress);
```

### subject

**Type**: `string`

Email subject line.

```javascript
console.log(email.subject); // "Password Reset Request"

// Use in filtering
const email = await inbox.waitForEmail({
	subject: /Password Reset/,
});
```

### text

**Type**: `string | null`

Plain text content of the email.

```javascript
console.log(email.text);
// "Hello,\n\nClick here to reset your password:\nhttps://..."

// May be null if email is HTML-only
if (email.text) {
	expect(email.text).toContain('reset your password');
}
```

### html

**Type**: `string | null`

HTML content of the email.

```javascript
console.log(email.html);
// "<html><body><p>Hello,</p><a href='https://...'>Reset Password</a></body></html>"

// May be null if email is plain text only
if (email.html) {
	expect(email.html).toContain('<a href');
}
```

### receivedAt

**Type**: `Date`

When the email was received by the gateway.

```javascript
console.log(email.receivedAt); // Date: 2024-01-15T12:00:00.000Z

// Check if email arrived recently
const ageInSeconds = (new Date() - email.receivedAt) / 1000;
expect(ageInSeconds).toBeLessThan(60); // Received within last minute
```

### isRead

**Type**: `boolean`

Whether the email has been marked as read.

```javascript
console.log(email.isRead); // false

await email.markAsRead();

console.log(email.isRead); // true
```

### links

**Type**: `string[]`

All URLs extracted from the email (text and HTML).

```javascript
console.log(email.links);
// [
//   "https://example.com/verify?token=abc123",
//   "https://example.com/unsubscribe",
//   "https://example.com/privacy"
// ]

// Find specific link
const verifyLink = email.links.find((url) => url.includes('/verify'));
expect(verifyLink).toBeDefined();

// Test link
const response = await fetch(verifyLink);
expect(response.ok).toBe(true);
```

### attachments

**Type**: `AttachmentData[]`

Array of email attachments.

```javascript
console.log(email.attachments.length); // 2

email.attachments.forEach((att) => {
	console.log(att.filename); // "invoice.pdf"
	console.log(att.contentType); // "application/pdf"
	console.log(att.size); // 15234 bytes
	console.log(att.content); // Uint8Array
});
```

See [Working with Attachments](/client-node/guides/attachments/) for details.

### authResults

**Type**: `AuthResults`

Email authentication results (SPF, DKIM, DMARC, reverse DNS).

```javascript
const auth = email.authResults;

console.log(auth.spf?.status); // "pass"
console.log(auth.dkim?.length); // 1
console.log(auth.dmarc?.status); // "pass"

// Validate all checks
const validation = auth.validate();
if (!validation.passed) {
	console.error('Authentication failed:', validation.failures);
}
```

See [Authentication Results](/client-node/concepts/auth-results/) for details.

### headers

**Type**: `Record<string, unknown>`

All email headers as a key-value object.

```javascript
console.log(email.headers);
// {
//   "from": "noreply@example.com",
//   "to": "user@mail.example.com",
//   "subject": "Welcome",
//   "message-id": "<abc123@example.com>",
//   "date": "Mon, 15 Jan 2024 12:00:00 +0000",
//   "content-type": "text/html; charset=utf-8",
//   ...
// }

// Access specific headers
const messageId = email.headers['message-id'];
const contentType = email.headers['content-type'];
```

### metadata

**Type**: `Record<string, unknown>`

Additional metadata associated with the email.

```javascript
console.log(email.metadata);
// {
//   emailSizeBytes: 5432,
//   encryptedAt: "2024-01-15T12:00:00.000Z",
//   ...
// }
```

## Email Methods

### markAsRead()

Mark the email as read.

```javascript
await email.markAsRead();

console.log(email.isRead); // true
```

### delete()

Delete the email from the inbox.

```javascript
await email.delete();

// Email is now deleted
try {
	await inbox.getEmail(email.id);
} catch (error) {
	console.log('Email deleted'); // EmailNotFoundError
}
```

### getRaw()

Get the raw email source (decrypted MIME).

```javascript
const raw = await email.getRaw();

console.log(raw.id); // Email ID
console.log(raw.raw);
// "From: sender@example.com\r\nTo: recipient@example.com\r\n..."
```

## Common Patterns

### Content Validation

```javascript
const email = await inbox.waitForEmail({
	subject: /Welcome/,
	timeout: 10000,
});

// Validate sender
expect(email.from).toBe('noreply@example.com');

// Validate content
expect(email.text).toContain('Thank you for signing up');
expect(email.html).toContain('<h1>Welcome</h1>');

// Validate links
const verifyLink = email.links.find((url) => url.includes('/verify'));
expect(verifyLink).toBeDefined();
expect(verifyLink).toMatch(/^https:\/\//);
```

### Link Extraction and Testing

```javascript
const email = await inbox.waitForEmail({ subject: /Reset/ });

// Extract reset link
const resetLink = email.links.find((url) => url.includes('reset-password') || url.includes('token='));

expect(resetLink).toBeDefined();

// Extract token from link
const url = new URL(resetLink);
const token = url.searchParams.get('token');

expect(token).toBeTruthy();
expect(token.length).toBeGreaterThan(20);

// Test the link
const response = await fetch(resetLink);
expect(response.status).toBe(200);
```

### Multi-Part Emails

```javascript
// Email with both text and HTML
if (email.text && email.html) {
	// Validate both versions have key content
	expect(email.text).toContain('Welcome');
	expect(email.html).toContain('<h1>Welcome</h1>');
}

// HTML-only email
if (email.html && !email.text) {
	console.log('HTML-only email');
	expect(email.html).toContain('<!DOCTYPE html>');
}

// Plain text only
if (email.text && !email.html) {
	console.log('Plain text email');
}
```

### Time-Based Assertions

```javascript
const startTime = new Date();

// Trigger email
await sendWelcomeEmail(inbox.emailAddress);

// Wait and receive
const email = await inbox.waitForEmail({ timeout: 10000 });

// Verify it arrived quickly
const deliveryTime = (email.receivedAt - startTime) / 1000;
expect(deliveryTime).toBeLessThan(5); // Within 5 seconds
```

### Email Metadata Analysis

```javascript
console.log('Email details:');
console.log('- From:', email.from);
console.log('- Subject:', email.subject);
console.log('- Received:', email.receivedAt.toISOString());
console.log('- Size:', email.text?.length || 0, 'chars');
console.log('- Links:', email.links.length);
console.log('- Attachments:', email.attachments.length);

// Check email authentication
const auth = email.authResults.validate();
console.log('- Auth passed:', auth.passed);
if (!auth.passed) {
	console.log('- Auth failures:', auth.failures);
}
```

## Testing Examples

### Jest Example

```javascript
describe('Welcome Email', () => {
	let inbox, email;

	beforeEach(async () => {
		inbox = await client.createInbox();
	});

	afterEach(async () => {
		await inbox.delete();
	});

	test('should send welcome email on signup', async () => {
		await registerUser(inbox.emailAddress);

		email = await inbox.waitForEmail({
			subject: /Welcome/,
			timeout: 10000,
		});

		expect(email.from).toBe('noreply@example.com');
		expect(email.subject).toContain('Welcome');
		expect(email.text).toContain('Thank you for signing up');

		const verifyLink = email.links.find((url) => url.includes('/verify'));
		expect(verifyLink).toBeDefined();
	});

	test('should include unsubscribe link', async () => {
		await registerUser(inbox.emailAddress);

		email = await inbox.waitForEmail({ timeout: 10000 });

		const unsubLink = email.links.find((url) => url.includes('/unsubscribe') || url.includes('list-unsubscribe'));

		expect(unsubLink).toBeDefined();
	});
});
```

### Vitest Example

```javascript
import { describe, it, expect, beforeEach, afterEach } from 'vitest';

describe('Password Reset Flow', () => {
	let inbox;

	beforeEach(async () => {
		inbox = await client.createInbox();
	});

	afterEach(async () => {
		await inbox.delete();
	});

	it('sends reset email with valid token', async () => {
		await requestPasswordReset(inbox.emailAddress);

		const email = await inbox.waitForEmail({
			subject: /reset/i,
			timeout: 10000,
		});

		expect(email.from).toBe('security@example.com');

		const resetLink = email.links[0];
		expect(resetLink).toMatch(/^https:\/\//);
		expect(resetLink).toContain('token=');

		// Verify token format
		const url = new URL(resetLink);
		const token = url.searchParams.get('token');
		expect(token).toHaveLength(64);
	});
});
```

## Troubleshooting

### Email Content is Null

```javascript
if (!email.text && !email.html) {
	console.error('Email has no content');
	console.log('Headers:', email.headers);
	console.log('Raw:', await email.getRaw());
}
```

### Links Not Extracted

```javascript
if (email.links.length === 0) {
	console.log('No links found');
	console.log('Text:', email.text);
	console.log('HTML:', email.html);

	// Manually extract
	const urlRegex = /https?:\/\/[^\s]+/g;
	const textLinks = email.text?.match(urlRegex) || [];
	console.log('Manual extraction:', textLinks);
}
```

### Decryption Errors

```javascript
try {
	const email = await inbox.getEmail(emailId);
} catch (error) {
	if (error instanceof DecryptionError) {
		console.error('Failed to decrypt email');
		console.error('This may indicate:');
		console.error('- Wrong private key');
		console.error('- Corrupted data');
		console.error('- Server issue');
	}
}
```

## Next Steps

- **[Authentication Results](/client-node/concepts/auth-results/)** - Email authentication details
- **[Working with Attachments](/client-node/guides/attachments/)** - Handle email attachments
- **[Email Authentication](/client-node/guides/authentication/)** - Test SPF/DKIM/DMARC
- **[API Reference: Email](/client-node/api/email/)** - Complete API documentation
