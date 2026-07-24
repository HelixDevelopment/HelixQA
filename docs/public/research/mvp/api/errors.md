# Error Handling Reference

All error responses use a standard JSON envelope and appropriate HTTP status codes.

## Standard Error Response Format

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

| Field | Type | Description |
|-------|------|-------------|
| `error.code` | string | Machine-readable error code |
| `error.message` | string | Human-readable description |
| `error.details` | object | Optional additional context (varies by error) |

## HTTP Status Codes

| Code | Meaning | When Used |
|------|---------|-----------|
| `200` | OK | Successful retrieval or update |
| `201` | Created | Resource successfully created |
| `204` | No Content | Successful deletion or action with no response body |
| `400` | Bad Request | Malformed JSON, missing required fields, invalid parameters |
| `401` | Unauthorized | Missing, expired, or invalid authentication credentials |
| `403` | Forbidden | Authenticated but insufficient permissions for this resource |
| `404` | Not Found | Resource does not exist or is not accessible to the caller |
| `409` | Conflict | Idempotency key reuse with different payload, or resource state conflict |
| `422` | Unprocessable Entity | Request is well-formed but semantically invalid (e.g., amount exceeds balance) |
| `429` | Too Many Requests | Rate limit exceeded |
| `500` | Internal Server Error | Unexpected server failure |
| `502` | Bad Gateway | Upstream provider returned an error |
| `503` | Service Unavailable | Temporary outage or maintenance |

## Error Codes

### Authentication & Authorization

| Code | HTTP | Description |
|------|------|-------------|
| `unauthorized` | 401 | Missing or malformed `Authorization` header |
| `invalid_credentials` | 401 | Email or password is incorrect |
| `token_expired` | 401 | Access token has expired; use `/auth/refresh` |
| `invalid_refresh_token` | 401 | Refresh token is invalid, expired, or already used |
| `mfa_required` | 401 | MFA setup/verification required before access is granted |
| `mfa_invalid_code` | 401 | TOTP code is incorrect |
| `forbidden` | 403 | User lacks required role or scope for this operation |
| `account_disabled` | 403 | User account has been deactivated |
| `account_suspended` | 403 | Merchant account is suspended |

### Validation

| Code | HTTP | Description |
|------|------|-------------|
| `validation_error` | 400 | Request body failed validation; `details` contains field-level errors |
| `missing_required_field` | 400 | A required field is missing from the request body |
| `invalid_format` | 400 | Field value does not match expected format (e.g., invalid UUID, bad email) |
| `invalid_enum_value` | 400 | Field value is not one of the allowed enum values |
| `amount_too_small` | 400 | Amount is below the minimum (e.g., less than 1 for currency) |
| `amount_too_large` | 400 | Amount exceeds maximum allowed value |

### Resource

| Code | HTTP | Description |
|------|------|-------------|
| `not_found` | 404 | Requested resource does not exist |
| `merchant_not_found` | 404 | Merchant ID is invalid or does not exist |
| `customer_not_found` | 404 | Customer ID is invalid or does not exist |
| `transaction_not_found` | 404 | Transaction ID is invalid or does not exist |
| `subscription_not_found` | 404 | Subscription ID is invalid or does not exist |
| `invoice_not_found` | 404 | Invoice ID is invalid or does not exist |
| `payout_not_found` | 404 | Payout ID is invalid or does not exist |
| `dispute_not_found` | 404 | Dispute ID is invalid or does not exist |
| `payment_method_not_found` | 404 | Payment method ID is invalid or does not exist |
| `webhook_not_found` | 404 | Webhook config ID is invalid or does not exist |
| `provider_not_found` | 404 | Provider config ID is invalid or does not exist |
| `api_key_not_found` | 404 | API key ID is invalid or does not exist |
| `user_not_found` | 404 | User ID is invalid or does not exist |

### Conflict & Idempotency

| Code | HTTP | Description |
|------|------|-------------|
| `idempotency_conflict` | 409 | Idempotency key reused with a different request payload |
| `email_already_exists` | 409 | Registration email is already in use |
| `duplicate_provider_config` | 409 | Provider is already configured for this merchant |
| `subscription_already_cancelled` | 409 | Attempted to cancel an already-cancelled subscription |
| `transaction_already_refunded` | 409 | Transaction has already been fully refunded |
| `dispute_already_submitted` | 409 | Evidence has already been submitted for this dispute |

### Business Logic

| Code | HTTP | Description |
|------|------|-------------|
| `insufficient_funds` | 422 | Transaction amount exceeds available balance |
| `refund_exceeds_original` | 422 | Refund amount exceeds the original transaction amount |
| `provider_not_configured` | 422 | Requested payment provider is not configured for this merchant |
| `provider_inactive` | 422 | Requested payment provider is inactive |
| `payment_method_required` | 422 | No payment method provided for a charge that requires one |
| `customer_required` | 422 | Customer ID is required for this operation |
| `subscription_plan_required` | 422 | Plan ID is required when creating a subscription |
| `invoice_already_paid` | 422 | Invoice has already been paid |

