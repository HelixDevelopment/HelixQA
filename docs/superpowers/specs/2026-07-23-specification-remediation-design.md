# Specification Remediation Design

**Date**: 2026-07-23  
**Feature**: Helix Seller Platform  
**Trigger**: Deep analysis (speckit-analyze) identified 3 CRITICAL + 4 HIGH + 6 MEDIUM findings  
**Goal**: Fix all gaps before Superpowers implementation handoff  
**Scope**: 6 files modified, ~695 lines added/changed, 274 tasks, 18 phases

---

## Decisions

| Issue | Decision | Rationale |
|-------|----------|-----------|
| F1: JWT RS256 vs HS256 | **Keep RS256** per constitution §7.2 | Multi-service verification rationale; standard for microservices |
| F2: Auth endpoints missing | **Full auth API in contract** | SDK generation requires complete contract |
| F3: User entity missing | **Add User + ApiKey to data-model.md** | AuditLog FK is dangling; tasks reference nonexistent entity |
| F4: Observability missing | **New Phase 16 (OTel + Prometheus + Grafana)** | Constitution §8.2 mandates; cannot defer |
| F5: Backup missing | **New Phase 17 (pg_dump + WAL archiving)** | Constitution §8.3 mandates RPO ~1h |
| F6: Duplicate tasks | **Remove T134, keep T053** | T053 is earlier in chain; T134 is too late for foundational pkg |
| F7: WebSocket/load testing | **New Phase 18** | Constitution §3.1 + SC-004/SC-005 |
| SonarQube gap | **Add to Phase 18** | Constitution §4.1 mandates static analysis |
| PDF/HTML pipeline | **Add to Phase 18** | Constitution §4.2 mandates sync |
| CRUD gaps in API | **Add missing endpoints** | Tasks reference nonexistent endpoints |

---

## Files Modified

| File | Groups | Change Summary |
|------|--------|----------------|
| `internal/config/config.go` | A | RS256 fields: JWTPrivateKeyPath, JWTPublicKeyPath, JWTAccessExpiry, JWTRefreshExpiry |
| `.env.example` | A | RS256 env vars + key generation instructions |
| `specs/.../contracts/api-v1.yaml` | B, E | Auth paths (7), User paths (2), ApiKey paths (3), ExchangeRate path (1), CRUD gaps (~8 endpoints) |
| `specs/.../data-model.md` | C | User entity, ApiKey entity |
| `specs/.../tasks.md` | A3, D, F, G, H, I | Remove T134, update T053/T069E/T003, add Phases 16-18, update execution order |
| `specs/.../plan.md` | J | Add configs/ and docs/operations/ to project structure |

---

## Group A: JWT RS256 Config Fix

### A1: `internal/config/config.go`

**Struct** — replace lines 16-17:
```go
// REMOVE:
JWTSecret      string
JWTExpiry      time.Duration

// ADD:
JWTPrivateKeyPath string
JWTPublicKeyPath  string
JWTAccessExpiry   time.Duration
JWTRefreshExpiry  time.Duration
```

**Load()** — replace lines 48-49:
```go
// REMOVE:
JWTSecret:       getEnv("JWT_SECRET", "change-me-in-production"),
JWTExpiry:       getEnvAsDuration("JWT_EXPIRY", 24*time.Hour),

// ADD:
JWTPrivateKeyPath: getEnv("JWT_PRIVATE_KEY_PATH", "keys/jwt_private.pem"),
JWTPublicKeyPath:  getEnv("JWT_PUBLIC_KEY_PATH", "keys/jwt_public.pem"),
JWTAccessExpiry:   getEnvAsDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
JWTRefreshExpiry:  getEnvAsDuration("JWT_REFRESH_EXPIRY", 168*time.Hour),
```

### A2: `.env.example`

Replace lines 17-19:
```ini
# REMOVE:
JWT_SECRET=change-me-in-production
JWT_EXPIRY=24h

# ADD:
# JWT (RS256 — generate keys:
#   openssl genrsa -out keys/jwt_private.pem 2048
#   openssl rsa -in keys/jwt_private.pem -pubout -out keys/jwt_public.pem
#   chmod 600 keys/jwt_private.pem)
JWT_PRIVATE_KEY_PATH=keys/jwt_private.pem
JWT_PUBLIC_KEY_PATH=keys/jwt_public.pem
JWT_ACCESS_EXPIRY=15m
JWT_REFRESH_EXPIRY=168h
```

### A3: `tasks.md` — T003, T059, T069E

**T003** (line 39): Update content list:
```
Content: DATABASE_URL, REDIS_URL, SERVER_PORT, JWT_PRIVATE_KEY_PATH, JWT_PUBLIC_KEY_PATH, JWT_ACCESS_EXPIRY, JWT_REFRESH_EXPIRY, LOG_LEVEL, STRIPE_API_KEY, PAYPAL_CLIENT_ID, PAYPAL_SECRET, SQUARE_ACCESS_TOKEN, NATS_URL, ENCRYPTION_KEY
```

