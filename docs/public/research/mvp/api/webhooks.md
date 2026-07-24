# Webhook Documentation

Webhooks let you receive real-time notifications when events occur in the Helix Seller platform — transactions settle, subscriptions cancel, disputes open, and more.

Helix Seller handles two directions of webhook traffic:

1. **Incoming** — Helix receives webhooks from payment providers (Stripe, PayPal, Square).
2. **Outgoing** — Helix sends webhooks to your configured endpoint.

---

## Outgoing Webhooks (Merchant-Configured)

### Creating a Webhook

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/webhooks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://myapp.com/webhooks/helix",
    "events": [
      "transaction.succeeded",
      "transaction.failed",
      "subscription.cancelled",
      "dispute.opened"
    ]
  }'
```

**Response** (`201 Created`):

```json
{
  "id": "w1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "merchant_id": "550e8400-e29b-41d4-a716-446655440000",
  "url": "https://myapp.com/webhooks/helix",
  "events": ["transaction.succeeded", "transaction.failed", "subscription.cancelled", "dispute.opened"],
  "is_active": true,
  "metadata": {},
  "created_at": "2026-07-23T10:00:00Z",
  "updated_at": "2026-07-23T10:00:00Z"
}
```

> **Note**: The webhook `secret` is generated server-side and used for signature verification. It is not exposed in the API response. Contact support to retrieve or rotate the signing secret.

### Webhook Payload Format

All outgoing webhook payloads follow this structure:

```json
{
  "id": "evt_a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "type": "transaction.succeeded",
  "created_at": "2026-07-23T10:00:00Z",
  "merchant_id": "550e8400-e29b-41d4-a716-446655440000",
  "data": {
    "object": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "type": "charge",
      "amount": 1999,
      "currency": "USD",
      "status": "succeeded",
      "provider": "stripe",
      "provider_transaction_id": "ch_abc123",
      "customer_id": "cust_001",
      "created_at": "2026-07-23T09:55:00Z"
    }
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Unique event ID |
| `type` | string | Event type (e.g., `transaction.succeeded`) |
| `created_at` | datetime | When the event occurred |
| `merchant_id` | UUID | Merchant that owns the event |
| `data.object` | object | The resource that triggered the event |

### Signature Verification

Each outgoing webhook includes an `X-Helix-Signature` header containing an HMAC-SHA256 signature of the raw request body, signed with your webhook secret.

```
X-Helix-Signature: sha256=a1b2c3d4e5f6...
```

**Verification in Node.js**:

```javascript
const crypto = require('crypto');

function verifyWebhookSignature(payload, signature, secret) {
  const expected = 'sha256=' + crypto
    .createHmac('sha256', secret)
    .update(payload, 'utf8')
    .digest('hex');

  return crypto.timingSafeEqual(
    Buffer.from(signature),
    Buffer.from(expected)
  );
}

// In your webhook handler:
app.post('/webhooks/helix', (req, res) => {
  const signature = req.headers['x-helix-signature'];
  const isValid = verifyWebhookSignature(
    JSON.stringify(req.body),
    signature,
    process.env.HELIX_WEBHOOK_SECRET
  );

  if (!isValid) {
    return res.status(401).json({ error: 'Invalid signature' });
  }

  // Process the event
  const event = req.body;
  switch (event.type) {
    case 'transaction.succeeded':
      handleTransactionSuccess(event.data.object);
      break;
    case 'subscription.cancelled':
      handleSubscriptionCancelled(event.data.object);
      break;
  }

  res.status(200).json({ received: true });
});
```

**Verification in Python**:

```python
import hmac
import hashlib

def verify_webhook_signature(payload: bytes, signature: str, secret: str) -> bool:
    expected = 'sha256=' + hmac.new(
        secret.encode('utf-8'),
        payload,
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(signature, expected)

# In your webhook handler (Flask example):
@app.route('/webhooks/helix', methods=['POST'])
def handle_webhook():
    signature = request.headers.get('X-Helix-Signature', '')
    if not verify_webhook_signature(request.data, signature, WEBHOOK_SECRET):
        return jsonify({'error': 'Invalid signature'}), 401

    event = request.json
    if event['type'] == 'transaction.succeeded':
        handle_transaction_success(event['data']['object'])

    return jsonify({'received': True}), 200
```

### Retry Policy

If your endpoint returns a non-2xx status code or times out (30 second timeout), the webhook is retried with exponential backoff:

| Attempt | Delay | Cumulative Time |
|---------|-------|-----------------|
| 1 | Immediate | 0 |
| 2 | 1 minute | 1 min |
| 3 | 5 minutes | 6 min |
| 4 | 30 minutes | 36 min |
| 5 | 2 hours | 2 hrs 36 min |
| 6 | 24 hours | 26 hrs 36 min |

After 6 failed attempts, the webhook event is marked as `failed` and a dead-letter record is created. You can replay failed events via the API.

**Best practices**:

- Return `200` quickly (within 5 seconds). Process asynchronously.
- Use idempotency in your handler — the same event may be delivered more than once.
- Monitor the webhook configuration for `last_triggered_at` and error rates.

### Managing Webhooks

**List webhooks**:

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/webhooks" \
  -H "Authorization: Bearer $TOKEN"
```

**Get a specific webhook**:

```bash
curl "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/webhooks/$WEBHOOK_ID" \
  -H "Authorization: Bearer $TOKEN"
```

**Update a webhook**:

```bash
curl -X PUT "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/webhooks/$WEBHOOK_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://myapp.com/webhooks/v2",
    "events": ["transaction.succeeded"]
  }'
