# goRawrSquirrel 🐿️

Opinionated gRPC middleware toolkit for Go.

This package is not a framework. It provides minimal, composable primitives to build and compose middleware, with a focus on keeping things small and interoperable. You can adapt these building blocks to gRPC interceptors without lock‑in.

## Install

```bash
go get github.com/Keksclan/goRawrSquirrel
```

## Quick start

```go
package main

import (
    "context"
    "fmt"
    gs "github.com/Keksclan/goRawrSquirrel"
)

func main() {
    logmw := func(next gs.HandlerFunc) gs.HandlerFunc {
        return func(ctx context.Context) error {
            fmt.Println("before")
            err := next(ctx)
            fmt.Println("after")
            return err
        }
    }

    h := func(ctx context.Context) error {
        fmt.Println("handling...")
        return nil
    }

    wrapped := gs.Wrap(h, logmw)
    _ = wrapped(context.Background())
}
```

## Feature roadmap

- [x] Minimal middleware chain utilities
- [ ] Adapters for gRPC unary/stream interceptors
- [ ] Context tagging helpers
- [ ] Retry/backoff helpers
- [ ] Rate limiting hooks
- [ ] Logging & tracing glue
- [ ] Metrics helpers (Prometheus/OpenTelemetry)
- [ ] Error mapping conventions
- [ ] CI: linting & tests

## License

Apache-2.0
