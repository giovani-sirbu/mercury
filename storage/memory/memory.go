package memory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
)

// Memory is a Redis-backed key-value store.
//
// The default Get / Set methods go straight to Redis — they do NOT touch the
// in-process TinyLFU local cache. This is the safe default for any key that
// might be written by another service, because TinyLFU has no cross-process
// invalidation and would otherwise serve stale values for up to localCacheTTL.
//
// Callers that want the local layer (single-process writer, read-heavy hot
// path, staleness tolerable) opt in explicitly via Memory.Local(), which
// returns a view whose Get / Set go through TinyLFU.
//
// The struct is configured once at service bootstrap (server.NewServer) and
// reused for the lifetime of the process. Callers must hold a *Memory — the
// pointer receiver is required because the singleton state (sync.Once + shared
// client) must be observed across method calls.
//
// Prior versions used a value receiver and called Init() inside every
// Set/Get/Delete. Each call spun up a new Redis UniversalClient plus a fresh
// TinyLFU, so the local cache was effectively dead and every operation paid
// connection-handshake cost (measured at ~3.5ms/op on master). The singleton
// keeps the client pool and the local cache alive for the lifetime of the
// process.
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

// localCacheCapacity bounds the TinyLFU front. 50k is sized for the platform's
// active-trades-ids + per-trade + per-symbol fan-out (each pair owns several
// keys); the prior 10k saturated and churned under realistic load.
const localCacheCapacity = 50_000

// localCacheTTL caps how long a locally-cached value can be served without
// re-checking Redis. Short enough that control-plane changes (lock release,
// status flip) propagate within a tick; long enough to absorb burst reads.
const localCacheTTL = time.Minute

func (m *Memory) init() {
	m.once.Do(func() {
		localCache := cache.NewTinyLFU(localCacheCapacity, localCacheTTL)
		// Protocol: 2 pins RESP2 (master's fix) — go-redis v9 defaults to
		// RESP3, which the deployed Redis/Dragonfly didn't speak cleanly.
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

// Set stores obj under key with the given TTL. Writes go straight to Redis —
// the in-process TinyLFU local cache is NOT populated.
//
// Correctness-first default: the local cache has no cross-process invalidation,
// so writing through it would let one service's stale local entry shadow a
// fresh remote value written by another service. The previous behaviour caused
// the trade duplicate-buy bug (hermes kept reading PositionType="new" from its
// local cache after agora had written "buy" to Redis).
//
// Opt in to the local layer via Memory.Local() only for keys with a known
// single-process writer where serving the same value many times in a row is
// worth a small staleness window.
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

// Exists checks key presence directly against Redis, bypassing the TinyLFU
// local cache. Use this for control-plane keys (trade locks, etc.) where a
// stale local-cache hit (Cache.Get reading a 60s-old "locked" value after
// the source-of-truth was deleted) would cause the caller to skip work that
// should run. Returns true when the key exists, false when missing or on
// remote outage (fail-open is correct here — a temporary Redis blip should
// not appear as a permanent lock).
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

// SetNX atomically sets key only if it does not already exist. Returns
// (true, nil) if this caller acquired the key, (false, nil) if another caller
// already holds it, or (false, err) on transport error.
//
// Used as a distributed mutex: bypasses the TinyLFU local cache (a stale local
// hit would falsely report the key as free) and goes straight to Redis SETNX.
// Fails closed on remote outage — callers must treat (false, err) as "did not
// acquire" and not proceed with the protected action.
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

// Get decodes the value stored at key into obj. Reads go straight to Redis —
// the in-process TinyLFU local cache is NOT consulted.
//
// obj must already be a pointer (e.g. &result). Passing &obj yields
// *interface{}, which msgpack reflects through to a non-addressable Value and
// panics with "reflect.Value.Set using unaddressable value". Forwarding obj
// directly preserves the caller's pointer.
//
// Correctness-first default: the local cache has no cross-process invalidation
// — another service's Cache.Delete clears Redis but cannot touch this process's
// TinyLFU. Going straight to Redis is the only way to guarantee that a value
// written by one service is immediately visible to readers in another.
//
// Opt in to the local layer via Memory.Local() only for keys with a known
// single-process writer where serving the same value many times in a row is
// worth a small staleness window.
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

// DeleteByPattern removes every key matching keyPattern (glob style).
// This scans the entire keyspace and should be used sparingly — prefer explicit
// keys when the caller knows what was invalidated.
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

// Close releases the underlying Redis client.
// Services that shut down gracefully should call this to flush in-flight work.
func (m *Memory) Close() error {
	if m.client == nil {
		return nil
	}
	return m.client.Close()
}

// deleteIfValueScript is the compare-and-delete used for value-owned locks:
// the key is removed only when it still holds the caller's token, so an
// owner can never release a lock that expired and was re-acquired by
// somebody else. A Lua script because a client-side GET+DEL would race
// between the two commands.
var deleteIfValueScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
end
return 0
`)

// SetNXValue atomically claims key with the caller-supplied value (stored
// raw, no msgpack — the value is an ownership token, not an object).
// Returns (true, nil) when this caller acquired the key, (false, nil) when
// another holder exists, (false, err) on transport error. Bypasses the
// TinyLFU local cache and fails closed on remote outage, exactly like SetNX.
func (m *Memory) SetNXValue(key string, value string, expiration time.Duration) (bool, error) {
	m.init()
	if err := m.remoteUnavailable(); err != nil {
		return false, err
	}
	acquired, err := m.client.SetNX(context.Background(), prefixed(key), value, expiration).Result()
	if err != nil {
		m.recordRemoteError(err)
		return false, err
	}
	m.recordRemoteSuccess()
	return acquired, nil
}

// GetString returns the raw string stored at key ("" when the key does not
// exist). Raw GET, no msgpack decoding: pairs with SetNXValue for reading
// lock ownership tokens.
func (m *Memory) GetString(key string) (string, error) {
	m.init()
	if err := m.remoteUnavailable(); err != nil {
		return "", err
	}
	value, err := m.client.Get(context.Background(), prefixed(key)).Result()
	if errors.Is(err, redis.Nil) {
		m.recordRemoteSuccess()
		return "", nil
	}
	if err != nil {
		m.recordRemoteError(err)
		return "", err
	}
	m.recordRemoteSuccess()
	return value, nil
}

// DeleteIfValue removes key only when its current value equals value (Lua
// compare-and-delete). Returns whether the key was deleted; false with a nil
// error means the key was absent or owned by someone else — both are
// non-errors for a lock release.
func (m *Memory) DeleteIfValue(key string, value string) (bool, error) {
	m.init()
	if err := m.remoteUnavailable(); err != nil {
		return false, err
	}
	deleted, err := deleteIfValueScript.Run(context.Background(), m.client, []string{prefixed(key)}, value).Int()
	if err != nil {
		m.recordRemoteError(err)
		return false, err
	}
	m.recordRemoteSuccess()
	return deleted > 0, nil
}