**T059** (line 287): Update context files:
```
Context Files: internal/config/config.go (JWTPublicKeyPath, JWTAccessExpiry), internal/model/errors.go
```

**T069E** (lines 383-387): Update description + context files:
```
- [ ] T069E [P] Create `internal/service/jwt.go` — JWT RS256 access/refresh token generation, validation, refresh flow (access 15 min, refresh 7 d)
  - **Context Files**: `internal/config/config.go` (JWTPrivateKeyPath, JWTPublicKeyPath, JWTAccessExpiry, JWTRefreshExpiry), `internal/model/user.go`
  - **Creates**: `internal/service/jwt.go`
  - **Dependencies**: T069A
  - **External**: `github.com/golang-jwt/jwt/v5`
```

---

## Group B: Auth Endpoints in API Contract

Add to `api-v1.yaml` before the `/health` path.

### Auth Paths

| Path | Method | Operation | Tags |
|------|--------|-----------|------|
| `/auth/register` | POST | register | Auth |
| `/auth/login` | POST | login | Auth |
| `/auth/refresh` | POST | refreshToken | Auth |
| `/auth/logout` | POST | logout | Auth |
| `/auth/mfa/setup` | POST | setupMFA | Auth |
| `/auth/mfa/verify` | POST | verifyMFA | Auth |

### User Paths

| Path | Method | Operation | Tags |
|------|--------|-----------|------|
| `/users/{userId}` | GET | getUser | Users |
| `/users/{userId}` | PUT | updateUser | Users |

### API Key Paths

| Path | Method | Operation | Tags |
|------|--------|-----------|------|
| `/api-keys` | POST | createApiKey | API Keys |
| `/api-keys` | GET | listApiKeys | API Keys |
| `/api-keys/{apiKeyId}` | DELETE | revokeApiKey | API Keys |

### Auth Schemas (add to `components/schemas`)

- `LoginRequest` — email + password
- `RegisterRequest` — email + password + name + company_name
- `TokenPair` — access_token + refresh_token + token_type + expires_in
- `RefreshRequest` — refresh_token
- `MFASetupResponse` — secret + qr_code_url + recovery_codes
- `MFAVerifyRequest` — code (6 digits)

### API Key Schemas

- `ApiKey` — id, name, key_prefix, scopes, rate_limit, is_active, last_used_at, expires_at, created_at
- `CreateApiKeyRequest` — name, scopes, rate_limit, expires_at
- `ApiKeyCreatedResponse` — id, name, key (shown once), key_prefix, created_at

---

## Group C: Data Model — User + ApiKey Entities

Insert before AuditLog section in `data-model.md` (before line 276).

### User Entity

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| email | VARCHAR(255) | UNIQUE, NOT NULL | Login email |
| password_hash | VARCHAR(255) | NOT NULL | Argon2id hash |
| name | VARCHAR(255) | NOT NULL | Display name |
| role | ENUM | NOT NULL | root_admin, account_admin, user |
| merchant_id | UUID | FK → Merchant, NULL | Associated merchant (NULL for root_admin) |
| is_active | BOOLEAN | NOT NULL, DEFAULT true | Account enabled |
| mfa_enabled | BOOLEAN | NOT NULL, DEFAULT false | MFA activated |
| mfa_secret | VARCHAR(64) | NULL | TOTP secret (encrypted) |
| mfa_recovery_codes | JSONB | NULL | Hashed recovery codes |
| last_login_at | TIMESTAMPTZ | NULL | Last successful login |
| last_login_ip | INET | NULL | Last login IP |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL | Last update timestamp |

**Indexes**: email (unique), merchant_id, role  
**Note**: Root admin (role = root_admin) has merchant_id = NULL, single row.

### ApiKey Entity

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK | Unique identifier |
| merchant_id | UUID | FK → Merchant, NOT NULL | Owning merchant |
| user_id | UUID | FK → User, NOT NULL | Created by user |
| name | VARCHAR(255) | NOT NULL | Human-readable name |
| key_hash | VARCHAR(64) | UNIQUE, NOT NULL | SHA-256 hash of full key |
| key_prefix | VARCHAR(8) | NOT NULL | First 8 chars for identification |
| scopes | JSONB | NOT NULL, DEFAULT '[]' | Allowed operations |
| rate_limit | INTEGER | NOT NULL, DEFAULT 0 | Requests per second (0 = unlimited) |
| is_active | BOOLEAN | NOT NULL, DEFAULT true | Key enabled |
| last_used_at | TIMESTAMPTZ | NULL | Last usage timestamp |
| expires_at | TIMESTAMPTZ | NULL | Expiration (NULL = never) |
| created_at | TIMESTAMPTZ | NOT NULL | Creation timestamp |

