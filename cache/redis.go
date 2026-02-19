package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// L2 is a Redis-backed cache layer. All operations fail soft: if Redis is
// unavailable, methods return a miss (or silently discard the write) instead
// of surfacing the error to the caller.
type L2 struct {
	rdb *redis.Client
}

// NewL2 creates a new Redis-backed L2 cache.
func NewL2(addr, password string, db int) *L2 {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &L2{rdb: rdb}
}

// Get retrieves a value by key. Returns (nil, false, nil) on a miss or when
// Redis is unreachable.
func (l *L2) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := l.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		// Fail soft: treat connection errors as a miss.
		return nil, false, nil
	}
	return val, true, nil
}

// Set stores a value under key with the given TTL. A zero TTL means the entry
// has no automatic expiration. Errors are silently discarded (fail soft).
func (l *L2) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	_ = l.rdb.Set(ctx, key, val, ttl).Err()
	return nil
}

// Ping checks the Redis connection.
func (l *L2) Ping(ctx context.Context) error {
	return l.rdb.Ping(ctx).Err()
}

// Close closes the underlying Redis client.
func (l *L2) Close() error {
	return l.rdb.Close()
}
