package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/helix-seller/helix-seller/internal/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		observability.RequestDuration.Record(c.Request.Context(), duration,
			metric.WithAttributes(
				attribute.String("method", c.Request.Method),
				attribute.String("path", c.FullPath()),
				attribute.String("status", status),
			),
		)

		observability.RequestCounter.Add(c.Request.Context(), 1,
			metric.WithAttributes(
				attribute.String("method", c.Request.Method),
				attribute.String("path", c.FullPath()),
				attribute.String("status", status),
			),
		)

		if c.Writer.Status() >= 400 {
			observability.RequestErrors.Add(c.Request.Context(), 1,
				metric.WithAttributes(
					attribute.String("method", c.Request.Method),
					attribute.String("path", c.FullPath()),
					attribute.String("status", status),
				),
			)
		}
	}
}
