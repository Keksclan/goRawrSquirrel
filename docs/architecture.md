# Architecture

> Internal design reference for contributors and advanced integrators.
> For usage examples and the public API surface see the [README](../README.md).

---

## 1. Project Structure

```
goRawrSquirrel/
├── server.go            # Server type, NewServer entry-point
├── options.go           # Functional options (Option closures)
├── config.go            # Private config struct assembled by options
├── defaults.go          # DefaultOptions() convenience bundle
│
├── internal/
│   └── core/
│       ├── middleware.go # MiddlewareBuilder — priority-sorted collector
│       └── builder.go   # Translates interceptor slices → grpc.ServerOption
│
├── interceptors/
│   ├── chain.go         # ChainUnary / ChainStream — closure-based chaining
│   ├── recovery.go      # Panic recovery (unary + stream)
│   ├── requestid.go     # Per-request UUID injection
│   ├── auth.go          # AuthFunc adapter
│   ├── ratelimit.go     # Token-bucket with policy-aware override
│   └── ipblock.go       # IP allow/deny interceptor
│
├── cache/
│   ├── cache.go         # Cache interface (Get / Set / GetOrSet)
│   ├── l1.go            # Ristretto-backed in-process cache
│   ├── redis.go         # go-redis L2 implementation
│   └── tiered.go        # L1 → L2 → loader composition with singleflight
│
├── policy/
│   ├── group.go         # GroupBuilder, Policy, rule, matchKind
│   ├── matcher.go       # rule.match() — exact / prefix / regex
│   └── resolver.go      # Resolver.Resolve() — best-match dispatch
│
├── security/
│   ├── ipblock.go       # IPBlocker (CIDR parsing, allow/deny evaluation)
│   └── resolver.go      # Client IP resolution (peer, trusted proxies, headers)
│
├── auth/
│   └── auth.go          # AuthFunc type definition
│
├── contextx/
│   ├── keys.go          # Private context-key types
│   ├── actor.go         # Actor value in context
│   ├── group.go         # Group value in context
│   └── requestid.go     # Request-ID value in context
│
├── ratelimit/
│   └── limiter.go       # Token-bucket limiter (golang.org/x/time/rate)
│
├── docs/
│   └── usage.md
└── examples/
```

### Dependency direction

```
server.go / options.go
    │
    ├──► internal/core      (build-time wiring, unexported)
    ├──► interceptors        (gRPC interceptor implementations)
    ├──► cache               (caching contract + implementations)
    ├──► policy              (method-level policy resolution)
    ├──► security            (IP evaluation logic)
    ├──► auth                (AuthFunc contract)
    └──► ratelimit           (token-bucket primitive)
```

Leaf packages (`auth`, `contextx`, `ratelimit`, `security`, `policy`, `cache`)
have **zero intra-project imports**. They depend only on the standard library,
gRPC, and their specific third-party library (ristretto, go-redis,
x/time/rate). This makes each package independently testable and replaceable.

---

## 2. Middleware Layering Model

The middleware system is split into three distinct phases that execute at
different times:

### Phase 1 — Collection (server init)

Each `Option` closure calls `MiddlewareBuilder.Add(order, unary, stream)`,
appending a `middleware` entry that pairs an integer priority with an optional
unary and/or stream interceptor. No sorting happens yet; options can be
supplied in any order.

### Phase 2 — Sorting & Flattening (server init)

`MiddlewareBuilder.Build()` runs a **stable sort** on collected entries by
`Order`. It then splits the sorted list into two flat slices:
`[]grpc.UnaryServerInterceptor` and `[]grpc.StreamServerInterceptor`,
skipping nil entries. Stable sort guarantees that interceptors registered at
the same priority level preserve their insertion order.

### Phase 3 — Chain Composition (server init)

`interceptors.ChainUnary` / `ChainStream` fold the sorted slices into a
single `grpc.UnaryServerInterceptor` / `grpc.StreamServerInterceptor` via
recursive closure construction. The outermost closure invokes interceptor[0],
whose `handler` argument invokes interceptor[1], and so on until the real
handler is reached. This produces a classic onion model.

`core.BuildServerOptions` converts the two composed interceptors into
`grpc.ServerOption` values (`grpc.UnaryInterceptor`, `grpc.StreamInterceptor`)
passed to `grpc.NewServer`. After this point the chain is frozen — **zero
per-request allocation** for chaining itself.

