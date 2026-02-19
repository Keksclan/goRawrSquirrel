// Package auth provides the authentication function type used by the
// optional authentication middleware.
package auth

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// AuthFunc is a user-supplied callback that authenticates a gRPC request.
// It receives the request context, the full method name, and the incoming
// metadata.  On success it returns a (possibly enriched) context; on failure
// it returns an error.
//
// The library does NOT parse tokens — that is the responsibility of the
// AuthFunc implementation.
type AuthFunc func(ctx context.Context, fullMethod string, md metadata.MD) (context.Context, error)
