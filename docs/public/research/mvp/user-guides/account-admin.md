# Account Admin Guide

## Account Setup

As Account Admin, you manage your organization's merchants, users, and billing. This guide covers all administrative functions for your account.

## Dashboard

Your dashboard provides:

- **Merchant Overview** — Status of your merchant accounts
- **Revenue Summary** — Your organization's revenue metrics
- **Recent Transactions** — Latest payment activity
- **Subscription Status** — Active subscriptions and renewals

Access: `/dashboard`

## Provider Configuration

### Stripe Integration

Navigate to `/settings/providers/stripe`

1. **Connect Stripe Account**
   - Click **Connect with Stripe**
   - Authorize Helix Seller in Stripe dashboard
   - Configure webhook endpoints

2. **Webhook Configuration**
   - Endpoint URL: `https://api.helix.dev/webhooks/stripe`
   - Events to subscribe:
     - `invoice.paid`
     - `invoice.payment_failed`
     - `customer.subscription.created`
     - `customer.subscription.updated`
     - `customer.subscription.deleted`

3. **Test Webhook**
   - Use Stripe CLI to send test events
   - Verify events appear in Helix dashboard

### PayPal Integration

Navigate to `/settings/providers/paypal`

1. **Connect PayPal Account**
   - Enter PayPal Client ID
   - Enter PayPal Client Secret
   - Configure webhook URL

2. **Webhook Configuration**
   - Endpoint URL: `https://api.helix.dev/webhooks/paypal`
   - Events:
     - `PAYMENT.CAPTURE.COMPLETED`
     - `PAYMENT.CAPTURE.DENIED`
     - `BILLING.SUBSCRIPTION.CREATED`
     - `BILLING.SUBSCRIPTION.ACTIVATED`
     - `BILLING.SUBSCRIPTION.CANCELLED`

### Square Integration

Navigate to `/settings/providers/square`

1. **Connect Square Account**
   - Enter Square Application ID
   - Enter Square Access Token
   - Configure webhook URL

2. **Webhook Configuration**
   - Endpoint URL: `https://api.helix.dev/webhooks/square`
   - Events:
     - `payment.completed`
     - `payment.failed`
     - `subscription.created`
     - `subscription.updated`
     - `subscription.cancelled`

## User Invitation and Management

### Invite User

1. Navigate to `/settings/users`
2. Click **Invite User**
3. Enter email address
4. Select role:
   - **Account Admin** — Full account management
   - **Standard User** — View and limited actions
5. Click **Send Invitation**

### Manage Users

- **View Users** — See all account users
- **Edit Role** — Change user permissions
- **Deactivate** — Remove user access
- **Resend Invitation** — Send new invitation email
- **View Activity** — See user action history

### Role Permissions

| Action | Account Admin | Standard User |
|--------|---------------|---------------|
| View Dashboard | ✅ | ✅ |
| View Transactions | ✅ | ✅ |
| Manage Customers | ✅ | ✅ |
| Manage API Keys | ✅ | ✅ |
| Manage Webhooks | ✅ | ❌ |
| Manage Users | ✅ | ❌ |
| Manage Billing | ✅ | ❌ |
| Configure Providers | ✅ | ❌ |

## Transaction Monitoring

### View Transactions

Navigate to `/transactions`

- **Filter by Status** — Completed, pending, failed, refunded
- **Filter by Date** — Custom date range
- **Filter by Amount** — Min/max amount
- **Search** — By transaction ID, customer email

### Transaction Details

Click any transaction to view:

- Transaction ID
- Amount and currency
- Status and timestamps
- Customer information
- Payment provider details
- Webhook events received

### Export Transactions

1. Set filters as needed
2. Click **Export**
3. Select format (CSV, JSON)
4. Download file

## Webhook Configuration

### Manage Webhooks

Navigate to `/settings/webhooks`

1. **Add Webhook**
   - Enter endpoint URL
   - Select events to subscribe
   - Add secret for signature verification
   - Save webhook

2. **Test Webhook**
   - Click **Test** on any webhook
   - Send test payload
   - Verify endpoint receives event

3. **View Webhook Logs**
   - See all webhook attempts
   - View request/response details
   - Check delivery status
   - Retry failed webhooks

### Webhook Events

Available events:

- `subscription.created` — New subscription
- `subscription.updated` — Subscription changed
- `subscription.cancelled` — Subscription cancelled
- `subscription.payment.succeeded` — Payment successful
- `subscription.payment.failed` — Payment failed
- `invoice.created` — Invoice generated
- `invoice.paid` — Invoice paid
- `customer.created` — New customer

## Billing and Invoicing

### View Billing

Navigate to `/billing`

- **Current Plan** — Your subscription plan
- **Usage** — Current period usage
- **Payment Method** — Saved payment methods
- **Invoice History** — Past invoices

### Manage Payment Methods

1. Navigate to `/billing/payment-methods`
2. **Add Payment Method** — Credit card or bank account
3. **Set Default** — Choose primary payment method
4. **Remove** — Delete old payment methods

### Invoice Management

- **View Invoices** — See all past invoices
- **Download PDF** — Get invoice PDF
- **Pay Invoice** — Pay outstanding invoices
- **Update Billing Info** — Change billing address/name

## Best Practices

1. **Regular Reviews** — Check transactions weekly
2. **Webhook Monitoring** — Ensure webhooks are delivering
3. **User Audits** — Review user access monthly
4. **Backup Contacts** — Keep admin contact info updated
5. **Provider Updates** — Keep API credentials secure and rotated
6. **Documentation** — Document configuration changes
