# Authentication Guide

Helix Seller supports two authentication mechanisms: **JWT Bearer tokens** (for user sessions) and **API keys** (for server-to-server integrations).

## JWT Bearer Token Flow

### Step 1: Register

Create a merchant account and admin user in one call.

```bash
curl -X POST https://seller.hxd3v.com/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@acme.com",
    "password": "securepass123",
    "name": "Jane Doe",
    "company_name": "Acme Corp"
  }'
```

**Response** (`201 Created`):

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "refresh_token": "dGhpcyBpcyBhIHJlZnJl...",
  "token_type": "Bearer",
  "expires_in": 900
}
```

The registering user receives the `root_admin` role for the new merchant.

### Step 2: Login

```bash
curl -X POST https://seller.hxd3v.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@acme.com",
    "password": "securepass123"
  }'
```

**Response** (`200 OK`): Same `TokenPair` shape as registration.

### Step 3: Use the Token

Include the access token in the `Authorization` header for all API calls:

```bash
curl https://seller.hxd3v.com/api/v1/merchants/{merchantId}/transactions \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..."
```

If MFA is enabled for the user, the login response will indicate MFA is required. Complete MFA verification before obtaining tokens — see [MFA section](#mfa-setup-and-verification) below.

## Refresh Token Flow

Access tokens expire after **15 minutes**. Use the refresh token to obtain a new pair without re-authenticating:

```bash
curl -X POST https://seller.hxd3v.com/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "dGhpcyBpcyBhIHJlZnJl..."}'
```

**Response** (`200 OK`):

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...(new)",
  "refresh_token": "(new refresh token)",
  "token_type": "Bearer",
  "expires_in": 900
}
```

Refresh tokens are **single-use** — each refresh returns a new refresh token. The previous one is invalidated.

### Logout

Revokes the current refresh token:

```bash
curl -X POST https://seller.hxd3v.com/api/v1/auth/logout \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

**Response**: `204 No Content`

## Session Management

| Token | Lifetime | Description |
|-------|----------|-------------|
| Access token | 15 minutes | Used for API authentication |
| Refresh token | 7 days | Used to obtain new access tokens |
| Idle timeout | 30 minutes | Session expires after 30 min of inactivity |

- Refresh tokens are bound to the user and invalidated on password change.
- Maximum 5 active refresh tokens per user (oldest revoked on overflow).

## API Key Management

API keys are intended for server-to-server integrations where a user session is not practical.

### Create an API Key

```bash
curl -X POST https://seller.hxd3v.com/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production Backend",
    "scopes": ["transactions:read", "transactions:write", "customers:read"],
    "rate_limit": 50,
    "expires_at": "2026-12-31T23:59:59Z"
  }'
```

**Response** (`201 Created`):

```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "name": "Production Backend",
  "key": "hxs_live_abc123def456ghi789jkl012mno345pqr678stu901",
  "key_prefix": "hxs_live_",
  "created_at": "2026-07-23T10:00:00Z"
}
```

> **Important**: The full API key is shown **only once** at creation. Store it securely — it cannot be retrieved later.

### Use an API Key

```bash
curl https://seller.hxd3v.com/api/v1/merchants/{merchantId}/transactions \
  -H "X-API-Key: hxs_live_abc123def456ghi789jkl012mno345pqr678stu901"
```

### List API Keys

```bash
curl https://seller.hxd3v.com/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN"
```

**Response** (`200 OK`):

```json
{
  "data": [
    {
      "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "name": "Production Backend",
      "key_prefix": "hxs_live_",
      "scopes": ["transactions:read", "transactions:write", "customers:read"],
      "rate_limit": 50,
      "is_active": true,
      "last_used_at": "2026-07-23T12:34:56Z",
      "expires_at": "2026-12-31T23:59:59Z",
      "created_at": "2026-07-23T10:00:00Z"
    }
  ],
  "page": 1,
  "page_size": 20,
  "total": 1,
  "total_pages": 1
}
```

### Revoke an API Key

```bash
curl -X DELETE https://seller.hxd3v.com/api/v1/api-keys/a1b2c3d4-e5f6-7890-abcd-ef1234567890 \
  -H "Authorization: Bearer $TOKEN"
```

**Response**: `204 No Content`

### Rotate an API Key

To rotate: create a new key, update your integrations, then revoke the old one.

## API Key Scopes

Scopes limit what an API key can access. Format: `<resource>:<permission>`.

| Scope | Access |
|-------|--------|
| `transactions:read` | List and view transactions |
| `transactions:write` | Create charges, refunds |
| `customers:read` | List and view customers |
| `customers:write` | Create and update customers |
| `subscriptions:read` | List and view subscriptions |
| `subscriptions:write` | Create, update, cancel subscriptions |
| `invoices:read` | List and view invoices |
| `invoices:write` | Create invoices |
| `payouts:read` | List and view payouts |
| `disputes:read` | List and view disputes |
| `disputes:write` | Create disputes, submit evidence |
| `payment_methods:read` | List and view payment methods |
| `payment_methods:write` | Create and delete payment methods |
| `analytics:read` | Access analytics endpoints |
| `billing:read` | View fees and billing invoices |
| `webhooks:write` | Manage webhook configurations |
| `providers:write` | Configure payment providers |

If no scopes are provided, the key has **full access** (all scopes).

## MFA Setup and Verification

### Enable MFA

```bash
curl -X POST https://seller.hxd3v.com/api/v1/auth/mfa/setup \
  -H "Authorization: Bearer $TOKEN"
```

**Response** (`200 OK`):

```json
{
  "secret": "JBSWY3DPEHPK3PXP",
  "qr_code_url": "otpauth://totp/HelixSeller:admin@acme.com?secret=JBSWY3DPEHPK3PXP&issuer=HelixSeller",
  "recovery_codes": [
    "ABC1-2345",
    "DEF6-7890",
    "GHI1-2345"
  ]
}
```

Scan the QR code with an authenticator app (Google Authenticator, Authy, etc.), then verify.

### Verify MFA

```bash
curl -X POST https://seller.hxd3v.com/api/v1/auth/mfa/verify \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"code": "123456"}'
```

**Response**: `200 OK` on success, `401` if code is invalid.

After MFA is enabled, login requires an additional step (the MFA code must be verified before tokens are issued).

## RBAC Permissions

| Role | Description | Permissions |
|------|-------------|-------------|
| `root_admin` | Merchant owner | Full access to all resources, user management, billing, provider config |
| `account_admin` | Administrative user | Full access to transactions, customers, subscriptions; limited user/billing access |
| `user` | Standard user | Read access to transactions, customers, subscriptions; write access to charges and refunds |

Role-based access is enforced at the API level. Attempting to access a resource beyond your role returns `403 Forbidden`.

### Update User Role

```bash
curl -X PUT https://seller.hxd3v.com/api/v1/users/{userId} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "role": "account_admin",
    "name": "John Smith"
  }'
```

## Error Responses

Authentication errors return standard error responses:

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `unauthorized` | 401 | Missing or invalid credentials |
| `token_expired` | 401 | Access token has expired — use refresh token |
| `invalid_refresh_token` | 401 | Refresh token is invalid or revoked |
| `mfa_required` | 401 | MFA verification needed |
| `mfa_invalid_code` | 401 | TOTP code is incorrect |
| `forbidden` | 403 | Insufficient permissions for this resource |
| `account_disabled` | 403 | User account has been deactivated |
