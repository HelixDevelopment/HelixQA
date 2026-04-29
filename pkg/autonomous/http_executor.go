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

	mu          sync.Mutex
	tokenCache  map[string]string // creds-key → bearer token
	lastResponse []byte           // for ActionTypeAssert follow-ups
	lastStatus   int
	lastHeaders  http.Header
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
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		LoginPath:  "/api/v1/auth/login",
		TokenField: "session_token",
		tokenCache: map[string]string{},
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

	// Execute
	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		return ActionResult{Success: false, Message: fmt.Sprintf("http: request failed: %v", err)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

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

	var creds Credentials
	credsKey := mode
	switch {
	case strings.EqualFold(mode, "admin"):
		creds = h.AdminCreds
		if creds.Username == "" {
			creds = Credentials{Username: "admin", Password: "admin123"}
		}
	case strings.HasPrefix(mode, "as:"):
		user := strings.TrimSpace(mode[len("as:"):])
		var ok bool
		creds, ok = h.UserCredentials[user]
		if !ok {
			return fmt.Errorf("auth as:%s — credentials not registered", user)
		}
	default:
		return fmt.Errorf("unknown AuthMode %q (expected: none|admin|as:<user>|raw:<token>)", mode)
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

func (h *HTTPExecutor) login(ctx context.Context, creds Credentials) (string, error) {
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed status=%d body=%s", resp.StatusCode, truncateOutput(respBody, 200))
	}
	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("login response decode: %w", err)
	}
	tok, _ := decoded[h.TokenField].(string)
	if tok == "" {
		return "", fmt.Errorf("login response missing field %q", h.TokenField)
	}
	return tok, nil
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
