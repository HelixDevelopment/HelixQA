// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/testbank"
)

// TestHTTPExecutor_BasicGet verifies a vanilla GET against a real
// loopback server returns Success and surfaces the response body.
func TestHTTPExecutor_BasicGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy","build":"25"}`))
	}))
	defer srv.Close()

	h := NewHTTPExecutor(srv.URL)
	res := h.Execute(context.Background(), "GET", "/health", testbank.TestStep{
		ExpectStatus:   200,
		ExpectJSONPath: "$.status",
	})
	require.True(t, res.Success, "expected success, got: %s", res.Message)

	status, hdr, body := h.LastResponse()
	assert.Equal(t, 200, status)
	assert.Equal(t, "application/json", hdr.Get("Content-Type"))
	assert.Contains(t, string(body), "healthy")
}

// TestHTTPExecutor_PostJSONBody verifies a POST with a structured
// body (map) is JSON-encoded with Content-Type: application/json
// and ExpectStatus enforces.
func TestHTTPExecutor_PostJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var got map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, "admin", got["username"])
		assert.Equal(t, "admin123", got["password"])
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"session_token":"jwt-abc","expires_at":"2026-04-30"}`))
	}))
	defer srv.Close()

	h := NewHTTPExecutor(srv.URL)
	res := h.Execute(context.Background(), "POST", "/api/v1/auth/login", testbank.TestStep{
		Body:           map[string]string{"username": "admin", "password": "admin123"},
		ExpectStatus:   200,
		ExpectJSONPath: "$.session_token",
	})
	require.True(t, res.Success, "expected success, got: %s", res.Message)
}

// TestHTTPExecutor_StatusMismatch verifies ExpectStatus actually
// fails the step (Article XI: assertion has teeth).
func TestHTTPExecutor_StatusMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	h := NewHTTPExecutor(srv.URL)
	res := h.Execute(context.Background(), "GET", "/protected", testbank.TestStep{
		ExpectStatus: 200,
	})
	require.False(t, res.Success, "expected failure on 401 vs 200")
	assert.Contains(t, res.Message, "status 401")
	assert.Contains(t, res.Message, "expected 200")
}

// TestHTTPExecutor_BodyContainsMismatch verifies
// ExpectBodyContains rejects when the substring is absent.
func TestHTTPExecutor_BodyContainsMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	h := NewHTTPExecutor(srv.URL)
	res := h.Execute(context.Background(), "GET", "/foo", testbank.TestStep{
		ExpectBodyContains: "Inception",
	})
	require.False(t, res.Success, "expected failure when 'Inception' missing")
	assert.Contains(t, res.Message, "missing")
}

// TestHTTPExecutor_AdminAuthCachesToken verifies that the first
// AuthMode="admin" step performs a login round-trip and subsequent
// steps reuse the cached token (no second login call).
func TestHTTPExecutor_AdminAuthCachesToken(t *testing.T) {
	loginCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/login":
			loginCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"session_token":"cached-jwt"}`))
		case "/api/v1/users":
			assert.Equal(t, "Bearer cached-jwt", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"users":[{"id":1}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	h := NewHTTPExecutor(srv.URL)
	for i := 0; i < 3; i++ {
		res := h.Execute(context.Background(), "GET", "/api/v1/users", testbank.TestStep{
			AuthMode:     "admin",
			ExpectStatus: 200,
		})
		require.True(t, res.Success, "iter %d: %s", i, res.Message)
	}
	assert.Equal(t, 1, loginCount, "admin login should be cached after first call (got %d calls)", loginCount)
}

// TestHTTPExecutor_RawTokenAuth verifies AuthMode="raw:<token>"
// attaches the token verbatim without a login call.
func TestHTTPExecutor_RawTokenAuth(t *testing.T) {
	loginCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/login" {
			loginCount++
		}
		assert.Equal(t, "Bearer my-static-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	h := NewHTTPExecutor(srv.URL)
	res := h.Execute(context.Background(), "GET", "/api/v1/users", testbank.TestStep{
		AuthMode: "raw:my-static-token",
	})
	require.True(t, res.Success, res.Message)
	assert.Zero(t, loginCount, "raw: token must not trigger a login call")
}

// TestHTTPExecutor_AsUserAuth verifies AuthMode="as:<user>" uses
// the matching credentials from UserCredentials.
func TestHTTPExecutor_AsUserAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/login":
			var got map[string]string
			_ = json.NewDecoder(r.Body).Decode(&got)
			assert.Equal(t, "viewer", got["username"])
			assert.Equal(t, "viewer123", got["password"])
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"session_token":"viewer-jwt"}`))
		default:
			assert.Equal(t, "Bearer viewer-jwt", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	h := NewHTTPExecutor(srv.URL)
	h.UserCredentials = map[string]Credentials{
		"viewer": {Username: "viewer", Password: "viewer123"},
	}
	res := h.Execute(context.Background(), "GET", "/api/v1/media", testbank.TestStep{
		AuthMode: "as:viewer",
	})
	require.True(t, res.Success, res.Message)
}

// TestHTTPExecutor_AsUserUnknownUser verifies AuthMode="as:nope"
// returns a clear error when the user is not registered.
func TestHTTPExecutor_AsUserUnknownUser(t *testing.T) {
	h := NewHTTPExecutor("http://unused")
	res := h.Execute(context.Background(), "GET", "/x", testbank.TestStep{
		AuthMode: "as:ghost",
	})
	require.False(t, res.Success)
	assert.Contains(t, res.Message, "ghost")
	assert.Contains(t, res.Message, "credentials not registered")
}

// TestHTTPExecutor_BadJSONPath verifies an unparseable JSON path
// surfaces a clean error instead of crashing.
func TestHTTPExecutor_BadJSONPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"a":1}`))
	}))
	defer srv.Close()

	h := NewHTTPExecutor(srv.URL)
	res := h.Execute(context.Background(), "GET", "/x", testbank.TestStep{
		ExpectJSONPath: "no_dollar",
	})
	require.False(t, res.Success)
	assert.Contains(t, res.Message, "must start with $")
}

