# Incident Response Playbook

Procedures for identifying, triaging, and resolving incidents on the Helix Seller platform.

---

## Severity Levels

| Level | Name | Description | Response Time | Update Frequency |
|-------|------|-------------|---------------|------------------|
| **P0** | Critical | Complete platform outage or data loss | Immediate | Every 15 min |
| **P1** | High | Major feature broken, payment processing affected | Within 15 min | Every 30 min |
| **P2** | Medium | Degraded performance, non-critical feature broken | Within 1 hour | Every 2 hours |
| **P3** | Low | Minor issue, cosmetic bug, or improvement needed | Within 4 hours | Daily |

### Severity Classification Examples

**P0:**
- Database unreachable
- Payment processing completely down
- Data breach detected
- All API endpoints returning 500

**P1:**
- Payment provider (Stripe/PayPal) intermittently failing
- Order creation failing for subset of users
- Authentication broken for all users
- Redis/NATS down causing degraded functionality

**P2:**
- API response times >2s consistently
- Single payment provider failing (others working)
- Background job queue backing up
- Webhook delivery failures

**P3:**
- UI rendering issues
- Non-critical log errors
- Documentation outdated
- Performance optimization opportunities

---

## On-Call Rotation

### Schedule

On-call is managed weekly. Primary on-call engineer has full responsibility for incident response.

| Week | Primary | Secondary |
|------|---------|-----------|
| Week 1 | Engineer A | Engineer B |
| Week 2 | Engineer B | Engineer C |
| Week 3 | Engineer C | Engineer A |

### Responsibilities

- Respond to all alerts within SLA
- Lead incident communication
- Make escalation decisions
- Author post-incident review

### Handoff

At rotation change:
1. Review open incidents
2. Share context on ongoing issues
3. Verify monitoring/alerting is functional

---

## Escalation Matrix

```
On-Call Engineer
    │
    ├── Cannot resolve within 30 min ──> Senior Engineer
    │
    ├── Data breach / security ──> Security Lead (immediate)
    │
    ├── Customer-facing outage >1h ──> Engineering Manager
    │
    ├── Data loss confirmed ──> CTO + Engineering Manager
    │
    └── Legal/compliance impact ──> Legal + CTO
```

### Contact Information

| Role | Name | Phone | Slack |
|------|------|-------|-------|
| On-Call | (see rotation) | — | @oncall |
| Senior Engineer | — | — | @senior-eng |
| Engineering Manager | — | — | @eng-manager |
| Security Lead | — | — | @security |
| CTO | — | — | @cto |

---

## Communication Templates

### Internal: Incident Declared

```
🚨 INCIDENT DECLARED — P{N}
Title: {Short description}
Impact: {Who is affected, what's broken}
On-Call: {Name}
Status: Investigating
Channel: #incidents-{date}
```

### Internal: Status Update

```
📊 UPDATE — P{N} — {Time}
Status: {Investigating|Identified|Fixing|Monitoring}
What we know: {Current understanding}
What we're doing: {Actions in progress}
Next update: {Time}
```

### Internal: Incident Resolved

```
✅ RESOLVED — P{N}
Duration: {Total time}
Impact: {Summary of affected users/features}
Root cause: {Brief description}
Resolution: {What fixed it}
Post-incident review: Scheduled for {Date}
```

### External: Customer-Facing (Status Page)

```
[Investigating] We are investigating reports of {issue}.
Our team is actively working to identify the cause.
Next update in 30 minutes.

[Identified] We've identified the cause of {issue}.
A fix is being deployed. Some users may continue
to experience {symptom} until the fix propagates.

[Monitoring] A fix has been deployed and we are
monitoring for stability. {Feature} should now
be working normally.

[Resolved] The issue has been fully resolved.
We apologize for the inconvenience. A detailed
post-incident summary will be published within 48 hours.
```

---

## Provider Outage Response

When a payment provider (Stripe, PayPal, Square) experiences an outage.

### Detection

```bash
# Check Stripe status
curl -s https://status.stripe.com/api/v2/status.json | jq .

# Check PayPal status
curl -s https://www.paypal.com/status/api/status | jq .

# Check provider API health
curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer ${STRIPE_API_KEY}" \
  https://api.stripe.com/v1/balance
```

### Response Steps

1. **Verify** the outage is provider-side (not our config/network)
2. **Check** provider status pages for known incidents
3. **Enable** fallback provider if available:
   - Stripe down → Enable PayPal/Square as primary
   - PayPal down → Enable Stripe/Square as primary
   - Square down → Enable Stripe/PayPal as primary
4. **Notify** customer support with expected timeline
5. **Monitor** provider status page for resolution
6. **Revert** primary provider routing once stable

### Provider Fallback Configuration

```bash
# In .env, swap provider priority:
# Primary: STRIPE_API_KEY (normal)
# Fallback: Enable PAYPAL as primary by setting
#   PAYPAL_PREFERRED=true

# Restart application
sudo systemctl restart helix-seller.service
```

---

## Database Failure Response

