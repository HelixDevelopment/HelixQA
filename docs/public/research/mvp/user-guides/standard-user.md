# Standard User Guide

## Login and Authentication

### First Time Login

1. Receive invitation email from your account admin
2. Click the **Set Password** link
3. Create a strong password (minimum 12 characters)
4. Log in with your email and new password

### Regular Login

1. Navigate to `https://app.helix.dev/login`
2. Enter your email address
3. Enter your password
4. Complete MFA if enabled
5. Access your dashboard

### Password Requirements

- Minimum 12 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one number
- At least one special character
- Cannot reuse last 5 passwords

## MFA Setup

### Enable MFA

1. Navigate to `/settings/security`
2. Click **Enable MFA**
3. Scan QR code with authenticator app (Google Authenticator, Authy, etc.)
4. Enter the 6-digit code from your app
5. Save backup codes in a secure location

### Using MFA

1. After entering password, you'll be prompted for MFA code
2. Open your authenticator app
3. Enter the current 6-digit code
4. Click **Verify**

### Backup Codes

- Generated when MFA is enabled
- Store in a secure location (password manager recommended)
- Each code can be used once
- Generate new codes if compromised

### Recovery

If you lose access to your authenticator app:

1. Click **Lost your device?** on login
2. Enter a backup code
3. Access your account
4. Reconfigure MFA with new device

## Viewing Transactions

### Transaction List

Navigate to `/transactions`

- **Default View** — Recent transactions (last 30 days)
- **Filter Options**:
  - Status: All, Completed, Pending, Failed
  - Date Range: Custom start/end dates
  - Amount: Min/max values
- **Search** — By transaction ID or customer email

### Transaction Details

Click any transaction to view:

- **Transaction ID** — Unique identifier
- **Amount** — Transaction amount and currency
- **Status** — Current status
- **Date** — Transaction timestamp
- **Customer** — Customer email and details
- **Payment Method** — How payment was made
- **Provider** — Payment provider used

### Export Transactions

1. Apply desired filters
2. Click **Export**
3. Choose format:
   - **CSV** — For spreadsheets
   - **JSON** — For developers
4. Download file

## Managing Customers

### Customer List

Navigate to `/customers`

- **View All Customers** — See all customers
- **Search** — By name or email
- **Filter** — By status, subscription plan

### Customer Details

Click any customer to view:

- **Contact Information** — Email, name, company
- **Subscription Status** — Current plan and status
- **Transaction History** — All payments
- **Notes** — Internal notes about customer

### Add Customer

1. Click **Add Customer**
2. Enter details:
   - Email address (required)
   - Full name
   - Company name
   - Phone number
3. Click **Save**

### Edit Customer

1. Navigate to customer details
2. Click **Edit**
3. Update information
4. Click **Save**

### Customer Actions

- **View Subscription** — See subscription details
- **View Transactions** — See payment history
- **Add Note** — Add internal note
- **Export Data** — Download customer data

## API Key Management

### View API Keys

Navigate to `/settings/api-keys`

- **Active Keys** — Currently valid keys
- **Last Used** — When each key was last used
- **Permissions** — What each key can access

### Create API Key

1. Click **Generate New Key**
2. Enter description (e.g., "Production API Key")
3. Select permissions:
   - **Read Only** — View data only
   - **Read/Write** — View and modify data
   - **Admin** — Full access
4. Click **Generate**
5. **Copy and save** the key (shown only once)

### Revoke API Key

1. Find the key in the list
2. Click **Revoke**
3. Confirm revocation
4. Key immediately stops working

### API Key Security

- **Never share** API keys publicly
- **Use environment variables** to store keys
- **Rotate regularly** — Generate new keys periodically
- **Monitor usage** — Check for unusual activity
- **Revoke unused** — Remove keys no longer in use

## Settings

### Profile Settings

Navigate to `/settings/profile`

- **Display Name** — Your display name
- **Email** — Contact email (requires verification)
- **Timezone** — Your local timezone
- **Language** — Interface language

### Notification Settings

Navigate to `/settings/notifications`

- **Email Notifications** — Toggle email alerts
- **Transaction Alerts** — Notify on payments
- **Subscription Alerts** — Notify on subscription changes
- **Security Alerts** — Notify on login attempts

### Security Settings

Navigate to `/settings/security`

- **Change Password** — Update your password
- **Enable MFA** — Set up multi-factor authentication
- **Active Sessions** — View and revoke logged-in devices
- **Login History** — See recent login attempts

## Best Practices

1. **Enable MFA** — Always use multi-factor authentication
2. **Strong Passwords** — Use unique, complex passwords
3. **Regular Reviews** — Check transactions and customers weekly
4. **API Key Hygiene** — Rotate keys regularly, revoke unused ones
5. **Secure Storage** — Use a password manager for credentials
6. **Monitor Activity** — Review login history periodically
7. **Keep Updated** — Stay informed about security practices
