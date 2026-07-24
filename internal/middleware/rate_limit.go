package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func RateLimit(client *redis.Client, rps int, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		key := fmt.Sprintf("rate_limit:%s:%d", c.ClientIP(), time.Now().Unix()/60)

		count, err := client.Incr(ctx, key).Result()
		if err != nil {
			logger.Error("rate limit redis error", zap.Error(err))
			c.Next()
			return
		}

		if count == 1 {
			client.Expire(ctx, key, 2*time.Minute)
		}

		remaining := rps - int(count)
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rps))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, remaining)))

		if count > int64(rps) {
			logger.Warn("rate limit exceeded",
				zap.String("ip", c.ClientIP()),
				zap.String("path", c.Request.URL.Path),
				zap.Int64("count", count),
				zap.Int("limit", rps),
			)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}

		c.Next()
	}
}
