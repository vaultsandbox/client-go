---
title: Delivery Strategies
description: Learn about SSE and polling delivery strategies in VaultSandbox Client
---

VaultSandbox Client supports two email delivery strategies: **Server-Sent Events (SSE)** for real-time updates and **Polling** for compatibility. The SDK intelligently chooses the best strategy automatically or allows manual configuration.

## Overview

When you wait for emails or subscribe to new email notifications, the SDK needs to know when emails arrive. It does this using one of two strategies:

1. **SSE (Server-Sent Events)**: Real-time push notifications from the server
2. **Polling**: Periodic checking for new emails

## Strategy Comparison

| Feature             | SSE                             | Polling                     |
| ------------------- | ------------------------------- | --------------------------- |
| **Latency**         | Near-instant (~100ms)           | Poll interval (default: 2s) |
| **Server Load**     | Lower (persistent connection)   | Higher (repeated requests)  |
| **Network Traffic** | Lower (only when emails arrive) | Higher (constant polling)   |
| **Compatibility**   | Requires persistent connections | Works everywhere            |
| **Firewall/Proxy**  | May be blocked                  | Always works                |
| **Battery Impact**  | Lower (push-based)              | Higher (constant requests)  |

## Auto Strategy (Recommended)

The default `auto` strategy automatically selects the best delivery method:

1. **Tries SSE first** - Attempts to establish an SSE connection
2. **Falls back to polling** - If SSE fails or is unavailable, uses polling
3. **Adapts to environment** - Works seamlessly in different network conditions

```javascript
import { VaultSandboxClient } from '@vaultsandbox/client';

// Auto strategy (default)
const client = new VaultSandboxClient({
	url: 'https://smtp.vaultsandbox.com',
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'auto', // Default, can be omitted
});

// SDK will automatically choose the best strategy
const inbox = await client.createInbox();
const email = await inbox.waitForEmail({ timeout: 10000 });
```

### When Auto Chooses SSE

- Gateway supports SSE
- Network allows persistent connections
- No restrictive proxy/firewall

### When Auto Falls Back to Polling

- Gateway doesn't support SSE
- SSE connection fails
- Behind restrictive proxy/firewall
- Network requires periodic reconnection

## SSE Strategy

Server-Sent Events provide real-time push notifications when emails arrive.

### Advantages

- **Near-instant delivery**: Emails appear within milliseconds
- **Lower server load**: Single persistent connection
- **Efficient**: Only transmits when emails arrive
- **Battery-friendly**: No constant polling

### Configuration

```javascript
const client = new VaultSandboxClient({
	url: 'https://smtp.vaultsandbox.com',
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'sse',
	sseReconnectInterval: 5000, // Wait 5s before reconnecting
	sseMaxReconnectAttempts: 10, // Try up to 10 reconnections
});
```

### SSE Configuration Options

| Option                    | Type     | Default | Description                            |
| ------------------------- | -------- | ------- | -------------------------------------- |
| `sseReconnectInterval`    | `number` | `5000`  | Initial delay before reconnection (ms) |
| `sseMaxReconnectAttempts` | `number` | `10`    | Maximum reconnection attempts          |

### Reconnection Behavior

SSE uses **exponential backoff** for reconnections:

```
1st attempt: sseReconnectInterval (5s)
2nd attempt: sseReconnectInterval * 2 (10s)
3rd attempt: sseReconnectInterval * 4 (20s)
...up to sseMaxReconnectAttempts
```

### Example Usage

```javascript
const client = new VaultSandboxClient({
	url: 'https://smtp.vaultsandbox.com',
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'sse',
});

const inbox = await client.createInbox();

// Real-time subscription (uses SSE)
const subscription = inbox.onNewEmail((email) => {
	console.log(`Instant notification: ${email.subject}`);
});

// Waiting also uses SSE (faster than polling)
const email = await inbox.waitForEmail({
	timeout: 10000,
	subject: /Welcome/,
});

subscription.unsubscribe();
```

### When to Use SSE

- **Real-time monitoring**: When you need instant email notifications
- **Long-running tests**: Reduces overall test time
- **High email volume**: More efficient than polling
- **Development/local**: Fast feedback during development

### Limitations

- Requires persistent HTTP connection support
- May not work behind some corporate proxies
- Some cloud environments may close long-lived connections
- Requires server-side SSE support

## Polling Strategy

Polling periodically checks for new emails at a configured interval.

### Advantages

- **Universal compatibility**: Works in all environments
- **Firewall-friendly**: Standard HTTP requests
- **Predictable**: Easy to reason about behavior
- **Resilient**: Automatically recovers from transient failures

### Configuration

```javascript
const client = new VaultSandboxClient({
	url: 'https://smtp.vaultsandbox.com',
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'polling',
	pollingInterval: 2000, // Check every 2 seconds
});
```

### Polling Configuration Options

| Option            | Type     | Default | Description                       |
| ----------------- | -------- | ------- | --------------------------------- |
| `pollingInterval` | `number` | `2000`  | How often to poll for emails (ms) |

### Example Usage

