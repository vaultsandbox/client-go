# Callback Concurrency Options (Evaluation Plan)

This document outlines options for keeping the callback-based API while adding safety for bursty or slow handlers. Each option includes example changes and usage patterns so you can compare impact and complexity.

## Baseline (Current)
**Behavior:** For each email event, each registered callback runs in its own goroutine.
**Pros:** Minimal latency; simplest API.
**Cons:** Unbounded goroutines if emails or callbacks spike.

**Example (current behavior):**
```go
for _, cb := range callbacks {
    go cb(email)
}
```

---

## Option A: Concurrency Cap (Semaphore)
**Behavior:** Limits number of concurrent callback goroutines. Extra events wait for a slot.
**Pros:** Simple; keeps API the same; prevents runaway goroutines.
**Cons:** If handlers are slow, events back up and delay delivery.

**Example change (monitor dispatch):**
```go
// In InboxMonitor
sem := make(chan struct{}, 4) // max 4 concurrent handler goroutines

for _, cb := range callbacks {
    sem <- struct{}{}
    go func(cb Callback) {
        defer func() { <-sem }()
        cb(email)
    }(cb)
}
```

**How to use:**
- Default cap could be 4.
- Add option: `WithCallbackConcurrency(n int)`.

**Evaluation:**
- Send 25 emails with a 200ms handler.
- Observe steady concurrency and predictable total time.

---

## Option B: Single Dispatcher (Serial)
**Behavior:** Events are delivered one at a time in a single goroutine.
**Pros:** Simple; deterministic; no concurrent handler execution.
**Cons:** Slow handlers block all delivery.

**Example change:**
```go
// In InboxMonitor
for _, cb := range callbacks {
    cb(email)
}
```

**How to use:**
- Best for strictly low-volume, fast handlers.

**Evaluation:**
- Send 25 emails with a 200ms handler.
- Expect total time ~25 * 200ms per callback.

---

## Option C: Buffered Dispatch Queue + Worker Pool
**Behavior:** Events are queued and handled by a small fixed worker pool.
**Pros:** Bounded concurrency; avoids spawning per-callback goroutines.
**Cons:** More internal machinery; queue can fill.

**Example change (internal queue):**
```go
// queue size 50, worker count 4
queue := make(chan *Email, 50)

for i := 0; i < 4; i++ {
    go func() {
        for email := range queue {
            for _, cb := range callbacks {
                cb(email)
            }
        }
    }()
}

// on event
queue <- email
```

**How to use:**
- Add options: `WithCallbackQueue(n int)`, `WithCallbackWorkers(n int)`.

**Evaluation:**
- Send 100 emails quickly; verify queue backs up but concurrency stays fixed.

---

## Option D: Best-Effort (Drop on Overload)
**Behavior:** If queue or semaphore is full, drop the event or callback to protect process.
**Pros:** Hard cap on resource use; prevents stalls.
**Cons:** Can lose events; only suitable for non-critical callbacks.

**Example change:**
```go
select {
case sem <- struct{}{}:
    go func(cb Callback) {
        defer func() { <-sem }()
        cb(email)
    }(cb)
default:
    // drop or log
}
```

**How to use:**
- Option: `WithCallbackDropOnFull(true)`.

**Evaluation:**
- Send 100 emails with a 1s handler; verify drops happen.

---

## Option E: Channel-Based API (Idiomatic Go)
**Behavior:** Events are pushed into a channel; consumers range/select on it.
**Pros:** Idiomatic; natural backpressure; easy to integrate with `select`/`context`.
**Cons:** Different usage style; requires changes to public API.

**Example change (public API):**
```go
// In Inbox
func (i *Inbox) Events(ctx context.Context) <-chan *Email {
    ch := make(chan *Email, 10)
    // internal dispatcher writes into ch
    go func() {
        defer close(ch)
        for {
            select {
            case <-ctx.Done():
                return
            case email := <-i.internalEvents:
                ch <- email
            }
        }
    }()
    return ch
}
```

**How to use:**
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

for email := range inbox.Events(ctx) {
    // handle email
    break
}
```

**Evaluation:**
- Send 25 emails rapidly; observe consumer pacing via channel buffer.

---

## Option X: Internal Channel Backbone (No Public API Change)
**Behavior:** Internally route events through a private channel, but keep callbacks as the public API.
**Pros:** Keeps current UX; makes a future `Events(ctx)` trivial to add.
**Cons:** Small internal refactor now; no direct user-facing benefit yet.

**Example change (internal):**
```go
// internal event bus
events := make(chan *Email, 10)

// producer writes to events
events <- email

// dispatcher consumes and calls callbacks
go func() {
    for email := range events {
        for _, cb := range callbacks {
            go cb(email)
        }
    }
}()
```

**How to use:** No change for consumers.

**Evaluation:**
- Verify existing callback tests pass.
- Confirm no behavioral changes for single email delivery.

---

## Recommendation for Your Use Case
- If you want minimal change: **Option A** with `WithCallbackConcurrency(4)`.
- If you want absolute simplicity: **Option B** (serial delivery).
- If you want bounded concurrency + buffering: **Option C**.
- If you want idiomatic Go patterns: **Option E**.
- If you want to keep callbacks now but prepare for channels later: **Option X**.

---

## Quick Comparison Table
- **Lowest complexity:** Option B
- **Best safety/effort balance:** Option A
- **Most control:** Option C
- **Hard safety cap:** Option D
- **Most idiomatic Go:** Option E
- **Future-proofing with no API change:** Option X

---

## Current vs Channel (Code-Wise)
**Current (callbacks):**
```go
inbox.OnEmail(func(email *Email) {
    // handle email
})
```

**Channel-based:**
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

for email := range inbox.Events(ctx) {
    // handle email
    break
}
```
