package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupRouter(mw ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mw...)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	r.Handle("OPTIONS", "/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	return r
}

func TestRequestID_GeneratesUUID(t *testing.T) {
	r := setupRouter(RequestID())
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID header to be set")
	}
}

func TestRequestID_PreservesExisting(t *testing.T) {
	r := setupRouter(RequestID())
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "my-custom-id")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") != "my-custom-id" {
		t.Errorf("expected my-custom-id, got %s", w.Header().Get("X-Request-ID"))
	}
}

func TestCORS_SetsHeaders(t *testing.T) {
	r := setupRouter(CORS())
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected Access-Control-Allow-Origin *")
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Access-Control-Allow-Methods")
	}
}

func TestCORS_HandlesOptions(t *testing.T) {
	r := setupRouter(CORS())
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("expected 204 for OPTIONS, got %d", w.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	r := setupRouter(SecurityHeaders())
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	tests := []struct {
		header   string
		expected string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Content-Security-Policy", "default-src 'self'"},
		{"Strict-Transport-Security", "max-age=31536000; includeSubDomains"},
		{"Cache-Control", "no-store, no-cache, must-revalidate"},
		{"Pragma", "no-cache"},
	}

	for _, tt := range tests {
		got := w.Header().Get(tt.header)
		if got != tt.expected {
			t.Errorf("%s: expected %q, got %q", tt.header, tt.expected, got)
		}
	}
}

func TestRequestSize_LimitsBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestSizeLimit(100))
	r.POST("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// Small body should pass
	body := make([]byte, 50)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for small body, got %d", w.Code)
	}
}
