package interceptors

import (
	"context"

	"github.com/Keksclan/goRawrSquirrel/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// errBlocked is allocated once to avoid per-request allocations on the hot path.
var errBlocked = status.Error(codes.PermissionDenied, "blocked")

// IPBlockUnary returns a unary server interceptor that denies requests when the
// IPBlocker's Evaluate method returns false.
func IPBlockUnary(b *security.IPBlocker) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		if !b.Evaluate(ctx, md) {
			return nil, errBlocked
		}
		return handler(ctx, req)
	}
}

// IPBlockStream returns a stream server interceptor that denies requests when
// the IPBlocker's Evaluate method returns false.
func IPBlockStream(b *security.IPBlocker) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()
		md, _ := metadata.FromIncomingContext(ctx)
		if !b.Evaluate(ctx, md) {
			return errBlocked
		}
		return handler(srv, ss)
	}
}
