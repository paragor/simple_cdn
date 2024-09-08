package cachebehavior

import (
	"fmt"
	"github.com/paragor/simple_cdn/pkg/cache"
	"github.com/paragor/simple_cdn/pkg/logger"
	"github.com/paragor/simple_cdn/pkg/user"
	"go.uber.org/zap"
	"net/http"
)

type OrderedCacheControlFallbackConfig []struct {
	User         user.Config        `yaml:"user"`
	CacheControl cache.CacheControl `yaml:"cache_control"`
}

func (c *OrderedCacheControlFallbackConfig) Validate() error {
	if c == nil {
		return nil
	}
	for i, itemConfig := range *c {
		if err := itemConfig.User.Validate(); err != nil {
			return fmt.Errorf("ordered cache control '%d' 'user' is invalid: %w", i, err)
		}
		if err := itemConfig.CacheControl.Validate(); err != nil {
			return fmt.Errorf("ordered cache control '%d' 'cache_control' is invalid: %w", i, err)
		}
	}
	return nil
}

func (c *OrderedCacheControlFallbackConfig) ToCacheControlParser() CacheControlParser {
	if c == nil {
		return &orderedCacheControlFallback{}
	}
	result := orderedCacheControlFallback(make([]orderedCacheControlFallbackItem, 0, len(*c)))
	for _, itemConfig := range *c {
		result = append(result, orderedCacheControlFallbackItem{
			user:         itemConfig.User.ToUser(),
			cacheControl: *itemConfig.CacheControl.Clone(),
		})
	}
	return &result
}

type CacheControlParser interface {
	GetCacheControl(request *http.Request, response *http.Response) cache.CacheControl
}

type orderedCacheControlFallbackItem struct {
	user         user.User
	cacheControl cache.CacheControl
}

type orderedCacheControlFallback []orderedCacheControlFallbackItem

func (o *orderedCacheControlFallback) GetCacheControl(request *http.Request, response *http.Response) cache.CacheControl {
	if request.Method != http.MethodGet {
		return cache.CacheControl{}
	}
	cacheControl := cache.ParseCacheControlHeader(response.Header.Get("Cache-Control"))
	if o == nil {
		return cacheControl
	}
	for i, fallback := range *o {
		if fallback.user.IsUser(request) {
			logger.FromCtx(request.Context()).
				With(zap.String("component", "orderedCacheControlFallback")).
				With(zap.Int("cache_control_fallback_index", i)).
				Debug("use fallback cache control")
			return *fallback.cacheControl.Clone()
		}
	}
	return cacheControl
}
