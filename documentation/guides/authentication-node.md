---
title: Email Authentication Testing
description: Test SPF, DKIM, and DMARC email authentication
---

VaultSandbox validates SPF, DKIM, DMARC, and reverse DNS for every email, helping you catch authentication issues before production.

## Why Test Email Authentication?

Email authentication prevents:

- Emails being marked as spam
- Emails being rejected by receivers
- Domain spoofing and phishing
- Delivery failures

Testing authentication ensures your emails will be trusted by Gmail, Outlook, and other providers.

## Basic Authentication Check

### Using validate()

```javascript
const email = await inbox.waitForEmail({ timeout: 10000 });
const validation = email.authResults.validate();

if (validation.passed) {
	console.log('✅ All authentication checks passed');
} else {
	console.error('❌ Authentication failures:');
	validation.failures.forEach((f) => console.error(`  - ${f}`));
}
```

### Checking Individual Results

```javascript
const auth = email.authResults;

console.log('SPF:', auth.spf?.status);
console.log('DKIM:', auth.dkim?.[0]?.status);
console.log('DMARC:', auth.dmarc?.status);
console.log('Reverse DNS:', auth.reverseDns?.status);
```

## Testing SPF

### Basic SPF Test

```javascript
test('email passes SPF check', async () => {
	await sendEmail(inbox.emailAddress);

	const email = await inbox.waitForEmail({ timeout: 10000 });

	expect(email.authResults.spf).toBeDefined();
	expect(email.authResults.spf.status).toBe('pass');
});
```

### Detailed SPF Validation

```javascript
test('SPF validation details', async () => {
	const email = await inbox.waitForEmail({ timeout: 10000 });
	const spf = email.authResults.spf;

	if (spf) {
		expect(spf.status).toBe('pass');
		expect(spf.domain).toBe('example.com');

		console.log(`SPF ${spf.status} for ${spf.domain}`);
		console.log(`Info: ${spf.info}`);
	}
});
```

### Handling SPF Failures

```javascript
test('handles SPF failure', async () => {
	const email = await inbox.waitForEmail({ timeout: 10000 });
	const spf = email.authResults.spf;

	if (spf && spf.status !== 'pass') {
		console.warn(`SPF ${spf.status}:`, spf.info);

		// Common failures
		if (spf.status === 'fail') {
			console.error('Server IP not authorized in SPF record');
			console.error('Action: Add server IP to SPF record');
		} else if (spf.status === 'softfail') {
			console.warn('Server probably not authorized (~all in SPF)');
		} else if (spf.status === 'none') {
			console.error('No SPF record found');
			console.error('Action: Add SPF record to DNS');
		}
	}
});
```

## Testing DKIM

### Basic DKIM Test

```javascript
test('email has valid DKIM signature', async () => {
	await sendEmail(inbox.emailAddress);

	const email = await inbox.waitForEmail({ timeout: 10000 });

	expect(email.authResults.dkim).toBeDefined();
	expect(email.authResults.dkim.length).toBeGreaterThan(0);
	expect(email.authResults.dkim[0].status).toBe('pass');
});
```

### Multiple DKIM Signatures

```javascript
test('validates all DKIM signatures', async () => {
	const email = await inbox.waitForEmail({ timeout: 10000 });
	const dkim = email.authResults.dkim;

	if (dkim && dkim.length > 0) {
		console.log(`Email has ${dkim.length} DKIM signature(s)`);

		dkim.forEach((sig, index) => {
			console.log(`Signature ${index + 1}:`, {
				status: sig.status,
				domain: sig.domain,
				selector: sig.selector,
			});

			expect(sig.status).toBe('pass');
		});

		// At least one signature should pass
		const anyPassed = dkim.some((sig) => sig.status === 'pass');
		expect(anyPassed).toBe(true);
	}
});
```

### DKIM Selector Verification

```javascript
test('DKIM uses correct selector', async () => {
	const email = await inbox.waitForEmail({ timeout: 10000 });
	const dkim = email.authResults.dkim?.[0];

	if (dkim) {
		expect(dkim.selector).toBe('default'); // Or your expected selector
		expect(dkim.domain).toBe('example.com');

		// DKIM DNS record should exist at:
		// {selector}._domainkey.{domain}
		console.log(`DKIM key at: ${dkim.selector}._domainkey.${dkim.domain}`);
	}
});
```

