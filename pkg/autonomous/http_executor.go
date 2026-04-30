// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"digital.vasic.helixqa/pkg/testbank"
)

// HTTPExecutor performs HTTP requests for ActionTypeHTTP test
// steps and asserts on the response. It is generic — it has no
// knowledge of any specific API surface; the caller supplies a
// BaseURL and per-step TestStep fields specify method, path,
// body, headers, expected status, and JSON-path / body-contains
// assertions.
//
// One HTTPExecutor instance per test session: it caches admin
// session tokens in tokenCache so repeated AuthMode="admin" steps
// don't trigger N login round-trips.
//
// Added 2026-04-29 to close the BLUFF-HELIXQA-BANKS-REWRITE-001
// gap. Before this, HelixQA banks for HTTP-flavoured surfaces
// (full-qa-api.json, full-qa-web.json, atmosphere.json) had to
// use ActionTypeDescription with prose actions like
// "POST /api/v1/auth/login with body {…}" that the executor
// could not run. This executor makes those banks structurally
// executable per Article XI §11.5.
type HTTPExecutor struct {
	// BaseURL is the root URL prepended to every step's path
	// (e.g. http://thinker.local:8092). Required.
	BaseURL string
	// HTTPClient is the underlying *http.Client. Defaults to a
	// 30-second-timeout client if nil.
	HTTPClient *http.Client
	// AdminCreds holds the admin login credentials used by
	// AuthMode="admin" steps. Only populated when at least one
	// step's AuthMode requires login. Empty struct means
	// "admin/admin123" defaults are used.
	AdminCreds Credentials
	// UserCredentials maps username → credentials for AuthMode
	// "as:<user>" steps. Empty by default.
	UserCredentials map[string]Credentials
	// LoginPath is the auth-login endpoint, default
	// "/api/v1/auth/login".
	LoginPath string
	// TokenField is the JSON key in the login response that
	// contains the bearer token, default "session_token".
	TokenField string

	// CSRFPreflightPath, when non-empty, is a safe GET endpoint that
	// the executor calls before any mutating request (POST/PUT/PATCH/
	// DELETE) targeting CSRFGuardedPaths. The catalog-api convention
	// (root_middleware/csrf): GET/HEAD/OPTIONS mints a fresh token
	// returned in the X-CSRF-Token header AND a `csrf` cookie; POST/
	// PUT/DELETE then require both to match. Default
	// "/api/v1/admin/system-info" — picks any /admin/* GET because
	// the same guard runs there.
	CSRFPreflightPath string
	// CSRFGuardedPathPrefixes lists request-path prefixes that are
	// behind the CSRF guard. Default {"/api/v1/admin/"}.
	CSRFGuardedPathPrefixes []string
	// CSRFCookieNames is the ordered list of cookie names the guard
	// might use. The first match wins. catalog-api uses
	// `__Host-csrf` (with the __Host- prefix) over HTTPS and the
	// httptest fixtures use bare "csrf" over HTTP, so the default
	// list contains both.
	CSRFCookieNames []string
	// CSRFHeaderName is the request header name expected by the
	// guard. Default "X-CSRF-Token".
	CSRFHeaderName string

	mu           sync.Mutex
	tokenCache   map[string]string // creds-key → bearer token
	lastResponse []byte            // for ActionTypeAssert follow-ups
	lastStatus   int
	lastHeaders  http.Header
	// csrfToken / csrfCookieName / csrfCookieValue carry the most
	// recent CSRF pair from a preflight GET, reused across mutating
	// calls. csrfCookieName preserves whichever of CSRFCookieNames
	// was actually present in the preflight response, so the
	// replay sets exactly the same cookie name the server expects.
	csrfToken       string
	csrfCookieName  string
	csrfCookieValue string
}

// Credentials is a username + password pair.
type Credentials struct {
	Username string
	Password string
}

