package gorawrsquirrel

import (
	"github.com/Keksclan/goRawrSquirrel/auth"
	"github.com/Keksclan/goRawrSquirrel/cache"
	"github.com/Keksclan/goRawrSquirrel/interceptors"
	"github.com/Keksclan/goRawrSquirrel/policy"
	"github.com/Keksclan/goRawrSquirrel/ratelimit"
	"github.com/Keksclan/goRawrSquirrel/security"
	"google.golang.org/grpc"
)

// Middleware order constants. Lower values execute first.
const (
	orderRecovery    = 10
	orderIPBlock     = 20
	orderRateLimit   = 25
	orderAuth        = 28
	orderRequestID   = 30
	orderInterceptor = 100
)

// Option configures a Server.
type Option func(*config)

// WithUnaryInterceptor appends a unary server interceptor to the chain.
func WithUnaryInterceptor(i grpc.UnaryServerInterceptor) Option {
	return func(c *config) {
		c.middlewares.Add(orderInterceptor, i, nil)
	}
}

// WithStreamInterceptor appends a stream server interceptor to the chain.
func WithStreamInterceptor(i grpc.StreamServerInterceptor) Option {
	return func(c *config) {
		c.middlewares.Add(orderInterceptor, nil, i)
	}
}

// WithRecovery prepends panic-recovery interceptors to the unary and stream
// chains so that a panic inside a handler returns codes.Internal instead of
// crashing the process.
func WithRecovery() Option {
	return func(c *config) {
		c.middlewares.Add(orderRecovery, interceptors.RecoveryUnary(), interceptors.RecoveryStream())
		c.middlewares.Add(orderRequestID, interceptors.RequestIDUnary(), interceptors.RequestIDStream())
	}
}

// WithResolver sets the policy resolver used for method-level policy lookup.
func WithResolver(r *policy.Resolver) Option {
	return func(c *config) {
		c.resolver = r
	}
}

// WithIPBlocker registers an IP-blocking middleware that uses b to decide
// whether an incoming request should be rejected based on its peer address.
// Blocked requests receive codes.PermissionDenied.
//
// Example:
//
//	blocker := security.NewIPBlocker(denyList)
//	gs.NewServer(gs.WithIPBlocker(blocker))
func WithIPBlocker(b *security.IPBlocker) Option {
	return func(c *config) {
		c.ipBlocker = b
		c.middlewares.Add(orderIPBlock, interceptors.IPBlockUnary(b), interceptors.IPBlockStream(b))
	}
}

// WithAuth registers an authentication middleware that invokes fn for every
// incoming unary and stream request. fn receives the request context, the
// fully-qualified gRPC method name, and the incoming metadata; it must return
// an (optionally enriched) context or an error to reject the call.
//
// If fn returns an error that is already a gRPC status error it is forwarded
// as-is; otherwise the error is wrapped as codes.Unauthenticated.
//
// Example:
//
//	gs.WithAuth(func(ctx context.Context, method string, md metadata.MD) (context.Context, error) {
//		if len(md.Get("authorization")) == 0 {
//			return ctx, status.Error(codes.Unauthenticated, "missing token")
//		}
//		return ctx, nil
//	})
func WithAuth(fn auth.AuthFunc) Option {
	return func(c *config) {
		c.middlewares.Add(orderAuth, interceptors.AuthUnary(fn), interceptors.AuthStream(fn))
	}
}

// WithRateLimitGlobal enables a global token-bucket rate limiter. Requests
// that exceed the limit are rejected with codes.ResourceExhausted. rps sets
// the sustained requests-per-second rate and burst sets the maximum number of
// requests allowed in a single burst.
//
// When a [policy.Resolver] has been configured via [WithResolver] and a method
// matches a group with a RateLimit rule, the per-group limit is used instead
// of the global one.
//
// Example:
//
//	// Allow 500 sustained req/s with bursts up to 100.
//	gs.WithRateLimitGlobal(500, 100)
func WithRateLimitGlobal(rps float64, burst int) Option {
	return func(c *config) {
		l := ratelimit.NewLimiter(rps, burst)
		c.middlewares.Add(orderRateLimit,
			interceptors.RateLimitUnary(l, c.resolver),
			interceptors.RateLimitStream(l, c.resolver),
		)
	}
}

// WithCacheL1 enables an in-process L1 cache backed by ristretto. maxEntries
// controls the approximate upper bound on the number of entries the cache can
// hold. The resulting [cache.Cache] is accessible via [Server.Cache].
//
// When combined with [WithCacheRedis] the two layers are merged into a tiered
// cache that checks L1 first, then Redis (L2), then the loader.
//
// WithCacheL1 panics if the underlying ristretto cache cannot be created.
//
// Example:
//
//	srv := gs.NewServer(gs.WithCacheL1(10_000))
//	srv.Cache().Set(ctx, "key", value, time.Minute)
func WithCacheL1(maxEntries int) Option {
	return func(c *config) {
		l1, err := cache.NewL1(int64(maxEntries))
		if err != nil {
			panic("gorawrsquirrel: failed to create L1 cache: " + err.Error())
		}
		c.l1 = l1
		c.cache = l1
	}
}

// WithCacheRedis enables a Redis-backed L2 cache. addr is the Redis server
// address (e.g. "localhost:6379"), password is the AUTH password (use "" for
// none), and db selects the Redis database index.
//
// When combined with [WithCacheL1] the resulting cache checks L1 first, then
// Redis (L2), then the loader. If Redis is unavailable at runtime, operations
// fail soft (no panics).
//
// Example:
//
//	gs.NewServer(
//		gs.WithCacheL1(10_000),
//		gs.WithCacheRedis("localhost:6379", "", 0),
//	)
func WithCacheRedis(addr, password string, db int) Option {
	return func(c *config) {
		c.l2 = cache.NewL2(addr, password, db)
	}
}
