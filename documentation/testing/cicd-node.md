---
title: CI/CD Integration
description: Learn how to integrate VaultSandbox email testing into your CI/CD pipelines
---

VaultSandbox is designed specifically for automated testing in CI/CD pipelines. This guide shows you how to integrate email testing into popular CI/CD platforms.

## Jest Setup

Configure Jest with proper setup and teardown for reliable email testing:

### Basic Configuration

```javascript
// jest.config.js
module.exports = {
	testTimeout: 30000, // Increase timeout for email delivery
	setupFilesAfterEnv: ['<rootDir>/tests/setup.js'],
	testEnvironment: 'node',
};
```

### Setup File

```javascript
// tests/setup.js
import { VaultSandboxClient } from '@vaultsandbox/client';

// Make client available globally if needed
global.vaultSandboxClient = null;

beforeAll(() => {
	// Verify environment variables are set
	if (!process.env.VAULTSANDBOX_URL) {
		throw new Error('VAULTSANDBOX_URL environment variable is required');
	}
	if (!process.env.VAULTSANDBOX_API_KEY) {
		throw new Error('VAULTSANDBOX_API_KEY environment variable is required');
	}
});

afterAll(async () => {
	// Clean up any remaining inboxes
	if (global.vaultSandboxClient) {
		try {
			const deleted = await global.vaultSandboxClient.deleteAllInboxes();
			console.log(`Cleaned up ${deleted} inboxes`);
		} catch (error) {
			console.error('Failed to clean up inboxes:', error);
		}
	}
});
```

### Test Structure

```javascript
// tests/email.test.js
import { VaultSandboxClient } from '@vaultsandbox/client';

describe('Email Tests', () => {
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
			try {
				await inbox.delete();
			} catch (error) {
				console.error('Failed to delete inbox:', error);
			}
		}
	});

	test('should receive welcome email', async () => {
		await sendWelcomeEmail(inbox.emailAddress);

		const email = await inbox.waitForEmail({
			timeout: 10000,
			subject: /Welcome/,
		});

		expect(email.subject).toContain('Welcome');
		expect(email.from).toBe('noreply@example.com');
	});
});
```

## GitHub Actions

### Basic Workflow

```yaml
# .github/workflows/test.yml
name: Email Tests

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  email-tests:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Setup Node.js
        uses: actions/setup-node@v3
        with:
          node-version: '20'
          cache: 'npm'

      - name: Install dependencies
        run: npm ci

      - name: Run email tests
        env:
          VAULTSANDBOX_URL: ${{ secrets.VAULTSANDBOX_URL }}
          VAULTSANDBOX_API_KEY: ${{ secrets.VAULTSANDBOX_API_KEY }}
        run: npm test -- --testPathPattern=email
```

### With Docker Compose

If you're running VaultSandbox Gateway locally in CI:

```yaml
# .github/workflows/test-with-gateway.yml
name: Email Tests (Self-Hosted)

on: [push, pull_request]

jobs:
  email-tests:
    runs-on: ubuntu-latest

    services:
      vaultsandbox:
        image: vaultsandbox/gateway:latest
        ports:
          - 3000:3000
          - 2525:25
        env:
          API_KEYS: test-api-key-12345
          SMTP_HOST: 0.0.0.0
          SMTP_PORT: 25

    steps:
      - uses: actions/checkout@v3

      - name: Setup Node.js
        uses: actions/setup-node@v3
        with:
          node-version: '20'

      - name: Install dependencies
        run: npm ci

      - name: Wait for VaultSandbox
        run: |
          timeout 30 sh -c 'until nc -z localhost 3000; do sleep 1; done'

      - name: Run email tests
        env:
          VAULTSANDBOX_URL: http://localhost:3000
          VAULTSANDBOX_API_KEY: test-api-key-12345
        run: npm test
```

### Parallel Testing

```yaml
# .github/workflows/test-parallel.yml
name: Parallel Email Tests

on: [push, pull_request]

jobs:
  email-tests:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        test-group: [auth, transactional, notifications]

    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '20'

      - run: npm ci

      - name: Run test group
        env:
          VAULTSANDBOX_URL: ${{ secrets.VAULTSANDBOX_URL }}
          VAULTSANDBOX_API_KEY: ${{ secrets.VAULTSANDBOX_API_KEY }}
        run: npm test -- --testPathPattern=${{ matrix.test-group }}
```

## GitLab CI

### Basic Pipeline

```yaml
# .gitlab-ci.yml
stages:
  - test

email-tests:
  stage: test
  image: node:20
  cache:
    paths:
      - node_modules/
  before_script:
    - npm ci
  script:
    - npm test -- --testPathPattern=email
  variables:
    VAULTSANDBOX_URL: $VAULTSANDBOX_URL
    VAULTSANDBOX_API_KEY: $VAULTSANDBOX_API_KEY
```

