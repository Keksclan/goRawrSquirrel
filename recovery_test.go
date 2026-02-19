package gorawrsquirrel

import (
	"context"
	"testing"

	"github.com/Keksclan/goRawrSquirrel/contextx"
	"github.com/Keksclan/goRawrSquirrel/interceptors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRecoveryUnaryReturnInternalOnPanic(t *testing.T) {
	ic := interceptors.RecoveryUnary()

	handler := func(_ context.Context, _ any) (any, error) {
		panic("boom")
	}

	resp, err := ic(t.Context(), "req", &grpc.UnaryServerInfo{}, handler)
	if resp != nil {
		t.Fatalf("expected nil response, got %v", resp)
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Internal {
		t.Fatalf("expected codes.Internal, got %v", st.Code())
	}
	if st.Message() != "internal server error" {
		t.Fatalf("expected %q, got %q", "internal server error", st.Message())
	}
}

func TestRecoveryUnaryPassthroughOnNoPanic(t *testing.T) {
	ic := interceptors.RecoveryUnary()

	handler := func(_ context.Context, req any) (any, error) {
		return req, nil
	}

	resp, err := ic(t.Context(), "hello", &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "hello" {
		t.Fatalf("expected %q, got %v", "hello", resp)
	}
}

func TestRecoveryStreamReturnInternalOnPanic(t *testing.T) {
	ic := interceptors.RecoveryStream()

	handler := func(_ any, _ grpc.ServerStream) error {
		panic("boom")
	}

	err := ic(nil, nil, &grpc.StreamServerInfo{}, handler)

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Internal {
		t.Fatalf("expected codes.Internal, got %v", st.Code())
	}
	if st.Message() != "internal server error" {
		t.Fatalf("expected %q, got %q", "internal server error", st.Message())
	}
}

func TestRecoveryStreamPassthroughOnNoPanic(t *testing.T) {
	ic := interceptors.RecoveryStream()

	handler := func(_ any, _ grpc.ServerStream) error {
		return nil
	}

	err := ic(nil, nil, &grpc.StreamServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithRecoveryRegistersMiddleware(t *testing.T) {
	var cfg config
	WithRecovery()(&cfg)

	unary, stream := cfg.middlewares.Build()
	if len(unary) != 2 {
		t.Fatalf("expected 2 unary interceptors, got %d", len(unary))
	}
	if len(stream) != 2 {
		t.Fatalf("expected 2 stream interceptors, got %d", len(stream))
	}
}

func TestWithRecoveryIntegrationUnary(t *testing.T) {
	s := NewServer(WithRecovery())
	if s == nil {
		t.Fatal("NewServer(WithRecovery()) returned nil")
	}
}

func TestRequestIDUnaryInjectsRequestID(t *testing.T) {
	ic := interceptors.RequestIDUnary()

	var captured string
	handler := func(ctx context.Context, req any) (any, error) {
		captured = contextx.RequestIDFromContext(ctx)
		return req, nil
	}

	_, err := ic(t.Context(), "req", &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured == "" {
		t.Fatal("expected request ID in context, got empty string")
	}
}

func TestRequestIDUnaryPreservesExistingRequestID(t *testing.T) {
	ic := interceptors.RequestIDUnary()

	ctx := contextx.WithRequestID(t.Context(), "existing-id")
	var captured string
	handler := func(ctx context.Context, req any) (any, error) {
		captured = contextx.RequestIDFromContext(ctx)
		return req, nil
	}

	_, err := ic(ctx, "req", &grpc.UnaryServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured != "existing-id" {
		t.Fatalf("got %q, want %q", captured, "existing-id")
	}
}

func TestRecoveryUnaryNonStringPanic(t *testing.T) {
	ic := interceptors.RecoveryUnary()

	handler := func(_ context.Context, _ any) (any, error) {
		panic(42)
	}

	resp, err := ic(t.Context(), "req", &grpc.UnaryServerInfo{}, handler)
	if resp != nil {
		t.Fatalf("expected nil response, got %v", resp)
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Internal {
		t.Fatalf("expected codes.Internal, got %v", st.Code())
	}
}