### Why closure-based chaining instead of `grpc.ChainUnaryInterceptor`?

The library composes its own chain so that it controls the exact folding
order and can guarantee determinism. Using gRPC's built-in chain helpers would
work but would require trusting their implementation details across versions.
Own chaining also lets the library short-circuit on empty/single-interceptor
slices with zero overhead.

---

## 3. Interceptor Ordering Philosophy

Every middleware is associated with a **compile-time constant** priority:

| Constant           | Value | Rationale                                                                              |
|--------------------|------:|----------------------------------------------------------------------------------------|
| `orderRecovery`    |    10 | Must be outermost so every downstream panic is caught.                                 |
| `orderIPBlock`     |    20 | Reject banned IPs before spending CPU on auth or rate-limit accounting.                |
| `orderRateLimit`   |    25 | Apply rate limits before authentication to protect the auth layer from floods.         |
| `orderAuth`        |    28 | Authenticate after rate-limiting; no point verifying tokens for throttled requests.    |
| `orderRequestID`   |    30 | Inject a trace ID only for requests that survived the security/quota gauntlet.         |
| `orderInterceptor` |   100 | User-supplied interceptors always run innermost, closest to the handler.               |

Design consequences:

* **Option call order is irrelevant.** `WithAuth` before `WithRecovery` or
  vice-versa — the runtime chain is identical because the stable sort on
  `Order` is deterministic.
* **Same-priority stability.** Multiple `WithUnaryInterceptor` calls share
  `orderInterceptor = 100`. Because the sort is stable, they execute in the
  order the caller supplied them, giving users fine-grained control within the
  custom-interceptor band.
* **Recovery always installs Request-ID.** `WithRecovery()` adds both the
  recovery interceptor (order 10) and the request-ID interceptor (order 30)
  in a single call. This co-registration prevents a common misconfiguration
  where panics are caught but responses lack a trace ID.

---

## 4. Policy Resolution Model

The `policy` package implements a method-level rule engine that other
interceptors (currently `ratelimit`) query at request time.

### Data model

```
Resolver
  └── []GroupBuilder
        ├── name    string
        ├── rules   []rule          ← matching predicates
        │     ├── kind    matchKind   (kindExact | kindPrefix | kindRegex)
        │     ├── pattern string
        │     └── re      *regexp.Regexp  (only for kindRegex)
        └── policy  *Policy
              ├── RateLimit    *RateLimitRule
              ├── Timeout      time.Duration
              └── AuthRequired bool
```

### Resolution algorithm (`Resolver.Resolve`)

For a given `fullMethod` string (e.g. `/myapp.v1.Users/GetUser`):

1. Iterate every rule in every group.
2. Call `rule.match(fullMethod)` which returns `(matched bool, length int)`.
3. Pick the best match using a strict priority cascade:
   - **Match kind** — `kindExact` (0) > `kindPrefix` (1) > `kindRegex` (2).
     Lower `matchKind` value wins.
   - **Match length** — among rules of the same kind, longer match wins.
   - **Registration order** — among equal-kind, equal-length rules, the group
     registered first in `NewResolver(groups...)` wins (iteration order).

4. Return `(groupName, *Policy, true)` or `("", nil, false)` if nothing
   matched.

### Integration pattern

Interceptors that are policy-aware (e.g. `interceptors.RateLimitUnary`)
accept a `*policy.Resolver` and call `Resolve` on every request. If a match
is found and the policy contains a relevant rule (e.g. `RateLimit`), the
per-group configuration overrides the global default. If no policy matches,
the global configuration applies. This keeps the interceptor code simple —
it never interprets policy semantics itself, it just checks for a non-nil
field.

---

## 5. Caching Architecture (L1 + L2)

### Contract

```go
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, bool, error)
    Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
    GetOrSet(ctx context.Context, key string, ttl time.Duration,
             loader func(context.Context) ([]byte, error)) ([]byte, error)
}
```

All three implementations (`L1`, `L2`, `Tiered`) satisfy this interface.
Callers interact only with `Cache`; they never know which layer served the
response.

### L1 — `cache.L1`

Wraps `dgraph-io/ristretto`, an admission-controlled concurrent cache.
Ristretto was chosen because:

* It uses a TinyLFU admission policy, which avoids cache pollution from
  one-hit-wonder keys.
