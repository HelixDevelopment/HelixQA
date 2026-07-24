# HXS — Helix Seller Full-System Challenge Suite

| Field | Value |
|-------|-------|
| Revision | 1 |
| Created | 2026-07-24 |
| Status | approved |
| Workable Items Key | HXS |
| Constitution Compliance | §11.4.27, §11.4.28, §11.4.29, §11.4.31, §11.4.74 |

---

## 1. Submodule Integration

Two new submodules added to `.gitmodules` at grouped layout (`submodules/<name>/`):

```
[submodule "challenges"]
    path = submodules/challenges
    url = git@github.com:vasic-digital/Challenges.git
[submodule "helix_qa"]
    path = submodules/helix_qa
    url = git@github.com:HelixDevelopment/HelixQA.git
```

Both use lowercase snake_case per §11.4.29.

### Directory Structure

```
helix_seller/
├── .gitmodules
├── submodules/
│   ├── challenges/      ← vasic-digital/Challenges (anti-bluff lib, challenge framework)
│   └── helix_qa/        ← HelixDevelopment/HelixQA (bridge, recording, OpenCV pipeline)
├── docs/
│   ├── challenges/       ← challenge documentation
│   ├── workable-items/   ← HXS workable items tracking (version-controlled)
│   └── recordings/       ← video outputs (gitignored, binary)
├── tests/
│   └── challenges/
│       ├── config/       ← env-specific config (hosts, ports, credentials)
│       ├── baselines/    ← OpenCV visual regression baselines (screenshots)
│       ├── scripts/      ← hxs_*.sh modular challenge scripts
│       └── run_challenges.sh ← top-level orchestration entrypoint
└── .gitignore            ← recordings/ and large artifacts excluded
```

### Dependency Layout Per §11.4.28(C)

- `submodules/challenges/` owns further submodules? Check `helix-deps.yaml`. Resolve recursively.
- `submodules/helix_qa/` — same check. Any conflict flagged to operator.
- HelixQA bridge runs as a Docker container or systemd service per its own docs.

### helix-deps.yaml for Helix Seller

A project-level `helix-deps.yaml` at root declares:

```yaml
schema_version: 1
deps:
  - name: challenges
    ssh_url: git@github.com:vasic-digital/Challenges.git
    ref: main
    why: "Full-system challenge suite framework — anti_bluff.sh library, challenge execution infrastructure"
    layout: grouped
  - name: helix_qa
    ssh_url: git@github.com:HelixDevelopment/HelixQA.git
    ref: main
    why: "QA infrastructure — recording pipeline, OpenCV visual analysis, findings stream, bridge API"
    layout: grouped
transitive_handling:
  recursive: true
  conflict_resolution: operator-required
```

---

## 2. Challenge Suite Architecture

Seven modular scripts under `tests/challenges/scripts/`, each independently runnable:

| # | Script | Purpose |
|---|--------|---------|
| 1 | `hxs_setup.sh` | Clean DB → migrate → start services → create user accounts with default creds → health-check |
| 2 | `hxs_frontend_e2e.sh` | Angular portal — all 12 pages, all flows, happy paths, edge cases, failure paths |
| 3 | `hxs_backend_e2e.sh` | Full API surface — auth, CRUD, webhooks, webSocket, reconciliation, background tasks |
| 4 | `hxs_recording.sh` | HelixQA bridge recording orchestration |
| 5 | `hxs_opencv_analysis.sh` | Latest OpenCV visual analysis against recordings |
| 6 | `hxs_workable_items.sh` | HXS-keyed findings capture to structured YAML |
| 7 | `hxs_repeat_until_green.sh` | Orchestrator — endless loop until zero issues |

### Dependency Chain

```
hxs_setup.sh ───────────────────────────────────────────┐
    │                                                     │
    ├── hxs_frontend_e2e.sh ─────▶ recorded by ─────────┤ │
    │                              hxs_recording.sh      │ │
    ├── hxs_backend_e2e.sh ──────▶ recorded by ─────────┤ │
    │                              hxs_recording.sh      │ │
    │                                                     │ │
    └── hxs_opencv_analysis.sh ◀── analyzes recordings ──┘ │
        │                                                   │
        └── hxs_workable_items.sh ─── creates HXS items ──┘
            │
            └── hxs_repeat_until_green.sh ◀── orchestrates ── all of the above
```

### Common Pattern (All Scripts)

Each script:
1. Sources `anti_bluff.sh` from `submodules/challenges/lib/`
2. Uses `ab_init`, `ab_pass`, `ab_fail`, `ab_skip`, `ab_summary` for structured results
3. Writes results to `/tmp/hxs_<script_name>.results` (NDJSON)
4. Exits 0 (all pass) or 1 (any fail) or 2 (skip: infrastructure not available)

---

## 3. Setup Script (hxs_setup.sh)

