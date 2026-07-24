package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		latency := time.Since(start)

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("request_id", c.GetString("request_id")),
			zap.String("client_ip", c.ClientIP()),
		}

		if len(c.Errors) > 0 {
			for _, e := range c.Errors {
				logger.Error("request error",
					append(fields, zap.String("error", e.Error()))...,
				)
			}
		} else {
			logger.Info("request", fields...)
		}
	}
}
