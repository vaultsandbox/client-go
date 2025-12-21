---
title: VaultSandboxClient API
description: Complete API reference for the VaultSandboxClient class
---

The `VaultSandboxClient` is the main entry point for interacting with the VaultSandbox Gateway. It handles authentication, inbox creation, and provides utility methods for managing inboxes.

## Constructor

```typescript
new VaultSandboxClient(config: ClientConfig)
```

Creates a new VaultSandbox client instance.

### ClientConfig

Configuration options for the client.

```typescript
interface ClientConfig {
	url: string;
	apiKey: string;
	strategy?: 'sse' | 'polling' | 'auto';
	pollingInterval?: number;
	maxRetries?: number;
	retryDelay?: number;
	retryOn?: number[];
	sseReconnectInterval?: number;
	sseMaxReconnectAttempts?: number;
}
```

#### Properties

| Property                  | Type                           | Required | Default                          | Description                                         |
| ------------------------- | ------------------------------ | -------- | -------------------------------- | --------------------------------------------------- |
| `url`                     | `string`                       | Yes      | -                                | Gateway URL (e.g., `https://smtp.vaultsandbox.com`) |
| `apiKey`                  | `string`                       | Yes      | -                                | Your API authentication key                         |
| `strategy`                | `'sse' \| 'polling' \| 'auto'` | No       | `'auto'`                         | Email delivery strategy                             |
| `pollingInterval`         | `number`                       | No       | `2000`                           | Polling interval in milliseconds                    |
| `maxRetries`              | `number`                       | No       | `3`                              | Maximum retry attempts for HTTP requests            |
| `retryDelay`              | `number`                       | No       | `1000`                           | Base delay in milliseconds between retries          |
| `retryOn`                 | `number[]`                     | No       | `[408, 429, 500, 502, 503, 504]` | HTTP status codes that trigger a retry              |
| `sseReconnectInterval`    | `number`                       | No       | `5000`                           | Initial delay before SSE reconnection (ms)          |
| `sseMaxReconnectAttempts` | `number`                       | No       | `10`                             | Maximum SSE reconnection attempts                   |

#### Example

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

const client = new VaultSandboxClient({
	url: 'https://smtp.vaultsandbox.com',
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'auto',
	maxRetries: 5,
	retryDelay: 2000,
});
```

## Methods

### createInbox()

Creates a new email inbox with automatic key generation and encryption setup.

```typescript
createInbox(options?: CreateInboxOptions): Promise<Inbox>
```

#### Parameters

- `options` (optional): Configuration for the inbox

```typescript
interface CreateInboxOptions {
	ttl?: number;
	emailAddress?: string;
}
```

| Property       | Type     | Description                                                                                |
| -------------- | -------- | ------------------------------------------------------------------------------------------ |
| `ttl`          | `number` | Time-to-live for the inbox in seconds (min: 60, max: 604800, default: server's defaultTtl) |
| `emailAddress` | `string` | Request a specific email address (max 254 chars, e.g., `test@inbox.vaultsandbox.com`)      |

#### Returns

`Promise<Inbox>` - The created inbox instance

#### Example

```javascript
// Create inbox with default settings
const inbox = await client.createInbox();
console.log(inbox.emailAddress);

// Create inbox with custom TTL (1 hour)
const inbox = await client.createInbox({ ttl: 3600 });

