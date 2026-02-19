package policy

import (
	"testing"
	"time"
)

func TestResolve_ExactMatch(t *testing.T) {
	r := NewResolver(
		Group("admin").
			Exact("/admin.Service/Delete").
			Policy(Policy{AuthRequired: true}),
	)

	name, pol, ok := r.Resolve("/admin.Service/Delete")
	if !ok {
		t.Fatal("expected a match")
	}
	if name != "admin" {
		t.Fatalf("got group %q, want %q", name, "admin")
	}
	if !pol.AuthRequired {
		t.Fatal("expected AuthRequired to be true")
	}
}

func TestResolve_PrefixMatch(t *testing.T) {
	r := NewResolver(
		Group("public").
			Prefix("/public.").
			Policy(Policy{Timeout: 5 * time.Second}),
	)

	name, pol, ok := r.Resolve("/public.Service/List")
	if !ok {
		t.Fatal("expected a match")
	}
	if name != "public" {
		t.Fatalf("got group %q, want %q", name, "public")
	}
	if pol.Timeout != 5*time.Second {
		t.Fatalf("got timeout %v, want %v", pol.Timeout, 5*time.Second)
	}
}

func TestResolve_RegexMatch(t *testing.T) {
	r := NewResolver(
		Group("health").
			Regex(`/grpc\.health\.`).
			Policy(Policy{}),
	)

	_, _, ok := r.Resolve("/grpc.health.v1.Health/Check")
	if !ok {
		t.Fatal("expected a regex match")
	}
}

func TestResolve_NoMatch(t *testing.T) {
	r := NewResolver(
		Group("admin").Exact("/admin.Service/Delete").Policy(Policy{}),
	)

	_, _, ok := r.Resolve("/other.Service/Get")
	if ok {
		t.Fatal("expected no match")
	}
}

func TestResolve_ExactBeatsPrefix(t *testing.T) {
	r := NewResolver(
		Group("prefix-group").
			Prefix("/svc.Service/").
			Policy(Policy{Timeout: 1 * time.Second}),
		Group("exact-group").
			Exact("/svc.Service/Get").
			Policy(Policy{Timeout: 2 * time.Second}),
	)

	name, pol, ok := r.Resolve("/svc.Service/Get")
	if !ok {
		t.Fatal("expected a match")
	}
	if name != "exact-group" {
		t.Fatalf("exact should beat prefix: got %q", name)
	}
	if pol.Timeout != 2*time.Second {
		t.Fatalf("got timeout %v, want %v", pol.Timeout, 2*time.Second)
	}
}

func TestResolve_PrefixBeatsRegex(t *testing.T) {
	r := NewResolver(
		Group("regex-group").
			Regex(`/svc\.Service/`).
			Policy(Policy{Timeout: 1 * time.Second}),
		Group("prefix-group").
			Prefix("/svc.Service/").
			Policy(Policy{Timeout: 2 * time.Second}),
	)

	name, _, ok := r.Resolve("/svc.Service/List")
	if !ok {
		t.Fatal("expected a match")
	}
	if name != "prefix-group" {
		t.Fatalf("prefix should beat regex: got %q", name)
	}
}

func TestResolve_LongerPrefixWins(t *testing.T) {
	r := NewResolver(
		Group("short").
			Prefix("/svc.").
			Policy(Policy{Timeout: 1 * time.Second}),
		Group("long").
			Prefix("/svc.Service/").
			Policy(Policy{Timeout: 2 * time.Second}),
	)

	name, _, ok := r.Resolve("/svc.Service/Get")
	if !ok {
		t.Fatal("expected a match")
	}
	if name != "long" {
		t.Fatalf("longer prefix should win: got %q", name)
	}
}

func TestResolve_StableFallback(t *testing.T) {
	// Two exact matches of equal length — the first registered group wins.
	r := NewResolver(
		Group("first").
			Exact("/svc.Service/Get").
			Policy(Policy{Timeout: 1 * time.Second}),
		Group("second").
			Exact("/svc.Service/Get").
			Policy(Policy{Timeout: 2 * time.Second}),
	)

	name, pol, ok := r.Resolve("/svc.Service/Get")
	if !ok {
		t.Fatal("expected a match")
	}
	if name != "first" {
		t.Fatalf("first-registered group should win: got %q", name)
	}
	if pol.Timeout != 1*time.Second {
		t.Fatalf("got timeout %v, want %v", pol.Timeout, 1*time.Second)
	}
}

func TestResolve_MultipleRulesInGroup(t *testing.T) {
	r := NewResolver(
		Group("mixed").
			Exact("/svc.A/One").
			Prefix("/svc.B/").
			Regex(`/svc\.C/`).
			Policy(Policy{AuthRequired: true}),
	)

	for _, method := range []string{
		"/svc.A/One",
		"/svc.B/Two",
		"/svc.C/Three",
	} {
		name, _, ok := r.Resolve(method)
		if !ok {
			t.Fatalf("expected match for %s", method)
		}
		if name != "mixed" {
			t.Fatalf("got group %q for %s, want %q", name, method, "mixed")
		}
	}
}

func TestResolve_RateLimitPolicy(t *testing.T) {
	r := NewResolver(
		Group("limited").
			Exact("/api.Service/Heavy").
			Policy(Policy{
				RateLimit: &RateLimitRule{Rate: 100, Window: time.Minute},
			}),
	)

	_, pol, ok := r.Resolve("/api.Service/Heavy")
	if !ok {
		t.Fatal("expected a match")
	}
	if pol.RateLimit == nil {
		t.Fatal("expected RateLimit to be set")
	}
	if pol.RateLimit.Rate != 100 {
		t.Fatalf("got rate %d, want 100", pol.RateLimit.Rate)
	}
}
