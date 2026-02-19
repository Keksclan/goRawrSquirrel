package breaker

import (
	"testing"
	"time"
)

func newTestBreaker(cfg Config) (*Breaker, *time.Time) {
	b := New(cfg)
	now := time.Now()
	b.nowFunc = func() time.Time { return now }
	return b, &now
}

func TestClosedToOpen(t *testing.T) {
	b, _ := newTestBreaker(Config{
		FailureThreshold:   3,
		OpenTimeout:        5 * time.Second,
		HalfOpenMaxSuccess: 1,
	})

	if s := b.State(); s != Closed {
		t.Fatalf("expected Closed, got %d", s)
	}

	b.OnFailure()
	b.OnFailure()
	if s := b.State(); s != Closed {
		t.Fatalf("expected Closed after 2 failures, got %d", s)
	}

	b.OnFailure() // 3rd failure => trip
	if s := b.State(); s != Open {
		t.Fatalf("expected Open after 3 failures, got %d", s)
	}
}

func TestOpenBlocks(t *testing.T) {
	b, _ := newTestBreaker(Config{
		FailureThreshold:   1,
		OpenTimeout:        5 * time.Second,
		HalfOpenMaxSuccess: 1,
	})

	b.OnFailure() // trip
	if b.Allow() {
		t.Fatal("expected Allow()=false in Open state")
	}
}

func TestOpenToHalfOpenAfterTimeout(t *testing.T) {
	b, now := newTestBreaker(Config{
		FailureThreshold:   1,
		OpenTimeout:        5 * time.Second,
		HalfOpenMaxSuccess: 2,
	})

	b.OnFailure() // trip to Open
	if b.Allow() {
		t.Fatal("expected blocked in Open")
	}

	// Advance time past OpenTimeout
	*now = now.Add(6 * time.Second)

	if s := b.State(); s != HalfOpen {
		t.Fatalf("expected HalfOpen after timeout, got %d", s)
	}
	if !b.Allow() {
		t.Fatal("expected Allow()=true in HalfOpen")
	}
}

func TestHalfOpenSuccessToClosed(t *testing.T) {
	b, now := newTestBreaker(Config{
		FailureThreshold:   1,
		OpenTimeout:        5 * time.Second,
		HalfOpenMaxSuccess: 2,
	})

	b.OnFailure()
	*now = now.Add(6 * time.Second)

	// Now in HalfOpen
	if s := b.State(); s != HalfOpen {
		t.Fatalf("expected HalfOpen, got %d", s)
	}

	b.OnSuccess()
	if s := b.State(); s != HalfOpen {
		t.Fatalf("expected still HalfOpen after 1 success, got %d", s)
	}

	b.OnSuccess() // 2nd success => close
	if s := b.State(); s != Closed {
		t.Fatalf("expected Closed after %d successes, got %d", 2, s)
	}
}

func TestHalfOpenFailureToOpen(t *testing.T) {
	b, now := newTestBreaker(Config{
		FailureThreshold:   1,
		OpenTimeout:        5 * time.Second,
		HalfOpenMaxSuccess: 3,
	})

	b.OnFailure()
	*now = now.Add(6 * time.Second)

	if s := b.State(); s != HalfOpen {
		t.Fatalf("expected HalfOpen, got %d", s)
	}

	b.OnFailure() // any failure in HalfOpen => Open
	if s := b.State(); s != Open {
		t.Fatalf("expected Open after HalfOpen failure, got %d", s)
	}
}

func TestSuccessResetsFailureCount(t *testing.T) {
	b, _ := newTestBreaker(Config{
		FailureThreshold:   3,
		OpenTimeout:        5 * time.Second,
		HalfOpenMaxSuccess: 1,
	})

	b.OnFailure()
	b.OnFailure()
	b.OnSuccess() // resets count
	b.OnFailure()
	b.OnFailure()
	// Only 2 consecutive failures after reset, should still be Closed
	if s := b.State(); s != Closed {
		t.Fatalf("expected Closed, got %d", s)
	}
}

func TestHalfOpenProbeLimit(t *testing.T) {
	b, now := newTestBreaker(Config{
		FailureThreshold:   1,
		OpenTimeout:        5 * time.Second,
		HalfOpenMaxSuccess: 2,
	})

	b.OnFailure()
	*now = now.Add(6 * time.Second)

	// HalfOpen allows up to HalfOpenMaxSuccess probes
	if !b.Allow() {
		t.Fatal("expected first probe allowed")
	}
	b.OnSuccess() // successes=1

	if !b.Allow() {
		t.Fatal("expected second probe allowed")
	}
	// After HalfOpenMaxSuccess successes recorded, further Allow should
	// still work because we haven't called OnSuccess yet for the second probe.
	// But once we do:
	b.OnSuccess() // successes=2 => transitions to Closed

	if s := b.State(); s != Closed {
		t.Fatalf("expected Closed, got %d", s)
	}
}
