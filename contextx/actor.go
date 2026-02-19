package contextx

import "context"

// Actor represents the authenticated identity behind a request. It is
// typically populated by an authentication interceptor and stored in the
// request context via [WithActor]. Downstream handlers retrieve it with
// [ActorFromContext].
//
// Example:
//
//	actor := contextx.Actor{Subject: "user-42", Tenant: "acme"}
//	ctx = contextx.WithActor(ctx, actor)
type Actor struct {
	Subject  string
	Tenant   string
	ClientID string
	Scopes   []string
}

// WithActor returns a derived context that carries the given Actor.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey, a)
}

// ActorFromContext extracts the Actor stored in ctx.
// The boolean return value indicates whether an Actor was present.
func ActorFromContext(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey).(Actor)
	return a, ok
}