### Flow

1. **Clean environment**
   - Drop and recreate test PostgreSQL database
   - Run all migrations (`go run cmd/migrate/main.go up`)
   - Clear Redis (if running)
   - Remove old recordings from `docs/recordings/`
   - Ensure NATS is healthy

2. **Start services**
   - Start Helix Seller server (`go run cmd/server/main.go` with test config)
   - Wait for health endpoint (`GET /health` → 200)
   - Start Angular dev server (`ng serve` in `web/`)
   - Wait for Angular (`GET http://localhost:4200` → 200)

3. **Create user accounts**
   - Admin: `admin@helix.test` / `admin123!`
   - Merchant: `merchant@helix.test` / `merchant123!`
   - Customer: `customer@helix.test` / `customer123!`
   - Store credentials in `tests/challenges/config/credentials.env`
   - Verify login via API (`POST /api/v1/auth/login` → JWT)

4. **Health-check all services**
   - Backend API health
   - Angular portal accessibility
   - WebSocket connectivity
   - HelixQA bridge health (if available)

### Output

- `/tmp/hxs_setup.results` — structured success/fail per step
- `tests/challenges/config/credentials.env` — version-controlled default credentials
- Exit 0 if all services healthy, exit 1 otherwise

---

## 4. Frontend E2E Script (hxs_frontend_e2e.sh)

### Test Scope — All 12 Angular Pages

| Page | Route | Test Cases |
|------|-------|------------|
| Login | `/login` | Valid creds, invalid creds, empty fields, JWT expiry redirect, already logged-in redirect |
| Dashboard | `/dashboard` | Widget rendering, data loading, empty state, error state, real-time updates |
| Products | `/products` | List, search, pagination, create (valid/invalid), edit, delete, empty catalog |
| Orders | `/orders` | List, filter by status, order detail, cancel, refund, invoice download |
| Customers | `/customers` | List, search, customer detail, transaction history, empty state |
| Merchant Profile | `/merchant/profile` | View, edit (valid/invalid), save, cancel |
| Merchant Settings | `/merchant/settings` | Payment providers toggle, notification pref, theme toggle (light/dark) |
| Payouts | `/payouts` | List, upcoming payout, history, empty state |
| Webhooks | `/webhooks` | List, create endpoint, test ping, delete, secret reveal |
| Providers | `/providers` | Stripe connect, PayPal config, Square config, status badges |
| Subscription | `/subscription` | Current plan display, upgrade, downgrade, cancel |
| Reports | `/reports` | Sales report, payout report, date range filter, export CSV |

### Testing Methodology

- Uses `curl` + `grep`/`jq` for API-level assertions
- Uses HelixQA bridge for browser-based UI assertions via screenshot comparison
- Visual regression: compare screenshots against baselines in `tests/challenges/baselines/`
- Each page has: happy path test, empty state test, error state test, edge case test

### Assertions Per Page

```
ab_pass / ab_fail for:
  - Page loads (HTTP 200 + expected title in HTML)
  - Data renders (key elements present)
  - CRUD operations succeed where applicable
  - Error states handled gracefully (no bare stack traces)
  - Responsive layout (viewport widths: 375px, 768px, 1440px)
```

---

## 5. Backend E2E Script (hxs_backend_e2e.sh)

### API Surface Coverage

| Category | Endpoints | Test Cases |
|----------|-----------|------------|
| Auth | `POST /api/v1/auth/login`, `/register`, `/logout`, `/refresh` | Valid creds, wrong password, expired token, refresh flow, duplicate email |
| Merchants | `GET /api/v1/merchants`, `POST`, `GET /:id`, `PUT /:id` | CRUD, validation, auth enforcement, not-found, duplicate |
| Products | `GET /api/v1/products`, `POST`, `GET /:id`, `PUT /:id`, `DELETE /:id` | CRUD, pagination, search, soft-delete |
| Orders | `GET /api/v1/orders`, `POST`, `GET /:id`, `PUT /:id/cancel` | State machine, refund, validation, not-found |
| Payments | `POST /api/v1/payments/charge`, `POST /refund` | Idempotency keys, amount validation, provider routing |
| Webhooks | `POST /api/v1/webhooks/stripe`, `/paypal`, `/square` | Signature verification, malformed bodies, replay protection |
| Payouts | `GET /api/v1/payouts`, `GET /api/v1/payouts/:id` | List, detail, status transitions |
| WebSocket | `ws://localhost:8080/ws` | Connect, auth, subscribe to events, receive updates |

### Systematic Testing

- Each endpoint tested with: valid payload, invalid payload, missing auth, wrong role, malformed input
- Rate limiting: verify 429 after threshold
- Idempotency: same key twice → same result
- Audit logging: verify audit entries created for mutating operations

---

## 6. Recording Pipeline

### Flow

