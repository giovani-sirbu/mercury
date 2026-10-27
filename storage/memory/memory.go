package memory

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
)

// Memory is a Redis-backed key-value store. The default Get / Set go straight
// to Redis and never touch the in-process TinyLFU layer: that layer has no
// cross-process invalidation, so a key another service writes would be served
// stale for up to localCacheTTL. Local() opts into it for keys with a single
// in-process writer. Hold a *Memory: the client and the local cache are a
// singleton initialised once per process.
type Memory struct {
	Address  []string
	Password string
	User     string
	PoolSize int

	once       sync.Once
	handler    *cache.Cache
	localCache cache.LocalCache
	client     redis.UniversalClient

	remoteMu               sync.RWMutex
	remoteUnavailableUntil time.Time
	remoteUnavailableErr   error
	remoteRetryInterval    time.Duration
}

// localCacheCapacity bounds the TinyLFU front.
const localCacheCapacity = 50_000

// localCacheTTL caps how long a locally cached value is served without re-checking Redis.
const localCacheTTL = time.Minute

func (m *Memory) init() {
	m.once.Do(func() {
		localCache := cache.NewTinyLFU(localCacheCapacity, localCacheTTL)
		// Protocol 2 pins RESP2, which the deployed Redis/Dragonfly speaks.
		m.client = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:    m.Address,
			Password: m.Password,
			Username: m.User,
			PoolSize: m.PoolSize,
			Protocol: 2,
		})
		m.localCache = localCache
		m.handler = cache.New(&cache.Options{
			Redis:      m.client,
			LocalCache: localCache,
		})
	})
}

func prefixed(key string) string {
	return fmt.Sprintf("%s%s", os.Getenv("REDIS_PREFIX"), key)
}

// Set stores obj under key with the given TTL, straight to Redis.
func (m *Memory) Set(key string, obj interface{}, expiration time.Duration) error {
	m.init()
	if err := m.remoteUnavailable(); err != nil {
		return err
	}
	err := m.handler.Set(&cache.Item{
		Ctx:            context.Background(),
		Key:            prefixed(key),
		Value:          obj,
		TTL:            expiration,
		SkipLocalCache: true,
	})
	if err != nil {
		m.recordRemoteError(err)
		return err
	}
	m.recordRemoteSuccess()
	return nil
}

// Exists checks key presence against Redis; false on a remote outage (fail open).
func (m *Memory) Exists(key string) (bool, error) {
	m.init()
	if err := m.remoteUnavailable(); err != nil {
		return false, err
	}
	n, err := m.client.Exists(context.Background(), prefixed(key)).Result()
	if err != nil {
		m.recordRemoteError(err)
		return false, err
	}
	m.recordRemoteSuccess()
	return n > 0, nil
}

// SetNX atomically sets key if absent, a distributed mutex: (true, nil) means
// this caller acquired it, (false, nil) that another holds it, (false, err) a
// transport error. It fails closed on a remote outage: the caller must not
// run the protected action.
func (m *Memory) SetNX(key string, expiration time.Duration) (bool, error) {
	m.init()
	if err := m.remoteUnavailable(); err != nil {
		return false, err
	}
	acquired, err := m.client.SetNX(context.Background(), prefixed(key), "1", expiration).Result()
	if err != nil {
		m.recordRemoteError(err)
		return false, err
	}
	m.recordRemoteSuccess()
	return acquired, nil
}

// Get decodes the value at key into obj, straight to Redis. obj must already
// be a pointer: passing &obj yields *interface{}, which msgpack cannot set.
func (m *Memory) Get(key string, obj interface{}) error {
	m.init()
	if err := m.remoteUnavailable(); err != nil {
		return err
	}
	err := m.handler.GetSkippingLocalCache(context.Background(), prefixed(key), obj)
	if err != nil {
		m.recordRemoteError(err)
		return err
	}
	m.recordRemoteSuccess()
	return nil
}

// Delete removes key from both the local and the remote cache.
func (m *Memory) Delete(key string) error {
	m.init()
	cacheKey := prefixed(key)
	if err := m.remoteUnavailable(); err != nil {
		m.handler.DeleteFromLocalCache(cacheKey)
		return err
	}
	err := m.handler.Delete(context.Background(), cacheKey)
	if err != nil {
		m.recordRemoteError(err)
		return err
	}
	m.recordRemoteSuccess()
	return nil
}

// DeleteByPattern removes every key matching keyPattern; it scans the keyspace, so use sparingly.
func (m *Memory) DeleteByPattern(keyPattern string) error {
	m.init()
	if err := m.remoteUnavailable(); err != nil {
		return err
	}
	ctx := context.Background()
	keys, err := m.client.Keys(ctx, prefixed(keyPattern)).Result()
	if err != nil {
		m.recordRemoteError(err)
		return err
	}
	if len(keys) == 0 {
		m.recordRemoteSuccess()
		return nil
	}
	err = m.client.Del(ctx, keys...).Err()
	if err != nil {
		m.recordRemoteError(err)
		return err
	}
	m.recordRemoteSuccess()
	return nil
}

// Ping checks connectivity to the remote cache.
func (m *Memory) Ping() error {
	m.init()
	err := m.client.Ping(context.Background()).Err()
	if err != nil {
		m.recordRemoteError(err)
		return err
	}
	m.recordRemoteSuccess()
	return nil
}

// Close releases the Redis client.
func (m *Memory) Close() error {
	if m.client == nil {
		return nil
	}
	return m.client.Close()
}
