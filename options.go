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

// WithIPBlocker sets the IP blocker and registers the IP-block middleware.
func WithIPBlocker(b *security.IPBlocker) Option {
	return func(c *config) {
		c.ipBlocker = b
		c.middlewares.Add(orderIPBlock, interceptors.IPBlockUnary(b), interceptors.IPBlockStream(b))
	}
}

// WithAuth registers an authentication middleware that calls the supplied
// AuthFunc for every request. If the AuthFunc returns an error that is already
// a gRPC status error it is forwarded as-is; otherwise the error is wrapped as
// codes.Unauthenticated.
func WithAuth(fn auth.AuthFunc) Option {
	return func(c *config) {
		c.middlewares.Add(orderAuth, interceptors.AuthUnary(fn), interceptors.AuthStream(fn))
	}
}

// WithRateLimitGlobal enables a global token-bucket rate limiter that rejects
// requests with codes.ResourceExhausted when the limit is exceeded.
func WithRateLimitGlobal(rps float64, burst int) Option {
	return func(c *config) {
		l := ratelimit.NewLimiter(rps, burst)
		c.middlewares.Add(orderRateLimit, interceptors.RateLimitUnary(l), interceptors.RateLimitStream(l))
	}
}

// WithCacheL1 enables an in-process L1 cache backed by ristretto.
// maxEntries controls how many entries the cache can hold.
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

// WithCacheRedis enables a Redis-backed L2 cache. When combined with
// WithCacheL1 the resulting cache checks L1 first, then L2, then the loader.
// If Redis is unavailable at runtime, operations fail soft (no panics).
func WithCacheRedis(addr, password string, db int) Option {
	return func(c *config) {
		c.l2 = cache.NewL2(addr, password, db)
	}
}
