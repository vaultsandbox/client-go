---
title: Client Configuration
description: Configure the VaultSandbox client for your environment
---

This page covers all configuration options for the VaultSandbox Node.js client.

## Basic Configuration

### Creating a Client

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

const client = new VaultSandboxClient({
	url: 'https://mail.example.com',
	apiKey: 'your-api-key',
});
```

## Configuration Options

### Required Options

#### url

**Type**: `string`

**Description**: Base URL of your VaultSandbox Gateway

**Examples**:

```javascript
url: 'https://mail.example.com';
url: 'http://localhost:3000'; // Local development
```

**Requirements**:

- Must include protocol (`https://` or `http://`)
- Should not include trailing slash
- Must be accessible from your application

#### apiKey

**Type**: `string`

**Description**: API key for authentication

**Example**:

```javascript
apiKey: 'vs_1234567890abcdef...';
```

**Best practices**:

- Store in environment variables
- Never commit to version control
- Rotate periodically

### Optional Options

#### strategy

**Type**: `'sse' | 'polling' | 'auto'`

**Default**: `'auto'`

**Description**: Email delivery strategy

**Options**:

- `'auto'` - Automatically choose best strategy (tries SSE first, falls back to polling)
- `'sse'` - Server-Sent Events for real-time delivery
- `'polling'` - Poll for new emails at intervals

**Examples**:

```javascript
strategy: 'auto'; // Recommended
strategy: 'sse'; // Force SSE
strategy: 'polling'; // Force polling
```

**When to use each**:

- `'auto'`: Most use cases (recommended)
- `'sse'`: When you need real-time, low-latency delivery
- `'polling'`: When SSE is blocked by firewall/proxy

#### pollingInterval

**Type**: `number` (milliseconds)

**Default**: `2000`

**Description**: Polling interval when using polling strategy

**Examples**:

```javascript
pollingInterval: 2000; // Poll every 2 seconds (default)
pollingInterval: 5000; // Poll every 5 seconds
pollingInterval: 500; // Poll every 500ms (aggressive)
```

**Considerations**:

- Lower = more responsive, more API calls
- Higher = less API calls, slower detection
- Subject to rate limiting

#### maxRetries

**Type**: `number`

**Default**: `3`

**Description**: Maximum retry attempts for failed HTTP requests

**Examples**:

```javascript
maxRetries: 3; // Default
maxRetries: 5; // More resilient
maxRetries: 0; // No retries
```

#### retryDelay

**Type**: `number` (milliseconds)

**Default**: `1000`

**Description**: Base delay between retry attempts (uses exponential backoff)

**Examples**:

```javascript
retryDelay: 1000; // 1s, 2s, 4s, ...
retryDelay: 500; // 500ms, 1s, 2s, ...
retryDelay: 2000; // 2s, 4s, 8s, ...
```

#### retryOn

**Type**: `number[]` (HTTP status codes)

**Default**: `[408, 429, 500, 502, 503, 504]`

**Description**: HTTP status codes that trigger a retry

**Example**:

```javascript
retryOn: [408, 429, 500, 502, 503, 504]; // Default
retryOn: [500, 502, 503]; // Only server errors
retryOn: []; // Never retry
```

#### sseReconnectInterval

**Type**: `number` (milliseconds)

**Default**: `5000`

**Description**: Initial delay before SSE reconnection attempt

**Examples**:

```javascript
sseReconnectInterval: 5000; // Default
sseReconnectInterval: 1000; // Faster reconnection
sseReconnectInterval: 10000; // Slower reconnection
```

**Note**: Uses exponential backoff (5s, 10s, 20s, ...)

#### sseMaxReconnectAttempts

**Type**: `number`

**Default**: `10`

**Description**: Maximum SSE reconnection attempts before giving up

**Examples**:

```javascript
sseMaxReconnectAttempts: 10; // Default
sseMaxReconnectAttempts: Infinity; // Never give up
sseMaxReconnectAttempts: 3; // Give up quickly
```

## Configuration Examples

### Production Configuration

```javascript
const client = new VaultSandboxClient({
	// Required
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,

	// Recommended production settings
	strategy: 'auto',
	maxRetries: 5,
	retryDelay: 2000,
	sseReconnectInterval: 5000,
	sseMaxReconnectAttempts: 10,
});
```

### CI/CD Configuration

