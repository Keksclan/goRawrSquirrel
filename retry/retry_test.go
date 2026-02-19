package retry

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDo_RetriesOnUnavailableThenSucceeds(t *testing.T) {
	calls := 0
	cfg := Config{
		MaxAttempts: 4,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		RetryCodes:  []codes.Code{codes.Unavailable},
	}

	result, err := Do(t.Context(), cfg, func(_ context.Context) (string, error) {
		calls++
		if calls < 3 {
			return "", status.Error(codes.Unavailable, "try again")
		}
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected %q, got %q", "ok", result)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDo_StopsOnNonRetryableCode(t *testing.T) {
	calls := 0
	cfg := Config{
		MaxAttempts: 5,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		RetryCodes:  []codes.Code{codes.Unavailable},
	}

	_, err := Do(t.Context(), cfg, func(_ context.Context) (string, error) {
		calls++
		return "", status.Error(codes.InvalidArgument, "bad request")
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
	if calls != 1 {
		t.Fatalf("expected 1 call (no retries), got %d", calls)
	}
}

func TestDo_RespectsContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()

	cfg := Config{
		MaxAttempts: 100,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		RetryCodes:  []codes.Code{codes.Unavailable},
	}

	_, err := Do(ctx, cfg, func(_ context.Context) (int, error) {
		return 0, status.Error(codes.Unavailable, "down")
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

func TestDo_MaxAttemptsExhausted(t *testing.T) {
	calls := 0
	cfg := Config{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		RetryCodes:  []codes.Code{codes.Unavailable},
	}

	_, err := Do(t.Context(), cfg, func(_ context.Context) (string, error) {
		calls++
		return "", status.Error(codes.Unavailable, "still down")
	})

	if err == nil {
		t.Fatal("expected error after exhausting attempts")
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDo_SucceedsOnFirstAttempt(t *testing.T) {
	cfg := Config{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		RetryCodes:  []codes.Code{codes.Unavailable},
	}

	result, err := Do(t.Context(), cfg, func(_ context.Context) (int, error) {
		return 42, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Fatalf("expected 42, got %d", result)
	}
}

func TestBackoff_ExponentialWithCap(t *testing.T) {
	cfg := Config{
		BaseDelay: 100 * time.Millisecond,
		MaxDelay:  500 * time.Millisecond,
	}

	d0 := backoff(cfg, 0) // 100ms
	d1 := backoff(cfg, 1) // 200ms
	d2 := backoff(cfg, 2) // 400ms
	d3 := backoff(cfg, 3) // 800ms → capped at 500ms

	if d0 != 100*time.Millisecond {
		t.Fatalf("attempt 0: expected 100ms, got %v", d0)
	}
	if d1 != 200*time.Millisecond {
		t.Fatalf("attempt 1: expected 200ms, got %v", d1)
	}
	if d2 != 400*time.Millisecond {
		t.Fatalf("attempt 2: expected 400ms, got %v", d2)
	}
	if d3 != 500*time.Millisecond {
		t.Fatalf("attempt 3: expected 500ms (capped), got %v", d3)
	}
}
