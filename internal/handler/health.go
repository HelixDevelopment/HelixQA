package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type dbPinger interface {
	Ping(ctx context.Context) error
}

type redisClient interface {
	Ping(ctx context.Context) *redis.StatusCmd
}

type HealthHandler struct {
	db     dbPinger
	redis  redisClient
	logger *zap.Logger
}

func NewHealthHandler(db *pgxpool.Pool, redis *redis.Client, logger *zap.Logger) *HealthHandler {
	return &HealthHandler{db: db, redis: redis, logger: logger}
}

func (h *HealthHandler) Health(c *gin.Context) {
	status := gin.H{"status": "healthy"}
	httpStatus := http.StatusOK

	if err := h.db.Ping(c.Request.Context()); err != nil {
		status["postgresql"] = gin.H{"status": "unhealthy", "error": err.Error()}
		httpStatus = http.StatusServiceUnavailable
	} else {
		status["postgresql"] = gin.H{"status": "healthy"}
	}

	if err := h.redis.Ping(c.Request.Context()).Err(); err != nil {
		status["redis"] = gin.H{"status": "unhealthy", "error": err.Error()}
		httpStatus = http.StatusServiceUnavailable
	} else {
		status["redis"] = gin.H{"status": "healthy"}
	}

	c.JSON(httpStatus, status)
}

func (h *HealthHandler) Readiness(c *gin.Context) {
	if err := h.db.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}
