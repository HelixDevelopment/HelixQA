CREATE TYPE payout_status AS ENUM ('pending', 'in_transit', 'paid', 'failed', 'cancelled');
CREATE TYPE payout_method AS ENUM ('standard', 'instant');

CREATE TABLE payouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    provider VARCHAR(50) NOT NULL,
    provider_payout_id VARCHAR(255) NULL,
    amount BIGINT NOT NULL,
    currency CHAR(3) NOT NULL,
    status payout_status NOT NULL,
    method payout_method NOT NULL,
    arrival_date DATE NOT NULL,
    fee_amount BIGINT NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payouts_merchant_id ON payouts (merchant_id);
CREATE INDEX idx_payouts_status ON payouts (status);
CREATE INDEX idx_payouts_arrival_date ON payouts (arrival_date);
CREATE INDEX idx_payouts_created_at ON payouts (created_at);

CREATE TRIGGER set_payouts_updated_at
    BEFORE UPDATE ON payouts
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
