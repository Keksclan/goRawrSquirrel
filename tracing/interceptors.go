// Package tracing provides OpenTelemetry tracing interceptors for gRPC
// servers. It is entirely optional — tracing is only active when
// [TracingConfig] is wired in via the WithOpenTelemetry server option.
package tracing

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	grpcStatus "google.golang.org/grpc/status"
)

// TracingConfig holds the OpenTelemetry configuration used by the gRPC
// tracing interceptors.
type TracingConfig struct {
	// TracerProvider supplies the Tracer used to create spans. When nil the
	// global otel.GetTracerProvider() is used.
	TracerProvider trace.TracerProvider

	// Propagators extracts and injects trace context from/into carriers.
	// When nil the global otel.GetTextMapPropagator() is used.
	Propagators propagation.TextMapPropagator
}

// tracer returns a configured [trace.Tracer].
func (c *TracingConfig) tracer() trace.Tracer {
	tp := c.TracerProvider
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	return tp.Tracer("github.com/Keksclan/goRawrSquirrel/tracing")
}

// propagators returns the configured propagator (or global default).
func (c *TracingConfig) propagators() propagation.TextMapPropagator {
	if c.Propagators != nil {
		return c.Propagators
	}
	return otel.GetTextMapPropagator()
}

// UnaryServerInterceptor returns a [grpc.UnaryServerInterceptor] that creates
// a span for every unary RPC. If cfg is nil the interceptor is a no-op
// passthrough.
func UnaryServerInterceptor(cfg *TracingConfig) grpc.UnaryServerInterceptor {
	if cfg == nil {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			return handler(ctx, req)
		}
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = extract(ctx, cfg)
		ctx, span := cfg.tracer().Start(ctx, info.FullMethod, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		service, method := splitFullMethod(info.FullMethod)
		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.service", service),
			attribute.String("rpc.method", method),
		)

		resp, err := handler(ctx, req)
		recordStatus(span, err)
		return resp, err
	}
}

// StreamServerInterceptor returns a [grpc.StreamServerInterceptor] that
// creates a span for every streaming RPC. If cfg is nil the interceptor is a
// no-op passthrough.
func StreamServerInterceptor(cfg *TracingConfig) grpc.StreamServerInterceptor {
	if cfg == nil {
		return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, ss)
		}
	}
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := extract(ss.Context(), cfg)
		ctx, span := cfg.tracer().Start(ctx, info.FullMethod, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		service, method := splitFullMethod(info.FullMethod)
		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.service", service),
			attribute.String("rpc.method", method),
		)

		err := handler(srv, &wrappedStream{ServerStream: ss, ctx: ctx})
		recordStatus(span, err)
		return err
	}
}

// --- helpers ----------------------------------------------------------------

// metadataCarrier adapts gRPC [metadata.MD] to the OTel
// [propagation.TextMapCarrier] interface.
type metadataCarrier metadata.MD

func (mc metadataCarrier) Get(key string) string {
	vals := metadata.MD(mc).Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (mc metadataCarrier) Set(key, value string) {
	metadata.MD(mc).Set(key, value)
}

func (mc metadataCarrier) Keys() []string {
	md := metadata.MD(mc)
	keys := make([]string, 0, len(md))
	for k := range md {
		keys = append(keys, k)
	}
	return keys
}

// extract pulls trace context from incoming gRPC metadata into ctx.
func extract(ctx context.Context, cfg *TracingConfig) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}
	return cfg.propagators().Extract(ctx, metadataCarrier(md))
}

// splitFullMethod splits "/service/method" into ("service", "method").
func splitFullMethod(fullMethod string) (string, string) {
	fullMethod = strings.TrimPrefix(fullMethod, "/")
	service, method, ok := strings.Cut(fullMethod, "/")
	if !ok {
		return fullMethod, ""
	}
	return service, method
}

// recordStatus sets the span status and records the gRPC status code.
func recordStatus(span trace.Span, err error) {
	st, _ := grpcStatus.FromError(err)
	span.SetAttributes(attribute.String("rpc.grpc.status_code", st.Code().String()))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, st.Message())
	} else {
		span.SetStatus(codes.Ok, "")
	}
}

// wrappedStream overrides Context() to carry the traced context.
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }
