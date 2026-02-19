// Package main demonstrates goRawrSquirrel's authentication middleware.
//
// It starts a gRPC server with a simple AuthFunc that validates a bearer
// token from request metadata, then sends two requests — one without and
// one with the correct token — printing the outcome of each.
//
// Run:
//
//	go run ./examples/auth-demo
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	gs "github.com/Keksclan/goRawrSquirrel"
	"github.com/Keksclan/goRawrSquirrel/auth"
	"github.com/Keksclan/goRawrSquirrel/contextx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ── tiny proto-compatible message ──────────────────────────────────────

type msg struct{}

func (msg) ProtoMessage()            {}
func (msg) Reset()                   {}
func (msg) String() string           { return "" }
func (msg) Marshal() ([]byte, error) { return []byte{}, nil }
func (*msg) Unmarshal([]byte) error  { return nil }

// ── auth function ──────────────────────────────────────────────────────

const validToken = "Bearer super-secret"

func demoAuth(ctx context.Context, _ string, md metadata.MD) (context.Context, error) {
	vals := md.Get("authorization")
	if len(vals) == 0 || vals[0] != validToken {
		return ctx, errors.New("missing or invalid authorization token")
	}
	// Enrich context with caller identity.
	return contextx.WithActor(ctx, contextx.Actor{
		Subject: "demo-user",
		Tenant:  "demo-tenant",
	}), nil
}

// ── service definition ─────────────────────────────────────────────────

var serviceDesc = grpc.ServiceDesc{
	ServiceName: "demo.AuthService",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				in := new(msg)
				if err := dec(in); err != nil {
					return nil, err
				}
				handler := func(ctx context.Context, _ any) (any, error) {
					actor, ok := contextx.ActorFromContext(ctx)
					if ok {
						fmt.Printf("  → server: authenticated caller subject=%s tenant=%s\n", actor.Subject, actor.Tenant)
					}
					return &msg{}, nil
				}
				if interceptor != nil {
					return interceptor(ctx, in, &grpc.UnaryServerInfo{
						Server:     srv,
						FullMethod: "/demo.AuthService/Ping",
					}, handler)
				}
				return handler(ctx, in)
			},
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "demo.proto",
}

// ── main ───────────────────────────────────────────────────────────────

func main() {
	fmt.Println("auth-demo: demonstrates goRawrSquirrel authentication middleware")
	fmt.Println()

	// 1. Build server with auth enabled.
	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithAuth(auth.AuthFunc(demoAuth)),
	)
	srv.GRPC().RegisterService(&serviceDesc, struct{}{})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}
	go func() { _ = srv.GRPC().Serve(lis) }()
	defer srv.GRPC().Stop()

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// 2. Call WITHOUT a token → expect Unauthenticated.
	fmt.Println("── call without token ──")
	var reply msg
	err = conn.Invoke(context.Background(), "/demo.AuthService/Ping", &msg{}, &reply)
	st, _ := status.FromError(err)
	fmt.Printf("  result: code=%s message=%q\n", st.Code(), st.Message())
	if st.Code() != codes.Unauthenticated {
		fmt.Fprintf(os.Stderr, "expected Unauthenticated, got %s\n", st.Code())
		os.Exit(1)
	}

	// 3. Call WITH a valid token → expect OK and actor printed by the handler.
	fmt.Println()
	fmt.Println("── call with valid token ──")
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", validToken)
	err = conn.Invoke(ctx, "/demo.AuthService/Ping", &msg{}, &reply)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unexpected error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  result: code=OK")

	fmt.Println()
	fmt.Println("✓ auth-demo complete — unauthenticated requests are rejected, valid tokens pass through")
}
