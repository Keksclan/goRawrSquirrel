package main

import (
	"context"
	"fmt"
	"time"

	gs "github.com/Keksclan/goRawrSquirrel"
)

func main() {
	logmw := func(next gs.HandlerFunc) gs.HandlerFunc {
		return func(ctx context.Context) error {
			fmt.Println("before")
			err := next(ctx)
			fmt.Println("after")
			return err
		}
	}

	timing := func(next gs.HandlerFunc) gs.HandlerFunc {
		return func(ctx context.Context) error {
			start := time.Now()
			err := next(ctx)
			fmt.Printf("took %s\n", time.Since(start))
			return err
		}
	}

	h := func(ctx context.Context) error {
		fmt.Println("handling...")
		return nil
	}

	wrapped := gs.Wrap(h, logmw, timing)
	_ = wrapped(context.Background())
}
