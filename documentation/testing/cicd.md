---
title: CI/CD Integration
description: Learn how to integrate VaultSandbox email testing into your CI/CD pipelines
---

VaultSandbox is designed specifically for automated testing in CI/CD pipelines. This guide shows you how to integrate email testing into popular CI/CD platforms.

## Go Testing Setup

Configure your Go tests with proper setup and teardown for reliable email testing.

### Test Structure

```go
// email_test.go
package myapp_test

import (
	"context"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go"
)

var client *vaultsandbox.Client

func TestMain(m *testing.M) {
	// Verify environment variables are set
	if os.Getenv("VAULTSANDBOX_URL") == "" {
		panic("VAULTSANDBOX_URL environment variable is required")
	}
	if os.Getenv("VAULTSANDBOX_API_KEY") == "" {
		panic("VAULTSANDBOX_API_KEY environment variable is required")
	}

	// Run tests
	code := m.Run()

	// Global cleanup
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		deleted, err := client.DeleteAllInboxes(ctx)
		if err != nil {
			println("Failed to clean up inboxes:", err.Error())
		} else if deleted > 0 {
			println("Cleaned up", deleted, "inboxes")
		}
		client.Close()
	}

	os.Exit(code)
}

func setupClient(t *testing.T) *vaultsandbox.Client {
	t.Helper()

	c, err := vaultsandbox.New(
		os.Getenv("VAULTSANDBOX_API_KEY"),
		vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
		vaultsandbox.WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Store for global cleanup
	client = c
	return c
}

func TestWelcomeEmail(t *testing.T) {
	client := setupClient(t)

	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("Failed to create inbox: %v", err)
	}
	defer inbox.Delete(ctx)

	// Trigger your application to send the email
	sendWelcomeEmail(inbox.EmailAddress())

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Welcome`)),
	)
	if err != nil {
		t.Fatalf("Failed to receive email: %v", err)
	}

	if email.From != "noreply@example.com" {
		t.Errorf("Expected from noreply@example.com, got %s", email.From)
	}
}
```

### Using Table-Driven Tests

```go
func TestEmailNotifications(t *testing.T) {
	client := setupClient(t)
	ctx := context.Background()

	tests := []struct {
		name            string
		triggerFunc     func(email string)
		subjectPattern  *regexp.Regexp
		expectedFrom    string
	}{
		{
			name:           "welcome email",
			triggerFunc:    sendWelcomeEmail,
			subjectPattern: regexp.MustCompile(`Welcome`),
			expectedFrom:   "noreply@example.com",
		},
		{
			name:           "password reset",
			triggerFunc:    sendPasswordReset,
			subjectPattern: regexp.MustCompile(`Reset`),
			expectedFrom:   "security@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inbox, err := client.CreateInbox(ctx)
			if err != nil {
				t.Fatalf("Failed to create inbox: %v", err)
			}
			defer inbox.Delete(ctx)

			tt.triggerFunc(inbox.EmailAddress())

			email, err := inbox.WaitForEmail(ctx,
				vaultsandbox.WithWaitTimeout(10*time.Second),
				vaultsandbox.WithSubjectRegex(tt.subjectPattern),
			)
			if err != nil {
				t.Fatalf("Failed to receive email: %v", err)
			}

			if email.From != tt.expectedFrom {
				t.Errorf("Expected from %s, got %s", tt.expectedFrom, email.From)
			}
		})
	}
}
```

### Test Helper Functions

```go
// testhelpers/email.go
package testhelpers

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/vaultsandbox/client-go"
)

// MustCreateClient creates a VaultSandbox client or fails the test.
func MustCreateClient(t *testing.T) *vaultsandbox.Client {
	t.Helper()

	client, err := vaultsandbox.New(
		os.Getenv("VAULTSANDBOX_API_KEY"),
		vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
		vaultsandbox.WithTimeout(getTimeout()),
	)
	if err != nil {
		t.Fatalf("Failed to create VaultSandbox client: %v", err)
	}

	t.Cleanup(func() {
		client.Close()
	})

	return client
}