```

**Delete a webhook**:

```bash
curl -X DELETE "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/webhooks/$WEBHOOK_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Incoming Webhooks (Provider → Helix)

Helix Seller receives webhooks from Stripe, PayPal, and Square to keep transaction statuses in sync. These endpoints are unauthenticated (providers authenticate via their own signature mechanisms).

### Stripe Ingress

**Endpoint**: `POST /webhooks/stripe`

Stripe sends events to this endpoint. Helix verifies the `Stripe-Signature` header using the configured webhook signing secret.

**Supported Stripe events**:

- `payment_intent.succeeded`
- `payment_intent.payment_failed`
- `charge.refunded`
- `charge.dispute.created`
- `charge.dispute.closed`
- `customer.subscription.created`
- `customer.subscription.updated`
- `customer.subscription.deleted`
- `invoice.paid`
- `invoice.payment_failed`

**Verification**: Helix uses `stripe.webhooks.constructEvent()` internally to verify the signature against the merchant's configured Stripe webhook secret.

### PayPal Ingress

**Endpoint**: `POST /webhooks/paypal`

PayPal sends events to this endpoint. Helix verifies the `PayPal-Transmission-Id` and `PayPal-Cert-Url` headers.

**Supported PayPal events**:

- `PAYMENT.CAPTURE.COMPLETED`
- `PAYMENT.CAPTURE.DENIED`
- `PAYMENT.CAPTURE.REFUNDED`
- `PAYMENT.CAPTURE.PENDING`
- `CHECKOUT.ORDER.APPROVED`
- `CHECKOUT.ORDER.COMPLETED`
- `BILLING.SUBSCRIPTION.ACTIVATED`
- `BILLING.SUBSCRIPTION.CANCELLED`
- `BILLING.SUBSCRIPTION.UPDATED`

### Square Ingress

**Endpoint**: `POST /webhooks/square`

Square sends events to this endpoint. Helix verifies the `X-Square-Signature` header.

**Supported Square events**:

- `payment.completed`
- `payment.failed`
- `refund.completed`
- `refund.failed`
- `subscription.created`
- `subscription.updated`
- `subscription.deleted`
- `dispute.created`
- `dispute.state_changed`

---

## Event Types Catalog

### Transaction Events

| Event Type | Description |
|------------|-------------|
| `transaction.created` | A new transaction has been created |
| `transaction.succeeded` | Transaction completed successfully |
| `transaction.failed` | Transaction failed |
| `transaction.refunded` | Transaction was refunded (partial or full) |
| `transaction.reversed` | Transaction was reversed/charged back |

### Subscription Events

| Event Type | Description |
|------------|-------------|
| `subscription.created` | New subscription created |
| `subscription.updated` | Subscription plan changed |
| `subscription.cancelled` | Subscription cancelled |
| `subscription.trial_ending` | Trial period ending soon (7 days) |
| `subscription.past_due` | Payment failed; subscription in past-due state |
| `subscription.renewed` | Subscription successfully renewed |

### Invoice Events

| Event Type | Description |
|------------|-------------|
| `invoice.created` | New invoice generated |
| `invoice.paid` | Invoice paid |
| `invoice.voided` | Invoice voided |
| `invoice.overdue` | Invoice past due date |

### Dispute Events

| Event Type | Description |
|------------|-------------|
| `dispute.opened` | New dispute opened |
| `dispute.evidence_submitted` | Evidence submitted for dispute |
| `dispute.resolved_won` | Dispute resolved in merchant's favor |
| `dispute.resolved_lost` | Dispute resolved against merchant |

### Payout Events

| Event Type | Description |
|------------|-------------|
| `payout.created` | Payout initiated |
| `payout.in_transit` | Payout in transit to bank |
| `payout.paid` | Payout deposited |
| `payout.failed` | Payout failed |

### Payment Method Events

| Event Type | Description |
|------------|-------------|
| `payment_method.created` | New payment method tokenized |
| `payment_method.expiring` | Card expiring soon (30 days) |
| `payment_method.deleted` | Payment method removed |

### Provider Events

| Event Type | Description |
|------------|-------------|
| `provider.healthy` | Provider health check passed |
| `provider.degraded` | Provider experiencing issues |
| `provider.unhealthy` | Provider down; failover triggered |

---

## Webhook Testing

### Test Your Endpoint

Before going live, use a tool like [webhook.site](https://webhook.site) or [Svix](https://svix.com) to inspect payloads:

```bash
curl -X POST "https://seller.hxd3v.com/api/v1/merchants/$MERCHANT_ID/webhooks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://webhook.site/YOUR_UNIQUE_ID",
    "events": ["transaction.succeeded"]
  }'
```

Then create a test transaction and observe the webhook delivery at webhook.site.

### Replay Events

If you need to replay a failed or missed webhook event, contact support with the event ID (`evt_...`) or use the dead-letter queue API (if available).

---

## Webhook Configuration Reference

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Webhook config ID |
| `merchant_id` | UUID | Merchant owner |
| `url` | string (URI) | Target endpoint URL |
| `secret` | string | Signing secret (internal, not in API response) |
| `events` | array | Event types to subscribe to (use `["*"]` for all events) |
| `is_active` | boolean | Whether webhook is active |
| `metadata` | object | Custom key-value pairs |
| `created_at` | datetime | Creation timestamp |
| `updated_at` | datetime | Last update timestamp |
