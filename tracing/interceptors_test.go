package tracing

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	grpcStatus "google.golang.org/grpc/status"
	grpcCodes "google.golang.org/grpc/codes"
)

// newTestConfig returns a TracingConfig backed by an in-memory span recorder.
func newTestConfig(t *testing.T) (*TracingConfig, *tracetest.SpanRecorder) {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = tp.Shutdown(t.Context()) })
	return &TracingConfig{
		TracerProvider: tp,
		Propagators:    propagation.TraceContext{},
	}, rec
}

// ---------- Unary -----------------------------------------------------------

func TestUnaryInterceptor_CreatesSpan(t *testing.T) {
	cfg, rec := newTestConfig(t)
	ic := UnaryServerInterceptor(cfg)

	handler := func(_ context.Context, req any) (any, error) { return "ok", nil }
	info := &grpc.UnaryServerInfo{FullMethod: "/rawr.Ping/Ping"}

	resp, err := ic(t.Context(), "req", info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("expected %q, got %v", "ok", resp)
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Name() != "/rawr.Ping/Ping" {
		t.Fatalf("expected span name %q, got %q", "/rawr.Ping/Ping", span.Name())
	}
	if span.SpanKind() != trace.SpanKindServer {
		t.Fatalf("expected SpanKindServer, got %v", span.SpanKind())
	}

	assertAttr(t, span.Attributes(), "rpc.system", "grpc")
	assertAttr(t, span.Attributes(), "rpc.service", "rawr.Ping")
	assertAttr(t, span.Attributes(), "rpc.method", "Ping")
	assertAttr(t, span.Attributes(), "rpc.grpc.status_code", "OK")
}

func TestUnaryInterceptor_RecordsError(t *testing.T) {
	cfg, rec := newTestConfig(t)
	ic := UnaryServerInterceptor(cfg)

	handler := func(_ context.Context, _ any) (any, error) {
		return nil, grpcStatus.Error(grpcCodes.NotFound, "not found")
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}

	_, err := ic(t.Context(), "req", info, handler)
	if err == nil {
		t.Fatal("expected error")
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Status().Code != codes.Error {
		t.Fatalf("expected Error status, got %v", span.Status().Code)
	}
	assertAttr(t, span.Attributes(), "rpc.grpc.status_code", "NotFound")
}

func TestUnaryInterceptor_NilConfig_Passthrough(t *testing.T) {
	ic := UnaryServerInterceptor(nil)
	handler := func(_ context.Context, req any) (any, error) { return req, nil }
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}

	resp, err := ic(t.Context(), "hello", info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "hello" {
		t.Fatalf("expected %q, got %v", "hello", resp)
	}
}

func TestUnaryInterceptor_ExtractsTraceContext(t *testing.T) {
	cfg, rec := newTestConfig(t)
	ic := UnaryServerInterceptor(cfg)

	// Inject a traceparent header into incoming metadata.
	md := metadata.Pairs("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	ctx := metadata.NewIncomingContext(t.Context(), md)

	handler := func(_ context.Context, req any) (any, error) { return req, nil }
	info := &grpc.UnaryServerInfo{FullMethod: "/svc/Method"}

	_, err := ic(ctx, "req", info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	sc := spans[0].SpanContext()
	if sc.TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace context not extracted; traceID = %s", sc.TraceID())
	}
}

// ---------- Stream ----------------------------------------------------------

type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeServerStream) Context() context.Context { return f.ctx }

func TestStreamInterceptor_CreatesSpan(t *testing.T) {
	cfg, rec := newTestConfig(t)
	ic := StreamServerInterceptor(cfg)

	ss := &fakeServerStream{ctx: t.Context()}
	info := &grpc.StreamServerInfo{FullMethod: "/rawr.Ping/Watch"}

	handler := func(_ any, _ grpc.ServerStream) error { return nil }
	err := ic(nil, ss, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Name() != "/rawr.Ping/Watch" {
		t.Fatalf("expected span name %q, got %q", "/rawr.Ping/Watch", span.Name())
	}
	assertAttr(t, span.Attributes(), "rpc.system", "grpc")
	assertAttr(t, span.Attributes(), "rpc.service", "rawr.Ping")
	assertAttr(t, span.Attributes(), "rpc.method", "Watch")
}

func TestStreamInterceptor_RecordsError(t *testing.T) {
	cfg, rec := newTestConfig(t)
	ic := StreamServerInterceptor(cfg)

	ss := &fakeServerStream{ctx: t.Context()}
	info := &grpc.StreamServerInfo{FullMethod: "/svc/Method"}

	handler := func(_ any, _ grpc.ServerStream) error {
		return errors.New("stream failed")
	}
	err := ic(nil, ss, info, handler)
	if err == nil {
		t.Fatal("expected error")
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status().Code != codes.Error {
		t.Fatalf("expected Error status, got %v", spans[0].Status().Code)
	}
}

func TestStreamInterceptor_NilConfig_Passthrough(t *testing.T) {
	ic := StreamServerInterceptor(nil)
	called := false
	handler := func(_ any, _ grpc.ServerStream) error { called = true; return nil }
	ss := &fakeServerStream{ctx: t.Context()}
	info := &grpc.StreamServerInfo{FullMethod: "/svc/Method"}

	err := ic(nil, ss, info, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

// ---------- helpers ---------------------------------------------------------

func TestSplitFullMethod(t *testing.T) {
	tests := []struct {
		input   string
		service string
		method  string
	}{
		{"/rawr.Ping/Ping", "rawr.Ping", "Ping"},
		{"/service/method", "service", "method"},
		{"noSlash", "noSlash", ""},
	}
	for _, tt := range tests {
		svc, meth := splitFullMethod(tt.input)
		if svc != tt.service || meth != tt.method {
			t.Errorf("splitFullMethod(%q) = (%q, %q), want (%q, %q)", tt.input, svc, meth, tt.service, tt.method)
		}
	}
}

func assertAttr(t *testing.T, attrs []attribute.KeyValue, key, want string) {
	t.Helper()
	for _, a := range attrs {
		if string(a.Key) == key {
			if a.Value.AsString() != want {
				t.Errorf("attribute %q = %q, want %q", key, a.Value.AsString(), want)
			}
			return
		}
	}
	t.Errorf("attribute %q not found", key)
}
