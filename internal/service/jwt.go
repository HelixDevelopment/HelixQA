package service

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/helix-seller/helix-seller/internal/config"
)

type JWTService struct {
	privateKey    *rsa.PrivateKey
	publicKey     *rsa.PublicKey
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

func NewJWTService(cfg *config.Config) (*JWTService, error) {
	privateKeyData, err := os.ReadFile(cfg.JWTPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(privateKeyData)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	publicKeyData, err := os.ReadFile(cfg.JWTPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}

	block, _ := pem.Decode(publicKeyData)
	if block == nil {
		return nil, fmt.Errorf("decode public key PEM")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	publicKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}

	return &JWTService{
		privateKey:    privateKey,
		publicKey:     publicKey,
		accessExpiry:  cfg.JWTAccessExpiry,
		refreshExpiry: cfg.JWTRefreshExpiry,
	}, nil
}

func (s *JWTService) GenerateAccessToken(userID uuid.UUID, email, role string, merchantID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub":         userID.String(),
		"email":       email,
		"role":        role,
		"merchant_id": merchantID.String(),
		"exp":         time.Now().Add(s.accessExpiry).Unix(),
		"iat":         time.Now().Unix(),
		"jti":         uuid.New().String(),
		"token_type":  "access",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.privateKey)
}

func (s *JWTService) GenerateRefreshToken(userID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub":        userID.String(),
		"exp":        time.Now().Add(s.refreshExpiry).Unix(),
		"iat":        time.Now().Unix(),
		"jti":        uuid.New().String(),
		"token_type": "refresh",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.privateKey)
}

func (s *JWTService) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func (s *JWTService) PublicKey() *rsa.PublicKey {
	return s.publicKey
}
