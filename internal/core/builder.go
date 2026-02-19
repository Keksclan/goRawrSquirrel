package core

import "google.golang.org/grpc"

// BuildServerOptions translates interceptor slices into grpc.ServerOption
// values that can be passed to grpc.NewServer. This keeps the wiring logic
// isolated from the public API surface.
func BuildServerOptions(
	unary []grpc.UnaryServerInterceptor,
	stream []grpc.StreamServerInterceptor,
	chainUnary func([]grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor,
	chainStream func([]grpc.StreamServerInterceptor) grpc.StreamServerInterceptor,
) []grpc.ServerOption {
	var opts []grpc.ServerOption

	if u := chainUnary(unary); u != nil {
		opts = append(opts, grpc.UnaryInterceptor(u))
	}

	if s := chainStream(stream); s != nil {
		opts = append(opts, grpc.StreamInterceptor(s))
	}

	return opts
}
