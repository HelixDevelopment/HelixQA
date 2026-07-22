# 2. Payment Provider Deep Dive

## Paddle

Flow:

Customer
→ Paddle Checkout
→ Payment
→ Paddle tax engine
→ Paddle webhook
→ Helix backend
→ Subscription activation

Required backend events:
- subscription_created
- subscription_updated
- subscription_cancelled
- subscription_payment_succeeded
- subscription_payment_failed

## Lemon Squeezy

Flow:

Customer
→ Lemon checkout
→ Payment
→ Webhook
→ Backend subscription service

Required components:
- checkout creation
- webhook verification
- subscription state machine

## Revenue Example

Example: 100 EUR annual subscription.

Approximate research model:
- EU buyer with VAT: ~93.54 EUR after MoR fee and VAT handling
- non-EU buyer: ~94.54 EUR

Exact fees depend on provider pricing and payout method.
