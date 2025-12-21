---
title: Real-time Monitoring
description: Subscribe to emails as they arrive using Server-Sent Events
---

VaultSandbox supports real-time email notifications via Server-Sent Events (SSE), enabling instant processing of emails as they arrive.

## Basic Subscription

### Subscribe to Single Inbox

```javascript
const inbox = await client.createInbox();

console.log(`Monitoring: ${inbox.emailAddress}`);

const subscription = inbox.onNewEmail((email) => {
	console.log(`📧 New email: ${email.subject}`);
	console.log(`   From: ${email.from}`);
	console.log(`   Received: ${email.receivedAt}`);
});

// Later, stop monitoring
// subscription.unsubscribe();
```

### Subscribe with Processing

```javascript
const subscription = inbox.onNewEmail(async (email) => {
	console.log('Processing:', email.subject);

	// Extract links
	if (email.links.length > 0) {
		console.log('Links found:', email.links);
	}

	// Check authentication
	const auth = email.authResults.validate();
	if (!auth.passed) {
		console.warn('Authentication failed:', auth.failures);
	}

	// Mark as processed
	await email.markAsRead();
});
```

## Monitoring Multiple Inboxes

### Using InboxMonitor

```javascript
const inbox1 = await client.createInbox();
const inbox2 = await client.createInbox();
const inbox3 = await client.createInbox();

const monitor = client.monitorInboxes([inbox1, inbox2, inbox3]);

monitor.on('email', (inbox, email) => {
	console.log(`📧 Email in ${inbox.emailAddress}`);
	console.log(`   Subject: ${email.subject}`);
	console.log(`   From: ${email.from}`);
});

// Later, stop monitoring all
// monitor.unsubscribe();
```

### Monitoring with Handlers

```javascript
const monitor = client.monitorInboxes([inbox1, inbox2]);

monitor.on('email', async (inbox, email) => {
	if (email.from === 'alerts@example.com') {
		await handleAlert(email);
	} else if (email.subject.includes('Invoice')) {
		await handleInvoice(inbox, email);
	} else {
		console.log('Other email:', email.subject);
	}
});
```

## Unsubscribing

### Unsubscribe from Single Inbox

```javascript
const subscription = inbox.onNewEmail((email) => {
	console.log('Email:', email.subject);
});

// Unsubscribe when done
subscription.unsubscribe();
```

### Conditional Unsubscribe

```javascript
const subscription = inbox.onNewEmail((email) => {
	console.log('Email:', email.subject);

	// Unsubscribe after first email
	if (email.subject.includes('Welcome')) {
		subscription.unsubscribe();
	}
});
```

### Unsubscribe from Monitor

```javascript
const monitor = client.monitorInboxes([inbox1, inbox2]);

monitor.on('email', (inbox, email) => {
	console.log('Email:', email.subject);
});

// Unsubscribe from all inboxes
monitor.unsubscribe();
```

## Real-World Patterns

### Wait for Specific Email

```javascript
async function waitForSpecificEmail(inbox, predicate, timeout = 30000) {
	return new Promise((resolve, reject) => {
		const timer = setTimeout(() => {
			subscription.unsubscribe();
			reject(new Error('Timeout waiting for email'));
		}, timeout);

		const subscription = inbox.onNewEmail((email) => {
			if (predicate(email)) {
				clearTimeout(timer);
				subscription.unsubscribe();
				resolve(email);
			}
		});
	});
}

// Usage
const email = await waitForSpecificEmail(inbox, (e) => e.subject.includes('Password Reset'), 10000);
```

### Collect Multiple Emails

```javascript
async function collectEmails(inbox, count, timeout = 30000) {
	const emails = [];

	return new Promise((resolve, reject) => {
		const timer = setTimeout(() => {
			subscription.unsubscribe();
			reject(new Error(`Timeout: only received ${emails.length}/${count}`));
		}, timeout);

		const subscription = inbox.onNewEmail((email) => {
			emails.push(email);
			console.log(`Received ${emails.length}/${count}`);

			if (emails.length >= count) {
				clearTimeout(timer);
				subscription.unsubscribe();
				resolve(emails);
			}
		});
	});
}

// Usage
const emails = await collectEmails(inbox, 3, 20000);
```

### Process Email Pipeline

```javascript
async function processEmailPipeline(inbox) {
	const subscription = inbox.onNewEmail(async (email) => {
		try {
			console.log('Processing:', email.subject);

			// Step 1: Validate
			const auth = email.authResults.validate();
			if (!auth.passed) {
				console.warn('Failed auth:', auth.failures);
				return;
			}

			// Step 2: Extract data
			const links = email.links;
			const attachments = email.attachments;

			// Step 3: Store/process
			await storeEmail(email);

			// Step 4: Notify
			await notifyProcessed(email.id);

			// Step 5: Cleanup
			await email.delete();

			console.log('✅ Processed:', email.subject);
		} catch (error) {
			console.error('❌ Error processing:', error);
		}
	});

	return subscription;
}

// Usage
const subscription = await processEmailPipeline(inbox);
// Later: subscription.unsubscribe();
```

## Testing with Real-Time Monitoring

### Integration Test

