---
title: Inbox Import/Export
description: Learn how to export and import inboxes for test reproducibility and cross-environment sharing
---

VaultSandbox allows you to export and import inboxes, including their encryption keys and metadata. This enables advanced workflows like test reproducibility, manual testing, cross-environment sharing, and debugging.

## Overview

When you export an inbox, you get a JSON object containing:

- Email address
- Inbox identifier
- Expiration time
- **Public encryption key** (base64-encoded)
- **Secret encryption key** (sensitive!)
- **Server public signing key**
- Export timestamp

This exported data can be imported into another client instance, allowing you to access the same inbox from different environments or at different times.

## Security Warning

**Exported inbox data contains private encryption keys.** Anyone with this data can:

- Read all emails in the inbox
- Impersonate the inbox to receive new emails
- Decrypt all future emails sent to the inbox

**Never:**

- Commit exported data to version control
- Share exported data over insecure channels
- Store exported data in plaintext in production

**Always:**

- Treat exported data as sensitive credentials
- Encrypt exported files at rest
- Use secure channels for sharing
- Rotate/delete inboxes after use

## Use Cases

### 1. Test Reproducibility

Export an inbox at the end of a test run to reproduce issues later:

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

describe('Email Flow', () => {
	let client;
	let inbox;

	beforeEach(async () => {
		client = new VaultSandboxClient({ url, apiKey });
		inbox = await client.createInbox();
	});

	afterEach(async () => {
		// Export on test failure
		if (this.currentTest.state === 'failed') {
			const exportData = inbox.export();
			const filename = `./debug/inbox-${Date.now()}.json`;
			fs.writeFileSync(filename, JSON.stringify(exportData, null, 2));
			console.log(`Inbox exported to ${filename}`);
		}

		await inbox?.delete();
	});

	test('should receive welcome email', async () => {
		await sendWelcomeEmail(inbox.emailAddress);

		const email = await inbox.waitForEmail({
			timeout: 10000,
			subject: /Welcome/,
		});

		expect(email.subject).toContain('Welcome');
	});
});
```

### 2. Manual Testing

Export an inbox from automated tests for manual verification:

```javascript
// In your test
const inbox = await client.createInbox();

// Export for manual testing
await client.exportInboxToFile(inbox, './manual-test-inbox.json');

console.log(`Manual test inbox: ${inbox.emailAddress}`);
console.log('Exported to: ./manual-test-inbox.json');

// Continue with automated tests...
```

Then manually inspect:

```bash
# Use the exported inbox in a manual test script
npx tsx scripts/check-inbox.ts ./manual-test-inbox.json
```

### 3. Cross-Environment Sharing

Export an inbox from one environment and import it in another:

```javascript
// Development environment
const devClient = new VaultSandboxClient({
	url: 'https://dev.vaultsandbox.com',
	apiKey: process.env.DEV_API_KEY,
});

const inbox = await devClient.createInbox();
const exportData = inbox.export();

// Save to shared location
fs.writeFileSync('./shared/staging-inbox.json', JSON.stringify(exportData));

// ---

// Staging environment
const stagingClient = new VaultSandboxClient({
	url: 'https://dev.vaultsandbox.com', // Must match!
	apiKey: process.env.STAGING_API_KEY,
});

const exportedData = JSON.parse(fs.readFileSync('./shared/staging-inbox.json', 'utf8'));

const inbox = await stagingClient.importInbox(exportedData);
console.log(`Imported inbox: ${inbox.emailAddress}`);
```

### 4. Debugging Production Issues

Export a problematic inbox from production for local debugging:

```javascript
// Production: Export the inbox
const inbox = await client.createInbox();
// ... test runs, issue occurs ...

await client.exportInboxToFile(inbox, './production-issue-123.json');

// ---

// Local development: Import and investigate
const localClient = new VaultSandboxClient({
	url: 'https://smtp.vaultsandbox.com', // Same server as production
	apiKey: process.env.LOCAL_API_KEY,
});

const inbox = await localClient.importInboxFromFile('./production-issue-123.json');

// Check emails
const emails = await inbox.listEmails();
console.log(`Found ${emails.length} emails`);

emails.forEach((email) => {
	console.log(`\n---`);
	console.log(`Subject: ${email.subject}`);
	console.log(`From: ${email.from}`);
	console.log(`Received: ${email.receivedAt.toISOString()}`);
	console.log(`Links: ${email.links.length}`);
	console.log(`Attachments: ${email.attachments.length}`);
});
```

## Export Methods

### Export to Object

```typescript
export(): ExportedInboxData
```

Returns a JavaScript object with the inbox data:

```javascript
const inbox = await client.createInbox();
const data = inbox.export();

console.log(data);
// {
//   emailAddress: 'test123@inbox.vaultsandbox.com',
//   inboxHash: 'abc123...',
//   expiresAt: '2024-12-01T12:00:00.000Z',
//   serverSigPk: 'base64-encoded-server-signing-key',
//   publicKeyB64: 'base64-encoded-public-key',
//   secretKeyB64: 'base64-encoded-secret-key',
//   exportedAt: '2024-11-30T08:00:00.000Z'
// }

