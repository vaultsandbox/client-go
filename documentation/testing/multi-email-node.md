---
title: Testing Multi-Email Scenarios
description: Learn how to test scenarios involving multiple emails with VaultSandbox
---

Many real-world email flows involve sending multiple emails in sequence or in parallel. VaultSandbox provides efficient methods for testing these scenarios without using arbitrary timeouts.

## Waiting for Multiple Emails

The `waitForEmailCount()` method is the recommended way to test scenarios that send multiple emails. It's more efficient and reliable than using arbitrary timeouts.

### Basic Example

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,
});

const inbox = await client.createInbox();

// Send multiple emails
await sendNotifications(inbox.emailAddress, 3);

// Wait for all 3 emails to arrive (polls every 1s by default)
await inbox.waitForEmailCount(3, { timeout: 30000 });

// Now list and verify all emails
const emails = await inbox.listEmails();
expect(emails.length).toBe(3);
expect(emails[0].subject).toContain('Notification');

await inbox.delete();
```

## Testing Email Sequences

Test workflows that send emails in a specific sequence:

```javascript
describe('Welcome Email Sequence', () => {
	let client;
	let inbox;

	beforeEach(async () => {
		client = new VaultSandboxClient({
			url: process.env.VAULTSANDBOX_URL,
			apiKey: process.env.VAULTSANDBOX_API_KEY,
		});
		inbox = await client.createInbox();
	});

	afterEach(async () => {
		await inbox?.delete();
	});

	test('should send complete onboarding sequence', async () => {
		// Trigger user registration
		await registerUser(inbox.emailAddress);

		// Wait for all 4 onboarding emails
		await inbox.waitForEmailCount(4, { timeout: 30000 });

		const emails = await inbox.listEmails();

		// Verify sequence order and content
		expect(emails[0].subject).toContain('Welcome');
		expect(emails[1].subject).toContain('Getting Started');
		expect(emails[2].subject).toContain('Tips and Tricks');
		expect(emails[3].subject).toContain("We're Here to Help");

		// Verify timing between emails
		const time1 = emails[0].receivedAt.getTime();
		const time2 = emails[1].receivedAt.getTime();
		const timeDiff = time2 - time1;

		// Emails should be spaced at least 1 second apart
		expect(timeDiff).toBeGreaterThan(1000);
	});
});
```

## Testing Batch Notifications

Test scenarios where multiple similar emails are sent at once:

```javascript
test('should receive batch of order confirmation emails', async () => {
	const orderIds = ['ORD-001', 'ORD-002', 'ORD-003'];

	// Place multiple orders
	for (const orderId of orderIds) {
		await placeOrder(inbox.emailAddress, orderId);
	}

	// Wait for all confirmations
	await inbox.waitForEmailCount(3, { timeout: 30000 });

	const emails = await inbox.listEmails();

	// Verify each order has a confirmation
	for (const orderId of orderIds) {
		const confirmation = emails.find((email) => email.text.includes(orderId));
		expect(confirmation).toBeDefined();
		expect(confirmation.subject).toContain('Order Confirmation');
	}
});
```

## Testing Email Timing

Validate that emails arrive within expected time windows:

```javascript
test('should send emails at correct intervals', async () => {
	const startTime = Date.now();

	// Trigger time-based email sequence
	await startTrialPeriod(inbox.emailAddress);

	// Wait for initial email immediately
	const welcome = await inbox.waitForEmail({
		timeout: 10000,
		subject: /Welcome to your trial/,
	});

	expect(Date.now() - startTime).toBeLessThan(5000);

	// Wait for reminder email (should come after delay)
	await inbox.waitForEmailCount(2, { timeout: 60000 });

	const emails = await inbox.listEmails();
	const reminder = emails.find((email) => email.subject.includes('Trial Reminder'));

	const timeBetween = reminder.receivedAt.getTime() - welcome.receivedAt.getTime();

	// Reminder should come at least 30 seconds after welcome
	expect(timeBetween).toBeGreaterThan(30000);
});
```

## Processing Emails as They Arrive

For scenarios where you need to process emails immediately as they arrive, use `onNewEmail()`:

```javascript
test('should process notifications in real-time', async () => {
	const receivedSubjects = [];

	// Subscribe to new emails
	const subscription = inbox.onNewEmail((email) => {
		receivedSubjects.push(email.subject);
		console.log(`Received: ${email.subject}`);
	});

	// Trigger multiple notifications
	await sendMultipleNotifications(inbox.emailAddress, 5);

	// Wait for all emails to arrive
	await inbox.waitForEmailCount(5, { timeout: 30000 });

	// Cleanup subscription
	subscription.unsubscribe();

	// Verify all were processed
	expect(receivedSubjects.length).toBe(5);
	receivedSubjects.forEach((subject) => {
		expect(subject).toContain('Notification');
	});
});
```

## Testing Parallel Email Flows

Test scenarios where different email types are triggered simultaneously:

```javascript
test('should handle multiple concurrent email types', async () => {
	// Trigger different email flows simultaneously
	await Promise.all([
		sendWelcomeEmail(inbox.emailAddress),
		sendOrderConfirmation(inbox.emailAddress, 'ORD-123'),
		sendNewsletterSubscription(inbox.emailAddress),
	]);

	// Wait for all 3 emails
	await inbox.waitForEmailCount(3, { timeout: 30000 });

	const emails = await inbox.listEmails();

	// Verify all email types arrived
	const welcome = emails.find((e) => e.subject.includes('Welcome'));
	const order = emails.find((e) => e.subject.includes('Order'));
	const newsletter = emails.find((e) => e.subject.includes('Newsletter'));

	expect(welcome).toBeDefined();
	expect(order).toBeDefined();
	expect(newsletter).toBeDefined();
});
```

## Filtering and Validating Multiple Emails

Use array methods to validate email collections:

```javascript
test('should validate all notification emails', async () => {
	await sendBulkNotifications(inbox.emailAddress, 10);

	await inbox.waitForEmailCount(10, { timeout: 30000 });

	const emails = await inbox.listEmails();

	// All should be from the same sender
	const uniqueSenders = [...new Set(emails.map((e) => e.from))];
	expect(uniqueSenders.length).toBe(1);
	expect(uniqueSenders[0]).toBe('notifications@example.com');

	// All should have valid authentication
	emails.forEach((email) => {
		const validation = email.authResults.validate();
		expect(validation).toBeDefined();
	});

	// All should have links
	const emailsWithLinks = emails.filter((e) => e.links.length > 0);
	expect(emailsWithLinks.length).toBe(10);

	// Check that all emails are unique
	const subjects = emails.map((e) => e.subject);
	const uniqueSubjects = new Set(subjects);
	expect(uniqueSubjects.size).toBe(10);
});
```

## Testing with Multiple Inboxes

Test scenarios involving multiple recipients:

```javascript
test('should send emails to multiple recipients', async () => {
	// Create multiple inboxes
	const inbox1 = await client.createInbox();
	const inbox2 = await client.createInbox();
	const inbox3 = await client.createInbox();

	try {
		// Send announcement to all
		await sendAnnouncement([inbox1.emailAddress, inbox2.emailAddress, inbox3.emailAddress]);

		// Wait for emails in all inboxes
		await Promise.all([
			inbox1.waitForEmail({ timeout: 10000, subject: /Announcement/ }),
			inbox2.waitForEmail({ timeout: 10000, subject: /Announcement/ }),
			inbox3.waitForEmail({ timeout: 10000, subject: /Announcement/ }),
		]);

		// Verify all received the same content
		const email1 = await inbox1.getEmail((await inbox1.listEmails())[0].id);
		const email2 = await inbox2.getEmail((await inbox2.listEmails())[0].id);
		const email3 = await inbox3.getEmail((await inbox3.listEmails())[0].id);

		expect(email1.subject).toBe(email2.subject);
		expect(email2.subject).toBe(email3.subject);
		expect(email1.text).toBe(email2.text);
	} finally {
		// Cleanup all inboxes
		await Promise.all([inbox1.delete(), inbox2.delete(), inbox3.delete()]);
	}
});
```

## Monitoring Multiple Inboxes

Use `monitorInboxes()` to watch multiple inboxes simultaneously:

```javascript
test('should monitor multiple inboxes for new emails', async () => {
	const inbox1 = await client.createInbox();
	const inbox2 = await client.createInbox();

	const receivedEmails = [];

	// Monitor both inboxes
	const monitor = client.monitorInboxes([inbox1, inbox2]);

	monitor.on('email', (inbox, email) => {
		receivedEmails.push({
			inboxAddress: inbox.emailAddress,
			subject: email.subject,
		});
	});

	try {
		// Send emails to both inboxes
		await sendEmail(inbox1.emailAddress, 'Test 1');
		await sendEmail(inbox2.emailAddress, 'Test 2');

		// Wait for both emails
		await Promise.all([
			inbox1.waitForEmailCount(1, { timeout: 10000 }),
			inbox2.waitForEmailCount(1, { timeout: 10000 }),
		]);

		// Give the monitor a moment to process
		await new Promise((resolve) => setTimeout(resolve, 1000));

		expect(receivedEmails.length).toBe(2);
		expect(receivedEmails[0].inboxAddress).toBe(inbox1.emailAddress);
		expect(receivedEmails[1].inboxAddress).toBe(inbox2.emailAddress);
	} finally {
		monitor.unsubscribe();
		await Promise.all([inbox1.delete(), inbox2.delete()]);
	}
});
```

## Best Practices

### Use waitForEmailCount() for Known Quantities

When you know exactly how many emails to expect, always use `waitForEmailCount()`:

```javascript
// Good: Efficient and reliable
await inbox.waitForEmailCount(3, { timeout: 30000 });

