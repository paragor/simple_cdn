package cache

import (
	"net/http"
	"strings"
	"time"
)

type Item struct {
	SavedAt     time.Time
	CacheHeader CacheControl
	Headers     map[string][]string
	Body        []byte
}

func (item *Item) CanUseCache(now time.Time) bool {
	return item.CacheHeader.Public && now.Sub(item.SavedAt) < item.CacheHeader.SMaxAge
}

func (item *Item) CanStaleIfError(now time.Time) bool {
	return item.CacheHeader.Public && now.Sub(item.SavedAt) < item.CacheHeader.StaleIfError
}

func (item *Item) CanStaleWhileRevalidation(now time.Time) bool {
	return item.CacheHeader.Public && now.Sub(item.SavedAt) < item.CacheHeader.StaleWhileInvalidation
}

func ShouldPersist(response *http.Response) bool {
	cacheControlHeader := response.Header.Get("Cache-Control")
	if len(cacheControlHeader) == 0 {
		return false
	}
	cacheControl := ParseCacheControlHeader(cacheControlHeader)
	return cacheControl.ShouldCDNPersist()
}

func ItemFromResponse(response *http.Response, body []byte) *Item {
	cacheControlHeader := response.Header.Get("Cache-Control")
	if len(cacheControlHeader) == 0 {
		return nil
	}
	cacheControl := ParseCacheControlHeader(cacheControlHeader)
	if !cacheControl.ShouldCDNPersist() {
		return nil
	}
	return &Item{
		SavedAt:     time.Now(),
		Headers:     response.Header.Clone(),
		Body:        body,
		CacheHeader: cacheControl,
	}
}

func (item *Item) Write(w http.ResponseWriter) error {
	for k, values := range item.Headers {
		lowerHeader := strings.ToLower(k)
		if lowerHeader == "x-cache-status" || lowerHeader == "set-cookie" {
			continue
		}
		for _, v := range values {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(200)
	_, err := w.Write(item.Body)
	return err
}
