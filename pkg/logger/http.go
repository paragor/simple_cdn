package logger

import (
	"context"
	"fmt"
	"github.com/felixge/httpsnoop"
	"github.com/google/uuid"
	"github.com/paragor/simple_cdn/pkg/user"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/http"
	"time"
)

func HttpSetLoggerMiddleware(forceEmitDebugLogging user.User, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestId := r.Header.Get("X-Request-ID")
		if len(requestId) == 0 {
			requestId = uuid.NewString()
			r.Header.Set("X-Request-ID", requestId)
		}
		ctx := r.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		var logger *zap.Logger
		if forceEmitDebugLogging.IsUser(r) {
			logger = DebugLogger()
		} else {
			logger = Logger()
		}
		r = r.WithContext(ToCtx(logger.With(zap.String("request_id", requestId)), ctx))
		handler.ServeHTTP(w, r)
	})
}

func HttpLoggingMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := FromCtx(r.Context())
		if !log.Level().Enabled(zapcore.DebugLevel) {
			handler.ServeHTTP(w, r)
			return
		}
		status := 0
		sentByte := 0
		w = httpsnoop.Wrap(w, httpsnoop.Hooks{
			WriteHeader: func(headerFunc httpsnoop.WriteHeaderFunc) httpsnoop.WriteHeaderFunc {
				return func(code int) {
					status = code
					headerFunc(code)
				}
			},
			Write: func(writeFunc httpsnoop.WriteFunc) httpsnoop.WriteFunc {
				return func(b []byte) (int, error) {
					s, err := writeFunc(b)
					sentByte += s
					return s, err
				}
			},
		})
		start := time.Now()
		handler.ServeHTTP(w, r)
		log.With(
			zap.Int("status_code", status),
			zap.Duration("request_duration", time.Now().Sub(start)),
			zap.String("request_path", r.URL.Path),
			zap.String("method", r.Method),
			zap.String("host", r.URL.Host),
			zap.String("remote_addr", r.Header.Get("X-Real-Ip")),
			zap.String("request_query", r.URL.RawQuery),
			zap.String("user_agent", r.Header.Get("User-Agent")),
			zap.Int("response_size", sentByte),
		).Debug("handle request")
	})
}

func HttpRecoveryMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				FromCtx(r.Context()).
					WithOptions(zap.AddStacktrace(zapcore.ErrorLevel)).
					With(zap.Error(fmt.Errorf("%v", err))).
					Error("panic on request handler")
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()

		handler.ServeHTTP(w, r)
	})
}
