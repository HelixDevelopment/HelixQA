package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/helix-seller/helix-seller/internal/model"
	"github.com/helix-seller/helix-seller/internal/repository"
)

type AuthService struct {
	userRepo *repository.UserRepo
}

func NewAuthService(userRepo *repository.UserRepo) *AuthService {
	return &AuthService{userRepo: userRepo}
}

func (s *AuthService) HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(hash), nil
}

func (s *AuthService) VerifyPassword(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, ":")
	if len(parts) != 2 {
		return false, model.NewValidationError("invalid hash format")
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false, model.NewValidationError("invalid hash format")
	}

	expectedHash, err := hex.DecodeString(parts[1])
	if err != nil {
		return false, model.NewValidationError("invalid hash format")
	}

	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	if len(hash) != len(expectedHash) {
		return false, nil
	}

	for i := range hash {
		if hash[i] != expectedHash[i] {
			return false, nil
		}
	}
	return true, nil
}

func (s *AuthService) Authenticate(ctx context.Context, email, password string) (*model.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, model.ErrUnauthorized
	}

	if !user.IsActive {
		return nil, model.ErrForbidden
	}

	ok, err := s.VerifyPassword(password, user.PasswordHash)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, model.ErrUnauthorized
	}

	return user, nil
}
