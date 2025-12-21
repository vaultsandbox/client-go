---
title: Authentication Results
description: Understanding email authentication (SPF, DKIM, DMARC) in VaultSandbox
---

VaultSandbox validates email authentication for every received email, providing detailed SPF, DKIM, DMARC, and reverse DNS results.

## What is Email Authentication?

Email authentication helps verify that an email:

- Came from the claimed sender domain (**SPF**)
- Wasn't modified in transit (**DKIM**)
- Complies with the domain's policy (**DMARC**)
- Came from a legitimate mail server (**Reverse DNS**)

## AuthResults Object

Every email has an `authResults` property:

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });

const auth = email.authResults;

console.log(auth.spf); // SPF result
console.log(auth.dkim); // DKIM results (array)
console.log(auth.dmarc); // DMARC result
console.log(auth.reverseDns); // Reverse DNS result
```

## SPF (Sender Policy Framework)

Verifies the sending server is authorized to send from the sender's domain.

### SPF Result Structure

```javascript
const spf = email.authResults.spf;

if (spf) {
	console.log(spf.status); // "pass", "fail", "softfail", "neutral", etc.
	console.log(spf.domain); // Domain checked
	console.log(spf.info); // Human-readable details
}
```

### SPF Status Values

| Status      | Meaning                                    |
| ----------- | ------------------------------------------ |
| `pass`      | Sending server is authorized               |
| `fail`      | Sending server is NOT authorized           |
| `softfail`  | Probably not authorized (policy says ~all) |
| `neutral`   | Domain makes no assertion                  |
| `temperror` | Temporary error during check               |
| `permerror` | Permanent error in SPF record              |
| `none`      | No SPF record found                        |

### SPF Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });

if (email.authResults.spf) {
	const spf = email.authResults.spf;

	expect(spf.status).toBe('pass');
	expect(spf.domain).toBe('example.com');

	console.log(`SPF ${spf.status} for ${spf.domain}`);
}
```

## DKIM (DomainKeys Identified Mail)

Cryptographically verifies the email hasn't been modified and came from the claimed domain.

### DKIM Result Structure

```javascript
const dkim = email.authResults.dkim; // Array of results

if (dkim && dkim.length > 0) {
	dkim.forEach((result) => {
		console.log(result.status); // "pass", "fail", "none"
		console.log(result.domain); // Signing domain
		console.log(result.selector); // DKIM selector
		console.log(result.info); // Human-readable details
	});
}
```

**Note**: An email can have multiple DKIM signatures (one per signing domain).

### DKIM Status Values

| Status | Meaning                 |
| ------ | ----------------------- |
| `pass` | Signature is valid      |
| `fail` | Signature is invalid    |
| `none` | No DKIM signature found |

### DKIM Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });

if (email.authResults.dkim && email.authResults.dkim.length > 0) {
	const dkim = email.authResults.dkim[0];

	expect(dkim.status).toBe('pass');
	expect(dkim.domain).toBe('example.com');

	console.log(`DKIM ${dkim.status} (${dkim.selector}._domainkey.${dkim.domain})`);
}
```

## DMARC (Domain-based Message Authentication)

Checks that SPF or DKIM align with the From address and enforces the domain's policy.

### DMARC Result Structure

```javascript
const dmarc = email.authResults.dmarc;

if (dmarc) {
	console.log(dmarc.status); // "pass", "fail", "none"
	console.log(dmarc.domain); // Domain checked
	console.log(dmarc.policy); // Domain's policy (none, quarantine, reject)
	console.log(dmarc.aligned); // Whether SPF/DKIM aligns with From domain
	console.log(dmarc.info); // Human-readable details
}
```

### DMARC Status Values

| Status | Meaning                                  |
| ------ | ---------------------------------------- |
| `pass` | DMARC check passed (SPF or DKIM aligned) |
| `fail` | DMARC check failed                       |
| `none` | No DMARC policy found                    |

### DMARC Policies

| Policy       | Meaning                         |
| ------------ | ------------------------------- |
| `none`       | No action (monitoring only)     |
| `quarantine` | Treat suspicious emails as spam |
| `reject`     | Reject emails that fail DMARC   |

### DMARC Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });

if (email.authResults.dmarc) {
	const dmarc = email.authResults.dmarc;

	expect(dmarc.status).toBe('pass');
	expect(dmarc.domain).toBe('example.com');

	console.log(`DMARC ${dmarc.status} (policy: ${dmarc.policy})`);
}
```

