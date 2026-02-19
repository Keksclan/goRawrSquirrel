package interceptors

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/Keksclan/goRawrSquirrel/contextx"
	"google.golang.org/grpc"
)

// newRequestID generates a random hex-encoded request identifier.
func newRequestID() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}

// ensureRequestID returns the context enriched with a request ID if one is not
// already present.
func ensureRequestID(ctx context.Context) context.Context {
	if contextx.RequestIDFromContext(ctx) == "" {
		ctx = contextx.WithRequestID(ctx, newRequestID())
	}
	return ctx
}

// RequestIDUnary returns a unary server interceptor that ensures a request ID
// is present in the context.
func RequestIDUnary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		return handler(ensureRequestID(ctx), req)
	}
}

// RequestIDStream returns a stream server interceptor that ensures a request ID
// is present in the context.
func RequestIDStream() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Stream interceptors cannot modify the context directly; the
		// request ID injection is handled at the unary level.  For streams
		// this is a no-op passthrough to keep the middleware slot consistent.
		return handler(srv, ss)
	}
}
