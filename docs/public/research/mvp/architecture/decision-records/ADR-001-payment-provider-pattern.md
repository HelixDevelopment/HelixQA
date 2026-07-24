# ADR-001: Payment Provider Adapter Pattern

## Status

**Accepted** — 2026-07-23

## Context

Helix Seller must support multiple payment providers (Stripe, PayPal, Square) behind a single unified API. Each provider has its own:

- API request/response formats
- Authentication mechanisms (API keys, OAuth2, client credentials)
- Webhook payload structures and signature schemes
- Error semantics and retry behaviors
- Subscription lifecycle models and state machines
- Currency and tax handling rules

Merchant applications should integrate once against a single interface and be able to switch or add providers without modifying application code. The system must also support automatic provider fallback: if the primary provider is unavailable or failing, traffic should route to the next healthy provider transparently.

The core problem is **provider heterogeneity** — each integration point introduces unique complexity that must be contained, not propagated through the system.

## Decision

We adopt the **Adapter pattern** (GoF) for payment provider integration. Each provider is wrapped behind a common `PaymentProvider` interface that defines the operations the rest of the system needs:

```go
type PaymentProvider interface {
    // Charge initiates a payment against a tokenized payment method.
    Charge(ctx context.Context, req ChargeRequest) (*ChargeResponse, error)

    // Refund issues a full or partial refund against a previous charge.
    Refund(ctx context.Context, req RefundRequest) (*RefundResponse, error)

    // CreateSubscription creates a recurring billing agreement.
    CreateSubscription(ctx context.Context, req SubscriptionRequest) (*SubscriptionResponse, error)

    // CancelSubscription terminates a recurring billing agreement.
    CancelSubscription(ctx context.Context, id string) error

    // VerifyWebhookSignature validates the authenticity of an incoming webhook.
    VerifyWebhookSignature(payload []byte, signature string) (bool, error)

    // ParseWebhookEvent normalizes a provider-specific webhook into a domain event.
    ParseWebhookEvent(payload []byte) (*WebhookEvent, error)
}
```

Each provider implements this interface in a dedicated adapter package:

```
internal/provider/
├── stripe/
│   ├── adapter.go        # StripeAdapter implements PaymentProvider
│   ├── client.go         # Stripe API client
│   ├── mapper.go         # Domain ↔ Stripe request/response mapping
│   └── webhook.go        # Stripe webhook parsing and verification
├── paypal/
│   ├── adapter.go
│   ├── client.go
│   ├── mapper.go
│   └── webhook.go
├── square/
│   ├── adapter.go
│   ├── client.go
│   ├── mapper.go
│   └── webhook.go
└── provider.go           # PaymentProvider interface definition
```

The **Payment Router** holds a registry of configured providers and selects the target based on:

1. Merchant provider preferences (priority order)
2. Currency support (not all providers support all currencies)
3. Circuit breaker state (open circuits exclude the provider)
4. Geographic routing rules (regulatory or performance-based)

Provider configuration is stored in PostgreSQL and hot-reloaded via Redis cache invalidation when the merchant updates settings.

## Consequences

### Positive

- **New providers require one new adapter package** — no changes to the routing, processing, or API layers.
- **Clear separation of concerns** — provider-specific quirks (auth, data formats, error handling) are isolated to the adapter.
- **Testability** — the `PaymentProvider` interface enables mocking for unit and integration tests.
- **Fallback is natural** — the router iterates through the provider list; a failed circuit breaker skips to the next.
- **Provider deprecation** — removing a provider means deleting its adapter and updating configuration. No code paths change.
- **Webhook uniformity** — `ParseWebhookEvent` normalizes all provider webhooks into a single `WebhookEvent` domain model, simplifying downstream processing.

### Negative

- **Interface design up front** — the `PaymentProvider` interface must cover all provider capabilities. If a capability is unique to one provider (e.g., Stripe's 3D Secure), it may need extension via provider-specific methods or a capability query interface.
- **Mapping complexity** — each adapter must maintain bidirectional mappers between domain models and provider models. These mappers need thorough test coverage.
- **Provider API versioning** — adapters must handle provider API version changes independently. A breaking change in one provider's API does not affect others, but each adapter must be updated.
- **Initial implementation cost** — three adapters (Stripe, PayPal, Square) must be built before the system is functional. This is mitigated by starting with one provider (Stripe) and adding others incrementally.

### Mitigations

- Start with a minimal interface covering charge, refund, subscription CRUD, and webhook handling. Extend as real-world requirements emerge.
- Write comprehensive adapter tests using provider sandbox/test environments.
- Pin provider SDK versions in `go.mod` and update deliberately.
