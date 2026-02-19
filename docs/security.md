# Security

This document describes the security model of goRawrSquirrel, what the library
handles, what it delegates to the caller, and how to deploy it safely.

---

## 1. Authentication Responsibilities

goRawrSquirrel does **not** implement any authentication protocol. It provides
an integration point — `auth.AuthFunc` — and the middleware plumbing that calls
it on every request.

| Concern | Owner |
|---|---|
| Invoking the callback at the correct point in the chain (priority 28) | Library |
| Wrapping non-gRPC errors as `codes.Unauthenticated` | Library |
| Token parsing, signature verification, expiry checks | **You** |
| Populating the context with identity/claims | **You** |
| Session management, token refresh, revocation | **You** |

The `AuthFunc` receives the request context, the full gRPC method name, and the
incoming metadata. Return a non-nil error to reject the call; return an
enriched context to propagate identity downstream.

```go
type AuthFunc func(ctx context.Context, fullMethod string, md metadata.MD) (context.Context, error)
```

There is no built-in token format requirement. JWT, mTLS-derived identities,
opaque tokens, or any other scheme can be used — the library is agnostic.

---

## 2. IP Hard-Block Trust Model

`security.IPBlocker` operates in one of two modes:

- **AllowList** — only IPs matching at least one configured CIDR are permitted.
- **DenyList** — IPs matching any configured CIDR are rejected; all others pass.

Evaluation is binary: match or no match. There is no scoring, no partial
trust, and no logging-only mode. If the client IP cannot be determined (e.g.
missing peer info), the request is **denied** regardless of mode.

CIDRs and trusted-proxy ranges are parsed once at construction time via
`netip.ParsePrefix`. A bare IP address (no prefix length) is treated as a
single-host prefix (`/32` for IPv4, `/128` for IPv6).

The IP block middleware runs at priority **20**, before rate limiting (25) and
authentication (28). Blocked requests are rejected early and consume no
downstream resources.

---

## 3. Trusted Proxies

When a reverse proxy or load balancer sits in front of the gRPC server, the
TCP peer address belongs to the proxy, not the client. goRawrSquirrel resolves
the real client address as follows:

1. Extract the peer address from the gRPC context.
2. If the peer address falls within any configured `TrustedProxies` CIDR, read
   the first valid IP from the metadata headers listed in `HeaderPriority`.
3. If no trusted proxy matches, or no valid header IP is found, use the peer
   address as-is.

The default header priority is:

```
x-real-ip → x-forwarded-for
```

For `X-Forwarded-For`, the **left-most** (first) entry is used, which
represents the original client according to the conventional append order.

**Key points:**

- Only headers from connections whose peer address is in `TrustedProxies` are
  consulted. An untrusted peer cannot influence IP resolution via headers.
- If `TrustedProxies` is empty, headers are never inspected — the peer address
  is always used directly.
- Configure `TrustedProxies` to match exactly the network ranges of your
  proxies. Overly broad ranges (e.g. `0.0.0.0/0`) defeat the purpose of the
  trust check.

---

## 4. Metadata Spoofing Risks

gRPC metadata is caller-controlled. Any client can send arbitrary key-value
pairs, including `authorization`, `x-forwarded-for`, and `x-real-ip`.

The library mitigates header-based IP spoofing through the trusted-proxy gate
described in section 3: forwarded-address headers are only honored when the
direct peer is in the `TrustedProxies` set. Without this, a client could
inject a fabricated IP to bypass allow/deny lists.

Metadata used in your `AuthFunc` is equally untrusted input. Standard
precautions apply:

- Validate and sanitize all metadata values before use.
- Do not trust metadata-derived identity without cryptographic verification
  (e.g. signature checks on bearer tokens).
- Treat the `fullMethod` string as routing information, not as a security
  assertion — it reflects the called method, not caller privilege.

---

## 5. Rate Limiting as DoS Mitigation

