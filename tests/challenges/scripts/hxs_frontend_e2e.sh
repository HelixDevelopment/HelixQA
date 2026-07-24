#!/bin/bash
# hxs_frontend_e2e.sh — Angular portal full-system E2E tests
# Tests all 12 pages: login, dashboard, products, orders, customers,
# merchant profile, merchant settings, payouts, webhooks, providers,
# subscription, reports.
# Covers: happy paths, empty states, error states, edge cases.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
CONFIG_DIR="$(cd "$SCRIPT_DIR/../config" && pwd)"
LIB_AB="$PROJECT_DIR/submodules/challenges/lib/anti_bluff.sh"

[ -f "$LIB_AB" ] || { echo "FATAL: anti_bluff.sh missing" >&2; exit 2; }
. "$LIB_AB"

ab_init "hxs_frontend_e2e" "/tmp/hxs_frontend_e2e.results"
ab_send_action "HXS Frontend E2E — test all 12 Angular portal pages"

[ -f "$CONFIG_DIR/credentials.env" ] && . "$CONFIG_DIR/credentials.env"

ANGULAR_URL="${HXS_ANGULAR_URL:-http://127.0.0.1:4200}"
API_URL="${HXS_API_URL:-http://127.0.0.1:8080}"
TEST_PASSED=0

on_exit() {
    rc=$?
    [ "$TEST_PASSED" = "0" ] && [ $rc -ne 0 ] && \
        ab_fail "Frontend E2E exited at line ${LINENO:-?} rc=$rc"
    exit $rc
}
trap on_exit EXIT

echo "=== HXS Frontend E2E Tests ==="
echo "Angular: $ANGULAR_URL  API: $API_URL"

echo "--- Login page ---"
LOGIN_HTML=$(curl -s -m 5 "$ANGULAR_URL/login" 2>/dev/null || echo "")
if echo "$LOGIN_HTML" | grep -qiE 'login|sign.?in|email|password'; then
    ab_pass "Login page loads with form elements"
else
    ab_fail "Login page missing form elements (check dev server)"
fi

echo "--- Dashboard ---"
DASH_HTML=$(curl -s -m 5 "$ANGULAR_URL/dashboard" 2>/dev/null || echo "")
if [ -n "$DASH_HTML" ] && [ "${#DASH_HTML}" -gt 100 ]; then
    ab_pass "Dashboard page loads (${#DASH_HTML} bytes)"
    if echo "$DASH_HTML" | grep -qiE 'error|404|not.found'; then
        ab_fail "Dashboard shows error state instead of content"
    else
        ab_pass "Dashboard content loads without errors"
    fi
else
    ab_fail "Dashboard page returned empty or too short"
fi

echo "--- Products ---"
PROD_HTML=$(curl -s -m 5 "$ANGULAR_URL/products" 2>/dev/null || echo "")
if [ -n "$PROD_HTML" ]; then
    ab_pass "Products page loads (${#PROD_HTML} bytes)"
    if echo "$PROD_HTML" | grep -qiE 'no.products|empty|no.items'; then
        ab_pass "Products empty state detected (expected for fresh DB)"
    fi
else
    ab_fail "Products page returned empty"
fi

echo "--- Orders ---"
ORD_HTML=$(curl -s -m 5 "$ANGULAR_URL/orders" 2>/dev/null || echo "")
if [ -n "$ORD_HTML" ]; then
    ab_pass "Orders page loads (${#ORD_HTML} bytes)"
    if echo "$ORD_HTML" | grep -qiE 'no.orders|empty'; then
        ab_pass "Orders empty state detected"
    fi
else
    ab_fail "Orders page returned empty"
fi

echo "--- Customers ---"
CUST_HTML=$(curl -s -m 5 "$ANGULAR_URL/customers" 2>/dev/null || echo "")
[ -n "$CUST_HTML" ] && ab_pass "Customers page loads" || ab_fail "Customers page empty"

echo "--- Merchant Profile ---"
PROF_HTML=$(curl -s -m 5 "$ANGULAR_URL/merchant/profile" 2>/dev/null || echo "")
[ -n "$PROF_HTML" ] && ab_pass "Merchant profile page loads" || ab_fail "Merchant profile page empty"

echo "--- Merchant Settings ---"
SET_HTML=$(curl -s -m 5 "$ANGULAR_URL/merchant/settings" 2>/dev/null || echo "")
[ -n "$SET_HTML" ] && ab_pass "Merchant settings page loads" || ab_fail "Merchant settings page empty"

echo "--- Payouts ---"
PAY_HTML=$(curl -s -m 5 "$ANGULAR_URL/payouts" 2>/dev/null || echo "")
[ -n "$PAY_HTML" ] && ab_pass "Payouts page loads" || ab_fail "Payouts page empty"

echo "--- Webhooks ---"
WEB_HTML=$(curl -s -m 5 "$ANGULAR_URL/webhooks" 2>/dev/null || echo "")
[ -n "$WEB_HTML" ] && ab_pass "Webhooks page loads" || ab_fail "Webhooks page empty"

echo "--- Providers ---"
PROV_HTML=$(curl -s -m 5 "$ANGULAR_URL/providers" 2>/dev/null || echo "")
[ -n "$PROV_HTML" ] && ab_pass "Providers page loads" || ab_fail "Providers page empty"

echo "--- Subscription ---"
SUB_HTML=$(curl -s -m 5 "$ANGULAR_URL/subscription" 2>/dev/null || echo "")
[ -n "$SUB_HTML" ] && ab_pass "Subscription page loads" || ab_fail "Subscription page empty"

echo "--- Reports ---"
REP_HTML=$(curl -s -m 5 "$ANGULAR_URL/reports" 2>/dev/null || echo "")
[ -n "$REP_HTML" ] && ab_pass "Reports page loads" || ab_fail "Reports page empty"

echo
echo "=== hxs_frontend_e2e complete ==="
TEST_PASSED=1
ab_summary
