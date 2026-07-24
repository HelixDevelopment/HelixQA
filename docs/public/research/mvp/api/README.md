# Helix Seller API Reference

Unified payment facade providing a single REST API across Stripe, PayPal, and Square.

## Base URL

| Environment | Base URL |
|-------------|----------|
| Production  | `https://seller.hxd3v.com/api/v1` |
| Staging     | `https://sta.seller.hxd3v.com/api/v1` |
| Development | `https://dev.seller.hxd3v.com/api/v1` |

All endpoints are versioned under `/v1`. The OpenAPI 3.1 spec lives at `specs/001-helix-seller-platform/contracts/api-v1.yaml`.

## Authentication

The API supports two authentication methods (you may use either):

| Method | Header | Description |
|--------|--------|-------------|
| JWT Bearer Token | `Authorization: Bearer <token>` | Short-lived access token (15 min). Obtained via `/auth/login` or `/auth/register`. |
| API Key | `X-API-Key: <key>` | Long-lived key for server-to-server integration. Scoped and rate-limited per key. |

Most endpoints require authentication. Exceptions: `/auth/register`, `/auth/login`, `/auth/refresh`, `/webhooks/stripe`, `/webhooks/paypal`, `/webhooks/square`, `/health`.

See [authentication.md](authentication.md) for full details.

## Rate Limiting

Rate limits are applied **per merchant** and can be configured per API key:

- Default: 100 requests per second per merchant.
- API keys can override with a custom `rate_limit` (requests per second, `0` = unlimited).
- Exceeded limits return `429 Too Many Requests` with a standard error response.

## Request Format

- **Content-Type**: `application/json`
- **Body**: All request bodies must be valid JSON.
- **UUIDs**: Resource IDs are UUIDs (e.g., `550e8400-e29b-41d4-a716-446655440000`).
- **Amounts**: All monetary amounts are integers in the **smallest currency unit** (e.g., cents for USD: `1999` = $19.99).
- **Currencies**: ISO 4217 codes (e.g., `USD`, `EUR`).

## Response Format

All responses return JSON. Successful responses:

- `200 OK` — Resource retrieved or updated.
- `201 Created` — Resource created.
- `204 No Content` — Deletion or action with no body.

## Error Handling

Errors follow a standard envelope:

```json
{
  "error": {
    "code": "validation_error",
    "message": "The request body is invalid",
    "details": {
      "field": "email",
      "reason": "must be a valid email address"
    }
  }
}
```

See [errors.md](errors.md) for the full error catalog.

## Pagination

List endpoints return paginated results. Control with query parameters:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | integer | `1` | Page number (>= 1) |
| `page_size` | integer | `20` | Items per page (1-100) |

Response envelope:

```json
{
  "data": [...],
  "page": 1,
  "page_size": 20,
  "total": 150,
  "total_pages": 8
}
```

## Idempotency

For safe retries on `POST` requests (charges, refunds, subscriptions, etc.):

- Include the `Idempotency-Key` header with a unique value per request.
- The key is scoped to the endpoint and merchant.
- Replaying the same key within the window returns the original response without re-executing.
- Recommended: UUID v4 per logical operation.

```bash
curl -X POST https://seller.hxd3v.com/api/v1/merchants/{merchantId}/transactions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000" \
  -d '{"amount": 1999, "currency": "USD", "provider": "stripe"}'
```

## CORS Policy

- **Allowed origins**: Configurable per merchant (defaults to `*` in development).
- **Allowed methods**: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`.
- **Allowed headers**: `Authorization`, `Content-Type`, `X-API-Key`, `Idempotency-Key`.
- **Credentials**: Supported when origins are explicit (not `*`).

## Resource Hierarchy

```
Merchant
  ├── Customer
  │     └── PaymentMethod
  ├── Transaction (charge / refund / payout)
  ├── Subscription
  ├── Invoice
  ├── Payout
  ├── Dispute
  ├── WebhookConfig
  ├── ProviderConfig
  ├── ExchangeRate
  ├── Analytics
  └── Billing
```

All merchant-scoped endpoints are prefixed with `/merchants/{merchantId}`.

## Quick Start

```bash
# 1. Register
curl -X POST https://seller.hxd3v.com/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "merchant@example.com",
    "password": "securepass123",
    "name": "Jane Doe",
    "company_name": "Acme Corp"
  }'

# 2. Login (returns JWT)
curl -X POST https://seller.hxd3v.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "merchant@example.com", "password": "securepass123"}'

# 3. Create a charge
TOKEN="<access_token_from_login>"
MERCHANT_ID="<merchant_id>"

curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/transactions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 2500,
    "currency": "USD",
    "provider": "stripe",
    "description": "Order #1234"
  }'
```

## Documentation Index

| Document | Description |
|----------|-------------|
| [README.md](README.md) | This file — API overview and quick start |
| [authentication.md](authentication.md) | JWT flow, API keys, MFA, RBAC, session management |
| [endpoints.md](endpoints.md) | Complete endpoint reference with examples |
| [errors.md](errors.md) | Error codes, status codes, validation format |
| [webhooks.md](webhooks.md) | Incoming/outgoing webhooks, signature verification, retries |
