CREATE TYPE invoice_status AS ENUM ('draft', 'open', 'paid', 'void', 'uncollectible');

CREATE TABLE invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE RESTRICT,
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL,
    provider VARCHAR(50) NOT NULL,
    provider_invoice_id VARCHAR(255) NULL,
    amount BIGINT NOT NULL,
    currency CHAR(3) NOT NULL,
    status invoice_status NOT NULL,
    due_date DATE NOT NULL,
    paid_at TIMESTAMPTZ NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_invoices_merchant_id ON invoices (merchant_id);
CREATE INDEX idx_invoices_customer_id ON invoices (customer_id);
CREATE INDEX idx_invoices_status ON invoices (status);
CREATE INDEX idx_invoices_due_date ON invoices (due_date);
CREATE INDEX idx_invoices_paid_at ON invoices (paid_at);

CREATE TRIGGER set_invoices_updated_at
    BEFORE UPDATE ON invoices
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
