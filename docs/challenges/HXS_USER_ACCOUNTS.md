# HXS Default Test User Accounts

These credentials are used by the HXS challenge suite for testing.

| Role | Email | Password | Name |
|------|-------|----------|------|
| Admin | admin@helix.test | admin123! | Admin User |
| Merchant | merchant@helix.test | merchant123! | Test Merchant |
| Customer | customer@helix.test | customer123! | Test Customer |

## How Accounts Are Created

1. `hxs_setup.sh` calls `POST /api/v1/auth/register` for each user
2. If user already exists, it attempts login to verify credentials
3. Credentials are stored in `tests/challenges/config/credentials.env`

## Security Notes

- These are TEST credentials for development/testing only
- Change passwords before deploying to production
- Credentials file is version-controlled intentionally for CI/CD
