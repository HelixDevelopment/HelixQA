# Research: Helix Seller Platform

**Date**: 2026-07-23
**Spec**: [spec.md](spec.md)

## Payment Provider Integration Research

### Stripe Integration

**Decision**: Primary provider via Stripe API v2024-12-18+
**Rationale**: Most popular payment platform, excellent API design, comprehensive documentation, strong Go SDK (github.com/stripe/stripe-go/v76)
**Alternatives considered**: Adyen (more complex onboarding), Braintree (PayPal-owned, less independent)

**Key endpoints used**:
- Payment Intents (charges)
- Refunds
- Subscriptions + Plans
- Invoices
- Payouts (Stripe Connect for marketplace)
- Disputes + Evidence

**Webhook events**: payment_intent.succeeded, payment_intent.payment_failed, charge.refunded, invoice.paid, invoice.payment_failed, payout.paid, payout.failed, dispute.created, dispute.closed

### PayPal Integration

**Decision**: Via PayPal REST API v2
**Rationale**: Second-largest payment platform, strong wallet/BNPL adoption, good international coverage
**Alternatives considered**: Braintree (more complex, owned by PayPal)

**Key endpoints used**:
- Orders (charges)
- Captures
- Refunds
- Subscriptions
- Payouts (PayPal Payouts API)

**Webhook events**: PAYMENT.CAPTURE.COMPLETED, PAYMENT.CAPTURE.DENIED, PAYMENT.CAPTURE.REFUNDED, BILLING.SUBSCRIPTION.ACTIVATED, BILLING.SUBSCRIPTION.CANCELLED

### Square Integration

**Decision**: Via Square Connect API v2
**Rationale**: Strong POS/in-person payments, good for hybrid online+physical merchants
**Alternatives considered**: None at MVP (in-person payment focus)

**Key endpoints used**:
- Payments (charges)
- Refunds
- Subscriptions (Square Subscriptions API)
- Payouts

**Webhook events**: payment.completed, payment.failed, refund.completed, subscription.updated

## Exchange Rate Research

**Decision**: Free tier primary (frankfurter.app + exchangerate-api.com) with paid fallback (Open Exchange Rates)
**Rationale**: frankfurter.app is free, open-source, updated daily from ECB; exchangerate-api.com free tier provides 1500 req/month; Open Exchange Rates paid tier as fallback for high-volume needs
**Alternatives considered**: Provider-native conversion (higher per-txn cost), manual rates (no automation), Fixer.io (paid only)

**Caching strategy**: Redis with 1-hour TTL for free tier, 5-minute TTL for paid tier. Fallback chain: frankfurter → exchangerate-api → Open Exchange Rates → last cached value.

## Idempotency Research

**Decision**: PostgreSQL advisory locks + idempotency key table
**Rationale**: Advisory locks provide row-level exclusive access without explicit lock tables; idempotency key table stores request hashes with TTL for deduplication
**Alternatives considered**: Redis-only idempotency (volatile, lost on restart), application-level mutexes (don't survive restarts)

**Implementation**: 
1. Client sends idempotency key in request header
2. System computes hash(key + endpoint + body)
3. INSERT into idempotency_keys with ON CONFLICT DO NOTHING
4. If conflict → return cached response
5. If new → acquire advisory lock, process, store response, release lock
6. TTL: 24 hours for charges, 7 days for subscriptions

## Circuit Breaker Research

**Decision**: Custom implementation following Netflix Hystrix pattern
**Rationale**: Go ecosystem lacks a mature, maintained circuit breaker library; custom implementation allows fine-tuning for payment provider specifics
**Alternatives considered**: sony/gobreaker (unmaintained), afex/hystrix-go (unmaintained)

**States**: Closed (normal) → Open (failing, reject immediately) → Half-Open (test with single request)
**Thresholds**: 3 consecutive failures → Open; 30s cooldown → Half-Open; 1 success → Closed; 1 failure → Open

## Webhook Signature Verification Research

**Decision**: Per-provider HMAC verification using provider-specific algorithms
**Rationale**: Each provider uses different signing mechanisms; must verify authenticity before processing

**Provider-specific approaches**:
- **Stripe**: HMAC-SHA256 with webhook signing secret (stripe-signature header)
- **PayPal**: RSA-SHA256 with PayPal's certificate chain (PayPal-Auth-Algo + PayPal-Cert-Url headers)
- **Square**: HMAC-SHA256 with webhook signature key (x-square-hmacsha256 header)

## PCI DSS Tokenization Research

**Decision**: Delegate all card tokenization to payment providers
**Rationale**: Providers (Stripe.js, PayPal JS SDK, Square Web Payments SDK) handle card data on the client side, returning tokens. Platform never sees raw card data.
**Alternatives considered**: Self-hosted tokenization (requires PCI SAQ D compliance, significantly more complex)

**Implementation**: Client-side SDKs (Stripe Elements, PayPal Buttons, Square Web Payments) tokenize card data → platform receives tokens → uses tokens for subsequent operations.

## Background Job Queue Research

**Decision**: PostgreSQL-backed task queue (in-house pattern)
**Rationale**: Avoids additional infrastructure (Redis-based queues add complexity); PostgreSQL provides ACID, durability, and advisory locks for concurrent workers
**Alternatives considered**: Redis-based queue (less durable), NATS JetStream (already used for events, but not ideal for job semantics), external job queue (over-engineered for MVP)

**Implementation**: tasks table with status (pending/running/completed/failed), worker pool claiming tasks with SELECT ... FOR UPDATE SKIP LOCKED, exponential backoff on failure, DLQ for permanently failed tasks.

## Reconciliation Research

**Decision**: Daily batch reconciliation with hourly incremental checks
**Rationale**: Payment reconciliation must match platform records against provider records; daily is sufficient for most use cases; hourly catches critical discrepancies faster
**Alternatives considered**: Real-time reconciliation (too expensive for API calls), weekly (too slow for dispute deadlines)

**Implementation**: For each provider, fetch settled transactions for the period, compare against platform records, flag discrepancies (missing, amount mismatch, status mismatch), generate reconciliation report, alert on critical mismatches.

## Multi-Currency Settlement Research

**Decision**: Merchant settlement in preferred currency; platform handles conversion
**Rationale**: Merchants want to receive funds in their local currency; platform absorbs conversion risk via exchange rate margins
**Alternatives considered**: Merchant receives in transaction currency (complex for merchants), provider-native settlement (limited to provider's supported currencies)

**Implementation**: Transaction amount in original currency → exchange rate applied at time of transaction → settlement amount in merchant's preferred currency → payout via provider.
