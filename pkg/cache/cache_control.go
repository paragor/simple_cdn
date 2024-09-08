package cache

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type CacheControl struct {
	Public               bool          `yaml:"public"`
	MaxAge               time.Duration `yaml:"max-age"`
	SMaxAge              time.Duration `yaml:"s-maxage"`
	StaleWhileRevalidate time.Duration `yaml:"stale-while-revalidate"`
	StaleIfError         time.Duration `yaml:"stale-if-error"`
}

func (cc *CacheControl) ttl() time.Duration {
	return max(cc.SMaxAge, cc.StaleIfError, cc.StaleWhileRevalidate)
}

func (cc *CacheControl) ShouldCDNPersist() bool {
	return cc.Public && (cc.SMaxAge > 0 || cc.StaleWhileRevalidate > 0 || cc.StaleIfError > 0)
}
func (cc *CacheControl) Validate() error {
	if cc.MaxAge < 0 {
		return fmt.Errorf("max-age cant not be < 0")
	}
	if cc.SMaxAge < 0 {
		return fmt.Errorf("s-maxage cant not be < 0")
	}
	if cc.StaleWhileRevalidate < 0 {
		return fmt.Errorf("stale-while-revalidate cant not be < 0")
	}
	if cc.StaleIfError < 0 {
		return fmt.Errorf("stale-if-error cant not be < 0")
	}
	return nil
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
			result.StaleWhileRevalidate = time.Second * time.Duration(duration)
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

func (cc *CacheControl) Clone() *CacheControl {
	return &CacheControl{
		Public:               cc.Public,
		MaxAge:               cc.MaxAge,
		SMaxAge:              cc.SMaxAge,
		StaleWhileRevalidate: cc.StaleWhileRevalidate,
		StaleIfError:         cc.StaleIfError,
	}
}
