package contextx

import "testing"

func TestWithRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(t.Context(), "req-abc-123")
	got := RequestIDFromContext(ctx)
	if got != "req-abc-123" {
		t.Fatalf("got %q, want %q", got, "req-abc-123")
	}
}

func TestRequestIDFromContextMissing(t *testing.T) {
	got := RequestIDFromContext(t.Context())
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}
