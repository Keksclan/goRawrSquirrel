package gorawrsquirrel

import (
	"context"
	"testing"

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

func TestWithRecoveryPrependsInterceptors(t *testing.T) {
	var cfg config
	WithRecovery()(&cfg)

	if len(cfg.unaryInterceptors) != 1 {
		t.Fatalf("expected 1 unary interceptor, got %d", len(cfg.unaryInterceptors))
	}
	if len(cfg.streamInterceptors) != 1 {
		t.Fatalf("expected 1 stream interceptor, got %d", len(cfg.streamInterceptors))
	}
}

func TestWithRecoveryIntegrationUnary(t *testing.T) {
	s := NewServer(WithRecovery())
	if s == nil {
		t.Fatal("NewServer(WithRecovery()) returned nil")
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
