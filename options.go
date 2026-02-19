package gorawrsquirrel

import (
	"github.com/Keksclan/goRawrSquirrel/interceptors"
	"github.com/Keksclan/goRawrSquirrel/policy"
	"github.com/Keksclan/goRawrSquirrel/security"
	"google.golang.org/grpc"
)

// Middleware order constants. Lower values execute first.
const (
	orderRecovery    = 10
	orderIPBlock     = 20
	orderRequestID   = 30
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
		c.middlewares.Add(orderRequestID, interceptors.RequestIDUnary(), interceptors.RequestIDStream())
	}
}

// WithResolver sets the policy resolver used for method-level policy lookup.
func WithResolver(r *policy.Resolver) Option {
	return func(c *config) {
		c.resolver = r
	}
}

// WithIPBlocker sets the IP blocker and registers the IP-block middleware.
func WithIPBlocker(b *security.IPBlocker) Option {
	return func(c *config) {
		c.ipBlocker = b
		c.middlewares.Add(orderIPBlock, interceptors.IPBlockUnary(b), interceptors.IPBlockStream(b))
	}
}
