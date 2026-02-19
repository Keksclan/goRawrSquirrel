package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// Server is a minimal wrapper around a gRPC server with optional metrics.
type Server struct {
	grpcServer *grpc.Server
}

// NewServer creates a Server by applying functional options and wiring the
// resulting interceptor chains into grpc.NewServer.
func NewServer(opts ...Option) *Server {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}

	var serverOpts []grpc.ServerOption

	if u := chainUnary(cfg.unaryInterceptors); u != nil {
		serverOpts = append(serverOpts, grpc.UnaryInterceptor(u))
	}

	if s := chainStream(cfg.streamInterceptors); s != nil {
		serverOpts = append(serverOpts, grpc.StreamInterceptor(s))
	}

	return &Server{
		grpcServer: grpc.NewServer(serverOpts...),
	}
}

// GRPC returns the underlying *grpc.Server so callers can register services.
func (s *Server) GRPC() *grpc.Server {
	return s.grpcServer
}

// MetricsHandler returns an http.Handler that serves Prometheus metrics.
func (s *Server) MetricsHandler() http.Handler {
	return promhttp.Handler()
}
