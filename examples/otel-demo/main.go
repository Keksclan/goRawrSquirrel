// Package main demonstrates OpenTelemetry tracing with goRawrSquirrel.
// It configures a stdout exporter (for demo purposes only), starts a gRPC
// server with tracing enabled, calls Ping, and prints the captured trace.
package main

import (
	"context"
	"fmt"
	"net"
	"os"

	gs "github.com/Keksclan/goRawrSquirrel"
	"github.com/Keksclan/goRawrSquirrel/ping"
	"github.com/Keksclan/goRawrSquirrel/tracing"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// --- exporter (user responsibility, shown here for demo) ----------------
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create stdout exporter: %v\n", err)
		os.Exit(1)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// --- server -------------------------------------------------------------
	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithOpenTelemetry(tracing.TracingConfig{
			TracerProvider: tp,
		}),
	)
	srv.RegisterPing(ping.DefaultHandler())

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen: %v\n", err)
		os.Exit(1)
	}

	go func() { _ = srv.GRPC().Serve(lis) }()
	defer srv.GRPC().Stop()

	// --- client call --------------------------------------------------------
	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to dial: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	req := &ping.PingRequest{Message: "hello from otel-demo"}
	resp := new(ping.PingResponse)

	if err := conn.Invoke(context.Background(), "/rawr.Ping/Ping", req, resp); err != nil {
		fmt.Fprintf(os.Stderr, "Ping failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Message:        %s\n", resp.Message)
	fmt.Printf("ServerTimeUnix: %d\n", resp.ServerTimeUnix)
	fmt.Println("\n(trace output appears above via stdout exporter)")
}
