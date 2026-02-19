package contextx

// contextKey is an unexported type used as context key to avoid collisions
// with keys defined in other packages.
type contextKey int

const (
	actorKey contextKey = iota
	requestIDKey
	groupKey
)
