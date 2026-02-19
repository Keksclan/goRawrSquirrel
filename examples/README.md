# Examples

This directory contains standalone demo programs that each showcase a single
goRawrSquirrel feature. Every example starts a real gRPC server, exercises the
feature, and prints human-readable output explaining what happens.

## Running

From the repository root:

```bash
go run ./examples/<name>
```

## Index

| Directory | Feature | Description |
|-----------|---------|-------------|
| [`auth-demo`](auth-demo) | Authentication | Registers an `AuthFunc` that validates a bearer token, then sends one unauthenticated and one authenticated request to show accept/reject behaviour. |
| [`cache-demo`](cache-demo) | L1 Cache | Enables the in-process ristretto cache, calls `GetOrSet` twice with a slow loader, and demonstrates that the second call is served from cache (no loader invoked). |
| [`ratelimit-demo`](ratelimit-demo) | Rate Limiting | Configures a tight global token-bucket limit (2 req/s, burst 2) and fires rapid requests, showing which succeed and which are rejected with `ResourceExhausted`. |
| [`auth`](auth) | Authentication (full) | Extended auth example with Actor context enrichment and a registered gRPC service. |
| [`basic`](basic) | Quickstart | Minimal server setup with recovery, rate limiting, auth, and L1 cache wired together. |
| [`cache`](cache) | L1 / L2 Cache | Cache demo that optionally enables Redis L2 via `REDIS_ADDR`. |
| [`ratelimit`](ratelimit) | Rate Limiting + IP Block | Advanced example combining per-group policy limits, global limits, and IP blocking. |

## Guidelines

- Each demo is **standalone** — it requires no external services (unless noted)
  and no `.proto` compilation.
- Demos print **step-by-step output** so you can follow along in the terminal.
- Exit code is non-zero if the demo detects unexpected behaviour, making it
  suitable for CI smoke tests.