// TestHTTPExecutor_TimeoutPropagatesContext verifies a cancelled
// context aborts the request quickly (no goroutine leak).
func TestHTTPExecutor_TimeoutPropagatesContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response — would block longer than ctx allows.
		select {
		case <-time.After(2 * time.Second):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			return
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	h := NewHTTPExecutor(srv.URL)
	start := time.Now()
	res := h.Execute(ctx, "GET", "/slow", testbank.TestStep{})
	elapsed := time.Since(start)
	require.False(t, res.Success)
	assert.Less(t, elapsed.Milliseconds(), int64(1500),
		"context cancellation must abort within ~timeout, took %v", elapsed)
}

// TestParseHTTPAction covers the action-string parser corner cases.
func TestParseHTTPAction(t *testing.T) {
	tests := []struct{ in, m, p string }{
		{"GET /health", "GET", "/health"},
		{"POST /api/v1/auth/login", "POST", "/api/v1/auth/login"},
		{"  PUT   /x  ", "PUT", "/x"},
		{"/health", "GET", "/health"},
		{"", "", ""},
	}
	for _, tc := range tests {
		m, p := parseHTTPAction(tc.in)
		assert.Equal(t, tc.m, m, "method for %q", tc.in)
		assert.Equal(t, tc.p, p, "path for %q", tc.in)
	}
}

