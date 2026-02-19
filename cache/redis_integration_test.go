package cache

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func redisL2(t *testing.T) *L2 {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set, skipping Redis integration test")
	}
	l2 := NewL2(addr, "", 0)
	t.Cleanup(func() { _ = l2.Close() })
	if err := l2.Ping(t.Context()); err != nil {
		t.Fatalf("cannot reach Redis at %s: %v", addr, err)
	}
	return l2
}

func TestL2_GetSet(t *testing.T) {
	l2 := redisL2(t)
	ctx := t.Context()

	key := "test:l2:getset:" + t.Name()

	// Miss returns false.
	_, ok, err := l2.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if ok {
		t.Fatal("expected miss")
	}

	// Set then Get.
	if err := l2.Set(ctx, key, []byte("v1"), 10*time.Second); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	val, ok, err := l2.Get(ctx, key)
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

func TestTiered_L1_L2_Loader(t *testing.T) {
	l2 := redisL2(t)
	l1 := mustNewL1(t)
	tc := NewTiered(l1, l2)
	ctx := t.Context()

	key := "test:tiered:" + t.Name()

	var calls atomic.Int32
	loader := func(_ context.Context) ([]byte, error) {
		calls.Add(1)
		return []byte("from-loader"), nil
	}

	// First call — loader invoked, stored in L1 and L2.
	v, err := tc.GetOrSet(ctx, key, 30*time.Second, loader)
	if err != nil {
		t.Fatalf("GetOrSet 1: %v", err)
	}
	if string(v) != "from-loader" {
		t.Fatalf("got %q, want %q", v, "from-loader")
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("loader called %d times, want 1", n)
	}

	// Second call — served from L1, loader not called.
	v, err = tc.GetOrSet(ctx, key, 30*time.Second, loader)
	if err != nil {
		t.Fatalf("GetOrSet 2: %v", err)
	}
	if string(v) != "from-loader" {
		t.Fatalf("got %q, want %q", v, "from-loader")
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("loader called %d times, want 1", n)
	}

	// Evict L1, value should come from L2.
	l1Fresh := mustNewL1(t)
	tc2 := NewTiered(l1Fresh, l2)

	v, err = tc2.GetOrSet(ctx, key, 30*time.Second, loader)
	if err != nil {
		t.Fatalf("GetOrSet 3 (L2 hit): %v", err)
	}
	if string(v) != "from-loader" {
		t.Fatalf("got %q, want %q", v, "from-loader")
	}
	// Loader still called only once.
	if n := calls.Load(); n != 1 {
		t.Fatalf("loader called %d times, want 1", n)
	}
}

func TestL2_FailSoft(t *testing.T) {
	// Connect to a bogus address — operations must not panic or return errors.
	l2 := NewL2("localhost:1", "", 0)
	t.Cleanup(func() { _ = l2.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	_, ok, err := l2.Get(ctx, "no-such-key")
	if err != nil {
		t.Fatalf("expected nil error on unreachable Redis, got: %v", err)
	}
	if ok {
		t.Fatal("expected miss")
	}

	if err := l2.Set(ctx, "k", []byte("v"), time.Second); err != nil {
		t.Fatalf("expected nil error on unreachable Redis, got: %v", err)
	}
}
