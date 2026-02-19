# Performance Guide

This document describes the performance characteristics of goRawrSquirrel and
provides guidance for tuning a production deployment.

---

## Allocation Philosophy

goRawrSquirrel follows a **zero-allocation hot path** design. All middleware
chains are assembled once during `NewServer` and never reallocated:

- `ChainUnary` / `ChainStream` compose the interceptor slice into a single
  closure at startup. The per-request path executes the pre-built closure
  directly — no slice iteration, no reflection, no interface boxing.
- Cache values are copied with `bytes.Clone` on read so that callers receive
  an isolated slice. This avoids sharing mutable memory between goroutines
  while keeping allocations predictable (one alloc per cache hit).
- The `call` deduplication mechanism in L1 uses a `sync.WaitGroup` per
  in-flight key so that concurrent requests for the same cache entry share a
  single loader invocation instead of each allocating their own.

The net effect is that the only allocations on a typical request are the ones
you opt into (e.g., reading a cached value or generating a request ID).

---

## Interceptor Ordering for Performance

Every middleware in goRawrSquirrel has a fixed **priority number**. Lower
values execute first, wrapping all higher-numbered middleware:

| Priority | Middleware         | Purpose                                |
|----------|--------------------|----------------------------------------|
| 10       | Recovery           | Catch panics, guarantee a gRPC status  |
| 20       | IP Block           | Reject banned IPs immediately          |
| 25       | Rate Limit         | Enforce token-bucket throughput cap    |
| 28       | Authentication     | Validate credentials / tokens          |
| 30       | Request ID         | Tag the request for tracing            |
| 100      | Custom interceptors| User-supplied middleware                |

This ordering is intentional and performance-motivated. The guiding principle
is: **reject as early and as cheaply as possible**.

### Why IP Block Comes Before Rate Limit

IP blocking (priority 20) is a simple set-membership check — typically an
in-memory map or CIDR lookup. It requires no external I/O, no token
accounting, and no synchronisation beyond a read lock. By placing it before the
rate limiter, banned IPs are dropped before they consume a token from the
bucket. This prevents a denial-of-service from a blocked address from
exhausting the legitimate rate-limit budget.

### Why Rate Limit Comes Before Authentication

Rate limiting (priority 25) runs before authentication (priority 28) because:

1. **Cost**: A token-bucket check is an atomic compare-and-swap on a local
   counter. Authentication may involve parsing JWTs, querying a database, or
   calling an external identity provider — orders of magnitude more expensive.
2. **Protection**: If rate limiting ran *after* auth, an attacker could flood
   the auth layer with invalid tokens, consuming CPU and network resources
   before the rate limiter ever fires.
3. **Fairness**: Applying the rate limit early ensures that the throughput cap
   reflects *all* incoming traffic, not just the subset that passes
   authentication.

---

## Caching Strategy and Cost Tradeoffs

goRawrSquirrel provides a two-tier caching model:

| Tier | Implementation | Latency | Capacity | Scope          |
|------|----------------|---------|----------|----------------|
| L1   | Ristretto      | ~ns     | Bounded by `maxCost` | Per-process |
| L2   | Redis          | ~ms     | Limited by Redis memory | Shared across processes |

### L1 — In-Process Cache

