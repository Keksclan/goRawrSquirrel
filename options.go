package gorawrsquirrel

import (
	"github.com/Keksclan/goRawrSquirrel/interceptors"
	"google.golang.org/grpc"
)

// Middleware order constants. Lower values execute first.
const (
	orderRecovery    = 0
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
	}
}
