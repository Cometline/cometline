package logging

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger = newLogger(zapcore.ErrorLevel)

func newLogger(level zapcore.Level) *zap.Logger {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.AddSync(os.Stderr),
		level,
	)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

// ParseLevel maps a log level name to zapcore.Level.
func ParseLevel(raw string) (zapcore.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.ErrorLevel, fmt.Errorf("unknown log level %q (want debug, info, warn, or error)", raw)
	}
}

// Init replaces the process-wide logger at the given level.
func Init(level zapcore.Level) {
	logger = newLogger(level)
}

// L returns the process-wide structured logger.
func L() *zap.Logger { return logger }

// Gin logs every HTTP request with the same logger used by agent internals.
func Gin() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		status := c.Writer.Status()
		if path == "/api/v1/health" && status < 400 {
			return
		}
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Int64("latency_ms", time.Since(start).Milliseconds()),
			zap.String("client_ip", c.ClientIP()),
		}
		if raw := c.Request.URL.RawQuery; raw != "" {
			fields = append(fields, zap.String("query", raw))
		}
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
		}

		switch {
		case status >= 500:
			logger.Error("http.request", fields...)
		case status >= 400:
			logger.Warn("http.request", fields...)
		default:
			logger.Info("http.request", fields...)
		}
	}
}
