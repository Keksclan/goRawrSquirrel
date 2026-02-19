package cache

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func mustNewL1(t *testing.T) *L1 {
	t.Helper()
	c, err := NewL1(1000)
	if err != nil {
		t.Fatalf("NewL1: %v", err)
	}
	return c
}

func TestL1_GetSet(t *testing.T) {
	c := mustNewL1(t)
	ctx := t.Context()

	// Miss returns false.
	_, ok, err := c.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if ok {
		t.Fatal("expected miss")
	}

	// Set then Get.
	if err := c.Set(ctx, "k1", []byte("v1"), 0); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	val, ok, err := c.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if !ok {
		t.Fatal("expected hit")
	}
	if string(val) != "v1" {
		t.Fatalf("got %q, want %q", val, "v1")
	}
}

func TestL1_GetOrSet_LoaderCalledOnce(t *testing.T) {
	c := mustNewL1(t)
	ctx := t.Context()

	var calls atomic.Int32
	loader := func(_ context.Context) ([]byte, error) {
		calls.Add(1)
		return []byte("loaded"), nil
	}

	v1, err := c.GetOrSet(ctx, "k", time.Minute, loader)
	if err != nil {
		t.Fatalf("GetOrSet 1: %v", err)
	}
	if string(v1) != "loaded" {
		t.Fatalf("got %q, want %q", v1, "loaded")
	}

	v2, err := c.GetOrSet(ctx, "k", time.Minute, loader)
	if err != nil {
		t.Fatalf("GetOrSet 2: %v", err)
	}
	if string(v2) != "loaded" {
		t.Fatalf("got %q, want %q", v2, "loaded")
	}

	if n := calls.Load(); n != 1 {
		t.Fatalf("loader called %d times, want 1", n)
	}
}

func TestL1_TTLExpires(t *testing.T) {
	c := mustNewL1(t)
	ctx := t.Context()

	if err := c.Set(ctx, "ttl", []byte("temp"), 50*time.Millisecond); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	// Should be present immediately.
	_, ok, _ := c.Get(ctx, "ttl")
	if !ok {
		t.Fatal("expected hit before TTL")
	}

	// Wait for expiration. Ristretto cleanup may need a bit of extra time.
	time.Sleep(200 * time.Millisecond)

	_, ok, _ = c.Get(ctx, "ttl")
	if ok {
		t.Fatal("expected miss after TTL")
	}
}
