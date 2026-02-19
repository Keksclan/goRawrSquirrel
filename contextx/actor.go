package contextx

import "context"

// Actor describes the authenticated caller for a request.
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
