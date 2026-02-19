package security

import (
	"context"
	"net"
	"net/netip"
	"strings"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// defaultHeaderPriority is the ordered list of metadata keys inspected when
// the caller does not provide an explicit HeaderPriority.
var defaultHeaderPriority = []string{"x-real-ip", "x-forwarded-for"}

// resolveClientAddr determines the effective client address from the gRPC
// context and metadata.
//
// It first extracts the peer (remote) address from ctx.  If the peer address
// is within trustedProxies, the function walks headerPriority in order and
// returns the first valid IP found in the metadata.  Otherwise (or when no
// valid header IP is found) it returns the peer address itself.
func resolveClientAddr(ctx context.Context, md metadata.MD, trustedProxies []netip.Prefix, headerPriority []string) (netip.Addr, bool) {
	peerAddr, ok := peerAddrFromContext(ctx)
	if !ok {
		return netip.Addr{}, false
	}

	if isTrustedProxy(peerAddr, trustedProxies) {
		if addr, found := addrFromHeaders(md, headerPriority); found {
			return addr, true
		}
	}

	return peerAddr, true
}

// peerAddrFromContext extracts the IP address from the gRPC peer information
// stored in ctx.
func peerAddrFromContext(ctx context.Context) (netip.Addr, bool) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return netip.Addr{}, false
	}
	return addrFromNetAddr(p.Addr)
}

// addrFromNetAddr parses a net.Addr into a netip.Addr, stripping any port.
func addrFromNetAddr(addr net.Addr) (netip.Addr, bool) {
	addrStr := addr.String()

	// Try parsing as host:port first.
	if host, _, err := net.SplitHostPort(addrStr); err == nil {
		addrStr = host
	}

	ip, err := netip.ParseAddr(addrStr)
	if err != nil {
		return netip.Addr{}, false
	}
	return ip, true
}

// isTrustedProxy reports whether addr falls within any of the given prefixes.
func isTrustedProxy(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// addrFromHeaders walks the header keys in priority order and returns the
// first valid IP address found.  For multi-value headers such as
// X-Forwarded-For the left-most (client) entry is used.
func addrFromHeaders(md metadata.MD, priority []string) (netip.Addr, bool) {
	for _, key := range priority {
		vals := md.Get(key)
		for _, v := range vals {
			// X-Forwarded-For may contain comma-separated IPs.
			for part := range strings.SplitSeq(v, ",") {
				trimmed := strings.TrimSpace(part)
				if trimmed == "" {
					continue
				}
				if ip, err := netip.ParseAddr(trimmed); err == nil {
					return ip, true
				}
			}
		}
	}
	return netip.Addr{}, false
}
