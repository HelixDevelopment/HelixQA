package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupRBACRouter(role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", role)
		c.Next()
	})
	r.GET("/admin", RequireRole(RoleRootAdmin, RoleAccountAdmin), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	r.GET("/user-only", RequireRole(RoleUser), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	r.GET("/any-role", RequireRole(RoleRootAdmin, RoleAccountAdmin, RoleUser), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	return r
}

func TestRequireRole_RootAdminAllowed(t *testing.T) {
	r := setupRBACRouter("root_admin")
	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireRole_AccountAdminAllowed(t *testing.T) {
	r := setupRBACRouter("account_admin")
	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireRole_UserForbidden(t *testing.T) {
	r := setupRBACRouter("user")
	req := httptest.NewRequest("GET", "/admin", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequireRole_EmptyRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequireRole(RoleRootAdmin))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (empty role should be unauthorized)", w.Code, http.StatusUnauthorized)
	}
}

func TestRequireRole_AnyRoleAllowed(t *testing.T) {
	roles := []string{"root_admin", "account_admin", "user"}
	for _, role := range roles {
		r := setupRBACRouter(role)
		req := httptest.NewRequest("GET", "/any-role", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("role %q: status = %d, want %d", role, w.Code, http.StatusOK)
		}
	}
}

func TestRequireRole_UserOnly(t *testing.T) {
	r := setupRBACRouter("user")
	req := httptest.NewRequest("GET", "/user-only", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireRole_RootAdminDeniedUserOnly(t *testing.T) {
	r := setupRBACRouter("root_admin")
	req := httptest.NewRequest("GET", "/user-only", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}
