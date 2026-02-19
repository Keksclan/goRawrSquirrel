package security

import (
	"net"
	"testing"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// fakePeerAddr implements net.Addr for testing purposes.
type fakePeerAddr struct{ addr string }

func (f fakePeerAddr) Network() string { return "tcp" }
func (f fakePeerAddr) String() string  { return f.addr }

func TestDenyList_BlocksMatchingIP(t *testing.T) {
	blocker, err := NewIPBlocker(Config{
		Mode:  DenyList,
		CIDRs: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := peer.NewContext(t.Context(), &peer.Peer{
		Addr: fakePeerAddr{addr: "10.1.2.3:5000"},
	})

	if blocker.Evaluate(ctx, nil) {
		t.Fatal("expected 10.1.2.3 to be blocked by deny list")
	}
}

func TestDenyList_AllowsNonMatchingIP(t *testing.T) {
	blocker, err := NewIPBlocker(Config{
		Mode:  DenyList,
		CIDRs: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := peer.NewContext(t.Context(), &peer.Peer{
		Addr: fakePeerAddr{addr: "192.168.1.1:5000"},
	})

	if !blocker.Evaluate(ctx, nil) {
		t.Fatal("expected 192.168.1.1 to be allowed by deny list")
	}
}

func TestAllowList_AllowsMatchingIP(t *testing.T) {
	blocker, err := NewIPBlocker(Config{
		Mode:  AllowList,
		CIDRs: []string{"192.168.0.0/16"},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := peer.NewContext(t.Context(), &peer.Peer{
		Addr: fakePeerAddr{addr: "192.168.1.50:8080"},
	})

	if !blocker.Evaluate(ctx, nil) {
		t.Fatal("expected 192.168.1.50 to be allowed by allow list")
	}
}

func TestAllowList_BlocksNonMatchingIP(t *testing.T) {
	blocker, err := NewIPBlocker(Config{
		Mode:  AllowList,
		CIDRs: []string{"192.168.0.0/16"},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := peer.NewContext(t.Context(), &peer.Peer{
		Addr: fakePeerAddr{addr: "10.0.0.1:8080"},
	})

	if blocker.Evaluate(ctx, nil) {
		t.Fatal("expected 10.0.0.1 to be blocked by allow list")
	}
}

func TestTrustedProxy_UsesHeader(t *testing.T) {
	blocker, err := NewIPBlocker(Config{
		Mode:           DenyList,
		CIDRs:          []string{"203.0.113.0/24"},
		TrustedProxies: []string{"10.0.0.1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Peer is the trusted proxy.
	ctx := peer.NewContext(t.Context(), &peer.Peer{
		Addr: fakePeerAddr{addr: "10.0.0.1:9000"},
	})

	md := metadata.Pairs("x-real-ip", "203.0.113.42")

	if blocker.Evaluate(ctx, md) {
		t.Fatal("expected 203.0.113.42 (from header via trusted proxy) to be denied")
	}
}

func TestUntrustedProxy_IgnoresHeader(t *testing.T) {
	blocker, err := NewIPBlocker(Config{
		Mode:           DenyList,
		CIDRs:          []string{"203.0.113.0/24"},
		TrustedProxies: []string{"10.0.0.1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Peer is NOT the trusted proxy.
	ctx := peer.NewContext(t.Context(), &peer.Peer{
		Addr: fakePeerAddr{addr: "172.16.0.5:9000"},
	})

	// Header claims a denied IP, but should be ignored.
	md := metadata.Pairs("x-real-ip", "203.0.113.42")

	if !blocker.Evaluate(ctx, md) {
		t.Fatal("expected 172.16.0.5 to be allowed — header should be ignored for untrusted proxy")
	}
}

func TestTrustedProxy_FallsBackToPeerWhenHeaderMissing(t *testing.T) {
	blocker, err := NewIPBlocker(Config{
		Mode:           DenyList,
		CIDRs:          []string{"203.0.113.0/24"},
		TrustedProxies: []string{"10.0.0.1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := peer.NewContext(t.Context(), &peer.Peer{
		Addr: fakePeerAddr{addr: "10.0.0.1:9000"},
	})

	// No header present — should fall back to peer addr (the proxy itself).
	if !blocker.Evaluate(ctx, nil) {
		t.Fatal("expected trusted proxy addr to be allowed when no header is set")
	}
}

func TestXForwardedFor_MultipleIPs(t *testing.T) {
	blocker, err := NewIPBlocker(Config{
		Mode:           DenyList,
		CIDRs:          []string{"198.51.100.0/24"},
		TrustedProxies: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := peer.NewContext(t.Context(), &peer.Peer{
		Addr: fakePeerAddr{addr: "10.0.0.1:9000"},
	})

	md := metadata.Pairs("x-forwarded-for", "198.51.100.5, 10.0.0.2")

	if blocker.Evaluate(ctx, md) {
		t.Fatal("expected leftmost IP 198.51.100.5 from X-Forwarded-For to be denied")
	}
}

func TestNoPeer_DeniesRequest(t *testing.T) {
	blocker, err := NewIPBlocker(Config{
		Mode:  AllowList,
		CIDRs: []string{"0.0.0.0/0"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Context without peer information.
	if blocker.Evaluate(t.Context(), nil) {
		t.Fatal("expected denial when peer info is missing")
	}
}

func TestNewIPBlocker_InvalidCIDR(t *testing.T) {
	_, err := NewIPBlocker(Config{
		Mode:  DenyList,
		CIDRs: []string{"not-a-cidr"},
	})
	if err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
}

func TestNewIPBlocker_InvalidTrustedProxy(t *testing.T) {
	_, err := NewIPBlocker(Config{
		Mode:           DenyList,
		TrustedProxies: []string{"not-valid"},
	})
	if err == nil {
		t.Fatal("expected error for invalid trusted proxy")
	}
}

func TestDefaultHeaderPriority(t *testing.T) {
	blocker, err := NewIPBlocker(Config{
		Mode:           DenyList,
		CIDRs:          []string{"203.0.113.0/24"},
		TrustedProxies: []string{"10.0.0.1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := peer.NewContext(t.Context(), &peer.Peer{
		Addr: fakePeerAddr{addr: "10.0.0.1:9000"},
	})

	// x-real-ip has higher priority than x-forwarded-for by default.
	md := metadata.Pairs(
		"x-real-ip", "203.0.113.1",
		"x-forwarded-for", "192.168.1.1",
	)

	if blocker.Evaluate(ctx, md) {
		t.Fatal("expected x-real-ip (203.0.113.1) to be used and denied")
	}
}

func TestCustomHeaderPriority(t *testing.T) {
	blocker, err := NewIPBlocker(Config{
		Mode:           AllowList,
		CIDRs:          []string{"172.16.0.0/12"},
		TrustedProxies: []string{"10.0.0.1"},
		HeaderPriority: []string{"x-custom-ip"},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := peer.NewContext(t.Context(), &peer.Peer{
		Addr: fakePeerAddr{addr: "10.0.0.1:9000"},
	})

	md := metadata.Pairs("x-custom-ip", "172.16.5.5")

	if !blocker.Evaluate(ctx, md) {
		t.Fatal("expected 172.16.5.5 from custom header to be allowed")
	}
}

// Ensure the resolver works with a real *net.TCPAddr (not just our fake).
func TestRealTCPAddr(t *testing.T) {
	blocker, err := NewIPBlocker(Config{
		Mode:  DenyList,
		CIDRs: []string{"192.0.2.0/24"},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := peer.NewContext(t.Context(), &peer.Peer{
		Addr: &net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 1234},
	})

	if blocker.Evaluate(ctx, nil) {
		t.Fatal("expected 192.0.2.1 to be denied")
	}
}
