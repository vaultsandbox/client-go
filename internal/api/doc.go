// Package api provides HTTP client functionality for communicating with the
// VaultSandbox API. It handles authentication, request/response serialization,
// and automatic retry logic with exponential backoff for transient failures.
//
// # Client Creation
//
// The package provides two ways to create a client:
//
//   - [NewClient]: Struct-based configuration for explicit, type-safe setup.
//   - [New]: Functional options pattern for flexible configuration.
//
// Both methods require an API key and base URL. The API key is sent via the
// X-API-Key header on every request.
//
// # Retry Behavior
//
// The client automatically retries failed requests with exponential backoff.
// By default, requests are retried up to 3 times for these HTTP status codes:
//
//   - 408 Request Timeout
//   - 429 Too Many Requests
//   - 500 Internal Server Error
//   - 502 Bad Gateway
//   - 503 Service Unavailable
//   - 504 Gateway Timeout
//
// The retry delay doubles with each attempt (1s, 2s, 4s, ...). Configure retry
// behavior using [Config.MaxRetries], [Config.RetryDelay], and [Config.RetryOn].
//
// # Error Handling
//
// The package defines sentinel errors for common API error conditions:
//
//   - [ErrUnauthorized]: Invalid or expired API key (401).
//   - [ErrInboxNotFound]: Inbox does not exist (404).
//   - [ErrEmailNotFound]: Email does not exist (404).
//   - [ErrInboxAlreadyExists]: Inbox with that address already exists (409).
//   - [ErrRateLimited]: Rate limit exceeded (429).
//
// Use errors.Is to check for specific error types:
//
//	if errors.Is(err, api.ErrInboxNotFound) {
//	    // Handle missing inbox
//	}
//
// # Thread Safety
//
// The [Client] type is safe for concurrent use. Multiple goroutines may call
// methods on a single Client simultaneously.
package api
