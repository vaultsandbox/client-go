---
title: Waiting for Emails
description: Efficiently wait for and filter emails in your tests
---

VaultSandbox provides powerful methods for waiting for emails with filtering and timeout support.

## Basic Waiting

### Wait for Any Email

```javascript
const email = await inbox.waitForEmail({
	timeout: 30000, // 30 seconds
});

console.log('Received:', email.subject);
```

### With Default Timeout

```javascript
// Uses default 30 second timeout
const email = await inbox.waitForEmail();
```

## Filtering Options

### Filter by Subject

```javascript
// Exact match
const email = await inbox.waitForEmail({
	timeout: 10000,
	subject: 'Password Reset',
});

// Regex match
const email = await inbox.waitForEmail({
	timeout: 10000,
	subject: /reset/i, // Case-insensitive
});
```

### Filter by Sender

```javascript
// Exact match
const email = await inbox.waitForEmail({
	timeout: 10000,
	from: 'noreply@example.com',
});

// Regex match
const email = await inbox.waitForEmail({
	timeout: 10000,
	from: /@example\.com$/, // Any @example.com address
});
```

### Multiple Filters

```javascript
const email = await inbox.waitForEmail({
	timeout: 10000,
	subject: /welcome/i,
	from: 'support@example.com',
});
```

### Custom Predicate

```javascript
const email = await inbox.waitForEmail({
	timeout: 10000,
	predicate: (email) => {
		// Custom logic
		return email.to.includes('specific@example.com') && email.links.length > 0 && email.subject.includes('Verify');
	},
});
```

## Waiting for Multiple Emails

### Wait for Specific Count

```javascript
// Trigger multiple emails
await sendNotifications(inbox.emailAddress, 3);

// Wait for all 3 to arrive
await inbox.waitForEmailCount(3, {
	timeout: 30000,
	pollInterval: 1000, // Check every second
});

// Now list all emails
const emails = await inbox.listEmails();
expect(emails.length).toBe(3);
```

### Process as They Arrive

```javascript
async function waitForEmails(inbox, count) {
	const emails = [];

	for (let i = 0; i < count; i++) {
		const email = await inbox.waitForEmail({ timeout: 30000 });
		emails.push(email);
		console.log(`Received ${i + 1}/${count}: ${email.subject}`);
	}

	return emails;
}

// Usage
const emails = await waitForEmails(inbox, 3);
```

## Timeout Handling

### With Error Handling

```javascript
try {
	const email = await inbox.waitForEmail({ timeout: 5000 });
	console.log('Email received:', email.subject);
} catch (error) {
	if (error instanceof TimeoutError) {
		console.error('No email received within 5 seconds');
	} else {
		throw error;
	}
}
```

### With Fallback

```javascript
async function waitForEmailWithFallback(inbox, options) {
	try {
		return await inbox.waitForEmail(options);
	} catch (error) {
		if (error instanceof TimeoutError) {
			console.log('Timeout, checking if email arrived anyway');
			const emails = await inbox.listEmails();
			return emails[emails.length - 1]; // Return latest
		}
		throw error;
	}
}
```

### Retry Pattern

```javascript
async function waitWithRetry(inbox, options, maxRetries = 3) {
	for (let i = 0; i < maxRetries; i++) {
		try {
			return await inbox.waitForEmail(options);
		} catch (error) {
			if (error instanceof TimeoutError && i < maxRetries - 1) {
				console.log(`Attempt ${i + 1} failed, retrying...`);
				continue;
			}
			throw error;
		}
	}
}
```

## Polling Configuration

### Custom Poll Interval

```javascript
// Poll every 500ms (more responsive)
const email = await inbox.waitForEmail({
	timeout: 10000,
	pollInterval: 500,
});

// Poll every 5 seconds (less frequent)
const email = await inbox.waitForEmail({
	timeout: 60000,
	pollInterval: 5000,
});
```

### Efficient Polling

```javascript
// For quick tests - poll frequently
const email = await inbox.waitForEmail({
	timeout: 5000,
	pollInterval: 200, // Check 5 times per second
});

// For slow email services - poll less frequently
const email = await inbox.waitForEmail({
	timeout: 120000, // 2 minutes
	pollInterval: 10000, // Check every 10 seconds
});
```

## Real-World Examples

### Password Reset Flow

```javascript
test('password reset email', async () => {
	// Trigger reset
	await app.requestPasswordReset(inbox.emailAddress);

	// Wait for reset email
	const email = await inbox.waitForEmail({
		timeout: 10000,
		subject: /reset/i,
		from: 'security@example.com',
	});

	// Validate content
	expect(email.subject).toContain('Password Reset');
	expect(email.links.length).toBeGreaterThan(0);

	// Extract and test link
	const resetLink = email.links.find((url) => url.includes('/reset'));
	expect(resetLink).toBeDefined();
});
```

### Welcome Email with Verification

```javascript
test('welcome email with verification link', async () => {
	// Sign up
	await app.signup({
		email: inbox.emailAddress,
		name: 'Test User',
	});

	// Wait for welcome email
	const email = await inbox.waitForEmail({
		timeout: 10000,
		subject: /welcome/i,
		predicate: (e) => e.links.some((link) => link.includes('/verify')),
	});

	// Extract verification link
	const verifyLink = email.links.find((url) => url.includes('/verify'));

	// Test verification
	const response = await fetch(verifyLink);
	expect(response.ok).toBe(true);
});
```

### Multi-Step Email Flow

