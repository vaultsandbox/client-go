---
title: Testing Password Reset Flows
description: Learn how to test password reset email flows with VaultSandbox
---

Password reset flows are one of the most common email testing scenarios. This guide demonstrates how to use VaultSandbox to test password reset emails end-to-end, including link extraction and email validation.

## Basic Password Reset Test

Here's a complete example of testing a password reset flow:

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

const client = new VaultSandboxClient({
	url: 'https://smtp.vaultsandbox.com',
	apiKey: process.env.VAULTSANDBOX_API_KEY,
});

const inbox = await client.createInbox();

// Trigger password reset in your application
await yourApp.requestPasswordReset(inbox.emailAddress);

// Wait for and validate the reset email
const email = await inbox.waitForEmail({
	timeout: 10000,
	subject: /Reset your password/,
});

// Extract reset link
const resetLink = email.links.find((url) => url.includes('/reset-password'));
console.log('Reset link:', resetLink);

// Validate email authentication
const authValidation = email.authResults.validate();
expect(typeof authValidation.passed).toBe('boolean');
expect(Array.isArray(authValidation.failures)).toBe(true);

await inbox.delete();
```

## Jest Integration

Integrate password reset testing into your Jest test suite:

```javascript
describe('Password Reset Flow', () => {
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
		if (inbox) {
			await inbox.delete();
		}
	});

	test('should send password reset email with valid link', async () => {
		// Trigger password reset
		await requestPasswordReset(inbox.emailAddress);

		// Wait for email
		const email = await inbox.waitForEmail({
			timeout: 10000,
			subject: /Reset your password/,
		});

		// Validate sender
		expect(email.from).toBe('noreply@example.com');

		// Validate content
		expect(email.text).toContain('requested a password reset');
		expect(email.html).toBeTruthy();

		// Extract and validate reset link
		const resetLink = email.links.find((url) => url.includes('/reset-password'));
		expect(resetLink).toBeDefined();
		expect(resetLink).toMatch(/^https:\/\//);
		expect(resetLink).toContain('token=');
	});

	test('should contain user information in reset email', async () => {
		const userEmail = inbox.emailAddress;
		const userName = 'John Doe';

		await requestPasswordReset(userEmail, { name: userName });

		const email = await inbox.waitForEmail({
			timeout: 10000,
			subject: /Reset your password/,
		});

		// Verify personalization
		expect(email.text).toContain(userName);
		expect(email.to).toContain(userEmail);
	});

	test('should validate reset link is functional', async () => {
		await requestPasswordReset(inbox.emailAddress);

		const email = await inbox.waitForEmail({
			timeout: 10000,
			subject: /Reset your password/,
		});

		const resetLink = email.links.find((url) => url.includes('/reset-password'));

		// Test that the link is accessible
		const response = await fetch(resetLink);
		expect(response.ok).toBe(true);
		expect(response.status).toBe(200);
	});
});
```

## Link Extraction Patterns

VaultSandbox automatically extracts all links from emails. Here are common patterns for finding password reset links:

```javascript
// Find by path
const resetLink = email.links.find((url) => url.includes('/reset-password'));

// Find by domain
const resetLink = email.links.find((url) => url.includes('yourdomain.com/reset'));

// Find by query parameter
const resetLink = email.links.find((url) => url.includes('token='));

// Find using regex
const resetLink = email.links.find((url) => /\/reset.*token=/i.test(url));

