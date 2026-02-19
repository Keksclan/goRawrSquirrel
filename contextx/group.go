package contextx

import "context"

// WithGroup returns a derived context that carries the given group name.
func WithGroup(ctx context.Context, group string) context.Context {
	return context.WithValue(ctx, groupKey, group)
}

// GroupFromContext extracts the group name stored in ctx.
// It returns an empty string when no group is present.
func GroupFromContext(ctx context.Context) string {
	g, _ := ctx.Value(groupKey).(string)
	return g
}