### Detection

```bash
# Test connection
psql -U helix -d helix_seller -c "SELECT 1" 2>&1

# Check PostgreSQL status
sudo systemctl status postgresql

# Check disk space
df -h /var/lib/postgresql

# Check for lock contention
psql -U helix -d helix_seller -c "
  SELECT pid, state, query, wait_event_type, wait_event
  FROM pg_stat_activity
  WHERE state != 'idle'
  ORDER BY query_start;
"
```

### Response Steps

1. **Confirm** the failure (not just a transient connection issue)
2. **Check** PostgreSQL service status
3. **Check** disk space, memory, and CPU
4. **Review** PostgreSQL logs:
   ```bash
   sudo tail -100 /var/log/postgresql/postgresql-16-main.log
   ```
5. **If service is down**, restart:
   ```bash
   sudo systemctl restart postgresql
   ```
6. **If data is corrupted**, follow RESTORE_RUNBOOK.md
7. **If disk is full**, free space:
   ```bash
   # Clean WAL files (if in recovery)
   sudo journalctl --vacuum-size=100M

   # Move old backups to external storage
   mv /backups/helix_seller/daily/*.dump /mnt/external/backups/
   ```

---

## Security Breach Response

### Immediate Actions (First 15 Minutes)

1. **DO NOT** shut down affected systems (preserve evidence)
2. **Contain** the breach:
   ```bash
   # Revoke compromised credentials immediately
   # Rotate JWT keys
   openssl genrsa -out keys/jwt_private.pem 2048
   openssl rsa -in keys/jwt_private.pem -pubout -out keys/jwt_public.pem

   # Rotate encryption key (if compromised)
   # Generate new ENCRYPTION_KEY
   openssl rand -hex 32

   # Invalidate all active sessions
   redis-cli FLUSHALL
   ```
3. **Document** everything: timestamps, actions, observations
4. **Notify** Security Lead immediately

### Investigation Steps

```bash
# Check for unauthorized access in logs
journalctl -u helix-seller --since "24 hours ago" | \
  grep -E "(401|403|login|auth)" | tail -50

# Check for unusual database queries
psql -U helix -d helix_seller -c "
  SELECT pid, usename, application_name, client_addr,
         query, query_start
  FROM pg_stat_activity
  WHERE query NOT LIKE '%pg_stat_activity%'
  ORDER BY query_start DESC
  LIMIT 20;
"

# Check for unexpected network connections
ss -tlnp | grep -E "(5432|6379|4222|8080)"

# Check for suspicious processes
ps aux | grep -v grep | grep -E "(curl|wget|nc|ncat)"
```

### Containment Checklist

- [ ] Compromised credentials rotated
- [ ] Affected systems isolated (if needed)
- [ ] Access logs preserved
- [ ] Database audit logs reviewed
- [ ] All API keys regenerated
- [ ] Webhook secrets rotated
- [ ] Customer notification prepared (if required)
- [ ] Legal/compliance notified (if required)
- [ ] Law enforcement notified (if required)

### Evidence Preservation

```bash
# Create forensic snapshot
sudo tar -czf /evidence/incident_$(date +%Y%m%d_%H%M%S).tar.gz \
  /var/log/postgresql/ \
  /var/log/helix-seller/ \
  /var/lib/postgresql/data/pg_log/

# Copy database audit logs
psql -U helix -d helix_seller -c "
  COPY (SELECT * FROM audit_log WHERE created_at > now() - interval '24 hours')
  TO '/evidence/audit_log_export.csv' CSV HEADER;
"
```

---

## Post-Incident Review Process

### Timeline

| Activity | Deadline |
|----------|----------|
| Incident ticket created | During incident |
| Draft post-incident review | Within 48 hours |
| Review meeting scheduled | Within 1 week |
| Action items assigned | During review meeting |
| Action items completed | Within 2 weeks |

### Post-Incident Review Template

```markdown
# Post-Incident Review: {INCIDENT TITLE}

## Summary
- **Date:** YYYY-MM-DD
- **Duration:** X hours Y minutes
- **Severity:** P{N}
- **Impact:** {Description of affected users/features}

## Timeline (UTC)
- HH:MM — {Event}
- HH:MM — {Event}
- HH:MM — {Event}

## Root Cause
{Detailed description of what caused the incident}

## What Went Well
- {Thing 1}
- {Thing 2}

## What Went Wrong
- {Thing 1}
- {Thing 2}

## Action Items
| # | Action | Owner | Priority | Due | Status |
|---|--------|-------|----------|-----|--------|
| 1 | {Action} | {Who} | {P0-P3} | {Date} | Open |
| 2 | {Action} | {Who} | {P0-P3} | {Date} | Open |

## Lessons Learned
{Key takeaways for the team}
```

### Review Meeting Agenda

1. Incident walkthrough (5 min)
2. Timeline review (10 min)
3. Root cause analysis (15 min)
4. Action items discussion (15 min)
5. Process improvement suggestions (10 min)
6. Assign action item owners (5 min)
