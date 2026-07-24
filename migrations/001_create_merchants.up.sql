CREATE TYPE merchant_status AS ENUM ('active', 'suspended', 'pending_verification', 'pending');

CREATE TABLE merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    legal_name VARCHAR(255) NOT NULL DEFAULT '',
    trade_name VARCHAR(255) NOT NULL DEFAULT '',
    phone VARCHAR(50) NOT NULL DEFAULT '',
    country VARCHAR(2) NOT NULL DEFAULT 'US',
    kyc_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    email VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    status merchant_status NOT NULL DEFAULT 'pending_verification',
    default_currency CHAR(3) NOT NULL DEFAULT 'USD',
    timezone VARCHAR(50) NOT NULL DEFAULT 'UTC',
    branding JSONB NOT NULL DEFAULT '{}',
    settings JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX idx_merchants_slug ON merchants (slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_merchants_email ON merchants (email) WHERE deleted_at IS NULL;
CREATE INDEX idx_merchants_status ON merchants (status) WHERE deleted_at IS NULL;

CREATE TRIGGER set_merchants_updated_at
    BEFORE UPDATE ON merchants
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
