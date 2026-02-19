package core

import "google.golang.org/grpc"

// InterceptorRegistry collects unary and stream interceptors that will later
// be chained and applied to a gRPC server. It provides a single place to
// accumulate interceptors before server construction.
type InterceptorRegistry struct {
	Unary  []grpc.UnaryServerInterceptor
	Stream []grpc.StreamServerInterceptor
}

// AddUnary appends a unary server interceptor to the registry.
func (r *InterceptorRegistry) AddUnary(i grpc.UnaryServerInterceptor) {
	r.Unary = append(r.Unary, i)
}

// AddStream appends a stream server interceptor to the registry.
func (r *InterceptorRegistry) AddStream(i grpc.StreamServerInterceptor) {
	r.Stream = append(r.Stream, i)
}

// PrependUnary inserts a unary interceptor at the front of the chain.
func (r *InterceptorRegistry) PrependUnary(i grpc.UnaryServerInterceptor) {
	r.Unary = append([]grpc.UnaryServerInterceptor{i}, r.Unary...)
}

// PrependStream inserts a stream interceptor at the front of the chain.
func (r *InterceptorRegistry) PrependStream(i grpc.StreamServerInterceptor) {
	r.Stream = append([]grpc.StreamServerInterceptor{i}, r.Stream...)
}
