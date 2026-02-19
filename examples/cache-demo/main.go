// Package main demonstrates goRawrSquirrel's in-process L1 cache.
//
// It creates a server with an L1 cache, stores a value via a simulated
// expensive loader, and then retrieves it again — showing that the second
// call is served from cache and is significantly faster.
//
// Run:
//
//	go run ./examples/cache-demo
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	gs "github.com/Keksclan/goRawrSquirrel"
)

func main() {
	fmt.Println("cache-demo: demonstrates goRawrSquirrel L1 in-process cache")
	fmt.Println()

	// 1. Build a server with an L1 cache (max 1 000 entries).
	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithCacheL1(1_000),
	)

	c := srv.Cache()
	if c == nil {
		fmt.Fprintln(os.Stderr, "cache is nil — WithCacheL1 did not initialise the cache")
		os.Exit(1)
	}

	ctx := context.Background()

	// loader simulates an expensive computation that takes 50 ms.
	loader := func(_ context.Context) ([]byte, error) {
		fmt.Println("  → loader: computing value (sleeping 50 ms)…")
		time.Sleep(50 * time.Millisecond)
		return []byte("expensive-result"), nil
	}

	// 2. First call — cache miss, loader is invoked.
	fmt.Println("── first call (cache miss) ──")
	start := time.Now()
	val, err := c.GetOrSet(ctx, "demo-key", 10*time.Second, loader)
	firstDur := time.Since(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "first call error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  result: %s  (took %s)\n", val, firstDur.Round(time.Millisecond))

	// 3. Second call — cache hit, loader is NOT invoked.
	fmt.Println()
	fmt.Println("── second call (cache hit) ──")
	start = time.Now()
	val, err = c.GetOrSet(ctx, "demo-key", 10*time.Second, loader)
	secondDur := time.Since(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "second call error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  result: %s  (took %s)\n", val, secondDur.Round(time.Millisecond))

	// 4. Verify the cache hit was faster.
	if secondDur >= firstDur {
		fmt.Fprintln(os.Stderr, "expected second call to be faster than the first")
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("✓ cache-demo complete — second call served from L1 cache, no loader invoked")
}
