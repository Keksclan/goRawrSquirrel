// Package gorawrsquirrel provides minimal, composable middleware primitives
// that can be adapted for gRPC interceptors without imposing a framework.
package gorawrsquirrel

import "context"

// HandlerFunc is the minimal unit of work that middlewares wrap.
type HandlerFunc func(ctx context.Context) error

// Middleware transforms a HandlerFunc, allowing pre/post behavior composition.
type Middleware func(HandlerFunc) HandlerFunc

// Chain composes middlewares from left to right, i.e., Chain(A, B)(h) => A(B(h)).
func Chain(mw ...Middleware) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		for i := len(mw) - 1; i >= 0; i-- {
			next = mw[i](next)
		}
		return next
	}
}

// Wrap applies the middleware chain to a handler and returns the wrapped handler.
func Wrap(h HandlerFunc, mw ...Middleware) HandlerFunc {
	if len(mw) == 0 {
		return h
	}
	return Chain(mw...)(h)
}