### With Docker Compose

```yaml
# .gitlab-ci.yml
stages:
  - test

email-tests:
  stage: test
  image: node:20
  services:
    - name: vaultsandbox/gateway:latest
      alias: vaultsandbox
  variables:
    VAULTSANDBOX_URL: http://vaultsandbox:3000
    VAULTSANDBOX_API_KEY: test-api-key-12345
    # Service configuration
    API_KEYS: test-api-key-12345
    SMTP_HOST: 0.0.0.0
  before_script:
    - npm ci
    - apt-get update && apt-get install -y netcat-openbsd
    - timeout 30 sh -c 'until nc -z vaultsandbox 3000; do sleep 1; done'
  script:
    - npm test
```

## CircleCI

```yaml
# .circleci/config.yml
version: 2.1

jobs:
  email-tests:
    docker:
      - image: cimg/node:20.0
    steps:
      - checkout
      - restore_cache:
          keys:
            - v1-dependencies-{{ checksum "package-lock.json" }}
      - run:
          name: Install dependencies
          command: npm ci
      - save_cache:
          paths:
            - node_modules
          key: v1-dependencies-{{ checksum "package-lock.json" }}
      - run:
          name: Run email tests
          command: npm test
          environment:
            VAULTSANDBOX_URL: ${VAULTSANDBOX_URL}
            VAULTSANDBOX_API_KEY: ${VAULTSANDBOX_API_KEY}

workflows:
  test:
    jobs:
      - email-tests
```

## Jenkins

```groovy
// Jenkinsfile
pipeline {
    agent {
        docker {
            image 'node:20'
        }
    }

    environment {
        VAULTSANDBOX_URL = credentials('vaultsandbox-url')
        VAULTSANDBOX_API_KEY = credentials('vaultsandbox-api-key')
    }

    stages {
        stage('Install') {
            steps {
                sh 'npm ci'
            }
        }

        stage('Test') {
            steps {
                sh 'npm test'
            }
        }
    }

    post {
        always {
            junit 'test-results/**/*.xml'
        }
    }
}
```

## Environment Variables

### Required Variables

Set these environment variables in your CI platform:

| Variable               | Description            | Example                         |
| ---------------------- | ---------------------- | ------------------------------- |
| `VAULTSANDBOX_URL`     | Gateway URL            | `https://smtp.vaultsandbox.com` |
| `VAULTSANDBOX_API_KEY` | API authentication key | `vs_1234567890abcdef`           |

### Optional Variables

| Variable                        | Description           | Default |
| ------------------------------- | --------------------- | ------- |
| `VAULTSANDBOX_STRATEGY`         | Delivery strategy     | `auto`  |
| `VAULTSANDBOX_TIMEOUT`          | Default timeout (ms)  | `30000` |
| `VAULTSANDBOX_POLLING_INTERVAL` | Polling interval (ms) | `2000`  |

### Configuration Helper

```javascript
// config/vaultsandbox.js
export function getVaultSandboxConfig() {
	return {
		url: process.env.VAULTSANDBOX_URL,
		apiKey: process.env.VAULTSANDBOX_API_KEY,
		strategy: process.env.VAULTSANDBOX_STRATEGY || 'auto',
		timeout: parseInt(process.env.VAULTSANDBOX_TIMEOUT || '30000', 10),
		pollingInterval: parseInt(process.env.VAULTSANDBOX_POLLING_INTERVAL || '2000', 10),
	};
}

// Usage in tests
import { getVaultSandboxConfig } from '../config/vaultsandbox';

const client = new VaultSandboxClient(getVaultSandboxConfig());
```

## Best Practices

### Always Clean Up

Ensure inboxes are deleted even when tests fail:

```javascript
afterEach(async () => {
	if (inbox) {
		try {
			await inbox.delete();
		} catch (error) {
			// Log but don't fail the test
			console.error('Failed to delete inbox:', error);
		}
	}
});
```

### Use Global Cleanup

Add a final cleanup step to delete any orphaned inboxes:

```javascript
afterAll(async () => {
	const client = new VaultSandboxClient({
		url: process.env.VAULTSANDBOX_URL,
		apiKey: process.env.VAULTSANDBOX_API_KEY,
	});

	try {
		const deleted = await client.deleteAllInboxes();
		if (deleted > 0) {
			console.log(`Cleaned up ${deleted} orphaned inboxes`);
		}
	} catch (error) {
		console.error('Failed to clean up orphaned inboxes:', error);
	}
});
```

### Set Appropriate Timeouts

CI environments can be slower than local development:

