package gorawrsquirrel

import (
	"context"
	"net"
	"testing"

	"github.com/Keksclan/goRawrSquirrel/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func TestIPBlockIntegrationBlockedReturnsPermissionDenied(t *testing.T) {
	// AllowList with only 192.168.0.0/16 — bufconn peer address (127.0.0.1)
	// will NOT match, so the call must be denied.
	blocker, err := security.NewIPBlocker(security.Config{
		Mode:  security.AllowList,
		CIDRs: []string{"192.168.0.0/16"},
	})
	if err != nil {
		t.Fatalf("NewIPBlocker: %v", err)
	}

	srv := NewServer(
		WithRecovery(),
		WithIPBlocker(blocker),
	)

	// Register the gRPC health service so we have a callable method.
	healthpb.RegisterHealthServer(srv.GRPC(), &stubHealthServer{})

	// Set up bufconn listener.
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)

	go func() {
		_ = srv.GRPC().Serve(lis)
	}()
	t.Cleanup(func() { srv.GRPC().Stop() })

	// Dial via bufconn.
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	client := healthpb.NewHealthClient(conn)
	_, err = client.Check(t.Context(), &healthpb.HealthCheckRequest{})

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", st.Code())
	}
}

// stubHealthServer is a minimal health server that always returns SERVING.
type stubHealthServer struct {
	healthpb.UnimplementedHealthServer
}
