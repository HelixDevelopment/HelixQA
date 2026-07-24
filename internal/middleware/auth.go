package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type Claims struct {
	UserID     string `json:"user_id"`
	MerchantID string `json:"merchant_id"`
	Role       string `json:"role"`
	jwt.RegisteredClaims
}

func Auth(publicKeyPath string, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		apiKey := c.GetHeader("X-API-Key")

		if apiKey != "" {
			c.Set("auth_method", "api_key")
			c.Set("api_key", apiKey)
			c.Next()
			return
		}

		if authHeader == "" {
			logger.Warn("missing authentication",
				zap.String("path", c.Request.URL.Path),
				zap.String("request_id", c.GetString("request_id")),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			logger.Warn("invalid authorization header format",
				zap.String("path", c.Request.URL.Path),
				zap.String("request_id", c.GetString("request_id")),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization header format",
			})
			return
		}

		tokenString := parts[1]

		pubKeyData, err := os.ReadFile(publicKeyPath)
		if err != nil {
			logger.Error("failed to read public key", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
			return
		}

		pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubKeyData)
		if err != nil {
			logger.Error("failed to parse public key", zap.Error(err))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "internal server error",
			})
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return pubKey, nil
		})

		if err != nil {
			logger.Warn("invalid token",
				zap.Error(err),
				zap.String("path", c.Request.URL.Path),
				zap.String("request_id", c.GetString("request_id")),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok || !token.Valid {
			logger.Warn("invalid token claims",
				zap.String("path", c.Request.URL.Path),
				zap.String("request_id", c.GetString("request_id")),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid token claims",
			})
			return
		}

		c.Set("auth_method", "jwt")
		c.Set("user_id", claims.UserID)
		c.Set("merchant_id", claims.MerchantID)
		c.Set("role", claims.Role)
		c.Set("claims", claims)
		c.Next()
	}
}

func OptionalAuth(publicKeyPath string, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		apiKey := c.GetHeader("X-API-Key")

		if apiKey != "" {
			c.Set("auth_method", "api_key")
			c.Set("api_key", apiKey)
			c.Next()
			return
		}

		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.Next()
			return
		}

		tokenString := parts[1]

		pubKeyData, err := os.ReadFile(publicKeyPath)
		if err != nil {
			c.Next()
			return
		}

		pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubKeyData)
		if err != nil {
			c.Next()
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return pubKey, nil
		})

		if err != nil || !token.Valid {
			c.Next()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.Next()
			return
		}

		c.Set("auth_method", "jwt")
		c.Set("user_id", claims.UserID)
		c.Set("merchant_id", claims.MerchantID)
		c.Set("role", claims.Role)
		c.Set("claims", claims)
		c.Next()
	}
}
