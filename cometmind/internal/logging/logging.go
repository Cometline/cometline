package logging

import (
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// L returns the process-wide structured logger.
func L() *slog.Logger { return logger }

// Gin logs every HTTP request with the same slog logger used by agent internals.
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
		attrs := []any{
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		}
		if raw := c.Request.URL.RawQuery; raw != "" {
			attrs = append(attrs, "query", raw)
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		switch {
		case status >= 500:
			logger.Error("http.request", attrs...)
		case status >= 400:
			logger.Warn("http.request", attrs...)
		default:
			logger.Info("http.request", attrs...)
		}
	}
}