```
hxs_recording.sh
  │
  ├─ 1. GET /v1/health → assert "ok" body
  ├─ 2. POST /v1/recording/start
  │      {"test_name":"hxs_<run_id>","interval_ms":500}
  ├─ 3. Execute frontend + backend test scripts (recorded live)
  ├─ 4. POST /v1/recording/stop → get recording_path
  └─ 5. Copy recording to docs/recordings/hxs_<run_id>_<timestamp>.mp4
```

### Bridge API Dependencies

- URL: `http://127.0.0.1:7842` (configurable via `HELIXQA_BRIDGE_URL`)
- Endpoints: `/v1/health`, `/v1/recording/start`, `/v1/recording/stop`
- If bridge unavailable → script exits 2 (SKIP), recording skipped, tests run without video

### Output Format

- MP4 video files at `docs/recordings/hxs_<run_id>_<run_count>_<timestamp>.mp4`
- Each recording named per the sub-challenge being recorded
- Manifest at `docs/recordings/MANIFEST.yaml` listing all recordings with metadata

---

## 7. OpenCV Visual Analysis

### Latest OpenCV Integration

HelixQA already provides `pkg/vision/` with:
- `element_detector.go` — ORB/SIFT/AKAZE template matching
- `ocr.go` — Tesseract OCR binding
- `frame_analyzer.go` — frame-by-frame diff analysis

### Update to Latest OpenCV

1. Update `gocv.io/x/gocv` in HelixQA `go.mod` to latest release
2. Verify OpenCV system library is the latest version
3. Enable new OpenCV features:
   - Improved ORB descriptor matching
   - Better SIFT performance (non-free module)
   - QR code / barcode detection
   - Text detection (EAST model)

### Analysis Pipeline

```
hxs_opencv_analysis.sh
  │
  ├─ 1. POST /v1/analyze/start {"test_name":"hxs_<run_id>"}
  ├─ 2. Stream findings via GET /v1/findings/stream
  ├─ 3. For each finding:
  │      ├─ match against baseline (template)
  │      ├─ verify text via OCR
  │      ├─ measure layout shift
  │      └─ detect DOM/visual anomalies
  ├─ 4. Write structured results to /tmp/hxs_analysis.results
  └─ 5. Update baseline gallery on request
```

### Detection Capabilities

| Feature | Algorithm | Purpose |
|---------|-----------|---------|
| Button detection | ORB template matching | Verify buttons are present and clickable |
| Text verification | Tesseract OCR | Verify displayed text matches expected |
| Layout comparison | SIFT feature matching | Visual regression against baseline screenshots |
| Animation detection | Frame diff analysis | Detect jank, frozen frames, or visual glitches |
| Error popup detection | AKAZE matching | Match error dialog templates |
| QR/barcode | OpenCV QRCodeDetector | Verify payment QR codes render correctly |

---

## 8. Workable Items System (HXS)

### Key Prefix

`HXS` — all workable items use this prefix.

### Storage Location

`docs/workable-items/` — version-controlled YAML files.

### Item Schema

```yaml
---
id: HXS-001
title: "Short description of issue"
status: open | in_progress | fixed | verified | closed
severity: critical | important | minor | enhancement
source: "hxs_frontend_e2e.sh:142"
challenge_run: "2026-07-24T21:30:00Z"
description: >
  Full description of the issue found.
recordings:
  - "docs/recordings/hxs_<run_id>_<timestamp>.mp4"
findings:
  - field: "element_detector.match"
    expected: ">= 0.85"
    actual: "0.42"
    decision: FAIL
root_cause: "What investigation revealed"
fix:
  pr: "commit hash"
  files: ["path/to/file.go:45"]
  verified_by: "hxs_verify_<n>.sh"
related_items: []
```

### DAB (Data Asset Base)

`docs/workable-items/DAB.yaml` — master index:

```yaml
---
dab_id: HXS-DAB
title: "Helix Seller Workable Items Data Asset Base"
created: "2026-07-24"
items:
  - id: HXS-001
    title: "..."
    status: closed
    severity: critical
  - id: HXS-002
    title: "..."
    status: open
    severity: minor
summary:
  total: 2
  open: 1
  in_progress: 0
  fixed: 0
  verified: 0
  closed: 1
```

### Lifecycle

```
open → in_progress → fixed → verified → closed
  ↑                      ↑
  │                      └── verification run must pass
  └── auto-created on FAIL or warning
```

Auto-created by `hxs_workable_items.sh` when a challenge script reports a FAIL. Transitions to `verified` only after a dedicated verification sub-challenge passes.

---

## 9. Orchestrator — Endless Autonomous Loop

### `hxs_repeat_until_green.sh`

