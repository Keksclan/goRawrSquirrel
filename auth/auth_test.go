package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Keksclan/goRawrSquirrel/auth"
	"github.com/Keksclan/goRawrSquirrel/contextx"
	"github.com/Keksclan/goRawrSquirrel/interceptors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// fakeAuth returns an AuthFunc that checks for "authorization" metadata.
// If the value is "valid-token", it injects an Actor; otherwise it returns an error.
func fakeAuth() auth.AuthFunc {
	return func(ctx context.Context, _ string, md metadata.MD) (context.Context, error) {
		vals := md.Get("authorization")
		if len(vals) == 0 || vals[0] != "valid-token" {
			return ctx, errors.New("bad token")
		}
		return contextx.WithActor(ctx, contextx.Actor{
			Subject: "user-1",
		}), nil
	}
}

func TestAuthUnary_MissingAuth(t *testing.T) {
	ic := interceptors.AuthUnary(fakeAuth())

	handler := func(_ context.Context, _ any) (any, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	}

	// No metadata → unauthenticated.
	_, err := ic(t.Context(), "req", &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}, handler)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unauthenticated {
		t.Fatalf("expected codes.Unauthenticated, got %v", st.Code())
	}
}

func TestAuthUnary_ValidAuth(t *testing.T) {
	ic := interceptors.AuthUnary(fakeAuth())

	var capturedActor contextx.Actor
	handler := func(ctx context.Context, req any) (any, error) {
		a, ok := contextx.ActorFromContext(ctx)
		if !ok {
			t.Fatal("expected actor in context")
		}
		capturedActor = a
		return "ok", nil
	}

	// Inject metadata with valid token.
	md := metadata.Pairs("authorization", "valid-token")
	ctx := metadata.NewIncomingContext(t.Context(), md)

	resp, err := ic(ctx, "req", &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected %q, got %v", "ok", resp)
	}
	if capturedActor.Subject != "user-1" {
		t.Fatalf("expected Subject %q, got %q", "user-1", capturedActor.Subject)
	}
}

func TestAuthUnary_StatusErrorPassthrough(t *testing.T) {
	// AuthFunc that returns a status error with a custom code.
	fn := func(ctx context.Context, _ string, _ metadata.MD) (context.Context, error) {
		return ctx, status.Error(codes.PermissionDenied, "forbidden")
	}
	ic := interceptors.AuthUnary(fn)

	handler := func(_ context.Context, _ any) (any, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	}

	_, err := ic(t.Context(), "req", &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}, handler)
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("expected codes.PermissionDenied, got %v", st.Code())
	}
}
