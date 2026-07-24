ALTER TABLE merchants
    DROP COLUMN IF EXISTS legal_name,
    DROP COLUMN IF EXISTS trade_name,
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS country,
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS kyc_status;