// NewHTTPExecutor constructs an HTTPExecutor with sensible
// defaults. baseURL is required; admin defaults to admin/admin123
// if zero.
func NewHTTPExecutor(baseURL string) *HTTPExecutor {
	return &HTTPExecutor{
		BaseURL:                 strings.TrimRight(baseURL, "/"),
		HTTPClient:              &http.Client{Timeout: 30 * time.Second},
		LoginPath:               "/api/v1/auth/login",
		TokenField:              "session_token",
		tokenCache:              map[string]string{},
		CSRFPreflightPath:       "/api/v1/admin/system-info",
		CSRFGuardedPathPrefixes: []string{"/api/v1/admin/"},
		// catalog-api's CSRF guard (root_middleware/csrf.go) sets a
		// `__Host-csrf` cookie (the __Host- prefix requires Secure +
		// no Domain attribute, so the browser binds the cookie to
		// the exact origin). The test fixture in
		// http_executor_test.go uses the bare "csrf" name because
		// httptest.NewServer is HTTP — `__Host-` cookies are
		// rejected over HTTP. Both must be matched, so try the
		// fully-prefixed name first and fall back to "csrf".
		CSRFCookieNames: []string{"__Host-csrf", "csrf"},
		CSRFHeaderName:  "X-CSRF-Token",
	}
}