// Avoid: Arbitrary timeout with polling
await new Promise((resolve) => setTimeout(resolve, 10000));
const emails = await inbox.listEmails();
```

### Set Appropriate Timeouts

Calculate timeouts based on expected email count and delivery speed:

```javascript
// For fast local testing
await inbox.waitForEmailCount(5, { timeout: 10000 }); // 2s per email

// For CI/CD or production gateways
await inbox.waitForEmailCount(5, { timeout: 30000 }); // 6s per email

// For very large batches
await inbox.waitForEmailCount(100, { timeout: 120000 }); // 1.2s per email
```

### Verify Email Ordering When Important

If order matters, explicitly check timestamps:

```javascript
const emails = await inbox.listEmails();

// Sort by received time
emails.sort((a, b) => a.receivedAt.getTime() - b.receivedAt.getTime());

// Verify first email came before second
expect(emails[0].receivedAt.getTime()).toBeLessThan(emails[1].receivedAt.getTime());
```

### Clean Up Multiple Inboxes

Use `Promise.all()` to clean up multiple inboxes efficiently:

```javascript
afterEach(async () => {
	if (inboxes && inboxes.length > 0) {
		await Promise.all(inboxes.map((inbox) => inbox.delete()));
	}
});
```

### Use Descriptive Test Names

Make it clear what email scenario you're testing:

```javascript
// Good: Clear what's being tested
test('should send 3 order confirmation emails in sequence', async () => {});

