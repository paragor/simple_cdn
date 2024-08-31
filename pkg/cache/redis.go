package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/klauspost/compress/zstd"
	"github.com/paragor/simple_cdn/pkg/logger"
	"github.com/paragor/simple_cdn/pkg/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"time"
)

type RedisConfig struct {
	Addr              string        `yaml:"addr"`
	Username          string        `yaml:"username"`
	Password          string        `yaml:"password"`
	DB                int           `yaml:"db"`
	GetTimeout        time.Duration `yaml:"get_timeout"`
	SetTimeout        time.Duration `yaml:"set_timeout"`
	ConnectionTimeout time.Duration `yaml:"connection_timeout"`
}

func (c *RedisConfig) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("addr shoud not be empty")
	}
	if c.DB < 0 {
		return fmt.Errorf("db shoud not < 0")
	}
	if c.GetTimeout <= 0 {
		return fmt.Errorf("get_timeout shoud not <= 0")
	}
	if c.SetTimeout <= 0 {
		return fmt.Errorf("get_timeout shoud not <= 0")
	}
	return nil
}

func (c *RedisConfig) Cache() Cache {
	return newRedisCache(
		redis.NewClient(&redis.Options{
			Addr:                  c.Addr,
			Username:              c.Username,
			Password:              c.Password,
			DB:                    c.DB,
			WriteTimeout:          c.SetTimeout,
			ReadTimeout:           c.GetTimeout,
			PoolTimeout:           c.ConnectionTimeout,
			ContextTimeoutEnabled: true,
			DialTimeout:           c.ConnectionTimeout,
			MinIdleConns:          5,
		}),
		c.SetTimeout,
		c.GetTimeout,
	)
}

type redisCache struct {
	getTimeout time.Duration
	setTimeout time.Duration
	client     *redis.Client
}

func newRedisCache(
	client *redis.Client,
	setTimeout time.Duration,
	getTimeout time.Duration,
) Cache {
	return &redisCache{
		client:     client,
		setTimeout: setTimeout,
		getTimeout: getTimeout,
	}
}

func (c *redisCache) Get(ctx context.Context, key string) *Item {
	log := logger.FromCtx(ctx).
		With(zap.String("component", "cache.redis")).
		With(zap.String("cache_key", key))
	ctx, cancel := context.WithTimeout(context.Background(), c.getTimeout)
	defer cancel()
	redisValueCompressed, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		log.With(zap.Error(err)).Error("cant get cache")
		metrics.CacheErrors.Inc()
		return nil
	}
	bytesBuffer, bytesBufferClean := getBytesBuffer()
	defer bytesBufferClean()

	bytesBuffer, err = zstdDecoder.DecodeAll(redisValueCompressed, bytesBuffer)
	if err != nil {
		log.With(zap.Error(err)).Error("cant decompress cache")
		metrics.CacheErrors.Inc()
		return nil
	}
	item := &Item{}
	if err := json.Unmarshal(bytesBuffer, item); err != nil {
		log.With(zap.Error(err)).Error("cant unmarshal cache")
		metrics.CacheErrors.Inc()
		return nil
	}
	if !item.CacheHeader.ShouldCDNPersist() {
		return nil
	}
	return item
}

func (c *redisCache) Set(ctx context.Context, key string, value *Item) {
	log := logger.FromCtx(ctx).
		With(zap.String("component", "cache.redis")).
		With(zap.String("cache_key", key))
	ttl := value.CacheHeader.ttl()
	if ttl <= 0 {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		log.With(zap.Error(err)).Error("cant marshal cache")
		metrics.CacheErrors.Inc()
		return
	}
	bytesBuffer, bytesBufferClean := getBytesBuffer()
	defer bytesBufferClean()
	bytesBuffer = zstdEncoder.EncodeAll(data, bytesBuffer)
	ctx, cancel := context.WithTimeout(context.Background(), c.setTimeout)
	defer cancel()
	_, err = c.client.SetNX(ctx, key, bytesBuffer, ttl).Result()
	if err != nil {
		log.With(zap.Error(err)).Error("cant save cache")
		metrics.CacheErrors.Inc()
	}
}

var zstdEncoder *zstd.Encoder
var zstdDecoder *zstd.Decoder

func init() {
	var err error
	zstdEncoder, err = zstd.NewWriter(nil)
	if err != nil {
		panic("cant init zstd encoder " + err.Error())
	}
	zstdDecoder, err = zstd.NewReader(nil)
	if err != nil {
		panic("cant init zstd decoder " + err.Error())
	}
}

func (c *redisCache) Invalidate(ctx context.Context, keyPattern string) error {
	log := logger.FromCtx(ctx)
	metrics.CacheInvalidations.Inc()
	itemsCount := 0
	defer func() {
		log.
			With(zap.String("invalidate_key", keyPattern)).
			With(zap.Int("items_count", itemsCount)).
			Info("invalidate cache")
	}()
	iter := c.client.Scan(ctx, 0, keyPattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			itemsCount++
			metrics.CacheInvalidatedItems.Inc()
		}
	}
	if err := iter.Err(); err != nil {
		return err
	}
	return nil
}
