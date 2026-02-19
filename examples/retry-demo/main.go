// Package main demonstrates the retry helper by calling Ping against a server
// that fails the first N requests with Unavailable before succeeding.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync/atomic"
	"time"

	gs "github.com/Keksclan/goRawrSquirrel"
	"github.com/Keksclan/goRawrSquirrel/ping"
	"github.com/Keksclan/goRawrSquirrel/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// failingHandler returns Unavailable for the first n calls, then delegates to
// the default handler.
type failingHandler struct {
	remaining atomic.Int32
}

func (h *failingHandler) Ping(ctx context.Context, req *ping.PingRequest) (*ping.PingResponse, error) {
	if h.remaining.Add(-1) >= 0 {
		return nil, status.Error(codes.Unavailable, "not ready yet")
	}
	return ping.DefaultHandler().Ping(ctx, req)
}

func main() {
	handler := &failingHandler{}
	handler.remaining.Store(3) // fail first 3 calls

	srv := gs.NewServer(gs.WithRecovery())
	srv.RegisterPing(handler)

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

	cfg := retry.Config{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    time.Second,
		Jitter:      0.2,
		RetryCodes:  []codes.Code{codes.Unavailable},
	}

	resp, err := retry.Do(context.Background(), cfg, func(ctx context.Context) (*ping.PingResponse, error) {
		req := &ping.PingRequest{Message: "hello with retry"}
		out := new(ping.PingResponse)
		if err := conn.Invoke(ctx, "/rawr.Ping/Ping", req, out); err != nil {
			return nil, err
		}
		return out, nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Ping failed after retries: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Message:        %s\n", resp.Message)
	fmt.Printf("ServerTimeUnix: %d\n", resp.ServerTimeUnix)
}
