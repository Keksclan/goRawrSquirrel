# goRawrSquirrel Usage Guide

Comprehensive usage documentation for the **goRawrSquirrel** gRPC middleware
toolkit.

---

## Table of Contents

1. [Server Creation](#1-server-creation)
2. [Functional Options](#2-functional-options)
3. [Rate Limiting](#3-rate-limiting)
4. [Authentication Hook](#4-authentication-hook)
5. [Caching](#5-caching)
6. [Method Groups](#6-method-groups)
7. [IP Hardblock](#7-ip-hardblock)

---

## 1. Server Creation

`goRawrSquirrel` wraps the standard `*grpc.Server` and configures it through
functional options passed to `NewServer`. The returned `*Server` exposes the
underlying gRPC server via `GRPC()`, so you retain full control over service
registration, the listener, and the server lifecycle.

### Minimal Server

```go
package main

import (
	"log"
	"net"

	gs "github.com/Keksclan/goRawrSquirrel"
)

func main() {
	srv := gs.NewServer(
		gs.WithRecovery(), // panic recovery + request-ID injection
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

### Using Default Options

`DefaultOptions()` returns the recommended production defaults (currently
`WithRecovery()`). You can append additional options on top:

```go
opts := append(gs.DefaultOptions(),
	gs.WithRateLimitGlobal(500, 100),
	gs.WithCacheL1(10_000),
)
srv := gs.NewServer(opts...)
```

### Prometheus Metrics

Every `Server` exposes a Prometheus HTTP handler:

```go
import "net/http"

go func() {
	http.Handle("/metrics", srv.MetricsHandler())
	log.Fatal(http.ListenAndServe(":9090", nil))
}()
```

---

## 2. Functional Options

All server behaviour is configured through `Option` values passed to
`NewServer`. Each option targets a single concern and registers its middleware
at a **fixed priority level**, guaranteeing a deterministic execution order
regardless of how options are arranged in the call.

| Priority | Option                 | Description                               |
|----------|------------------------|-------------------------------------------|
| 10       | `WithRecovery()`       | Panic recovery + request-ID injection     |
| 20       | `WithIPBlocker(b)`     | IP allow/deny list enforcement            |
| 25       | `WithRateLimitGlobal()`| Token-bucket rate limiting                |
| 28       | `WithAuth(fn)`         | Pluggable authentication callback         |
| 30       | *(request-ID)*         | Injected automatically by `WithRecovery`  |
| 100      | `WithUnaryInterceptor` / `WithStreamInterceptor` | Custom interceptors |

Lower numbers execute first. Recovery always runs outermost so that panics in
any downstream middleware are caught.

### Available Options

| Function | Purpose |
|---|---|
| `WithRecovery()` | Adds panic-recovery and per-request ID interceptors (unary + stream). |
| `WithRateLimitGlobal(rps, burst)` | Enables a global token-bucket rate limiter. |
| `WithAuth(fn)` | Registers an `auth.AuthFunc` authentication middleware. |
| `WithCacheL1(maxEntries)` | Enables an in-process L1 cache backed by ristretto. |
| `WithCacheRedis(addr, password, db)` | Enables a Redis-backed L2 cache. When combined with L1, creates a tiered cache. |
| `WithIPBlocker(b)` | Registers an IP allow/deny-list middleware. |
| `WithResolver(r)` | Sets the policy resolver used for method-level policy lookup (e.g., per-group rate limits). |
| `WithUnaryInterceptor(i)` | Appends a custom unary server interceptor. |
| `WithStreamInterceptor(i)` | Appends a custom stream server interceptor. |

### Composing Options

Options compose naturally. Pass as many as you need:

```go
srv := gs.NewServer(
	gs.WithRecovery(),
	gs.WithRateLimitGlobal(1000, 200),
	gs.WithAuth(myAuthFunc),
	gs.WithCacheL1(50_000),
	gs.WithCacheRedis("localhost:6379", "", 0),
	gs.WithIPBlocker(blocker),
	gs.WithResolver(resolver),
)
```

---

## 3. Rate Limiting

goRawrSquirrel ships a token-bucket rate limiter that rejects excess requests
with `codes.ResourceExhausted`.

### 3.1 Global Rate Limit

Apply a single limit to **all** incoming requests:

```go
srv := gs.NewServer(
	gs.WithRecovery(),
	// Allow 500 requests/second with bursts up to 100.
	gs.WithRateLimitGlobal(500, 100),
)
```

### 3.2 Per-Group Rate Limit

When a `policy.Resolver` is configured, methods that match a group use the
**group's** rate-limit rule instead of the global one.

```go
package main

import (
	"log"
	"net"
	"time"

	gs "github.com/Keksclan/goRawrSquirrel"
	"github.com/Keksclan/goRawrSquirrel/policy"
)

func main() {
	resolver := policy.NewResolver(
		// Expensive analytics endpoints: 10 req / 30s.
		policy.Group("analytics").
			Prefix("/myapp.Analytics/").
			Policy(policy.Policy{
				RateLimit: &policy.RateLimitRule{
					Rate:   10,
					Window: 30 * time.Second,
				},
			}),

		// Health check: generous limit.
		policy.Group("health").
			Exact("/grpc.health.v1.Health/Check").
			Policy(policy.Policy{
				RateLimit: &policy.RateLimitRule{
					Rate:   1000,
					Window: 1 * time.Second,
				},
			}),
	)

	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithResolver(resolver),
		// Global fallback: 500 req/s, burst 100.
		gs.WithRateLimitGlobal(500, 100),
	)

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

> **Note:** If a method matches a group that has a `RateLimit` rule, that
> per-group limit is applied. All other methods fall back to the global limit.

---

## 4. Authentication Hook

### 4.1 Implementing a Custom `AuthFunc`

The `auth.AuthFunc` type is a callback invoked for every request:

```go
type AuthFunc func(ctx context.Context, fullMethod string, md metadata.MD) (context.Context, error)
```

- **`ctx`** — the incoming request context.
- **`fullMethod`** — the full gRPC method name (e.g. `"/myapp.Users/Get"`).
- **`md`** — the incoming gRPC metadata (headers).
- **Return** an enriched context on success, or an error to reject the request.

If the returned error is already a gRPC status error it is forwarded as-is;
otherwise it is automatically wrapped as `codes.Unauthenticated`.

#### Example: JWT Validation

```go
package main

import (
	"context"
	"errors"
	"log"
	"net"

	gs "github.com/Keksclan/goRawrSquirrel"
	"github.com/Keksclan/goRawrSquirrel/contextx"
	"google.golang.org/grpc/metadata"
)

// myAuthFunc validates a bearer token and enriches the context with an Actor.
func myAuthFunc(ctx context.Context, _ string, md metadata.MD) (context.Context, error) {
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return ctx, errors.New("missing authorization header")
	}

	token := vals[0] // e.g. "Bearer eyJhbGci..."

	// Your custom token validation logic here.
	claims, err := validateJWT(token)
	if err != nil {
		return ctx, err
	}

	// Inject the authenticated actor into the context.
	return contextx.WithActor(ctx, contextx.Actor{
		Subject:  claims.Subject,
		Tenant:   claims.Tenant,
		ClientID: claims.ClientID,
		Scopes:   claims.Scopes,
	}), nil
}

func main() {
	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithAuth(myAuthFunc),
	)

	// pb.RegisterMyServiceServer(srv.GRPC(), &myService{})

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	if err := srv.GRPC().Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
```

### 4.2 Injecting and Retrieving the Actor

The `contextx` package provides `Actor`, `WithActor`, and `ActorFromContext`
to carry caller identity through the request lifecycle.

#### Actor Struct

```go
type Actor struct {
	Subject  string   // Who the caller is (e.g. user ID, email).
	Tenant   string   // Multi-tenant identifier.
	ClientID string   // OAuth2 client ID.
	Scopes   []string // Granted permission scopes.
}
```

#### Storing the Actor (inside your AuthFunc)

```go
ctx = contextx.WithActor(ctx, contextx.Actor{
	Subject: "alice",
	Tenant:  "acme",
	Scopes:  []string{"read", "write"},
})
```

#### Reading the Actor (inside a handler)

```go
func (s *myService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	actor, ok := contextx.ActorFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "no actor in context")
	}
	log.Printf("request by %s (tenant=%s)", actor.Subject, actor.Tenant)
	// ...
}
```

---

## 5. Caching

goRawrSquirrel provides a `cache.Cache` interface with three operations:

```go
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error
	GetOrSet(ctx context.Context, key string, ttl time.Duration,
		loader func(context.Context) ([]byte, error)) ([]byte, error)
}
```

The configured cache is accessible via `srv.Cache()`.

### 5.1 L1 — In-Process Cache

L1 is an in-process cache backed by [ristretto](https://github.com/dgraph-io/ristretto).
It deduplicates concurrent loads for the same key (singleflight behaviour).

```go
srv := gs.NewServer(
	gs.WithRecovery(),
	gs.WithCacheL1(10_000), // hold up to 10 000 entries
)

c := srv.Cache() // cache.Cache backed by L1
```

#### Direct L1 Get / Set

```go
ctx := context.Background()

// Store a value with a 5-minute TTL.
err := c.Set(ctx, "user:42", []byte(`{"name":"Alice"}`), 5*time.Minute)

// Retrieve the value.
val, hit, err := c.Get(ctx, "user:42")
if hit {
	fmt.Println("cached:", string(val))
}
```

### 5.2 L2 — Redis Cache

Enable a Redis-backed L2 cache with `WithCacheRedis`. All Redis operations
**fail soft** — connection errors are treated as cache misses, never panics.

```go
srv := gs.NewServer(
	gs.WithRecovery(),
	gs.WithCacheRedis("localhost:6379", "", 0),
)
```

### 5.3 Tiered Cache (L1 + L2)

When **both** L1 and L2 are configured, goRawrSquirrel automatically creates a
**tiered cache** that checks L1 first, then L2, then the loader. Writes
populate both layers. On an L2 hit the value is promoted into L1.

```go
srv := gs.NewServer(
	gs.WithRecovery(),
	gs.WithCacheL1(50_000),                        // L1 in-process
	gs.WithCacheRedis("localhost:6379", "", 0),     // L2 Redis
)

c := srv.Cache() // Tiered: L1 → L2 → loader
```

### 5.4 `GetOrSet` Example

`GetOrSet` is the recommended read-through pattern. On a cache miss it calls
the loader **exactly once** (concurrent callers for the same key are
deduplicated), stores the result, and returns it.

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	gs "github.com/Keksclan/goRawrSquirrel"
)

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func main() {
	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithCacheL1(10_000),
		gs.WithCacheRedis("localhost:6379", "", 0),
	)

	c := srv.Cache()
	ctx := context.Background()

	// GetOrSet: returns cached data or calls the loader on a miss.
	data, err := c.GetOrSet(ctx, "user:42", 5*time.Minute, func(ctx context.Context) ([]byte, error) {
		// This loader runs at most once per cache miss (concurrent callers
		// for the same key are deduplicated).
		fmt.Println("loader: fetching user 42 from database")
		user := User{ID: 42, Name: "Alice"}
		return json.Marshal(user)
	})
	if err != nil {
		log.Fatalf("cache error: %v", err)
	}

	var user User
	_ = json.Unmarshal(data, &user)
	fmt.Printf("user: %+v\n", user)
}
```

---

## 6. Method Groups

The `policy` package lets you define **method groups** with matching rules and
attach policies (rate limits, timeouts, auth requirements) to each group.
A `Resolver` evaluates the full gRPC method name against all groups and returns
the best match.

**Priority:** Exact > Prefix > Regex. Among matches of the same kind, the
longer match wins. Equal-length matches are resolved by registration order.

### 6.1 Exact Match

Matches the full method name exactly:

```go
policy.Group("health").
	Exact("/grpc.health.v1.Health/Check").
	Policy(policy.Policy{
		RateLimit: &policy.RateLimitRule{Rate: 1000, Window: time.Second},
	})
```

### 6.2 Prefix Match

Matches any method whose name starts with the given prefix:

```go
policy.Group("admin").
	Prefix("/myapp.Admin/").
	Policy(policy.Policy{
		AuthRequired: true,
		Timeout:      2 * time.Second,
	})
```

### 6.3 Regex Match

Matches any method whose name satisfies the regular expression. The pattern is
compiled at build time — an invalid regex will panic.

```go
policy.Group("streaming").
	Regex(`^/myapp\.\w+/Stream`).
	Policy(policy.Policy{
		RateLimit: &policy.RateLimitRule{Rate: 50, Window: 10 * time.Second},
	})
```

### Full Example — Combining Groups

```go
package main

import (
	"log"
	"net"
	"time"

	gs "github.com/Keksclan/goRawrSquirrel"
	"github.com/Keksclan/goRawrSquirrel/policy"
)

func main() {
	resolver := policy.NewResolver(
		// Exact: health endpoint with a generous limit.
		policy.Group("health").
			Exact("/grpc.health.v1.Health/Check").
			Policy(policy.Policy{
				RateLimit: &policy.RateLimitRule{Rate: 1000, Window: time.Second},
			}),

		// Prefix: all admin methods require auth and have a short timeout.
		policy.Group("admin").
			Prefix("/myapp.Admin/").
			Policy(policy.Policy{
				AuthRequired: true,
				Timeout:      2 * time.Second,
			}),

		// Regex: streaming methods are rate-limited more tightly.
		policy.Group("streaming").
			Regex(`^/myapp\.\w+/Stream`).
			Policy(policy.Policy{
				RateLimit: &policy.RateLimitRule{Rate: 50, Window: 10 * time.Second},
			}),
	)

	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithResolver(resolver),
		gs.WithRateLimitGlobal(500, 100),
	)

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

---

## 7. IP Hardblock

The `security` package provides `IPBlocker` — a middleware that allows or
denies requests based on the client IP address. It supports CIDR ranges,
individual IPs, and trusted-proxy awareness for extracting the real client IP.

### 7.1 DenyList

Block specific IPs or CIDR ranges; all other traffic is allowed.

```go
package main

import (
	"log"
	"net"

	gs "github.com/Keksclan/goRawrSquirrel"
	"github.com/Keksclan/goRawrSquirrel/security"
)

func main() {
	blocker, err := security.NewIPBlocker(security.Config{
		Mode: security.DenyList,
		CIDRs: []string{
			"192.168.1.0/24", // block entire subnet
			"10.0.0.5",       // block single IP (treated as /32)
		},
	})
	if err != nil {
		log.Fatalf("ipblocker: %v", err)
	}

	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithIPBlocker(blocker),
	)

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

### 7.2 AllowList

Only permit traffic from explicitly listed IPs or CIDR ranges; reject
everything else.

```go
blocker, err := security.NewIPBlocker(security.Config{
	Mode: security.AllowList,
	CIDRs: []string{
		"10.0.0.0/8",        // internal network
		"203.0.113.0/24",    // partner subnet
	},
})
if err != nil {
	log.Fatalf("ipblocker: %v", err)
}

srv := gs.NewServer(
	gs.WithRecovery(),
	gs.WithIPBlocker(blocker),
)
```

### 7.3 Trusted Proxies

When your gRPC server sits behind a load balancer or reverse proxy, the direct
peer address is the proxy — not the real client. `TrustedProxies` tells the
`IPBlocker` which proxy IPs to trust, so it can extract the real client IP from
forwarding headers (e.g. `x-forwarded-for`, `x-real-ip`).

```go
blocker, err := security.NewIPBlocker(security.Config{
	Mode: security.DenyList,
	CIDRs: []string{
		"203.0.113.50", // blocked attacker
	},
	TrustedProxies: []string{
		"10.0.0.1/32",     // load balancer
		"172.16.0.0/12",   // internal proxy range
	},
	// Optional: control which headers are checked (in priority order).
	// Defaults to a sensible list when omitted.
	HeaderPriority: []string{"x-forwarded-for", "x-real-ip"},
})
if err != nil {
	log.Fatalf("ipblocker: %v", err)
}

srv := gs.NewServer(
	gs.WithRecovery(),
	gs.WithIPBlocker(blocker),
)
```

### Full IP Blocking Example

```go
package main

import (
	"log"
	"net"

	gs "github.com/Keksclan/goRawrSquirrel"
	"github.com/Keksclan/goRawrSquirrel/security"
)

func main() {
	blocker, err := security.NewIPBlocker(security.Config{
		Mode: security.AllowList,
		CIDRs: []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
		},
		TrustedProxies: []string{
			"10.0.0.1/32",
		},
	})
	if err != nil {
		log.Fatalf("ipblocker: %v", err)
	}

	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithIPBlocker(blocker),
		gs.WithRateLimitGlobal(500, 100),
	)

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

---

## Quick Reference

```go
import (
	gs       "github.com/Keksclan/goRawrSquirrel"
	"github.com/Keksclan/goRawrSquirrel/auth"
	"github.com/Keksclan/goRawrSquirrel/cache"
	"github.com/Keksclan/goRawrSquirrel/contextx"
	"github.com/Keksclan/goRawrSquirrel/policy"
	"github.com/Keksclan/goRawrSquirrel/security"
)
```

| Package | Key Types / Functions |
|---|---|
| `gorawrsquirrel` | `NewServer`, `DefaultOptions`, `Option`, `Server` |
| `auth` | `AuthFunc` |
| `cache` | `Cache` (interface), `L1`, `L2`, `Tiered` |
| `contextx` | `Actor`, `WithActor`, `ActorFromContext` |
| `policy` | `Group`, `GroupBuilder`, `Resolver`, `NewResolver`, `Policy`, `RateLimitRule` |
| `security` | `IPBlocker`, `NewIPBlocker`, `Config`, `AllowList`, `DenyList` |
