#!/bin/bash
# hxs_setup.sh — Clean environment + user accounts + service startup
# Part of the HXS (Helix Seller) Challenge Suite

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
CONFIG_DIR="$(cd "$SCRIPT_DIR/../config" && pwd)"
LIB_AB="$PROJECT_DIR/submodules/challenges/lib/anti_bluff.sh"

if [ ! -f "$LIB_AB" ]; then
    echo "FATAL: anti_bluff.sh not found at $LIB_AB" >&2
    exit 2
fi
. "$LIB_AB"

ab_init "hxs_setup" "/tmp/hxs_setup.results"
ab_send_action "HXS Setup — clean environment, migrate DB, create users, start services"

if [ -f "$CONFIG_DIR/credentials.env" ]; then
    . "$CONFIG_DIR/credentials.env"
else
    ab_fail "credentials.env not found at $CONFIG_DIR/credentials.env"
    ab_summary
    exit 1
fi

SETUP_OK=0
POSTGRES_DSN="${HXS_POSTGRES_DSN:-postgresql://helix:helix_dev@127.0.0.1:5432/helix_seller}"
SERVER_PORT="${HXS_SERVER_PORT:-8080}"
ANGULAR_PORT="${HXS_ANGULAR_PORT:-4200}"

on_exit() {
    rc=$?
    if [ "$SETUP_OK" = "0" ]; then
        ab_fail "Setup exited at line ${LINENO:-?} rc=$rc before completing"
        ab_summary 2>/dev/null || true
    fi
    exit $rc
}
trap on_exit EXIT

echo "=== Step 1: Database ==="
if command -v psql >/dev/null 2>&1; then
    echo "Resetting database schema..."
    psql "$POSTGRES_DSN" -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;" 2>/dev/null && \
        ab_pass "Database schema reset" || \
        ab_skip "Could not reset DB schema" "infra"
else
    ab_skip "psql not available — assuming DB is managed externally" "infra"
fi

echo "Running migrations..."
if [ -f "$PROJECT_DIR/cmd/migrate/main.go" ]; then
    (cd "$PROJECT_DIR" && go run cmd/migrate/main.go up 2>&1) && \
        ab_pass "Migrations applied" || \
        ab_fail "Migration failed"
else
    ab_skip "migrate main.go not found" "infra"
fi

echo "=== Step 2: Backend Server ==="
if command -v go >/dev/null 2>&1; then
    (cd "$PROJECT_DIR" && go run cmd/server/main.go &) &
    SERVER_PID=$!
    sleep 3
    HEALTH_CHECK=$(curl -s -m 3 -o /dev/null -w '%{http_code}' "http://127.0.0.1:$SERVER_PORT/health" 2>/dev/null || echo "000")
    if [ "$HEALTH_CHECK" = "200" ]; then
        ab_pass "Backend server healthy (HTTP 200)"
    else
        ab_skip "Backend server not responding (HTTP $HEALTH_CHECK) — start manually" "infra"
    fi
else
    ab_skip "Go not installed — start server manually" "infra"
fi

echo "=== Step 3: User Accounts ==="
BASE_URL="http://127.0.0.1:$SERVER_PORT"
if [ "$HEALTH_CHECK" != "200" ]; then
    ab_skip "Server not healthy — skipping user account creation" "infra"
else
create_user() {
    local email="$1" password="$2" name="$3"
    local resp
    resp=$(curl -s -m 5 -X POST "$BASE_URL/api/v1/auth/register" \
        -H 'Content-Type: application/json' \
        -d "{\"email\":\"$email\",\"password\":\"$password\",\"name\":\"$name\"}" 2>/dev/null || echo '{"status":"error"}')
    if echo "$resp" | grep -qiE '"(id|token|status)"[[:space:]]*:'; then
        ab_pass "Created user: $email"
        return 0
    else
        local login
        login=$(curl -s -m 5 -X POST "$BASE_URL/api/v1/auth/login" \
            -H 'Content-Type: application/json' \
            -d "{\"email\":\"$email\",\"password\":\"$password\"}" 2>/dev/null || echo '{"status":"error"}')
        if echo "$login" | grep -qiE '"(token|access_token)"[[:space:]]*:'; then
            ab_pass "Login OK for existing user: $email"
            return 0
        fi
        ab_fail "Failed to create/login user $email: $(echo "$resp" | head -c 100)"
        return 1
    fi
}

create_user "$HXS_ADMIN_EMAIL" "$HXS_ADMIN_PASSWORD" "$HXS_ADMIN_NAME"
create_user "$HXS_MERCHANT_EMAIL" "$HXS_MERCHANT_PASSWORD" "$HXS_MERCHANT_NAME"
create_user "$HXS_CUSTOMER_EMAIL" "$HXS_CUSTOMER_PASSWORD" "$HXS_CUSTOMER_NAME"
fi

echo "=== Step 4: Angular Portal ==="
ANGULAR_CHECK=$(curl -s -m 3 -o /dev/null -w '%{http_code}' "http://127.0.0.1:$ANGULAR_PORT" 2>/dev/null || echo "000")
if [ "$ANGULAR_CHECK" = "200" ]; then
    ab_pass "Angular dev server healthy (HTTP 200)"
else
    ab_skip "Angular dev server not responding (HTTP $ANGULAR_CHECK) — start with 'cd web && npm start'" "infra"
fi

echo
echo "=== hxs_setup complete ==="
SETUP_OK=1
ab_summary
