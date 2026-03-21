package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// func RequestID() gin.HandlerFunc {
// 	return func(c *gin.Context) {

// 		id := uuid.NewString()

// 		c.Set(string(RequestIDKey), id)

// 		ctx := c.Request.Context()
// 		ctx = contextWithRequestID(ctx, id)

// 		c.Request = c.Request.WithContext(ctx)

// 		c.Writer.Header().Set("X-Request-ID", id)

// 		c.Next()
// 	}
// }

// func contextWithRequestID(ctx context.Context, id string) context.Context {
// 	return context.WithValue(ctx, RequestIDKey, id)
// }

func GinLogger() gin.HandlerFunc {

	return func(c *gin.Context) {

		start := time.Now()

		c.Next()

		latency := time.Since(start)

		status := c.Writer.Status()

		// reqID, _ := c.Get(string(RequestIDKey))

		slog.Info("http request",
			"status", status,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"ip", c.ClientIP(),
			"latency", latency.String(),
			// "request_id", reqID,
		)
	}
}

func Recovery() gin.HandlerFunc {

	return func(c *gin.Context) {

		defer func() {

			if err := recover(); err != nil {

				// reqID, _ := c.Get(string(RequestIDKey))

				slog.Error("panic recovered",
					"error", err,
					"path", c.Request.URL.Path,
					// "request_id", reqID,
				)

				c.AbortWithStatus(500)
			}

		}()

		c.Next()
	}
}