// Save to file
fs.writeFileSync('inbox.json', JSON.stringify(data, null, 2));
```

### Export to File

```typescript
async exportInboxToFile(inboxOrEmail: Inbox | string, filePath: string): Promise<void>
```

Directly writes the inbox data to a JSON file:

```javascript
const inbox = await client.createInbox();

// Export by inbox instance
await client.exportInboxToFile(inbox, './backups/inbox.json');

// Export by email address
await client.exportInboxToFile(inbox.emailAddress, './backups/inbox.json');
```

### Using VaultSandboxClient

Both export methods are available on the client:

```javascript
// From inbox instance
const data = inbox.export();

// From client (any inbox managed by this client)
const data = client.exportInbox(inbox);
const data = client.exportInbox(inbox.emailAddress);
```

## Import Methods

### Import from Object

```typescript
importInbox(data: ExportedInboxData): Promise<Inbox>
```

Imports inbox data from a JavaScript object:

```javascript
const exportedData = JSON.parse(fs.readFileSync('./backup.json', 'utf8'));

const inbox = await client.importInbox(exportedData);
console.log(`Imported: ${inbox.emailAddress}`);

// Use inbox normally
const emails = await inbox.listEmails();
```

### Import from File

```typescript
importInboxFromFile(filePath: string): Promise<Inbox>
```

Directly imports an inbox from a JSON file:

```javascript
const inbox = await client.importInboxFromFile('./backups/inbox.json');
console.log(`Imported: ${inbox.emailAddress}`);

// Monitor for new emails
const subscription = inbox.onNewEmail((email) => {
	console.log(`New email: ${email.subject}`);
});
```

## Import Validation

The SDK validates imported data and throws errors for invalid imports:

```javascript
import { InvalidImportDataError, InboxAlreadyExistsError } from '@vaultsandbox/client';

try {
	const inbox = await client.importInbox(data);
} catch (error) {
	if (error instanceof InvalidImportDataError) {
		console.error('Invalid import data:', error.message);
		// Possible causes:
		// - Missing required fields
		// - Invalid encryption keys
		// - Server URL mismatch
		// - Corrupted JSON
	} else if (error instanceof InboxAlreadyExistsError) {
		console.error('Inbox already imported in this client');
		// The inbox is already available in this client instance
	}
}
```

## Complete Examples

### Manual Testing Workflow

```javascript
// scripts/export-test-inbox.ts
import { VaultSandboxClient } from '@vaultsandbox/client';
import fs from 'fs';

async function createTestInbox() {
	const client = new VaultSandboxClient({
		url: process.env.VAULTSANDBOX_URL,
		apiKey: process.env.VAULTSANDBOX_API_KEY,
	});

	const inbox = await client.createInbox();

	console.log(`Created test inbox: ${inbox.emailAddress}`);
	console.log(`Expires at: ${inbox.expiresAt.toISOString()}`);

	// Export for manual use
	await client.exportInboxToFile(inbox, './tmp/test-inbox.json');
	console.log('Exported to: ./tmp/test-inbox.json');

	console.log('\nSend test emails to this address, then run:');
	console.log('  npx tsx scripts/check-test-inbox.ts');
}

createTestInbox().catch(console.error);
```

```javascript
// scripts/check-test-inbox.ts
import { VaultSandboxClient } from '@vaultsandbox/client';
import fs from 'fs';

async function checkTestInbox() {
	const client = new VaultSandboxClient({
		url: process.env.VAULTSANDBOX_URL,
		apiKey: process.env.VAULTSANDBOX_API_KEY,
	});

	// Import the test inbox
	const inbox = await client.importInboxFromFile('./tmp/test-inbox.json');
	console.log(`Monitoring: ${inbox.emailAddress}\n`);

	// Show existing emails
	const emails = await inbox.listEmails();
	console.log(`Found ${emails.length} existing emails:\n`);

	emails.forEach((email, i) => {
		console.log(`${i + 1}. "${email.subject}" from ${email.from}`);
		console.log(`   Received: ${email.receivedAt.toLocaleString()}`);
		console.log(`   Links: ${email.links.length}`);
		console.log();
	});

	// Monitor for new emails
	console.log('Waiting for new emails (Ctrl+C to exit)...\n');

	inbox.onNewEmail((email) => {
		console.log(`📧 New email received!`);
		console.log(`   Subject: ${email.subject}`);
		console.log(`   From: ${email.from}`);
		console.log(`   Received: ${email.receivedAt.toLocaleString()}`);
		console.log();
	});
}

checkTestInbox().catch(console.error);
```

### Test Debugging Workflow

```javascript
// jest.config.js
module.exports = {
	reporters: ['default', ['./test-utils/inbox-export-reporter.js', { outputDir: './debug' }]],
};
```

```javascript
// test-utils/inbox-export-reporter.js
const fs = require('fs');
const path = require('path');

class InboxExportReporter {
	constructor(globalConfig, options) {
		this._globalConfig = globalConfig;
		this._options = options;
	}

