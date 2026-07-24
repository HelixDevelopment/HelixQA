CREATE TYPE transaction_type AS ENUM ('charge', 'refund', 'payout');
CREATE TYPE transaction_status AS ENUM ('pending', 'processing', 'succeeded', 'failed', 'cancelled', 'reversed');

CREATE TABLE transactions (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id UUID REFERENCES customers(id) ON DELETE SET NULL,
    provider VARCHAR(50) NOT NULL,
    provider_transaction_id VARCHAR(255) NOT NULL,
    type transaction_type NOT NULL,
    amount BIGINT NOT NULL,
    currency CHAR(3) NOT NULL,
    status transaction_status NOT NULL,
    payment_method_id UUID REFERENCES payment_methods(id) ON DELETE SET NULL,
    idempotency_key VARCHAR(255) UNIQUE,
    description TEXT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    error_code VARCHAR(100) NULL,
    error_message TEXT NULL,
    fee_amount BIGINT NOT NULL DEFAULT 0,
    net_amount BIGINT NULL,
    processed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE UNIQUE INDEX idx_transactions_provider_transaction ON transactions (provider, provider_transaction_id, created_at);
CREATE INDEX idx_transactions_merchant_id ON transactions (merchant_id, created_at);
CREATE INDEX idx_transactions_customer_id ON transactions (customer_id, created_at);
CREATE INDEX idx_transactions_status ON transactions (status, created_at);
CREATE INDEX idx_transactions_type ON transactions (type, created_at);
CREATE INDEX idx_transactions_created_at ON transactions (created_at);
CREATE INDEX idx_transactions_processed_at ON transactions (processed_at);

CREATE TRIGGER set_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
