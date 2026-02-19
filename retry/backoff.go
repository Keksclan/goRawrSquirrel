// Package retry provides a generic retry helper with exponential backoff and
// jitter for use around client-side gRPC invocations. It is completely
// optional and never installed automatically inside server interceptors.
package retry

import (
	"math"
	"math/rand"
	"time"
)

// backoff returns the delay for the given attempt (0-indexed) according to
// exponential back-off with optional jitter. The returned duration is capped
// at cfg.MaxDelay.
func backoff(cfg Config, attempt int) time.Duration {
	delay := float64(cfg.BaseDelay) * math.Pow(2, float64(attempt))
	if max := float64(cfg.MaxDelay); delay > max {
		delay = max
	}
	if cfg.Jitter > 0 {
		// jitter adds up to ±Jitter fraction of the delay.
		delay += delay * cfg.Jitter * (rand.Float64()*2 - 1)
	}
	if delay < 0 {
		delay = 0
	}
	return time.Duration(delay)
}
