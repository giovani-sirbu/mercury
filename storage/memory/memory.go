package memory

import (
	"context"
	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
	"time"
)

type Memory struct {
	Address  []string
	Password string
	User     string
	PoolSize int
}

func (m Memory) Init() *cache.Cache {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    m.Address,
		Password: m.Password,
		Username: m.User,
		PoolSize: m.PoolSize,
	})

	cacheHandler := cache.New(&cache.Options{
		Redis:      client,
		LocalCache: cache.NewTinyLFU(1000, time.Minute),
	})
	return cacheHandler
}

func (m Memory) Set(key string, obj interface{}, expiration time.Duration) error {
	ctx := context.TODO()

	cacheHandler := m.Init()
	defer ctx.Done()

	err := cacheHandler.Set(&cache.Item{
		Ctx:   ctx,
		Key:   key,
		Value: obj,
		TTL:   expiration,
	})

	return err
}

func (m Memory) Get(key string, obj interface{}) error {
	ctx := context.TODO()

	cacheHandler := m.Init()
	defer ctx.Done()

	err := cacheHandler.Get(ctx, key, &obj)

	return err
}

func (m Memory) Delete(key string) error {
	ctx := context.TODO()

	cacheHandler := m.Init()
	defer ctx.Done()

	err := cacheHandler.Delete(ctx, key)

	return err
}