```javascript
const client = new VaultSandboxClient({
	url: 'https://smtp.vaultsandbox.com',
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'polling',
	pollingInterval: 1000, // Poll every 1 second
});

const inbox = await client.createInbox();

// Polling-based subscription
const subscription = inbox.onNewEmail((email) => {
	console.log(`Polled notification: ${email.subject}`);
});

// Waiting uses polling (checks every pollingInterval)
const email = await inbox.waitForEmail({
	timeout: 10000,
	subject: /Welcome/,
});

subscription.unsubscribe();
```

### Choosing Poll Interval

Different intervals suit different scenarios:

```javascript
// Fast polling (500ms) - Development/local testing
const fastClient = new VaultSandboxClient({
	url: 'http://localhost:3000',
	apiKey: 'dev-key',
	strategy: 'polling',
	pollingInterval: 500,
});

// Standard polling (2000ms) - Default, good balance
const standardClient = new VaultSandboxClient({
	url: 'https://smtp.vaultsandbox.com',
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'polling',
	pollingInterval: 2000,
});

// Slow polling (5000ms) - CI/CD or rate-limited environments
const slowClient = new VaultSandboxClient({
	url: 'https://smtp.vaultsandbox.com',
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'polling',
	pollingInterval: 5000,
});
```

### When to Use Polling

- **Corporate networks**: Restrictive firewall/proxy environments
- **CI/CD pipelines**: Guaranteed compatibility
- **Rate-limited APIs**: Avoid hitting request limits
- **Debugging**: Predictable request timing
- **Low email volume**: Polling overhead is minimal

### Performance Optimization

For `waitForEmailCount()`, you can override the polling interval:

```javascript
// Default client polling: 2s
const client = new VaultSandboxClient({
	url: 'https://smtp.vaultsandbox.com',
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'polling',
	pollingInterval: 2000,
});

// Override for specific operation (faster)
await inbox.waitForEmailCount(5, {
	timeout: 30000,
	pollInterval: 1000, // Check every 1s for this operation
});
```

## Choosing the Right Strategy

### Use Auto (Default)

For most use cases, let the SDK choose:

```javascript
const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	// strategy: 'auto' is implicit
});
```

**Best for:**

- General testing
- Unknown network conditions
- Mixed environments (dev, staging, CI)
- When you want it to "just work"

### Force SSE

When you need guaranteed real-time performance:

```javascript
const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'sse',
});
```

**Best for:**

- Local development (known to support SSE)
- Real-time monitoring dashboards
- High-volume email testing
- Latency-sensitive tests

**Caveat:** Will throw `SSEError` if SSE is unavailable.

### Force Polling

When compatibility is more important than speed:

```javascript
const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'polling',
	pollingInterval: process.env.CI ? 3000 : 1000,
});
```

**Best for:**

- CI/CD environments (guaranteed to work)
- Corporate networks with restrictive proxies
- When SSE is known to be problematic
- Rate-limited scenarios

## Environment-Specific Configuration

### Development

Fast feedback with SSE:

```javascript
// .env.development
VAULTSANDBOX_URL=http://localhost:3000
VAULTSANDBOX_STRATEGY=sse

// config.js
const client = new VaultSandboxClient({
  url: process.env.VAULTSANDBOX_URL,
  apiKey: process.env.VAULTSANDBOX_API_KEY,
  strategy: process.env.VAULTSANDBOX_STRATEGY || 'auto',
});
```

### CI/CD

Reliable polling:

```javascript
// .env.ci
VAULTSANDBOX_URL=https://smtp.vaultsandbox.com
VAULTSANDBOX_STRATEGY=polling
VAULTSANDBOX_POLLING_INTERVAL=3000

// config.js
const client = new VaultSandboxClient({
  url: process.env.VAULTSANDBOX_URL,
  apiKey: process.env.VAULTSANDBOX_API_KEY,
  strategy: process.env.VAULTSANDBOX_STRATEGY || 'auto',
  pollingInterval: parseInt(process.env.VAULTSANDBOX_POLLING_INTERVAL || '2000'),
});
```

### Production Testing

Auto with tuned reconnection:

```javascript
const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'auto',
	// SSE config (if available)
	sseReconnectInterval: 10000,
	sseMaxReconnectAttempts: 5,
	// Polling fallback config
	pollingInterval: 5000,
});
```

## Monitoring Strategy Performance

### Check Active Strategy

```javascript
const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'auto',
});

const inbox = await client.createInbox();

// Subscribe and check what strategy is being used
const subscription = inbox.onNewEmail((email) => {
	console.log(`Email received: ${email.subject}`);
});

// The SDK will log which strategy it's using
// Check logs for: "Using SSE strategy" or "Using polling strategy"
```

### Measure Email Delivery Latency

```javascript
async function measureDeliveryLatency() {
	const client = new VaultSandboxClient({ url, apiKey });
	const inbox = await client.createInbox();

	const startTime = Date.now();

	// Send email
	await sendTestEmail(inbox.emailAddress);

	// Wait for email
	const email = await inbox.waitForEmail({ timeout: 30000 });

	const latency = Date.now() - startTime;
	console.log(`Email delivery latency: ${latency}ms`);

	await inbox.delete();
}
```