The L1 cache (`cache.L1`) wraps [dgraph-io/ristretto](https://github.com/dgraph-io/ristretto),
an admission-controlled, concurrent-safe cache. Key characteristics:

- **Admission policy**: Ristretto uses a TinyLFU admission filter, so
  frequently accessed keys are retained while one-hit wonders are evicted
  quickly. This means the effective hit rate is significantly higher than a
  simple LRU for skewed workloads.
- **Singleflight deduplication**: `GetOrSet` deduplicates concurrent loads for
  the same key. If ten goroutines request the same missing key simultaneously,
  only one loader executes; the other nine wait and share the result. This
  prevents the thundering-herd problem on cache misses.
- **Cost = 1 per entry**: Every entry has a uniform cost of 1, so `maxCost`
  directly translates to "maximum number of entries".

### L2 — Redis Cache

The L2 cache (`cache.L2`) uses [go-redis/redis](https://github.com/redis/go-redis).
It is designed as a **fail-soft** layer:

- On read errors (including connection failures), the cache returns a miss
  rather than propagating the error. This prevents a Redis outage from
  cascading into service-wide failures.
- On write errors, the write is silently discarded. The next request will
  simply miss and re-populate.

### Tiered Fallback

When both L1 and L2 are configured, reads follow this path:

```
Request → L1 hit? → return
              ↓ miss
          L2 hit? → populate L1 → return
              ↓ miss
          Loader → populate L1 + L2 → return
```

This means the Redis round-trip is only incurred when the in-process cache
does not already hold the entry, and frequently accessed keys converge into L1
quickly.

**Tradeoff**: Enabling L2 adds a network hop on every L1 miss. If your
workload has a very high L1 hit rate (>95 %), the marginal benefit of L2 is
small. L2 shines in multi-instance deployments where a cache miss in one
process can be served from the shared Redis tier instead of hitting the
underlying data source.

---

## Redis Latency Considerations

Redis performance has a direct impact on tail latency for cache misses. Keep
the following in mind:

- **Network proximity**: Deploy Redis in the same availability zone (ideally
  the same VPC subnet) as your gRPC service. Cross-AZ round-trips add 1–2 ms
  of baseline latency per operation.
- **Connection pooling**: The default `go-redis` client maintains a pool of
  connections. Ensure the pool size is large enough to avoid blocking under
  peak concurrency (see *Production Tuning* below).
- **Serialisation cost**: Values are stored as raw `[]byte`. Avoid caching
  very large payloads (>1 MB) unless the cost of recomputing them justifies
  the network transfer time.
- **Fail-soft behaviour**: Because L2 swallows errors, a slow Redis instance
  manifests as increased loader invocations rather than explicit errors. Monitor
  your Redis latency percentiles (p50, p99) and loader call rate to detect
  degradation early.
- **Pipeline / batching**: The current API operates on single keys. If your
  workload involves bulk lookups, consider a dedicated Redis pipeline outside
  the cache abstraction.

---

## Production Tuning Advice

### RPS Settings

`WithRateLimitGlobal(rps, burst)` configures a token-bucket limiter where
`rps` is the steady-state refill rate (requests per second).

- Set `rps` to approximately **80 %** of your measured sustainable throughput.
  This leaves headroom for organic traffic spikes without triggering rejections
  during normal operation.
- If you use per-group rate limits via the policy resolver, set the global
  limit to the aggregate capacity of all groups combined, plus a margin for
  unclassified traffic.

### Burst Settings

The `burst` parameter controls the maximum number of requests that can be
served instantaneously before the bucket must refill.

- A good starting point is **10–20 %** of `rps`. For example, 500 rps with a
  burst of 50–100.
- Higher burst values absorb short traffic spikes (e.g., client retries after
  a deploy) but increase the peak load your backend must handle.
- Lower burst values provide tighter traffic shaping at the cost of rejecting
  legitimate micro-bursts.

### Cache Sizing

#### L1 (`maxEntries`)

- Ristretto's `NumCounters` is set to `maxEntries * 10` internally, which is
  the recommended ratio for accurate frequency estimation.
- Size L1 to hold your **hot working set**. A cache that is too small evicts
  useful entries and reduces hit rate; a cache that is too large wastes memory
  and increases GC pressure.
- As a rule of thumb, start with 10 000–50 000 entries for a typical
  microservice and adjust based on observed hit-rate metrics.

#### L2 (Redis)

- Use Redis `maxmemory` with an `allkeys-lfu` eviction policy to mirror the
  LFU admission behaviour of L1.
- Size the Redis instance to hold your full warm data set. Unlike L1, Redis is
  shared across all service instances, so its hit rate benefits from the
  combined access pattern.

### Connection Pooling

The `go-redis` client creates a connection pool by default. Key tunables
(configured via `redis.Options` if you instantiate the client directly):

| Parameter     | Default | Recommendation                          |
|---------------|---------|-----------------------------------------|
| `PoolSize`    | 10 × GOMAXPROCS | Set to peak concurrent cache operations. Over-provisioning wastes file descriptors; under-provisioning causes goroutine blocking. |
| `MinIdleConns`| 0       | Set to a small positive value (e.g., 5–10) to avoid cold-start latency after idle periods. |
| `DialTimeout` | 5 s     | Reduce to 1–2 s in low-latency environments so that a hung Redis does not block request processing for too long. |
| `ReadTimeout` | 3 s     | 1 s is usually sufficient for cache reads. |
| `WriteTimeout`| 3 s     | 1 s is usually sufficient for cache writes. |

> **Tip:** Monitor the `go-redis` pool stats (`PoolStats()`) in your metrics
> pipeline. A sustained non-zero `Timeouts` counter signals that the pool is
> undersized.

---

## Benchmarks

> **Placeholder** — This section will be populated with benchmark results in a
> future update. Planned benchmarks include:
>
> - **Interceptor chain overhead**: Nanoseconds per unary call through N
>   interceptors (N = 0, 1, 3, 5).
> - **Rate limiter throughput**: Maximum sustained rps before token exhaustion
>   with varying burst sizes.
> - **L1 cache hit / miss latency**: p50 and p99 for `Get`, `Set`, and
>   `GetOrSet` under concurrent load.
> - **Tiered cache round-trip**: End-to-end latency for L1 miss → L2 hit and
>   L1 miss → L2 miss → loader paths.
> - **IP blocker lookup**: Cost of CIDR matching for block lists of varying
>   sizes (10, 100, 1 000 rules).
>
> To run benchmarks locally once they are available:
>
> ```bash
> go test -bench=. -benchmem ./...
> ```
