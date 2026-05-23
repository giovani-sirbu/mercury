package memory

import (
	"context"
	"time"

	"github.com/go-redis/cache/v9"
)

// LocalView wraps Memory to expose Get / Set that go through the in-process
// TinyLFU local cache in front of Redis. Default Memory.Get / Set bypass the
// local cache; callers opt in to the local layer via Memory.Local() when the
// same data is served many times from the same process and a small staleness
// window (localCacheTTL, currently one minute) is acceptable.
//
// CRITICAL: only safe for keys with a single-process writer. Examples:
//
//   - agora-only statistics, dashboards, top-pair lists
//   - sisyphus-only backtest state
//   - hermes-only AI indicators, cooldown, inverse amounts
//
// Keys written by one service and read by another MUST use the default
// Memory.Get / Set — the TinyLFU layer has no cross-process invalidation, so
// a delete from the writer process cannot reach this process's local entry.
type LocalView struct {
	m *Memory
}

// Local returns a view that uses the local cache layer. Cheap — returns a
// thin handle, no allocation beyond the wrapper struct. Do not cache the
// returned pointer long-lived; chain at the call site:
//
//	err := cache.Local().Get(key, &val)
//	err := cache.Local().Set(key, val, ttl)
func (m *Memory) Local() *LocalView {
	return &LocalView{m: m}
}

// Get reads through the local cache layer. On a local miss it fetches from
// Redis and populates TinyLFU for subsequent reads in this process.
//
// During a remote outage, falls back to whatever local entry exists (if any),
// matching the prior Memory.Get behaviour that this view replaces. The fallback
// is what makes the local layer useful as a burst-absorption cache — a brief
// Redis blip does not break read paths whose data was already warmed.
func (v *LocalView) Get(key string, obj interface{}) error {
	v.m.init()
	cacheKey := prefixed(key)
	if err := v.m.remoteUnavailable(); err != nil {
		if v.m.localCache != nil {
			if _, ok := v.m.localCache.Get(cacheKey); ok {
				return v.m.handler.Get(context.Background(), cacheKey, obj)
			}
		}
		return err
	}
	err := v.m.handler.Get(context.Background(), cacheKey, obj)
	if err != nil {
		v.m.recordRemoteError(err)
		return err
	}
	v.m.recordRemoteSuccess()
	return nil
}

// Set writes through to Redis and populates the local cache. Pairs with
// LocalView.Get: callers reading through the local layer should also write
// through it, otherwise the very next read in the same process would miss the
// fresh entry and pay a Redis round trip every time.
func (v *LocalView) Set(key string, obj interface{}, expiration time.Duration) error {
	v.m.init()
	if err := v.m.remoteUnavailable(); err != nil {
		return err
	}
	err := v.m.handler.Set(&cache.Item{
		Ctx:   context.Background(),
		Key:   prefixed(key),
		Value: obj,
		TTL:   expiration,
	})
	if err != nil {
		v.m.recordRemoteError(err)
		return err
	}
	v.m.recordRemoteSuccess()
	return nil
}
