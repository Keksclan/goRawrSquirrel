package interceptors

import (
	"context"

	"google.golang.org/grpc"
)

// ChainUnary composes multiple unary interceptors into a single one.
// Interceptors execute in the order they appear in the slice.
func ChainUnary(interceptors []grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	switch len(interceptors) {
	case 0:
		return nil
	case 1:
		return interceptors[0]
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		curr := handler
		for i := len(interceptors) - 1; i > 0; i-- {
			next := curr
			ic := interceptors[i]
			curr = func(ctx context.Context, req any) (any, error) {
				return ic(ctx, req, info, next)
			}
		}
		return interceptors[0](ctx, req, info, curr)
	}
}

// ChainStream composes multiple stream interceptors into a single one.
// Interceptors execute in the order they appear in the slice.
func ChainStream(interceptors []grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	switch len(interceptors) {
	case 0:
		return nil
	case 1:
		return interceptors[0]
	}

	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		curr := handler
		for i := len(interceptors) - 1; i > 0; i-- {
			next := curr
			ic := interceptors[i]
			curr = func(srv any, ss grpc.ServerStream) error {
				return ic(srv, ss, info, next)
			}
		}
		return interceptors[0](srv, ss, info, curr)
	}
}
