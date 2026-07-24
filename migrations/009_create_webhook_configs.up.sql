CREATE TABLE webhook_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    url VARCHAR(2048) NOT NULL,
    secret VARCHAR(255) NOT NULL,
    events TEXT[] NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webhook_configs_merchant_id ON webhook_configs (merchant_id);
CREATE INDEX idx_webhook_configs_is_active ON webhook_configs (is_active);

CREATE TRIGGER set_webhook_configs_updated_at
    BEFORE UPDATE ON webhook_configs
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
