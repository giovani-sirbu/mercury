package memory

import (
	"context"
	"fmt"
	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
	"os"
	"time"
)

type Memory struct {
	Address  []string
	Password string
	User     string
	PoolSize int
}

func (m Memory) Init() (*cache.Cache, redis.UniversalClient) {
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
	return cacheHandler, client
}

func (m Memory) Set(key string, obj interface{}, expiration time.Duration) error {
	ctx := context.TODO()

	cacheHandler, _ := m.Init()
	defer ctx.Done()

	keyWithPrefix := fmt.Sprintf("%s%s", os.Getenv("REDIS_PREFIX"), key)

	err := cacheHandler.Set(&cache.Item{
		Ctx:   ctx,
		Key:   keyWithPrefix,
		Value: obj,
		TTL:   expiration,
	})

	return err
}

func (m Memory) Get(key string, obj interface{}) error {
	ctx := context.TODO()

	cacheHandler, _ := m.Init()
	defer ctx.Done()

	keyWithPrefix := fmt.Sprintf("%s%s", os.Getenv("REDIS_PREFIX"), key)
	err := cacheHandler.Get(ctx, keyWithPrefix, &obj)

	return err
}

func (m Memory) Delete(key string) error {
	ctx := context.TODO()

	cacheHandler, _ := m.Init()
	defer ctx.Done()

	keyWithPrefix := fmt.Sprintf("%s%s", os.Getenv("REDIS_PREFIX"), key)
	err := cacheHandler.Delete(ctx, keyWithPrefix)

	return err
}

func (m Memory) DeleteByPattern(keyPattern string) error {
	ctx := context.TODO()

	_, client := m.Init()
	defer ctx.Done()

	keyPatternWithPrefix := fmt.Sprintf("%s%s", os.Getenv("REDIS_PREFIX"), keyPattern)
	keys, err := client.Keys(ctx, keyPatternWithPrefix).Result()
	for _, key := range keys {
		client.Del(ctx, key)
	}
	return err
}

func (m Memory) Ping() error {
	ctx := context.Background()
	_, client := m.Init()

	defer ctx.Done()

	err := client.Ping(ctx).Err()
	return err
}
