---
title: Node.js Client
description: Overview of the VaultSandbox Node.js SDK for email testing
---

The official Node.js SDK for VaultSandbox Gateway. It handles quantum-safe encryption automatically, letting you focus on testing email workflows.

## Key Capabilities

- **Automatic Encryption**: ML-KEM-768 key encapsulation + AES-256-GCM encryption handled transparently
- **Real-Time Delivery**: SSE-based email delivery with smart polling fallback
- **Email Authentication**: Built-in SPF/DKIM/DMARC validation helpers
- **Full Email Access**: Decrypted content, headers, links, and attachments
- **TypeScript Support**: Comprehensive type definitions included

## Requirements

- Node.js 20+ (not supported in browsers or edge runtimes)
- VaultSandbox Gateway server
- Valid API key

## Gateway Server

The SDK connects to a VaultSandbox Gateway - a receive-only SMTP server you self-host. It handles email reception, authentication validation, and encryption. You can run one with Docker in minutes.

See [Gateway Overview](/gateway/) or jump to [Quick Start](/getting-started/quickstart/) to deploy one.

## Quick Example

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

const client = new VaultSandboxClient({
	url: 'https://gateway.example.com',
	apiKey: 'your-api-key',
});

// Create inbox (keypair generated automatically)
const inbox = await client.createInbox();

// Send email to inbox.emailAddress from your application...

// Wait for email
const email = await inbox.waitForEmail({ timeout: 30000 });

console.log('Subject:', email.subject);
console.log('Text:', email.text);

// Cleanup
await inbox.delete();
```

## Links

- [GitHub Repository](https://github.com/vaultsandbox/client-node)
- [npm Package](https://www.npmjs.com/package/@vaultsandbox/client)

## Next Steps

- [Installation](/client-node/installation/) - Install the SDK
- [Configuration](/client-node/configuration/) - Client options and setup
- [Core Concepts](/client-node/concepts/inboxes/) - Inboxes, emails, and authentication
- [API Reference](/client-node/api/client/) - Full API documentation
