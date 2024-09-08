package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/paragor/simple_cdn/pkg/cache"
	"github.com/paragor/simple_cdn/pkg/cachebehavior"
	"github.com/paragor/simple_cdn/pkg/logger"
	"github.com/paragor/simple_cdn/pkg/metrics"
	"github.com/paragor/simple_cdn/pkg/upstream"
	"github.com/paragor/simple_cdn/pkg/user"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"time"
)

func getLogLevel() zapcore.Level {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "info", "":
		return zapcore.InfoLevel
	case "debug":
		return zapcore.DebugLevel
	case "error":
		return zapcore.ErrorLevel
	case "warn":
		return zapcore.WarnLevel
	default:
		panic("unknown logger level")
	}
}

var app = "simple_cdn"

func main() {
	logLevel := getLogLevel()
	logger.Init(app, logLevel)
	metrics.Init(app)
	log := logger.Logger()
	configPath := flag.String("config", "", "config path in yaml format")
	checkConfig := flag.Bool("check-config", false, "only check validation and exit")
	flag.Parse()
	data, err := os.ReadFile(*configPath)
	if err != nil {
		panic(err)
	}

	config, err := ParseConfig(data)
	if err != nil {
		log.With(zap.Error(err)).Fatal("cant parse config")
	}
	log.With(zap.String("description", config.CanPersistCache.ToUser().String())).Info("can persist cache config")
	log.With(zap.String("description", config.CanLoadCache.ToUser().String())).Info("can load cache config")
	log.With(zap.String("description", config.CanForceEmitDebugLogging.ToUser().String())).Info("can force emit debug logging")
	if *checkConfig {
		log.Info("check-config is set, config is valid")
		os.Exit(0)
	}

	cacheDb := config.Cache.Cache()
	handler := cachebehavior.NewCacheBehavior(
		config.CanPersistCache.ToUser(),
		config.CanLoadCache.ToUser(),
		&config.CacheKeyConfig,
		config.Upstream.CreateUpstream(),
		cacheDb,
		config.OrderedCacheControlFallback.ToCacheControlParser(),
	)
	handler = logger.HttpRecoveryMiddleware(handler)
	handler = logger.HttpLoggingMiddleware(handler)
	handler = logger.HttpSetLoggerMiddleware(config.CanForceEmitDebugLogging.ToUser(), handler)

	mainServer := http.Server{
		Addr:    config.ListenAddr,
		Handler: handler,
	}
	diagnosticServer := http.Server{
		Addr:    config.DiagnosticAddr,
		Handler: GetDiagnosticServerHandler(cacheDb),
	}

	diagnosticServer.RegisterOnShutdown(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		defer cancel()
		_ = mainServer.Shutdown(ctx)
	})
	mainServer.RegisterOnShutdown(func() {
		_ = diagnosticServer.Close()
	})

	go func() {
		time.Sleep(time.Second * 5)
		log.Debug("starting diagnostic server")
		if err = diagnosticServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.With(zap.Error(err)).Error("on listen diagnostic server")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		defer cancel()
		_ = mainServer.Shutdown(ctx)
	}()
	log.Debug("starting main server")
	if err = mainServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.With(zap.Error(err)).Fatal("on listen main server")
	}
	log.Info("good bye")
}

type Config struct {
	ListenAddr                  string                                          `yaml:"listen_addr"`
	DiagnosticAddr              string                                          `yaml:"diagnostic_addr"`
	CanPersistCache             user.Config                                     `yaml:"can_persist_cache"`
	CanLoadCache                user.Config                                     `yaml:"can_load_cache"`
	CanForceEmitDebugLogging    user.Config                                     `yaml:"can_force_emit_debug_logging"`
	CacheKeyConfig              cache.KeyConfig                                 `yaml:"cache_key_config"`
	Upstream                    upstream.Config                                 `yaml:"upstream"`
	Cache                       cache.Config                                    `yaml:"cache"`
	OrderedCacheControlFallback cachebehavior.OrderedCacheControlFallbackConfig `yaml:"ordered_cache_control_fallback"`
}

func (c *Config) Validate() error {
	if c.ListenAddr == "" {
		c.ListenAddr = ":8080"
	}
	if c.DiagnosticAddr == "" {
		c.DiagnosticAddr = ":7070"
	}
	if err := c.CanForceEmitDebugLogging.Validate(); err != nil {
		return fmt.Errorf("when_collect_debug_logging invalid: %w", err)
	}
	if err := c.CanPersistCache.Validate(); err != nil {
		return fmt.Errorf("can_persist_cache invalid: %w", err)
	}
	if err := c.CanLoadCache.Validate(); err != nil {
		return fmt.Errorf("can_load_cache invalid: %w", err)
	}
	if err := c.CacheKeyConfig.Validate(); err != nil {
		return fmt.Errorf("cache_key_config invalid: %w", err)
	}
	if err := c.Upstream.Validate(); err != nil {
		return fmt.Errorf("upstream invalid: %w", err)
	}
	if err := c.Cache.Validate(); err != nil {
		return fmt.Errorf("cache invalid: %w", err)
	}
	if err := c.OrderedCacheControlFallback.Validate(); err != nil {
		return fmt.Errorf("ordered_cache_control_fallback invalid: %w", err)
	}
	return nil
}

func ParseConfig(data []byte) (*Config, error) {
	config := &Config{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("error on unmarshal: %w", err)
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("error on validation: %w", err)
	}
	return config, nil
}

func GetDiagnosticServerHandler(cacheDb cache.Cache) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/readyz", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		_, _ = writer.Write([]byte("ok"))
	})
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		_, _ = writer.Write([]byte("ok"))
	})
	mux.HandleFunc("/invalidate", func(writer http.ResponseWriter, request *http.Request) {
		ctx := request.Context()
		if ctx == nil {
			var cancel func()
			ctx, cancel = context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()
		}
		keyPattern := request.URL.Query().Get("pattern")
		if keyPattern == "" {
			http.Error(writer, "query 'pattern' is empty", 400)
			return
		}
		if err := cacheDb.Invalidate(ctx, keyPattern); err != nil {
			http.Error(writer, "cant invalidate cache:"+err.Error(), 500)
		}

		writer.WriteHeader(200)
		_, _ = writer.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}
