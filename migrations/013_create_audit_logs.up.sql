CREATE TYPE audit_actor_type AS ENUM ('root_admin', 'account_admin', 'user', 'system', 'api_key');

CREATE TABLE audit_logs (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    actor_id UUID NOT NULL,
    actor_type audit_actor_type NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id UUID NULL,
    changes JSONB NULL,
    ip_address INET NULL,
    user_agent TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE INDEX idx_audit_logs_merchant_id ON audit_logs (merchant_id, created_at);
CREATE INDEX idx_audit_logs_actor_id ON audit_logs (actor_id, created_at);
CREATE INDEX idx_audit_logs_action ON audit_logs (action, created_at);
CREATE INDEX idx_audit_logs_resource ON audit_logs (resource_type, resource_id, created_at);
CREATE INDEX idx_audit_logs_created_at ON audit_logs (created_at);
