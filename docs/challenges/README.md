# HXS — Helix Seller Challenge Suite

## Overview

The HXS (Helix Seller) challenge suite provides full-system testing for the
Helix Seller platform. It implements an autonomous loop that:

1. Sets up a clean environment (DB, services, user accounts)
2. Tests all 12 Angular portal pages (frontend E2E)
3. Tests the full API surface (backend E2E)
4. Records all testing live via HelixQA bridge
5. Analyzes recordings with latest OpenCV (visual regression, OCR, element detection)
6. Captures findings as structured HXS workable items
7. Loops until zero issues remain

## Prerequisites

- Helix Seller services running (or will be auto-started)
- PostgreSQL database accessible
- Go 1.22+ installed
- Node.js + Angular CLI (for frontend testing)
- HelixQA bridge (optional — for recording + OpenCV analysis)
- `curl`, `jq`, `psql` utilities

## Usage

Run the full autonomous suite:
```bash
bash tests/challenges/run_challenges.sh
```

Set max runs (exit after N iterations even if issues remain):
```bash
HXS_MAX_RUNS=3 bash tests/challenges/run_challenges.sh
```

Run individual scripts:
```bash
# Setup only
bash tests/challenges/scripts/hxs_setup.sh

# Frontend tests only
bash tests/challenges/scripts/hxs_frontend_e2e.sh

# Backend tests only
bash tests/challenges/scripts/hxs_backend_e2e.sh

# Recording (requires HelixQA bridge)
bash tests/challenges/scripts/hxs_recording.sh start hxs_manual
bash tests/challenges/scripts/hxs_recording.sh stop hxs_manual
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HXS_API_URL` | `http://127.0.0.1:8080` | Backend API URL |
| `HXS_ANGULAR_URL` | `http://127.0.0.1:4200` | Angular dev server URL |
| `HXS_POSTGRES_DSN` | `postgresql://helix:helix_dev@127.0.0.1:5432/helix_seller` | DB connection |
| `HELIXQA_BRIDGE_URL` | `http://127.0.0.1:7842` | HelixQA bridge URL |
| `HXS_MAX_RUNS` | `0` (infinite) | Maximum orchestration iterations |
| `HXS_FINDINGS_TIMEOUT_S` | `30` | Timeout for findings stream |

## Outputs

| Path | Description |
|------|-------------|
| `docs/recordings/*.mp4` | Live test recordings |
| `docs/workable-items/HXS-*.yaml` | Individual workable items |
| `docs/workable-items/DAB.yaml` | Data Asset Base (master index) |
| `docs/challenges/orchestrator.log` | Orchestrator run log |
| `docs/challenges/HXS_FINAL_REPORT.md` | Final run report |

## Architecture

```
tests/challenges/
├── config/credentials.env     ← Default test credentials
├── baselines/                 ← OpenCV visual regression baselines
├── scripts/
│   ├── hxs_setup.sh           ← Environment setup
│   ├── hxs_frontend_e2e.sh    ← Angular portal tests
│   ├── hxs_backend_e2e.sh     ← API surface tests
│   ├── hxs_recording.sh       ← Recording orchestration
│   ├── hxs_opencv_analysis.sh ← Visual analysis
│   ├── hxs_workable_items.sh  ← Findings capture
│   └── hxs_repeat_until_green.sh ← Orchestrator
└── run_challenges.sh          ← Entrypoint

submodules/
├── challenges/                ← vasic-digital/Challenges (anti_bluff.sh lib)
└── helix_qa/                  ← HelixDevelopment/HelixQA (bridge, OpenCV)
```

## Constitution Compliance

| § | Requirement | Status |
|---|-------------|--------|
| §11.4.27 | Challenges + HelixQA as required dependencies | Added at `submodules/challenges/` and `submodules/helix_qa/` |
| §11.4.28(C) | Dependency layout | Grouped under `submodules/<name>/` |
| §11.4.29 | Lowercase snake_case | All paths follow convention |
| §11.4.31 | helix-deps.yaml at root | `helix-deps.yaml` present |
| §11.4.74 | Catalogue-first discovery | Surveyed — both repos in Testing + QA category |
