// Package main demonstrates the built-in Ping RPC by starting a gRPC server,
// calling Ping, and printing the response.
package main

import (
	"context"
	"fmt"
	"net"
	"os"

	gs "github.com/Keksclan/goRawrSquirrel"
	"github.com/Keksclan/goRawrSquirrel/ping"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// You can supply your own fun messages when FunMode is enabled:
	//   srv := gs.NewServer(
	//       gs.WithRecovery(),
	//       gs.WithFunMode(true),
	//       gs.WithFunMessages([]string{"Woo!", "High five!", "Nailed it!"}),
	//   )
	srv := gs.NewServer(gs.WithRecovery())
	srv.RegisterPing(ping.DefaultHandler())

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen: %v\n", err)
		os.Exit(1)
	}

	go func() { _ = srv.GRPC().Serve(lis) }()
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

	req := &ping.PingRequest{Message: "hello"}
	resp := new(ping.PingResponse)

	if err := conn.Invoke(context.Background(), "/rawr.Ping/Ping", req, resp); err != nil {
		fmt.Fprintf(os.Stderr, "Ping failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Message:        %s\n", resp.Message)
	fmt.Printf("ServerTimeUnix: %d\n", resp.ServerTimeUnix)
}