## Reverse DNS

Verifies the sending server's IP resolves to a hostname that matches the sending domain.

### Reverse DNS Result Structure

```javascript
const reverseDns = email.authResults.reverseDns;

if (reverseDns) {
	console.log(reverseDns.status); // "pass", "fail", "none"
	console.log(reverseDns.ip); // Server IP
	console.log(reverseDns.hostname); // Resolved hostname (may be empty)
	console.log(reverseDns.info); // Human-readable details
}
```

### Reverse DNS Status Values

| Status | Meaning              |
| ------ | -------------------- |
| `pass` | Reverse DNS verified |
| `fail` | Reverse DNS failed   |
| `none` | No PTR record        |

### Reverse DNS Example

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });

if (email.authResults.reverseDns) {
	const rdns = email.authResults.reverseDns;

	console.log(`Reverse DNS: ${rdns.ip} → ${rdns.hostname}`);
	console.log(`Status: ${rdns.status}`);
}
```

## Validation Helper

The `validate()` method provides a summary of all authentication checks:

```javascript
const validation = email.authResults.validate();

console.log(validation.passed); // Overall pass/fail
console.log(validation.spfPassed); // SPF check
console.log(validation.dkimPassed); // DKIM check
console.log(validation.dmarcPassed); // DMARC check
console.log(validation.reverseDnsPassed); // Reverse DNS check
console.log(validation.failures); // Array of failure reasons
```

### Validation Result Structure

```typescript
interface AuthValidation {
	passed: boolean; // True if all checks passed
	spfPassed: boolean; // SPF passed
	dkimPassed: boolean; // At least one DKIM passed
	dmarcPassed: boolean; // DMARC passed
	reverseDnsPassed: boolean; // Reverse DNS passed
	failures: string[]; // Array of failure descriptions
}
```

### Validation Examples

**All checks pass**:

```javascript
const validation = email.authResults.validate();

// {
//   passed: true,
//   spfPassed: true,
//   dkimPassed: true,
//   dmarcPassed: true,
//   reverseDnsPassed: true,
//   failures: []
// }

expect(validation.passed).toBe(true);
```

**Some checks fail**:

```javascript
const validation = email.authResults.validate();

// {
//   passed: false,
//   spfPassed: false,
//   dkimPassed: true,
//   dmarcPassed: false,
//   reverseDnsPassed: true,
//   failures: [
//     "SPF check failed: status 'fail'",
//     "DMARC check failed: status 'fail'"
//   ]
// }

