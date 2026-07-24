CREATE TABLE customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    external_id VARCHAR(255) NULL,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NULL,
    phone VARCHAR(50) NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX idx_customers_merchant_id ON customers (merchant_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_customers_merchant_email ON customers (merchant_id, email) WHERE deleted_at IS NULL AND email IS NOT NULL;
CREATE UNIQUE INDEX idx_customers_merchant_external_id ON customers (merchant_id, external_id) WHERE deleted_at IS NULL AND external_id IS NOT NULL;

CREATE TRIGGER set_customers_updated_at
    BEFORE UPDATE ON customers
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
