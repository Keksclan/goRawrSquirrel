package interceptors

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRecoveryUnary_Panic_ReturnsInternal(t *testing.T) {
	ic := RecoveryUnary()
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
}

func TestRecoveryUnary_NoPanic_Passthrough(t *testing.T) {
	ic := RecoveryUnary()
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

func TestRecoveryUnary_NonStringPanic_ReturnsInternal(t *testing.T) {
	ic := RecoveryUnary()
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

func TestRecoveryStream_Panic_ReturnsInternal(t *testing.T) {
	ic := RecoveryStream()
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
}

func TestRecoveryStream_NoPanic_Passthrough(t *testing.T) {
	ic := RecoveryStream()
	handler := func(_ any, _ grpc.ServerStream) error {
		return nil
	}

	err := ic(nil, nil, &grpc.StreamServerInfo{}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
