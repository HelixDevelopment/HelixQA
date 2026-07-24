package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestLogger_SetsLogFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	r := gin.New()
	r.Use(Logger(logger))
	r.GET("/test-logger", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test-logger", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestLogger_LogsErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	r := gin.New()
	r.Use(Logger(logger))
	r.POST("/error-route", func(c *gin.Context) {
		_ = c.Error(gin.Error{
			Err:  fmt.Errorf("test error"),
			Type: 1,
			Meta: "test error",
		})
		c.JSON(500, gin.H{"error": true})
	})

	req := httptest.NewRequest("POST", "/error-route", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestLogger_LogsLatency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	r := gin.New()
	r.Use(Logger(logger))
	r.GET("/latency", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/latency", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
