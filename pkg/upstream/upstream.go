package upstream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Config struct {
	TransportPoolConfig TransportPoolConfig `yaml:"transport_pool_config"`
	Host                string              `yaml:"host"`
	Scheme              string              `yaml:"scheme"`
	RequestTimeout      time.Duration       `yaml:"request_timeout"`
}

func (c *Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host can not be empty")
	}
	if c.Scheme != "http" && c.Scheme != "https" {
		return fmt.Errorf("scheme should have value http or https")
	}
	if c.RequestTimeout < 0 {
		return fmt.Errorf("request_timeout should be >= 0")
	}
	return c.TransportPoolConfig.Validate()
}

func (c *Config) CreateUpstream() Upstream {
	requestTimeout := c.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = time.Second * 360
	}

	return newSingleHostUpstream(
		NewTransportPool(c.TransportPoolConfig),
		requestTimeout,
		c.Scheme,
		c.Host,
	)
}

type Upstream interface {
	Do(originRequest *http.Request) (*http.Response, error)
}

type singleHostUpstream struct {
	pool           http.RoundTripper
	requestTimeout time.Duration
	targetScheme   string
	targetHost     string
}

func newSingleHostUpstream(
	roundTripper http.RoundTripper,
	requestTimeout time.Duration,
	targetScheme string,
	targetHost string,
) Upstream {
	return &singleHostUpstream{
		pool:           roundTripper,
		requestTimeout: requestTimeout,
		targetScheme:   targetScheme,
		targetHost:     targetHost,
	}
}

func (u *singleHostUpstream) Do(originRequest *http.Request) (*http.Response, error) {
	bufferData, bufferDataClean := getBytesBuffer()
	defer bufferDataClean()

	requestBody := bytes.NewBuffer(bufferData)
	_, _ = requestBody.ReadFrom(originRequest.Body)

	request := originRequest.Clone(context.Background())
	request.URL.Scheme = u.targetScheme
	request.URL.Host = u.targetHost
	request.Host = u.targetHost
	if requestBody.Len() == 0 {
		request.Body = nil
	} else {
		request.Body = io.NopCloser(requestBody)
	}
	removeHopByHopHeaders(request.Header)
	if ua := request.Header.Get("User-Agent"); len(ua) == 0 {
		// If the outbound request doesn't have a User-Agent header set,
		// don't send the default Go HTTP client User-Agent.
		request.Header.Set("User-Agent", "")
	}
	done := make(chan bool, 1)
	var response *http.Response
	var err error
	timer := time.NewTimer(u.requestTimeout)
	defer timer.Stop()
	go func() {
		response, err = u.pool.RoundTrip(request)
		done <- true
	}()
	select {
	case <-done:
		removeHopByHopHeaders(response.Header)
		return response, err
	case <-timer.C:
		return nil, fmt.Errorf("request timeout")
	}
}

var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, 0)
	},
}

func getBytesBuffer() ([]byte, func()) {
	responseBytesBuffer := bufferPool.Get().([]byte)[:0]
	return responseBytesBuffer, func() {
		bufferPool.Put(responseBytesBuffer[:0])
	}
}

// Hop-by-hop headers. These are removed when sent to the backend.
// As of RFC 7230, hop-by-hop headers are required to appear in the
// Connection header field. These are the headers defined by the
// obsoleted RFC 2616 (section 13.5.1) and are used for backward
// compatibility.
var hopHeaders = []string{
	"Connection",
	"Proxy-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",      // canonicalized version of "TE"
	"Trailer", // not Trailers per URL above; https://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	"Upgrade",

	"Accept-Encoding",
}

func removeHopByHopHeaders(h http.Header) {
	// RFC 2616, section 13.5.1: Remove a set of known hop-by-hop headers.
	// This behavior is superseded by the RFC 7230 Connection header, but
	// preserve it for backwards compatibility.
	for _, f := range hopHeaders {
		h.Del(f)
	}
}
