package retry

import (
	"context"
	"slices"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Config controls the retry behaviour of [Do].
type Config struct {
	// MaxAttempts is the maximum number of times fn is called (including the
	// first attempt). Values ≤ 1 mean no retries.
	MaxAttempts int

	// BaseDelay is the delay before the first retry. Subsequent retries use
	// exponential back-off: BaseDelay * 2^attempt.
	BaseDelay time.Duration

	// MaxDelay caps the computed back-off delay.
	MaxDelay time.Duration

	// Jitter adds randomness to the delay. A value of 0.2 means ±20 % of
	// the computed delay. Zero disables jitter.
	Jitter float64

	// RetryCodes lists the gRPC status codes that are considered retryable.
	// An empty list means no error is retried.
	RetryCodes []codes.Code
}

// Do calls fn up to cfg.MaxAttempts times, retrying only when the returned
// error carries a gRPC status code listed in cfg.RetryCodes. Between
// attempts an exponential back-off delay (with optional jitter) is applied.
//
// The context is checked before every retry; if ctx is done the function
// returns immediately with the context error.
func Do[T any](ctx context.Context, cfg Config, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	attempts := max(cfg.MaxAttempts, 1)

	for i := range attempts {
		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}

		// Last attempt — return immediately regardless of code.
		if i == attempts-1 {
			return zero, err
		}

		// Check whether the error code is retryable.
		if st, ok := status.FromError(err); !ok || !slices.Contains(cfg.RetryCodes, st.Code()) {
			return zero, err
		}

		// Wait with back-off, but respect context cancellation.
		delay := backoff(cfg, i)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, ctx.Err()
		case <-timer.C:
		}
	}

	// Unreachable, but keeps the compiler happy.
	return zero, nil
}
