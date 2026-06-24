# Traced

An HTTP server that ingests distributed tracing spans, assembles them into traces, and serves them to a dashboard. Built with Java 21 and Spring Boot.

## What is this?

When a user clicks "checkout" on an e-commerce site, that single action triggers a chain of service calls: checkout → inventory → payment → shipping. Each hop in that chain is a **span**. A **trace** is the full tree of spans that belong to the same request.

This server is the **collector** in that pipeline. It receives span batches from multiple concurrent services, groups them into traces by `trace_id`, maintains a rolling time window of recent data, and serves the assembled traces to a dashboard for inspection.

## Architecture

```
Emitter (20 concurrent workers)
        │
        │ POST /spans (batches)
        ▼
┌─────────────────────────────────┐
│         SpanController          │
│  POST /spans                    │
│  GET  /traces                   │
│  GET  /traces/{id}              │
└────────────┬────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│           TraceStore            │
│                                 │
│  HashMap<traceId, Trace>        │
│  ReadWriteLock                  │
│  Background eviction (@Scheduled│
│  every 30s)                     │
└─────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────┐
│          Dashboard              │
│  GET /traces  (polls every 3s)  │
└─────────────────────────────────┘
```

## Key Design Decisions

**In-memory storage** — all trace data lives in a `HashMap`. No database. The rolling window bounds memory growth: data older than `WINDOW_MINUTES` is continuously evicted in the background.

**ReadWriteLock** — multiple dashboard readers can query simultaneously without blocking each other. Writers (span ingest) take an exclusive lock only when modifying the store.

**Out-of-order spans** — child spans frequently arrive before the root span because services emit spans as they complete. The store accepts and groups all spans by `trace_id` regardless of arrival order. The root span is identified at read time by the absence of a `parent_span_id`.

**Rolling window eviction** — a background thread runs every 30 seconds and removes spans outside the window. Eviction is not deferred to the next request.

## Project Structure

```
src/
└── main/java/com/traced/uchiha/traced/
    ├── TracedApplication.java   # Entry point, enables scheduling
    ├── WebConfig.java           # CORS configuration
    └── span/
        ├── Span.java            # Span record (immutable)
        ├── Trace.java           # Trace (mutable, accumulates spans)
        ├── TraceSummary.java    # Read model for GET /traces
        ├── TraceStore.java      # In-memory store, eviction, locking
        ├── SpanController.java  # HTTP handlers
        ├── IngestRequest.java   # POST /spans request body
        └── TraceListResponse.java # GET /traces response body
```

## Requirements

- Java 21
- Maven 3.8+
- Docker (to run the emitter and dashboard)

## Running

```bash
# Clone the repo
git clone https://github.com/DanielPopoola/traced
cd traced/java

# Start the server
mvn spring-boot:run
```

Server starts on `http://localhost:8080`.

### Configuration

Set via `application.properties` or environment variables:

| Property | Default | Description |
|---|---|---|
| `traced.window-minutes` | `30` | Rolling window size in minutes |
| `server.port` | `8080` | HTTP port |

## API

### `POST /spans`
Ingest a batch of spans.

```bash
curl -X POST http://localhost:8080/spans \
  -H "Content-Type: application/json" \
  -d '{
    "spans": [{
      "traceId": "abc123",
      "spanId": "span1",
      "parentSpanId": null,
      "service": "checkout",
      "operation": "purchase",
      "status": "ok",
      "startTime": 1700000000000000000,
      "endTime":   1700000001000000000
    }]
  }'
```

### `GET /traces`
List all traces in the current window.

```bash
curl "http://localhost:8080/traces?limit=20&after=0&before=0"
```

| Parameter | Default | Description |
|---|---|---|
| `limit` | `20` | Max results (1–1000) |
| `after` | `0` | Filter by startTime after (nanoseconds) |
| `before` | `0` | Filter by startTime before (nanoseconds) |

### `GET /traces/{id}`
Get a single trace with all its spans.

```bash
curl http://localhost:8080/traces/abc123
```

### `GET /health`
Health check. Returns `200 OK`.

## Running the Emitter and Dashboard

Start the server first, then from the `go/` directory:

```bash
# Start the dashboard (http://localhost:8081)
make up

# Run the emitter and verifier against your server
TARGET_URL=http://host.docker.internal:8080 make emit
```

A passing verifier run looks like:

```
INFO verifier starting
INFO verification summary expected=13532 found=13532 missing=0 span_mismatches=0
INFO all checks passed
```

## Running Tests

```bash
mvn test
```

Tests cover:
- Out-of-order span assembly
- Rolling window filtering on ingest
- Background eviction
- Error status propagation
- Concurrent writes under 20 simultaneous threads

## Reference

This is a Java reimplementation of the original Go version in `../go`, built as part of the [Backend Engineer Path](https://github.com/benx421/traced).