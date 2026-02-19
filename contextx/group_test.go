package contextx

import "testing"

func TestWithGroupRoundTrip(t *testing.T) {
	ctx := WithGroup(t.Context(), "admin")
	got := GroupFromContext(ctx)
	if got != "admin" {
		t.Fatalf("got %q, want %q", got, "admin")
	}
}

func TestGroupFromContextMissing(t *testing.T) {
	got := GroupFromContext(t.Context())
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}