## Testing DMARC

### Basic DMARC Test

```javascript
test('email passes DMARC', async () => {
	await sendEmail(inbox.emailAddress);

	const email = await inbox.waitForEmail({ timeout: 10000 });

	expect(email.authResults.dmarc).toBeDefined();
	expect(email.authResults.dmarc.status).toBe('pass');
});
```

### DMARC Policy Verification

```javascript
test('DMARC policy is enforced', async () => {
	const email = await inbox.waitForEmail({ timeout: 10000 });
	const dmarc = email.authResults.dmarc;

	if (dmarc) {
		console.log('DMARC status:', dmarc.status);
		console.log('DMARC policy:', dmarc.policy);

		// Policy should be restrictive in production
		expect(['quarantine', 'reject']).toContain(dmarc.policy);
	}
});
```

### DMARC Alignment Check

```javascript
test('DMARC alignment requirements', async () => {
	const email = await inbox.waitForEmail({ timeout: 10000 });

	// DMARC requires either SPF or DKIM to align with From domain
	const validation = email.authResults.validate();

	if (!validation.dmarcPassed) {
		console.log('DMARC failed. Checking alignment:');
		console.log('SPF passed:', validation.spfPassed);
		console.log('DKIM passed:', validation.dkimPassed);

		// At least one should pass for DMARC to pass
		expect(validation.spfPassed || validation.dkimPassed).toBe(true);
	}
});
```

## Testing Reverse DNS

### Basic Reverse DNS Test

```javascript
test('server has valid reverse DNS', async () => {
	const email = await inbox.waitForEmail({ timeout: 10000 });
	const rdns = email.authResults.reverseDns;

	if (rdns) {
		expect(rdns.status).toBe('pass');
		expect(rdns.hostname).toBeTruthy();

		console.log(`Reverse DNS: ${rdns.ip} → ${rdns.hostname}`);
		console.log(`Status: ${rdns.status}`);
	}
});
```

## Complete Authentication Test

### All Checks Pass

```javascript
test('email passes all authentication checks', async () => {
	await app.sendProductionEmail(inbox.emailAddress);

	const email = await inbox.waitForEmail({ timeout: 10000 });
	const validation = email.authResults.validate();

	// All checks should pass in production
	expect(validation.passed).toBe(true);
	expect(validation.spfPassed).toBe(true);
	expect(validation.dkimPassed).toBe(true);
	expect(validation.dmarcPassed).toBe(true);

	// Log results
	console.log('Authentication Results:');
	console.log('  SPF:', validation.spfPassed ? '✅' : '❌');
	console.log('  DKIM:', validation.dkimPassed ? '✅' : '❌');
	console.log('  DMARC:', validation.dmarcPassed ? '✅' : '❌');
	console.log('  Reverse DNS:', validation.reverseDnsPassed ? '✅' : '❌');
});
```

### Graceful Failure Handling

```javascript
test('handles authentication failures gracefully', async () => {
	const email = await inbox.waitForEmail({ timeout: 10000 });
	const validation = email.authResults.validate();

	// Log failures without failing test (for non-production)
	if (!validation.passed) {
		console.warn('⚠️  Authentication issues detected:');
		validation.failures.forEach((failure) => {
			console.warn(`  - ${failure}`);
		});

		// Provide remediation steps
		if (!validation.spfPassed) {
			console.log('\nTo fix SPF:');
			console.log('  Add to DNS: v=spf1 ip4:YOUR_SERVER_IP -all');
		}

		if (!validation.dkimPassed) {
			console.log('\nTo fix DKIM:');
			console.log('  1. Generate DKIM keys');
			console.log('  2. Add public key to DNS');
			console.log('  3. Configure mail server to sign emails');
		}

		if (!validation.dmarcPassed) {
			console.log('\nTo fix DMARC:');
			console.log('  Add to DNS: v=DMARC1; p=none; rua=mailto:dmarc@example.com');
		}
	}

	// In production, this should be expect(validation.passed).toBe(true)
});
```

## Real-World Testing Patterns

