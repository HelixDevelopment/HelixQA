package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/helix-seller/helix-seller/internal/observability"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func initMetrics() {
	meterProvider := sdkmetric.NewMeterProvider()
	meter := meterProvider.Meter("test")
	observability.InitMetrics(meter)
}

func TestMetrics_Success(t *testing.T) {
	initMetrics()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Metrics())
	r.GET("/test-metrics", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test-metrics", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestMetrics_ErrorStatus(t *testing.T) {
	initMetrics()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Metrics())
	r.GET("/error-route", func(c *gin.Context) {
		c.JSON(500, gin.H{"error": true})
	})

	req := httptest.NewRequest("GET", "/error-route", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestMetrics_4xxStatus(t *testing.T) {
	initMetrics()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Metrics())
	r.GET("/not-found", func(c *gin.Context) {
		c.JSON(404, gin.H{"error": "not found"})
	})

	req := httptest.NewRequest("GET", "/not-found", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestMetrics_201Status(t *testing.T) {
	initMetrics()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Metrics())
	r.POST("/create", func(c *gin.Context) {
		c.JSON(201, gin.H{"created": true})
	})

	req := httptest.NewRequest("POST", "/create", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
}

func TestTracing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Tracing("test-service"))
	r.GET("/test-tracing", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test-tracing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
