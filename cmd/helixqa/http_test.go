// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"digital.vasic.helixqa/pkg/autonomous"
	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/testbank"
)

// newFakeServer stands in for a real HelixCode server: it returns the
// same auth-boundary shapes the real bank asserts on (401 with the
// "Authorization header required" body on a protected route, 401 for a
// garbage bearer token, 400 on an empty login body). It is httptest
// (real HTTP over a real socket), not an in-process mock of the
// executor — the executor makes a genuine network round-trip.
func newFakeServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		switch {
		case authz == "":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Authorization header required"}`))
		case !strings.HasPrefix(authz, "Bearer "):
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Invalid authorization header format"}`))
		default:
			// Any Bearer token that is not a real JWT → invalid/expired.
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"Invalid or expired token"}`))
		}
	})

	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = decodeJSON(r, &body)
		if body["username"] == nil || body["password"] == nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"username and password are required"}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Login failed"}`))
	})

	return httptest.NewServer(mux)
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// caseFromSteps builds a one-case slice for RunHTTPBank.
func caseFromSteps(id, name string, steps ...testbank.TestStep) []*testbank.TestCase {
	return []*testbank.TestCase{{ID: id, Name: name, Steps: steps}}
}

// TestRunHTTPBank_PassOnLiveServer proves the bridge drives a real
// http: step over a real socket and PASSes when status + body match.
func TestRunHTTPBank_PassOnLiveServer(t *testing.T) {
	srv := newFakeServer(t)
	defer srv.Close()

	exec := autonomous.NewHTTPExecutor(srv.URL)
	cases := caseFromSteps("HXC-AUTH-006",
		"Protected /api/v1/tasks requires auth (401 + exact message)",
		testbank.TestStep{
			Name:               "GET /api/v1/tasks without auth",
			Action:             "http: GET /api/v1/tasks",
			ExpectStatus:       401,
			ExpectBodyContains: "Authorization header required",
		},
	)

	var buf bytes.Buffer
	rep := RunHTTPBank(context.Background(), exec, cases, true, &buf)

	if rep.PassedCases != 1 || rep.FailedCases != 0 {
		t.Fatalf("expected 1 PASS / 0 FAIL, got %+v\noutput:\n%s", rep, buf.String())
	}
	if rep.Cases[0].Verdict != "PASS" {
		t.Fatalf("verdict = %q, want PASS; detail=%q", rep.Cases[0].Verdict, rep.Cases[0].Detail)
	}
	if !strings.Contains(buf.String(), "PASS  HXC-AUTH-006") {
		t.Fatalf("missing per-case PASS line in output:\n%s", buf.String())
	}
}

// TestRunHTTPBank_RawTokenAndLoginPaths exercises the raw: auth header
// path and the empty-login 400 path — two more real-server round-trips.
func TestRunHTTPBank_RawTokenAndLoginPaths(t *testing.T) {
	srv := newFakeServer(t)
	defer srv.Close()

	exec := autonomous.NewHTTPExecutor(srv.URL)
	cases := caseFromSteps("HXC-AUTH-MULTI", "raw token + empty login",
		testbank.TestStep{
			Name:               "garbage bearer",
			Action:             "http: GET /api/v1/tasks",
			AuthMode:           "raw:not-a-real-jwt",
			ExpectStatus:       401,
			ExpectBodyContains: "Invalid or expired token",
		},
		testbank.TestStep{
			Name:               "empty login body",
			Action:             "http: POST /api/v1/auth/login",
			Body:               map[string]any{},
			ExpectStatus:       400,
			ExpectBodyContains: "required",
		},
	)

	var buf bytes.Buffer
	rep := RunHTTPBank(context.Background(), exec, cases, true, &buf)
	if rep.PassedCases != 1 || rep.FailedCases != 0 {
		t.Fatalf("expected 1 PASS, got %+v\n%s", rep, buf.String())
	}
	if rep.Cases[0].Passed != 2 {
		t.Fatalf("expected 2 passed http steps, got %d", rep.Cases[0].Passed)
	}
}

// TestRunHTTPBank_MutationWrongStatusFails is the §1.1 paired
// mutation: the SAME live request that PASSes above MUST FAIL when the
// bank's expected status is deliberately wrong (200 instead of 401).
// If this still PASSed, the bridge would be a bluff — asserting
// nothing. It proves the executor's status assertion is load-bearing
// and propagated to a non-zero case verdict + exit signal.
func TestRunHTTPBank_MutationWrongStatusFails(t *testing.T) {
	srv := newFakeServer(t)
	defer srv.Close()

	exec := autonomous.NewHTTPExecutor(srv.URL)
	cases := caseFromSteps("HXC-AUTH-006-MUT",
		"MUTATION: expect 200 where server really returns 401",
		testbank.TestStep{
			Name:         "GET /api/v1/tasks without auth (wrong expectation)",
			Action:       "http: GET /api/v1/tasks",
			ExpectStatus: 200, // deliberately wrong — server returns 401
		},
	)

	var buf bytes.Buffer
	rep := RunHTTPBank(context.Background(), exec, cases, true, &buf)

	if rep.FailedCases != 1 || rep.PassedCases != 0 {
		t.Fatalf("mutation must FAIL: got %+v\noutput:\n%s", rep, buf.String())
	}
	if rep.Cases[0].Verdict != "FAIL" {
		t.Fatalf("verdict = %q, want FAIL", rep.Cases[0].Verdict)
	}
	// The failure detail must carry the REAL observed status (401), not
	// a generic message — proves we surface real wire data.
	if !strings.Contains(rep.Cases[0].Detail, "401") {
		t.Fatalf("failure detail missing real status 401: %q", rep.Cases[0].Detail)
	}
}

