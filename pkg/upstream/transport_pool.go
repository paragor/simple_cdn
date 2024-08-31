package upstream

import (
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"
)

type TransportPoolConfig struct {
	Size                int           `yaml:"size,omitempty"`
	MaxIdleConnsPerHost int           `yaml:"max_idle_conns_per_host,omitempty"`
	IdleConnTimeout     time.Duration `yaml:"idle_conn_timeout"`
	ConnTimeout         time.Duration `yaml:"conn_timeout"`
	KeepAliveTimeout    time.Duration `yaml:"keep_alive_timeout"`
	MaxLifeTime         time.Duration `yaml:"max_life_time"`
}

func (c *TransportPoolConfig) Validate() error {
	if c.Size <= 0 {
		return fmt.Errorf("size should be >= 0")
	}
	if c.MaxIdleConnsPerHost <= 0 {
		return fmt.Errorf("max_idle_conns_per_host should be >= 0")
	}
	if c.IdleConnTimeout <= 0 {
		return fmt.Errorf("idle_conn_timeout should be >= 0")
	}
	if c.ConnTimeout <= 0 {
		return fmt.Errorf("conn_timeout should be >= 0")
	}
	if c.KeepAliveTimeout <= 0 {
		return fmt.Errorf("keep_alive_timeout should be >= 0")
	}
	if c.MaxLifeTime <= 0 {
		return fmt.Errorf("MaxLifeTime should be >= 0")
	}
	return nil
}

type TransportPool struct {
	config TransportPoolConfig

	sync.Mutex
	transports []*Transport
	curr       int
}

type Transport struct {
	http.Transport
	poolDeadLine time.Time
}

func NewTransportPool(config TransportPoolConfig) *TransportPool {
	pool := &TransportPool{config: config}
	for i := 0; i < config.Size; i++ {
		pool.transports = append(pool.transports, pool.newTransport())
	}

	return pool
}

func (pool *TransportPool) RoundTrip(req *http.Request) (*http.Response, error) {
	return pool.Next().RoundTrip(req)
}

func (pool *TransportPool) Next() http.RoundTripper {
	pool.Lock()
	defer pool.Unlock()
	pool.curr = (pool.curr + 1) % len(pool.transports)

	ret := pool.transports[pool.curr]

	if time.Now().After(pool.transports[pool.curr].poolDeadLine) {
		pool.transports[pool.curr] = pool.newTransport()
	}

	return ret
}

func (pool *TransportPool) newTransport() *Transport {
	return &Transport{
		Transport: http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   pool.config.ConnTimeout,
				KeepAlive: pool.config.KeepAliveTimeout,
			}).DialContext,
			MaxIdleConnsPerHost: pool.config.MaxIdleConnsPerHost,
			IdleConnTimeout:     pool.config.ConnTimeout,
		},
		poolDeadLine: time.Now().Add(pool.config.MaxLifeTime + time.Duration(rand.Intn(int(pool.config.MaxLifeTime)/10))),
	}
}