```pseudo
RUN_COUNT = 0
while True:
    RUN_COUNT += 1
    RUN_ID = "hxs_run_$(date +%Y%m%d_%H%M%S)"
    
    log "=== HXS Run $RUN_COUNT: $RUN_ID ==="
    
    # Phase 1: Setup
    run_or_skip hxs_setup.sh
    
    # Phase 2: Recorded testing
    hxs_recording.sh start
    run hxs_frontend_e2e.sh
    run hxs_backend_e2e.sh
    hxs_recording.sh stop
    
    # Phase 3: Analysis
    run hxs_opencv_analysis.sh
    
    # Phase 4: Findings capture
    run hxs_workable_items.sh
    
    # Phase 5: Decision
    if has_issues():
        log "⚠️ Issues found — creating/updating workable items"
        for each CRITICAL issue:
            run systematic_debugging_subagent()
            commit_and_push_fix()
        # loop continues for re-validation
    else:
        log "✅ ALL CLEAN — zero issues"
        generate_final_report()
        commit_and_push()
        create_git_tag("hxs-v1-run-${RUN_COUNT}")
        break
```

### Exit Conditions

- Only exits when ALL assertions across ALL scripts pass
- Zero warnings, zero failures, zero findings
- Generates final report at `docs/challenges/HXS_FINAL_REPORT.md`
- Creates git tag `hxs-v1-run-<N>`

### Systematic Debugging Integration

When a CRITICAL item is detected:
1. Orchestrator pauses the main loop
2. Launches a systematic-debugging sub-agent
3. Sub-agent investigates root cause, applies fix
4. Fix is committed with `HXS-XXX` reference in message
5. Submodule pointers updated if changes were in submodules
6. Loop resumes from `hxs_setup.sh` for fresh validation

---

## 10. Implementation Plan

### Phase A: Submodule Setup

1. Add `submodules/challenges` to `.gitmodules` + `git submodule add`
2. Add `submodules/helix_qa` to `.gitmodules` + `git submodule add`
3. Create `helix-deps.yaml` at project root
4. Run `install_upstreams.sh` from any submodule that has it
5. Update `.gitignore` for recordings and large artifacts
6. Commit submodule pointer changes

### Phase B: Challenge Scripts

1. Create `tests/challenges/config/` with default credentials template
2. Create `tests/challenges/scripts/hxs_setup.sh`
3. Create `tests/challenges/scripts/hxs_frontend_e2e.sh`
4. Create `tests/challenges/scripts/hxs_backend_e2e.sh`
5. Create `tests/challenges/scripts/hxs_recording.sh`
6. Create `tests/challenges/scripts/hxs_opencv_analysis.sh`
7. Create `tests/challenges/scripts/hxs_workable_items.sh`
8. Create `tests/challenges/scripts/hxs_repeat_until_green.sh`
9. Create `tests/challenges/run_challenges.sh` (top-level entrypoint)

### Phase C: Documentation

1. Create `docs/challenges/README.md` — how to run challenges
2. Create `docs/challenges/HXS_USER_ACCOUNTS.md` — default credentials
3. Create `docs/workable-items/DAB.yaml` — initial DAB
4. Create `docs/recordings/MANIFEST.yaml` — recording manifest template
5. Create `tests/challenges/baselines/` directory with initial baseline screenshots

### Phase D: OpenCV Update

1. Update HelixQA `go.mod` to latest `gocv.io/x/gocv`
2. Rebuild HelixQA bridge
3. Verify OpenCV system library version

### Phase E: Verification

1. Run `hxs_setup.sh` standalone — verify clean environment
2. Run `hxs_frontend_e2e.sh` standalone — verify frontend tests
3. Run `hxs_backend_e2e.sh` standalone — verify backend tests
4. Run `hxs_repeat_until_green.sh` — full orchestration pass
5. Verify workable items created correctly
6. Verify recordings produced
7. Commit final state, tag, push to all upstreams

---

## 11. Compliance Matrix

| Constitution § | Requirement | Implementation |
|----------------|-------------|----------------|
| §11.4.27 | Challenges + HelixQA as required dependencies | `.gitmodules` entries at `submodules/challenges/` and `submodules/helix_qa/` |
| §11.4.28(C) | Dependency layout | Grouped under `submodules/<name>/` |
| §11.4.29 | Lowercase snake_case | `submodules/challenges/`, `submodules/helix_qa/`, `tests/challenges/scripts/hxs_*.sh` |
| §11.4.31 | helix-deps.yaml at root | `helix-deps.yaml` with recursive transitive handling |
| §11.4.74 | Catalogue-first discovery | `submodules-catalogue.md` surveyed — both repos listed in Testing + QA category |
| §11.4.36 | install_upstreams on add | Run after each submodule addition |
| §11.4.169(5) | Challenges submodule for anti-bluff banks | Anti-bluff challenge tests included in suite |
| §11.4.169(6) | HelixQA as test orchestrator | Recording + analysis pipeline via bridge |
