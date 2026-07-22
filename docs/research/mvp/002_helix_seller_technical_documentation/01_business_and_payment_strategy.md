# 1. Business and Payment Strategy

## Objective

Enable a solo developer resident in Serbia to sell subscription-based software globally.

The original research identifies the core constraint: direct Stripe usage is difficult without a supported-country business entity. The recommended architecture is Merchant of Record (MoR).

## Recommended Model

Primary:
- Paddle
- Lemon Squeezy

Secondary:
- Gumroad

Future:
- Serbian company + local PSP only after sufficient revenue.

## Decision

Use MoR because:
- no foreign company required
- VAT handling delegated
- invoices handled
- recurring billing handled
- chargebacks handled
- developer focuses on product

The supplied research describes MoR platforms as handling payment processing, taxation, invoicing and subscription management while exposing checkout and webhook integration. fileciteturn0file0L298-L305
