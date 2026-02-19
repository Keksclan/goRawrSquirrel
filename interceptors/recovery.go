package interceptors

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/Keksclan/goRawrSquirrel/contextx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// RecoveryUnary returns a unary server interceptor that recovers from panics
// and returns an Internal gRPC error instead of crashing the process.
// It also ensures a request ID is present in the context.
func RecoveryUnary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		ctx = ensureRequestID(ctx)
		defer func() {
			if r := recover(); r != nil {
				resp = nil
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}

// RecoveryStream returns a stream server interceptor that recovers from panics
// and returns an Internal gRPC error instead of crashing the process.
func RecoveryStream() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(srv, ss)
	}
}
