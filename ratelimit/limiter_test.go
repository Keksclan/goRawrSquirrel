package ratelimit_test

import (
	"testing"

	"github.com/Keksclan/goRawrSquirrel/ratelimit"
)

func TestLimiter_AllowUnderLimit(t *testing.T) {
	// burst=5 means the first 5 calls must succeed.
	l := ratelimit.NewLimiter(1, 5)
	for i := range 5 {
		if !l.Allow() {
			t.Fatalf("expected Allow() == true for request %d", i)
		}
	}
}

func TestLimiter_BlocksWhenBurstExhausted(t *testing.T) {
	// burst=2, very low rps so tokens don't refill during the test.
	l := ratelimit.NewLimiter(0.001, 2)

	// Exhaust the burst.
	l.Allow()
	l.Allow()

	if l.Allow() {
		t.Fatal("expected Allow() == false after burst exhausted")
	}
}
