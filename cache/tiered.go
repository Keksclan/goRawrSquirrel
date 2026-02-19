package cache

import (
	"bytes"
	"context"
	"sync"
	"time"
)

// Tiered combines an L1 (in-process) and L2 (Redis) cache. Reads check L1
// first, then L2, then the loader. Writes populate both layers.
type Tiered struct {
	l1 *L1
	l2 *L2

	mu    sync.Mutex
	loads map[string]*call
}

// NewTiered creates a two-level cache.
func NewTiered(l1 *L1, l2 *L2) *Tiered {
	return &Tiered{
		l1:    l1,
		l2:    l2,
		loads: make(map[string]*call),
	}
}

// Get checks L1, then L2. On an L2 hit the value is promoted into L1 (with
// zero TTL since we don't know the original TTL).
func (t *Tiered) Get(ctx context.Context, key string) ([]byte, bool, error) {
	// L1
	if v, ok, err := t.l1.Get(ctx, key); err != nil || ok {
		return v, ok, err
	}
	// L2
	v, ok, err := t.l2.Get(ctx, key)
	if err != nil || !ok {
		return nil, false, err
	}
	// Promote to L1.
	_ = t.l1.Set(ctx, key, v, 0)
	return v, true, nil
}

// Set writes the value to both L2 and L1.
func (t *Tiered) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	_ = t.l2.Set(ctx, key, val, ttl)
	return t.l1.Set(ctx, key, val, ttl)
}

// GetOrSet follows the L1 → L2 → loader pattern, deduplicating concurrent
// loads for the same key.
func (t *Tiered) GetOrSet(ctx context.Context, key string, ttl time.Duration, loader func(context.Context) ([]byte, error)) ([]byte, error) {
	// 1. Check L1.
	if v, ok, _ := t.l1.Get(ctx, key); ok {
		return v, nil
	}

	// 2. Check L2. On hit, promote to L1.
	if v, ok, _ := t.l2.Get(ctx, key); ok {
		_ = t.l1.Set(ctx, key, v, ttl)
		return bytes.Clone(v), nil
	}

	// 3. Singleflight loader.
	t.mu.Lock()
	if c, ok := t.loads[key]; ok {
		t.mu.Unlock()
		c.wg.Wait()
		if c.err != nil {
			return nil, c.err
		}
		return bytes.Clone(c.val), nil
	}

	c := &call{}
	c.wg.Add(1)
	t.loads[key] = c
	t.mu.Unlock()

	c.val, c.err = loader(ctx)
	if c.err == nil {
		// 4. Store in L2, then L1.
		_ = t.l2.Set(ctx, key, c.val, ttl)
		_ = t.l1.Set(ctx, key, c.val, ttl)
	}
	c.wg.Done()

	t.mu.Lock()
	delete(t.loads, key)
	t.mu.Unlock()

	if c.err != nil {
		return nil, c.err
	}
	return bytes.Clone(c.val), nil
}
