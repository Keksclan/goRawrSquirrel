# goRawrSquirrel 🐿️

**Composable, opinionated gRPC middleware toolkit for Go**

---

## Why goRawrSquirrel?

Building production gRPC services means solving the same cross-cutting concerns
repeatedly: recovery, authentication, rate limiting, caching, request tracing.
goRawrSquirrel provides a curated set of middleware primitives that compose
cleanly via functional options — no code generation, no runtime magic, no
framework lock-in.

- **Single `NewServer` call** wires everything together.
- **Deterministic middleware order** — each middleware has a fixed priority, so
  the execution sequence is predictable regardless of option order.
- **Fail-safe defaults** — recovery and request-ID injection are always
  installed when you opt in, keeping every response traceable.

## Features

- [x] Panic recovery (unary + stream)
- [x] Per-request ID injection
- [x] Token-bucket rate limiting (global and per-group)
- [x] Pluggable authentication (`AuthFunc` contract)
- [x] IP blocking
- [x] Policy resolver for method-level rules
- [x] L1 in-process cache (ristretto)
- [x] L2 Redis cache with tiered fallback
- [x] Prometheus metrics endpoint
- [ ] OpenTelemetry tracing glue
- [ ] Retry / back-off helpers
- [ ] Circuit breaker

## Installation

```bash
go get github.com/Keksclan/goRawrSquirrel
```

Requires **Go 1.26** or later.

## Quickstart

```go
package main

import (
	"context"
	"errors"
	"log"
	"net"

	gs "github.com/Keksclan/goRawrSquirrel"
	"google.golang.org/grpc/metadata"
)

func main() {
	srv := gs.NewServer(
		// Recover from panics and tag every request with a unique ID.
		gs.WithRecovery(),

		// Global rate limit: 500 req/s with a burst of 100.
		gs.WithRateLimitGlobal(500, 100),

		// Authentication — inspect metadata and return an error to reject.
		gs.WithAuth(func(ctx context.Context, method string, md metadata.MD) (context.Context, error) {
			if len(md.Get("authorization")) == 0 {
				return ctx, errors.New("missing authorization token")
			}
			return ctx, nil
		}),

		// In-process L1 cache holding up to 10 000 entries.
		gs.WithCacheL1(10_000),
	)

	// Register your gRPC services on the underlying server.
	// pb.RegisterMyServiceServer(srv.GRPC(), &myService{})

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	log.Println("serving on :50051")
	if err := srv.GRPC().Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
```

## Middleware Order Philosophy

Every middleware registers at a fixed priority level. This guarantees a
consistent execution order regardless of how options are passed to `NewServer`:

| Priority | Middleware        |
|----------|-------------------|
| 10       | Recovery          |
| 20       | IP Block          |
| 25       | Rate Limit        |
| 28       | Authentication    |
| 30       | Request ID        |
| 100      | Custom interceptors |

Lower numbers execute first. Recovery always runs outermost so that panics in
any downstream middleware are caught and converted to a proper gRPC status.

## Performance Focus

- **Zero-allocation hot path** — middleware chains are built once at startup;
  per-request overhead is limited to the interceptors you actually enable.
- **Ristretto L1 cache** — contention-friendly, admission-controlled
  in-process cache for latency-sensitive lookups.
- **Tiered caching** — when both L1 and Redis (L2) are configured, reads check
  local memory first, falling back to Redis only on a miss.

## Not a Framework

goRawrSquirrel is a toolkit, not a framework. It produces a standard
`*grpc.Server` — you own the listener, the registration, and the lifecycle.
Nothing prevents you from using only the pieces you need or replacing any
component with your own implementation.

## License

Apache-2.0
