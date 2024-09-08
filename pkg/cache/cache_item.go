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
	return item.CacheHeader.Public && now.Sub(item.SavedAt) < item.CacheHeader.StaleWhileRevalidate
}

func ItemFromResponse(response *http.Response, cacheControl CacheControl, body []byte) *Item {
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