if (!validation.passed) {
	console.error('Authentication failures:');
	validation.failures.forEach((failure) => {
		console.error(`  - ${failure}`);
	});
}
```

## Testing Patterns

### Strict Authentication

```javascript
test('email passes all authentication checks', async () => {
	await sendEmail(inbox.emailAddress);

	const email = await inbox.waitForEmail({ timeout: 10000 });
	const validation = email.authResults.validate();

	expect(validation.passed).toBe(true);
	expect(validation.spfPassed).toBe(true);
	expect(validation.dkimPassed).toBe(true);
	expect(validation.dmarcPassed).toBe(true);
});
```

### Lenient Authentication

```javascript
test('email has valid DKIM signature', async () => {
	await sendEmail(inbox.emailAddress);

	const email = await inbox.waitForEmail({ timeout: 10000 });

	// Only check DKIM (most reliable)
	expect(email.authResults.dkim).toBeDefined();
	expect(email.authResults.dkim.length).toBeGreaterThan(0);
	expect(email.authResults.dkim[0].status).toBe('pass');
});
```

### Handling Missing Authentication

```javascript
test('handles emails without authentication', async () => {
	const email = await inbox.waitForEmail({ timeout: 10000 });

	// Some senders don't have SPF/DKIM configured
	const validation = email.authResults.validate();

	// Check if validation was performed
	expect(typeof validation.passed).toBe('boolean');
	expect(Array.isArray(validation.failures)).toBe(true);

	// Log results for debugging
	if (!validation.passed) {
		console.log('Auth failures (expected for test emails):', validation.failures);
	}
});
```

### Testing Specific Checks

```javascript
describe('Email Authentication', () => {
	let inbox, email;

	beforeEach(async () => {
		inbox = await client.createInbox();
		await sendEmail(inbox.emailAddress);
		email = await inbox.waitForEmail({ timeout: 10000 });
	});

	afterEach(async () => {
		await inbox.delete();
	});

	test('SPF check', () => {
		if (email.authResults.spf) {
			expect(email.authResults.spf.status).toMatch(/pass|neutral|softfail/);
		}
	});

	test('DKIM check', () => {
		if (email.authResults.dkim && email.authResults.dkim.length > 0) {
			const anyPassed = email.authResults.dkim.some((d) => d.status === 'pass');
			expect(anyPassed).toBe(true);
		}
	});

	test('DMARC check', () => {
		if (email.authResults.dmarc) {
			expect(email.authResults.dmarc.status).toMatch(/pass|none/);
		}
	});
});
```

## Why Authentication Matters

### Production Readiness

Testing authentication catches issues like:

- **Misconfigured SPF records** → emails rejected by Gmail/Outlook
- **Missing DKIM signatures** → reduced deliverability
- **DMARC failures** → emails sent to spam
- **Reverse DNS mismatches** → flagged as suspicious

### Real-World Example

```javascript
test('production email configuration', async () => {
	await app.sendWelcomeEmail(inbox.emailAddress);

	const email = await inbox.waitForEmail({ timeout: 10000 });
	const validation = email.authResults.validate();

	// In production, these should all pass
	if (!validation.passed) {
		console.error('❌ Email authentication issues detected:');
		validation.failures.forEach((f) => console.error(`   ${f}`));
		console.error('');
		console.error('Action required:');

		if (!validation.spfPassed) {
			console.error('- Fix SPF record for your domain');
		}
		if (!validation.dkimPassed) {
			console.error('- Configure DKIM signing in your email service');
		}
		if (!validation.dmarcPassed) {
			console.error('- Add/fix DMARC policy');
		}
	}

	// Fail test if authentication fails
	expect(validation.passed).toBe(true);
});
```

## Troubleshooting

### No Authentication Results

```javascript
if (!email.authResults.spf && !email.authResults.dkim && !email.authResults.dmarc) {
	console.log('No authentication performed');
	console.log('This may happen for:');
	console.log('- Emails sent from localhost/internal servers');
	console.log('- Test SMTP servers without authentication');
}
```

### All Checks Fail

```javascript
const validation = email.authResults.validate();

if (!validation.passed) {
	console.error('Authentication failed:', validation.failures);

	// Common causes:
	// 1. No SPF record: Add "v=spf1 ip4:YOUR_IP -all" to DNS
	// 2. No DKIM: Configure your mail server to sign emails
	// 3. No DMARC: Add "v=DMARC1; p=none" to DNS
	// 4. Wrong IP: Update SPF record with correct server IP
}
```

### Understanding Failure Reasons

```javascript
const validation = email.authResults.validate();

validation.failures.forEach((failure) => {
	if (failure.includes('SPF')) {
		console.log('Fix SPF: Update DNS TXT record for your domain');
	}
	if (failure.includes('DKIM')) {
		console.log('Fix DKIM: Enable DKIM signing in your email service');
	}
	if (failure.includes('DMARC')) {
		console.log('Fix DMARC: Add DMARC policy to DNS');
	}
});
```

## Next Steps

- **[Email Authentication Guide](/client-node/guides/authentication/)** - Testing authentication in depth
- **[Email Objects](/client-node/concepts/emails/)** - Understanding email structure
- **[Testing Patterns](/client-node/testing/password-reset/)** - Real-world testing examples
- **[Gateway Security](/gateway/security/)** - Understanding the security model
