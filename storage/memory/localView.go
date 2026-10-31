package memory

import (
	"context"
	"time"

	"github.com/go-redis/cache/v9"
)

// LocalView wraps Memory with Get / Set that go through the in-process
// TinyLFU layer in front of Redis, for keys with a single-process writer
// served many times from the same process under a localCacheTTL staleness
// window. A key another service writes must use the default Memory.Get / Set:
// the local layer has no cross-process invalidation.
type LocalView struct {
	m *Memory
}

// Local returns the view that uses the local cache layer; chain it at the call site.
func (m *Memory) Local() *LocalView {
	return &LocalView{m: m}
}

// Get reads through the local cache, filling it from Redis on a miss; during
// a remote outage a warm local entry is still served.
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

// Set writes through to Redis and the local cache; pair it with LocalView.Get.
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
