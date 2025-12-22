// Package delivery provides email delivery strategies for receiving new emails
// from inboxes. It supports multiple delivery mechanisms that can be selected
// based on server capabilities and network conditions.
//
// # Delivery Strategies
//
// The package implements three delivery strategies:
//
//   - [PollingStrategy]: Periodically polls the API for new emails. Uses adaptive
//     backoff to reduce API calls when no new emails arrive. Most reliable but
//     higher latency than SSE.
//
//   - [SSEStrategy]: Uses Server-Sent Events for real-time push notifications.
//     Lowest latency but requires persistent HTTP connection support.
//
//   - [AutoStrategy]: Automatically selects between SSE and polling. Tries SSE
//     first with a timeout, then falls back to polling if SSE fails to connect.
//
// # Usage
//
// All strategies implement the [Strategy] interface for event-driven delivery
// and the [FullStrategy] interface which adds Close():
//
//	cfg := delivery.Config{APIClient: apiClient}
//	strategy := delivery.NewAutoStrategy(cfg)
//
//	inboxes := []delivery.InboxInfo{{Hash: hash, EmailAddress: email}}
//	strategy.Start(ctx, inboxes, func(event *api.SSEEvent) error {
//	    // Handle new email event
//	    return nil
//	})
//	defer strategy.Stop()
//
// Email waiting is handled at the Inbox level using callbacks, which leverages
// SSE for instant notifications when available.
//
// # Backoff and Retry
//
// Both polling and SSE strategies implement exponential backoff with jitter:
//
//   - Polling increases intervals from 2s to 30s max when no changes detected
//   - SSE reconnects with exponential backoff up to 10 attempts
//   - Jitter prevents thundering herd when multiple clients reconnect
//
// # Thread Safety
//
// All strategy types are safe for concurrent use. Inboxes can be added or
// removed while the strategy is running.
package delivery
