CREATE TYPE dispute_status AS ENUM ('warning_needs_response', 'under_review', 'lost', 'won', 'closed');

CREATE TABLE disputes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(id, created_at) ON DELETE RESTRICT,
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    provider VARCHAR(50) NOT NULL,
    provider_dispute_id VARCHAR(255) NOT NULL,
    reason VARCHAR(255) NOT NULL,
    status dispute_status NOT NULL,
    amount BIGINT NOT NULL,
    evidence_deadline TIMESTAMPTZ NULL,
    evidence_submitted_at TIMESTAMPTZ NULL,
    resolution VARCHAR(255) NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_disputes_transaction_id ON disputes (transaction_id);
CREATE INDEX idx_disputes_merchant_id ON disputes (merchant_id);
CREATE INDEX idx_disputes_status ON disputes (status);
CREATE INDEX idx_disputes_evidence_deadline ON disputes (evidence_deadline);

CREATE TRIGGER set_disputes_updated_at
    BEFORE UPDATE ON disputes
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
