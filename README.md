# Traced — Span Ingestion Server

> Part of the [Backend Engineer Path](https://github.com/benx421/traced).

An HTTP server that ingests distributed trace spans from multiple concurrent services, assembles them into traces, applies a rolling time window, and serves accurate results to a dashboard. Built in Go using only the standard library.

---

## Background

When checkout is slow and nobody knows why, you open the tracing dashboard and see the full picture: checkout called inventory, inventory called payment, payment timed out. Each hop is a **span**. Together they form a **trace**.

This server is the collector. It receives spans from multiple services simultaneously, handles out-of-order delivery (children arriving before their parent), evicts stale data continuously in the background, and serves the assembled traces to a dashboard for querying.

---

## Architecture

```
POST /spans  ─────────────────────────────────────────────────┐
                                                               ▼
                                                     ┌─────────────────┐
GET /traces  ──────────────────────────────────────► │     Handler     │
                                                     │  (HTTP adapter) │
GET /traces/{id}  ─────────────────────────────────► └────────┬────────┘
                                                              │
                                                              ▼
                                                     ┌─────────────────┐
                                                     │      Store      │
                                                     │                 │
                                                     │ map[trace_id →  │
                                                     │    *Trace]      │
                                                     │                 │
                                                     │  sync.Mutex     │
                                                     └────────┬────────┘
                                                              │
                                              ┌───────────────┘
                                              │
                                    ┌─────────▼──────────┐
                                    │  Background Evictor │
                                    │  time.Ticker 30s    │
                                    └────────────────────┘
```

**Handler** — thin HTTP adapter. Parses requests, validates input, delegates to the store, serializes responses. No business logic.

**Store** — owns all shared state. Groups spans by `trace_id`, tracks error status, filters the rolling window, evicts stale data. Protected by a single `sync.Mutex`.

**Background Evictor** — a goroutine started at server startup. Wakes every 30 seconds, acquires the store lock, removes spans older than `WINDOW_MINUTES`, and deletes empty traces.

---

## Project Structure

```
cmd/server/main.go          # wiring: env vars, store, handler, routes
internal/
  handler/handler.go        # HTTP handlers
  store/store.go            # data store, eviction, concurrency
  store/store_test.go       # unit tests
go.mod
TRADEOFFS.md                # storage, eviction, concurrency decisions
```

---

## API

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/spans` | Ingest a batch of spans (1–500) |
| `GET` | `/traces` | List trace summaries within the rolling window |
| `GET` | `/traces/{id}` | Full span tree for a single trace |
| `GET` | `/health` | Health check |

**Query parameters for `GET /traces`:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | integer | 20 | Max traces to return (1–1000) |
| `after` | int64 | — | Root `start_time` >= this value (nanoseconds) |
| `before` | int64 | — | Root `start_time` < this value (nanoseconds) |

Response includes `total` (count before limit) and `traces` ordered by `start_time` descending.

Full contract available at `http://localhost:8081/docs` after starting the dashboard.

---

## Prerequisites

- Go 1.25+
- Docker + Docker Compose
- Make (macOS/Linux)

---

## Running

```bash
# 1. Clone the repository
git clone https://github.com/DanielPopoola/span_ingestion
cd span_ingestion

# 2. Configure environment
cp .env.example .env
# Edit TARGET_URL to point at your server
# Linux: TARGET_URL=http://$(ip route | awk '/default/ { print $3 }'):8000
# macOS/Windows: TARGET_URL=http://host.docker.internal:8000

# 3. Start the server
go run ./cmd/server/

# 4. Start the dashboard (separate terminal)
make up
# Open http://localhost:8081 — enter http://localhost:8000 and click Connect

# 5. Run the emitter and verifier (separate terminal)
make emit
```

---

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `PORT` | `8000` | Server listen port |
| `WINDOW_MINUTES` | `30` | Rolling window duration in minutes |

---

## Tests

```bash
go test ./...
```

Test coverage targets three correctness properties:

- **Out-of-order assembly** — children arriving before the root are stored and linked correctly when the root appears, both within a single batch and across multiple batches
- **Rolling window eviction** — spans outside the window are removed; in-window spans survive; empty traces are deleted
- **Concurrent writes** — 20 goroutines writing simultaneously produce the exact expected span count with no corruption or data loss

---

## Verifier Results

Default load (20 workers, 5 req/s):
```
expected=13905 found=13905 missing=0 span_mismatches=0
all checks passed
```

10x load (200 workers, 5 req/s) — failure mode validation:
```
expected=113708 found=114254 missing=0 span_mismatches=0
all checks passed
```
Timeouts appeared under sustained 10x load as predicted (lock contention). Correctness held — zero missing traces, zero span mismatches — even as throughput degraded.

---

## Design Decisions

See [TRADEOFFS.md](./TRADEOFFS.md) for decisions on storage design, eviction strategy, concurrency model, failure mode analysis, and what would change with more time.