	onTestResult(test, testResult, aggregatedResult) {
		testResult.testResults.forEach((result) => {
			if (result.status === 'failed' && result.inbox) {
				const filename = `inbox-${result.fullName.replace(/\s+/g, '-')}-${Date.now()}.json`;
				const filepath = path.join(this._options.outputDir, filename);

				fs.mkdirSync(this._options.outputDir, { recursive: true });
				fs.writeFileSync(filepath, JSON.stringify(result.inbox, null, 2));

				console.log(`Exported failed test inbox to: ${filepath}`);
			}
		});
	}
}

module.exports = InboxExportReporter;
```

### Cross-Environment Sync

```javascript
// scripts/sync-inbox-to-staging.ts
import { VaultSandboxClient } from '@vaultsandbox/client';
import fs from 'fs';

async function syncInbox() {
	// Export from development
	const devClient = new VaultSandboxClient({
		url: 'https://dev.vaultsandbox.com',
		apiKey: process.env.DEV_API_KEY,
	});

	const devInbox = await devClient.createInbox();
	console.log(`Created dev inbox: ${devInbox.emailAddress}`);

	// Export
	const exportData = devInbox.export();
	const exportPath = './tmp/staging-sync.json';
	fs.writeFileSync(exportPath, JSON.stringify(exportData, null, 2));

	console.log(`Exported to: ${exportPath}`);
	console.log('\nRun in staging environment:');
	console.log('  npx tsx scripts/import-from-dev.ts');

	// Keep inbox alive
	console.log('\nInbox will remain active for manual testing...');
	await new Promise(() => {}); // Keep running
}

syncInbox().catch(console.error);
```

```javascript
// scripts/import-from-dev.ts
import { VaultSandboxClient } from '@vaultsandbox/client';

async function importFromDev() {
	const stagingClient = new VaultSandboxClient({
		url: 'https://dev.vaultsandbox.com', // Same server!
		apiKey: process.env.STAGING_API_KEY,
	});

	const inbox = await stagingClient.importInboxFromFile('./tmp/staging-sync.json');

	console.log(`Imported inbox: ${inbox.emailAddress}`);
	console.log('Checking for emails...\n');

	const emails = await inbox.listEmails();
	emails.forEach((email) => {
		console.log(`- ${email.subject} (${email.from})`);
	});
}

importFromDev().catch(console.error);
```

## Best Practices

### 1. Secure Storage

Never store exported data in plaintext:

```javascript
import crypto from 'crypto';

function exportInboxSecurely(inbox, password) {
	const data = inbox.export();
	const json = JSON.stringify(data);

	// Encrypt with password
	const cipher = crypto.createCipher('aes-256-cbc', password);
	let encrypted = cipher.update(json, 'utf8', 'hex');
	encrypted += cipher.final('hex');

	return encrypted;
}

function importInboxSecurely(encryptedData, password) {
	const decipher = crypto.createDecipher('aes-256-cbc', password);
	let decrypted = decipher.update(encryptedData, 'hex', 'utf8');
	decrypted += decipher.final('utf8');

	return JSON.parse(decrypted);
}
```

### 2. Server URL Matching

Imported inboxes must be used with the same server:

```javascript
// Export from server A
const clientA = new VaultSandboxClient({
	url: 'https://server-a.vaultsandbox.com',
	apiKey: 'key-a',
});
const inbox = await clientA.createInbox();
const data = inbox.export();

// Import must use same server
const clientB = new VaultSandboxClient({
	url: 'https://server-a.vaultsandbox.com', // ✅ Same server
	apiKey: 'key-b', // Different API key is OK
});

await clientB.importInbox(data); // Works

// Wrong server will fail
const clientC = new VaultSandboxClient({
	url: 'https://server-c.vaultsandbox.com', // ❌ Different server
	apiKey: 'key-c',
});

await clientC.importInbox(data); // Throws InvalidImportDataError
```

### 3. Clean Up Exported Inboxes

Delete inboxes when done to avoid quota issues:

```javascript
async function debugWithImportedInbox(filepath) {
	const client = new VaultSandboxClient({ url, apiKey });

	try {
		const inbox = await client.importInboxFromFile(filepath);

		// Debug...
		const emails = await inbox.listEmails();
		console.log(`Found ${emails.length} emails`);
	} finally {
		// Clean up if you're done
		await inbox.delete();
	}
}
```

### 4. Version Exported Data

Include metadata in exports for tracking:

```javascript
function exportWithMetadata(inbox) {
	const data = inbox.export();

	return {
		version: '1.0',
		exportedAt: new Date().toISOString(),
		exportedBy: process.env.USER,
		environment: process.env.NODE_ENV,
		inbox: data,
	};
}

function importWithMetadata(data) {
	console.log(`Import from: ${data.exportedBy}`);
	console.log(`Exported at: ${data.exportedAt}`);
	console.log(`Environment: ${data.environment}`);

	return client.importInbox(data.inbox);
}
```

## Next Steps

- [Delivery Strategies](/client-node/advanced/strategies) - SSE vs Polling
- [Error Handling](/client-node/api/errors) - Handle import errors
- [VaultSandboxClient API](/client-node/api/client) - Client import/export methods
