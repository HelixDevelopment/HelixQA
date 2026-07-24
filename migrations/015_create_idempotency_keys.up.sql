CREATE TABLE idempotency_keys (
    id SERIAL PRIMARY KEY,
    key_hash VARCHAR(64) NOT NULL,
    response JSONB NOT NULL,
    status_code SMALLINT NOT NULL,
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    UNIQUE (key_hash)
);

CREATE INDEX idx_idempotency_keys_expires_at ON idempotency_keys (expires_at);
