# Traced

This repository contains the implementation of the [Traced Project](https://github.com/benx421/traced), a span ingestion server that receives span batches from concurrent emitters, assembles them into traces, applies a rolling time window, and serves trace data to a dashboard.



## Project Structure

Each implementation is located in its respective directory:

```text
traced/
├── go/          # Go implementation
└── java/        # Java implementation
```

Refer to the README in each language directory for detailed installation, configuration, and usage instructions.

## Development Standards

All implementations follow these standards:

- Mandatory unit tests for core logic
- In-memory storage with a rolling eviction window
- Concurrent-safe reads and writes
- Out-of-order span assembly
- Modular, maintainable code structure
- **TRADEOFFS.md**: Documentation of architectural decisions and trade-offs