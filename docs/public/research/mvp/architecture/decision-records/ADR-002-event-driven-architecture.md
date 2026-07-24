# ADR-002: Event-Driven Architecture with NATS JetStream

## Status

**Accepted** — 2026-07-23

## Context

Helix Seller processes payment operations that generate multiple downstream effects:

- **Internal state updates** — subscription status changes, invoice generation, audit trail entries
- **Outgoing notifications** — webhook dispatch to merchant applications, email notifications, real-time dashboard updates
- **Cross-service communication** — the web dashboard, CLI, and mobile clients need real-time payment status
- **Reconciliation** — periodic comparison of internal state with provider state requires event history
- **Debugging and audit** — complete event log for troubleshooting failed payments, subscription disputes, and compliance

The system must guarantee that events are not lost, are processed at least once, and can be consumed by multiple independent consumers without tight coupling between the event producer and consumers.

Previous approaches considered:

1. **Direct function calls** — simple but creates tight coupling; adding a new consumer requires modifying the producer.
2. **Database polling** — reliable but inefficient; introduces latency and database load.
3. **Redis Pub/Sub** — fast but fire-and-forget; no persistence, no replay, no consumer groups.
4. **RabbitMQ** — robust but operationally complex; overkill for the current scale.

## Decision

We adopt **NATS JetStream** as the internal event bus for all domain events. NATS JetStream provides:

- **Durability** — messages are persisted to disk and survive broker restarts
- **At-least-once delivery** — consumers acknowledge messages; unacknowledged messages are redelivered
- **Ordered streams** — messages within a stream are strictly ordered by sequence number
- **Consumer groups** — multiple consumers can process events in parallel without duplication
- **Replay** — consumers can start from any point in the stream history
- **Horizontal scaling** — NATS clustering supports scaling across nodes

### Event Schema

All domain events follow a common envelope:

```go
type DomainEvent struct {
    ID            string          `json:"id"`             // UUID v4, unique per event
    Type          string          `json:"type"`           // e.g., "payment.succeeded"
    AggregateID   string          `json:"aggregate_id"`   // Entity ID (payment, subscription)
    AggregateType string          `json:"aggregate_type"` // e.g., "payment", "subscription"
    Timestamp     time.Time       `json:"timestamp"`      // UTC, nanosecond precision
    Version       int             `json:"version"`        // Schema version for evolution
    Metadata      map[string]string `json:"metadata"`    // Correlation ID, actor, source
    Payload       json.RawMessage `json:"payload"`        // Event-specific data
}
```

### Event Types

| Event | Trigger | Consumers |
|-------|---------|-----------|
| `payment.initiated` | Payment charge requested | Audit, dashboard |
| `payment.succeeded` | Payment confirmed by provider | Subscription service, invoice service, outgoing webhooks, dashboard |
| `payment.failed` | Payment rejected by provider | Retry engine, outgoing webhooks, dashboard |
| `payment.refunded` | Refund processed | Invoice service, outgoing webhooks, dashboard |
| `subscription.created` | New subscription activated | Outgoing webhooks, dashboard |
| `subscription.updated` | Plan change or renewal | Outgoing webhooks, dashboard |
| `subscription.cancelled` | Cancellation requested | Outgoing webhooks, dashboard |
| `subscription.expired` | Subscription lapsed | Outgoing webhooks, dashboard |
| `invoice.generated` | Invoice PDF created | Outgoing webhooks, object storage |
| `webhook.received` | Incoming provider webhook | Audit, event store |
| `reconciliation.discrepancy` | State mismatch detected | Alerting, dashboard |

### Stream Configuration

```yaml
streams:
  - name: DOMAIN_EVENTS
    subjects:
      - "domain.>"
    storage: file
    retention: limits
    max_age: 720h        # 30 days
    max_msgs: 10000000
    replication: 1        # single-node; increase for cluster
    discard: old
```

### Consumer Groups

| Consumer Group | Purpose | Delivery |
|---------------|---------|----------|
| `webhook-dispatch` | Send outgoing webhooks to merchant applications | At-least-once, ordered per subscription |
| `analytics` | Feed payment/subscription data to analytics pipeline | At-least-once, independent |
| `reconciliation` | Periodic state comparison with providers | At-least-once, scheduled |
| `audit-logger` | Append all events to audit trail | Exactly-once (deduplicated by event ID) |
| `realtime-push` | Push events to connected WebSocket/SSE clients | At-most-once (real-time, best-effort) |

## Consequences

### Positive

- **Loose coupling** — producers publish events without knowing who consumes them. New consumers are added by creating a new subscriber, not by modifying the producer.
- **Durability** — events are never lost. If a consumer is down, it resumes from where it left off (persistent consumers) or from a configured start point (new consumers).
- **Replay capability** — the full event history is available for debugging, reconciliation, and rebuilding read models.
- **Scalability** — NATS JetStream scales horizontally. Adding nodes increases throughput without code changes.
- **Operational simplicity** — NATS is a single binary with minimal configuration. No separate broker cluster management (unlike Kafka or RabbitMQ).
- **Webhook reliability** — outgoing webhook dispatch is driven by events, not by inline calls. Failed dispatches are retried independently without blocking the payment flow.
- **Audit trail** — the event stream serves as a natural audit log. All state transitions are captured with full context.

### Negative

- **Eventual consistency** — consumers see state changes asynchronously. The dashboard may briefly show stale data after a payment succeeds. This is acceptable for a payment system where correctness matters more than immediacy.
- **Event schema evolution** — changing event schemas requires versioning. Consumers must handle multiple schema versions during rolling deployments. Mitigated by the `Version` field and backward-compatible schema changes.
- **Debugging complexity** — tracing a payment through multiple async consumers requires correlation IDs. Mitigated by the `Metadata.CorrelationID` field propagated through all events.
- **NATS as a dependency** — the system now depends on NATS availability. Mitigated by JetStream durability and the fact that NATS failure degrades real-time features (WebSocket push, outgoing webhooks) but does not block payment processing (which writes directly to PostgreSQL).

### Mitigations

- Use correlation IDs consistently across all events and log entries for end-to-end traceability.
- Implement dead-letter queues for events that fail processing after maximum retries.
- Monitor consumer lag and alert when it exceeds thresholds.
- Keep event schemas backward-compatible; use additive changes where possible.
- Payment operations write to PostgreSQL synchronously (source of truth) and publish events asynchronously (notifications). This ensures payment correctness even if the event bus is temporarily unavailable.
