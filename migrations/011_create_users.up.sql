CREATE TYPE user_role AS ENUM ('root_admin', 'account_admin', 'user');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    role user_role NOT NULL,
    merchant_id UUID REFERENCES merchants(id) ON DELETE RESTRICT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    mfa_enabled BOOLEAN NOT NULL DEFAULT false,
    mfa_secret VARCHAR(64) NULL,
    mfa_recovery_codes JSONB NULL,
    last_login_at TIMESTAMPTZ NULL,
    last_login_ip INET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_merchant_id ON users (merchant_id);
CREATE INDEX idx_users_role ON users (role);

CREATE TRIGGER set_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
