package core

import (
	"cmp"
	"slices"

	"google.golang.org/grpc"
)

// middleware represents a single interceptor pair (unary + stream) with a
// deterministic execution order. Lower Order values run first.
type middleware struct {
	Unary  grpc.UnaryServerInterceptor
	Stream grpc.StreamServerInterceptor
	Order  int
}

// MiddlewareBuilder collects middleware entries and produces sorted interceptor
// slices ready for chaining.
type MiddlewareBuilder struct {
	entries []middleware
}

// Add registers a middleware entry with the given order.
// Either interceptor may be nil if only one direction is needed.
func (b *MiddlewareBuilder) Add(order int, unary grpc.UnaryServerInterceptor, stream grpc.StreamServerInterceptor) {
	b.entries = append(b.entries, middleware{
		Unary:  unary,
		Stream: stream,
		Order:  order,
	})
}

// Build sorts the collected middleware by Order (stable) and returns the
// separated unary and stream interceptor slices.
func (b *MiddlewareBuilder) Build() ([]grpc.UnaryServerInterceptor, []grpc.StreamServerInterceptor) {
	slices.SortStableFunc(b.entries, func(a, c middleware) int {
		return cmp.Compare(a.Order, c.Order)
	})

	var unary []grpc.UnaryServerInterceptor
	var stream []grpc.StreamServerInterceptor

	for _, m := range b.entries {
		if m.Unary != nil {
			unary = append(unary, m.Unary)
		}
		if m.Stream != nil {
			stream = append(stream, m.Stream)
		}
	}

	return unary, stream
}
