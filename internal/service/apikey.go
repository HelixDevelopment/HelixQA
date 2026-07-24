package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type ApiKeyService struct {
	db *pgxpool.Pool
}

func NewApiKeyService(db *pgxpool.Pool) *ApiKeyService {
	return &ApiKeyService{db: db}
}

func (s *ApiKeyService) Create(ctx context.Context, merchantID, userID uuid.UUID, name string, scopes []string, rateLimit int, expiresAt *time.Time) (string, *model.ApiKey, error) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", nil, fmt.Errorf("generate api key: %w", err)
	}

	fullKey := hex.EncodeToString(keyBytes)
	keyHash := sha256.Sum256([]byte(fullKey))
	hashHex := hex.EncodeToString(keyHash[:])
	keyPrefix := fullKey[:8]

	apiKey := &model.ApiKey{
		ID:         uuid.New(),
		MerchantID: merchantID,
		UserID:     userID,
		Name:       name,
		KeyPrefix:  keyPrefix,
		KeyHash:    hashHex,
		Scopes:     scopes,
		RateLimit:  rateLimit,
		IsActive:   true,
		ExpiresAt:  expiresAt,
		CreatedAt:  time.Now(),
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO api_keys (id, merchant_id, user_id, name, key_prefix, key_hash, scopes, rate_limit, is_active, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		apiKey.ID, apiKey.MerchantID, apiKey.UserID, apiKey.Name,
		apiKey.KeyPrefix, apiKey.KeyHash, apiKey.Scopes, apiKey.RateLimit,
		apiKey.IsActive, apiKey.ExpiresAt,
	)
	if err != nil {
		return "", nil, fmt.Errorf("insert api key: %w", err)
	}

	return fullKey, apiKey, nil
}

func (s *ApiKeyService) Validate(ctx context.Context, key string) (*model.ApiKey, error) {
	keyHash := sha256.Sum256([]byte(key))
	hashHex := hex.EncodeToString(keyHash[:])

	apiKey := &model.ApiKey{}
	err := s.db.QueryRow(ctx,
		`SELECT id, merchant_id, user_id, name, key_prefix, key_hash, scopes, rate_limit, is_active, expires_at, created_at, last_used_at
		 FROM api_keys WHERE key_hash = $1`, hashHex,
	).Scan(
		&apiKey.ID, &apiKey.MerchantID, &apiKey.UserID, &apiKey.Name,
		&apiKey.KeyPrefix, &apiKey.KeyHash, &apiKey.Scopes, &apiKey.RateLimit,
		&apiKey.IsActive, &apiKey.ExpiresAt, &apiKey.CreatedAt, &apiKey.LastUsedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, model.ErrUnauthorized
	}
	if err != nil {
		return nil, fmt.Errorf("query api key: %w", err)
	}

	if !apiKey.IsActive {
		return nil, model.ErrUnauthorized
	}

	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, model.ErrUnauthorized
	}

	_, err = s.db.Exec(ctx,
		`UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`, apiKey.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update last used: %w", err)
	}

	return apiKey, nil
}

func (s *ApiKeyService) ListByMerchant(ctx context.Context, merchantID uuid.UUID) ([]*model.ApiKey, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, merchant_id, user_id, name, key_prefix, scopes, rate_limit, is_active, expires_at, created_at, last_used_at
		 FROM api_keys WHERE merchant_id = $1 ORDER BY created_at DESC`, merchantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []*model.ApiKey
	for rows.Next() {
		k := &model.ApiKey{}
		if err := rows.Scan(
			&k.ID, &k.MerchantID, &k.UserID, &k.Name,
			&k.KeyPrefix, &k.Scopes, &k.RateLimit,
			&k.IsActive, &k.ExpiresAt, &k.CreatedAt, &k.LastUsedAt,
		); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *ApiKeyService) Revoke(ctx context.Context, keyID uuid.UUID) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE api_keys SET is_active = false WHERE id = $1`, keyID,
	)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
