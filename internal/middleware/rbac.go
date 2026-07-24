package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Role string

const (
	RoleRootAdmin    Role = "root_admin"
	RoleAccountAdmin Role = "account_admin"
	RoleUser         Role = "user"
)

func RequireRole(roles ...Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := c.GetString("role")
		if userRole == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			return
		}

		for _, r := range roles {
			if string(r) == userRole {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "insufficient permissions",
		})
	}
}
