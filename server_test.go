package gorawrsquirrel

import (
	"context"
	"net/http"
	"testing"

	"github.com/Keksclan/goRawrSquirrel/interceptors"
	"google.golang.org/grpc"
)

func TestNewServerReturnsNonNil(t *testing.T) {
	s := NewServer()
	if s == nil {
		t.Fatal("NewServer() returned nil")
	}
}

func TestGRPCReturnsNonNil(t *testing.T) {
	s := NewServer()
	if s.GRPC() == nil {
		t.Fatal("GRPC() returned nil")
	}
}

func TestMetricsHandlerImplementsHTTPHandler(t *testing.T) {
	s := NewServer()
	var h http.Handler = s.MetricsHandler()
	if h == nil {
		t.Fatal("MetricsHandler() returned nil")
	}
}

// makeUnaryInterceptor returns a unary interceptor that appends tag to the log slice.
func makeUnaryInterceptor(tag string, log *[]string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		*log = append(*log, tag+":before")
		resp, err := handler(ctx, req)
		*log = append(*log, tag+":after")
		return resp, err
	}
}

func TestChainUnaryOrder(t *testing.T) {
	var log []string
	a := makeUnaryInterceptor("A", &log)
	b := makeUnaryInterceptor("B", &log)
	c := makeUnaryInterceptor("C", &log)

	chained := interceptors.ChainUnary([]grpc.UnaryServerInterceptor{a, b, c})

	handler := func(ctx context.Context, req any) (any, error) {
		log = append(log, "handler")
		return "ok", nil
	}

	resp, err := chained(t.Context(), "req", &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}

	expected := []string{"A:before", "B:before", "C:before", "handler", "C:after", "B:after", "A:after"}
	if len(log) != len(expected) {
		t.Fatalf("log length mismatch: got %v, want %v", log, expected)
	}
	for i := range expected {
		if log[i] != expected[i] {
			t.Fatalf("log[%d] = %q, want %q\nfull log: %v", i, log[i], expected[i], log)
		}
	}
}

func TestChainUnarySingle(t *testing.T) {
	var called bool
	ic := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		called = true
		return handler(ctx, req)
	}
	chained := interceptors.ChainUnary([]grpc.UnaryServerInterceptor{ic})

	_, _ = chained(t.Context(), nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req any) (any, error) {
		return nil, nil
	})
	if !called {
		t.Fatal("single interceptor was not called")
	}
}

func TestChainUnaryNil(t *testing.T) {
	chained := interceptors.ChainUnary(nil)
	if chained != nil {
		t.Fatal("ChainUnary(nil) should return nil")
	}
}

// makeStreamInterceptor returns a stream interceptor that appends tag to the log slice.
func makeStreamInterceptor(tag string, log *[]string) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		*log = append(*log, tag+":before")
		err := handler(srv, ss)
		*log = append(*log, tag+":after")
		return err
	}
}

func TestChainStreamOrder(t *testing.T) {
	var log []string
	a := makeStreamInterceptor("A", &log)
	b := makeStreamInterceptor("B", &log)

	chained := interceptors.ChainStream([]grpc.StreamServerInterceptor{a, b})

	handler := func(srv any, ss grpc.ServerStream) error {
		log = append(log, "handler")
		return nil
	}

	err := chained(nil, nil, &grpc.StreamServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"A:before", "B:before", "handler", "B:after", "A:after"}
	if len(log) != len(expected) {
		t.Fatalf("log length mismatch: got %v, want %v", log, expected)
	}
	for i := range expected {
		if log[i] != expected[i] {
			t.Fatalf("log[%d] = %q, want %q\nfull log: %v", i, log[i], expected[i], log)
		}
	}
}

func TestNewServerWithInterceptors(t *testing.T) {
	s := NewServer(
		WithUnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}),
		WithStreamInterceptor(func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, ss)
		}),
	)
	if s.GRPC() == nil {
		t.Fatal("GRPC() returned nil after options applied")
	}
}

func TestOptionFunc(t *testing.T) {
	// Verify that Option is a func(*config) — compile-time check.
	var _ Option = func(c *config) {
		_ = c
	}
}
