// Package main demonstrates goRawrSquirrel's optional authentication
// middleware by starting a real gRPC server with a fake AuthFunc that checks
// the "authorization" metadata header and injects an Actor into the context.
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

// fakeAuthFunc checks for a hard-coded token and enriches the context with an Actor.
func fakeAuthFunc(ctx context.Context, _ string, md metadata.MD) (context.Context, error) {
	vals := md.Get("authorization")
	if len(vals) == 0 || vals[0] != "Bearer my-secret-token" {
		return ctx, errors.New("missing or invalid token")
	}
	return contextx.WithActor(ctx, contextx.Actor{
		Subject:  "alice",
		Tenant:   "acme",
		ClientID: "cli-1",
		Scopes:   []string{"read", "write"},
	}), nil
}

// greeterService is a minimal gRPC service that reads the Actor from context.
type greeterService struct{}

// serviceDesc describes a minimal gRPC service with a single unary method.
var serviceDesc = grpc.ServiceDesc{
	ServiceName: "example.Greeter",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Hello",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				in := new(msg)
				if err := dec(in); err != nil {
					return nil, err
				}
				handler := func(ctx context.Context, _ any) (any, error) {
					actor, ok := contextx.ActorFromContext(ctx)
					if !ok {
						return &msg{}, nil
					}
					fmt.Printf("handler: authenticated subject=%s\n", actor.Subject)
					return &msg{}, nil
				}
				if interceptor != nil {
					return interceptor(ctx, in, &grpc.UnaryServerInfo{
						Server:     srv,
						FullMethod: "/example.Greeter/Hello",
					}, handler)
				}
				return handler(ctx, in)
			},
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "example.proto",
}

// msg is a trivial message type satisfying the proto codec.
type msg struct{}

func (msg) ProtoMessage()            {}
func (msg) Reset()                   {}
func (msg) String() string           { return "" }
func (msg) Marshal() ([]byte, error) { return []byte{}, nil }
func (*msg) Unmarshal([]byte) error  { return nil }

func main() {
	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithAuth(auth.AuthFunc(fakeAuthFunc)),
	)

	srv.GRPC().RegisterService(&serviceDesc, &greeterService{})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen: %v\n", err)
		os.Exit(1)
	}

	go func() {
		if err := srv.GRPC().Serve(lis); err != nil {
			// Server stopped.
		}
	}()
	defer srv.GRPC().Stop()

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to dial: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// 1. Call without auth → should be rejected.
	fmt.Println("--- call without token ---")
	var reply msg
	err = conn.Invoke(context.Background(), "/example.Greeter/Hello", &msg{}, &reply)
	st, _ := status.FromError(err)
	fmt.Printf("code=%v message=%q\n", st.Code(), st.Message())
	if st.Code() != codes.Unauthenticated {
		fmt.Fprintf(os.Stderr, "expected Unauthenticated, got %v\n", st.Code())
		os.Exit(1)
	}

	// 2. Call with valid token → should succeed and print subject.
	fmt.Println("--- call with valid token ---")
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer my-secret-token")
	err = conn.Invoke(ctx, "/example.Greeter/Hello", &msg{}, &reply)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unexpected error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("code=OK")
}
