package cache

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

type CacheControl struct {
	Public                 bool
	MaxAge                 time.Duration
	SMaxAge                time.Duration
	StaleWhileInvalidation time.Duration
	StaleIfError           time.Duration
}

func (cc *CacheControl) ttl() time.Duration {
	return max(cc.SMaxAge, cc.StaleIfError, cc.StaleWhileInvalidation)
}

func (cc *CacheControl) ShouldCDNPersist() bool {
	return cc.Public && (cc.SMaxAge > 0 || cc.StaleWhileInvalidation > 0 || cc.StaleIfError > 0)
}

func ParseCacheControlHeader(str string) CacheControl {
	result := CacheControl{}
	for _, token := range strings.Split(strings.TrimSpace(strings.ToLower(str)), " ") {
		token = strings.TrimFunc(token, func(r rune) bool {
			return unicode.IsSpace(r) || r == ','
		})
		if token == "" {
			continue
		}
		if token == "public" {
			result.Public = true
			continue
		}
		if strings.HasPrefix(token, "max-age=") {
			value := strings.TrimPrefix(token, "max-age=")
			duration, err := strconv.Atoi(value)
			if err != nil {
				continue
			}
			result.MaxAge = time.Second * time.Duration(duration)
			continue
		}
		if strings.HasPrefix(token, "s-maxage=") {
			value := strings.TrimPrefix(token, "s-maxage=")
			duration, err := strconv.Atoi(value)
			if err != nil {
				continue
			}
			result.SMaxAge = time.Second * time.Duration(duration)
			continue
		}
		if strings.HasPrefix(token, "stale-while-revalidate=") {
			value := strings.TrimPrefix(token, "stale-while-revalidate=")
			duration, err := strconv.Atoi(value)
			if err != nil {
				continue
			}
			result.StaleWhileInvalidation = time.Second * time.Duration(duration)
			continue
		}
		if strings.HasPrefix(token, "stale-if-error=") {
			value := strings.TrimPrefix(token, "stale-if-error=")
			duration, err := strconv.Atoi(value)
			if err != nil {
				continue
			}
			result.StaleIfError = time.Second * time.Duration(duration)
			continue
		}

	}

	return result
}
