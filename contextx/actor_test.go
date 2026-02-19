package contextx

import (
	"slices"
	"testing"
)

func TestWithActorRoundTrip(t *testing.T) {
	ctx := t.Context()
	a := Actor{
		Subject:  "user-1",
		Tenant:   "tenant-a",
		ClientID: "client-42",
		Scopes:   []string{"read", "write"},
	}

	ctx = WithActor(ctx, a)
	got, ok := ActorFromContext(ctx)
	if !ok {
		t.Fatal("expected actor in context")
	}
	if got.Subject != a.Subject {
		t.Fatalf("Subject: got %q, want %q", got.Subject, a.Subject)
	}
	if got.Tenant != a.Tenant {
		t.Fatalf("Tenant: got %q, want %q", got.Tenant, a.Tenant)
	}
	if got.ClientID != a.ClientID {
		t.Fatalf("ClientID: got %q, want %q", got.ClientID, a.ClientID)
	}
	if !slices.Equal(got.Scopes, a.Scopes) {
		t.Fatalf("Scopes: got %v, want %v", got.Scopes, a.Scopes)
	}
}

func TestActorFromContextMissing(t *testing.T) {
	_, ok := ActorFromContext(t.Context())
	if ok {
		t.Fatal("expected no actor in empty context")
	}
}
