CREATE TYPE provider_health_status AS ENUM ('healthy', 'degraded', 'unhealthy');

CREATE TABLE provider_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    provider VARCHAR(50) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    config JSONB NOT NULL,
    fallback_order SMALLINT NOT NULL DEFAULT 0,
    health_status provider_health_status NOT NULL DEFAULT 'healthy',
    last_health_check TIMESTAMPTZ NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (merchant_id, provider)
);

CREATE INDEX idx_provider_configs_fallback_order ON provider_configs (fallback_order);

CREATE TRIGGER set_provider_configs_updated_at
    BEFORE UPDATE ON provider_configs
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
