// Package main demonstrates the L1 caching feature of goRawrSquirrel.
// It caches a computed value and shows that the second call is served from
// cache (significantly faster).
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	gs "github.com/Keksclan/goRawrSquirrel"
)

func main() {
	srv := gs.NewServer(
		gs.WithRecovery(),
		gs.WithCacheL1(1000),
	)

	c := srv.Cache()
	ctx := context.Background()

	// loader simulates an expensive computation.
	loader := func(_ context.Context) ([]byte, error) {
		time.Sleep(50 * time.Millisecond)
		return []byte("computed-result"), nil
	}

	// First call — cache miss, loader is invoked.
	start := time.Now()
	val, err := c.GetOrSet(ctx, "demo", 10*time.Second, loader)
	firstDur := time.Since(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "first call error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("first  call: %s  (%s)\n", val, firstDur.Round(time.Millisecond))

	// Second call — cache hit, no loader invocation.
	start = time.Now()
	val, err = c.GetOrSet(ctx, "demo", 10*time.Second, loader)
	secondDur := time.Since(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "second call error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("second call: %s  (%s)\n", val, secondDur.Round(time.Millisecond))

	if secondDur >= firstDur {
		fmt.Fprintln(os.Stderr, "expected second call to be faster")
		os.Exit(1)
	}
	fmt.Println("✓ second call was faster (served from cache)")
}
