package cache

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

// L1 is an in-process cache backed by ristretto.
type L1 struct {
	rc *ristretto.Cache[string, []byte]

	mu    sync.Mutex
	loads map[string]*call
}

// call deduplicates concurrent loads for the same key.
type call struct {
	wg  sync.WaitGroup
	val []byte
	err error
}

// NewL1 creates a new L1 cache. maxCost controls the maximum cost the cache
// can hold (each entry has a cost of 1).
func NewL1(maxCost int64) (*L1, error) {
	rc, err := ristretto.NewCache(&ristretto.Config[string, []byte]{
		NumCounters: maxCost * 10,
		MaxCost:     maxCost,
		BufferItems: 64,
	})
	if err != nil {
		return nil, err
	}
	return &L1{
		rc:    rc,
		loads: make(map[string]*call),
	}, nil
}

// Get retrieves a value by key.
func (l *L1) Get(_ context.Context, key string) ([]byte, bool, error) {
	v, ok := l.rc.Get(key)
	if !ok {
		return nil, false, nil
	}
	return bytes.Clone(v), true, nil
}

// Set stores a value under key with the given TTL.
func (l *L1) Set(_ context.Context, key string, val []byte, ttl time.Duration) error {
	l.rc.SetWithTTL(key, bytes.Clone(val), 1, ttl)
	l.rc.Wait()
	return nil
}

// GetOrSet returns the cached value for key. On a miss it calls loader once
// (deduplicating concurrent callers for the same key), stores the result, and
// returns it.
func (l *L1) GetOrSet(ctx context.Context, key string, ttl time.Duration, loader func(context.Context) ([]byte, error)) ([]byte, error) {
	if v, ok, _ := l.Get(ctx, key); ok {
		return v, nil
	}

	l.mu.Lock()
	if c, ok := l.loads[key]; ok {
		l.mu.Unlock()
		c.wg.Wait()
		if c.err != nil {
			return nil, c.err
		}
		return bytes.Clone(c.val), nil
	}

	c := &call{}
	c.wg.Add(1)
	l.loads[key] = c
	l.mu.Unlock()

	c.val, c.err = loader(ctx)
	if c.err == nil {
		_ = l.Set(ctx, key, c.val, ttl)
	}
	c.wg.Done()

	l.mu.Lock()
	delete(l.loads, key)
	l.mu.Unlock()

	if c.err != nil {
		return nil, c.err
	}
	return bytes.Clone(c.val), nil
}
