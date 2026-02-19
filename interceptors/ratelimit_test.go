package interceptors

import (
	"context"
	"testing"
	"time"

	"github.com/Keksclan/goRawrSquirrel/policy"
	"github.com/Keksclan/goRawrSquirrel/ratelimit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// okHandler is a trivial handler that always succeeds.
func okHandler(_ context.Context, _ any) (any, error) { return "ok", nil }

func codeOf(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	st, _ := status.FromError(err)
	return st.Code()
}

func TestRateLimitUnary_GlobalOnly(t *testing.T) {
	global := ratelimit.NewLimiter(0.001, 2) // burst 2, nearly no refill
	ic := RateLimitUnary(global, nil)

	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}

	// First two should pass (burst).
	for i := range 2 {
		_, err := ic(t.Context(), nil, info, okHandler)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
	}

	// Third should be rejected.
	_, err := ic(t.Context(), nil, info, okHandler)
	if codeOf(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", codeOf(err))
	}
}

func TestRateLimitUnary_PerGroupOverridesGlobal(t *testing.T) {
	// Global: burst=100 (very generous).
	global := ratelimit.NewLimiter(1000, 100)

	// Policy: /api.Service/Heavy limited to burst=2 (very tight).
	resolver := policy.NewResolver(
		policy.Group("heavy").
			Exact("/api.Service/Heavy").
			Policy(policy.Policy{
				RateLimit: &policy.RateLimitRule{Rate: 2, Window: time.Minute},
			}),
	)

	ic := RateLimitUnary(global, resolver)
	heavyInfo := &grpc.UnaryServerInfo{FullMethod: "/api.Service/Heavy"}

	// First two requests should pass (per-group burst=2).
	for i := range 2 {
		_, err := ic(t.Context(), nil, heavyInfo, okHandler)
		if err != nil {
			t.Fatalf("heavy request %d: unexpected error: %v", i, err)
		}
	}

	// Third should be rejected by the per-group limiter.
	_, err := ic(t.Context(), nil, heavyInfo, okHandler)
	if codeOf(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted for heavy, got %v", codeOf(err))
	}

	// An unmatched method should still use the global limiter and succeed.
	otherInfo := &grpc.UnaryServerInfo{FullMethod: "/api.Service/Light"}
	_, err = ic(t.Context(), nil, otherInfo, okHandler)
	if err != nil {
		t.Fatalf("light request: unexpected error: %v", err)
	}
}

func TestRateLimitUnary_ExactBeatsPrefixPolicy(t *testing.T) {
	// Prefix group: generous burst.
	// Exact group: tight burst of 1.
	resolver := policy.NewResolver(
		policy.Group("wide").
			Prefix("/api.Service/").
			Policy(policy.Policy{
				RateLimit: &policy.RateLimitRule{Rate: 100, Window: time.Minute},
			}),
		policy.Group("narrow").
			Exact("/api.Service/Heavy").
			Policy(policy.Policy{
				RateLimit: &policy.RateLimitRule{Rate: 1, Window: time.Minute},
			}),
	)

	global := ratelimit.NewLimiter(1000, 1000)
	ic := RateLimitUnary(global, resolver)

	heavyInfo := &grpc.UnaryServerInfo{FullMethod: "/api.Service/Heavy"}

	// First request passes (burst=1 for exact match "narrow").
	_, err := ic(t.Context(), nil, heavyInfo, okHandler)
	if err != nil {
		t.Fatalf("first heavy request: unexpected error: %v", err)
	}

	// Second should be rejected because the exact match "narrow" has burst=1.
	_, err = ic(t.Context(), nil, heavyInfo, okHandler)
	if codeOf(err) != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted from exact-match policy, got %v", codeOf(err))
	}

	// A different method under the prefix should still succeed (uses "wide" group).
	listInfo := &grpc.UnaryServerInfo{FullMethod: "/api.Service/List"}
	for range 5 {
		_, err = ic(t.Context(), nil, listInfo, okHandler)
		if err != nil {
			t.Fatalf("list request: unexpected error: %v", err)
		}
	}
}
