CREATE TYPE payment_method_type AS ENUM ('card', 'bank_account', 'wallet');

CREATE TABLE payment_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id UUID REFERENCES customers(id) ON DELETE SET NULL,
    type payment_method_type NOT NULL,
    provider VARCHAR(50) NOT NULL,
    provider_token VARCHAR(500) NOT NULL,
    fingerprint VARCHAR(255) NULL,
    brand VARCHAR(50) NULL,
    last4 CHAR(4) NULL,
    exp_month SMALLINT NULL,
    exp_year SMALLINT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payment_methods_merchant_id ON payment_methods (merchant_id);
CREATE INDEX idx_payment_methods_customer_id ON payment_methods (customer_id);
CREATE INDEX idx_payment_methods_fingerprint ON payment_methods (fingerprint);
CREATE INDEX idx_payment_methods_provider_token ON payment_methods (provider_token);

CREATE TRIGGER set_payment_methods_updated_at
    BEFORE UPDATE ON payment_methods
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
