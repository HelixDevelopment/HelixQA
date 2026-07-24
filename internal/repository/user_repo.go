package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, user *model.User) error {
	var merchantID *uuid.UUID
	if user.MerchantID != uuid.Nil {
		merchantID = &user.MerchantID
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, name, role, merchant_id, is_active, mfa_enabled, mfa_secret)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		user.ID, user.Email, user.PasswordHash, user.Name, user.Role,
		merchantID, user.IsActive, user.MfaEnabled, user.MfaSecret,
	)
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	user := &model.User{}
	var merchantID *uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, name, role, merchant_id, is_active, mfa_enabled, mfa_secret, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role,
		&merchantID, &user.IsActive, &user.MfaEnabled, &user.MfaSecret,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user by id: %w", err)
	}
	if merchantID != nil {
		user.MerchantID = *merchantID
	}
	return user, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	user := &model.User{}
	var merchantID *uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, name, role, merchant_id, is_active, mfa_enabled, mfa_secret, created_at, updated_at
		 FROM users WHERE email = $1`, email,
	).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role,
		&merchantID, &user.IsActive, &user.MfaEnabled, &user.MfaSecret,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user by email: %w", err)
	}
	if merchantID != nil {
		user.MerchantID = *merchantID
	}
	return user, nil
}

func (r *UserRepo) Update(ctx context.Context, user *model.User) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE users SET name = $1, role = $2, is_active = $3, mfa_enabled = $4, mfa_secret = $5, email = $7, updated_at = NOW()
		 WHERE id = $6`,
		user.Name, user.Role, user.IsActive, user.MfaEnabled, user.MfaSecret, user.ID, user.Email,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *UserRepo) ListByMerchant(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]*model.User, int64, error) {
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE merchant_id = $1`, merchantID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, email, password_hash, name, role, merchant_id, is_active, mfa_enabled, mfa_secret, created_at, updated_at
		 FROM users WHERE merchant_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		merchantID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		user := &model.User{}
		var merchantID *uuid.UUID
		if err := rows.Scan(
			&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role,
			&merchantID, &user.IsActive, &user.MfaEnabled, &user.MfaSecret,
			&user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		if merchantID != nil {
			user.MerchantID = *merchantID
		}
		users = append(users, user)
	}

	return users, total, nil
}

func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
