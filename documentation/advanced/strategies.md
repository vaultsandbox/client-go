---
title: Delivery Strategies
description: Learn about SSE and polling delivery strategies in VaultSandbox Client for Go
---

VaultSandbox Client supports two email delivery strategies: **Server-Sent Events (SSE)** for real-time updates and **Polling** for compatibility. The SDK intelligently chooses the best strategy automatically or allows manual configuration.

## Overview

When you wait for emails or subscribe to new email notifications, the SDK needs to know when emails arrive. It does this using one of two strategies:

1. **SSE (Server-Sent Events)**: Real-time push notifications from the server
2. **Polling**: Periodic checking for new emails with adaptive backoff

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

The default `StrategyAuto` automatically selects the best delivery method:

1. **Tries SSE first** - Attempts to establish an SSE connection
2. **Falls back to polling** - If SSE fails within 5 seconds, uses polling
3. **Adapts to environment** - Works seamlessly in different network conditions

```go
package main

import (
    "context"
    "os"
    "time"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    ctx := context.Background()

    // Auto strategy (default)
    client, err := vaultsandbox.New(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        // vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto), // Default, can be omitted
    )
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // SDK will automatically choose the best strategy
    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        panic(err)
    }

    email, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(10*time.Second),
    )
    if err != nil {
        panic(err)
    }
}
```

### When Auto Chooses SSE

- Gateway supports SSE
- Network allows persistent connections
- SSE connection established within 5 seconds
- No restrictive proxy/firewall

### When Auto Falls Back to Polling

- Gateway doesn't support SSE
- SSE connection fails
- SSE doesn't connect within timeout
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

```go
client, err := vaultsandbox.New(
    os.Getenv("VAULTSANDBOX_API_KEY"),
    vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),
)
```

### SSE Constants

The SSE strategy uses these configuration values (defined in the `delivery` package):

| Constant                  | Value | Description                                  |
| ------------------------- | ----- | -------------------------------------------- |
| `SSEReconnectInterval`    | 5s    | Base interval before reconnection attempts   |
| `SSEMaxReconnectAttempts` | 10    | Maximum consecutive reconnection attempts    |
| `SSEBackoffMultiplier`    | 2     | Multiplier for exponential backoff           |

### Reconnection Behavior

SSE uses **exponential backoff** for reconnections:

```
1st attempt: SSEReconnectInterval (5s)
2nd attempt: SSEReconnectInterval * 2 (10s)
3rd attempt: SSEReconnectInterval * 4 (20s)
...up to SSEMaxReconnectAttempts
```

### Example Usage

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    ctx := context.Background()

    client, err := vaultsandbox.New(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),
    )
    if err != nil {
        panic(err)
    }
    defer client.Close()

    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        panic(err)
    }
    defer inbox.Delete(ctx)

    // Create cancellable context for watching
    watchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    // Real-time watching (uses SSE)
    for email := range inbox.Watch(watchCtx) {
        fmt.Printf("Instant notification: %s\n", email.Subject)
        if strings.Contains(email.Subject, "Welcome") {
            break
        }
    }
}
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

### Adding Inboxes After Connection

When using SSE, the SDK automatically handles adding new inboxes after the connection is established. If you call `client.CreateInbox` or `client.ImportInbox` while SSE is already connected, the SDK will:

1. **Immediately trigger a reconnection** (without exponential backoff)
2. **Include the new inbox** in the updated connection
3. **Sync all inboxes** after reconnection to catch any emails that arrived during the brief reconnection window

This means you can safely add inboxes dynamically without any manual intervention:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Create initial inbox and start watching
inbox1, _ := client.CreateInbox(ctx)
go func() {
    for email := range inbox1.Watch(ctx) {
        handler(email)
    }
}()

// Later, add another inbox - events start flowing automatically
inbox2, _ := client.CreateInbox(ctx)
go func() {
    for email := range inbox2.Watch(ctx) {
        handler(email)
    }
}()