### Pre-Production Validation

```javascript
describe('Email Authentication - Pre-Production', () => {
	test('validates staging environment email auth', async () => {
		const email = await inbox.waitForEmail({ timeout: 10000 });
		const auth = email.authResults;

		// SPF should be configured
		if (auth.spf) {
			if (auth.spf.status !== 'pass') {
				console.error('❌ SPF not configured correctly');
				console.error(`   Status: ${auth.spf.status}`);
				console.error(`   Info: ${auth.spf.info}`);
			}
			expect(auth.spf.status).toMatch(/pass|neutral/);
		}

		// DKIM should be present
		if (auth.dkim && auth.dkim.length > 0) {
			const anyValid = auth.dkim.some((d) => d.status === 'pass');
			if (!anyValid) {
				console.error('❌ No valid DKIM signatures');
				console.error('   Fix: Configure DKIM signing in mail server');
			}
			expect(anyValid).toBe(true);
		}
	});
});
```

### Production Readiness Check

```javascript
test('production email configuration', async () => {
	const email = await inbox.waitForEmail({ timeout: 10000 });
	const validation = email.authResults.validate();

	// Production requirements
	const productionReady = {
		spf: validation.spfPassed,
		dkim: validation.dkimPassed,
		dmarc: validation.dmarcPassed,
		allPassed: validation.passed,
	};

	console.log('Production Readiness:');
	console.table(productionReady);

	// Fail if not production-ready
	if (!validation.passed) {
		const issues = validation.failures.join('\n  ');
		throw new Error(
			`Email not production-ready:\n  ${issues}\n\n` + `Fix these issues before deploying to production.`
		);
	}

	expect(validation.passed).toBe(true);
});
```

## Debugging Authentication Issues

### Verbose Logging

```javascript
function logAuthenticationDetails(email) {
	const auth = email.authResults;

	console.log('\n=== Email Authentication Details ===\n');

	// SPF
	if (auth.spf) {
		console.log('SPF:');
		console.log(`  Status: ${auth.spf.status}`);
		console.log(`  Domain: ${auth.spf.domain}`);
		console.log(`  Info: ${auth.spf.info}`);
	} else {
		console.log('SPF: No result');
	}

	// DKIM
	if (auth.dkim && auth.dkim.length > 0) {
		console.log('\nDKIM:');
		auth.dkim.forEach((sig, i) => {
			console.log(`  Signature ${i + 1}:`);
			console.log(`    Status: ${sig.status}`);
			console.log(`    Domain: ${sig.domain}`);
			console.log(`    Selector: ${sig.selector}`);
			console.log(`    Info: ${sig.info}`);
		});
	} else {
		console.log('\nDKIM: No signatures');
	}

	// DMARC
	if (auth.dmarc) {
		console.log('\nDMARC:');
		console.log(`  Status: ${auth.dmarc.status}`);
		console.log(`  Domain: ${auth.dmarc.domain}`);
		console.log(`  Policy: ${auth.dmarc.policy}`);
	} else {
		console.log('\nDMARC: No result');
	}

	// Reverse DNS
	if (auth.reverseDns) {
		console.log('\nReverse DNS:');
		console.log(`  Status: ${auth.reverseDns.status}`);
		console.log(`  IP: ${auth.reverseDns.ip}`);
		console.log(`  Hostname: ${auth.reverseDns.hostname}`);
	}

	// Validation Summary
	const validation = auth.validate();
	console.log('\nValidation Summary:');
	console.log(`  Overall: ${validation.passed ? '✅ PASS' : '❌ FAIL'}`);
	if (!validation.passed) {
		console.log(`  Failures:`);
		validation.failures.forEach((f) => console.log(`    - ${f}`));
	}
}

// Usage
const email = await inbox.waitForEmail({ timeout: 10000 });
logAuthenticationDetails(email);
```

## Next Steps

- **[Authentication Results](/client-node/concepts/auth-results/)** - Deep dive into auth results
- **[Email Objects](/client-node/concepts/emails/)** - Understanding email structure
- **[Testing Patterns](/client-node/testing/password-reset/)** - Real-world test examples
- **[Gateway Security](/gateway/security/)** - Understanding the security model