// Execute runs an ActionTypeHTTP step against BaseURL and applies
// any expectStatus / expectJSONPath / expectBodyContains
// assertions declared on the step. Returns ActionResult so the
// dispatch in performAction can use it the same way other
// executors do.
//
// The caller is responsible for parsing the action value
// ("METHOD PATH") via testbank.TestStep.ParseAction(); this method
// just consumes the (method, path, step) trio.
func (h *HTTPExecutor) Execute(
	ctx context.Context,
	method, path string,
	step testbank.TestStep,
) ActionResult {
	if h.BaseURL == "" {
		return ActionResult{Success: false, Message: "http: BaseURL not configured (set HELIXQA_HTTP_BASE_URL)"}
	}
	// Article XI §11.5: explicit step-level skip honored first.
	// A bank entry can declare _skip: true with _skip_reason to
	// document a deliberate non-execution (destructive operation
	// on shared infrastructure, missing fixture, converter
	// limitation, etc.). This is strictly more honest than letting
	// the request go out and producing a confusing PASS/FAIL.
	if step.Skip {
		reason := step.SkipReason
		if reason == "" {
			reason = "step marked _skip without reason — treat as SKIP-OK: #UNTRIAGED"
		}
		return ActionResult{
			Skipped: true,
			Message: fmt.Sprintf("http: step skipped — SKIP-OK: %s", reason),
		}
	}
	// Article XI §11.5: detect unresolved `{var}` placeholders left
	// over from the bank converter. The bank entry's prose describes
	// "GET /scans/{job_id}" expecting the converter to substitute
	// {job_id} from a prior step's response, but the runtime doesn't
	// yet support response capture / template expansion. Marking
	// these SKIPPED (with explicit reason) is honest — the test
	// can't run yet and a FAIL would be a bluff because the
	// catalog-api isn't actually broken; the harness just lacks the
	// feature.
	if placeholder := unresolvedPlaceholder(path); placeholder != "" {
		return ActionResult{
			Skipped: true,
			Message: fmt.Sprintf("http: unresolved placeholder %s in path — SKIP-OK: #BLUFF-HELIXQA-BANKS-VAR-SUBST-001 (executor lacks response capture / variable expansion; bank converter must hardcode an ID or runtime must implement extract:/template support)", placeholder),
		}
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return ActionResult{Success: false, Message: "http: method missing (use 'http: POST /path' format)"}
	}
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := h.BaseURL + path

	// Build body
	var bodyReader io.Reader
	contentType := ""
	if step.Body != nil {
		switch v := step.Body.(type) {
		case string:
			bodyReader = strings.NewReader(v)
		case []byte:
			bodyReader = bytes.NewReader(v)
		default:
			b, err := json.Marshal(v)
			if err != nil {
				return ActionResult{Success: false, Message: fmt.Sprintf("http: body marshal failed: %v", err)}
			}
			bodyReader = bytes.NewReader(b)
			contentType = "application/json"
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return ActionResult{Success: false, Message: fmt.Sprintf("http: build request failed: %v", err)}
	}
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	for k, v := range step.Headers {
		req.Header.Set(k, v)
	}

	// Auth
	if err := h.applyAuth(ctx, req, step.AuthMode); err != nil {
		return ActionResult{Success: false, Message: fmt.Sprintf("http: auth failed: %v", err)}
	}

	// Article XI §11.5: catalog-api's admin group sits behind a
	// double-submit-cookie CSRF guard (root_middleware/csrf.go). For
	// any mutating method targeting a CSRF-guarded prefix, do a
	// preflight GET to mint a token, capture cookie + header, and
	// replay both on the real call. Without this, every admin POST/
	// PUT/DELETE in the bank fails with 403 "missing csrf cookie".
	// Caught by FQA-API-047, FQA-API-243, FQA-API-252, FQA-API-253.
	if h.needsCSRF(method, path) {
		if err := h.ensureCSRF(ctx, step.AuthMode); err != nil {
			return ActionResult{Success: false, Message: fmt.Sprintf("http: csrf preflight failed: %v", err)}
		}
		h.mu.Lock()
		tok, ckName, ckVal := h.csrfToken, h.csrfCookieName, h.csrfCookieValue
		h.mu.Unlock()
		if tok != "" {
			req.Header.Set(h.CSRFHeaderName, tok)
		}
		if ckName != "" && ckVal != "" {
			req.AddCookie(&http.Cookie{Name: ckName, Value: ckVal})
		}
	}

	// Execute
	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return ActionResult{Success: false, Message: fmt.Sprintf("http: request failed: %v", err)}
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Article XI §11.5: 401 on a request that USED a cached
	// bearer token usually means the cached token was invalidated
	// (e.g. by a previous /auth/logout step in the bank, or
	// catalog-api session expiry). Evict the cache entry and
	// retry the request once with a fresh login. Without this,
	// every admin: call after FQA-API-007 (logout) silently fails
	// with 401 — the bluff scanner would file 14 "phantom"
	// catalog-api defects when the truth is just a stale cache.
	if resp.StatusCode == http.StatusUnauthorized && step.AuthMode != "" &&
		!strings.EqualFold(step.AuthMode, "none") &&
		!strings.HasPrefix(step.AuthMode, "raw:") {
		h.invalidateCachedToken(step.AuthMode)
		// Rebuild the request body reader (the original was consumed).
		var retryBody io.Reader
		if step.Body != nil {
			switch v := step.Body.(type) {
			case string:
				retryBody = strings.NewReader(v)
			case []byte:
				retryBody = bytes.NewReader(v)
			default:
				if b, err := json.Marshal(v); err == nil {
					retryBody = bytes.NewReader(b)
				}
			}
		}
		retryReq, retryErr := http.NewRequestWithContext(ctx, method, url, retryBody)
		if retryErr == nil {
			if contentType != "" {
				retryReq.Header.Set("Content-Type", contentType)
			}
			retryReq.Header.Set("Accept", "application/json")
			for k, v := range step.Headers {
				retryReq.Header.Set(k, v)
			}
			if err := h.applyAuth(ctx, retryReq, step.AuthMode); err == nil {
				if h.needsCSRF(method, path) {
					h.mu.Lock()
					tok, ckName, ckVal := h.csrfToken, h.csrfCookieName, h.csrfCookieValue
					h.mu.Unlock()
					if tok != "" {
						retryReq.Header.Set(h.CSRFHeaderName, tok)
					}
					if ckName != "" && ckVal != "" {
						retryReq.AddCookie(&http.Cookie{Name: ckName, Value: ckVal})
					}
				}
				if retryResp, retryErr := h.HTTPClient.Do(retryReq); retryErr == nil {
					retryBytes, _ := io.ReadAll(retryResp.Body)
					retryResp.Body.Close()
					resp = retryResp
					body = retryBytes
				}
			}
		}
	}

	h.mu.Lock()
	h.lastResponse = body
	h.lastStatus = resp.StatusCode
	h.lastHeaders = resp.Header
	h.mu.Unlock()

	// Assertions
	if step.ExpectStatus != 0 && resp.StatusCode != step.ExpectStatus {
		return ActionResult{
			Success: false,
			Message: fmt.Sprintf("http: %s %s → status %d, expected %d (body: %s)",
				method, path, resp.StatusCode, step.ExpectStatus, truncateOutput(body, 200)),
		}
	}
	if step.ExpectBodyContains != "" && !strings.Contains(string(body), step.ExpectBodyContains) {
		return ActionResult{
			Success: false,
			Message: fmt.Sprintf("http: response body missing %q (body: %s)",
				step.ExpectBodyContains, truncateOutput(body, 200)),
		}
	}
	if step.ExpectJSONPath != "" {
		ok, val, err := jsonPathExists(body, step.ExpectJSONPath)
		if err != nil {
			return ActionResult{Success: false, Message: fmt.Sprintf("http: json_path %q parse error: %v", step.ExpectJSONPath, err)}
		}
		if !ok {
			return ActionResult{Success: false, Message: fmt.Sprintf("http: json_path %q not found in response", step.ExpectJSONPath)}
		}
		// Cache token if the path is the configured TokenField — convenience for chained tests.
		if step.ExpectJSONPath == "$."+h.TokenField {
			if s, ok2 := val.(string); ok2 && s != "" {
				h.mu.Lock()
				h.tokenCache["__last_login__"] = s
				h.mu.Unlock()
			}
		}
	}

	return ActionResult{
		Success: true,
		Message: fmt.Sprintf("http: %s %s → %d (%dB)", method, path, resp.StatusCode, len(body)),
	}
}

// LastResponse returns the most recent response captured by
// Execute, for chained assertions or debugging. Safe for
// concurrent use.
func (h *HTTPExecutor) LastResponse() (status int, headers http.Header, body []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastStatus, h.lastHeaders, h.lastResponse
}

func (h *HTTPExecutor) applyAuth(ctx context.Context, req *http.Request, mode string) error {
	mode = strings.TrimSpace(mode)
	if mode == "" || strings.EqualFold(mode, "none") {
		return nil
	}
	if strings.HasPrefix(mode, "raw:") {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(mode[len("raw:"):]))
		return nil
	}

	creds, credsKey, err := h.resolveCreds(mode)
	if err != nil {
		return err
	}

	h.mu.Lock()
	cached, ok := h.tokenCache[credsKey]
	h.mu.Unlock()
	if ok && cached != "" {
		req.Header.Set("Authorization", "Bearer "+cached)
		return nil
	}

	tok, err := h.login(ctx, creds)
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.tokenCache[credsKey] = tok
	h.mu.Unlock()
	req.Header.Set("Authorization", "Bearer "+tok)
	return nil
}

