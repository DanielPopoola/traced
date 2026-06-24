# TRADEOFFS.md

## Storage Design

Upon reading the project description for the first time, and seeing massive concurrent writes, I thought to myself the storage layer needs an LSM-like engine. But then upon further evaluation and re-reading the spec, I understood that the project isn't testing durability, it's correctness under load. An in-memory map solves this with fewer moving parts. I even wanted to use Redis but simpler is better always.

The store is a `map[trace_id → *Trace]` where each Trace holds a flat list of spans and a boolean tracking whether any span has status "error". The boolean is set to true on ingest and never reset, if any span in a trace is an error, the trace is an error.

---

## Eviction

There are two paths for eviction.

The first is on ingest: when a span batch arrives, each span's `start_time` is compared to `now - WINDOW_MINUTES`. Spans outside the window are discarded immediately and never written to the store.

The second is background eviction: I have a list of spans stored in a Trace struct. I use a background worker using `time.Ticker` every 30 seconds. It checks if a span is within the time window by comparing `start_time` to `now - WINDOW_MINUTES`. If it's not, it's removed from the trace. In a case where the trace itself becomes empty, it's evicted from the store entirely.

To protect the write operation of the background worker and the HTTP handlers (which also write to the store when spans arrive), I use the same `sync.Mutex` from the write path. It's easier to reason about, like telling all operations "get in a straight line".

---

## Concurrency

Looking at the configuration of the emitter: 20 workers at a rate of 5 requests/second per worker gives 100 requests/second. However the dashboard polls every 3 seconds, comparing the rate of writes to the rate of reads is equivalent to 300:1.

So my approach was to use `sync.Mutex` for writes and for reads, but for reads there's a small side note,  I copy the data from the original array, then do the heavy workload of filtering for root spans, checking if it's in the window duration, and all that outside the lock. This makes sure a goroutine doesn't modify my original data while reading(I could've locked the whole operation to prevent it but it won't work well under load).
---

## What Breaks First

Under 10x load (1,000 writes/second), the single mutex becomes the problem. Goroutines queue up waiting for the lock faster than it's released, like Flash the sloth at the DMV in Zootopia, one slow window, infinite queue. This causes latency to increase rapidly, which leads to memory growth as blocked goroutines accumulate.

I tested it by setting number of workers for the emitter to 200, on startup it was fine at first then all of a sudden I was seeing timeouts errors, but the verfier claimed to have passed even though the logs say otherwise(2026/04/29 10:32:24 INFO done spans=397770 traces=113708
2026/04/29 10:32:32 INFO verifier starting
2026/04/29 10:32:38 INFO verification summary expected=113708 found=114254 missing=0 span_mismatches=0
2026/04/29 10:32:38 INFO all checks passed
)


---

## What I'd Do Differently With More Time

- Replace the in-memory store with a two external stores: (Redis) for fast writes and recent queries, and Postgres for durable audit storage
- Use separate stores with separate their own mutexes to share the load
- Use the Prometheus + Grafana observability layer to get real figures for load testing