# Quickstart Validation Guide: Helix Seller Platform

**Date**: 2026-07-23
**Spec**: [spec.md](spec.md)

## Prerequisites

- Go 1.22+
- PostgreSQL 16+ (or Docker/Podman)
- Redis 7+ (or Docker/Podman)
- Node.js 18+ (for web portal)

## Setup

```bash
# Clone and enter project
cd helix_seller

# Copy environment config
cp .env.example .env

# Start dependencies
make deps-up

# Run database migrations
make migrate-up

# Start the API server
make run
```

Server starts at http://localhost:8080

## Validation Scenarios

### Scenario 1: Merchant Onboarding (P1)

**Proves**: Merchant can be created and configured with a payment provider

```bash
# Create merchant
curl -X POST http://localhost:8080/api/v1/merchants \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Shop","email":"shop@test.com","default_currency":"USD"}'

# Response: {"id":"...","name":"Test Shop","status":"active",...}

# Configure Stripe provider
curl -X POST http://localhost:8080/api/v1/merchants/{merchantId}/providers \
  -H "Content-Type: application/json" \
  -d '{"provider":"stripe","config":{"api_key":"sk_test_..."},"fallback_order":0}'

# Expected: Provider configured, health_status: "healthy"
```

**Success criteria**: Merchant created, provider configured, test transaction succeeds

### Scenario 2: Process a Charge (P1)

**Proves**: Payment can be processed through the unified API

```bash
# Create customer
curl -X POST http://localhost:8080/api/v1/merchants/{merchantId}/customers \
  -H "Content-Type: application/json" \
  -d '{"name":"John Doe","email":"john@example.com"}'

# Process charge
curl -X POST http://localhost:8080/api/v1/merchants/{merchantId}/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 5000,
    "currency": "USD",
    "provider": "stripe",
    "customer_id": "{customerId}",
    "idempotency_key": "txn-001"
  }'

# Expected: Transaction with status "succeeded", amount: 5000, currency: "USD"
```

**Success criteria**: Transaction created, idempotent (replaying key returns same result)

### Scenario 3: Refund Transaction (P2)

**Proves**: Refund works through unified API

```bash
# Refund charge
curl -X POST http://localhost:8080/api/v1/merchants/{merchantId}/transactions/{transactionId}/refund \
  -H "Content-Type: application/json" \
  -d '{"amount": 2500,"reason":"customer_request"}'

# Expected: Refund transaction with status "succeeded", amount: 2500
```

**Success criteria**: Partial refund created, original transaction updated

### Scenario 4: Subscription Lifecycle (P2)

**Proves**: Recurring billing works end-to-end

```bash
# Create subscription
curl -X POST http://localhost:8080/api/v1/merchants/{merchantId}/subscriptions \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "{customerId}",
    "plan_id": "price_monthly_10",
    "provider": "stripe"
  }'

# Expected: Subscription with status "active", current_period_end set

# Cancel subscription
curl -X DELETE http://localhost:8080/api/v1/merchants/{merchantId}/subscriptions/{subscriptionId}

# Expected: Subscription with status "cancelled", cancel_at set
```

**Success criteria**: Subscription created, cancellation scheduled

### Scenario 5: Idempotency (P1)

**Proves**: Duplicate requests return same result

```bash
# Send same charge twice with same idempotency_key
curl -X POST http://localhost:8080/api/v1/merchants/{merchantId}/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":1000,"currency":"USD","provider":"stripe","idempotency_key":"idem-001"}'

curl -X POST http://localhost:8080/api/v1/merchants/{merchantId}/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":1000,"currency":"USD","provider":"stripe","idempotency_key":"idem-001"}'

# Expected: Both return identical response, only one transaction created
```

**Success criteria**: Second request returns cached response, no duplicate transaction

### Scenario 6: Webhook Ingress (P2)

**Proves**: Provider webhooks are received and processed

```bash
# Simulate Stripe webhook (requires valid signature)
curl -X POST http://localhost:8080/webhooks/stripe \
  -H "Content-Type: application/json" \
  -H "Stripe-Signature: t=...,v1=..." \
  -d '{"type":"payment_intent.succeeded","data":{"object":{"id":"pi_..."}}}'

# Expected: 200 OK, event processed, transaction updated
```

**Success criteria**: Webhook received, signature verified, event processed idempotently

### Scenario 7: Provider Fallback (P2)

**Proves**: Automatic fallback when primary provider fails

```bash
# Configure two providers with fallback order
curl -X POST http://localhost:8080/api/v1/merchants/{merchantId}/providers \
  -d '{"provider":"stripe","config":{"api_key":"sk_test_..."},"fallback_order":0}'

curl -X POST http://localhost:8080/api/v1/merchants/{merchantId}/providers \
  -d '{"provider":"paypal","config":{"client_id":"...","secret":"..."},"fallback_order":1}'

# Simulate Stripe failure (use invalid key or mock)
# Process charge with fallback
curl -X POST http://localhost:8080/api/v1/merchants/{merchantId}/transactions \
  -d '{"amount":1000,"currency":"USD","provider":"stripe","idempotency_key":"fb-001"}'

# Expected: If Stripe fails, fallback to PayPal; transaction succeeds via PayPal
```

**Success criteria**: Fallback triggers automatically, transaction completes via secondary provider

### Scenario 8: Multi-Currency (P2)

**Proves**: Currency conversion works

```bash
# Charge in EUR, merchant default is USD
curl -X POST http://localhost:8080/api/v1/merchants/{merchantId}/transactions \
  -d '{"amount":1000,"currency":"EUR","provider":"stripe","idempotency_key":"mc-001"}'

# Expected: Transaction created, exchange rate applied, net_amount in USD calculated
```

**Success criteria**: Exchange rate fetched, conversion applied, amounts correct

### Scenario 9: API Response Time (P1)

**Proves**: Performance SLO is met

```bash
# Measure response time for 100 requests
for i in $(seq 1 100); do
  curl -w "%{time_total}\n" -o /dev/null -s http://localhost:8080/health
done | sort -n | tail -1

# Expected: p95 < 150ms (95th percentile under 150ms)
```

**Success criteria**: p95 latency < 150ms for health endpoint

### Scenario 10: Web Portal Load (P3)

**Proves**: Dashboard loads within SLO

```bash
# Start web portal
cd web && npm start

# Open browser, navigate to http://localhost:4200
# Use Lighthouse to measure:
# - First Contentful Paint < 1.5s
# - Largest Contentful Paint < 2.5s
# - Cumulative Layout Shift < 0.1
```

**Success criteria**: Dashboard loads in < 1.5s, Lighthouse score > 90

## Test Data

For development/testing, use these pre-configured values:

- **Stripe test key**: `sk_test_...` (from Stripe dashboard)
- **PayPal sandbox**: `client_id` + `secret` from PayPal developer portal
- **Square sandbox**: `access_token` from Square developer portal
- **Test card (Stripe)**: 4242 4242 4242 4242, any future date, any CVC
- **Test card (PayPal)**: 4111 1111 1111 1111 (sandbox)
- **Test card (Square)**: 4532 7597 3454 5858 (sandbox)

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Server won't start | Check PostgreSQL and Redis are running: `make deps-up` |
| Migration fails | Check database URL in `.env` matches PostgreSQL config |
| Provider returns 401 | Verify API key is correct and not expired |
| Webhook not received | Check provider dashboard for delivery logs; verify URL is correct |
| Slow response times | Check Redis is running; verify database indexes are created |
