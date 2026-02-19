package gorawrsquirrel

import "google.golang.org/grpc"

// config holds the internal configuration assembled via functional options.
type config struct {
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
}
