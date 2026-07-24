# Helix Seller — User Guides

## Overview

Helix Seller supports three user roles with distinct permissions and capabilities:

| Role | Description | Access Level |
|------|-------------|--------------|
| [Root Admin](./root-admin.md) | System owner, full control | All features, system config, billing |
| [Account Admin](./account-admin.md) | Organization administrator | Merchant management, user management, billing |
| [Standard User](./standard-user.md) | Regular user | View transactions, manage customers, API keys |

## Quick Start

1. **Root Admin** — Access system dashboard at `/admin`
2. **Account Admin** — Access organization dashboard at `/dashboard`
3. **Standard User** — Access personal dashboard at `/home`

## Authentication

All users authenticate via:
- Email + password
- Multi-factor authentication (MFA) — recommended for all users
- SSO (planned for future release)

## Navigation

- **Dashboard** — Overview of key metrics
- **Merchants** — Manage merchant accounts (Admin+)
- **Transactions** — View and manage transactions
- **Customers** — Customer management
- **Settings** — Account and system configuration
- **API Keys** — Manage programmatic access
- **Webhooks** — Configure event notifications
