# 4. Database Design

## Users

Fields:
- id UUID
- email
- created_at

## Subscription

Fields:
- id
- user_id
- provider
- external_subscription_id
- status
- plan_id
- current_period_end

Statuses:

active
past_due
cancelled
expired

## Important Rules

Never trust frontend payment success pages.

Only verified provider webhooks can activate access.
