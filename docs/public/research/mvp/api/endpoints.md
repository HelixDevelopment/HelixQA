# Endpoint Reference

Base URL: `https://seller.hxd3v.com/api/v1`

All merchant-scoped endpoints require authentication (JWT Bearer token or API key) and are prefixed with `/merchants/{merchantId}` where `{merchantId}` is a UUID.

---

## Authentication

### POST /auth/register

Register a new merchant account and admin user.

**Auth**: None

**Request Body**:

```json
{
  "email": "admin@acme.com",
  "password": "securepass123",
  "name": "Jane Doe",
  "company_name": "Acme Corp"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `email` | string | yes | Admin email |
| `password` | string | yes | Account password |
| `name` | string | yes | Admin full name |
| `company_name` | string | no | Merchant company name |

**Response** (`201`):

```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "refresh_token": "dGhpcyBpcyBhIHJlZnJl...",
  "token_type": "Bearer",
  "expires_in": 900
}
```

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

---

### POST /auth/login

Authenticate and obtain JWT tokens.

**Auth**: None

**Request Body**:

```json
{
  "email": "admin@acme.com",
  "password": "securepass123"
}
```

**Response** (`200`): TokenPair

```bash
curl -X POST https://seller.hxd3v.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@acme.com", "password": "securepass123"}'
```

---

### POST /auth/refresh

Refresh an expired access token.

**Auth**: None

**Request Body**:

```json
{
  "refresh_token": "dGhpcyBpcyBhIHJlZnJl..."
}
```

**Response** (`200`): New TokenPair

```bash
curl -X POST https://seller.hxd3v.com/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "dGhpcyBpcyBhIHJlZnJl..."}'
```

---

### POST /auth/logout

Revoke the current refresh token.

**Auth**: Bearer token

**Response**: `204 No Content`

```bash
curl -X POST https://seller.hxd3v.com/api/v1/auth/logout \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /auth/mfa/setup

Generate TOTP secret, QR code URL, and recovery codes.

**Auth**: Bearer token

**Response** (`200`):

```json
{
  "secret": "JBSWY3DPEHPK3PXP",
  "qr_code_url": "otpauth://totp/HelixSeller:admin@acme.com?secret=JBSWY3DPEHPK3PXP&issuer=HelixSeller",
  "recovery_codes": ["ABC1-2345", "DEF6-7890", "GHI1-2345"]
}
```

```bash
curl -X POST https://seller.hxd3v.com/api/v1/auth/mfa/setup \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /auth/mfa/verify

Verify a TOTP code to complete MFA setup.

**Auth**: Bearer token

**Request Body**:

```json
{
  "code": "123456"
}
```

**Response**: `200 OK`

```bash
curl -X POST https://seller.hxd3v.com/api/v1/auth/mfa/verify \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"code": "123456"}'
```

---

## Merchants

### GET /merchants

List all merchants (admin-level access).

**Auth**: Bearer token

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | integer | 1 | Page number |
| `page_size` | integer | 20 | Items per page (1-100) |

**Response** (`200`):

```json
{
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Acme Corp",
      "email": "admin@acme.com",
      "slug": "acme-corp",
      "status": "active",
      "default_currency": "USD",
      "timezone": "America/New_York",
      "branding": {
        "primary_color": "#1a73e8",
        "logo_url": "https://example.com/logo.png"
      },
      "settings": {},
      "created_at": "2026-07-23T10:00:00Z",
      "updated_at": "2026-07-23T10:00:00Z"
    }
  ],
  "page": 1,
  "page_size": 20,
  "total": 1,
  "total_pages": 1
}
```

```bash
curl https://seller.hxd3v.com/api/v1/merchants \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /merchants

Create a new merchant.

**Auth**: Bearer token

**Request Body**:

