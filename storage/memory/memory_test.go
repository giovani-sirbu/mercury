package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/cache/v9"
)

// TestGetSkipsLocalCache pins the contract that Memory.Get goes straight to
// Redis without consulting TinyLFU. Set up: seed via Local().Set so both
// layers hold the value; then delete only the Redis copy via the raw client
// (simulating another process's invalidation). Memory.Get must miss.
//
// If this test ever passes when Memory.Get is supposed to bypass local but
// secretly serves from local, the trade duplicate-buy bug is back.
func TestGetSkipsLocalCache(t *testing.T) {
	addr := redisAvailable(t)
	m := Memory{Address: []string{addr}, PoolSize: 1}

	key := "test:get-skips-local"
	t.Cleanup(func() {
		_ = m.client.Del(context.Background(), prefixed(key)).Err()
	})

	if err := m.Local().Set(key, "seeded", time.Minute); err != nil {
		t.Fatalf("Local().Set: %v", err)
	}

	// Simulate another process clearing the Redis key. The local TinyLFU in
	// THIS process is intentionally left populated.
	if err := m.client.Del(context.Background(), prefixed(key)).Err(); err != nil {
		t.Fatalf("external Del: %v", err)
	}

	var out string
	err := m.Get(key, &out)
	if err == nil {
		t.Fatalf("expected Memory.Get to miss after Redis del; got value=%q (local cache was consulted)", out)
	}
	if !errors.Is(err, cache.ErrCacheMiss) {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}
}

// TestSetDoesNotPopulateLocalCache pins the contract that Memory.Set writes
// only to Redis and leaves TinyLFU untouched. Set up: write via Memory.Set,
// then delete only the Redis copy; Local().Get must miss because the local
// layer was never seeded.
func TestSetDoesNotPopulateLocalCache(t *testing.T) {
	addr := redisAvailable(t)
	m := Memory{Address: []string{addr}, PoolSize: 1}

	key := "test:set-skips-local"
	t.Cleanup(func() {
		_ = m.client.Del(context.Background(), prefixed(key)).Err()
	})

	if err := m.Set(key, "default-set", time.Minute); err != nil {
		t.Fatalf("Memory.Set: %v", err)
	}

	if err := m.client.Del(context.Background(), prefixed(key)).Err(); err != nil {
		t.Fatalf("external Del: %v", err)
	}

	var out string
	err := m.Local().Get(key, &out)
	if err == nil {
		t.Fatalf("expected Local().Get to miss (Memory.Set should not populate local); got value=%q", out)
	}
	if !errors.Is(err, cache.ErrCacheMiss) {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}
}

// TestLocalViewGetPopulatesLocal pins the opt-in path: when callers explicitly
// use Memory.Local().Set, the local cache IS populated and survives a Redis-
// only deletion. This is the property that makes the opt-in useful — single-
// writer read-heavy keys can absorb burst reads from in-process memory.
func TestLocalViewGetPopulatesLocal(t *testing.T) {
	addr := redisAvailable(t)
	m := Memory{Address: []string{addr}, PoolSize: 1}

	key := "test:local-get-populates"
	t.Cleanup(func() {
		_ = m.client.Del(context.Background(), prefixed(key)).Err()
	})

	if err := m.Local().Set(key, "local-set", time.Minute); err != nil {
		t.Fatalf("Local().Set: %v", err)
	}

	if err := m.client.Del(context.Background(), prefixed(key)).Err(); err != nil {
		t.Fatalf("external Del: %v", err)
	}

	var out string
	if err := m.Local().Get(key, &out); err != nil {
		t.Fatalf("expected Local().Get hit from TinyLFU after Redis del; got err: %v", err)
	}
	if out != "local-set" {
		t.Fatalf("expected %q, got %q", "local-set", out)
	}
}
