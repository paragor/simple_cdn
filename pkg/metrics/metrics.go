package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	CacheLoadTime         *prometheus.HistogramVec
	CacheErrors           prometheus.Counter
	CacheInvalidations    prometheus.Counter
	CacheInvalidatedItems prometheus.Counter
)

func Init(app string) {
	CacheInvalidations = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: app,
		Name:      "cache_invalidations",
		Help:      "cache_invalidations",
	})
	prometheus.MustRegister(CacheInvalidations)

	CacheInvalidatedItems = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: app,
		Name:      "cache_invalidated_items",
		Help:      "cache_invalidated_items",
	})
	prometheus.MustRegister(CacheInvalidatedItems)

	CacheErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: app,
		Name:      "cache_errors",
		Help:      "cache_errors",
	})
	prometheus.MustRegister(CacheErrors)

	CacheLoadTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: app,
		Name:      "cache_load_time",
		Help:      "cache_load_time",
	}, []string{"cache_status"})
	prometheus.MustRegister(CacheLoadTime)
}

func BoolToString(value bool, trueString, falseString string) string {
	if value {
		return trueString
	}
	return falseString
}
