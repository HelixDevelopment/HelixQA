# 5. Webhook Security

Security requirements:

1. Verify signatures
2. Reject replay attacks
3. Store event IDs
4. Process events idempotently
5. Log every state transition

Redis pattern:

SETNX webhook_event_id TTL

If already exists:
- acknowledge
- do not execute again

This prevents duplicate subscription activation.
