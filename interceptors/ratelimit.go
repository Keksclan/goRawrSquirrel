package interceptors

import (
	"context"
	"sync"

	"github.com/Keksclan/goRawrSquirrel/policy"
	"github.com/Keksclan/goRawrSquirrel/ratelimit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errRateLimited is allocated once to avoid per-request allocations on the hot path.
var errRateLimited = status.Error(codes.ResourceExhausted, "rate limit exceeded")

// rateLimitState holds the global limiter, an optional policy resolver, and a
// cache of per-group limiters created lazily from resolved policies.
type rateLimitState struct {
	global   *ratelimit.Limiter
	resolver *policy.Resolver

	mu     sync.Mutex
	groups map[string]*ratelimit.Limiter
}

// limiterFor returns the per-group limiter when the resolver matches
// fullMethod to a group with a RateLimit policy. Otherwise it returns the
// global limiter.
func (s *rateLimitState) limiterFor(fullMethod string) *ratelimit.Limiter {
	if s.resolver != nil {
		if _, pol, ok := s.resolver.Resolve(fullMethod); ok && pol != nil && pol.RateLimit != nil {
			return s.groupLimiter(fullMethod, pol.RateLimit)
		}
	}
	return s.global
}

// groupLimiter returns (or lazily creates) a per-group limiter keyed by the
// resolved group name.
func (s *rateLimitState) groupLimiter(fullMethod string, rl *policy.RateLimitRule) *ratelimit.Limiter {
	// Resolve again to get the group name (cheap — no allocations).
	name, _, _ := s.resolver.Resolve(fullMethod)

	s.mu.Lock()
	defer s.mu.Unlock()

	if l, ok := s.groups[name]; ok {
		return l
	}
	l := ratelimit.NewLimiter(float64(rl.Rate)/rl.Window.Seconds(), rl.Rate)
	s.groups[name] = l
	return l
}

// RateLimitUnary returns a unary server interceptor that rejects requests when
// the applicable rate limiter has been exhausted. When a policy resolver is
// provided and the method matches a group with a RateLimit rule, that
// per-group limiter is used; otherwise the global limiter applies.
func RateLimitUnary(l *ratelimit.Limiter, r *policy.Resolver) grpc.UnaryServerInterceptor {
	st := &rateLimitState{global: l, resolver: r, groups: make(map[string]*ratelimit.Limiter)}
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if !st.limiterFor(info.FullMethod).Allow() {
			return nil, errRateLimited
		}
		return handler(ctx, req)
	}
}

// RateLimitStream returns a stream server interceptor that rejects requests
// when the applicable rate limiter has been exhausted.
func RateLimitStream(l *ratelimit.Limiter, r *policy.Resolver) grpc.StreamServerInterceptor {
	st := &rateLimitState{global: l, resolver: r, groups: make(map[string]*ratelimit.Limiter)}
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if !st.limiterFor(info.FullMethod).Allow() {
			return errRateLimited
		}
		return handler(srv, ss)
	}
}
