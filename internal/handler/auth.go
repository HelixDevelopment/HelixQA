package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
	"github.com/helix-seller/helix-seller/internal/service"
)

type AuthHandler struct {
	authSvc     *service.AuthService
	jwtSvc      *service.JWTService
	mfaSvc      *service.MFAService
	userRepo    *repository.UserRepo
	redisClient *redis.Client
}

func NewAuthHandler(authSvc *service.AuthService, jwtSvc *service.JWTService, mfaSvc *service.MFAService, userRepo *repository.UserRepo, redisClient *redis.Client) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, jwtSvc: jwtSvc, mfaSvc: mfaSvc, userRepo: userRepo, redisClient: redisClient}
}

// POST /auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=12"`
		Name     string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hash, err := h.authSvc.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}
	user := &model.User{
		ID:           uuid.New(),
		Email:        req.Email,
		PasswordHash: hash,
		Name:         req.Name,
		Role:         model.RoleUser,
		IsActive:     true,
	}
	if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "user already exists"})
		return
	}
	accessToken, err := h.jwtSvc.GenerateAccessToken(user.ID, user.Email, string(user.Role), user.MerchantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	refreshToken, err := h.jwtSvc.GenerateRefreshToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          user,
	})
}

// POST /auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.authSvc.Authenticate(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if user.MfaEnabled {
		c.JSON(http.StatusOK, gin.H{"mfa_required": true, "user_id": user.ID})
		return
	}
	accessToken, err := h.jwtSvc.GenerateAccessToken(user.ID, user.Email, string(user.Role), user.MerchantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	refreshToken, err := h.jwtSvc.GenerateRefreshToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// POST /auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	claims, err := h.jwtSvc.ValidateToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}
	userID, err := uuid.Parse(claims["sub"].(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	accessToken, err := h.jwtSvc.GenerateAccessToken(user.ID, user.Email, string(user.Role), user.MerchantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"})
		return
	}
	refreshToken, err := h.jwtSvc.GenerateRefreshToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// POST /auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" && h.redisClient != nil {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			claims, err := h.jwtSvc.ValidateToken(parts[1])
			if err == nil {
				if jti, ok := claims["jti"]; ok {
					jtiStr := fmt.Sprint(jti)
					if exp, ok := claims["exp"]; ok {
						expUnix := int64(exp.(float64))
						expTime := time.Unix(expUnix, 0)
						ttl := time.Until(expTime)
						if ttl > 0 {
							h.redisClient.Set(c.Request.Context(), "token_blacklist:"+jtiStr, true, ttl)
						}
					}
				}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// POST /auth/mfa/setup
func (h *AuthHandler) SetupMFA(c *gin.Context) {
	userID := c.GetString("user_id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	user, err := h.userRepo.GetByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	secret, err := h.mfaSvc.GenerateSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate secret"})
		return
	}
	user.MfaSecret = &secret
	h.userRepo.Update(c.Request.Context(), user)
	recoveryCodes, _ := h.mfaSvc.GenerateRecoveryCodes(8)
	c.JSON(http.StatusOK, gin.H{
		"secret":         secret,
		"recovery_codes": recoveryCodes,
		"totp_url":       h.mfaSvc.TotpURL("HelixSeller", user.Email, secret),
	})
}

// POST /auth/mfa/verify
func (h *AuthHandler) VerifyMFA(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
		Code   string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	uid, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	user, err := h.userRepo.GetByID(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if user.MfaSecret == nil || !h.mfaSvc.Verify(*user.MfaSecret, req.Code) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid MFA code"})
		return
	}
	user.MfaEnabled = true
	h.userRepo.Update(c.Request.Context(), user)
	accessToken, err := h.jwtSvc.GenerateAccessToken(user.ID, user.Email, string(user.Role), user.MerchantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate access token"})
		return
	}
	refreshToken, err := h.jwtSvc.GenerateRefreshToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate refresh token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}
