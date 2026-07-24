package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type mockDB struct {
	pingErr error
}

func (m *mockDB) Ping(_ context.Context) error {
	return m.pingErr
}

type mockRedis struct {
	pingErr error
}

func (m *mockRedis) Ping(_ context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(context.Background())
	if m.pingErr != nil {
		cmd.SetErr(m.pingErr)
	}
	return cmd
}

func setupHealthRouter(h *HealthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", h.Health)
	r.GET("/health/ready", h.Readiness)
	r.GET("/health/live", h.Liveness)
	return r
}

func newTestHealthHandler(dbErr, redisErr error) *HealthHandler {
	return &HealthHandler{
		db:     &mockDB{pingErr: dbErr},
		redis:  &mockRedis{pingErr: redisErr},
		logger: zap.NewNop(),
	}
}

func TestHealth_AllHealthy(t *testing.T) {
	h := newTestHealthHandler(nil, nil)
	r := setupHealthRouter(h)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("status = %v, want healthy", resp["status"])
	}

	pg, ok := resp["postgresql"].(map[string]interface{})
	if !ok {
		t.Fatal("missing postgresql key")
	}
	if pg["status"] != "healthy" {
		t.Errorf("postgresql.status = %v, want healthy", pg["status"])
	}

	red, ok := resp["redis"].(map[string]interface{})
	if !ok {
		t.Fatal("missing redis key")
	}
	if red["status"] != "healthy" {
		t.Errorf("redis.status = %v, want healthy", red["status"])
	}
}

func TestHealth_DBUnhealthy(t *testing.T) {
	h := newTestHealthHandler(errors.New("db down"), nil)
	r := setupHealthRouter(h)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["status"] != "healthy" {
		t.Errorf("overall status = %v, want healthy", resp["status"])
	}

	pg := resp["postgresql"].(map[string]interface{})
	if pg["status"] != "unhealthy" {
		t.Errorf("postgresql.status = %v, want unhealthy", pg["status"])
	}
	if pg["error"] != "db down" {
		t.Errorf("postgresql.error = %v, want 'db down'", pg["error"])
	}
}

func TestHealth_RedisUnhealthy(t *testing.T) {
	h := newTestHealthHandler(nil, errors.New("redis down"))
	r := setupHealthRouter(h)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	red := resp["redis"].(map[string]interface{})
	if red["status"] != "unhealthy" {
		t.Errorf("redis.status = %v, want unhealthy", red["status"])
	}
	if red["error"] != "redis down" {
		t.Errorf("redis.error = %v, want 'redis down'", red["error"])
	}
}

func TestHealth_BothUnhealthy(t *testing.T) {
	h := newTestHealthHandler(errors.New("db down"), errors.New("redis down"))
	r := setupHealthRouter(h)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestReadiness_Ready(t *testing.T) {
	h := newTestHealthHandler(nil, nil)
	r := setupHealthRouter(h)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["status"] != "ready" {
		t.Errorf("status = %v, want ready", resp["status"])
	}
}

func TestReadiness_NotReady(t *testing.T) {
	h := newTestHealthHandler(errors.New("db down"), nil)
	r := setupHealthRouter(h)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["status"] != "not ready" {
		t.Errorf("status = %v, want 'not ready'", resp["status"])
	}
}

func TestLiveness(t *testing.T) {
	h := newTestHealthHandler(nil, nil)
	r := setupHealthRouter(h)

	req := httptest.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["status"] != "alive" {
		t.Errorf("status = %v, want alive", resp["status"])
	}
}
