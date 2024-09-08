package cachebehavior

import (
	"github.com/paragor/simple_cdn/pkg/cache"
	"github.com/paragor/simple_cdn/pkg/user"
	"net/http"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func Test_orderedCacheControlFallback_GetCacheControl(t *testing.T) {
	initMetricsAndLogs()
	type args struct {
		request  *http.Request
		response *http.Response
	}
	tests := []struct {
		o    orderedCacheControlFallback
		args args
		want cache.CacheControl
	}{
		{
			o: []orderedCacheControlFallbackItem{
				{
					user: must1(user.PathPattern("^/fallback$")),
					cacheControl: cache.CacheControl{
						Public:               true,
						MaxAge:               0,
						SMaxAge:              1 * time.Hour,
						StaleWhileRevalidate: 2 * time.Hour,
						StaleIfError:         3 * time.Hour,
					},
				},
			},
			args: args{
				request:  createRequest(http.MethodGet, "http://localhost/", http.Header{}, nil, nil),
				response: createResponse(200, http.Header{}, nil),
			},
			want: cache.CacheControl{},
		},
		{
			o: []orderedCacheControlFallbackItem{
				{
					user: must1(user.PathPattern("^/fallback$")),
					cacheControl: cache.CacheControl{
						Public:               true,
						MaxAge:               0,
						SMaxAge:              1 * time.Hour,
						StaleWhileRevalidate: 2 * time.Hour,
						StaleIfError:         3 * time.Hour,
					},
				},
			},
			args: args{
				request:  createRequest(http.MethodGet, "http://localhost/", http.Header{}, nil, nil),
				response: createResponse(200, http.Header{"Cache-Control": {"public, max-age=100, s-maxage=200, stale-while-revalidate=300, stale-if-error=400"}}, nil),
			},
			want: cache.CacheControl{
				Public:               true,
				MaxAge:               100 * time.Second,
				SMaxAge:              200 * time.Second,
				StaleWhileRevalidate: 300 * time.Second,
				StaleIfError:         400 * time.Second,
			},
		},
		{
			o: []orderedCacheControlFallbackItem{
				{
					user: must1(user.PathPattern("^/fallback$")),
					cacheControl: cache.CacheControl{
						Public:               true,
						MaxAge:               0,
						SMaxAge:              1 * time.Hour,
						StaleWhileRevalidate: 2 * time.Hour,
						StaleIfError:         3 * time.Hour,
					},
				},
			},
			args: args{
				request:  createRequest(http.MethodGet, "http://localhost/fallback", http.Header{}, nil, nil),
				response: createResponse(200, http.Header{"Cache-Control": {"public, max-age=100, s-maxage=200, stale-while-revalidate=300, stale-if-error=400"}}, nil),
			},
			want: cache.CacheControl{
				Public:               true,
				MaxAge:               0,
				SMaxAge:              1 * time.Hour,
				StaleWhileRevalidate: 2 * time.Hour,
				StaleIfError:         3 * time.Hour,
			},
		},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			if got := tt.o.GetCacheControl(tt.args.request, tt.args.response); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCacheControl() = %v, want %v", got, tt.want)
			}
		})
	}
}
