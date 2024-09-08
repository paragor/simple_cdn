package cache

import (
	"reflect"
	"testing"
	"time"
)

func TestParseCacheHeader(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name string
		args args
		want CacheControl
	}{
		{
			name: "public, max-age=100, s-maxage=200, stale-while-revalidate=300, stale-if-error=400",
			args: args{
				str: "public, max-age=100, s-maxage=200, stale-while-revalidate=300, stale-if-error=400",
			},
			want: CacheControl{
				Public:               true,
				MaxAge:               100 * time.Second,
				SMaxAge:              200 * time.Second,
				StaleWhileRevalidate: 300 * time.Second,
				StaleIfError:         400 * time.Second,
			},
		},
		{
			name: "nothing",
			args: args{
				str: "nothing",
			},
			want: CacheControl{
				Public:               false,
				MaxAge:               0,
				SMaxAge:              0,
				StaleWhileRevalidate: 0,
				StaleIfError:         0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseCacheControlHeader(tt.args.str); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseCacheControlHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}
