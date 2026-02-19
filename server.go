package gorawrsquirrel

import (
	"net/http"

	"github.com/Keksclan/goRawrSquirrel/cache"
	"github.com/Keksclan/goRawrSquirrel/interceptors"
	"github.com/Keksclan/goRawrSquirrel/internal/core"
	"github.com/Keksclan/goRawrSquirrel/ping"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
)

// Server is a composable wrapper around a [grpc.Server] that layers middleware
// (recovery, authentication, rate limiting, caching, IP blocking) via
// functional [Option] values passed to [NewServer].
//
// After construction the underlying gRPC server is available through [Server.GRPC]
// so that service implementations can be registered normally:
//
//	srv := gs.NewServer(gs.WithRecovery())
//	pb.RegisterMyServiceServer(srv.GRPC(), &myImpl{})
type Server struct {
	grpcServer *grpc.Server
	cache      cache.Cache
	cfg        config
}

// NewServer creates a new [Server] by applying the supplied functional [Option]
// values and wiring the resulting unary and stream interceptor chains into
// [grpc.NewServer]. Middleware execution order is determined by fixed priority
// levels (see package-level constants), not by the order options are passed.
//
// Example:
//
//	srv := gs.NewServer(
//		gs.WithRecovery(),
//		gs.WithRateLimitGlobal(500, 100),
//		gs.WithAuth(myAuthFunc),
//		gs.WithCacheL1(10_000),
//	)
func NewServer(opts ...Option) *Server {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}

	// When both L1 and L2 are configured, combine them into a tiered cache.
	if cfg.l1 != nil && cfg.l2 != nil {
		cfg.cache = cache.NewTiered(cfg.l1, cfg.l2)
	}

	unary, stream := cfg.middlewares.Build()
	serverOpts := core.BuildServerOptions(unary, stream, interceptors.ChainUnary, interceptors.ChainStream)

	return &Server{
		grpcServer: grpc.NewServer(serverOpts...),
		cache:      cfg.cache,
		cfg:        cfg,
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

// RegisterPing registers the built-in rawr.Ping health-check service on the
// underlying gRPC server using the supplied [ping.Handler]. If h is nil and
// FunMode is enabled (via [WithFunMode]), a fun handler is used; otherwise
// the default echo handler is registered.
func (s *Server) RegisterPing(h ping.Handler) {
	if h == nil {
		if s.cfg.funMode {
			h = ping.FunHandler(s.cfg.funRand)
		} else {
			h = ping.DefaultHandler()
		}
	}
	ping.Register(s.grpcServer, h)
}

// MetricsHandler returns an http.Handler that serves Prometheus metrics.
func (s *Server) MetricsHandler() http.Handler {
	return promhttp.Handler()
}
