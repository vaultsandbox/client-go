---
title: Client Configuration
description: Configure the VaultSandbox client for your environment
---

This page covers all configuration options for the VaultSandbox Go client.

## Basic Configuration

### Creating a Client

```go
import "github.com/vaultsandbox/client-go"

client, err := vaultsandbox.New("your-api-key",
	vaultsandbox.WithBaseURL("https://mail.example.com"),
)
if err != nil {
	log.Fatal(err)
}
defer client.Close()
```

## Configuration Options

### Required Parameters

#### apiKey

**Type**: `string`

**Description**: API key for authentication. Passed as the first argument to `New()`.

**Example**:

```go
client, err := vaultsandbox.New("vs_1234567890abcdef...")
```

**Best practices**:

- Store in environment variables
- Never commit to version control
- Rotate periodically

### Client Options

Options are passed as variadic arguments to `New()`.

#### WithBaseURL

**Signature**: `WithBaseURL(url string) Option`

**Default**: `https://api.vaultsandbox.com`

**Description**: Base URL of your VaultSandbox Gateway

**Examples**:

```go
vaultsandbox.WithBaseURL("https://mail.example.com")
vaultsandbox.WithBaseURL("http://localhost:3000") // Local development
```

**Requirements**:

- Must include protocol (`https://` or `http://`)
- Should not include trailing slash
- Must be accessible from your application

#### WithDeliveryStrategy

**Signature**: `WithDeliveryStrategy(strategy DeliveryStrategy) Option`

**Default**: `StrategyAuto`

**Description**: Email delivery strategy

**Options**:

- `StrategyAuto` - Automatically choose best strategy (tries SSE first, falls back to polling)
- `StrategySSE` - Server-Sent Events for real-time delivery
- `StrategyPolling` - Poll for new emails at intervals

**Examples**:

```go
vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto)    // Recommended
vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE)     // Force SSE
vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling) // Force polling
```

**When to use each**:

- `StrategyAuto`: Most use cases (recommended)
- `StrategySSE`: When you need real-time, low-latency delivery
- `StrategyPolling`: When SSE is blocked by firewall/proxy

#### WithTimeout

**Signature**: `WithTimeout(timeout time.Duration) Option`

**Default**: `60 * time.Second`

**Description**: Default timeout for operations

**Examples**:

```go
vaultsandbox.WithTimeout(30 * time.Second)
vaultsandbox.WithTimeout(2 * time.Minute)
```

#### WithRetries

**Signature**: `WithRetries(count int) Option`

**Default**: `3`

**Description**: Maximum retry attempts for failed HTTP requests

**Examples**:

```go
vaultsandbox.WithRetries(3) // Default
vaultsandbox.WithRetries(5) // More resilient
vaultsandbox.WithRetries(0) // No retries
```

#### WithRetryOn

**Signature**: `WithRetryOn(statusCodes []int) Option`

**Default**: `[]int{408, 429, 500, 502, 503, 504}`

**Description**: HTTP status codes that trigger a retry

**Example**:

```go
vaultsandbox.WithRetryOn([]int{408, 429, 500, 502, 503, 504}) // Default
vaultsandbox.WithRetryOn([]int{500, 502, 503})                // Only server errors
vaultsandbox.WithRetryOn([]int{})                             // Never retry
```

#### WithHTTPClient

**Signature**: `WithHTTPClient(client *http.Client) Option`

**Default**: `http.DefaultClient` with timeout

**Description**: Custom HTTP client for advanced networking needs

**Example**:

```go
httpClient := &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true,
	},
}

client, err := vaultsandbox.New(apiKey,
	vaultsandbox.WithHTTPClient(httpClient),
)
```

## Inbox Options

Options passed to `CreateInbox()`.

#### WithTTL

**Signature**: `WithTTL(ttl time.Duration) InboxOption`

**Default**: `1 * time.Hour`

**Description**: Time-to-live for the inbox

**Constraints**:

- Minimum: 1 minute (`MinTTL`)
- Maximum: 7 days (`MaxTTL`) or server-configured limit

**Examples**:

```go
inbox, err := client.CreateInbox(ctx,
	vaultsandbox.WithTTL(30 * time.Minute),
)

inbox, err := client.CreateInbox(ctx,
	vaultsandbox.WithTTL(24 * time.Hour),
)
```

#### WithEmailAddress

**Signature**: `WithEmailAddress(email string) InboxOption`

**Default**: Auto-generated

**Description**: Request a specific email address

**Example**:

```go
inbox, err := client.CreateInbox(ctx,
	vaultsandbox.WithEmailAddress("test-user@mail.example.com"),
)
```

## Wait Options

Options passed to `WaitForEmail()` and `WaitForEmailCount()`.

#### WithWaitTimeout

**Signature**: `WithWaitTimeout(timeout time.Duration) WaitOption`

**Default**: `60 * time.Second`

**Description**: Maximum time to wait for email

**Example**:

```go
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithWaitTimeout(30 * time.Second),
)
```

#### WithPollInterval

**Signature**: `WithPollInterval(interval time.Duration) WaitOption`

**Default**: `2 * time.Second`

**Description**: Polling interval when using polling strategy

**Examples**:

```go
vaultsandbox.WithPollInterval(2 * time.Second)   // Default
vaultsandbox.WithPollInterval(5 * time.Second)   // Less aggressive
vaultsandbox.WithPollInterval(500 * time.Millisecond) // More responsive
```

#### WithSubject

**Signature**: `WithSubject(subject string) WaitOption`

**Description**: Filter emails by exact subject match

**Example**:

```go
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithSubject("Password Reset"),
)
```

#### WithSubjectRegex

