package interceptors

import (
	"context"

	"github.com/Keksclan/goRawrSquirrel/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// errUnauthenticated is allocated once to avoid per-request allocations on the hot path.
var errUnauthenticated = status.Error(codes.Unauthenticated, "unauthenticated")

// authError returns the original error if it is already a gRPC status error,
// otherwise wraps it as codes.Unauthenticated.
func authError(err error) error {
	if _, ok := status.FromError(err); ok {
		return err
	}
	return errUnauthenticated
}

// AuthUnary returns a unary server interceptor that calls the supplied
// AuthFunc before forwarding to the handler.
func AuthUnary(fn auth.AuthFunc) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		newCtx, err := fn(ctx, info.FullMethod, md)
		if err != nil {
			return nil, authError(err)
		}
		return handler(newCtx, req)
	}
}

// AuthStream returns a stream server interceptor that calls the supplied
// AuthFunc before forwarding to the handler.
func AuthStream(fn auth.AuthFunc) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()
		md, _ := metadata.FromIncomingContext(ctx)
		_, err := fn(ctx, info.FullMethod, md)
		if err != nil {
			return authError(err)
		}
		return handler(srv, ss)
	}
}