// resolveCreds maps an AuthMode string ("admin" / "as:<user>") to a
// (Credentials, cache-key) pair. Extracted so applyAuth and the
// 401-retry path in Execute share one source of truth.
func (h *HTTPExecutor) resolveCreds(mode string) (Credentials, string, error) {
	credsKey := mode
	switch {
	case strings.EqualFold(mode, "admin"):
		creds := h.AdminCreds
		if creds.Username == "" {
			creds = Credentials{Username: "admin", Password: "admin123"}
		}
		return creds, credsKey, nil
	case strings.HasPrefix(mode, "as:"):
		user := strings.TrimSpace(mode[len("as:"):])
		creds, ok := h.UserCredentials[user]
		if !ok {
			return Credentials{}, "", fmt.Errorf("auth as:%s — credentials not registered", user)
		}
		return creds, credsKey, nil
	}
	return Credentials{}, "", fmt.Errorf("unknown AuthMode %q (expected: none|admin|as:<user>|raw:<token>)", mode)
}

// invalidateCachedToken evicts the cached bearer for the given
// AuthMode. Used after a 401 on a cached-token request — the
// catalog-api invalidated the session (e.g. /auth/logout was just
// called) and the cache entry is now dead. The next applyAuth call
// will re-login. Article XI §11.5: silently keeping a stale token
// in the cache causes ~all subsequent admin: requests to fail with
// 401, which a reviewer would mistake for a catalog-api defect
// instead of an executor cache-staleness bug.
func (h *HTTPExecutor) invalidateCachedToken(mode string) {
	mode = strings.TrimSpace(mode)
	if mode == "" || strings.EqualFold(mode, "none") || strings.HasPrefix(mode, "raw:") {
		return
	}
	h.mu.Lock()
	delete(h.tokenCache, mode)
	h.mu.Unlock()
}

