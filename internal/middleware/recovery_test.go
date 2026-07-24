package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestRecovery_NoPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	r := gin.New()
	r.Use(Recovery(logger))
	r.GET("/ok", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRecovery_CatchesPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	r := gin.New()
	r.Use(Recovery(logger))
	r.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestRecovery_CatchesStringPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	r := gin.New()
	r.Use(Recovery(logger))
	r.GET("/panic-string", func(c *gin.Context) {
		panic("string panic value")
	})

	req := httptest.NewRequest("GET", "/panic-string", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestRecovery_CatchesErrorPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	r := gin.New()
	r.Use(Recovery(logger))
	r.GET("/panic-error", func(c *gin.Context) {
		panic(http.ErrAbortHandler)
	})

	req := httptest.NewRequest("GET", "/panic-error", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestRecovery_CatchesIntPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	r := gin.New()
	r.Use(Recovery(logger))
	r.GET("/panic-int", func(c *gin.Context) {
		panic(42)
	})

	req := httptest.NewRequest("GET", "/panic-int", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
