package interceptors

import (
	"context"
	"testing"

	"google.golang.org/grpc"
)

func makeUnaryTag(tag string, log *[]string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		*log = append(*log, tag+":before")
		resp, err := handler(ctx, req)
		*log = append(*log, tag+":after")
		return resp, err
	}
}

func makeStreamTag(tag string, log *[]string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		*log = append(*log, tag+":before")
		err := handler(srv, ss)
		*log = append(*log, tag+":after")
		return err
	}
}

func TestChainUnary_Order(t *testing.T) {
	var log []string
	chained := ChainUnary([]grpc.UnaryServerInterceptor{
		makeUnaryTag("A", &log),
		makeUnaryTag("B", &log),
		makeUnaryTag("C", &log),
	})

	handler := func(_ context.Context, _ any) (any, error) {
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
		t.Fatalf("log mismatch: got %v, want %v", log, expected)
	}
	for i := range expected {
		if log[i] != expected[i] {
			t.Fatalf("log[%d] = %q, want %q\nfull: %v", i, log[i], expected[i], log)
		}
	}
}

func TestChainUnary_Empty(t *testing.T) {
	if ChainUnary(nil) != nil {
		t.Fatal("ChainUnary(nil) should return nil")
	}
}

func TestChainUnary_Single(t *testing.T) {
	var called bool
	ic := func(_ context.Context, _ any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		called = true
		return handler(t.Context(), nil)
	}
	chained := ChainUnary([]grpc.UnaryServerInterceptor{ic})
	_, _ = chained(t.Context(), nil, &grpc.UnaryServerInfo{}, func(_ context.Context, _ any) (any, error) {
		return nil, nil
	})
	if !called {
		t.Fatal("single interceptor was not called")
	}
}

func TestChainStream_Order(t *testing.T) {
	var log []string
	chained := ChainStream([]grpc.StreamServerInterceptor{
		makeStreamTag("A", &log),
		makeStreamTag("B", &log),
	})

	handler := func(_ any, _ grpc.ServerStream) error {
		log = append(log, "handler")
		return nil
	}

	err := chained(nil, nil, &grpc.StreamServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"A:before", "B:before", "handler", "B:after", "A:after"}
	if len(log) != len(expected) {
		t.Fatalf("log mismatch: got %v, want %v", log, expected)
	}
	for i := range expected {
		if log[i] != expected[i] {
			t.Fatalf("log[%d] = %q, want %q\nfull: %v", i, log[i], expected[i], log)
		}
	}
}

func TestChainStream_Empty(t *testing.T) {
	if ChainStream(nil) != nil {
		t.Fatal("ChainStream(nil) should return nil")
	}
}

func TestChainStream_Single(t *testing.T) {
	var called bool
	ic := func(_ any, _ grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		called = true
		return handler(nil, nil)
	}
	chained := ChainStream([]grpc.StreamServerInterceptor{ic})
	_ = chained(nil, nil, &grpc.StreamServerInfo{}, func(_ any, _ grpc.ServerStream) error {
		return nil
	})
	if !called {
		t.Fatal("single interceptor was not called")
	}
}
