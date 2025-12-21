---
title: Client Installation
description: Install and set up the VaultSandbox Node.js client SDK
---

The `@vaultsandbox/client` SDK provides a developer-friendly interface for integrating email testing into your Node.js applications and test suites.

## Requirements

- **Node.js**: 20.0.0 or higher
- **Platform**: Node.js only (not supported in browsers or edge runtimes)
- **VaultSandbox Gateway**: Running instance with API access

:::caution[Not for Browsers]
The VaultSandbox client uses Node.js-specific cryptography libraries and is **not compatible** with browsers or edge runtimes (Cloudflare Workers, Vercel Edge, etc.).

For browser-based testing, use the VaultSandbox Web UI.
:::

## Installation

### npm

```bash
npm install @vaultsandbox/client
```

### yarn

```bash
yarn add @vaultsandbox/client
```

### pnpm

```bash
pnpm add @vaultsandbox/client
```

## Quick Start

### Basic Usage

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

const client = new VaultSandboxClient({
	url: 'https://mail.example.com',
	apiKey: 'your-api-key',
});

const inbox = await client.createInbox();
console.log(`Send email to: ${inbox.emailAddress}`);

const email = await inbox.waitForEmail({ timeout: 30000 });
console.log('Received:', email.subject);

await inbox.delete();
```

## Verifying Installation

Create a test file `test-vaultsandbox.js`:

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

async function test() {
	const client = new VaultSandboxClient({
		url: process.env.VAULTSANDBOX_URL,
		apiKey: process.env.VAULTSANDBOX_API_KEY,
	});

	try {
		const serverInfo = await client.getServerInfo();
		console.log('✅ Connected to VaultSandbox');
		console.log('Allowed domains:', serverInfo.allowedDomains);

		const inbox = await client.createInbox();
		console.log('✅ Created inbox:', inbox.emailAddress);

		await inbox.delete();
		console.log('✅ Cleanup successful');

		console.log('\n🎉 Installation verified!');
	} catch (error) {
		console.error('❌ Error:', error.message);
		process.exit(1);
	}
}

test();
```

Run it:

```bash
export VAULTSANDBOX_URL=https://mail.example.com
export VAULTSANDBOX_API_KEY=your-api-key
node test-vaultsandbox.js
```

## TypeScript Support

The SDK is written in TypeScript and includes full type definitions.

### TypeScript Configuration

Ensure your `tsconfig.json` has:

```json
{
	"compilerOptions": {
		"target": "ES2022",
		"module": "ES2022",
		"moduleResolution": "node",
		"esModuleInterop": true,
		"strict": true
	}
}
```

### TypeScript Example

```typescript
import { VaultSandboxClient, Email, Inbox } from '@vaultsandbox/client';

const client: VaultSandboxClient = new VaultSandboxClient({
	url: 'https://mail.example.com',
	apiKey: process.env.VAULTSANDBOX_API_KEY!,
});

const inbox: Inbox = await client.createInbox();
const email: Email = await inbox.waitForEmail({ timeout: 30000 });

console.log(email.subject); // TypeScript knows this is a string
```

## ESM vs CommonJS

The VaultSandbox client is distributed as **ESM (ES Modules)** only.

### ESM (Recommended)

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';
```

**package.json**:

```json
{
	"type": "module"
}
```

### CommonJS (Requires Workaround)

If you must use CommonJS, use dynamic import:

```javascript
async function loadClient() {
	const { VaultSandboxClient } = await import('@vaultsandbox/client');
	return VaultSandboxClient;
}

const VaultSandboxClient = await loadClient();
const client = new VaultSandboxClient({ url, apiKey });
```

## Next Steps

- **[Configuration](/client-node/configuration/)** - Configure the client for your environment
- **[Core Concepts](/client-node/concepts/inboxes/)** - Understand inboxes, emails, and authentication
- **[Guides](/client-node/guides/managing-inboxes/)** - Learn common usage patterns
- **[Testing Patterns](/client-node/testing/password-reset/)** - Integrate with your test suite