// TestJSONPathExists covers the tiny json-path subset.
func TestJSONPathExists(t *testing.T) {
	body := []byte(`{
		"user": {"id": 7, "name": "alice"},
		"items": [{"id":"a"},{"id":"b"}],
		"empty": null
	}`)
	cases := []struct {
		path     string
		want     bool
		wantVal  any
		wantErr  bool
	}{
		{"$", true, nil, false},
		{"$.user", true, nil, false},
		{"$.user.name", true, "alice", false},
		{"$.user.missing", false, nil, false},
		{"$.items[0].id", true, "a", false},
		{"$.items[1].id", true, "b", false},
		{"$.items[5]", false, nil, false},
		{"$.empty", false, nil, false},
		{"no_dollar", false, nil, true},
	}
	for _, tc := range cases {
		ok, val, err := jsonPathExists(body, tc.path)
		if tc.wantErr {
			assert.Error(t, err, "path %q", tc.path)
			continue
		}
		assert.NoError(t, err, "path %q", tc.path)
		if tc.want {
			assert.True(t, ok, "path %q should exist", tc.path)
			if tc.wantVal != nil {
				assert.Equal(t, tc.wantVal, val, "path %q value", tc.path)
			}
		} else {
			assert.False(t, ok, "path %q should not exist", tc.path)
		}
	}
}

// TestRunAssertion_StatusEq verifies the assertion dispatcher.
func TestRunAssertion_StatusEq(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":42}`))
	}))
	defer srv.Close()
	h := NewHTTPExecutor(srv.URL)
	require.True(t, h.Execute(context.Background(), "POST", "/x", testbank.TestStep{}).Success)

	res := runAssertion(h, "status_eq: 201")
	require.True(t, res.Success, res.Message)

	res = runAssertion(h, "status_eq: 200")
	require.False(t, res.Success)
	assert.Contains(t, res.Message, "got 201")

	res = runAssertion(h, "json_path_eq: $.id = 42")
	require.True(t, res.Success, res.Message)

	res = runAssertion(h, "body_contains: 42")
	require.True(t, res.Success, res.Message)

	res = runAssertion(h, "body_contains: nope")
	require.False(t, res.Success)
}

// TestActionTypeHTTP_RoundTrip verifies the full ParseAction →
// performAction → HTTPExecutor.Execute round trip via a minimal
// performAction harness. (We don't invoke StructuredTestExecutor
// itself because that needs a navigator.ActionExecutor; instead
// we exercise the leaf path that ActionTypeHTTP routes to.)
func TestActionTypeHTTP_RoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/health", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer srv.Close()

	step := testbank.TestStep{
		Action:         "http: GET /api/v1/health",
		ExpectStatus:   200,
		ExpectJSONPath: "$.status",
	}
	at, val := step.ParseAction()
	require.Equal(t, testbank.ActionTypeHTTP, at)
	method, path := parseHTTPAction(val)
	require.Equal(t, "GET", method)
	require.Equal(t, "/api/v1/health", path)

	h := NewHTTPExecutor(srv.URL)
	res := h.Execute(context.Background(), method, path, step)
	require.True(t, res.Success, res.Message)
}

// TestHTTPExecutor_NoBaseURL verifies the executor surfaces a
// clean error when called without configuration (instead of
// silently no-oping or panicking).
func TestHTTPExecutor_NoBaseURL(t *testing.T) {
	h := &HTTPExecutor{}
	res := h.Execute(context.Background(), "GET", "/x", testbank.TestStep{})
	require.False(t, res.Success)
	assert.Contains(t, res.Message, "BaseURL not configured")
}

// AntiBluffVerification: the matching negative — if we DELETE the
// status check from ExpectStatus path, the test that asserted
// "wrong status returns failure" would still pass. The fix:
// re-run TestHTTPExecutor_StatusMismatch after commenting out the
// `if step.ExpectStatus != 0 && resp.StatusCode != step.ExpectStatus`
// block; the test must then FAIL. Verified manually before commit.
func TestHTTPExecutor_AntiBluffMarker(t *testing.T) {
	// This test exists to anchor the manual anti-bluff verification
	// and to ensure the package's intent is clear in code review.
	// It deliberately re-asserts the assertion already covered by
	// TestHTTPExecutor_StatusMismatch.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error":"boom"}`)
	}))
	defer srv.Close()
	h := NewHTTPExecutor(srv.URL)
	res := h.Execute(context.Background(), "GET", "/", testbank.TestStep{ExpectStatus: 200})
	require.False(t, res.Success)
	require.True(t, strings.Contains(res.Message, "status 500"))
}
