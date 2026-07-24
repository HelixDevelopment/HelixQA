package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- RateLimit ---

func TestRateLimit_RedisError_ContinuesToHandler(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:1"})
	logger := zap.NewNop()
	mw := RateLimit(client, 100, logger)

	r := gin.New()
	r.Use(mw)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (redis error should pass through)", w.Code, http.StatusOK)
	}
}

func TestRateLimit_NilRedis_ContinuesToHandler(t *testing.T) {
	logger := zap.NewNop()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	mw := RateLimit(client, 100, logger)

	r := gin.New()
	r.Use(mw)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, nil)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- Audit ---

func TestAudit_ResponseWriter_CapturesBody(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	rw := &responseWriter{
		ResponseWriter: c.Writer,
		body:           bytes.NewBufferString(""),
	}

	n, err := rw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("bytes written = %d, want 5", n)
	}
	if rw.body.String() != "hello" {
		t.Errorf("body = %q, want %q", rw.body.String(), "hello")
	}
}

func TestAudit_ResponseWriter_MultipleWrites(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	rw := &responseWriter{
		ResponseWriter: c.Writer,
		body:           bytes.NewBufferString(""),
	}

	rw.Write([]byte("first"))
	rw.Write([]byte("second"))
	if rw.body.String() != "firstsecond" {
		t.Errorf("body = %q, want %q", rw.body.String(), "firstsecond")
	}
}

// --- Tracing ---

func TestTracing_ReturnsHandler(t *testing.T) {
	mw := Tracing("test-service")
	if mw == nil {
		t.Fatal("Tracing returned nil")
	}
}

func TestTracing_WithEmptyServiceName(t *testing.T) {
	mw := Tracing("")
	r := gin.New()
	r.Use(mw)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, nil)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- RequestSizeLimit ---

func TestRequestSizeLimit_SmallBody_Passes(t *testing.T) {
	mw := RequestSizeLimit(1024)
	r := gin.New()
	r.Use(mw)
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, nil)
	})

	w := httptest.NewRecorder()
	body := strings.NewReader("small body")
	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequestSizeLimit_EmptyBody_Passes(t *testing.T) {
	mw := RequestSizeLimit(1024)
	r := gin.New()
	r.Use(mw)
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, nil)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequestSizeLimit_ZeroLimit(t *testing.T) {
	mw := RequestSizeLimit(0)
	r := gin.New()
	r.Use(mw)
	r.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, nil)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", strings.NewReader("data"))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- Logger ---

func TestLogger_DifferentHTTPMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			logger := zap.NewNop()
			mw := Logger(logger)

			r := gin.New()
			r.Use(mw)
			r.Any("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, nil)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/test", nil)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
			}
		})
	}
}

// --- Recovery ---

func TestRecovery_DifferentPanicValues(t *testing.T) {
	panicValues := []interface{}{
		"string panic",
		42,
		3.14,
		true,
		nil,
		struct{}{},
	}
	for _, pv := range panicValues {
		t.Run("panic", func(t *testing.T) {
			logger := zap.NewNop()
			mw := Recovery(logger)

			r := gin.New()
			r.Use(mw)
			r.GET("/test", func(c *gin.Context) {
				panic(pv)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)
			r.ServeHTTP(w, req)

			if w.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
			}
		})
	}
}

func TestRecovery_NoPanic_StillWorks(t *testing.T) {
	logger := zap.NewNop()
	mw := Recovery(logger)

	r := gin.New()
	r.Use(mw)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// --- CORS ---

func TestCORS_AllMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			mw := CORS()
			r := gin.New()
			r.Use(mw)
			r.Any("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, nil)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/test", nil)
			req.Header.Set("Origin", "https://example.com")
			r.ServeHTTP(w, req)

			if method == "OPTIONS" {
				if w.Code != http.StatusNoContent {
					t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
				}
			} else {
				if w.Code != http.StatusOK {
					t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
				}
			}
		})
	}
}

func TestCORS_PreflightRequest(t *testing.T) {
	mw := CORS()
	r := gin.New()
	r.Use(mw)
	r.Any("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, nil)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("Access-Control-Allow-Origin header not set")
	}
}

// --- SecurityHeaders ---

func TestSecurityHeaders_AllPresent(t *testing.T) {
	mw := SecurityHeaders()
	r := gin.New()
	r.Use(mw)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, nil)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	expectedHeaders := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Strict-Transport-Security",
		"Content-Security-Policy",
		"Referrer-Policy",
		"Cache-Control",
		"Pragma",
	}

	for _, h := range expectedHeaders {
		if w.Header().Get(h) == "" {
			t.Errorf("header %q not set", h)
		}
	}
}

// --- RequestID ---

func TestRequestID_GeneratesNewID(t *testing.T) {
	mw := RequestID()
	r := gin.New()
	r.Use(mw)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, nil)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header not set")
	}
}

func TestRequestID_PreservesExistingID(t *testing.T) {
	mw := RequestID()
	r := gin.New()
	r.Use(mw)
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, nil)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "my-custom-id")
	r.ServeHTTP(w, req)

	if w.Header().Get("X-Request-ID") != "my-custom-id" {
		t.Errorf("X-Request-ID = %q, want %q", w.Header().Get("X-Request-ID"), "my-custom-id")
	}
}