// Request specific email address
const inbox = await client.createInbox({
	emailAddress: 'mytest@inbox.vaultsandbox.com',
});
```

#### Errors

- `ApiError` - API-level error (invalid request, permission denied)
- `NetworkError` - Network connection failure
- `InboxAlreadyExistsError` - Requested email address is already in use

---

### deleteAllInboxes()

Deletes all inboxes associated with the current API key. Useful for cleanup in test environments.

```typescript
deleteAllInboxes(): Promise<number>
```

#### Returns

`Promise<number>` - Number of inboxes deleted

#### Example

```javascript
const deleted = await client.deleteAllInboxes();
console.log(`Deleted ${deleted} inboxes`);
```

#### Best Practice

Use this in test cleanup to avoid orphaned inboxes:

```javascript
afterAll(async () => {
	const deleted = await client.deleteAllInboxes();
	if (deleted > 0) {
		console.log(`Cleaned up ${deleted} orphaned inboxes`);
	}
});
```

---

### getServerInfo()

Retrieves information about the VaultSandbox Gateway server.

```typescript
getServerInfo(): Promise<ServerInfo>
```

#### Returns

`Promise<ServerInfo>` - Server information object

```typescript
interface ServerInfo {
	serverSigPk: string;
	algs: {
		kem: string;
		sig: string;
		aead: string;
		kdf: string;
	};
	context: string;
	maxTtl: number;
	defaultTtl: number;
	sseConsole: boolean;
	allowedDomains: string[];
}
```

| Property         | Type       | Description                                               |
| ---------------- | ---------- | --------------------------------------------------------- |
| `serverSigPk`    | `string`   | Base64URL-encoded server signing public key for ML-DSA-65 |
| `algs`           | `object`   | Cryptographic algorithms supported by the server          |
| `algs.kem`       | `string`   | Key encapsulation mechanism (e.g., `ML-KEM-768`)          |
| `algs.sig`       | `string`   | Digital signature algorithm (e.g., `ML-DSA-65`)           |
| `algs.aead`      | `string`   | Authenticated encryption (e.g., `AES-256-GCM`)            |
| `algs.kdf`       | `string`   | Key derivation function (e.g., `HKDF-SHA-512`)            |
| `context`        | `string`   | Context string for the encryption scheme                  |
| `maxTtl`         | `number`   | Maximum time-to-live for inboxes in seconds               |
| `defaultTtl`     | `number`   | Default time-to-live for inboxes in seconds               |
| `sseConsole`     | `boolean`  | Whether the server SSE console is enabled                 |
| `allowedDomains` | `string[]` | List of domains allowed for inbox creation                |

#### Example

```javascript
const info = await client.getServerInfo();
console.log(`Encryption: ${info.algs.kem}`);
console.log(`Max TTL: ${info.maxTtl}s, Default TTL: ${info.defaultTtl}s`);
console.log(`Allowed domains: ${info.allowedDomains.join(', ')}`);
```

---

### checkKey()

Validates the API key with the server.

```typescript
checkKey(): Promise<boolean>
```

#### Returns

`Promise<boolean>` - `true` if the API key is valid

#### Example

```javascript
const isValid = await client.checkKey();
if (!isValid) {
	throw new Error('Invalid API key');
}
```

#### Usage

Useful for verifying configuration before running tests:

```javascript
beforeAll(async () => {
	const client = new VaultSandboxClient({
		url: process.env.VAULTSANDBOX_URL,
		apiKey: process.env.VAULTSANDBOX_API_KEY,
	});

	const isValid = await client.checkKey();
	if (!isValid) {
		throw new Error('VaultSandbox API key is invalid');
	}
});
```

---

### monitorInboxes()

Monitors multiple inboxes simultaneously and emits events when new emails arrive.

```typescript
monitorInboxes(inboxes: Inbox[]): InboxMonitor
```

#### Parameters

- `inboxes`: Array of inbox instances to monitor

#### Returns

`InboxMonitor` - Event emitter for inbox monitoring

#### Example

```javascript
const inbox1 = await client.createInbox();
const inbox2 = await client.createInbox();

const monitor = client.monitorInboxes([inbox1, inbox2]);

monitor.on('email', (inbox, email) => {
	console.log(`New email in ${inbox.emailAddress}: ${email.subject}`);
});

// Later, stop monitoring
monitor.unsubscribe();
```

See [InboxMonitor API](/client-node/api/inbox#inboxmonitor) for more details.

- [Inbox API Reference](/client-node/api/inbox) - Learn about inbox methods
- [Email API Reference](/client-node/api/email) - Work with email objects
- [Error Handling](/client-node/api/errors) - Handle errors gracefully
- [Import/Export Guide](/client-node/advanced/import-export) - Advanced import/export usage

---

### exportInbox()

Exports an inbox's data and encryption keys for backup or sharing. The exported data includes sensitive key material and should be treated as confidential.

```typescript
exportInbox(inboxOrEmail: Inbox | string): ExportedInboxData
```

#### Parameters

- `inboxOrEmail`: Inbox instance or email address to export

#### Returns

`ExportedInboxData` - Serializable inbox data including keys

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
const exportedData = client.exportInbox(inbox);

// Save for later use (treat as sensitive!)
console.log(JSON.stringify(exportedData));
```