* It is contention-friendly under high goroutine counts (sharded internals).
* It manages its own memory budget — callers specify `maxEntries`, which is
  translated to a cost-based capacity.

### L2 — `cache.L2`

Wraps `go-redis/redis`. Serialization is raw `[]byte` — the caller owns
marshalling. Redis failures are **soft**: operations return errors rather
than panicking, and the `Tiered` wrapper tolerates L2 errors gracefully.

### Tiered — `cache.Tiered`

When both L1 and L2 are configured, `NewServer` creates a `Tiered` instance
that composes them:

```
Read path:   L1 hit? → return
             L1 miss → L2 hit? → promote to L1 → return
             L2 miss → call loader → store L2 → store L1 → return

Write path:  store L2 → store L1
```

**Singleflight on loader calls.** `Tiered.GetOrSet` uses a hand-rolled
singleflight mechanism (`sync.Mutex` + per-key `sync.WaitGroup`) to ensure
that concurrent cache misses for the same key invoke the loader exactly once.
Subsequent waiters receive a `bytes.Clone` of the result, preventing shared-
buffer mutation.

The choice of a hand-rolled singleflight (instead of `golang.org/x/sync/singleflight`)
is deliberate: `x/sync/singleflight` returns the same pointer to all callers,
which would require defensive copying anyway, and its `Forget`/`DoChan`
features are unnecessary here.

### Tiered assembly

```go
// server.go — NewServer
if cfg.l1 != nil && cfg.l2 != nil {
    cfg.cache = cache.NewTiered(cfg.l1, cfg.l2)
}
```

If only L1 is configured, `cfg.cache` points directly to the `L1` instance.
If only L2 is configured, the user gets a raw `L2` (no tiering). The `Server`
exposes the final `Cache` via `Server.Cache()`.

---

## 6. Separation of Concerns

### Root package (`gorawrsquirrel`)

**Role:** Public API surface and wiring.

Contains `Server`, `Option`, `config`, `NewServer`, and `DefaultOptions`.
This package imports everything else but exports only the types and functions
that consumers need. The private `config` struct ensures that internal
assembly state never leaks.

### `internal/core`

**Role:** Build-time plumbing that must not be imported by external code.

`MiddlewareBuilder` collects, sorts, and splits middleware entries.
`BuildServerOptions` converts interceptor slices to `grpc.ServerOption`.
These are implementation details of the root package; placing them under
`internal` enforces that guarantee at the compiler level.

### `interceptors`

**Role:** gRPC interceptor adapters — one file per concern.

Each interceptor is a pure function that returns
`grpc.UnaryServerInterceptor` and/or `grpc.StreamServerInterceptor`. They
depend on domain packages (`auth`, `security`, `ratelimit`, `policy`) for
logic and on `contextx` for propagating values, but they never import the
root package or `internal/core`. This keeps them testable in isolation.

`chain.go` lives here because it is an interceptor-level primitive (folding
interceptor slices), not a core-wiring concern.

### `cache`

**Role:** Caching contract and implementations.

Defines the `Cache` interface and provides `L1`, `L2`, and `Tiered`
implementations. No knowledge of gRPC, interceptors, or policies. This
makes it reusable outside the middleware context — any part of a service
can call `Server.Cache()` for general-purpose caching.

### `policy`

**Role:** Method-level rule matching and policy definitions.

Purely declarative: `GroupBuilder` builds matching rules, `Resolver`
evaluates them. The `Policy` struct is a plain data object with no behavior.
This package has no gRPC dependency at all — it operates on plain strings.

### `security`

**Role:** IP-level access control.

`IPBlocker` evaluates allow/deny lists against client IPs. `resolver.go`
extracts the real client IP from gRPC peer info and metadata headers,
handling trusted-proxy traversal. Separated from `interceptors` because IP
resolution logic is useful outside the interceptor context (e.g., logging,
audit).

### `auth`

**Role:** Authentication contract.

Defines `AuthFunc` — a single function type. This exists as its own package
to break an import cycle: `interceptors` needs the type, but the root
package also references it in `WithAuth`. A dedicated package lets both
import it without circular dependencies.

### `contextx`

**Role:** Typed context-value accessors.

