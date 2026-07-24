package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Claims struct {
	UserID     string `json:"user_id"`
	MerchantID string `json:"merchant_id"`
	Role       string `json:"role"`
	jwt.RegisteredClaims
}

func setAuthContext(c *gin.Context, claims *Claims) {
	c.Set("auth_method", "jwt")
	c.Set("user_id", claims.UserID)
	c.Set("merchant_id", claims.MerchantID)
	c.Set("role", claims.Role)
	c.Set("claims", claims)
	c.Set("token_jti", claims.ID)
	if claims.ExpiresAt != nil {
		c.Set("token_exp", claims.ExpiresAt.Time)
	}
}

func NewAuthMiddleware(publicKeyPath string, redisClient *redis.Client, logger *zap.Logger) (gin.HandlerFunc, error) {
	pubKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubKeyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// FIXME: API key validation (X-API-Key) needs a DB-backed implementation.
		// Currently skipped intentionally — require JWT for all authenticated routes.

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

		// Check token blacklist
		if redisClient != nil && claims.ID != "" {
			blacklisted, err := redisClient.Exists(c.Request.Context(), "token_blacklist:"+claims.ID).Result()
			if err == nil && blacklisted > 0 {
				logger.Warn("blacklisted token used",
					zap.String("jti", claims.ID),
					zap.String("path", c.Request.URL.Path),
					zap.String("request_id", c.GetString("request_id")),
				)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "token has been revoked",
				})
				return
			}
		}

		setAuthContext(c, claims)
		c.Next()
	}, nil
}

func Auth(publicKeyPath string, logger *zap.Logger) gin.HandlerFunc {
	return createAuthHandler(publicKeyPath, logger, true)
}

func OptionalAuth(publicKeyPath string, logger *zap.Logger) gin.HandlerFunc {
	return createAuthHandler(publicKeyPath, logger, false)
}

func createAuthHandler(publicKeyPath string, logger *zap.Logger, required bool) gin.HandlerFunc {
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
			if required {
				logger.Warn("missing authentication",
					zap.String("path", c.Request.URL.Path),
					zap.String("request_id", c.GetString("request_id")),
				)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "authentication required",
				})
			} else {
				c.Next()
			}
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			if required {
				logger.Warn("invalid authorization header format",
					zap.String("path", c.Request.URL.Path),
					zap.String("request_id", c.GetString("request_id")),
				)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "invalid authorization header format",
				})
			} else {
				c.Next()
			}
			return
		}

		tokenString := parts[1]

		pubKeyData, err := os.ReadFile(publicKeyPath)
		if err != nil {
			if required {
				logger.Error("failed to read public key", zap.Error(err))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
			} else {
				c.Next()
			}
			return
		}

		pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubKeyData)
		if err != nil {
			if required {
				logger.Error("failed to parse public key", zap.Error(err))
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
			} else {
				c.Next()
			}
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return pubKey, nil
		})

		if err != nil || !token.Valid {
			if required {
				logger.Warn("invalid token",
					zap.Error(err),
					zap.String("path", c.Request.URL.Path),
					zap.String("request_id", c.GetString("request_id")),
				)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "invalid or expired token",
				})
			} else {
				c.Next()
			}
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			if required {
				logger.Warn("invalid token claims",
					zap.String("path", c.Request.URL.Path),
					zap.String("request_id", c.GetString("request_id")),
				)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error": "invalid token claims",
				})
			} else {
				c.Next()
			}
			return
		}

		setAuthContext(c, claims)
		c.Next()
	}
}

// NewAuthMiddleware is the preferred approach — it loads the public key once at creation time
// and supports Redis-backed token blacklist checking.
