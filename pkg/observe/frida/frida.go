// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package frida is the HelixQA client for the Frida HTTP bridge
// sidecar (cmd/helixqa-frida-bridge/, future). Frida
// (https://frida.re) is a dynamic-instrumentation toolkit used in
// HelixQA Phase-5 to:
//
//   - hook sensitive app APIs during QA sessions and log every
//     invocation (credential fields, clipboard access, intent
//     broadcasts, WebView URLs) for security + behavior review.
//   - inject JavaScript test drivers into native apps without
//     modifying the app under test.
//
// Frida's native wire protocol (frida-server on port 27042) is a
// custom D-Bus-like binary format. Rather than reimplement it in Go,
// HelixQA uses a thin Python sidecar (cmd/helixqa-frida-bridge/,
// future) that speaks the native wire server-side and exposes an
// HTTP/JSON API client-side. Same sidecar-over-HTTP pattern as
// pkg/agent/omniparser, pkg/vision/text, pkg/nexus/observe/axtree/
// darwin, and axtree/windows.
//
// Wire format:
//
//	POST {endpoint}/sessions       → attaches to a target
//	    body: {"target": "com.example.app", "kind": "package"|"pid"}
//	    → {"session_id": "abc123"}
//
//	POST {endpoint}/sessions/{id}/scripts → loads a JS script
//	    body: {"script": "Interceptor.attach(...)"}
//	    → {"script_id": "xyz"}
//
//	POST {endpoint}/sessions/{id}/scripts/{script_id}/call
//	    body: {"method": "ping", "args": ["a", 1]}
//	    → {"result": <any>}
//
//	DELETE {endpoint}/sessions/{id} → detaches
package frida

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the Frida HTTP bridge client.
type Client struct {
	// Endpoint is the bridge sidecar base URL, e.g.
	// "http://127.0.0.1:17430". Required.
	Endpoint string

	// HTTPClient is the transport. Default 10s timeout — Frida
	// script attach can take a few seconds on cold start.
	HTTPClient *http.Client
}

// New returns a Client bound to the given endpoint.
func New(endpoint string) *Client {
	return &Client{Endpoint: endpoint}
}

// AttachKind identifies the target type.
type AttachKind string

const (
	// AttachPackage targets a running Android/iOS app by its bundle
	// identifier (Android: package name, iOS: CFBundleIdentifier).
	AttachPackage AttachKind = "package"
	// AttachPID targets a process by its numeric PID.
	AttachPID AttachKind = "pid"
	// AttachName targets a process by its executable name
	// (desktop/server processes).
	AttachName AttachKind = "name"
)

// Sentinel errors.
var (
	ErrEmptyEndpoint  = errors.New("helixqa/observe/frida: Endpoint not set")
	ErrEmptyTarget    = errors.New("helixqa/observe/frida: attach target must be non-empty")
	ErrEmptyScript    = errors.New("helixqa/observe/frida: script must be non-empty")
	ErrEmptyMethod    = errors.New("helixqa/observe/frida: RPC method must be non-empty")
	ErrEmptySessionID = errors.New("helixqa/observe/frida: session id must be non-empty")
	ErrEmptyScriptID  = errors.New("helixqa/observe/frida: script id must be non-empty")
)

// Session is a live Frida attachment handle returned from Attach.
type Session struct {
	ID string
}

// Script is a loaded Frida JS script handle returned from
// LoadScript.
type Script struct {
	ID string
}

// ---------------------------------------------------------------------------
// Attach / LoadScript / Call / Detach
// ---------------------------------------------------------------------------

// Attach opens a new Frida session against the given target.
func (c *Client) Attach(ctx context.Context, target string, kind AttachKind) (Session, error) {
	if c.Endpoint == "" {
		return Session{}, ErrEmptyEndpoint
	}
	if target == "" {
		return Session{}, ErrEmptyTarget
	}
	if kind == "" {
		kind = AttachPackage
	}
	var reply struct {
		SessionID string `json:"session_id"`
	}
	err := c.postJSON(ctx, "/sessions", map[string]any{
		"target": target,
		"kind":   string(kind),
	}, &reply)
	if err != nil {
		return Session{}, err
	}
	return Session{ID: reply.SessionID}, nil
}

// LoadScript injects the given JavaScript into the session and
// returns the script handle. Scripts are long-running; they remain
// active until Detach or until the target process exits.
func (c *Client) LoadScript(ctx context.Context, sess Session, script string) (Script, error) {
	if c.Endpoint == "" {
		return Script{}, ErrEmptyEndpoint
	}
	if sess.ID == "" {
		return Script{}, ErrEmptySessionID
	}
	if script == "" {
		return Script{}, ErrEmptyScript
	}
	var reply struct {
		ScriptID string `json:"script_id"`
	}
	err := c.postJSON(ctx, fmt.Sprintf("/sessions/%s/scripts", sess.ID), map[string]any{
		"script": script,
	}, &reply)
	if err != nil {
		return Script{}, err
	}
	return Script{ID: reply.ScriptID}, nil
}

// Call invokes a method on the Frida RPC surface exposed by the
// loaded script (`rpc.exports = { ping: () => "pong" }` in the JS
// side becomes `Call(..., "ping", nil)` on the Go side). The reply
// is decoded into out (pass a pointer to the expected type).
func (c *Client) Call(ctx context.Context, sess Session, script Script, method string, args []any, out any) error {
	if c.Endpoint == "" {
		return ErrEmptyEndpoint
	}
	if sess.ID == "" {
		return ErrEmptySessionID
	}
	if script.ID == "" {
		return ErrEmptyScriptID
	}
	if method == "" {
		return ErrEmptyMethod
	}
	var reply struct {
		Result json.RawMessage `json:"result"`
	}
	err := c.postJSON(ctx,
		fmt.Sprintf("/sessions/%s/scripts/%s/call", sess.ID, script.ID),
		map[string]any{"method": method, "args": args},
		&reply,
	)
	if err != nil {
		return err
	}
	if out != nil && len(reply.Result) > 0 {
		if err := json.Unmarshal(reply.Result, out); err != nil {
			return fmt.Errorf("frida: decode result: %w", err)
		}
	}
	return nil
}

// Detach closes the Frida session. Safe to call even if the
// session ID refers to an already-closed session (the bridge
// returns 404 which we interpret as success).
func (c *Client) Detach(ctx context.Context, sess Session) error {
	if c.Endpoint == "" {
		return ErrEmptyEndpoint
	}
	if sess.ID == "" {
		return ErrEmptySessionID
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.Endpoint+"/sessions/"+sess.ID, nil)
	if err != nil {
		return fmt.Errorf("frida: new DELETE request: %w", err)
	}
	client := c.httpClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("frida: DELETE: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil // already detached
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("frida: DELETE HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal: shared POST/JSON helper
// ---------------------------------------------------------------------------

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (c *Client) postJSON(ctx context.Context, path string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("frida: marshal %s: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint+path, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("frida: new request %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("frida: POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("frida: %s HTTP %d: %s", path, resp.StatusCode, string(b))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("frida: decode %s: %w", path, err)
		}
	}
	return nil
}