// Avoid: Vague description
test('should work with multiple emails', async () => {});
```

## Performance Considerations

### Polling Interval

Adjust the polling interval based on expected email volume:

```javascript
// Default: 1 second polling
await inbox.waitForEmailCount(10, { timeout: 30000 });

// Faster polling for time-sensitive tests
await inbox.waitForEmailCount(10, {
	timeout: 30000,
	pollInterval: 500, // Poll every 500ms
});

// Slower polling for large batches
await inbox.waitForEmailCount(100, {
	timeout: 120000,
	pollInterval: 5000, // Poll every 5 seconds
});
```

### Batch Operations

Fetch all emails once rather than making multiple API calls:

```javascript
// Good: Single API call
const emails = await inbox.listEmails();
const welcome = emails.find((e) => e.subject.includes('Welcome'));
const confirmation = emails.find((e) => e.subject.includes('Confirmation'));

// Avoid: Multiple API calls
const email1 = await inbox.getEmail(id1);
const email2 = await inbox.getEmail(id2);
const email3 = await inbox.getEmail(id3);
```

## Next Steps

- [CI/CD Integration](/client-node/testing/cicd) - Run multi-email tests in CI
- [Real-time Monitoring](/client-node/guides/real-time) - Process emails as they arrive
- [Managing Inboxes](/client-node/guides/managing-inboxes) - Learn more about inbox operations