```javascript
test('order confirmation and shipping notification', async () => {
	// Place order
	await app.placeOrder({
		email: inbox.emailAddress,
		items: ['widget'],
	});

	// Wait for confirmation
	const confirmation = await inbox.waitForEmail({
		timeout: 10000,
		subject: /order.*confirmed/i,
	});

	expect(confirmation.subject).toContain('Order Confirmed');
	expect(confirmation.text).toContain('Order #');

	// Simulate shipping
	await app.shipOrder(orderId);

	// Wait for shipping notification
	const shipping = await inbox.waitForEmail({
		timeout: 10000,
		subject: /shipped/i,
	});

	expect(shipping.subject).toContain('Shipped');
	expect(shipping.text).toContain('tracking');
});
```

### Email with Attachments

```javascript
test('invoice email with PDF attachment', async () => {
	await app.sendInvoice(inbox.emailAddress);

	const email = await inbox.waitForEmail({
		timeout: 10000,
		subject: /invoice/i,
		predicate: (e) => e.attachments.length > 0,
	});

	// Validate attachment
	const pdf = email.attachments.find((att) => att.contentType === 'application/pdf');

	expect(pdf).toBeDefined();
	expect(pdf.filename).toMatch(/invoice.*\.pdf/i);
	expect(pdf.size).toBeGreaterThan(0);
});
```

## Advanced Patterns

### Wait for First Matching Email

```javascript
async function waitForFirstMatch(inbox, matchers, timeout = 30000) {
	const startTime = Date.now();

	while (Date.now() - startTime < timeout) {
		const emails = await inbox.listEmails();

		for (const matcher of matchers) {
			const match = emails.find(matcher);
			if (match) return match;
		}

		await new Promise((resolve) => setTimeout(resolve, 1000));
	}

	throw new TimeoutError('No matching email found');
}

// Usage
const email = await waitForFirstMatch(inbox, [
	(e) => e.subject.includes('Welcome'),
	(e) => e.subject.includes('Verify'),
	(e) => e.from === 'support@example.com',
]);
```

### Wait with Progress Callback

```javascript
async function waitWithProgress(inbox, options, onProgress) {
	const startTime = Date.now();
	let attempts = 0;

	try {
		return await inbox.waitForEmail({
			...options,
			pollInterval: options.pollInterval || 1000,
		});
	} catch (error) {
		if (error instanceof TimeoutError) {
			const elapsed = Date.now() - startTime;
			onProgress({ attempts, elapsed, timedOut: true });
		}
		throw error;
	}
}

// Usage
const email = await waitWithProgress(inbox, { timeout: 10000, subject: /test/i }, (progress) => {
	console.log(`Attempt ${progress.attempts}, ${progress.elapsed}ms elapsed`);
});
```

### Conditional Waiting

```javascript
async function waitConditionally(inbox, options) {
	// First check if email already exists
	const existing = await inbox.listEmails();
	const match = existing.find((e) => options.subject.test(e.subject));

	if (match) {
		console.log('Email already present');
		return match;
	}

	// Wait for new email
	console.log('Waiting for email...');
	return await inbox.waitForEmail(options);
}
```

## Testing Patterns

### Flake-Free Tests

```javascript
// ✅ Good: Use waitForEmail, not sleep
test('receives email', async () => {
	await sendEmail(inbox.emailAddress);
	const email = await inbox.waitForEmail({ timeout: 10000 });
	expect(email).toBeDefined();
});

// ❌ Bad: Arbitrary sleep causes flakiness
test('receives email', async () => {
	await sendEmail(inbox.emailAddress);
	await sleep(5000); // May not be enough, or wastes time
	const emails = await inbox.listEmails();
	expect(emails.length).toBe(1);
});
```

### Fast Tests

```javascript
// ✅ Good: Short timeout for fast-sending systems
const email = await inbox.waitForEmail({ timeout: 2000 });

// ❌ Bad: Unnecessarily long timeout slows tests
const email = await inbox.waitForEmail({ timeout: 60000 });
```

### Parallel Email Tests

```javascript
test('multiple users receive emails', async () => {
	const inbox1 = await client.createInbox();
	const inbox2 = await client.createInbox();

	// Send emails
	await Promise.all([sendWelcome(inbox1.emailAddress), sendWelcome(inbox2.emailAddress)]);

	// Wait in parallel
	const [email1, email2] = await Promise.all([
		inbox1.waitForEmail({ timeout: 10000 }),
		inbox2.waitForEmail({ timeout: 10000 }),
	]);

	expect(email1.subject).toContain('Welcome');
	expect(email2.subject).toContain('Welcome');

	await Promise.all([inbox1.delete(), inbox2.delete()]);
});
```

## Troubleshooting

### Email Not Arriving

```javascript
// Add debug logging
try {
	console.log('Waiting for email...');
	const email = await inbox.waitForEmail({
		timeout: 10000,
		subject: /test/i,
	});
	console.log('Received:', email.subject);
} catch (error) {
	console.error('Timeout! Checking inbox manually:');
	const emails = await inbox.listEmails();
	console.log(`Found ${emails.length} emails:`);
	emails.forEach((e) => console.log(`  - ${e.subject}`));
	throw error;
}
```

### Filter Not Matching

```javascript
// Log filter mismatches
const email = await inbox.waitForEmail({
	timeout: 10000,
	predicate: (e) => {
		const matches = e.subject.includes('Test');
		if (!matches) {
			console.log(`Subject "${e.subject}" doesn't match`);
		}
		return matches;
	},
});
```

## Next Steps

- **[Managing Inboxes](/client-node/guides/managing-inboxes/)** - Learn inbox operations
- **[Real-time Monitoring](/client-node/guides/real-time/)** - Subscribe to emails as they arrive
- **[Testing Patterns](/client-node/testing/password-reset/)** - Real-world testing examples
- **[API Reference: Inbox](/client-node/api/inbox/)** - Complete API documentation
