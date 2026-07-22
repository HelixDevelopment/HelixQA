# 7. Deployment Plan

## Development

Docker Compose:
- postgres
- redis
- backend
- caddy

## Production

Recommended:

- VPS
- Caddy reverse proxy
- automatic TLS
- HTTP/3 enabled
- backups
- monitoring

## CI/CD

Pipeline:

commit
→ tests
→ build container
→ security scan
→ deploy
→ health check
