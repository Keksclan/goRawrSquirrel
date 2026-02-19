// Package cache provides a pluggable caching interface with an in-process
// L1 implementation backed by ristretto.
package cache

import (
	"context"
	"time"
)

// Cache is the public caching contract exposed to user logic.
type Cache interface {
	// Get retrieves a value by key. The boolean indicates a cache hit.
	Get(ctx context.Context, key string) ([]byte, bool, error)

	// Set stores a value under key with the given TTL. A zero TTL means the
	// entry has no automatic expiration.
	Set(ctx context.Context, key string, val []byte, ttl time.Duration) error

	// GetOrSet returns the cached value for key. On a cache miss it calls
	// loader exactly once, stores the result, and returns it.
	GetOrSet(ctx context.Context, key string, ttl time.Duration, loader func(context.Context) ([]byte, error)) ([]byte, error)
}
