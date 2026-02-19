package server

import "google.golang.org/grpc"

// config holds the internal configuration assembled via functional options.
type config struct {
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
}

// Option configures a Server.
type Option func(*config)

// WithUnaryInterceptor appends a unary server interceptor to the chain.
func WithUnaryInterceptor(i grpc.UnaryServerInterceptor) Option {
	return func(c *config) {
		c.unaryInterceptors = append(c.unaryInterceptors, i)
	}
}

// WithStreamInterceptor appends a stream server interceptor to the chain.
func WithStreamInterceptor(i grpc.StreamServerInterceptor) Option {
	return func(c *config) {
		c.streamInterceptors = append(c.streamInterceptors, i)
	}
}
