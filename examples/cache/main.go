// Package main demonstrates the caching feature of goRawrSquirrel.
// When REDIS_ADDR is set it enables a two-level cache (L1 + Redis L2);
// otherwise only the in-process L1 cache is used.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	gs "github.com/Keksclan/goRawrSquirrel"
)

func main() {
	opts := []gs.Option{
		gs.WithRecovery(),
		gs.WithCacheL1(1000),
	}

	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		fmt.Printf("Redis L2 enabled (%s)\n", addr)
		opts = append(opts, gs.WithCacheRedis(addr, "", 0))
	} else {
		fmt.Println("Redis L2 not configured (set REDIS_ADDR to enable)")
	}

	srv := gs.NewServer(opts...)

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
