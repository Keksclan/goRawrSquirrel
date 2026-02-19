// Package ratelimit provides a token-bucket rate limiter backed by
// golang.org/x/time/rate for use as a global gRPC request gate.
package ratelimit

import "golang.org/x/time/rate"

// Limiter wraps a token-bucket limiter that decides whether an incoming
// request should be allowed.
type Limiter struct {
	lim *rate.Limiter
}

// NewLimiter creates a Limiter that permits rps requests per second with the
// given burst size.
func NewLimiter(rps float64, burst int) *Limiter {
	return &Limiter{lim: rate.NewLimiter(rate.Limit(rps), burst)}
}

// Allow reports whether a single request may proceed.
func (l *Limiter) Allow() bool {
	return l.lim.Allow()
}
