package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger gin 请求日志中间件（kv 结构化输出，[GIN] 前缀与 gin 默认 Logger 一致）。
// 按状态码分级：>=500 Error、>=400 Warn、其余 Info。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.Request.URL.Path
		if query := c.Request.URL.RawQuery; query != "" {
			path += "?" + query
		}

		attrs := []any{
			"status", c.Writer.Status(),
			"method", c.Request.Method,
			"path", path,
			"ip", c.ClientIP(),
			"latency", time.Since(start).String(),
		}
		if errMsg := c.Errors.ByType(gin.ErrorTypePrivate).String(); errMsg != "" {
			attrs = append(attrs, "error", errMsg)
		}

		switch {
		case c.Writer.Status() >= http.StatusInternalServerError:
			slog.Error("[GIN]", attrs...)
		case c.Writer.Status() >= http.StatusBadRequest:
			slog.Warn("[GIN]", attrs...)
		default:
			slog.Info("[GIN]", attrs...)
		}
	}
}

// Recovery 捕获 handler panic，记录错误并返回 500，防止进程崩溃。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"path", c.Request.URL.Path,
				)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()

		c.Next()
	}
}
