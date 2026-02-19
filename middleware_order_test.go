package gorawrsquirrel

import (
	"context"
	"testing"

	"google.golang.org/grpc"
)

func TestMiddlewareOrderDeterminesExecution(t *testing.T) {
	var log []string

	mkUnary := func(tag string) grpc.UnaryServerInterceptor {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			log = append(log, tag)
			return handler(ctx, req)
		}
	}

	var cfg config
	// Register in reverse order; Order values should sort them correctly.
	cfg.middlewares.Add(300, mkUnary("C"), nil)
	cfg.middlewares.Add(100, mkUnary("A"), nil)
	cfg.middlewares.Add(200, mkUnary("B"), nil)

	unary, _ := cfg.middlewares.Build()

	// Execute the chain manually.
	handler := func(_ context.Context, req any) (any, error) {
		log = append(log, "handler")
		return req, nil
	}

	curr := handler
	for i := len(unary) - 1; i >= 0; i-- {
		next := curr
		ic := unary[i]
		curr = func(ctx context.Context, req any) (any, error) {
			return ic(ctx, req, &grpc.UnaryServerInfo{}, next)
		}
	}

	_, err := curr(t.Context(), "req")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"A", "B", "C", "handler"}
	if len(log) != len(expected) {
		t.Fatalf("log length mismatch: got %v, want %v", log, expected)
	}
	for i := range expected {
		if log[i] != expected[i] {
			t.Fatalf("log[%d] = %q, want %q\nfull log: %v", i, log[i], expected[i], log)
		}
	}
}

func TestMiddlewareOrderStableForSameOrder(t *testing.T) {
	var log []string

	mkUnary := func(tag string) grpc.UnaryServerInterceptor {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			log = append(log, tag)
			return handler(ctx, req)
		}
	}

	var cfg config
	// Same order: registration order should be preserved (stable sort).
	cfg.middlewares.Add(100, mkUnary("first"), nil)
	cfg.middlewares.Add(100, mkUnary("second"), nil)
	cfg.middlewares.Add(100, mkUnary("third"), nil)

	unary, _ := cfg.middlewares.Build()

	handler := func(_ context.Context, req any) (any, error) {
		log = append(log, "handler")
		return req, nil
	}

	curr := handler
	for i := len(unary) - 1; i >= 0; i-- {
		next := curr
		ic := unary[i]
		curr = func(ctx context.Context, req any) (any, error) {
			return ic(ctx, req, &grpc.UnaryServerInfo{}, next)
		}
	}

	_, err := curr(t.Context(), "req")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"first", "second", "third", "handler"}
	if len(log) != len(expected) {
		t.Fatalf("log length mismatch: got %v, want %v", log, expected)
	}
	for i := range expected {
		if log[i] != expected[i] {
			t.Fatalf("log[%d] = %q, want %q\nfull log: %v", i, log[i], expected[i], log)
		}
	}
}