```javascript
const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,

	// Faster polling for CI
	strategy: 'auto',
	pollingInterval: 1000, // Poll every second
	maxRetries: 3,
	retryDelay: 500,
});
```

### Development Configuration

```javascript
const client = new VaultSandboxClient({
	url: 'http://localhost:3000',
	apiKey: 'dev-api-key',

	// Aggressive settings for fast feedback
	strategy: 'polling', // More reliable in dev
	pollingInterval: 500, // Very responsive
	maxRetries: 1, // Fail fast
});
```

### High-Reliability Configuration

```javascript
const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,

	// Maximum reliability
	maxRetries: 10,
	retryDelay: 2000,
	retryOn: [408, 429, 500, 502, 503, 504],
	sseReconnectInterval: 1000,
	sseMaxReconnectAttempts: Infinity,
});
```

## Environment Variables

Store configuration in environment variables:

### `.env` File

```bash
VAULTSANDBOX_URL=https://mail.example.com
VAULTSANDBOX_API_KEY=vs_1234567890abcdef...
VAULTSANDBOX_STRATEGY=auto
VAULTSANDBOX_POLLING_INTERVAL=2000
```

### Usage

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';
import 'dotenv/config'; // Load .env file

const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: process.env.VAULTSANDBOX_STRATEGY || 'auto',
	pollingInterval: parseInt(process.env.VAULTSANDBOX_POLLING_INTERVAL || '2000'),
});
```

## Client Methods

### close()

Close the client and clean up resources:

```javascript
await client.close();
```

**What it does**:

- Terminates all active SSE connections
- Stops all polling operations
- Cleans up resources

**When to use**:

- After test suite completes
- Before process exit
- When client is no longer needed

**Example**:

```javascript
const client = new VaultSandboxClient({ url, apiKey });

try {
	// Use client
	const inbox = await client.createInbox();
	// ...
} finally {
	await client.close();
}
```

## Strategy Selection Guide

### Auto (Recommended)

**Use when**: You want optimal performance with automatic fallback

**Behavior**:

1. Tries SSE first
2. Falls back to polling if SSE fails
3. Automatically reconnects on errors

**Pros**:

- Best of both worlds
- No manual configuration needed
- Resilient to network issues

**Cons**:

- Slightly more complex internally

### SSE (Server-Sent Events)

**Use when**: You need real-time, low-latency delivery

**Behavior**:

- Persistent connection to server
- Push-based email notification
- Instant delivery

**Pros**:

- Real-time delivery (no polling delay)
- Efficient (no repeated HTTP requests)
- Deterministic tests

**Cons**:

- Requires persistent connection
- May be blocked by some proxies/firewalls
- More complex error handling

### Polling

**Use when**: SSE is blocked or unreliable

**Behavior**:

- Periodic HTTP requests for new emails
- Pull-based email retrieval
- Configurable interval

**Pros**:

- Works in all network environments
- No persistent connection required
- Simple and predictable

**Cons**:

- Delay based on polling interval
- More HTTP requests
- Less efficient than SSE

## Best Practices

### Security

**✅ DO**:

```javascript
// Use environment variables
const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,
});
```

**❌ DON'T**:

```javascript
// Hard-code credentials
const client = new VaultSandboxClient({
	url: 'https://mail.example.com',
	apiKey: 'vs_1234567890...', // ❌ Never do this
});
```

### Resource Management

**✅ DO**:

```javascript
const client = new VaultSandboxClient({ url, apiKey });

try {
	await runTests();
} finally {
	await client.close(); // Always clean up
}
```

**❌ DON'T**:

```javascript
const client = new VaultSandboxClient({ url, apiKey });
await runTests();
// ❌ Forgot to close, resources leak
```

### Error Handling

**✅ DO**:

```javascript
try {
	const inbox = await client.createInbox();
} catch (error) {
	if (error instanceof ApiError) {
		console.error('API error:', error.statusCode, error.message);
	} else if (error instanceof NetworkError) {
		console.error('Network error:', error.message);
	} else {
		console.error('Unexpected error:', error);
	}
}
```

## Next Steps

- **[Core Concepts: Inboxes](/client-node/concepts/inboxes/)** - Learn about inboxes
- **[Managing Inboxes](/client-node/guides/managing-inboxes/)** - Common inbox operations
- **[Testing Patterns](/client-node/testing/password-reset/)** - Integrate with your tests
- **[API Reference: Client](/client-node/api/client/)** - Full API documentation
