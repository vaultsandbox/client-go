---
title: Email API
description: Complete API reference for the Email class and related types
---

The `Email` class represents a decrypted email message in VaultSandbox. All emails are automatically decrypted when retrieved, so you can access content, headers, links, and attachments directly.

## Properties

### id

```typescript
id: string;
```

Unique identifier for this email. Use this to reference the email in API calls.

#### Example

```javascript
const emails = await inbox.listEmails();
console.log(`Email ID: ${emails[0].id}`);

// Get specific email
const email = await inbox.getEmail(emails[0].id);
```

---

### from

```typescript
from: string;
```

The sender's email address.

#### Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });
console.log(`From: ${email.from}`);

expect(email.from).toBe('noreply@example.com');
```

---

### to

```typescript
to: string[]
```

Array of recipient email addresses.

#### Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });
console.log(`To: ${email.to.join(', ')}`);

// Check if specific recipient is included
expect(email.to).toContain(inbox.emailAddress);
```

---

### subject

```typescript
subject: string;
```

The email subject line.

#### Example

```javascript
const email = await inbox.waitForEmail({
	timeout: 10000,
	subject: /Welcome/,
});

console.log(`Subject: ${email.subject}`);
expect(email.subject).toContain('Welcome');
```

---

### text

```typescript
text: string | null;
```

Plain text content of the email. May be `null` if the email only has HTML content.

#### Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });

if (email.text) {
	console.log('Plain text version:');
	console.log(email.text);

	// Validate content
	expect(email.text).toContain('Thank you for signing up');
}
```

---

### html

```typescript
html: string | null;
```

HTML content of the email. May be `null` if the email only has plain text.

#### Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });

if (email.html) {
	console.log('HTML version present');

	// Validate HTML structure
	expect(email.html).toContain('<a href=');
	expect(email.html).toContain('</html>');

	// Check for specific elements
	expect(email.html).toMatch(/<img[^>]+src="/);
}
```

---

### receivedAt

```typescript
receivedAt: Date;
```

The date and time when the email was received by VaultSandbox.

#### Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });
console.log(`Received at: ${email.receivedAt.toISOString()}`);

// Check if email was received recently
const now = Date.now();
const received = email.receivedAt.getTime();
const ageInSeconds = (now - received) / 1000;

expect(ageInSeconds).toBeLessThan(60); // Within last minute
```

---

### isRead

```typescript
isRead: boolean;
```

Whether this email has been marked as read.

#### Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });
console.log(`Read status: ${email.isRead}`);

// Mark as read
await email.markAsRead();

// Verify status changed
const updated = await inbox.getEmail(email.id);
expect(updated.isRead).toBe(true);
```

---

### links

```typescript
links: string[]
```

All URLs automatically extracted from the email content (both text and HTML).

#### Example

```javascript
const email = await inbox.waitForEmail({
	timeout: 10000,
	subject: /Password Reset/,
});

console.log(`Found ${email.links.length} links:`);
email.links.forEach((url) => console.log(`  - ${url}`));

// Find specific link
const resetLink = email.links.find((url) => url.includes('/reset-password'));
expect(resetLink).toBeDefined();
expect(resetLink).toMatch(/^https:\/\//);

// Extract query parameters
const url = new URL(resetLink);
const token = url.searchParams.get('token');
expect(token).toBeTruthy();
```

---

### headers

```typescript
headers: Record<string, unknown>;
```

All email headers as a key-value object.

#### Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });

console.log('Headers:');
console.log(`  Content-Type: ${email.headers['content-type']}`);
console.log(`  Message-ID: ${email.headers['message-id']}`);

// Check for custom headers
if (email.headers['x-custom-header']) {
	console.log(`Custom header: ${email.headers['x-custom-header']}`);
}
```

---

### attachments

```typescript
attachments: AttachmentData[]
```

Array of email attachments, automatically decrypted and ready to use.

```typescript
interface AttachmentData {
	filename: string;
	contentType: string;
	size: number;
	contentId?: string;
	contentDisposition?: string;
	checksum?: string;
	content?: Uint8Array;
}
```

#### Example

```javascript
const email = await inbox.waitForEmail({
	timeout: 10000,
	subject: /Invoice/,
});

console.log(`Attachments: ${email.attachments.length}`);

