# Maintenance Procedures

Scheduled and routine maintenance procedures for the Helix Seller platform.

---

## Scheduled Maintenance Windows

| Window | Schedule | Duration | Impact |
|--------|----------|----------|--------|
| Database maintenance | Weekly, Sunday 02:00-04:00 UTC | ~2 hours | Read-only possible |
| Dependency updates | Bi-weekly, Wednesday 10:00-12:00 UTC | ~2 hours | None (staging) |
| Security patches | As needed (within 48h of disclosure) | ~1 hour | Brief restart |
| Certificate renewal | 30 days before expiry | ~5 minutes | None (zero-downtime) |
| Log rotation | Daily, 00:00 UTC | ~5 minutes | None |

### Pre-Maintenance Checklist

- [ ] Notify team via Slack #ops-maintenance
- [ ] Verify backup exists (within last 24 hours)
- [ ] Confirm rollback procedure is ready
- [ ] Schedule during lowest traffic period
- [ ] Have on-call engineer available

---

## Database Maintenance

### VACUUM

Reclaim storage and update row visibility information.

```bash
# Full vacuum (requires exclusive lock — use during maintenance window)
psql -U helix -d helix_seller -c "VACUUM FULL VERBOSE;"

# Autovacuum status check
psql -U helix -d helix_seller -c "
  SELECT schemaname, relname, last_vacuum, last_autovacuum,
         n_dead_tup, n_live_tup,
         ROUND(n_dead_tup::numeric / NULLIF(n_live_tup, 0) * 100, 2) AS dead_pct
  FROM pg_stat_user_tables
  WHERE n_dead_tup > 1000
  ORDER BY n_dead_tup DESC;
"
```

### ANALYZE

Update table statistics for query planner.

```bash
# Analyze all tables
psql -U helix -d helix_seller -c "ANALYZE VERBOSE;"

# Analyze specific table
psql -U helix -d helix_seller -c "ANALYZE VERBOSE orders;"
```

### REINDEX

Rebuild indexes to reclaim space and improve performance.

```bash
# Reindex all indexes (blocks writes — maintenance window only)
psql -U helix -d helix_seller -c "REINDEX DATABASE VERBOSE helix_seller;"

# Reindex specific table (less disruptive)
psql -U helix -d helix_seller -c "REINDEX TABLE VERBOSE orders;"

# Check index bloat
psql -U helix -d helix_seller -c "
  SELECT indexrelname, idx_scan, pg_size_pretty(pg_relation_size(indexrelid))
  FROM pg_stat_user_indexes
  ORDER BY pg_relation_size(indexrelid) DESC
  LIMIT 10;
"
```

### Weekly Maintenance Script

```bash
#!/bin/bash
# /opt/helix-seller/scripts/db-maintenance.sh
# Run via cron: 0 2 * * 0 /opt/helix-seller/scripts/db-maintenance.sh

set -euo pipefail

DB="helix_seller"
DB_USER="helix"
LOG="/var/log/helix-seller/db-maintenance.log"

echo "=== DB Maintenance started at $(date -u) ===" >> "$LOG"

psql -U "$DB_USER" -d "$DB" -c "VACUUM (VERBOSE, ANALYZE);" >> "$LOG" 2>&1

echo "=== VACUUM ANALYZE complete ===" >> "$LOG"

psql -U "$DB_USER" -d "$DB" -c "
  SELECT schemaname, relname, n_dead_tup, n_live_tup
  FROM pg_stat_user_tables
  WHERE n_dead_tup > 10000
  ORDER BY n_dead_tup DESC;
" >> "$LOG" 2>&1

echo "=== Maintenance finished at $(date -u) ===" >> "$LOG"
```

### Partition Management (if applicable)

```bash
# Check partition sizes
psql -U helix -d helix_seller -c "
  SELECT schemaname, tablename,
         pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename))
  FROM pg_tables
  WHERE tablename LIKE 'orders_%'
  ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
"

# Drop old partitions (if retention policy requires)
psql -U helix -d helix_seller -c "
  -- Example: drop partitions older than 2 years
  -- Adjust date as needed
  DROP TABLE IF EXISTS orders_2024_01;
"
```

---

## Log Rotation

### Application Logs

Configure logrotate for the helix-seller service:

```bash
cat > /etc/logrotate.d/helix-seller << 'EOF'
/var/log/helix-seller/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    create 0640 helix helix
    postrotate
        systemctl reload helix-seller.service > /dev/null 2>&1 || true
    endscript
}
EOF
```