### Provider Errors

| Code | HTTP | Description |
|------|------|-------------|
| `provider_error` | 502 | Generic upstream provider error |
| `provider_timeout` | 502 | Provider did not respond in time |
| `provider_declined` | 422 | Provider declined the transaction |
| `provider_card_declined` | 422 | Card was declined by the issuer |
| `provider_insufficient_funds` | 422 | Provider reports insufficient funds |
| `provider_expired_card` | 422 | Card has expired |
| `provider_invalid_card` | 422 | Card number is invalid |
| `provider_rate_limited` | 429 | Provider rate limit hit; retry later |
| `provider_authentication_required` | 422 | 3D Secure or similar authentication required |

### Rate Limiting

| Code | HTTP | Description |
|------|------|-------------|
| `rate_limit_exceeded` | 429 | Too many requests; check `Retry-After` header |

### Server

| Code | HTTP | Description |
|------|------|-------------|
| `internal_error` | 500 | Unexpected server error |
| `service_unavailable` | 503 | Service is temporarily unavailable (maintenance or overload) |

## Validation Error Format

When `code` is `validation_error`, the `details` field contains structured information:

```json
{
  "error": {
    "code": "validation_error",
    "message": "The request body is invalid",
    "details": {
      "errors": [
        {
          "field": "email",
          "reason": "must be a valid email address",
          "received": "not-an-email"
        },
        {
          "field": "amount",
          "reason": "must be a positive integer",
          "received": -100
        }
      ]
    }
  }
}
```

## Provider Error Mapping

The platform normalizes provider-specific errors into the standard error codes above. This mapping applies:

| Provider | Provider Error | Helix Error Code |
|----------|---------------|------------------|
| Stripe | `card_declined` | `provider_card_declined` |
| Stripe | `insufficient_funds` | `provider_insufficient_funds` |
| Stripe | `expired_card` | `provider_expired_card` |
| Stripe | `incorrect_cvc` | `provider_card_declined` |
| Stripe | `processing_error` | `provider_error` |
| Stripe | `rate_limit` | `provider_rate_limited` |
| PayPal | `PAYMENT_NOT_AUTHORIZED` | `provider_declined` |
| PayPal | `TRANSACTION_REFUSED` | `provider_declined` |
| PayPal | `INTERNAL_ERROR` | `provider_error` |
| Square | `CARD_DECLINED` | `provider_card_declined` |
| Square | `VERIFY_CVV_FAILURE` | `provider_card_declined` |
| Square | `CARD_EXPIRED` | `provider_expired_card` |
| Square | `INSUFFICIENT_FUNDS` | `provider_insufficient_funds` |

## Examples

### 400 — Bad Request

```bash
curl -X POST https://seller.hxd3v.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "bad"}'
```

```json
{
  "error": {
    "code": "validation_error",
    "message": "The request body is invalid",
    "details": {
      "errors": [
        {
          "field": "email",
          "reason": "must be a valid email address",
          "received": "bad"
        },
        {
          "field": "password",
          "reason": "is required",
          "received": null
        }
      ]
    }
  }
}
```

### 401 — Unauthorized

```bash
curl https://seller.hxd3v.com/api/v1/merchants/550e8400-e29b-41d4-a716-446655440000/transactions
```

```json
{
  "error": {
    "code": "unauthorized",
    "message": "Missing or invalid authorization header"
  }
}
```

### 403 — Forbidden

```bash
curl -X DELETE https://seller.hxd3v.com/api/v1/users/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer $USER_TOKEN"
```

```json
{
  "error": {
    "code": "forbidden",
    "message": "Insufficient permissions for this operation"
  }
}
```

### 404 — Not Found

```bash
curl https://seller.hxd3v.com/api/v1/merchants/00000000-0000-0000-0000-000000000000 \
  -H "Authorization: Bearer $TOKEN"
```

```json
{
  "error": {
    "code": "merchant_not_found",
    "message": "Merchant not found"
  }
}
```

### 409 — Conflict

```bash
curl -X POST https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/transactions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Idempotency-Key: duplicate-key-123" \
  -H "Content-Type: application/json" \
  -d '{"amount": 1999, "currency": "USD", "provider": "stripe"}'
```

```json
{
  "error": {
    "code": "idempotency_conflict",
    "message": "Idempotency key reused with a different request payload"
  }
}
```

### 429 — Rate Limited

```bash
curl https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/transactions \
  -H "Authorization: Bearer $TOKEN"
```

```json
{
  "error": {
    "code": "rate_limit_exceeded",
    "message": "Rate limit exceeded. Retry after 2 seconds."
  }
}
```

The response includes a `Retry-After` header with the number of seconds to wait.

### 502 — Provider Error

```bash
curl -X POST https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/transactions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"amount": 1999, "currency": "USD", "provider": "stripe"}'
```

```json
{
  "error": {
    "code": "provider_error",
    "message": "Stripe returned an error: Your account is not activated for live payments.",
    "details": {
      "provider": "stripe",
      "provider_error_code": "account_not_active"
    }
  }
}
```
