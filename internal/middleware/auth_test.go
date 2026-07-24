package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupAuthRouter(mw ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(mw...)
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	return r
}

func TestAuth_APIKeyBypassesJWT(t *testing.T) {
	logger := zap.NewNop()
	r := setupAuthRouter(Auth("", logger))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("X-API-Key", "test-key-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuth_MissingAuth(t *testing.T) {
	logger := zap.NewNop()
	r := setupAuthRouter(Auth("/nonexistent.pem", logger))

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuth_InvalidBearerFormat(t *testing.T) {
	logger := zap.NewNop()
	r := setupAuthRouter(Auth("/nonexistent.pem", logger))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Token abc123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuth_BearerWithoutToken(t *testing.T) {
	logger := zap.NewNop()
	r := setupAuthRouter(Auth("/tmp/nonexistent_key_file.pem", logger))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d (Bearer with empty token hits key file read)", w.Code, http.StatusInternalServerError)
	}
}

func TestAuth_NonExistentKeyFile(t *testing.T) {
	logger := zap.NewNop()
	r := setupAuthRouter(Auth("/tmp/nonexistent_key_file.pem", logger))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestOptionalAuth_NoAuth(t *testing.T) {
	logger := zap.NewNop()
	r := setupAuthRouter(OptionalAuth("", logger))

	req := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (OptionalAuth should pass through)", w.Code, http.StatusOK)
	}
}

func TestOptionalAuth_WithAPIKey(t *testing.T) {
	logger := zap.NewNop()
	r := setupAuthRouter(OptionalAuth("", logger))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("X-API-Key", "my-api-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestOptionalAuth_InvalidFormat(t *testing.T) {
	logger := zap.NewNop()
	r := setupAuthRouter(OptionalAuth("", logger))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Token badformat")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("OptionalAuth should pass through on invalid format, got %d", w.Code)
	}
}

func TestOptionalAuth_NonExistentKey(t *testing.T) {
	logger := zap.NewNop()
	r := setupAuthRouter(OptionalAuth("/tmp/nonexistent_key_file.pem", logger))

	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("OptionalAuth should pass through on missing key file, got %d", w.Code)
	}
}

func TestAuth_APIKeySetsContext(t *testing.T) {
	logger := zap.NewNop()
	gin.SetMode(gin.TestMode)
	r := gin.New()

	var authMethod string
	r.Use(Auth("", logger))
	r.GET("/check", func(c *gin.Context) {
		authMethod = c.GetString("auth_method")
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/check", nil)
	req.Header.Set("X-API-Key", "key-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if authMethod != "api_key" {
		t.Errorf("auth_method = %q, want api_key", authMethod)
	}
}
