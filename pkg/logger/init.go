package logger

import (
	"context"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sync"
	"sync/atomic"
)

var doOnce sync.Once
var commonLogger *zap.Logger
var debugLogger *zap.Logger
var inited atomic.Bool

func Logger() *zap.Logger {
	if !inited.Load() {
		panic("loger not inited")
	}
	return commonLogger
}

// DebugLogger instead of Logger always have debug level
func DebugLogger() *zap.Logger {
	if !inited.Load() {
		panic("loger not inited")
	}
	return debugLogger
}

func Init(app string, loglevel zapcore.Level) {
	doOnce.Do(func() {
		commonLogger = initLogger(app, loglevel)
		debugLogger = initLogger(app, zap.DebugLevel)
		inited.Store(true)
	})
}

func initLogger(app string, loglevel zapcore.Level) *zap.Logger {
	config := zap.NewProductionConfig()
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.TimeKey = "@timestamp"
	config.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	config.EncoderConfig.CallerKey = "line_number"
	config.Level.SetLevel(loglevel)
	logger, err := config.Build()
	if err != nil {
		panic("cant build logger: " + err.Error())
	}
	logger = logger.WithOptions(zap.AddStacktrace(zapcore.PanicLevel))
	logger = logger.With(zap.String("app", app))
	return logger
}

type buildXContextKey struct{}

func ToCtx(logger *zap.Logger, ctx context.Context) context.Context {
	return context.WithValue(ctx, buildXContextKey{}, logger)
}

func FromCtx(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return Logger()
	}
	value := ctx.Value(buildXContextKey{})
	if logger, ok := value.(*zap.Logger); ok && logger != nil {
		return logger
	}
	return Logger()
}
