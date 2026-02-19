package interceptors

import (
	"context"

	"github.com/Keksclan/goRawrSquirrel/ratelimit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errRateLimited is allocated once to avoid per-request allocations on the hot path.
var errRateLimited = status.Error(codes.ResourceExhausted, "rate limit exceeded")

// RateLimitUnary returns a unary server interceptor that rejects requests when
// the global rate limiter has been exhausted.
func RateLimitUnary(l *ratelimit.Limiter) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if !l.Allow() {
			return nil, errRateLimited
		}
		return handler(ctx, req)
	}
}

// RateLimitStream returns a stream server interceptor that rejects requests
// when the global rate limiter has been exhausted.
func RateLimitStream(l *ratelimit.Limiter) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if !l.Allow() {
			return errRateLimited
		}
		return handler(srv, ss)
	}
}
