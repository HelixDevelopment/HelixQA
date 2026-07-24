# Root Admin Guide

## System Overview

As Root Admin, you have full control over the Helix Seller platform. This guide covers all administrative functions available to you.

## Dashboard

The root admin dashboard provides:

- **System Health** — Service status, uptime, error rates
- **Active Merchants** — Total and active merchant count
- **Revenue Overview** — Platform-wide revenue metrics
- **Recent Activity** — Audit log of administrative actions
- **Alerts** — System warnings and critical notifications

Access: `/admin/dashboard`

## Merchant Management

### View All Merchants

Navigate to `/admin/merchants` to see all registered merchants.

- Search by name, email, or ID
- Filter by status (active, suspended, pending)
- Sort by creation date, revenue, or subscription status

### Create Merchant

1. Click **Add Merchant**
2. Enter merchant details:
   - Company name
   - Contact email
   - Billing address
   - Tax ID (optional)
3. Select payment provider (Paddle, Lemon Squeezy)
4. Assign account admin
5. Click **Create**

### Manage Merchant

- **Suspend** — Temporarily disable merchant access
- **Reactivate** — Restore merchant access
- **Delete** — Permanently remove merchant (requires confirmation)
- **View Details** — See full merchant profile and history
- **View Subscriptions** — Manage merchant subscriptions
- **View Transactions** — Access merchant transaction history

### Merchant Configuration

- **Payment Provider** — Switch between Paddle and Lemon Squeezy
- **Webhook URLs** — Configure event notification endpoints
- **API Keys** — Generate and revoke API access
- **Billing Settings** — Configure invoicing and payment terms

## User Management

### View All Users

Navigate to `/admin/users` to manage platform users.

- Search by name or email
- Filter by role (root admin, account admin, standard user)
- Filter by status (active, inactive, locked)

### Create User

1. Click **Add User**
2. Enter user details:
   - Full name
   - Email address
   - Role assignment
3. Send invitation email
4. User sets password on first login

### Manage User

- **Edit Role** — Change user permissions
- **Deactivate** — Disable user access
- **Reactivate** — Restore user access
- **Reset Password** — Force password reset
- **View Activity** — See user audit log
- **Revoke Sessions** — Force logout from all devices

## System Configuration

### General Settings

Navigate to `/admin/settings`

- **Platform Name** — Branding configuration
- **Support Email** — Customer support contact
- **Default Currency** — Base currency for transactions
- **Timezone** — System-wide timezone setting

### Payment Provider Settings

Navigate to `/admin/settings/providers`

- **Paddle Configuration**
  - Vendor ID
  - API credentials
  - Webhook secret
  - Sandbox mode toggle

- **Lemon Squeezy Configuration**
  - Store ID
  - API key
  - Webhook secret
  - Environment toggle

### Email Configuration

- **SMTP Settings** — Outgoing email configuration
- **Email Templates** — Customize notification emails
- **Webhook Notifications** — Configure event callbacks

### Security Settings

- **Password Policy** — Minimum length, complexity requirements
- **Session Timeout** — Idle session expiration
- **MFA Requirements** — Enforce multi-factor authentication
- **API Rate Limits** — Configure rate limiting thresholds
- **IP Allowlisting** — Restrict access to specific IPs

## Monitoring and Alerts

### System Metrics

Navigate to `/admin/monitoring`

- **API Response Times** — p50, p95, p99 latencies
- **Error Rates** — 4xx and 5xx response rates
- **Database Performance** — Query times, connection pool
- **Redis Performance** — Cache hit rates, memory usage

### Alert Configuration

- **Email Alerts** — Critical system notifications
- **Webhook Alerts** — Integration with external monitoring
- **Slack/Discord** — Team notification channels

### Audit Log

Navigate to `/admin/audit`

- All administrative actions logged
- Filter by user, action type, date range
- Export audit logs for compliance

## Billing Oversight

### Platform Revenue

Navigate to `/admin/billing`

- **Monthly Recurring Revenue (MRR)** — Subscription revenue
- **Active Subscriptions** — Count by plan and status
- **Churn Rate** — Subscription cancellation rate
- **Revenue by Provider** — Breakdown by payment provider

### Provider Fees

- **Paddle Fees** — Transaction fees and processing costs
- **Lemon Squeezy Fees** — Platform fees and payouts
- **Fee Comparison** — Compare provider costs

### Payout Management

- **Payout Schedule** — Configure payout frequency
- **Payout History** — View past payouts
- **Invoice Management** — Generate and send invoices

## Best Practices

1. **Regular Audits** — Review audit logs weekly
2. **Monitor Alerts** — Respond to critical alerts promptly
3. **Backup Verification** — Verify backups are running correctly
4. **Security Reviews** — Monthly security configuration review
5. **Performance Monitoring** — Track system metrics for anomalies
6. **Documentation** — Keep configuration changes documented
