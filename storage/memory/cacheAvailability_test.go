package memory

import (
	"errors"
	"testing"
	"time"
)

func TestDeleteUsesRemoteBackoffAfterFailure(t *testing.T) {
	cache := Memory{
		Address:             []string{"127.0.0.1:1"},
		PoolSize:            1,
		remoteRetryInterval: time.Minute,
	}

	if err := cache.Delete("first-attempt"); err == nil {
		t.Fatal("expected first delete to fail when Redis is unavailable")
	}

	err := cache.Delete("second-attempt")
	if !errors.Is(err, ErrRemoteUnavailable) {
		t.Fatalf("expected remote unavailable error, got %v", err)
	}
}

func TestDeleteClearsLocalCacheDuringRemoteBackoff(t *testing.T) {
	cache := Memory{
		Address:             []string{"127.0.0.1:1"},
		PoolSize:            1,
		remoteRetryInterval: time.Minute,
	}

	key := "local-delete"
	// Seed via Local() — the default Memory.Set bypasses TinyLFU and would not
	// populate the local cache. Negative TTL keeps the go-redis/cache library
	// from attempting the Redis hop, so the seed succeeds even with the
	// unreachable address above.
	if err := cache.Local().Set(key, "cached-value", -1); err != nil {
		t.Fatalf("seed local cache: %v", err)
	}
	if _, ok := cache.localCache.Get(prefixed(key)); !ok {
		t.Fatal("expected seeded value in local cache")
	}

	if err := cache.Delete("open-circuit"); err == nil {
		t.Fatal("expected delete to open remote backoff")
	}
	if err := cache.Delete(key); !errors.Is(err, ErrRemoteUnavailable) {
		t.Fatalf("expected remote unavailable error, got %v", err)
	}
	if _, ok := cache.localCache.Get(prefixed(key)); ok {
		t.Fatal("expected delete to clear local cache while remote is unavailable")
	}
}
