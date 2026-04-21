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

// Memory is a Redis-backed key-value store with an in-process TinyLFU front.
//
// The struct is configured once at service bootstrap (server.NewServer) and reused
// for the lifetime of the process. Callers must hold a *Memory — the pointer
// receiver is required because the singleton state (sync.Once + shared client) must
// be observed across method calls.
//
// Prior versions used a value receiver and called Init() inside every Set/Get/Delete.
// Each call spun up a new Redis UniversalClient plus a fresh TinyLFU, so the local
// cache was effectively dead and every operation paid connection-handshake cost
// (measured at ~3.5ms/op on master). The singleton keeps the client pool and the
// local cache alive for the lifetime of the process.
type Memory struct {
	Address  []string
	Password string
	User     string
	PoolSize int

	once    sync.Once
	handler *cache.Cache
	client  redis.UniversalClient
}

func (m *Memory) init() {
	m.once.Do(func() {
		m.client = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:    m.Address,
			Password: m.Password,
			Username: m.User,
			PoolSize: m.PoolSize,
		})
		m.handler = cache.New(&cache.Options{
			Redis:      m.client,
			LocalCache: cache.NewTinyLFU(10000, time.Minute),
		})
	})
}

func prefixed(key string) string {
	return fmt.Sprintf("%s%s", os.Getenv("REDIS_PREFIX"), key)
}

// Set stores obj under key with the given TTL.
func (m *Memory) Set(key string, obj interface{}, expiration time.Duration) error {
	m.init()
	return m.handler.Set(&cache.Item{
		Ctx:   context.Background(),
		Key:   prefixed(key),
		Value: obj,
		TTL:   expiration,
	})
}

// Get decodes the value stored at key into obj. obj must already be a pointer
// (e.g. &result). Passing &obj yields *interface{}, which msgpack reflects through
// to a non-addressable Value and panics with "reflect.Value.Set using unaddressable
// value". Forwarding obj directly preserves the caller's pointer.
func (m *Memory) Get(key string, obj interface{}) error {
	m.init()
	return m.handler.Get(context.Background(), prefixed(key), obj)
}

// Delete removes key from both the local and the remote cache.
func (m *Memory) Delete(key string) error {
	m.init()
	return m.handler.Delete(context.Background(), prefixed(key))
}

// DeleteByPattern removes every key matching keyPattern (glob style).
// This scans the entire keyspace and should be used sparingly — prefer explicit
// keys when the caller knows what was invalidated.
func (m *Memory) DeleteByPattern(keyPattern string) error {
	m.init()
	ctx := context.Background()
	keys, err := m.client.Keys(ctx, prefixed(keyPattern)).Result()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return m.client.Del(ctx, keys...).Err()
}

// Ping checks connectivity to the remote cache.
func (m *Memory) Ping() error {
	m.init()
	return m.client.Ping(context.Background()).Err()
}

// Close releases the underlying Redis client.
// Services that shut down gracefully should call this to flush in-flight work.
func (m *Memory) Close() error {
	if m.client == nil {
		return nil
	}
	return m.client.Close()
}
