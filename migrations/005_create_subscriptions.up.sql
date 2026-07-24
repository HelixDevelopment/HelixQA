CREATE TYPE subscription_status AS ENUM ('active', 'past_due', 'cancelled', 'unpaid', 'trialing');
CREATE TYPE subscription_interval AS ENUM ('day', 'week', 'month', 'year');

CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    provider VARCHAR(50) NOT NULL,
    provider_subscription_id VARCHAR(255) NOT NULL,
    plan_id VARCHAR(255) NOT NULL,
    status subscription_status NOT NULL,
    amount BIGINT NOT NULL,
    currency CHAR(3) NOT NULL,
    interval subscription_interval NOT NULL,
    interval_count SMALLINT NOT NULL DEFAULT 1,
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end TIMESTAMPTZ NOT NULL,
    cancel_at TIMESTAMPTZ NULL,
    cancelled_at TIMESTAMPTZ NULL,
    trial_start TIMESTAMPTZ NULL,
    trial_end TIMESTAMPTZ NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_subscriptions_provider_subscription ON subscriptions (provider, provider_subscription_id);
CREATE INDEX idx_subscriptions_merchant_id ON subscriptions (merchant_id);
CREATE INDEX idx_subscriptions_customer_id ON subscriptions (customer_id);
CREATE INDEX idx_subscriptions_status ON subscriptions (status);
CREATE INDEX idx_subscriptions_current_period_end ON subscriptions (current_period_end);

CREATE TRIGGER set_subscriptions_updated_at
    BEFORE UPDATE ON subscriptions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