func (h *HTTPExecutor) login(ctx context.Context, creds Credentials) (string, error) {
	return h.loginWithRetry(ctx, creds, 0)
}

// loginWithRetry performs a login, honoring catalog-api's
// rate-limiter Retry-After header on 429 responses.
//
// Article XI §11.5: Without this, sequential bank verification
// runs hit the login rate limit, the cached admin token gets
// invalidated mid-suite (e.g. by a logout step), the auto-refresh
// path issues a fresh login that gets 429'd, and ALL subsequent
// admin: tests cascade-fail. The 60-second wait is bounded by
// MaxRetryAfter so a misbehaving rate-limiter can't stall a test
// run indefinitely.
//
// Retry depth caps at 1 — a single retry is enough to wait out
// a typical 60-second window. Anything more is a sign the test
// is firing logins faster than the rate limit allows, which is a
// bank-design issue.
func (h *HTTPExecutor) loginWithRetry(ctx context.Context, creds Credentials, depth int) (string, error) {
	const maxRetryAfter = 65 * time.Second
	body, err := json.Marshal(map[string]string{
		"username": creds.Username,
		"password": creds.Password,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.BaseURL+h.LoginPath, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	body2, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests && depth == 0 {
		retryAfter := parseRetryAfter(resp, body2)
		if retryAfter > maxRetryAfter {
			retryAfter = maxRetryAfter
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(retryAfter):
		}
		return h.loginWithRetry(ctx, creds, depth+1)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login failed status=%d body=%s", resp.StatusCode, truncateOutput(body2, 200))
	}
	var decoded map[string]any
	if err := json.Unmarshal(body2, &decoded); err != nil {
		return "", fmt.Errorf("login response decode: %w", err)
	}
	tok, _ := decoded[h.TokenField].(string)
	if tok == "" {
		return "", fmt.Errorf("login response missing field %q", h.TokenField)
	}
	return tok, nil
}

// parseRetryAfter extracts the wait duration from a 429
// response. Honors RFC7231: Retry-After header (seconds), and
// falls back to a JSON `retry_after` field in the body, then to
// a 60-second default.
func parseRetryAfter(resp *http.Response, body []byte) time.Duration {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err == nil {
		if v, ok := decoded["retry_after"]; ok {
			switch t := v.(type) {
			case float64:
				if t > 0 {
					return time.Duration(t) * time.Second
				}
			case string:
				if secs, err := strconv.Atoi(t); err == nil && secs > 0 {
					return time.Duration(secs) * time.Second
				}
			}
		}
	}
	return 60 * time.Second
}

// jsonPathExists evaluates a tiny subset of JSON-path expressions
// against body — enough to cover the expectations in HelixQA
// banks: $.foo, $.foo.bar, $.foo[0].bar. Returns
// (found, resolvedValue, err). It deliberately does NOT pull in
// a full JSONPath library — the bank's expectations are simple
// dot/bracket walks and adding a dependency for that would inflate
// the surface area.
func jsonPathExists(body []byte, path string) (bool, any, error) {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "$") {
		return false, nil, fmt.Errorf("path must start with $")
	}
	rest := path[1:] // drop leading $
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return false, nil, fmt.Errorf("body is not JSON: %w", err)
	}
	cur := root
	for rest != "" {
		switch {
		case strings.HasPrefix(rest, "."):
			rest = rest[1:]
			// read until next . or [
			end := strings.IndexAny(rest, ".[")
			var key string
			if end < 0 {
				key, rest = rest, ""
			} else {
				key, rest = rest[:end], rest[end:]
			}
			obj, ok := cur.(map[string]any)
			if !ok {
				return false, nil, nil
			}
			cur, ok = obj[key]
			if !ok {
				return false, nil, nil
			}
		case strings.HasPrefix(rest, "["):
			end := strings.Index(rest, "]")
			if end < 0 {
				return false, nil, fmt.Errorf("unterminated [ in path")
			}
			idx := strings.TrimSpace(rest[1:end])
			rest = rest[end+1:]
			arr, ok := cur.([]any)
			if !ok {
				return false, nil, nil
			}
			var n int
			if _, err := fmt.Sscanf(idx, "%d", &n); err != nil {
				return false, nil, fmt.Errorf("invalid array index %q: %w", idx, err)
			}
			if n < 0 || n >= len(arr) {
				return false, nil, nil
			}
			cur = arr[n]
		default:
			return false, nil, fmt.Errorf("unexpected token in path at %q", rest)
		}
	}
	return cur != nil, cur, nil
}