**Indexes**: key_hash (unique), merchant_id, user_id, is_active  
**Note**: Full API key shown only at creation time. Only hash + prefix stored.

---

## Group D: Remove Duplicate Task T134

### D1: Remove T134 (line 1296-1298)

Delete:
```
- [ ] T134 [P] Create `pkg/errors/errors.go` — Comprehensive error handling with HTTP status mapping
  - **Context Files**: `internal/model/errors.go`
  - **Creates**: `pkg/errors/errors.go`
```

### D2: Update T053 (line 259)

Change description:
```
- [ ] T053 [P] Create `pkg/errors/errors.go` — Comprehensive error wrapping, HTTP status mapping, error codes
```

---

## Group E: API Contract CRUD + ExchangeRate Gaps

Add to `api-v1.yaml` before the `/health` path.

### Missing CRUD Endpoints

| Entity | Path | Methods Added | Notes |
|--------|------|---------------|-------|
| Invoice | `/merchants/{id}/invoices/{invoiceId}` | GET | Was missing GET-by-ID |
| Payout | `/merchants/{id}/payouts` | POST | Was list-only |
| Payout | `/merchants/{id}/payouts/{payoutId}` | GET | Was missing GET-by-ID |
| Dispute | `/merchants/{id}/disputes` | POST | Was list-only |
| Dispute | `/merchants/{id}/disputes/{disputeId}` | GET | Was missing GET-by-ID |
| Dispute | `/merchants/{id}/disputes/{disputeId}/evidence` | POST | New: evidence submission |
| WebhookConfig | `/merchants/{id}/webhooks/{webhookId}` | GET, PUT, DELETE | Was missing GET/PUT/DELETE |
| ProviderConfig | `/merchants/{id}/providers/{providerId}` | GET, PUT, DELETE | Was missing GET/PUT/DELETE |
| ExchangeRate | `/merchants/{id}/exchange-rates` | GET | Was schema-only, no endpoints |

---

## Group F: Phase 16 — Observability & Monitoring

New phase in `tasks.md` after Phase 15 (SDK Generation).

### Tasks

| ID | Task | Creates/Modifies | Dependencies |
|----|------|------------------|--------------|
| T160 | OTel SDK initialization | `internal/observability/otel.go` | T005 |
| T161 | OTel HTTP tracing middleware | `internal/middleware/tracing.go` | T160 |
| T162 | Custom Prometheus metrics | `internal/observability/metrics.go` | T160 |
| T163 | Prometheus scrape config | `configs/prometheus.yml` | T162 |
| T164 | Grafana dashboard JSON | `configs/grafana/dashboards/helix-overview.json` | T162 |
| T165 | Grafana datasource provisioning | `configs/grafana/provisioning/datasources.yml` | T163 |
| T166 | Register OTel middleware in router | Modifies `internal/handler/router.go` | T063, T161, T162 |
| T167 | Grafana alerting rules | `configs/grafana/alerting/provider-failure.yml` | T162 |
| T168 | Alert generation script | `scripts/generate-alerts.sh` | T163 |
| T169 | Expand health check for OTel | Modifies `internal/handler/health.go` | T133, T160 |
| T170 | OTel/metrics/tracing tests | `tests/unit/otel_test.go`, etc. | T160, T161, T162 |

**Checkpoint**: Observability stack operational — traces, metrics, dashboards, alerts

---

## Group G: Phase 17 — Backup & Disaster Recovery

New phase in `tasks.md` after Phase 16.

### Tasks

| ID | Task | Creates | Dependencies |
|----|------|---------|--------------|
| T171 | Full backup script (pg_dump) | `scripts/backup-full.sh` | — |
| T172 | Incremental backup (WAL) | `scripts/backup-incremental.sh` | — |
| T173 | Backup verification script | `scripts/backup-verify.sh` | T171 |
| T174 | Systemd timer scheduling | `scripts/backup-schedule.sh` + service/timer units | T171, T172 |
| T175 | Restore runbook | `docs/operations/RESTORE_RUNBOOK.md` | T171, T172, T173 |
| T176 | Backup integration test | `tests/integration/backup_test.go` | T171, T173 |

**Checkpoint**: Backup infrastructure complete — daily full + hourly incrementals, restore runbook

---

## Group H: Phase 18 — WebSocket, Load Testing, Docs Pipeline

New phase in `tasks.md` after Phase 17.

### WebSocket Tasks

| ID | Task | Creates/Modifies | Dependencies |
|----|------|------------------|--------------|
| T177 | WebSocket hub (gorilla/websocket) | `internal/websocket/hub.go` | T058 |
| T178 | WebSocket handler + auth | `internal/websocket/handler.go` | T177, T059 |
| T179 | Register WebSocket endpoint | Modifies `internal/handler/router.go` | T063, T178 |
| T180 | WebSocket integration tests | `tests/integration/websocket_test.go` | T177, T178 |

