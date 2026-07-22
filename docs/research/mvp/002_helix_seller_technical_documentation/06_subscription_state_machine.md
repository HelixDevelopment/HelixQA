# 6. Subscription Lifecycle

States:

```
NONE
 |
checkout_started
 |
active
 |
+----------------+
|                |
past_due       cancelled
|                |
retry           expires
|
active
```

Rules:

Payment succeeded:
- activate
- extend expiration

Payment failed:
- mark past_due
- wait for provider retries

Cancellation:
- keep access until paid period ends
