package cachebehavior

import (
	"bytes"
	"github.com/paragor/simple_cdn/pkg/cache"
	"github.com/paragor/simple_cdn/pkg/logger"
	"github.com/paragor/simple_cdn/pkg/metrics"
	"github.com/paragor/simple_cdn/pkg/upstream"
	"github.com/paragor/simple_cdn/pkg/user"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const bufferSize = 32 * 1024

type cacheBehavior struct {
	upstream       upstream.Upstream
	cacheKeyConfig *cache.KeyConfig
	cache          cache.Cache

	canPersistCache    user.User
	canLoadCache       user.User
	cacheControlParser CacheControlParser
}

func NewCacheBehavior(
	canPersistCache user.User,
	canLoadCache user.User,
	cacheKeyConfig *cache.KeyConfig,
	upstream upstream.Upstream,
	cache cache.Cache,
	cacheControlParser CacheControlParser,
) http.Handler {
	return &cacheBehavior{
		upstream:           upstream,
		cacheKeyConfig:     cacheKeyConfig,
		cache:              cache,
		canPersistCache:    canPersistCache,
		canLoadCache:       canLoadCache,
		cacheControlParser: cacheControlParser,
	}
}
func (b *cacheBehavior) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	canPersistCache := b.canPersistCache.IsUser(r)
	canLoadCache := b.canLoadCache.IsUser(r)
	log :=
		logger.FromCtx(r.Context()).
			With(zap.String("component", "cacheBehavior")).
			With(zap.Bool("can_persist_cache", canPersistCache)).
			With(zap.Bool("can_load_cache", canLoadCache))

	now := time.Now()
	if r.Method != http.MethodGet || (!canPersistCache && !canLoadCache) {
		log.Debug("just proxy pass")
		response, err := b.upstream.Do(r)
		if err != nil {
			log.With(zap.Error(err)).Error("cant send request to upstream")
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		defer response.Body.Close()
		copyHeaders(response.Header, w.Header())
		w.Header().Set("X-Cache-Status", "MISS")
		w.WriteHeader(response.StatusCode)
		if err := ioCopy(w, response.Body); err != nil {
			log.With(zap.Error(err)).Warn("cant write response body")
		}
		return
	}
	var cacheItem *cache.Item
	if canLoadCache {
		start := time.Now()
		cacheItem = b.cache.Get(r.Context(), b.cacheKeyConfig.Apply(r))
		cacheStatus := metrics.BoolToString(cacheItem != nil, "HIT", "MISS")
		metrics.CacheLoadTime.
			WithLabelValues(cacheStatus).
			Observe(time.Now().Sub(start).Seconds())
		log.With(zap.Bool("found", cacheItem != nil)).
			With(zap.String("cache_status", cacheStatus)).
			Debug("load cache item")
	}
	if cacheItem != nil && cacheItem.CanUseCache(now) {
		log.Debug("response from cache")
		w.Header().Set("X-Cache-Status", "HIT")
		if err := cacheItem.Write(w); err != nil {
			log.With(zap.Error(err)).Warn("cant write cache response")
		}
		return
	}

	if cacheItem != nil && cacheItem.CanStaleWhileRevalidation(now) {
		log.Debug("response from stale")
		w.Header().Set("X-Cache-Status", "HIT-STALE")
		if err := cacheItem.Write(w); err != nil {
			log.With(zap.Error(err)).Warn("cant write cache response")
		}
		if !canPersistCache {
			return
		}
		go func() {
			cacheIsInvalidated := false
			log = log.With(zap.String("goroutine", "invalidation"))
			defer func() {
				log.With(zap.Bool("is_invalidated", cacheIsInvalidated)).Debug("stale cache invalidated")
			}()
			response, err := b.upstream.Do(r)
			if err != nil {
				log.With(zap.Error(err)).Error("upstream error")
				return
			}
			defer response.Body.Close()
			log = log.With(zap.Int("upstream_status", response.StatusCode))
			if response.StatusCode != 200 {
				log.Warn("not cachable status code")
				return
			}
			cacheBytesBuffer, cacheBytesBufferClean := getBytesBuffer()
			defer cacheBytesBufferClean()
			buffer := bytes.NewBuffer(cacheBytesBuffer)
			if _, err := buffer.ReadFrom(response.Body); err != nil {
				log.With(zap.Error(err)).Error("cant read upstream body")
				return
			}
			cacheControl := b.cacheControlParser.GetCacheControl(r, response)
			if !cacheControl.ShouldCDNPersist() {
				return
			}
			item := cache.ItemFromResponse(response, cacheControl, buffer.Bytes())
			if item == nil {
				return
			}
			b.cache.Set(r.Context(), b.cacheKeyConfig.Apply(r), item)
			cacheIsInvalidated = true
		}()
		return
	}

	response, err := b.upstream.Do(r)
	if err != nil {
		if cacheItem != nil && cacheItem.CanStaleIfError(now) {
			log.With(zap.Error(err)).Debug("use stale cache")
			w.Header().Set("X-Cache-Status", "HIT-ERROR")
			if err := cacheItem.Write(w); err != nil {
				log.With(zap.Error(err)).Warn("cant write cache response")
			}
			return
		}
		log.With(zap.Error(err)).Error("cant send request to upstream")
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		return
	}

	defer response.Body.Close()
	log = log.With(zap.Int("upstream_status", response.StatusCode))
	if response.StatusCode != 200 {
		if response.StatusCode >= 500 && cacheItem != nil && cacheItem.CanStaleIfError(now) {
			log.Info("response from cache due code >= 500")
			w.Header().Set("X-Cache-Status", "HIT-ERROR")
			if err := cacheItem.Write(w); err != nil {
				log.With(zap.Error(err)).Warn("cant write cache response")
			}
			return
		}
		log.Debug("response to client with not 200 status")
		copyHeaders(response.Header, w.Header())
		w.Header().Set("X-Cache-Status", "ERROR")
		w.WriteHeader(response.StatusCode)
		if err := ioCopy(w, response.Body); err != nil {
			log.With(zap.Error(err)).Warn("cant write response body")
		}
		return
	}
	copyHeaders(response.Header, w.Header())
	w.Header().Set("X-Cache-Status", "MISS")
	w.WriteHeader(response.StatusCode)
	cacheControl := b.cacheControlParser.GetCacheControl(r, response)
	if !canPersistCache || !cacheControl.ShouldCDNPersist() {
		log.Debug("response to client without cache save")
		if err = ioCopy(w, response.Body); err != nil {
			log.With(zap.Error(err)).Warn("cant write response body")
		}
		return
	}

	bodyBytes, bodyBytesClean := getBytesBuffer()
	bodyBuffer := bytes.NewBuffer(bodyBytes)
	if err := ioCopyWithPersist(w, response.Body, bodyBuffer); err != nil {
		log.With(zap.Error(err)).Error("cant read all body from upstream")
		bodyBytesClean()
		return
	}
	ctx := r.Context()
	go func() {
		cacheIsSaved := false
		log = log.With(zap.String("goroutine", "cache_saving"))
		defer func() {
			log.With(zap.Bool("is_saved", cacheIsSaved)).Debug("persist cache")
		}()
		defer bodyBytesClean()
		cacheItem = cache.ItemFromResponse(response, cacheControl, bodyBuffer.Bytes())
		if cacheItem == nil {
			return
		}
		b.cache.Set(ctx, b.cacheKeyConfig.Apply(r), cacheItem)
		cacheIsSaved = true
	}()
}

var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, 8*1024)
	},
}

func getBytesBuffer() ([]byte, func()) {
	responseBytesBuffer := bufferPool.Get().([]byte)[:0]
	return responseBytesBuffer, func() {
		bufferPool.Put(responseBytesBuffer[:0])
	}
}

func ioCopy(dst io.Writer, src io.Reader) error {
	responseBytesBuffer, responseBytesBufferClean := getBytesBuffer()
	defer responseBytesBufferClean()
	var buffer []byte
	buffer = responseBytesBuffer[:cap(responseBytesBuffer)]
	if len(buffer) < bufferSize {
		buffer = make([]byte, bufferSize)
	}
	_, err := io.CopyBuffer(dst, src, buffer)
	return err
}

func ioCopyWithPersist(dst io.Writer, src io.Reader, buffer *bytes.Buffer) error {
	if _, err := buffer.ReadFrom(src); err != nil {
		return err
	}
	_, err := dst.Write(buffer.Bytes())
	return err
}

func copyHeaders(from, to http.Header) {
	for k, values := range from {
		lowerHeader := strings.ToLower(k)
		if lowerHeader == "x-cache-status" {
			continue
		}
		for _, v := range values {
			to.Add(k, v)
		}
	}
}