### Compare Strategies

```javascript
async function compareStrategies() {
	// Test SSE
	const sseClient = new VaultSandboxClient({
		url,
		apiKey,
		strategy: 'sse',
	});
	const sseInbox = await sseClient.createInbox();
	const sseStart = Date.now();
	await sendTestEmail(sseInbox.emailAddress);
	await sseInbox.waitForEmail({ timeout: 10000 });
	const sseLatency = Date.now() - sseStart;

	// Test Polling
	const pollClient = new VaultSandboxClient({
		url,
		apiKey,
		strategy: 'polling',
		pollingInterval: 2000,
	});
	const pollInbox = await pollClient.createInbox();
	const pollStart = Date.now();
	await sendTestEmail(pollInbox.emailAddress);
	await pollInbox.waitForEmail({ timeout: 10000 });
	const pollLatency = Date.now() - pollStart;

	console.log(`SSE latency: ${sseLatency}ms`);
	console.log(`Polling latency: ${pollLatency}ms`);
	console.log(`Difference: ${pollLatency - sseLatency}ms`);

	await sseInbox.delete();
	await pollInbox.delete();
}
```

## Troubleshooting

### SSE Connection Failures

```javascript
import { SSEError } from '@vaultsandbox/client';

try {
	const client = new VaultSandboxClient({
		url: process.env.VAULTSANDBOX_URL,
		apiKey: process.env.VAULTSANDBOX_API_KEY,
		strategy: 'sse',
	});

	const inbox = await client.createInbox();
	// Use inbox...
} catch (error) {
	if (error instanceof SSEError) {
		console.error('SSE failed:', error.message);
		console.log('Falling back to polling...');

		// Recreate with polling
		const client = new VaultSandboxClient({
			url: process.env.VAULTSANDBOX_URL,
			apiKey: process.env.VAULTSANDBOX_API_KEY,
			strategy: 'polling',
		});

		const inbox = await client.createInbox();
		// Continue with polling...
	}
}
```

### Polling Too Slow

If emails arrive slowly with polling:

```javascript
// Problem: Default 2s polling is too slow
const client = new VaultSandboxClient({
	url,
	apiKey,
	strategy: 'polling',
	pollingInterval: 2000, // Check every 2s
});

// Solution 1: Faster polling
const fasterClient = new VaultSandboxClient({
	url,
	apiKey,
	strategy: 'polling',
	pollingInterval: 500, // Check every 500ms
});

// Solution 2: Use SSE if available
const sseClient = new VaultSandboxClient({
	url,
	apiKey,
	strategy: 'sse', // Real-time delivery
});

// Solution 3: Override for specific wait
await inbox.waitForEmail({
	timeout: 10000,
	pollInterval: 500, // Fast polling for this operation
});
```

## Best Practices

### 1. Use Auto Strategy by Default

Let the SDK choose unless you have specific requirements:

```javascript
// Good: Let SDK choose
const client = new VaultSandboxClient({ url, apiKey });

// Only specify when needed
const ciClient = new VaultSandboxClient({
	url,
	apiKey,
	strategy: 'polling', // CI needs guaranteed compatibility
});
```

### 2. Tune for Environment

Configure differently for each environment:

```javascript
function createClient() {
	const config = {
		url: process.env.VAULTSANDBOX_URL,
		apiKey: process.env.VAULTSANDBOX_API_KEY,
	};

	if (process.env.CI) {
		// CI: Reliable polling
		config.strategy = 'polling';
		config.pollingInterval = 3000;
	} else if (process.env.NODE_ENV === 'development') {
		// Dev: Fast SSE
		config.strategy = 'sse';
	} else {
		// Production: Auto with tuning
		config.strategy = 'auto';
		config.sseReconnectInterval = 5000;
		config.pollingInterval = 2000;
	}

	return new VaultSandboxClient(config);
}
```

### 3. Handle SSE Gracefully

Always have a fallback if forcing SSE:

```javascript
async function createClientWithFallback() {
	try {
		return new VaultSandboxClient({ url, apiKey, strategy: 'sse' });
	} catch (error) {
		if (error instanceof SSEError) {
			console.warn('SSE unavailable, using polling');
			return new VaultSandboxClient({ url, apiKey, strategy: 'polling' });
		}
		throw error;
	}
}
```

### 4. Don't Poll Too Aggressively

Avoid very short polling intervals in production:

```javascript
// Avoid: Too aggressive
const client = new VaultSandboxClient({
	url,
	apiKey,
	strategy: 'polling',
	pollingInterval: 100, // 100ms - too frequent!
});

// Good: Reasonable interval
const client = new VaultSandboxClient({
	url,
	apiKey,
	strategy: 'polling',
	pollingInterval: 2000, // 2s - balanced
});
```

## Next Steps

- [Real-time Monitoring Guide](/client-node/guides/real-time) - Using subscriptions
- [Configuration Reference](/client-node/configuration) - All config options
- [Error Handling](/client-node/api/errors) - Handle SSE errors
- [CI/CD Integration](/client-node/testing/cicd) - Strategy for CI environments
