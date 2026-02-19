// Package main demonstrates goRawrSquirrel's recovery interceptor by starting
// a real gRPC server, registering a service that panics, calling it from a
// client, and printing the returned status code (codes.Internal).
package main

import (
	"context"
	"fmt"
	"net"
	"os"

	gs "github.com/Keksclan/goRawrSquirrel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// panicService is a minimal gRPC service whose only method always panics.
type panicService struct{}

// PanicMethod is the handler that intentionally panics.
func (panicService) PanicMethod(_ any, _ grpc.ServerStream) error {
	panic("intentional panic for demo")
}

// serviceDesc describes a minimal gRPC service with a single unary-style
// method registered via the generic handler path so we can invoke it without
// generated protobuf stubs.
var serviceDesc = grpc.ServiceDesc{
	ServiceName: "example.PanicService",
	HandlerType: (*any)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Boom",
			Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				// Always panic before returning.
				panic("intentional panic for demo")
			},
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "example.proto",
}

func main() {
	// Create a goRawrSquirrel server with recovery enabled.
	srv := gs.NewServer(gs.WithRecovery())

	// Register our panicking service.
	srv.GRPC().RegisterService(&serviceDesc, &panicService{})

	// Listen on a random available port.
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen: %v\n", err)
		os.Exit(1)
	}

	// Start serving in the background.
	go func() {
		if err := srv.GRPC().Serve(lis); err != nil {
			// Server stopped.
		}
	}()
	defer srv.GRPC().Stop()

	// Dial the server.
	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to dial: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// Invoke the panicking method.
	var reply any
	err = conn.Invoke(context.Background(), "/example.PanicService/Boom", &reply, &reply)

	// Print the status code — should be codes.Internal thanks to recovery.
	st, _ := status.FromError(err)
	fmt.Println(st.Code())

	if st.Code() != codes.Internal {
		fmt.Fprintf(os.Stderr, "expected codes.Internal, got %v\n", st.Code())
		os.Exit(1)
	}
}
