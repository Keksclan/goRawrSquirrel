package gorawrsquirrel

import (
	"net/http"

	"github.com/Keksclan/goRawrSquirrel/cache"
	"github.com/Keksclan/goRawrSquirrel/interceptors"
	"github.com/Keksclan/goRawrSquirrel/internal/core"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// Server is a minimal wrapper around a gRPC server with optional metrics.
type Server struct {
	grpcServer *grpc.Server
	cache      cache.Cache
}

// NewServer creates a Server by applying functional options and wiring the
// resulting interceptor chains into grpc.NewServer.
func NewServer(opts ...Option) *Server {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}

	unary, stream := cfg.middlewares.Build()
	serverOpts := core.BuildServerOptions(unary, stream, interceptors.ChainUnary, interceptors.ChainStream)

	return &Server{
		grpcServer: grpc.NewServer(serverOpts...),
		cache:      cfg.cache,
	}
}

// GRPC returns the underlying *grpc.Server so callers can register services.
func (s *Server) GRPC() *grpc.Server {
	return s.grpcServer
}

// Cache returns the cache instance configured via WithCacheL1. It returns nil
// if no cache was configured.
func (s *Server) Cache() cache.Cache {
	return s.cache
}

// MetricsHandler returns an http.Handler that serves Prometheus metrics.
func (s *Server) MetricsHandler() http.Handler {
	return promhttp.Handler()
}