### PostgreSQL Logs

```bash
# Check current log settings
psql -U helix -d helix_seller -c "SHOW log_directory;"
psql -U helix -d helix_seller -c "SHOW log_filename;"
psql -U helix -d helix_seller -c "SHOW log_rotation_age;"
psql -U helix -d helix_seller -c "SHOW log_rotation_size;"
```

### Log Cleanup

```bash
# Manual cleanup
sudo find /var/log/helix-seller -name "*.log" -mtime +30 -delete

# Check disk usage
du -sh /var/log/helix-seller/
```

---

## Certificate Renewal

### Let's Encrypt (certbot)

```bash
# Check certificate expiry
sudo certbot certificates

# Manual renewal
sudo certbot renew --dry-run

# Actual renewal
sudo certbot renew

# Verify new certificate
sudo openssl x509 -in /etc/letsencrypt/live/api.helixseller.com/fullchain.pem \
  -noout -dates
```

### Auto-Renewal Setup

```bash
# certbot installs a systemd timer by default
systemctl list-timers | grep certbot

# Or add to crontab (if timer not available)
echo "0 3 * * * certbot renew --quiet --post-hook 'systemctl reload nginx'" | \
  sudo crontab -
```

### Custom Certificates

```bash
# Generate new key pair
openssl genrsa -out keys/tls_private.pem 2048
openssl rsa -in keys/tls_private.pem -pubout -out keys/tls_public.pem

# Generate CSR
openssl req -new -key keys/tls_private.pem \
  -out keys/csr.pem \
  -subj "/C=US/ST=State/L=City/O=Helix/CN=api.helixseller.com"

# After receiving signed cert
openssl x509 -in signed_cert.pem -out keys/tls_cert.pem
chmod 600 keys/tls_private.pem
```

---

## Dependency Updates

### Go Dependencies

```bash
# Check for outdated dependencies
go list -m -u all

# Update all dependencies
go get -u ./...
go mod tidy

# Verify no breaking changes
make test

# Check for security vulnerabilities
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

### Node.js Dependencies (if applicable)

```bash
# Check outdated
npm outdated

# Update
npm update

# Check for vulnerabilities
npm audit
npm audit fix
```

### Container Image Updates

```bash
# Pull latest images
podman pull postgres:16-alpine
podman pull redis:7-alpine

# Rebuild application container
podman-compose build --no-cache

# Update and restart
podman-compose up -d

# Clean old images
podman image prune -f
```

---

## Security Patches

### Process

1. Monitor security advisories:
   - Go: `https://groups.google.com/g/golang-announce`
   - PostgreSQL: `https://www.postgresql.org/support/security/`
   - Debian/Ubuntu: `apt list --upgradable`

2. Assess impact:
   ```bash
   # Check if vulnerable dependency is in use
   govulncheck ./...
   ```

3. Apply patch:
   ```bash
   # System packages
   sudo apt update && sudo apt upgrade -y

   # Go dependencies
   go get -u <vulnerable-package>
   go mod tidy
   ```

4. Verify:
   ```bash
   make test
   govulncheck ./...
   ```

5. Deploy:
   ```bash
   make build
   sudo systemctl restart helix-seller.service
   curl -s http://localhost:8080/health | jq .
   ```

### Emergency Patch Procedure

For critical vulnerabilities requiring immediate action:

```bash
# 1. Pull latest changes
git pull origin main

# 2. Build
make build

# 3. Restart (rolling if using multiple instances)
sudo systemctl restart helix-seller.service

# 4. Verify
curl -s http://localhost:8080/health | jq .

# 5. Monitor logs
sudo journalctl -u helix-seller -f --lines=20
```

---

## Maintenance Checklist (Copy-Paste)

```
MAINTENANCE WINDOW: YYYY-MM-DD HH:MM - HH:MM UTC
TYPE: [Database | Dependency | Security | Certificate]
CHANGE: [Description]

Pre-flight:
- [ ] Team notified
- [ ] Backup verified
- [ ] Rollback ready
- [ ] On-call available

Execution:
- [ ] Changes applied
- [ ] Tests passing
- [ ] Health checks green
- [ ] Monitoring normal

Post-flight:
- [ ] Documentation updated
- [ ] Team notified of completion
- [ ] Next maintenance scheduled
```