// MustCreateInbox creates an inbox or fails the test, with automatic cleanup.
func MustCreateInbox(t *testing.T, client *vaultsandbox.Client) *vaultsandbox.Inbox {
	t.Helper()

	ctx := context.Background()
	inbox, err := client.CreateInbox(ctx, vaultsandbox.WithTTL(5*time.Minute))
	if err != nil {
		t.Fatalf("Failed to create inbox: %v", err)
	}

	t.Cleanup(func() {
		if err := inbox.Delete(ctx); err != nil {
			t.Logf("Failed to delete inbox: %v", err)
		}
	})

	return inbox
}

func getTimeout() time.Duration {
	if os.Getenv("CI") != "" {
		return 30 * time.Second
	}
	return 10 * time.Second
}
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
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Run email tests
        env:
          VAULTSANDBOX_URL: ${{ secrets.VAULTSANDBOX_URL }}
          VAULTSANDBOX_API_KEY: ${{ secrets.VAULTSANDBOX_API_KEY }}
        run: go test -v -run TestEmail ./...
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
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Wait for VaultSandbox
        run: |
          timeout 30 sh -c 'until nc -z localhost 3000; do sleep 1; done'

      - name: Run email tests
        env:
          VAULTSANDBOX_URL: http://localhost:3000
          VAULTSANDBOX_API_KEY: test-api-key-12345
        run: go test -v ./...
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
        test-package: [auth, transactional, notifications]

    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Run test package
        env:
          VAULTSANDBOX_URL: ${{ secrets.VAULTSANDBOX_URL }}
          VAULTSANDBOX_API_KEY: ${{ secrets.VAULTSANDBOX_API_KEY }}
        run: go test -v ./tests/${{ matrix.test-package }}/...
```

## GitLab CI

### Basic Pipeline

```yaml
# .gitlab-ci.yml
stages:
  - test

email-tests:
  stage: test
  image: golang:1.24
  variables:
    VAULTSANDBOX_URL: $VAULTSANDBOX_URL
    VAULTSANDBOX_API_KEY: $VAULTSANDBOX_API_KEY
  script:
    - go test -v -run TestEmail ./...
```

### With Docker Compose

```yaml
# .gitlab-ci.yml
stages:
  - test

email-tests:
  stage: test
  image: golang:1.24
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
    - apt-get update && apt-get install -y netcat-openbsd
    - timeout 30 sh -c 'until nc -z vaultsandbox 3000; do sleep 1; done'
  script:
    - go test -v ./...
```

## CircleCI

```yaml
# .circleci/config.yml
version: 2.1