```javascript
test('real-time email processing', async () => {
	const inbox = await client.createInbox();
	const received = [];

	const subscription = inbox.onNewEmail((email) => {
		received.push(email);
	});

	// Send test emails
	await sendEmail(inbox.emailAddress, 'Test 1');
	await sendEmail(inbox.emailAddress, 'Test 2');

	// Wait for emails to arrive
	await new Promise((resolve) => setTimeout(resolve, 5000));

	expect(received.length).toBe(2);
	expect(received[0].subject).toBe('Test 1');
	expect(received[1].subject).toBe('Test 2');

	subscription.unsubscribe();
	await inbox.delete();
});
```

### Async Processing Test

```javascript
test('processes emails asynchronously', async () => {
	const inbox = await client.createInbox();
	const processed = [];

	const subscription = inbox.onNewEmail(async (email) => {
		await processEmail(email);
		processed.push(email.id);
	});

	await sendEmail(inbox.emailAddress, 'Test');

	// Wait for processing
	await waitUntil(() => processed.length > 0, 10000);

	expect(processed.length).toBe(1);

	subscription.unsubscribe();
	await inbox.delete();
});
```

## Error Handling

### Handle Subscription Errors

```javascript
const subscription = inbox.onNewEmail((email) => {
	try {
		processEmail(email);
	} catch (error) {
		console.error('Error processing email:', error);
		// Don't throw - keeps subscription active
	}
});
```

### Reconnection Handling

```javascript
// The SDK automatically reconnects on connection loss
// Configure reconnection behavior in client options

const client = new VaultSandboxClient({
	url,
	apiKey,
	strategy: 'sse',
	sseReconnectInterval: 5000, // Reconnect after 5s
	sseMaxReconnectAttempts: 10, // Try 10 times
});
```

### Graceful Shutdown

```javascript
async function gracefulShutdown(subscriptions) {
	console.log('Shutting down...');

	// Unsubscribe from all
	subscriptions.forEach((sub) => sub.unsubscribe());

	// Wait for pending operations
	await new Promise((resolve) => setTimeout(resolve, 1000));

	// Close client
	await client.close();

	console.log('Shutdown complete');
}

// Usage
const subs = [subscription1, subscription2];
await gracefulShutdown(subs);
```

## SSE vs Polling

### When to Use SSE

Use SSE (real-time) when:

- You need instant notification of new emails
- Processing emails as they arrive
- Building real-time dashboards
- Minimizing latency is critical

```javascript
const client = new VaultSandboxClient({
	url,
	apiKey,
	strategy: 'sse', // Force SSE
});
```

### When to Use Polling

Use polling when:

- SSE is blocked by firewall/proxy
- Running in environments that don't support persistent connections
- Batch processing is acceptable

```javascript
const client = new VaultSandboxClient({
	url,
	apiKey,
	strategy: 'polling',
	pollingInterval: 2000, // Poll every 2 seconds
});
```

### Auto Strategy (Recommended)

```javascript
const client = new VaultSandboxClient({
	url,
	apiKey,
	strategy: 'auto', // Tries SSE, falls back to polling
});
```

## Advanced Patterns

### Rate-Limited Processing

```javascript
const queue = [];
let processing = false;

const subscription = inbox.onNewEmail((email) => {
	queue.push(email);
	processQueue();
});

async function processQueue() {
	if (processing || queue.length === 0) return;

	processing = true;

	while (queue.length > 0) {
		const email = queue.shift();
		await processEmail(email);
		await delay(1000); // Rate limit: 1 per second
	}

	processing = false;
}

function delay(ms) {
	return new Promise((resolve) => setTimeout(resolve, ms));
}
```

### Priority Processing

```javascript
const subscription = inbox.onNewEmail(async (email) => {
	const priority = getPriority(email);

	if (priority === 'high') {
		await processImmediately(email);
	} else if (priority === 'medium') {
		await queueForProcessing(email);
	} else {
		await logAndDiscard(email);
	}
});

function getPriority(email) {
	if (email.subject.includes('URGENT')) return 'high';
	if (email.from === 'alerts@example.com') return 'high';
	if (email.attachments.length > 0) return 'medium';
	return 'low';
}
```

### Distributed Processing

```javascript
const monitor = client.monitorInboxes([inbox1, inbox2, inbox3]);

const workers = [createWorker('worker-1'), createWorker('worker-2'), createWorker('worker-3')];

let nextWorker = 0;

monitor.on('email', (inbox, email) => {
	const worker = workers[nextWorker];
	nextWorker = (nextWorker + 1) % workers.length;

	worker.process(email);
});
```

## Cleanup

### Proper Cleanup in Tests

```javascript
describe('Email Monitoring', () => {
	let client, inbox, subscription;

	beforeEach(async () => {
		client = new VaultSandboxClient({ url, apiKey });
		inbox = await client.createInbox();
	});

	afterEach(async () => {
		if (subscription) {
			subscription.unsubscribe();
		}
		if (inbox) {
			await inbox.delete();
		}
		await client.close();
	});

	test('monitors emails', async () => {
		subscription = inbox.onNewEmail((email) => {
			console.log('Email:', email.subject);
		});
		// Test code...
	});
});
```

## Next Steps

- **[Waiting for Emails](/client-node/guides/waiting-for-emails/)** - Alternative polling-based approach
- **[Managing Inboxes](/client-node/guides/managing-inboxes/)** - Inbox operations
- **[Delivery Strategies](/client-node/advanced/strategies/)** - SSE vs Polling deep dive
- **[Configuration](/client-node/configuration/)** - Configure SSE behavior
