# Local CI Integration

HelixQA is designed for local CI pipelines. The parent project has a permanent policy against GitHub Actions and any cloud-hosted CI/CD. All builds, services, and QA testing run locally using Podman containers. This guide covers how to integrate HelixQA into that workflow.

## Why Local CI

The Catalogizer project runs all CI locally for three reasons:

1. **Resource control** -- the host machine has strict resource limits (30-40% CPU/RAM budget). Cloud CI runners do not respect these constraints.
2. **Device access** -- Android devices and emulators are connected to the local network. Cloud CI cannot reach them.
3. **Privacy** -- LLM API keys and session data stay on the local machine.

HelixQA fits naturally into this model. It runs as a single binary, reads environment variables, and writes results to disk.

---

## Podman-Based Test Execution

### Building the Container

HelixQA ships with a `Dockerfile` and `docker-compose.qa-robot.yml`. Build the container image with:

```bash
podman build --network host -t helixqa:latest .
```

The `--network host` flag is mandatory. Default container networking causes SSL certificate issues when calling LLM provider APIs.

### Running in a Container

```bash
podman run --rm --network host \
    -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
    -e HELIX_WEB_URL="http://localhost:3000" \
    -v "$(pwd)/qa-results:/app/qa-results:Z" \
    -v "$(pwd):/project:ro,Z" \
    helixqa:latest \
    autonomous --project /project --platforms web --timeout 15m
```

Key flags:

| Flag | Purpose |
|------|---------|
| `--network host` | Access to localhost services and LAN devices |
| `-e` | Pass API keys and configuration |
| `-v qa-results` | Mount output directory for result persistence |
| `-v /project:ro` | Mount project root read-only for the learning phase |

### Using docker-compose.test.yml

The parent project provides a test stack that runs the API, web frontend, and Playwright together:

```yaml
# docker-compose.test.yml (simplified)
services:
  catalog-api:
    build: ./catalog-api
    network_mode: host
  catalog-web:
    build: ./catalog-web
    network_mode: host
    depends_on:
      - catalog-api
  helixqa:
    build: ./HelixQA
    network_mode: host
    depends_on:
      - catalog-web
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - HELIX_WEB_URL=http://localhost:3000
    volumes:
      - ./qa-results:/app/qa-results
      - .:/project:ro
```

Start the full stack:

```bash
podman-compose -f docker-compose.test.yml up --abort-on-container-exit
```

The `--abort-on-container-exit` flag stops all services when HelixQA finishes, which is the desired behaviour for CI.

---

## Running HelixQA from Scripts

### Minimal CI Script

Create a script at `scripts/run-helixqa.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# Source environment
if [[ -f .env ]]; then
    set -a
    source .env
    set +a
fi

# Verify prerequisites
command -v podman >/dev/null 2>&1 || {
    echo "ERROR: podman is required"
    exit 1
}

# Start application services
echo "Starting services..."
podman-compose -f docker-compose.dev.yml up -d

# Wait for services to be ready
echo "Waiting for services..."
for i in $(seq 1 30); do
    if curl -s -o /dev/null -w "%{http_code}" \
        http://localhost:3000 | grep -q "200"; then
        break
    fi
    sleep 2
done

# Run HelixQA
echo "Running HelixQA..."
helixqa autonomous \
    --project . \
    --platforms web \
    --timeout 30m \
    --curiosity=false \
    --report markdown,json \
    --output qa-results

EXIT_CODE=$?

# Stop services
echo "Stopping services..."
podman-compose -f docker-compose.dev.yml down

exit $EXIT_CODE
```

### Test Bank CI Script

For bank-driven testing without LLM dependencies:

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "Running test bank suite..."
helixqa run \
    --banks challenges/helixqa-banks/ \
    --platform web \
    --browser-url http://localhost:3000 \
    --speed fast \
    --record=false \
    --timeout 10m \
    --output qa-results

echo "Generating HTML report..."
helixqa report \
    --input qa-results \
    --format html
```

### Challenge Validation Script

Run HelixQA's self-validation challenges before the main QA session:

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "Running self-validation challenges..."
helixqa challenges run --all --verbose

if [[ $? -ne 0 ]]; then
    echo "ERROR: HelixQA self-validation failed"
    echo "Fix framework issues before running QA sessions"
    exit 1
fi

echo "Self-validation passed, proceeding to QA..."
helixqa autonomous --project . --platforms web --timeout 30m
```

---

## Parsing Results Programmatically

### Exit Codes

| Code | Meaning | CI Action |
|------|---------|-----------|
| `0` | All tests passed, no issues | Continue pipeline |
| `1` | Issues detected or runtime failure | Investigate results |

### JSON Report Parsing

The JSON report is the machine-readable output. Parse it with `jq` for automated decisions:

```bash
# Check if any critical findings exist
CRITICAL=$(cat qa-results/session-*/pipeline-report.json | \
    jq '[.findings[] | select(.severity == "critical")] | length')

if [[ "$CRITICAL" -gt 0 ]]; then
    echo "CRITICAL: $CRITICAL critical findings detected"
    exit 1
fi

# Extract pass/fail summary
cat qa-results/session-*/pipeline-report.json | \
    jq '{
        total: .total_tests,
        passed: .passed,
        failed: .failed,
        coverage: .coverage_ratio
    }'

# List all findings with severity
cat qa-results/session-*/pipeline-report.json | \
    jq '.findings[] | {id, severity, title}'
```

### Coverage Threshold Gate

Block the pipeline if coverage drops below a threshold:

```bash
COVERAGE=$(cat qa-results/session-*/pipeline-report.json | \
    jq '.coverage_ratio')

THRESHOLD=0.80

if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
    echo "FAIL: Coverage $COVERAGE below threshold $THRESHOLD"
    exit 1
fi

echo "PASS: Coverage $COVERAGE meets threshold $THRESHOLD"
```

---

## Integration with Security Scanning

HelixQA sessions can run alongside the project's security scanning tools. A typical local CI pipeline runs them sequentially to stay within the 30-40% resource budget:

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "=== Phase 1: Security Scan ==="
./scripts/security-scan.sh

echo "=== Phase 2: HelixQA Session ==="
./scripts/run-helixqa.sh

echo "=== Phase 3: SonarQube Analysis ==="
./scripts/run-sonarqube-scan.sh

echo "=== All phases complete ==="
```

### Resource-Limited Execution

Respect the host resource budget by limiting container resources:

```bash
podman run --rm --network host \
    --cpus=2 --memory=4g \
    -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
    -v "$(pwd)/qa-results:/app/qa-results:Z" \
    helixqa:latest \
    autonomous --project /project --platforms web --timeout 30m
```

| Component | CPU Limit | Memory Limit |
|-----------|----------|-------------|
| catalog-api | 2 CPUs | 4 GB |
| catalog-web | 1 CPU | 2 GB |
| helixqa | 2 CPUs | 4 GB |
| **Total** | **5 CPUs** | **10 GB** |

Keep total container budget under 4 CPUs and 8 GB RAM when running all containers simultaneously.

---

## Example: Full Local CI Pipeline

This script runs the complete pipeline that would normally be handled by a cloud CI service:

```bash
#!/usr/bin/env bash
# scripts/local-ci.sh -- Full local CI pipeline
set -euo pipefail

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
LOG_DIR="ci-logs/$TIMESTAMP"
mkdir -p "$LOG_DIR"

log() { echo "[$(date +%H:%M:%S)] $*" | tee -a "$LOG_DIR/pipeline.log"; }

# Phase 1: Build
log "Building catalog-api..."
cd catalog-api && go build -o ../build/catalog-api . 2>&1 | \
    tee "$LOG_DIR/build-api.log"
cd ..

log "Building catalog-web..."
cd catalog-web && npm run build 2>&1 | tee "$LOG_DIR/build-web.log"
cd ..

# Phase 2: Unit tests
log "Running Go tests..."
cd catalog-api && GOMAXPROCS=3 go test ./... -p 2 -parallel 2 2>&1 | \
    tee "$LOG_DIR/test-go.log"
cd ..

log "Running frontend tests..."
cd catalog-web && npm run test 2>&1 | tee "$LOG_DIR/test-web.log"
cd ..

# Phase 3: Start services
log "Starting services..."
podman-compose -f docker-compose.dev.yml up -d
sleep 10  # wait for services

# Phase 4: HelixQA
log "Running HelixQA session..."
helixqa autonomous \
    --project . \
    --platforms web \
    --timeout 30m \
    --curiosity=false \
    --report markdown,json \
    --output "qa-results/$TIMESTAMP" 2>&1 | \
    tee "$LOG_DIR/helixqa.log"

QA_EXIT=$?

# Phase 5: Collect results
log "Stopping services..."
podman-compose -f docker-compose.dev.yml down

# Phase 6: Report
if [[ $QA_EXIT -eq 0 ]]; then
    log "PIPELINE PASSED"
else
    log "PIPELINE FAILED -- review qa-results/$TIMESTAMP/"
fi

exit $QA_EXIT
```

Make it executable and run:

```bash
chmod +x scripts/local-ci.sh
./scripts/local-ci.sh
```

---

## Scheduling Recurring Runs

Use cron for periodic QA sessions:

```bash
# Run HelixQA every night at 2 AM
0 2 * * * cd /path/to/project && ./scripts/run-helixqa.sh >> /var/log/helixqa.log 2>&1
```

Or use systemd timers for more control over resource limits and logging.

## Related Pages

- [Containerisation](/advanced/containers) -- container setup and docker-compose files
- [Autonomous QA](/guides/autonomous-qa) -- detailed session walkthrough
- [CLI Reference](/reference/cli) -- all command flags
- [Configuration](/reference/config) -- environment variables
- [Challenges](/guides/challenges) -- self-validation before QA sessions