```json
{
  "name": "Acme Corp",
  "email": "admin@acme.com",
  "default_currency": "USD",
  "timezone": "America/New_York"
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | yes | — | Merchant name |
| `email` | string | yes | — | Contact email |
| `default_currency` | string | no | `USD` | ISO 4217 currency code |
| `timezone` | string | no | `UTC` | IANA timezone |

**Response** (`201`): Merchant object

```bash
curl -X POST https://seller.hxd3v.com/api/v1/merchants \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Acme Corp",
    "email": "admin@acme.com",
    "default_currency": "USD"
  }'
```

---

### GET /merchants/{merchantId}

Get merchant by ID.

**Auth**: Bearer token

**Path Parameters**:

| Param | Type | Description |
|-------|------|-------------|
| `merchantId` | UUID | Merchant ID |

**Response** (`200`): Merchant object

```bash
curl https://seller.hxd3v.com/api/v1/merchants/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer $TOKEN"
```

---

### PUT /merchants/{merchantId}

Update merchant details.

**Auth**: Bearer token

**Request Body**:

```json
{
  "name": "Acme Corp (Updated)",
  "email": "newemail@acme.com",
  "default_currency": "EUR",
  "timezone": "Europe/London",
  "branding": {
    "primary_color": "#ff5722"
  }
}
```

**Response** (`200`): Updated Merchant object

```bash
curl -X PUT https://seller.hxd3v.com/api/v1/merchants/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Acme Corp (Updated)"}'
```

---

## Customers

### GET /merchants/{merchantId}/customers

List customers for a merchant.

**Auth**: Bearer token or API key (`customers:read`)

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | integer | 1 | Page number |
| `page_size` | integer | 20 | Items per page (1-100) |

**Response** (`200`): Paginated list of Customer objects

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/customers?page=1&page_size=10" \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /merchants/{merchantId}/customers

Create a customer.

**Auth**: Bearer token or API key (`customers:write`)

**Request Body**:

```json
{
  "name": "John Smith",
  "email": "john@example.com",
  "phone": "+1-555-0123",
  "external_id": "cust_12345",
  "metadata": {
    "plan": "enterprise",
    "region": "us-east"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Customer name |
| `email` | string | no | Customer email |
| `phone` | string | no | Phone number |
| `external_id` | string | no | Your internal customer ID |
| `metadata` | object | no | Custom key-value pairs |

**Response** (`201`): Customer object

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/customers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "John Smith",
    "email": "john@example.com"
  }'
```

---

### GET /merchants/{merchantId}/customers/{customerId}

Get customer by ID.

**Auth**: Bearer token or API key (`customers:read`)

**Response** (`200`): Customer object

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/customers/$CUSTOMER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

### PUT /merchants/{merchantId}/customers/{customerId}

Update customer details.

**Auth**: Bearer token or API key (`customers:write`)

**Request Body**:

```json
{
  "name": "John Smith Jr.",
  "email": "john.jr@example.com",
  "metadata": {
    "plan": "enterprise",
    "upgraded": "2026-07-23"
  }
}
```

**Response** (`200`): Updated Customer object

```bash
curl -X PUT "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/customers/$CUSTOMER_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "John Smith Jr."}'
```

---

## Transactions

### GET /merchants/{merchantId}/transactions

List transactions with optional filters.

**Auth**: Bearer token or API key (`transactions:read`)

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | integer | 1 | Page number |
| `page_size` | integer | 20 | Items per page |
| `type` | string | — | Filter: `charge`, `refund`, `payout` |
| `status` | string | — | Filter: `pending`, `processing`, `succeeded`, `failed`, `cancelled`, `reversed` |
| `provider` | string | — | Filter: `stripe`, `paypal`, `square` |
| `customer_id` | UUID | — | Filter by customer |
| `from` | datetime | — | Start of date range (ISO 8601) |
| `to` | datetime | — | End of date range (ISO 8601) |

**Response** (`200`): Paginated list of Transaction objects

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/transactions?type=charge&status=succeeded&from=2026-07-01T00:00:00Z" \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /merchants/{merchantId}/transactions

Create a charge transaction.

**Auth**: Bearer token or API key (`transactions:write`)

**Request Body**:

```json
{
  "amount": 1999,
  "currency": "USD",
  "provider": "stripe",
  "customer_id": "550e8400-e29b-41d4-a716-446655440000",
  "payment_method_id": "pm_abc123",
  "idempotency_key": "550e8400-e29b-41d4-a716-446655440001",
  "description": "Order #1234",
  "metadata": {
    "order_id": "1234"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `amount` | integer | yes | Amount in smallest currency unit (cents) |
| `currency` | string | yes | ISO 4217 code |
| `provider` | string | yes | `stripe`, `paypal`, or `square` |
| `customer_id` | UUID | no | Associated customer |
| `payment_method_id` | UUID | no | Tokenized payment method |
| `idempotency_key` | string | no | Unique key for safe retries |
| `description` | string | no | Human-readable description |
| `metadata` | object | no | Custom key-value pairs |

**Response** (`201`): Transaction object

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/transactions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{
    "amount": 1999,
    "currency": "USD",
    "provider": "stripe",
    "description": "Order #1234"
  }'
```

---

### GET /merchants/{merchantId}/transactions/{transactionId}

Get transaction by ID.

**Auth**: Bearer token or API key (`transactions:read`)

**Response** (`200`): Transaction object

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/transactions/$TRANSACTION_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /merchants/{merchantId}/transactions/{transactionId}/refund

Refund a transaction (full or partial).

**Auth**: Bearer token or API key (`transactions:write`)

**Request Body**:

```json
{
  "amount": 500,
  "reason": "Customer returned item",
  "metadata": {
    "return_id": "RET-789"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `amount` | integer | yes | Refund amount in smallest unit |
| `reason` | string | no | Refund reason |
| `metadata` | object | no | Custom data |

**Response** (`201`): Transaction object (type: `refund`)

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/transactions/$TRANSACTION_ID/refund" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"amount": 500, "reason": "Customer returned item"}'
```

---

## Subscriptions

### GET /merchants/{merchantId}/subscriptions

List subscriptions.

**Auth**: Bearer token or API key (`subscriptions:read`)

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | integer | 1 | Page number |
| `page_size` | integer | 20 | Items per page |
| `status` | string | — | Filter: `active`, `past_due`, `cancelled`, `unpaid`, `trialing` |
| `customer_id` | UUID | — | Filter by customer |

**Response** (`200`): Paginated list of Subscription objects

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/subscriptions?status=active" \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /merchants/{merchantId}/subscriptions

Create a subscription.

**Auth**: Bearer token or API key (`subscriptions:write`)

**Request Body**:

```json
{
  "customer_id": "550e8400-e29b-41d4-a716-446655440000",
  "plan_id": "price_pro_monthly",
  "provider": "stripe",
  "trial_period_days": 14,
  "metadata": {
    "campaign": "summer2026"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `customer_id` | UUID | yes | Customer ID |
| `plan_id` | string | yes | Provider plan/price ID |
| `provider` | string | yes | `stripe`, `paypal`, or `square` |
| `trial_period_days` | integer | no | Free trial length |
| `metadata` | object | no | Custom data |

**Response** (`201`): Subscription object

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/subscriptions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "'"$CUSTOMER_ID"'",
    "plan_id": "price_pro_monthly",
    "provider": "stripe",
    "trial_period_days": 14
  }'
```

---

### GET /merchants/{merchantId}/subscriptions/{subscriptionId}

Get subscription by ID.

**Auth**: Bearer token or API key (`subscriptions:read`)

**Response** (`200`): Subscription object

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/subscriptions/$SUBSCRIPTION_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

### PATCH /merchants/{merchantId}/subscriptions/{subscriptionId}

Update a subscription (change plan, schedule cancellation).

**Auth**: Bearer token or API key (`subscriptions:write`)

**Request Body**:

```json
{
  "plan_id": "price_enterprise_monthly",
  "cancel_at": "2026-12-31T23:59:59Z",
  "metadata": {
    "upgraded_from": "pro"
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `plan_id` | string | New provider plan/price ID |
| `cancel_at` | datetime | Schedule cancellation at this time |
| `metadata` | object | Updated custom data |

**Response** (`200`): Updated Subscription object

```bash
curl -X PATCH "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/subscriptions/$SUBSCRIPTION_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"plan_id": "price_enterprise_monthly"}'
```

---

### DELETE /merchants/{merchantId}/subscriptions/{subscriptionId}

Cancel a subscription immediately.

**Auth**: Bearer token or API key (`subscriptions:write`)

**Response** (`200`): Cancelled Subscription object

```bash
curl -X DELETE "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/subscriptions/$SUBSCRIPTION_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Invoices

### GET /merchants/{merchantId}/invoices

List invoices.

**Auth**: Bearer token or API key (`invoices:read`)

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | integer | 1 | Page number |
| `page_size` | integer | 20 | Items per page |
| `status` | string | — | Filter: `draft`, `open`, `paid`, `void`, `uncollectible` |
| `customer_id` | UUID | — | Filter by customer |

**Response** (`200`): Paginated list of Invoice objects

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/invoices?status=open" \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /merchants/{merchantId}/invoices

Create an invoice.

**Auth**: Bearer token or API key (`invoices:write`)

**Request Body**:

```json
{
  "customer_id": "550e8400-e29b-41d4-a716-446655440000",
  "amount": 4999,
  "currency": "USD",
  "due_date": "2026-08-23",
  "description": "July 2026 subscription",
  "metadata": {
    "invoice_number": "INV-001"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `customer_id` | UUID | yes | Customer ID |
| `amount` | integer | yes | Invoice amount in smallest unit |
| `currency` | string | yes | ISO 4217 code |
| `due_date` | date | no | Payment due date |
| `description` | string | no | Invoice description |
| `metadata` | object | no | Custom data |

**Response** (`201`): Invoice object

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/invoices" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "'"$CUSTOMER_ID"'",
    "amount": 4999,
    "currency": "USD",
    "due_date": "2026-08-23"
  }'
```

---

### GET /merchants/{merchantId}/invoices/{invoiceId}

Get invoice by ID.

**Auth**: Bearer token or API key (`invoices:read`)

**Response** (`200`): Invoice object

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/invoices/$INVOICE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Payouts

### GET /merchants/{merchantId}/payouts

List payouts.

**Auth**: Bearer token or API key (`payouts:read`)

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | integer | 1 | Page number |
| `page_size` | integer | 20 | Items per page |
| `status` | string | — | Filter: `pending`, `in_transit`, `paid`, `failed`, `cancelled` |

**Response** (`200`): Paginated list of Payout objects

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/payouts?status=paid" \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /merchants/{merchantId}/payouts/{payoutId}

Request a payout.

**Auth**: Bearer token or API key

**Request Body**:

```json
{
  "amount": 10000,
  "currency": "USD"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `amount` | integer | yes | Payout amount in minor units |
| `currency` | string | yes | ISO 4217 code (3 chars) |

**Response** (`201`): Payout object

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/payouts/$PAYOUT_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"amount": 10000, "currency": "USD"}'
```

---

### GET /merchants/{merchantId}/payouts/{payoutId}

Get payout by ID.

**Auth**: Bearer token or API key (`payouts:read`)

**Response** (`200`): Payout object

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/payouts/$PAYOUT_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Disputes

### GET /merchants/{merchantId}/disputes

List disputes.

**Auth**: Bearer token or API key (`disputes:read`)

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | integer | 1 | Page number |
| `page_size` | integer | 20 | Items per page |
| `status` | string | — | Filter: `warning_needs_response`, `under_review`, `lost`, `won`, `closed` |

**Response** (`200`): Paginated list of Dispute objects

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/disputes?status=warning_needs_response" \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /merchants/{merchantId}/disputes/{disputeId}

Create (initiate) a dispute for a transaction.

**Auth**: Bearer token or API key (`disputes:write`)

**Request Body**:

```json
{
  "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
  "reason": "Unauthorized transaction"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `transaction_id` | UUID | yes | Disputed transaction ID |
| `reason` | string | yes | Dispute reason |

**Response** (`201`): Dispute object

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/disputes/$DISPUTE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "transaction_id": "'"$TRANSACTION_ID"'",
    "reason": "Unauthorized transaction"
  }'
```

---

### GET /merchants/{merchantId}/disputes/{disputeId}

Get dispute by ID.

**Auth**: Bearer token or API key (`disputes:read`)

**Response** (`200`): Dispute object

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/disputes/$DISPUTE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /merchants/{merchantId}/disputes/{disputeId}/evidence

Submit evidence for a dispute.

**Auth**: Bearer token or API key (`disputes:write`)

**Request Body**:

```json
{
  "evidence_text": "The customer authorized this transaction. Signed receipt attached.",
  "evidence_files": [
    "https://storage.example.com/receipt-1234.pdf",
    "https://storage.example.com/delivery-confirmation.png"
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `evidence_text` | string | no | Written evidence statement |
| `evidence_files` | array | no | URLs to supporting documents |

**Response** (`200`): Updated Dispute object

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/disputes/$DISPUTE_ID/evidence" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "evidence_text": "The customer authorized this transaction.",
    "evidence_files": ["https://storage.example.com/receipt.pdf"]
  }'
```

---

## Payment Methods

### GET /merchants/{merchantId}/payment-methods

List payment methods.

**Auth**: Bearer token or API key (`payment_methods:read`)

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | integer | 1 | Page number |
| `page_size` | integer | 20 | Items per page |
| `customer_id` | UUID | — | Filter by customer |
| `type` | string | — | Filter: `card`, `bank_account`, `wallet` |

**Response** (`200`): Paginated list of PaymentMethod objects

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/payment-methods?type=card" \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /merchants/{merchantId}/payment-methods

Create (tokenize) a payment method via provider SDK.

**Auth**: Bearer token or API key (`payment_methods:write`)

**Request Body**:

```json
{
  "provider": "stripe",
  "type": "card",
  "customer_id": "550e8400-e29b-41d4-a716-446655440000",
  "provider_token": "tok_visa_4242",
  "is_default": true,
  "metadata": {}
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provider` | string | yes | `stripe`, `paypal`, or `square` |
| `type` | string | yes | `card`, `bank_account`, or `wallet` |
| `customer_id` | UUID | no | Associated customer |
| `provider_token` | string | no | Token from client-side SDK |
| `is_default` | boolean | no | Set as default (default: `false`) |
| `metadata` | object | no | Custom data |

**Response** (`201`): PaymentMethod object

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/payment-methods" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "stripe",
    "type": "card",
    "customer_id": "'"$CUSTOMER_ID"'",
    "provider_token": "tok_visa_4242",
    "is_default": true
  }'
```

---

### GET /merchants/{merchantId}/payment-methods/{paymentMethodId}

Get payment method by ID.

**Auth**: Bearer token or API key (`payment_methods:read`)

**Response** (`200`): PaymentMethod object

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/payment-methods/$PM_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

### DELETE /merchants/{merchantId}/payment-methods/{paymentMethodId}

Delete a payment method.

**Auth**: Bearer token or API key (`payment_methods:write`)

**Response**: `204 No Content`

```bash
curl -X DELETE "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/payment-methods/$PM_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Webhooks (Outgoing)

### GET /merchants/{merchantId}/webhooks

List webhook configurations.

**Auth**: Bearer token or API key (`webhooks:write`)

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | integer | 1 | Page number |
| `page_size` | integer | 20 | Items per page |

**Response** (`200`): Paginated list of WebhookConfig objects

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/webhooks" \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /merchants/{merchantId}/webhooks

Create a webhook configuration.

**Auth**: Bearer token or API key (`webhooks:write`)

**Request Body**:

```json
{
  "url": "https://myapp.com/webhooks/helix",
  "events": [
    "transaction.succeeded",
    "transaction.failed",
    "subscription.cancelled"
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string (URI) | yes | Webhook endpoint URL |
| `events` | array | yes | Event types to subscribe to |

**Response** (`201`): WebhookConfig object

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/webhooks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://myapp.com/webhooks/helix",
    "events": ["transaction.succeeded", "transaction.failed"]
  }'
```

---

### GET /merchants/{merchantId}/webhooks/{webhookId}

Get webhook config by ID.

**Auth**: Bearer token or API key (`webhooks:write`)

**Response** (`200`): WebhookConfig object

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/webhooks/$WEBHOOK_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

### PUT /merchants/{merchantId}/webhooks/{webhookId}

Update webhook config.

**Auth**: Bearer token or API key (`webhooks:write`)

**Request Body**: Same as create

**Response** (`200`): Updated WebhookConfig object

```bash
curl -X PUT "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/webhooks/$WEBHOOK_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://myapp.com/webhooks/v2",
    "events": ["transaction.succeeded"]
  }'
```

---

### DELETE /merchants/{merchantId}/webhooks/{webhookId}

Delete webhook config.

**Auth**: Bearer token or API key (`webhooks:write`)

**Response**: `204 No Content`

```bash
curl -X DELETE "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/webhooks/$WEBHOOK_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Provider Configs

### GET /merchants/{merchantId}/providers

List payment provider configurations.

**Auth**: Bearer token

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | integer | 1 | Page number |
| `page_size` | integer | 20 | Items per page |

**Response** (`200`): Paginated list of ProviderConfig objects

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/providers" \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /merchants/{merchantId}/providers

Configure a payment provider.

**Auth**: Bearer token

**Request Body**:

```json
{
  "provider": "stripe",
  "config": {
    "api_key": "sk_live_abc123",
    "webhook_secret": "whsec_xyz789"
  },
  "fallback_order": 0
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provider` | string | yes | `stripe`, `paypal`, or `square` |
| `config` | object | yes | Provider-specific credentials |
| `fallback_order` | integer | no | Priority in fallback chain (0 = primary) |

**Response** (`201`): ProviderConfig object

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/providers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "stripe",
    "config": {"api_key": "sk_live_abc123"},
    "fallback_order": 0
  }'
```

---

### GET /merchants/{merchantId}/providers/{providerId}

Get provider config by ID.

**Auth**: Bearer token

**Response** (`200`): ProviderConfig object

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/providers/$PROVIDER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

### PUT /merchants/{merchantId}/providers/{providerId}

Update provider config.

**Auth**: Bearer token

**Request Body**: Same as create

**Response** (`200`): Updated ProviderConfig object

```bash
curl -X PUT "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/providers/$PROVIDER_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "stripe",
    "config": {"api_key": "sk_live_new_key"}
  }'
```

---

### DELETE /merchants/{merchantId}/providers/{providerId}

Delete provider config.

**Auth**: Bearer token

**Response**: `204 No Content`

```bash
curl -X DELETE "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/providers/$PROVIDER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Exchange Rates

### GET /merchants/{merchantId}/exchange-rates

Get cached exchange rates.

**Auth**: Bearer token

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `base_currency` | string | — | Base currency (3 chars) |
| `quote_currency` | string | — | Quote currency (3 chars) |
| `page` | integer | 1 | Page number |
| `page_size` | integer | 20 | Items per page |

**Response** (`200`): Paginated list of ExchangeRate objects

```json
{
  "data": [
    {
      "id": 1,
      "base_currency": "USD",
      "quote_currency": "EUR",
      "rate": "0.92345678",
      "source": "frankfurter",
      "fetched_at": "2026-07-23T10:00:00Z",
      "expires_at": "2026-07-23T11:00:00Z"
    }
  ],
  "page": 1,
  "page_size": 20,
  "total": 1,
  "total_pages": 1
}
```

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/exchange-rates?base_currency=USD&quote_currency=EUR" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Analytics

### GET /merchants/{merchantId}/analytics/summary

Get revenue, transaction count, and success rate summary.

**Auth**: Bearer token or API key (`analytics:read`)

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `period` | string | `month` | `day`, `week`, `month`, `quarter`, `year` |
| `start_date` | date | — | Custom start date |
| `end_date` | date | — | Custom end date |

**Response** (`200`):

```json
{
  "total_revenue": 125000,
  "total_transactions": 342,
  "success_rate": 0.973,
  "avg_transaction_value": 365,
  "period": "month"
}
```

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/analytics/summary?period=month" \
  -H "Authorization: Bearer $TOKEN"
```

---

### GET /merchants/{merchantId}/analytics/transactions

Get transaction analytics broken down by type, status, and provider.

**Auth**: Bearer token or API key (`analytics:read`)

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `period` | string | `month` | `day`, `week`, `month`, `quarter`, `year` |

**Response** (`200`):

```json
{
  "by_type": {
    "charge": 300,
    "refund": 42
  },
  "by_status": {
    "succeeded": 290,
    "failed": 10
  },
  "by_provider": {
    "stripe": 200,
    "paypal": 100,
    "square": 42
  },
  "daily_trend": [
    {
      "date": "2026-07-01",
      "count": 12,
      "revenue": 4500
    }
  ]
}
```

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/analytics/transactions?period=month" \
  -H "Authorization: Bearer $TOKEN"
```

---

### POST /merchants/{merchantId}/analytics/export

Export analytics report as CSV or PDF.

**Auth**: Bearer token or API key (`analytics:read`)

**Request Body**:

```json
{
  "format": "csv",
  "start_date": "2026-07-01",
  "end_date": "2026-07-23"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `format` | string | yes | `csv` or `pdf` |
| `start_date` | date | no | Start of report range |
| `end_date` | date | no | End of report range |

**Response** (`200`):

```json
{
  "download_url": "https://storage.example.com/reports/analytics-2026-07.csv",
  "expires_at": "2026-07-23T22:00:00Z"
}
```

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/analytics/export" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"format": "csv", "start_date": "2026-07-01", "end_date": "2026-07-23"}'
```

---

## Billing

### GET /merchants/{merchantId}/billing/fees

Get platform fee breakdown for a period.

**Auth**: Bearer token or API key (`billing:read`)

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `period` | string | `month` | `month`, `quarter`, `year` |
| `start_date` | date | — | Custom start date |
| `end_date` | date | — | Custom end date |

**Response** (`200`):

```json
{
  "total_fees": 3750,
  "fee_breakdown": [
    {
      "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
      "fee_amount": 60,
      "fee_percentage": 2.9,
      "provider": "stripe",
      "created_at": "2026-07-23T10:00:00Z"
    }
  ],
  "period": "month"
}
```

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/billing/fees?period=month" \
  -H "Authorization: Bearer $TOKEN"
```

---

### GET /merchants/{merchantId}/billing/invoices

Get platform billing invoices (Helix fees owed by the merchant).

**Auth**: Bearer token or API key (`billing:read`)

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | integer | 1 | Page number |
| `page_size` | integer | 20 | Items per page |

**Response** (`200`): Paginated list of billing invoice objects

```json
{
  "data": [
    {
      "id": "b1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "merchant_id": "550e8400-e29b-41d4-a716-446655440000",
      "period_start": "2026-07-01",
      "period_end": "2026-07-31",
      "total_fees": 3750,
      "status": "open",
      "due_date": "2026-08-15",
      "created_at": "2026-08-01T00:00:00Z"
    }
  ],
  "page": 1,
  "page_size": 20,
  "total": 1,
  "total_pages": 1
}
```

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/billing/invoices" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Users

### GET /users/{userId}

Get user by ID.

**Auth**: Bearer token

**Response** (`200`): User object

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "admin@acme.com",
  "name": "Jane Doe",
  "role": "root_admin",
  "merchant_id": "550e8400-e29b-41d4-a716-446655440000",
  "is_active": true,
  "mfa_enabled": false,
  "created_at": "2026-07-23T10:00:00Z",
  "updated_at": "2026-07-23T10:00:00Z"
}
```

```bash
curl "https://seller.hxd3v.com/api/v1/users/$USER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

### PUT /users/{userId}

Update user profile or role.

**Auth**: Bearer token

**Request Body**:

```json
{
  "name": "Jane Doe-Smith",
  "role": "account_admin",
  "is_active": true
}
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | User's display name |
| `role` | string | `root_admin`, `account_admin`, or `user` |
| `is_active` | boolean | Enable/disable account |

**Response** (`200`): Updated User object

```bash
curl -X PUT "https://seller.hxd3v.com/api/v1/users/$USER_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role": "account_admin"}'
```

---

## API Keys

### POST /api-keys

Create an API key.

**Auth**: Bearer token

**Request Body**:

```json
{
  "name": "Production Backend",
  "scopes": ["transactions:read", "transactions:write"],
  "rate_limit": 50,
  "expires_at": "2026-12-31T23:59:59Z"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Key name |
| `scopes` | array | no | Permission scopes (empty = full access) |
| `rate_limit` | integer | no | Requests/sec (0 = unlimited, default) |
| `expires_at` | datetime | no | Expiration (null = never) |

**Response** (`201`):

```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "name": "Production Backend",
  "key": "hxs_live_abc123def456ghi789jkl012mno345pqr678stu901",
  "key_prefix": "hxs_live_",
  "created_at": "2026-07-23T10:00:00Z"
}
```

```bash
curl -X POST https://seller.hxd3v.com/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Production Backend", "scopes": ["transactions:read"]}'
```

---

### GET /api-keys

List API keys.

**Auth**: Bearer token

**Query Parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `page` | integer | 1 | Page number |
| `page_size` | integer | 20 | Items per page |

**Response** (`200`): Paginated list of ApiKey objects

```bash
curl https://seller.hxd3v.com/api/v1/api-keys \
  -H "Authorization: Bearer $TOKEN"
```

---

### DELETE /api-keys/{apiKeyId}

Revoke an API key.

**Auth**: Bearer token

**Response**: `204 No Content`

```bash
curl -X DELETE https://seller.hxd3v.com/api/v1/api-keys/$API_KEY_ID \
  -H "Authorization: Bearer $TOKEN"
```

---

## Webhook Ingress

These endpoints receive webhooks from payment providers. They do not require authentication (providers authenticate via their own signature mechanisms).

### POST /webhooks/stripe

Receive Stripe webhook events.

**Auth**: None (verified via Stripe signature header)

```bash
# This is called by Stripe, not by your application
curl -X POST https://seller.hxd3v.com/api/v1/webhooks/stripe \
  -H "Content-Type: application/json" \
  -H "Stripe-Signature: t=1234567890,v1=abc123..." \
  -d '{"id": "evt_123", "type": "payment_intent.succeeded", ...}'
```

---

### POST /webhooks/paypal

Receive PayPal webhook events.

**Auth**: None (verified via PayPal signature)

```bash
curl -X POST https://seller.hxd3v.com/api/v1/webhooks/paypal \
  -H "Content-Type: application/json" \
  -d '{"id": "WH-123", "event_type": "PAYMENT.CAPTURE.COMPLETED", ...}'
```

---

### POST /webhooks/square

Receive Square webhook events.

**Auth**: None (verified via Square signature)

```bash
curl -X POST https://seller.hxd3v.com/api/v1/webhooks/square \
  -H "Content-Type: application/json" \
  -d '{"type": "payment.completed", "data": {...}}'
```

---

## System

### GET /health

Health check. Returns service status.

**Auth**: None

**Response** (`200`):

```json
{
  "status": "healthy",
  "time": "2026-07-23T10:00:00Z"
}
```

```bash
curl https://seller.hxd3v.com/api/v1/health
```
