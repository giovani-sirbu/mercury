package memory

import (
	"context"
	"crypto/tls"
	"github.com/go-redis/cache/v8"
	"github.com/go-redis/redis/v8"
	"time"
)

type Memory struct {
	Address  []string
	Password string
	User     string
}

func (m Memory) Init() (*cache.Cache, context.Context) {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:     m.Address,
		Password:  m.Password,
		Username:  m.User,
		TLSConfig: &tls.Config{},
	})

	cacheHandler := cache.New(&cache.Options{
		Redis:      client,
		LocalCache: cache.NewTinyLFU(1000, time.Minute),
	})
	return cacheHandler, client.Context()
}

func (m Memory) Set(key string, obj interface{}, expiration time.Duration) error {
	ctx := context.TODO()

	cacheHandler, clientContext := m.Init()
	defer clientContext.Done()
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

	cacheHandler, clientContext := m.Init()
	defer clientContext.Done()
	defer ctx.Done()

	err := cacheHandler.Get(ctx, key, &obj)

	return err
}

func (m Memory) Delete(key string) error {
	ctx := context.TODO()

	cacheHandler, clientContext := m.Init()
	defer clientContext.Done()
	defer ctx.Done()

	err := cacheHandler.Delete(ctx, key)

	return err
}