// parseHTTPAction splits a "METHOD /path" action value into
// method and path, tolerating extra whitespace.
func parseHTTPAction(value string) (method, path string) {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	if len(parts) == 1 {
		return "GET", parts[0]
	}
	return "", ""
}

// unresolvedPlaceholder returns the first `{var}`-style template
// placeholder in s that LOOKS unresolved (i.e. the var name
// matches a known capture-style identifier, not a real path
// segment). Returns "" if no placeholder is found. The check is
// deliberately conservative — a real path segment like
// `{"key":"v"}` in a query string is NOT a placeholder, only
// `/{job_id}` / `/{id}/` / `={smb_root}` patterns count.
//
// We accept the simple heuristic: anything matching `{[a-z_]+}`
// that the bank converter likely emitted as a placeholder.
// Substring tokens like `{` inside JSON query bodies don't reach
// this function — `path` is the URL path component only.
func unresolvedPlaceholder(path string) string {
	open := strings.IndexByte(path, '{')
	if open < 0 {
		return ""
	}
	close := strings.IndexByte(path[open:], '}')
	if close < 0 {
		return ""
	}
	frag := path[open : open+close+1]
	// Empty braces "{}" or single char are not placeholders.
	inner := frag[1 : len(frag)-1]
	if len(inner) == 0 {
		return ""
	}
	// Only treat lowercase + underscore + digits ID-ish names as
	// placeholders. Uppercase, hyphens, dots, etc. are real path
	// segments, not converter placeholders.
	for _, r := range inner {
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return ""
		}
	}
	return frag
}

// needsCSRF reports whether the given (method, path) is behind the
// CSRF guard and therefore requires a token+cookie pair on the
// request.
func (h *HTTPExecutor) needsCSRF(method, path string) bool {
	if h.CSRFPreflightPath == "" {
		return false
	}
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}
	for _, prefix := range h.CSRFGuardedPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// ensureCSRF performs a preflight GET to CSRFPreflightPath and
// captures the X-CSRF-Token header + csrf cookie. Cached for the
// lifetime of the executor — most CSRF guards mint long-lived
// tokens, and a session of bank tests is short enough that we
// don't need refresh logic. Idempotent: returns early if a token
// is already cached.
func (h *HTTPExecutor) ensureCSRF(ctx context.Context, authMode string) error {
	h.mu.Lock()
	have := h.csrfToken != "" && h.csrfCookieValue != ""
	h.mu.Unlock()
	if have {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.BaseURL+h.CSRFPreflightPath, nil)
	if err != nil {
		return fmt.Errorf("build preflight: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if err := h.applyAuth(ctx, req, authMode); err != nil {
		return fmt.Errorf("preflight auth: %w", err)
	}
	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("preflight fetch: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	tok := resp.Header.Get(h.CSRFHeaderName)
	var (
		cookieName  string
		cookieValue string
	)
	// Walk the configured candidate names in order — first match
	// wins. This handles deployments where the guard uses the
	// __Host- prefix (HTTPS) versus a bare cookie name (HTTP test
	// fixture).
	for _, name := range h.CSRFCookieNames {
		for _, c := range resp.Cookies() {
			if c.Name == name {
				cookieName = c.Name
				cookieValue = c.Value
				break
			}
		}
		if cookieName != "" {
			break
		}
	}
	if tok == "" && cookieValue == "" {
		// Guard not active for this deployment (e.g. NewCSRF
		// returned an err and main.go disabled the guard with a
		// warning). Treat as success — the actual call will go
		// through unguarded.
		return nil
	}
	h.mu.Lock()
	h.csrfToken = tok
	h.csrfCookieName = cookieName
	h.csrfCookieValue = cookieValue
	h.mu.Unlock()
	return nil
}