// TestRunHTTPBank_MutationWrongBodyFails is a second §1.1 mutation: a
// correct status but a body substring that is NOT present must FAIL.
func TestRunHTTPBank_MutationWrongBodyFails(t *testing.T) {
	srv := newFakeServer(t)
	defer srv.Close()

	exec := autonomous.NewHTTPExecutor(srv.URL)
	cases := caseFromSteps("HXC-AUTH-006-BODYMUT",
		"MUTATION: right status, body substring absent",
		testbank.TestStep{
			Name:               "GET /api/v1/tasks without auth",
			Action:             "http: GET /api/v1/tasks",
			ExpectStatus:       401,
			ExpectBodyContains: "this-substring-is-not-in-the-response",
		},
	)

	var buf bytes.Buffer
	rep := RunHTTPBank(context.Background(), exec, cases, false, &buf)
	if rep.FailedCases != 1 {
		t.Fatalf("body mutation must FAIL: got %+v\n%s", rep, buf.String())
	}
}

// TestRunHTTPBank_NonHTTPCaseSkipped proves a case with no http: steps
// is SKIPPED (not PASS, not FAIL) so non-HTTP banks never pollute the
// exit code.
func TestRunHTTPBank_NonHTTPCaseSkipped(t *testing.T) {
	srv := newFakeServer(t)
	defer srv.Close()

	exec := autonomous.NewHTTPExecutor(srv.URL)
	cases := caseFromSteps("HXC-NONHTTP", "adb-only case",
		testbank.TestStep{
			Name:   "press enter",
			Action: "adb_shell: input keyevent KEYCODE_ENTER",
		},
	)

	var buf bytes.Buffer
	rep := RunHTTPBank(context.Background(), exec, cases, false, &buf)
	if rep.SkippedCases != 1 || rep.PassedCases != 0 || rep.FailedCases != 0 {
		t.Fatalf("expected 1 SKIP, got %+v\n%s", rep, buf.String())
	}
}

// TestRunHTTPBank_LoadsRealBankFromYAML proves the bridge can load the
// actual schema (via a manager) and select http: steps. It uses an
// in-test bank file shape identical to banks/helixcode-auth.yaml's
// case structure.
func TestRunHTTPBank_LoadsRealBankFromYAML(t *testing.T) {
	srv := newFakeServer(t)
	defer srv.Close()

	yaml := `version: "1.0"
name: "fixture auth bank"
test_cases:
  - id: FIX-001
    name: "protected route requires auth"
    category: security
    priority: critical
    platforms: [api]
    steps:
      - name: "GET /api/v1/tasks without auth"
        action: "http: GET /api/v1/tasks"
        expect_status: 401
        expect_body_contains: "Authorization header required"
`
	dir := t.TempDir()
	path := dir + "/fixture.yaml"
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	mgr := testbank.NewManager()
	if err := mgr.LoadFile(path); err != nil {
		t.Fatalf("load bank: %v", err)
	}
	if got := len(mgr.All()); got != 1 {
		t.Fatalf("expected 1 case loaded, got %d", got)
	}

	exec := autonomous.NewHTTPExecutor(srv.URL)
	var buf bytes.Buffer
	rep := RunHTTPBank(context.Background(), exec, mgr.All(), false, &buf)
	if rep.PassedCases != 1 {
		t.Fatalf("expected 1 PASS from loaded bank, got %+v\n%s", rep, buf.String())
	}
}

// TestParseMethodPath covers the tolerant METHOD/PATH split.
func TestParseMethodPath(t *testing.T) {
	cases := []struct{ in, m, p string }{
		{"GET /api/v1/tasks", "GET", "/api/v1/tasks"},
		{"POST /api/v1/auth/login", "POST", "/api/v1/auth/login"},
		{"/health", "GET", "/health"},
		{"", "", ""},
	}
	for _, c := range cases {
		m, p := parseMethodPath(c.in)
		if m != c.m || p != c.p {
			t.Errorf("parseMethodPath(%q) = (%q,%q), want (%q,%q)", c.in, m, p, c.m, c.p)
		}
	}
}

// Guard: config.ParseBanks is the splitter the command relies on.
func TestParseBanksSplitterUsed(t *testing.T) {
	got := config.ParseBanks("a.yaml,b.yaml")
	if len(got) != 2 {
		t.Fatalf("ParseBanks split = %v, want 2 entries", got)
	}
}
