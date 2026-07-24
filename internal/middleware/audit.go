package middleware

import (
	"bytes"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func Audit(db *pgxpool.Pool, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		start := time.Now()

		rw := &responseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = rw

		c.Next()

		latency := time.Since(start)

		go func() {
			ctx := c.Request.Context()

			actorID := c.GetString("user_id")
			merchantID := c.GetString("merchant_id")

			resourceType := ""
			resourceID := ""
			path := c.Request.URL.Path

			_, err := db.Exec(ctx,
				`INSERT INTO audit_logs (actor_id, merchant_id, action, resource_type, resource_id, method, path, status_code, ip_address, user_agent, request_body_size, response_body_size, latency_ms, created_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
				actorID,
				merchantID,
				c.Request.Method,
				resourceType,
				resourceID,
				c.Request.Method,
				path,
				c.Writer.Status(),
				c.ClientIP(),
				c.Request.UserAgent(),
				len(bodyBytes),
				rw.body.Len(),
				latency.Milliseconds(),
				time.Now().UTC(),
			)
			if err != nil {
				logger.Error("failed to write audit log",
					zap.Error(err),
					zap.String("path", path),
					zap.String("request_id", c.GetString("request_id")),
				)
			}
		}()
	}
}