// Both inboxes now receive real-time events
```

The reconnection is transparent and fast, so there's no need to manually restart the client or coordinate inbox creation timing.

## Polling Strategy

Polling periodically checks for new emails with adaptive backoff and jitter.

### Advantages

- **Universal compatibility**: Works in all environments
- **Firewall-friendly**: Standard HTTP requests
- **Predictable**: Easy to reason about behavior
- **Resilient**: Automatically recovers from transient failures

### Configuration

```go
client, err := vaultsandbox.New(
    os.Getenv("VAULTSANDBOX_API_KEY"),
    vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
)
```

### Polling Constants

The polling strategy uses these configuration values (defined in the `delivery` package):

| Constant                   | Value | Description                                |
| -------------------------- | ----- | ------------------------------------------ |
| `PollingInitialInterval`   | 2s    | Starting interval between polls            |
| `PollingMaxBackoff`        | 30s   | Maximum interval between polls             |
| `PollingBackoffMultiplier` | 1.5   | Multiplier for adaptive backoff            |
| `PollingJitterFactor`      | 0.3   | Random jitter to prevent thundering herd   |

### Adaptive Backoff

The polling strategy uses **sync-status-based change detection** with adaptive backoff:

1. First checks a lightweight sync endpoint for changes
2. Only fetches full email lists when changes are detected
3. When no changes occur, polling intervals gradually increase
4. When changes are detected, intervals reset to initial value
5. Random jitter is added to prevent synchronized polling across clients

```
Initial poll: 2s
No changes: 2s * 1.5 = 3s (+ jitter)
No changes: 3s * 1.5 = 4.5s (+ jitter)
No changes: 4.5s * 1.5 = 6.75s (+ jitter)
...up to 30s maximum
Changes detected: reset to 2s
```

### Example Usage

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

func main() {
    ctx := context.Background()

    client, err := vaultsandbox.New(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
    )
    if err != nil {
        panic(err)
    }
    defer client.Close()

    inbox, err := client.CreateInbox(ctx)
    if err != nil {
        panic(err)
    }
    defer inbox.Delete(ctx)

    // Create cancellable context for watching
    watchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    // Polling-based watching
    for email := range inbox.Watch(watchCtx) {
        fmt.Printf("Polled notification: %s\n", email.Subject)
        if strings.Contains(email.Subject, "Welcome") {
            break
        }
    }
}
```

### Custom Poll Interval

You can override the polling interval for specific wait operations:

```go
// Default client polling
client, _ := vaultsandbox.New(
    os.Getenv("VAULTSANDBOX_API_KEY"),
    vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
)

inbox, _ := client.CreateInbox(ctx)

// Override for specific operation (faster)
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithWaitTimeout(30*time.Second),
    vaultsandbox.WithPollInterval(1*time.Second), // Check every 1s for this operation
)
```

### When to Use Polling

- **Corporate networks**: Restrictive firewall/proxy environments
- **CI/CD pipelines**: Guaranteed compatibility
- **Rate-limited APIs**: Avoid hitting request limits
- **Debugging**: Predictable request timing
- **Low email volume**: Polling overhead is minimal

## Choosing the Right Strategy

### Use Auto (Default)

For most use cases, let the SDK choose:

```go
client, err := vaultsandbox.New(
    os.Getenv("VAULTSANDBOX_API_KEY"),
    // strategy defaults to StrategyAuto
)
```

**Best for:**

- General testing
- Unknown network conditions
- Mixed environments (dev, staging, CI)
- When you want it to "just work"

### Force SSE

When you need guaranteed real-time performance:

```go
client, err := vaultsandbox.New(
    os.Getenv("VAULTSANDBOX_API_KEY"),
    vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),
)
```

**Best for:**

- Local development (known to support SSE)
- Real-time monitoring dashboards
- High-volume email testing
- Latency-sensitive tests

**Caveat:** Will fall back to polling if SSE fails to connect.

### Force Polling

When compatibility is more important than speed:

```go
strategy := vaultsandbox.StrategyPolling
if os.Getenv("CI") != "" {
    strategy = vaultsandbox.StrategyPolling
}

client, err := vaultsandbox.New(
    os.Getenv("VAULTSANDBOX_API_KEY"),
    vaultsandbox.WithDeliveryStrategy(strategy),
)
```

**Best for:**

- CI/CD environments (guaranteed to work)
- Corporate networks with restrictive proxies
- When SSE is known to be problematic
- Rate-limited scenarios

## Environment-Specific Configuration

### Development

Fast feedback with SSE:

```go
func newDevClient() (*vaultsandbox.Client, error) {
    return vaultsandbox.New(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithBaseURL("http://localhost:3000"),
        vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),
    )
}
```

### CI/CD

Reliable polling:

```go
func newCIClient() (*vaultsandbox.Client, error) {
    return vaultsandbox.New(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
    )
}
```

### Production Testing

Auto with reasonable defaults:

```go
func newProductionClient() (*vaultsandbox.Client, error) {
    return vaultsandbox.New(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto),
        vaultsandbox.WithTimeout(60*time.Second),
    )
}
```

### Environment-Aware Factory

```go
func createClient() (*vaultsandbox.Client, error) {
    opts := []vaultsandbox.Option{
        vaultsandbox.WithBaseURL(os.Getenv("VAULTSANDBOX_URL")),
    }

    switch {
    case os.Getenv("CI") != "":
        // CI: Reliable polling
        opts = append(opts, vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling))
    case os.Getenv("GO_ENV") == "development":
        // Dev: Fast SSE
        opts = append(opts, vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE))
    default:
        // Production: Auto
        opts = append(opts, vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto))
    }

    return vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"), opts...)
}
```

## Monitoring Strategy Performance

### Measure Email Delivery Latency

```go
func measureDeliveryLatency(ctx context.Context) {
    client, _ := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
    defer client.Close()

    inbox, _ := client.CreateInbox(ctx)
    defer inbox.Delete(ctx)

    startTime := time.Now()

    // Send email
    sendTestEmail(inbox.EmailAddress())

    // Wait for email
    _, err := inbox.WaitForEmail(ctx,
        vaultsandbox.WithWaitTimeout(30*time.Second),
    )
    if err != nil {
        panic(err)
    }

    latency := time.Since(startTime)
    fmt.Printf("Email delivery latency: %v\n", latency)
}
```

### Compare Strategies

```go
func compareStrategies(ctx context.Context) {
    // Test SSE
    sseClient, _ := vaultsandbox.New(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),
    )
    defer sseClient.Close()

    sseInbox, _ := sseClient.CreateInbox(ctx)
    sseStart := time.Now()
    sendTestEmail(sseInbox.EmailAddress())
    sseInbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
    sseLatency := time.Since(sseStart)

    // Test Polling
    pollClient, _ := vaultsandbox.New(
        os.Getenv("VAULTSANDBOX_API_KEY"),
        vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling),
    )
    defer pollClient.Close()

    pollInbox, _ := pollClient.CreateInbox(ctx)
    pollStart := time.Now()
    sendTestEmail(pollInbox.EmailAddress())
    pollInbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
    pollLatency := time.Since(pollStart)

    fmt.Printf("SSE latency: %v\n", sseLatency)
    fmt.Printf("Polling latency: %v\n", pollLatency)
    fmt.Printf("Difference: %v\n", pollLatency-sseLatency)

    sseInbox.Delete(ctx)
    pollInbox.Delete(ctx)
}
```

## Troubleshooting

### SSE Connection Failures

When SSE fails, the auto strategy automatically falls back to polling:

```go
import (
    "errors"

    vaultsandbox "github.com/vaultsandbox/client-go"
)

// Using auto strategy handles SSE failures gracefully
client, err := vaultsandbox.New(
    os.Getenv("VAULTSANDBOX_API_KEY"),
    vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto),
)
if err != nil {
    // Handle initialization error
    var netErr *vaultsandbox.NetworkError
    if errors.As(err, &netErr) {
        fmt.Printf("Network error: %v\n", netErr.Err)
    }
}

// For explicit SSE strategy, the SDK will fall back to polling on failure
client, err = vaultsandbox.New(
    os.Getenv("VAULTSANDBOX_API_KEY"),
    vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE),
)
if err != nil {
    var netErr *vaultsandbox.NetworkError
    if errors.As(err, &netErr) {
        fmt.Printf("Connection failed: %v\n", netErr.Err)
        // Consider using polling strategy
    }
}
```

### Polling Too Slow

If emails arrive slowly with polling:

```go
// Solution 1: Use faster poll interval for specific operations
email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithPollInterval(500*time.Millisecond), // Fast polling for this operation
    vaultsandbox.WithWaitTimeout(10*time.Second),
)

// Solution 2: Use SSE if available
client, _ := vaultsandbox.New(
    os.Getenv("VAULTSANDBOX_API_KEY"),
    vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE), // Real-time delivery
)

// Solution 3: Use auto and let it choose
client, _ := vaultsandbox.New(
    os.Getenv("VAULTSANDBOX_API_KEY"),
    vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto),
)
```

### Context Cancellation

All wait operations respect context cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithSubject("Welcome"),
)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        fmt.Println("Timed out waiting for email")
    } else if errors.Is(err, context.Canceled) {
        fmt.Println("Wait was canceled")
    }
}
```

## Best Practices

### 1. Use Auto Strategy by Default

Let the SDK choose unless you have specific requirements:

```go
// Good: Let SDK choose
client, _ := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))

// Only specify when needed
ciClient, _ := vaultsandbox.New(
    os.Getenv("VAULTSANDBOX_API_KEY"),
    vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling), // CI needs guaranteed compatibility
)
```

### 2. Tune for Environment

Configure differently for each environment:

```go
func createClient() (*vaultsandbox.Client, error) {
    opts := []vaultsandbox.Option{}

    if os.Getenv("CI") != "" {
        // CI: Reliable polling
        opts = append(opts, vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyPolling))
    } else if os.Getenv("GO_ENV") == "development" {
        // Dev: Fast SSE
        opts = append(opts, vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategySSE))
    } else {
        // Production: Auto
        opts = append(opts, vaultsandbox.WithDeliveryStrategy(vaultsandbox.StrategyAuto))
    }

    return vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"), opts...)
}
```

### 3. Always Use Contexts

All operations should use contexts for timeout and cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

email, err := inbox.WaitForEmail(ctx,
    vaultsandbox.WithSubjectRegex(regexp.MustCompile(`Welcome`)),
)
```

### 4. Clean Up Resources

Always close clients and unsubscribe from subscriptions:

```go
client, _ := vaultsandbox.New(os.Getenv("VAULTSANDBOX_API_KEY"))
defer client.Close()

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

inbox, _ := client.CreateInbox(ctx)
defer inbox.Delete(ctx)

go func() {
    for email := range inbox.Watch(ctx) {
        fmt.Println(email.Subject)
    }
}()
```

### 5. Handle Errors Appropriately

Use Go's error handling patterns:

```go
import "errors"

email, err := inbox.WaitForEmail(ctx, vaultsandbox.WithWaitTimeout(10*time.Second))
if err != nil {
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        fmt.Println("Timeout waiting for email")
    case errors.Is(err, vaultsandbox.ErrInboxNotFound):
        fmt.Println("Inbox was deleted or has expired")
    default:
        fmt.Printf("Unexpected error: %v\n", err)
    }
    return
}
```

## Next Steps

- [Real-time Monitoring Guide](/client-go/guides/real-time) - Using subscriptions
- [Configuration Reference](/client-go/configuration) - All config options
- [Error Handling](/client-go/api/errors) - Handle SSE errors
- [CI/CD Integration](/client-go/testing/cicd) - Strategy for CI environments
