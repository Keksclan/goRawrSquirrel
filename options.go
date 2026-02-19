package gorawrsquirrel

import (
	"github.com/Keksclan/goRawrSquirrel/interceptors"
	"google.golang.org/grpc"
)

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

// WithRecovery prepends panic-recovery interceptors to the unary and stream
// chains so that a panic inside a handler returns codes.Internal instead of
// crashing the process.
func WithRecovery() Option {
	return func(c *config) {
		c.unaryInterceptors = append([]grpc.UnaryServerInterceptor{interceptors.RecoveryUnary()}, c.unaryInterceptors...)
		c.streamInterceptors = append([]grpc.StreamServerInterceptor{interceptors.RecoveryStream()}, c.streamInterceptors...)
	}
}