`WithRateLimitGlobal(rps, burst)` installs a token-bucket limiter at priority
**25** (after IP blocking, before auth). Requests that exceed the limit receive
`codes.ResourceExhausted`.

When a `policy.Resolver` is configured, methods matching a group with a
`RateLimit` rule use the per-group limit instead of the global one. This allows
differentiated throughput for different API surfaces.

**What it does:**

- Caps the sustained request rate server-wide (or per-group).
- Sheds excess load before it reaches authentication or business logic.

**What it does not do:**

- Per-client or per-IP rate limiting. The built-in limiter is global. If you
  need per-caller limits, implement them in a custom interceptor.
- Distributed rate limiting. The token bucket is in-process. Multiple server
  instances each enforce their own independent limit.
- Protection against application-layer attacks that stay under the rate
  threshold. Rate limiting is one layer of defense, not a complete solution.

Set `rps` and `burst` based on your service's measured capacity. The burst
parameter controls the size of the token bucket and therefore how many requests
can pass in a short spike before steady-state enforcement kicks in.

---

## 6. Secure Deployment Recommendations

### Transport encryption

goRawrSquirrel produces a standard `*grpc.Server`. Configure TLS credentials
through the normal `grpc.Creds()` server option. The library does not manage
certificates.

### Proxy configuration

If running behind a load balancer or reverse proxy:

- Set `TrustedProxies` to the exact CIDR(s) of your proxy tier.
- Ensure your proxy strips or overwrites `X-Forwarded-For` / `X-Real-IP`
  before appending its own values, so upstream hops cannot inject addresses.
- If the proxy terminates TLS, ensure the internal link between proxy and
  server is secured (private network, mTLS, or equivalent).

### Listener binding

Bind the gRPC listener to the intended interface. In production this is
typically `0.0.0.0:<port>` behind a load balancer, or `127.0.0.1:<port>` when
co-located with a sidecar proxy. Avoid exposing management or debug listeners
on public interfaces.

### Middleware ordering

goRawrSquirrel enforces a fixed middleware priority:

| Priority | Middleware |
|---|---|
| 10 | Recovery |
| 20 | IP Block |
| 25 | Rate Limit |
| 28 | Authentication |
| 30 | Request ID |
| 100 | Custom interceptors |

This order ensures that panics are always caught (recovery first), blocked IPs
are rejected before consuming rate-limit tokens, and unauthenticated requests
are dropped before reaching business logic. Do not attempt to work around this
ordering.

### Secrets and configuration

- Do not embed credentials or CIDR lists in source code. Use environment
  variables, secret managers, or configuration files excluded from version
  control.
- Redis passwords for the L2 cache should come from a secrets provider, not
  from hard-coded strings.

---

## 7. Production Checklist

| # | Item | Notes |
|---|---|---|
| 1 | TLS enabled | Configure `grpc.Creds()` with valid certificates. |
| 2 | `WithRecovery()` enabled | Prevents panics from crashing the process. Also installs request-ID injection for traceability. |
| 3 | `AuthFunc` validates cryptographically | Token signature and expiry must be checked — the library only calls your function. |
| 4 | `TrustedProxies` scoped tightly | List only the actual proxy/load-balancer CIDRs. Never use `0.0.0.0/0`. |
| 5 | Proxy strips incoming forwarded headers | Prevents external clients from injecting `X-Forwarded-For` values. |
| 6 | Rate limit configured | Set `rps` and `burst` based on measured capacity. |
| 7 | IP block list reviewed | Use AllowList for internal services; DenyList for targeted blocks. |
| 8 | Redis password from secrets manager | Do not pass the L2 cache password as a literal in code. |
| 9 | Listener bound to correct interface | `127.0.0.1` for sidecar setups; `0.0.0.0` only behind a load balancer. |
| 10 | Monitoring in place | Export Prometheus metrics and alert on `ResourceExhausted` spikes and elevated error rates. |
