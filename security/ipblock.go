package security

import (
	"context"
	"fmt"
	"net/netip"

	"google.golang.org/grpc/metadata"
)

// Mode controls how the CIDR list is interpreted.
type Mode int

const (
	// AllowList only permits IPs that match at least one CIDR.
	AllowList Mode = iota
	// DenyList blocks IPs that match any CIDR and allows all others.
	DenyList
)

// Config holds the configuration for an IPBlocker.
type Config struct {
	Mode           Mode
	CIDRs          []string
	TrustedProxies []string
	HeaderPriority []string
}

// IPBlocker evaluates whether a client IP is allowed or denied based on the
// configured Mode and CIDR ranges.
type IPBlocker struct {
	mode           Mode
	cidrs          []netip.Prefix
	trustedProxies []netip.Prefix
	headerPriority []string
}

// NewIPBlocker creates an IPBlocker from the given Config.  It parses all CIDR
// strings and trusted-proxy strings up-front and returns an error if any entry
// is invalid.
func NewIPBlocker(cfg Config) (*IPBlocker, error) {
	cidrs, err := parsePrefixes(cfg.CIDRs)
	if err != nil {
		return nil, fmt.Errorf("ipblock: invalid CIDR: %w", err)
	}

	proxies, err := parsePrefixes(cfg.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("ipblock: invalid trusted proxy: %w", err)
	}

	hp := cfg.HeaderPriority
	if len(hp) == 0 {
		hp = defaultHeaderPriority
	}

	return &IPBlocker{
		mode:           cfg.Mode,
		cidrs:          cidrs,
		trustedProxies: proxies,
		headerPriority: hp,
	}, nil
}

// Evaluate determines whether the request identified by ctx and md is allowed.
//
// In AllowList mode the IP must match at least one CIDR to be allowed.
// In DenyList mode the IP must not match any CIDR to be allowed.
// If the client IP cannot be determined the request is denied.
func (b *IPBlocker) Evaluate(ctx context.Context, md metadata.MD) (allowed bool) {
	addr, ok := resolveClientAddr(ctx, md, b.trustedProxies, b.headerPriority)
	if !ok {
		return false
	}

	matched := matchesAny(addr, b.cidrs)

	switch b.mode {
	case AllowList:
		return matched
	case DenyList:
		return !matched
	default:
		return false
	}
}

// matchesAny reports whether addr is contained in any of the prefixes.
func matchesAny(addr netip.Addr, prefixes []netip.Prefix) bool {
	for _, p := range prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// parsePrefixes parses a slice of CIDR strings into netip.Prefix values.
// A plain IP address (without a prefix length) is treated as a single-host
// prefix (/32 for IPv4, /128 for IPv6).
func parsePrefixes(raw []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(raw))
	for _, s := range raw {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			// Try as a bare address.
			addr, addrErr := netip.ParseAddr(s)
			if addrErr != nil {
				return nil, fmt.Errorf("%q: %w", s, err)
			}
			p = netip.PrefixFrom(addr, addr.BitLen())
		}
		out = append(out, p)
	}
	return out, nil
}