**Signature**: `WithSubjectRegex(pattern *regexp.Regexp) WaitOption`

**Description**: Filter emails by subject regex

**Example**:

```go
pattern := regexp.MustCompile(`(?i)password.*reset`)
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithSubjectRegex(pattern),
)
```

#### WithFrom

**Signature**: `WithFrom(from string) WaitOption`

**Description**: Filter emails by exact sender match

**Example**:

```go
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithFrom("noreply@example.com"),
)
```

#### WithFromRegex

**Signature**: `WithFromRegex(pattern *regexp.Regexp) WaitOption`

**Description**: Filter emails by sender regex

**Example**:

```go
pattern := regexp.MustCompile(`@example\.com$`)
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithFromRegex(pattern),
)
```

#### WithPredicate

**Signature**: `WithPredicate(fn func(*Email) bool) WaitOption`

**Description**: Filter emails by custom predicate function

**Example**:

```go
email, err := inbox.WaitForEmail(ctx,
	vaultsandbox.WithPredicate(func(e *vaultsandbox.Email) bool {
		return len(e.Attachments) > 0
	}),
)
```

## Configuration Examples

### Production Configuration

```go
client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
	vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto),
	vaultsandbox.WithRetries(5),
	vaultsandbox.WithTimeout(60 * time.Second),
)
```

### CI/CD Configuration

```go
client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
	vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto),
	vaultsandbox.WithRetries(3),
	vaultsandbox.WithTimeout(30 * time.Second),
)
```

### Development Configuration

```go
client, err := vaultsandbox.New("dev-api-key",
	vaultsandbox.WithBaseURL("http://localhost:3000"),
	vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
	vaultsandbox.WithRetries(1),
)
```

### High-Reliability Configuration

```go
client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
	vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	vaultsandbox.WithRetries(10),
	vaultsandbox.WithRetryOn([]int{408, 429, 500, 502, 503, 504}),
	vaultsandbox.WithTimeout(2 * time.Minute),
)
```

## Environment Variables

Store configuration in environment variables:

### `.env` File

```bash
VAULTSANDBOX_URL=https://mail.example.com
VAULTSANDBOX_API_KEY=vs_1234567890abcdef...
```

### Usage

```go
import (
	"os"

	"github.com/joho/godotenv"
	"github.com/vaultsandbox/client-go"
)

func main() {
	// Load .env file
	godotenv.Load()

	client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
		vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
}
```

## Client Methods

### Close()

Close the client and clean up resources:

```go
err := client.Close()
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

```go
client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(url))
if err != nil {
	log.Fatal(err)
}
defer client.Close()

// Use client
inbox, err := client.CreateInbox(ctx)
// ...
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

**Do**:

```go
// Use environment variables
client, err := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"),
	vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
)
```

**Don't**:

```go
// Hard-code credentials
client, err := vaultsandbox.New("vs_1234567890...", // Never do this
	vaultsandbox.WithBaseURL("https://mail.example.com"),
)
```

### Resource Management

**Do**:

```go
client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(url))
if err != nil {
	log.Fatal(err)
}
defer client.Close() // Always clean up

runTests()
```

**Don't**:

```go
client, err := vaultsandbox.New(apiKey, vaultsandbox.WithBaseURL(url))
runTests()
// Forgot to close, resources leak
```

### Error Handling

**Error Types**:

- `APIError` - HTTP errors from the API with `StatusCode`, `Message`, and `RequestID` fields
- `NetworkError` - Network-level failures with `Err` (underlying error), `URL`, and `Attempt` fields
- `TimeoutError` - Operation deadline exceeded with `Operation` and `Timeout` fields
- `DecryptionError` - Email decryption failures with `Stage`, `Message`, and `Err` fields

**Sentinel Errors** (use with `errors.Is()`):

- `ErrUnauthorized` - Invalid or expired API key
- `ErrInboxNotFound` - Inbox does not exist
- `ErrEmailNotFound` - Email does not exist
- `ErrInboxExpired` - Inbox TTL has elapsed
- `ErrRateLimited` - API rate limit exceeded
- `ErrDecryptionFailed` - Email decryption failed
- `ErrClientClosed` - Client has been closed

**Do**:

```go
inbox, err := client.CreateInbox(ctx)
if err != nil {
	var apiErr *vaultsandbox.APIError
	var netErr *vaultsandbox.NetworkError

	switch {
	case errors.As(err, &apiErr):
		log.Printf("API error %d: %s (request: %s)", apiErr.StatusCode, apiErr.Message, apiErr.RequestID)
	case errors.As(err, &netErr):
		log.Printf("Network error on %s (attempt %d): %v", netErr.URL, netErr.Attempt, netErr.Err)
	case errors.Is(err, vaultsandbox.ErrUnauthorized):
		log.Printf("Invalid API key")
	case errors.Is(err, vaultsandbox.ErrRateLimited):
		log.Printf("Rate limited, retry later")
	default:
		log.Printf("Unexpected error: %v", err)
	}
	return
}
```

### Context Usage

**Do**:

```go
// Use context for cancellation and timeouts
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

email, err := inbox.WaitForEmail(ctx)
```

**Don't**:

```go
// Don't use context.Background() for long operations without timeout
email, err := inbox.WaitForEmail(context.Background()) // May hang forever
```

## Next Steps

- **[Core Concepts: Inboxes](/client-go/concepts/inboxes/)** - Learn about inboxes
- **[Managing Inboxes](/client-go/guides/managing-inboxes/)** - Common inbox operations
- **[Testing Patterns](/client-go/testing/password-reset/)** - Integrate with your tests
- **[API Reference: Client](/client-go/api/client/)** - Full API documentation
