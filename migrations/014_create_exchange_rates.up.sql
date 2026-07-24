CREATE TABLE exchange_rates (
    id SERIAL PRIMARY KEY,
    base_currency CHAR(3) NOT NULL,
    quote_currency CHAR(3) NOT NULL,
    rate NUMERIC(18, 8) NOT NULL,
    source VARCHAR(50) NOT NULL,
    fetched_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    UNIQUE (base_currency, quote_currency)
);

CREATE INDEX idx_exchange_rates_expires_at ON exchange_rates (expires_at);