jobs:
  email-tests:
    docker:
      - image: cimg/go:1.24
    steps:
      - checkout
      - restore_cache:
          keys:
            - go-mod-v1-{{ checksum "go.sum" }}
      - run:
          name: Download dependencies
          command: go mod download
      - save_cache:
          paths:
            - /home/circleci/go/pkg/mod
          key: go-mod-v1-{{ checksum "go.sum" }}
      - run:
          name: Run email tests
          command: go test -v ./...
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
            image 'golang:1.24'
        }
    }

    environment {
        VAULTSANDBOX_URL = credentials('vaultsandbox-url')
        VAULTSANDBOX_API_KEY = credentials('vaultsandbox-api-key')
    }

    stages {
        stage('Download') {
            steps {
                sh 'go mod download'
            }
        }

        stage('Test') {
            steps {
                sh 'go test -v -json ./... > test-results.json'
            }
        }
    }

    post {
        always {
            sh 'go-junit-report < test-results.json > test-results.xml || true'
            junit 'test-results.xml'
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
| `VAULTSANDBOX_TIMEOUT`          | Default timeout       | `60s`   |
| `VAULTSANDBOX_POLLING_INTERVAL` | Polling interval      | `2s`    |

### Configuration Helper

```go
// config/vaultsandbox.go
package config

import (
	"os"
	"time"

	"github.com/vaultsandbox/client-go"
)

// GetVaultSandboxOptions returns client options from environment variables.
func GetVaultSandboxOptions() []vaultsandbox.Option {
	opts := []vaultsandbox.Option{
		vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	}

	if timeout := os.Getenv("VAULTSANDBOX_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			opts = append(opts, vaultsandbox.WithTimeout(d))
		}
	}

	switch os.Getenv("VAULTSANDBOX_STRATEGY") {
	case "sse":
		opts = append(opts, vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE))
	case "polling":
		opts = append(opts, vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling))
	default:
		opts = append(opts, vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto))
	}

	return opts
}

// NewClient creates a VaultSandbox client from environment variables.
func NewClient() (*vaultsandbox.Client, error) {
	return vaultsandbox.New(
		os.Getenv("VAULTSANDBOX_API_KEY"),
		GetVaultSandboxOptions()...,
	)
}
```

## Best Practices

### Always Clean Up

Ensure inboxes are deleted even when tests fail using `defer` or `t.Cleanup`:

```go
func TestEmail(t *testing.T) {
	client := setupClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("Failed to create inbox: %v", err)
	}

	// Using defer ensures cleanup even on panic
	defer func() {
		if err := inbox.Delete(ctx); err != nil {
			t.Logf("Failed to delete inbox: %v", err)
		}
	}()

	// Or use t.Cleanup for automatic cleanup
	t.Cleanup(func() {
		inbox.Delete(context.Background())
	})

	// ... test code
}
```

### Use Global Cleanup

Add a final cleanup step to delete any orphaned inboxes:

```go
func TestMain(m *testing.M) {
	code := m.Run()

	// Global cleanup
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		deleted, err := client.DeleteAllInboxes(ctx)
		cancel()

		if err != nil {
			println("Failed to clean up orphaned inboxes:", err.Error())
		} else if deleted > 0 {
			println("Cleaned up", deleted, "orphaned inboxes")
		}
		client.Close()
	}

	os.Exit(code)
}
```

### Set Appropriate Timeouts

CI environments can be slower than local development:

```go
func getWaitTimeout() time.Duration {
	if os.Getenv("CI") != "" {
		return 30 * time.Second
	}
	return 10 * time.Second
}

func TestWelcomeEmail(t *testing.T) {
	// ...
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(getWaitTimeout()),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Welcome`)),
	)
	// ...
}
```

### Use Test Isolation

Each test should create its own inbox:

```go
// Good: Isolated tests
func TestEmailFlow(t *testing.T) {
	client := setupClient(t)

	t.Run("test 1", func(t *testing.T) {
		inbox, _ := client.CreateInbox(context.Background())
		defer inbox.Delete(context.Background())
		// Use inbox
	})

	t.Run("test 2", func(t *testing.T) {
		inbox, _ := client.CreateInbox(context.Background())
		defer inbox.Delete(context.Background())
		// Use different inbox
	})
}

// Avoid: Shared inbox across tests
var sharedInbox *vaultsandbox.Inbox // BAD: Shared state
```

### Use Context for Timeouts

Leverage Go's context for deadline propagation:

```go
func TestWithTimeout(t *testing.T) {
	client := setupClient(t)

	// Create a context with overall test timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("Failed to create inbox: %v", err)
	}
	defer inbox.Delete(context.Background()) // Use fresh context for cleanup

	// Wait inherits the parent context deadline
	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Welcome`)),
	)
	if err != nil {
		t.Fatalf("Failed to receive email: %v", err)
	}

	t.Logf("Received: %s", email.Subject)
}
```

### Log Helpful Debug Info

Add logging to help debug CI failures:

```go
func TestWelcomeEmail(t *testing.T) {
	client := setupClient(t)
	ctx := context.Background()

	inbox, err := client.CreateInbox(ctx)
	if err != nil {
		t.Fatalf("Failed to create inbox: %v", err)
	}
	defer inbox.Delete(ctx)

	t.Logf("Created inbox: %s", inbox.EmailAddress())

	sendWelcomeEmail(inbox.EmailAddress())
	t.Log("Triggered welcome email")

	email, err := inbox.WaitForEmail(ctx,
		vaultsandbox.WithWaitTimeout(10*time.Second),
		vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Welcome`)),
	)
	if err != nil {
		t.Fatalf("Failed to receive email: %v", err)
	}

	t.Logf("Received email: %s", email.Subject)

	if email.From != "noreply@example.com" {
		t.Errorf("Expected from noreply@example.com, got %s", email.From)
	}
}
```

## Troubleshooting

### Tests Timeout in CI

**Symptoms:** Tests pass locally but timeout in CI

**Solutions:**

- Increase timeout values for CI environment
- Check network connectivity to VaultSandbox Gateway
- Verify API key is correctly set in CI environment
- Use longer polling intervals to reduce API load

```go
func newClientWithCIConfig() (*vaultsandbox.Client, error) {
	timeout := 30 * time.Second
	if os.Getenv("CI") != "" {
		timeout = 60 * time.Second
	}

	return vaultsandbox.New(
		os.Getenv("VAULTSANDBOX_API_KEY"),
		vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
		vaultsandbox.WithTimeout(timeout),
	)
}
```

### Rate Limiting

**Symptoms:** Tests fail with rate limit errors

**Solutions:**

- Reduce test parallelization with `-parallel 1`
- Configure retry behavior
- Use fewer inboxes per test

```go
client, err := vaultsandbox.New(
	os.Getenv("VAULTSANDBOX_API_KEY"),
	vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	vaultsandbox.WithRetries(5),
	vaultsandbox.WithRetryOn([]int{408, 429, 500, 502, 503, 504}),
)
```

### Orphaned Inboxes

**Symptoms:** Running out of inbox quota

**Solutions:**

- Always use `defer` or `t.Cleanup` to delete inboxes
- Add global cleanup in `TestMain`
- Manually clean up using `DeleteAllInboxes`

```bash
# Manual cleanup script
go run scripts/cleanup-inboxes.go
```

```go
// scripts/cleanup-inboxes.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/vaultsandbox/client-go"
)

func main() {
	client, err := vaultsandbox.New(
		os.Getenv("VAULTSANDBOX_API_KEY"),
		vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	deleted, err := client.DeleteAllInboxes(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Deleted %d inboxes\n", deleted)
}
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
# Run tests with parallelization
go test -v -parallel 4 ./...

# Run specific packages in parallel CI jobs
go test -v ./tests/auth/...
go test -v ./tests/transactional/...
go test -v ./tests/notifications/...
```

### Reduce API Calls

Minimize API calls by batching operations:

```go
// Good: Single API call
emails, err := inbox.GetEmails(ctx)
if err != nil {
	t.Fatal(err)
}
var welcome *vaultsandbox.Email
for _, e := range emails {
	if strings.Contains(e.Subject, "Welcome") {
		welcome = e
		break
	}
}

// Avoid: Multiple API calls
email1, _ := inbox.GetEmail(ctx, id1)
email2, _ := inbox.GetEmail(ctx, id2)
```

### Use SSE for Real-time Tests

Enable SSE strategy for faster delivery in supported environments:

```go
client, err := vaultsandbox.New(
	os.Getenv("VAULTSANDBOX_API_KEY"),
	vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE), // Faster than polling
)
```

## Next Steps

- [Password Reset Testing](/client-go/testing/password-reset) - Specific test patterns
- [Multi-Email Scenarios](/client-go/testing/multi-email) - Testing multiple emails
- [Error Handling](/client-go/api/errors) - Handle failures gracefully