### Load Testing Tasks

| ID | Task | Creates | Dependencies |
|----|------|---------|--------------|
| T181 | k6: merchant config < 5 min | `tests/load/k6-merchant-config.js` | — |
| T182 | k6: API p95 < 150ms | `tests/load/k6-api-latency.js` | — |
| T183 | k6: dashboard < 1.5s | `tests/load/k6-dashboard.js` | — |
| T184 | Load test runner | `tests/load/run-all.sh` | T181, T182, T183 |

### Quality Gate Tasks

| ID | Task | Creates/Modifies | Dependencies |
|----|------|------------------|--------------|
| T185 | SonarQube config | `configs/sonar-project.properties` | — |
| T186 | SonarQube scan script | `scripts/sonar-scan.sh` | T185 |
| T187 | Coverage gate in quickstart | Modifies `quickstart.md` | T186 |

### Documentation Pipeline Tasks

| ID | Task | Creates | Dependencies |
|----|------|---------|--------------|
| T188 | pandoc build script | `scripts/docs-build.sh` | — |
| T189 | Docs build test | `tests/integration/docs_test.go` | T188 |

**Checkpoint**: Real-time events, load tests pass SLA, quality gates configured, docs pipeline operational

---

## Group I: Execution Order Updates

Update `tasks.md` Phase Dependencies section (after line 1500) to include new phases:

```
- **Observability (Phase 16)**: Depends on Phase 2 (config) — can run in parallel with user stories
- **Backup/DR (Phase 17)**: Independent — can run anytime after Phase 1
- **Real-Time & Load Testing (Phase 18)**: WebSocket depends on Phase 2.5 (auth); load tests depend on Phase 13 (polish)
```

Update Implementation Strategy → Full Delivery section (around line 1591):

```
### Full Delivery

1. Complete Phases 1-2.5 → Foundation ready
2. Complete Phases 3-6 → MVP functional
3. Complete Phase 14 → Reconciliation & billing active
4. Complete Phases 7-10 → All payment features
5. Complete Phase 11 → Web portal
6. Complete Phase 12 → CLI
7. Complete Phase 13 → Polish & security audit
8. Complete Phase 15 → SDKs generated
9. Complete Phase 16 → Observability operational
10. Complete Phase 17 → Backup infrastructure ready
11. Complete Phase 18 → WebSocket, load tests, quality gates, docs pipeline
12. **FINAL VALIDATION**: Run quickstart.md scenarios, security audit, SDK smoke tests, load tests
```

Update Parallel Opportunities section (around line 1537):

```
- Observability (Phase 16) can run in parallel with user stories
- Backup/DR (Phase 17) is fully independent
- WebSocket (Phase 18 partial) can start after Phase 2.5
- Load tests (Phase 18) require Phase 13 completion
- Documentation pipeline (Phase 18) is fully independent
```

---

## Group J: plan.md Updates

Add to project structure section (after line 122):

```markdown
configs/                # Prometheus, Grafana, SonarQube configs
├── prometheus.yml
├── grafana/
│   ├── dashboards/
│   └── provisioning/
├── sonar-project.properties
docs/operations/        # Operational runbooks
├── RESTORE_RUNBOOK.md
```

---

## Execution Order

Groups A–H are fully independent (no ordering constraints). Group I depends on F+G+H (new task IDs must exist). Group J is independent.

**Recommended execution sequence:**
1. Groups A, B, C, D, E, J — parallel (config, contract, model, task cleanup)
2. Groups F, G, H — parallel (new phases)
3. Group I — after F+G+H (execution order references)

---

## Verification

After all edits:

1. `grep -c "^- \[" specs/001-helix-seller-platform/tasks.md` — should be ~274
2. `grep -c "^## Phase" specs/001-helix-seller-platform/tasks.md` — should be 18
3. `grep "JWTPrivateKeyPath" internal/config/config.go` — should match
4. `grep "RS256" .specify/memory/constitution.md` — should match (unchanged)
5. `grep "RS256" specs/001-helix-seller-platform/contracts/api-v1.yaml` — bearerFormat: JWT (should be present)
6. `grep -c "User:" specs/001-helix-seller-platform/data-model.md` — should be 2 (User + ApiKey note)
7. `grep "T134" specs/001-helix-seller-platform/tasks.md` — should return nothing (removed)
8. `grep "T160" specs/001-helix-seller-platform/tasks.md` — should match (new phase)
9. `grep "/auth/login" specs/001-helix-seller-platform/contracts/api-v1.yaml` — should match
10. `grep "/exchange-rates" specs/001-helix-seller-platform/contracts/api-v1.yaml` — should match