email.attachments.forEach((attachment) => {
	console.log(`  - ${attachment.filename} (${attachment.size} bytes)`);
	console.log(`    Type: ${attachment.contentType}`);
});

// Find PDF attachment
const pdf = email.attachments.find((att) => att.contentType === 'application/pdf');
if (pdf && pdf.content) {
	fs.writeFileSync(`./downloads/${pdf.filename}`, pdf.content);
	console.log(`Saved ${pdf.filename}`);
}

// Process text attachment
const textFile = email.attachments.find((att) => att.contentType.includes('text'));
if (textFile && textFile.content) {
	const text = new TextDecoder().decode(textFile.content);
	console.log('Text content:', text);
}

// Parse JSON attachment
const jsonFile = email.attachments.find((att) => att.contentType.includes('json'));
if (jsonFile && jsonFile.content) {
	const json = new TextDecoder().decode(jsonFile.content);
	const data = JSON.parse(json);
	console.log('JSON data:', data);
}
```

See the [Attachments Guide](/client-node/guides/attachments) for more examples.

---

### authResults

```typescript
authResults: AuthResults;
```

Email authentication results including SPF, DKIM, and DMARC validation.

```typescript
interface AuthResults {
	spf?: SPFResult;
	dkim?: DKIMResult[];
	dmarc?: DMARCResult;
	reverseDns?: ReverseDNSResult;
	validate(): AuthValidation;
}
```

#### Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });

// Validate all authentication
const validation = email.authResults.validate();
console.log(`Authentication passed: ${validation.passed}`);

if (!validation.passed) {
	console.log('Failures:');
	validation.failures.forEach((failure) => console.log(`  - ${failure}`));
}

// Check individual results
if (email.authResults.spf) {
	console.log(`SPF Status: ${email.authResults.spf.status}`);
}

if (email.authResults.dkim && email.authResults.dkim.length > 0) {
	console.log(`DKIM Status: ${email.authResults.dkim[0].status}`);
}

if (email.authResults.dmarc) {
	console.log(`DMARC Status: ${email.authResults.dmarc.status}`);
}
```

See the [Authentication Guide](/client-node/guides/authentication) for more details.

---

### metadata

```typescript
metadata: Record<string, unknown>;
```

Additional metadata associated with the email.

#### Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });

if (email.metadata) {
	console.log('Metadata:', email.metadata);
}
```

## Methods

### markAsRead()

Marks this email as read.

```typescript
markAsRead(): Promise<void>
```

#### Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });

console.log(`Read status: ${email.isRead}`); // false

await email.markAsRead();
console.log('Marked as read');

// Verify status changed
const updated = await inbox.getEmail(email.id);
expect(updated.isRead).toBe(true);
```

---

### delete()

Deletes this email from the inbox.

```typescript
delete(): Promise<void>
```

#### Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });

// Delete the email
await email.delete();
console.log('Email deleted');

// Verify deletion
const emails = await inbox.listEmails();
expect(emails.find((e) => e.id === email.id)).toBeUndefined();
```

---

### getRaw()

Gets the raw MIME source of this email (decrypted).

```typescript
getRaw(): Promise<RawEmail>
```

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
const email = await inbox.waitForEmail({ timeout: 10000 });
const raw = await email.getRaw();

console.log('Raw MIME source:');
console.log(raw.raw);

// Save to .eml file
fs.writeFileSync(`email-${email.id}.eml`, raw.raw);
```

## AuthResults

The `AuthResults` object provides email authentication validation.

### Properties

#### spf

```typescript
spf?: SPFResult
```

SPF (Sender Policy Framework) validation result.

```typescript
interface SPFResult {
	status: 'pass' | 'fail' | 'softfail' | 'neutral' | 'none' | 'temperror' | 'permerror';
	domain?: string;
	ip?: string;
	info?: string;
}
```

#### dkim

```typescript
dkim?: DKIMResult[]
```

DKIM (DomainKeys Identified Mail) validation results. May have multiple signatures.

```typescript
interface DKIMResult {
	status: 'pass' | 'fail' | 'none';
	domain?: string;
	selector?: string;
	info?: string;
}
```

#### dmarc

```typescript
dmarc?: DMARCResult
```

DMARC (Domain-based Message Authentication) validation result.

```typescript
interface DMARCResult {
	status: 'pass' | 'fail' | 'none';
	policy?: 'none' | 'quarantine' | 'reject';
	aligned?: boolean;
	domain?: string;
	info?: string;
}
```

