// Package breaker provides a minimal, thread-safe circuit breaker.
//
// States:
//   - Closed: requests flow normally; failures are counted.
//   - Open: requests are blocked; after OpenTimeout the breaker transitions to HalfOpen.
//   - HalfOpen: a limited number of probe requests are allowed through;
//     if all succeed the breaker closes, any failure reopens it.
package breaker

import (
	"sync"
	"time"
)

// State represents the current circuit breaker state.
type State int

const (
	Closed State = iota
	Open
	HalfOpen
)

// Config holds the circuit breaker parameters.
type Config struct {
	// FailureThreshold is the number of consecutive failures in Closed state
	// before the breaker trips to Open.
	FailureThreshold int

	// OpenTimeout is how long the breaker stays Open before transitioning
	// to HalfOpen.
	OpenTimeout time.Duration

	// HalfOpenMaxSuccess is the number of consecutive successes required in
	// HalfOpen state to close the breaker again.
	HalfOpenMaxSuccess int
}

// Breaker is a minimal circuit breaker. All methods are safe for concurrent use.
type Breaker struct {
	mu sync.Mutex

	cfg Config

	state     State
	failures  int // consecutive failures in Closed
	successes int // consecutive successes in HalfOpen
	openedAt  time.Time
	nowFunc   func() time.Time // for testing; defaults to time.Now
}

// New creates a Breaker with the given configuration.
func New(cfg Config) *Breaker {
	return &Breaker{
		cfg:     cfg,
		state:   Closed,
		nowFunc: time.Now,
	}
}

// State returns the current state of the breaker. In Open state it may
// auto-transition to HalfOpen if the timeout has elapsed.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.checkOpenTimeout()
	return b.state
}

// Allow reports whether a request is allowed through. It returns true when the
// breaker is Closed, or HalfOpen with remaining probe slots. It returns false
// when the breaker is Open (and the timeout has not yet elapsed).
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.checkOpenTimeout()

	switch b.state {
	case Closed:
		return true
	case HalfOpen:
		return b.successes < b.cfg.HalfOpenMaxSuccess
	default: // Open
		return false
	}
}

// OnSuccess records a successful request.
func (b *Breaker) OnSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Closed:
		b.failures = 0
	case HalfOpen:
		b.successes++
		if b.successes >= b.cfg.HalfOpenMaxSuccess {
			b.state = Closed
			b.failures = 0
			b.successes = 0
		}
	}
}

// OnFailure records a failed request.
func (b *Breaker) OnFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Closed:
		b.failures++
		if b.failures >= b.cfg.FailureThreshold {
			b.toOpen()
		}
	case HalfOpen:
		b.toOpen()
	}
}

// checkOpenTimeout transitions from Open to HalfOpen when the timeout has
// elapsed. Must be called with b.mu held.
func (b *Breaker) checkOpenTimeout() {
	if b.state == Open && b.now().Sub(b.openedAt) >= b.cfg.OpenTimeout {
		b.state = HalfOpen
		b.successes = 0
	}
}

func (b *Breaker) toOpen() {
	b.state = Open
	b.openedAt = b.now()
	b.successes = 0
}

func (b *Breaker) now() time.Time {
	if b.nowFunc != nil {
		return b.nowFunc()
	}
	return time.Now()
}
