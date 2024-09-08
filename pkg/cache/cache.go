package cache

import (
	"context"
	"fmt"
	"sync"
)

type Config struct {
	Type  string      `yaml:"type"`
	Redis RedisConfig `yaml:"redis"`
}

func (c *Config) Validate() error {
	if c.Type != "redis" {
		return fmt.Errorf("type should be redis")
	}
	if err := c.Redis.Validate(); err != nil {
		return fmt.Errorf("redis is invalid: %w", err)
	}
	return nil
}

func (c *Config) Cache() Cache {
	if c.Type == "redis" {
		return c.Redis.Cache()
	}
	panic("only redis is supported")
}

type Cache interface {
	Get(ctx context.Context, key string) *Item
	Set(ctx context.Context, key string, value *Item)
	Invalidate(ctx context.Context, keyPattern string) error
}

var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, 8*1024)
	},
}

func getBytesBuffer() ([]byte, func()) {
	responseBytesBuffer := bufferPool.Get().([]byte)[:0]
	return responseBytesBuffer, func() {
		if cap(responseBytesBuffer) > 256*1024 {
			return
		}
		bufferPool.Put(responseBytesBuffer[:0])
	}
}