#### reverseDns

```typescript
reverseDns?: ReverseDNSResult
```

Reverse DNS lookup result.

```typescript
interface ReverseDNSResult {
	status: 'pass' | 'fail' | 'none';
	ip?: string;
	hostname?: string;
	info?: string;
}
```

### Methods

#### validate()

Validates all authentication results and returns a summary.

```typescript
validate(): AuthValidation
```

##### Returns

```typescript
interface AuthValidation {
	passed: boolean;
	spfPassed: boolean;
	dkimPassed: boolean;
	dmarcPassed: boolean;
	reverseDnsPassed: boolean;
	failures: string[];
}
```

##### Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });
const validation = email.authResults.validate();

console.log(`Overall: ${validation.passed ? 'PASS' : 'FAIL'}`);
console.log(`SPF: ${validation.spfPassed ? 'PASS' : 'FAIL'}`);
console.log(`DKIM: ${validation.dkimPassed ? 'PASS' : 'FAIL'}`);
console.log(`DMARC: ${validation.dmarcPassed ? 'PASS' : 'FAIL'}`);

if (!validation.passed) {
	console.log('\nFailures:');
	validation.failures.forEach((failure) => {
		console.log(`  - ${failure}`);
	});
}
```

## Complete Example

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';
import fs from 'fs';

async function completeEmailExample() {
	const client = new VaultSandboxClient({
		url: process.env.VAULTSANDBOX_URL,
		apiKey: process.env.VAULTSANDBOX_API_KEY,
	});

	try {
		const inbox = await client.createInbox();
		console.log(`Created inbox: ${inbox.emailAddress}`);

		// Trigger test email
		await sendTestEmail(inbox.emailAddress);

		// Wait for email
		const email = await inbox.waitForEmail({
			timeout: 10000,
			subject: /Test/,
		});

		// Basic info
		console.log('\n=== Email Details ===');
		console.log(`ID: ${email.id}`);
		console.log(`From: ${email.from}`);
		console.log(`To: ${email.to.join(', ')}`);
		console.log(`Subject: ${email.subject}`);
		console.log(`Received: ${email.receivedAt.toISOString()}`);
		console.log(`Read: ${email.isRead}`);

		// Content
		console.log('\n=== Content ===');
		if (email.text) {
			console.log('Plain text:');
			console.log(email.text.substring(0, 200) + '...');
		}
		if (email.html) {
			console.log('HTML version present');
		}

		// Links
		console.log('\n=== Links ===');
		console.log(`Found ${email.links.length} links:`);
		email.links.forEach((link) => console.log(`  - ${link}`));

		// Attachments
		console.log('\n=== Attachments ===');
		console.log(`Found ${email.attachments.length} attachments:`);
		email.attachments.forEach((att) => {
			console.log(`  - ${att.filename} (${att.contentType}, ${att.size} bytes)`);

			// Save attachment
			if (att.content) {
				fs.writeFileSync(`./downloads/${att.filename}`, att.content);
				console.log(`    Saved to ./downloads/${att.filename}`);
			}
		});

		// Authentication
		console.log('\n=== Authentication ===');
		const validation = email.authResults.validate();
		console.log(`Overall: ${validation.passed ? 'PASS' : 'FAIL'}`);
		console.log(`SPF: ${validation.spfPassed}`);
		console.log(`DKIM: ${validation.dkimPassed}`);
		console.log(`DMARC: ${validation.dmarcPassed}`);

		if (!validation.passed) {
			console.log('Failures:', validation.failures);
		}

		// Mark as read
		await email.markAsRead();
		console.log('\nMarked as read');

		// Get raw source
		const raw = await email.getRaw();
		fs.writeFileSync(`email-${email.id}.eml`, raw.raw);
		console.log(`Saved raw source to email-${email.id}.eml`);

		// Clean up
		await inbox.delete();
	} finally {
		await client.close();
	}
}

completeEmailExample().catch(console.error);
```

## Next Steps

- [Inbox API Reference](/client-node/api/inbox) - Learn about inbox methods
- [Attachments Guide](/client-node/guides/attachments) - Working with attachments
- [Authentication Guide](/client-node/guides/authentication) - Email authentication testing
- [Waiting for Emails](/client-node/guides/waiting-for-emails) - Best practices for email waiting
