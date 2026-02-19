package ping_test

import (
	"context"
	"math/rand"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/Keksclan/goRawrSquirrel/ping"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

func startServer(t *testing.T) *bufconn.Listener {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	ping.Register(s, ping.DefaultHandler())
	t.Cleanup(func() { s.Stop() })
	go func() { _ = s.Serve(lis) }()
	return lis
}

func dial(t *testing.T, lis *bufconn.Listener) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestRegisterService(t *testing.T) {
	s := grpc.NewServer()
	ping.Register(s, ping.DefaultHandler())
	info := s.GetServiceInfo()
	si, ok := info["rawr.Ping"]
	if !ok {
		t.Fatal("rawr.Ping service not registered")
	}
	found := false
	for _, m := range si.Methods {
		if m.Name == "Ping" {
			found = true
		}
	}
	if !found {
		t.Fatal("Ping method not found in service info")
	}
}

func TestPingViaBufconn(t *testing.T) {
	lis := startServer(t)
	conn := dial(t, lis)

	req := &ping.PingRequest{Message: "hello"}
	resp := new(ping.PingResponse)

	err := conn.Invoke(t.Context(), "/rawr.Ping/Ping", req, resp)
	if err != nil {
		t.Fatalf("Ping RPC failed: %v", err)
	}
	if resp.Message != "hello" {
		t.Fatalf("expected message %q, got %q", "hello", resp.Message)
	}
	if resp.ServerTimeUnix == 0 {
		t.Fatal("ServerTimeUnix should be non-zero")
	}
	// Verify the timestamp is recent (within last 5 seconds).
	if diff := time.Now().Unix() - resp.ServerTimeUnix; diff < 0 || diff > 5 {
		t.Fatalf("ServerTimeUnix is not recent: %d (diff %d)", resp.ServerTimeUnix, diff)
	}
}

func TestPingEmptyMessage(t *testing.T) {
	lis := startServer(t)
	conn := dial(t, lis)

	req := &ping.PingRequest{}
	resp := new(ping.PingResponse)

	err := conn.Invoke(t.Context(), "/rawr.Ping/Ping", req, resp)
	if err != nil {
		t.Fatalf("Ping RPC failed: %v", err)
	}
	if resp.Message != "" {
		t.Fatalf("expected empty message, got %q", resp.Message)
	}
}

// funMessages mirrors the internal list in ping.go for test assertions.
var funMessages = []string{
	"Squirrel power!",
	"Nom nom nom acorns!",
	"Tail flick activated!",
	"Scurry mode engaged!",
	"Nuts about this request!",
}

func TestDefaultHandler_FunModeOff_NeverReturnsFunMessage(t *testing.T) {
	h := ping.DefaultHandler()
	for range 100 {
		resp, err := h.Ping(t.Context(), &ping.PingRequest{Message: "hello"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if slices.Contains(funMessages, resp.Message) {
			t.Fatalf("DefaultHandler returned fun message %q, expected echo", resp.Message)
		}
		if resp.Message != "hello" {
			t.Fatalf("expected %q, got %q", "hello", resp.Message)
		}
	}
}

func TestFunHandler_DeterministicRand_ReturnsFunMessage(t *testing.T) {
	src := rand.NewSource(42)
	h := ping.FunHandler(src)

	var gotFun, gotEcho bool
	for range 20 {
		resp, err := h.Ping(t.Context(), &ping.PingRequest{Message: "hello"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if slices.Contains(funMessages, resp.Message) {
			gotFun = true
		} else if resp.Message == "hello" {
			gotEcho = true
		} else {
			t.Fatalf("unexpected message: %q", resp.Message)
		}
	}
	if !gotFun {
		t.Fatal("FunHandler with seed 42 never produced a fun message in 20 calls")
	}
	if !gotEcho {
		t.Fatal("FunHandler with seed 42 never echoed in 20 calls")
	}
}

func TestFunHandler_DeterministicRand_Reproducible(t *testing.T) {
	collect := func() []string {
		src := rand.NewSource(7)
		h := ping.FunHandler(src)
		msgs := make([]string, 10)
		for i := range 10 {
			resp, err := h.Ping(t.Context(), &ping.PingRequest{Message: "test"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			msgs[i] = resp.Message
		}
		return msgs
	}

	run1 := collect()
	run2 := collect()
	for i := range run1 {
		if run1[i] != run2[i] {
			t.Fatalf("non-deterministic at index %d: %q vs %q", i, run1[i], run2[i])
		}
	}
}