// Extract token from link
const url = new URL(resetLink);
const token = url.searchParams.get('token');
expect(token).toBeTruthy();
expect(token.length).toBeGreaterThan(20);
```

## Validating Email Content

Test the content and formatting of your password reset emails:

```javascript
test('should have properly formatted reset email', async () => {
	await requestPasswordReset(inbox.emailAddress);

	const email = await inbox.waitForEmail({
		timeout: 10000,
		subject: /Reset your password/,
	});

	// Validate plain text version
	expect(email.text).toBeTruthy();
	expect(email.text).toContain('reset your password');
	expect(email.text).not.toContain('undefined');
	expect(email.text).not.toContain('[object Object]');

	// Validate HTML version
	expect(email.html).toBeTruthy();
	expect(email.html).toContain('<a href=');
	expect(email.html).toContain('reset');

	// Validate email has exactly one reset link
	const resetLinks = email.links.filter((url) => url.includes('/reset-password'));
	expect(resetLinks.length).toBe(1);

	// Validate headers
	expect(email.headers['content-type']).toBeDefined();
});
```

## Testing Security Features

Validate email authentication to ensure your emails won't be marked as spam:

```javascript
test('should pass email authentication checks', async () => {
	await requestPasswordReset(inbox.emailAddress);

	const email = await inbox.waitForEmail({
		timeout: 10000,
		subject: /Reset your password/,
	});

	const validation = email.authResults.validate();

	// Check that validation was performed
	expect(validation).toBeDefined();
	expect(typeof validation.passed).toBe('boolean');
	expect(Array.isArray(validation.failures)).toBe(true);

	// Log any failures for debugging
	if (!validation.passed) {
		console.warn('Email authentication failures:', validation.failures);
	}

	// Check individual authentication methods (if configured)
	if (email.authResults.spf?.status) {
		expect(email.authResults.spf.status).toMatch(/pass|neutral|softfail/);
	}

	if (email.authResults.dkim && email.authResults.dkim.length > 0) {
		expect(email.authResults.dkim[0].status).toBeDefined();
	}
});
```

## Testing Reset Token Expiration

Test that your password reset emails include expiration information:

```javascript
test('should include expiration time in reset email', async () => {
	await requestPasswordReset(inbox.emailAddress);

	const email = await inbox.waitForEmail({
		timeout: 10000,
		subject: /Reset your password/,
	});

	// Validate expiration is mentioned
	const textLower = email.text.toLowerCase();
	const hasExpiration =
		textLower.includes('expires') ||
		textLower.includes('valid for') ||
		textLower.includes('24 hours') ||
		textLower.includes('1 hour');

	expect(hasExpiration).toBe(true);
});
```

## Testing Multiple Reset Requests

Test what happens when a user requests multiple password resets:

```javascript
test('should handle multiple reset requests', async () => {
	// Request multiple resets
	await requestPasswordReset(inbox.emailAddress);
	await requestPasswordReset(inbox.emailAddress);

	// Wait for both emails
	await inbox.waitForEmailCount(2, { timeout: 15000 });

	const emails = await inbox.listEmails();
	expect(emails.length).toBe(2);

	// Both should have reset links
	emails.forEach((email) => {
		const resetLink = email.links.find((url) => url.includes('/reset-password'));
		expect(resetLink).toBeDefined();
	});

	// Tokens should be different (if your app invalidates old tokens)
	const link1 = emails[0].links.find((url) => url.includes('/reset-password'));
	const link2 = emails[1].links.find((url) => url.includes('/reset-password'));

	const token1 = new URL(link1).searchParams.get('token');
	const token2 = new URL(link2).searchParams.get('token');

	expect(token1).not.toBe(token2);
});
```

## Best Practices

### Use Specific Subject Filters

Always filter by subject to ensure you're testing the right email:

```javascript
// Good: Specific subject filter
const email = await inbox.waitForEmail({
	timeout: 10000,
	subject: /Reset your password/,
});

// Avoid: No filter (might match wrong email in CI)
const email = await inbox.waitForEmail({ timeout: 10000 });
```

### Clean Up Inboxes

Always delete inboxes after tests to avoid hitting limits:

```javascript
afterEach(async () => {
	if (inbox) {
		await inbox.delete();
	}
});
```

### Use Appropriate Timeouts

Set realistic timeouts based on your email delivery speed:

```javascript
// Local development: shorter timeout
const email = await inbox.waitForEmail({ timeout: 5000 });

// CI/CD: longer timeout to account for slower environments
const email = await inbox.waitForEmail({ timeout: 15000 });
```

### Test Complete Flow

Don't just validate that the email was sent - test that the link actually works:

```javascript
test('should complete full password reset flow', async () => {
	// 1. Request reset
	await requestPasswordReset(inbox.emailAddress);

	// 2. Get email
	const email = await inbox.waitForEmail({
		timeout: 10000,
		subject: /Reset your password/,
	});

	// 3. Extract link
	const resetLink = email.links.find((url) => url.includes('/reset-password'));

	// 4. Visit reset page
	const response = await fetch(resetLink);
	expect(response.ok).toBe(true);

	// 5. Submit new password
	const newPassword = 'NewSecurePassword123!';
	await submitPasswordReset(resetLink, newPassword);

	// 6. Verify login with new password
	const loginSuccess = await login(inbox.emailAddress, newPassword);
	expect(loginSuccess).toBe(true);
});
```

## Next Steps

- [Testing Multi-Email Scenarios](/client-node/testing/multi-email) - Handle multiple emails
- [CI/CD Integration](/client-node/testing/cicd) - Run tests in your pipeline
- [Working with Attachments](/client-node/guides/attachments) - Test emails with attachments
