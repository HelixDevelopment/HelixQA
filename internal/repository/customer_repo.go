package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/helix-seller/helix-seller/internal/model"
)

type CustomerRepo struct {
	db *pgxpool.Pool
}

func NewCustomerRepo(db *pgxpool.Pool) *CustomerRepo {
	return &CustomerRepo{db: db}
}

func (r *CustomerRepo) Create(ctx context.Context, c *model.Customer) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO customers (id, merchant_id, external_id, name, email, phone, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())`,
		c.ID, c.MerchantID, c.ExternalID, c.Name, c.Email, c.Phone, c.Metadata,
	)
	return err
}

func (r *CustomerRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Customer, error) {
	c := &model.Customer{}
	err := r.db.QueryRow(ctx,
		`SELECT id, merchant_id, external_id, name, email, phone, metadata, created_at, updated_at
		 FROM customers WHERE id = $1`, id,
	).Scan(&c.ID, &c.MerchantID, &c.ExternalID, &c.Name, &c.Email, &c.Phone, &c.Metadata, &c.CreatedAt, &c.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query customer by id: %w", err)
	}
	return c, nil
}

func (r *CustomerRepo) ListByMerchant(ctx context.Context, merchantID uuid.UUID, page, pageSize int) ([]*model.Customer, int64, error) {
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	var total int64
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM customers WHERE merchant_id = $1`, merchantID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count customers: %w", err)
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, merchant_id, external_id, name, email, phone, metadata, created_at, updated_at
		 FROM customers WHERE merchant_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		merchantID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list customers: %w", err)
	}
	defer rows.Close()

	var customers []*model.Customer
	for rows.Next() {
		c := &model.Customer{}
		if err := rows.Scan(&c.ID, &c.MerchantID, &c.ExternalID, &c.Name, &c.Email, &c.Phone, &c.Metadata, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan customer: %w", err)
		}
		customers = append(customers, c)
	}
	return customers, total, nil
}

func (r *CustomerRepo) Update(ctx context.Context, c *model.Customer) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE customers SET name=$2, email=$3, phone=$4, metadata=$5, updated_at=NOW() WHERE id=$1`,
		c.ID, c.Name, c.Email, c.Phone, c.Metadata,
	)
	if err != nil {
		return fmt.Errorf("update customer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

func (r *CustomerRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM customers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete customer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