#### Security Warning

Exported data contains private encryption keys. Store securely and never commit to version control.

---

### importInbox()

Imports a previously exported inbox, restoring all data and encryption keys.

```typescript
importInbox(data: ExportedInboxData): Promise<Inbox>
```

#### Parameters

- `data`: Previously exported inbox data

#### Returns

`Promise<Inbox>` - The imported inbox instance

#### Example

```javascript
const exportedData = JSON.parse(savedInboxData);
const inbox = await client.importInbox(exportedData);

console.log(`Imported inbox: ${inbox.emailAddress}`);

// Use inbox normally
const emails = await inbox.listEmails();
```

#### Errors

- `InboxAlreadyExistsError` - Inbox is already imported in this client
- `InvalidImportDataError` - Import data is invalid or corrupted
- `ApiError` - Server rejected the import (inbox may not exist)

---

### exportInboxToFile()

Exports an inbox to a JSON file on disk.

```typescript
async exportInboxToFile(inboxOrEmail: Inbox | string, filePath: string): Promise<void>
```

#### Parameters

- `inboxOrEmail`: Inbox instance or email address to export
- `filePath`: Path where the JSON file will be written

#### Example

```javascript
const inbox = await client.createInbox();

// Export to file
await client.exportInboxToFile(inbox, './backup/inbox.json');

console.log('Inbox exported to ./backup/inbox.json');
```

---

### importInboxFromFile()

Imports an inbox from a JSON file.

```typescript
importInboxFromFile(filePath: string): Promise<Inbox>
```

#### Parameters

- `filePath`: Path to the exported inbox JSON file

#### Returns

`Promise<Inbox>` - The imported inbox instance

#### Example

```javascript
// Import from file
const inbox = await client.importInboxFromFile('./backup/inbox.json');

console.log(`Imported inbox: ${inbox.emailAddress}`);

// Monitor for new emails
const subscription = inbox.onNewEmail((email) => {
	console.log(`New email: ${email.subject}`);
});
```

#### Use Cases

- Test reproducibility across runs
- Sharing inboxes between environments
- Manual testing workflows
- Debugging production issues

---

### close()

Closes the client, terminates any active SSE or polling connections, and cleans up resources.

```typescript
close(): Promise<void>
```

#### Example

```javascript
const client = new VaultSandboxClient({ url, apiKey });

try {
	const inbox = await client.createInbox();
	// Use inbox...
} finally {
	await client.close();
}
```

#### Best Practice

Always close the client when done, especially in long-running processes:

```javascript
let client;

beforeAll(() => {
	client = new VaultSandboxClient({ url, apiKey });
});

afterAll(async () => {
	if (client) {
		await client.close();
	}
});
```

## Complete Example

Here's a complete example showing typical client usage:

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

async function main() {
	// Create client
	const client = new VaultSandboxClient({
		url: process.env.VAULTSANDBOX_URL,
		apiKey: process.env.VAULTSANDBOX_API_KEY,
		strategy: 'auto',
		maxRetries: 5,
	});

	try {
		// Verify API key
		const isValid = await client.checkKey();
		if (!isValid) {
			throw new Error('Invalid API key');
		}

		// Get server info
		const info = await client.getServerInfo();
		console.log(`Connected to VaultSandbox (default TTL: ${info.defaultTtl}s)`);

		// Create inbox
		const inbox = await client.createInbox();
		console.log(`Created inbox: ${inbox.emailAddress}`);

		// Export for later use
		await client.exportInboxToFile(inbox, './inbox-backup.json');

		// Wait for email
		const email = await inbox.waitForEmail({
			timeout: 30000,
			subject: /Test/,
		});

		console.log(`Received: ${email.subject}`);

		// Clean up
		await inbox.delete();

		// Delete any other orphaned inboxes
		const deleted = await client.deleteAllInboxes();
		console.log(`Cleaned up ${deleted} total inboxes`);
	} finally {
		// Always close the client
		await client.close();
	}
}

main().catch(console.error);
```

## Next Steps

- [Inbox API Reference](/client-node/api/inbox) - Learn about inbox methods
- [Email API Reference](/client-node/api/email) - Work with email objects
- [Error Handling](/client-node/api/errors) - Handle errors gracefully
- [Import/Export Guide](/client-node/advanced/import-export) - Advanced import/export usage
