package slog

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// func Slog() gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		start := time.Now()
// 		path := c.Request.URL.Path
// 		raw := c.Request.URL.RawQuery

// 		// 处理请求
// 		c.Next()

// 		// 请求结束后记录日志
// 		latency := time.Since(start)
// 		clientIP := c.ClientIP()
// 		method := c.Request.Method
// 		statusCode := c.Writer.Status()
// 		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

// 		if raw != "" {
// 			path = path + "?" + raw
// 		}

// 		// 根据状态码选择日志级别
// 		switch {
// 		case statusCode >= 500:
// 			slog.Error("server error",
// 				"status", statusCode,
// 				"latency", latency,
// 				"client_ip", clientIP,
// 				"method", method,
// 				"path", path,
// 				"error", errorMessage,
// 			)
// 		case statusCode >= 400:
// 			slog.Warn("client error",
// 				"status", statusCode,
// 				"latency", latency,
// 				"client_ip", clientIP,
// 				"method", method,
// 				"path", path,
// 				"error", errorMessage,
// 			)
// 		default:
// 			slog.Info("status", statusCode,
// 				"latency", latency,
// 				"client_ip", clientIP,
// 				"method", method,
// 				"path", path,
// 			)
// 		}
// 	}
// }

func SlogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 记录日志
		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		// attrs := []any{
		// 	"status", status,
		// 	"latency", latency,
		// 	"client_ip", clientIP,
		// 	"path", path,
		// }
		var msg string

		if errorMessage != "" {
			// attrs = append(attrs, "error", errorMessage)
			msg = fmt.Sprintf("[gin] %d %s %s%s %s %s err:%s", status, latency, query, clientIP, method, path, errorMessage)
		} else {

			msg = fmt.Sprintf("[gin] %d %s %s%s %s %s %s", status, latency, query, clientIP, method, path, query)
		}

		// 根据状态码选择日志级别
		switch {
		case status >= 500:
			slog.Error(msg)
		case status >= 400:
			slog.Warn(msg)
		default:
			slog.Info(msg)
		}
	}
}