Provides `WithActor` / `ActorFromContext`, `WithRequestID` /
`RequestIDFromContext`, and group-level equivalents. Private key types
prevent collisions. This package has zero external dependencies — only
`context` from the standard library.

### `ratelimit`

**Role:** Token-bucket primitive.

Wraps `golang.org/x/time/rate` into a `Limiter` type. Isolated so that
the rate-limiting algorithm can be swapped or extended without touching
interceptor code.

---

## 7. Why No Framework Magic

goRawrSquirrel is designed as a **toolkit**, not a framework. Concrete design
decisions that enforce this:

1. **`NewServer` returns a value, not a singleton.** There is no global
   state, no `init()` side effects, no package-level registries. You can
   create multiple `Server` instances in the same process with different
   configurations (useful for tests and multi-tenant setups).

2. **`Server.GRPC()` exposes the raw `*grpc.Server`.** The caller owns the
   listener, service registration, graceful shutdown, and health checking.
   goRawrSquirrel never calls `Serve` or `GracefulStop` — those are your
   decisions.

3. **No code generation.** The library operates entirely at the interceptor
   level. Protobuf definitions, service implementations, and message types
   are none of its concern.

4. **No reflection or struct tags.** Configuration is expressed through
   typed functional options. Policy rules use explicit builder calls (`Exact`,
   `Prefix`, `Regex`), not annotation strings that would be parsed at runtime.

5. **No interface pollution.** External contracts are minimal:
   - `cache.Cache` (3 methods)
   - `auth.AuthFunc` (1 function type)

   Everything else is concrete types. This avoids the "interface for
   everything" anti-pattern and lets the compiler catch misuse.

6. **Deterministic, debuggable startup.** The entire middleware chain is
   built synchronously inside `NewServer`. There are no lazy-init paths,
   no background goroutines spawned at import time, and no deferred
   configuration that could fail at first request.

---

## 8. How to Extend the Library Safely

### Adding a new interceptor

1. Create a new file in `interceptors/` (e.g., `interceptors/timeout.go`).
2. Implement a function that returns `grpc.UnaryServerInterceptor` and/or
   `grpc.StreamServerInterceptor`.
3. If the interceptor needs domain logic, place it in a dedicated leaf
   package (like `ratelimit/` or `security/`).
4. Add a priority constant in `options.go` (pick a value that reflects where
   in the chain the interceptor should execute).
5. Write a `With…()` option that calls `c.middlewares.Add(order, unary, stream)`.
6. Write tests — interceptors are plain functions, so table-driven tests with
   a fake `grpc.UnaryHandler` are straightforward.

### Adding a new cache layer

Implement the `cache.Cache` interface. The three-method contract (`Get`,
`Set`, `GetOrSet`) is intentionally narrow. To integrate it as a new tier:

- Compose it with existing layers by writing a new combinator similar to
  `Tiered`, or replace `Tiered` with a more general N-layer approach.
- Wire it in `NewServer` inside `server.go`, following the existing
  `if cfg.l1 != nil && cfg.l2 != nil` pattern.

### Adding new policy fields

1. Add the field to `policy.Policy` (e.g., `CircuitBreaker *CBConfig`).
2. In the interceptor that should respect the field, add a nil-check after
   calling `resolver.Resolve()`. If the field is present, use the per-group
   config; otherwise fall back to the global default.
3. The resolver itself requires no changes — it already returns the full
   `*Policy` to the caller.

### Propagating new request-scoped values

1. Add a new file in `contextx/` with a private key type and
   `With…` / `…FromContext` pair.
2. Set the value in the appropriate interceptor.
3. Read it in downstream handlers or interceptors via the accessor.

### Rules of thumb

- **Never import the root package from a leaf package.** Dependency arrows
  point inward (root → leaf), never outward. If you need a type from the
  root package in a leaf, extract it into a shared leaf package (the way
  `auth.AuthFunc` was extracted).
- **Never mutate `config` after `NewServer` returns.** The config is consumed
  once during construction and discarded. Post-init mutation would have no
  effect and signals a design mistake.
- **Keep interceptors stateless.** Any mutable state (rate-limiter buckets,
  cache entries) should live in the domain package. The interceptor is just
  the adapter that bridges gRPC's interceptor signature to the domain call.
- **Prefer concrete types over interfaces** unless you genuinely need
  substitutability in tests or across implementations. Premature interfaces
  increase cognitive load without adding value.
