# 3. Go Backend Architecture

## Stack

Backend:
- Go
- Gin Gonic
- PostgreSQL
- Redis
- Caddy
- HTTP/3 QUIC termination
- Brotli compression

## High Level Architecture

```
Mobile/Desktop/Web Client
          |
          HTTPS HTTP/3 QUIC
          |
        Caddy
          |
          |
       Gin API
          |
   ----------------
   |              |
PostgreSQL      Redis
```

## Responsibilities

Gin:
- API
- authentication
- webhook endpoints

PostgreSQL:
- users
- subscriptions
- invoices
- audit history

Redis:
- webhook idempotency
- caching
- temporary locks