```javascript
const CI_TIMEOUT = process.env.CI ? 30000 : 10000;

test('should receive email', async () => {
	const email = await inbox.waitForEmail({
		timeout: CI_TIMEOUT,
		subject: /Welcome/,
	});

	expect(email).toBeDefined();
});
```

### Use Test Isolation

Each test should create its own inbox:

```javascript
// Good: Isolated tests
describe('Email Flow', () => {
	let inbox;

	beforeEach(async () => {
		inbox = await client.createInbox();
	});

	test('test 1', async () => {
		// Use inbox
	});

	test('test 2', async () => {
		// Use different inbox
	});
});

// Avoid: Shared inbox across tests
describe('Email Flow', () => {
	let inbox;

	beforeAll(async () => {
		inbox = await client.createInbox(); // BAD: Shared state
	});
});
```

### Handle Flaky Tests

Add retries for occasionally flaky email tests:

```javascript
// jest.config.js
module.exports = {
	testTimeout: 30000,
	maxRetries: process.env.CI ? 2 : 0, // Retry twice in CI
};
```

Or use jest-retry:

```javascript
test(
	'should receive email',
	async () => {
		const email = await inbox.waitForEmail({
			timeout: 10000,
			subject: /Welcome/,
		});

		expect(email).toBeDefined();
	},
	{ retries: 2 }
);
```

### Log Helpful Debug Info

Add logging to help debug CI failures:

```javascript
test('should receive welcome email', async () => {
	console.log(`Created inbox: ${inbox.emailAddress}`);

	await sendWelcomeEmail(inbox.emailAddress);
	console.log('Triggered welcome email');

	const email = await inbox.waitForEmail({
		timeout: 10000,
		subject: /Welcome/,
	});

	console.log(`Received email: ${email.subject}`);
	expect(email.from).toBe('noreply@example.com');
});
```

## Troubleshooting

### Tests Timeout in CI

**Symptoms:** Tests pass locally but timeout in CI

**Solutions:**

- Increase timeout values for CI environment
- Check network connectivity to VaultSandbox Gateway
- Verify API key is correctly set in CI environment
- Use longer polling intervals to reduce API load

```javascript
const config = {
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	pollingInterval: process.env.CI ? 3000 : 1000,
};
```

### Rate Limiting

**Symptoms:** Tests fail with 429 status codes

**Solutions:**

- Reduce test parallelization
- Increase retry delay
- Use fewer inboxes per test
- Configure rate limit handling

```javascript
const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	maxRetries: 5,
	retryDelay: 2000,
	retryOn: [408, 429, 500, 502, 503, 504],
});
```

### Orphaned Inboxes

**Symptoms:** Running out of inbox quota

**Solutions:**

- Always use `afterEach` to delete inboxes
- Add global cleanup in `afterAll`
- Manually clean up using `deleteAllInboxes()`

```bash
# Manual cleanup script
npx tsx scripts/cleanup-inboxes.ts
```

```typescript
// scripts/cleanup-inboxes.ts
import { VaultSandboxClient } from '@vaultsandbox/client';

const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL!,
	apiKey: process.env.VAULTSANDBOX_API_KEY!,
});

const deleted = await client.deleteAllInboxes();
console.log(`Deleted ${deleted} inboxes`);
```

### Connection Issues

**Symptoms:** Cannot connect to VaultSandbox Gateway

**Solutions:**

- Verify URL is correct and accessible from CI
- Check firewall rules
- Ensure service is running (for self-hosted)
- Test with curl/wget in CI

```yaml
- name: Test connectivity
  run: curl -f $VAULTSANDBOX_URL/health || exit 1
```

## Performance Optimization

### Parallel Test Execution

Run tests in parallel for faster CI builds:

```bash
# Jest with workers
npm test -- --maxWorkers=4

# Split tests across CI jobs
npm test -- --shard=1/4
npm test -- --shard=2/4
npm test -- --shard=3/4
npm test -- --shard=4/4
```

### Reduce API Calls

Minimize API calls by batching operations:

```javascript
// Good: Single API call
const emails = await inbox.listEmails();
const welcome = emails.find((e) => e.subject.includes('Welcome'));

// Avoid: Multiple API calls
const email1 = await inbox.getEmail(id1);
const email2 = await inbox.getEmail(id2);
```

### Use SSE for Real-time Tests

Enable SSE strategy for faster delivery in supported environments:

```javascript
const client = new VaultSandboxClient({
	url: process.env.VAULTSANDBOX_URL,
	apiKey: process.env.VAULTSANDBOX_API_KEY,
	strategy: 'sse', // Faster than polling
});
```

## Next Steps

- [Password Reset Testing](/client-node/testing/password-reset) - Specific test patterns
- [Multi-Email Scenarios](/client-node/testing/multi-email) - Testing multiple emails
- [Error Handling](/client-node/api/errors) - Handle failures gracefully
