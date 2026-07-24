package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMetricsHandler_ReturnsMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/metrics", MetricsHandler())

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty metrics body")
	}
}

func TestMetricsHandler_ContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/metrics", MetricsHandler())

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct == "" {
		t.Error("expected Content-Type header")
	}
}
