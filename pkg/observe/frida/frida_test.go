// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package frida

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Mock Frida bridge
// ---------------------------------------------------------------------------

type bridgeState struct {
	attachTarget string
	attachKind   string
	loadedScript string
	calledMethod string
	calledArgs   []any
	detachedID   string
}

// mockBridge spins up an httptest server emulating the Frida HTTP
// bridge. Returns the server + bridgeState handle for assertions.
func mockBridge() (*httptest.Server, *bridgeState) {
	state := &bridgeState{}
	mux := http.NewServeMux()

	// POST /sessions
	mux.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Target string `json:"target"`
			Kind   string `json:"kind"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		state.attachTarget = body.Target
		state.attachKind = body.Kind
		_ = json.NewEncoder(w).Encode(map[string]string{"session_id": "sess-001"})
	})

	// DELETE /sessions/{id} and POST /sessions/{id}/scripts
	mux.HandleFunc("/sessions/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch r.Method {
		case http.MethodDelete:
			id := strings.TrimPrefix(path, "/sessions/")
			state.detachedID = id
			w.WriteHeader(http.StatusNoContent)
			return
		case http.MethodPost:
			if strings.HasSuffix(path, "/scripts") {
				var body struct {
					Script string `json:"script"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				state.loadedScript = body.Script
				_ = json.NewEncoder(w).Encode(map[string]string{"script_id": "script-001"})
				return
			}
			if strings.HasSuffix(path, "/call") {
				var body struct {
					Method string `json:"method"`
					Args   []any  `json:"args"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				state.calledMethod = body.Method
				state.calledArgs = body.Args
				// Echo-style reply: return {"result": "pong-<method>"}.
				_ = json.NewEncoder(w).Encode(map[string]any{"result": "pong-" + body.Method})
				return
			}
		}
		http.NotFound(w, r)
	})

	srv := httptest.NewServer(mux)
	return srv, state
}

// ---------------------------------------------------------------------------
// Attach
// ---------------------------------------------------------------------------

func TestAttach_HappyPath(t *testing.T) {
	srv, state := mockBridge()
	defer srv.Close()
	c := New(srv.URL)
	sess, err := c.Attach(context.Background(), "com.example.app", AttachPackage)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if sess.ID != "sess-001" {
		t.Fatalf("session id = %q", sess.ID)
	}
	if state.attachTarget != "com.example.app" || state.attachKind != "package" {
		t.Fatalf("bridge saw target=%q kind=%q", state.attachTarget, state.attachKind)
	}
}

func TestAttach_DefaultKindIsPackage(t *testing.T) {
	srv, state := mockBridge()
	defer srv.Close()
	c := New(srv.URL)
	_, _ = c.Attach(context.Background(), "com.x", "")
	if state.attachKind != "package" {
		t.Fatalf("kind = %q, want package (default)", state.attachKind)
	}
}

func TestAttach_PIDKind(t *testing.T) {
	srv, state := mockBridge()
	defer srv.Close()
	c := New(srv.URL)
	_, _ = c.Attach(context.Background(), "1234", AttachPID)
	if state.attachKind != "pid" {
		t.Fatalf("kind = %q, want pid", state.attachKind)
	}
}

func TestAttach_EmptyTargetError(t *testing.T) {
	c := New("http://localhost")
	_, err := c.Attach(context.Background(), "", AttachPackage)
	if !errors.Is(err, ErrEmptyTarget) {
		t.Fatalf("err = %v, want ErrEmptyTarget", err)
	}
}

func TestAttach_EmptyEndpointError(t *testing.T) {
	c := &Client{}
	_, err := c.Attach(context.Background(), "x", AttachPackage)
	if !errors.Is(err, ErrEmptyEndpoint) {
		t.Fatalf("err = %v, want ErrEmptyEndpoint", err)
	}
}

// ---------------------------------------------------------------------------
// LoadScript
// ---------------------------------------------------------------------------

func TestLoadScript_HappyPath(t *testing.T) {
	srv, state := mockBridge()
	defer srv.Close()
	c := New(srv.URL)
	sess, _ := c.Attach(context.Background(), "com.x", AttachPackage)
	sc, err := c.LoadScript(context.Background(), sess, `rpc.exports = { ping: () => "pong" }`)
	if err != nil {
		t.Fatalf("LoadScript: %v", err)
	}
	if sc.ID != "script-001" {
		t.Fatalf("script id = %q", sc.ID)
	}
	if !strings.Contains(state.loadedScript, "rpc.exports") {
		t.Fatalf("script body not captured: %q", state.loadedScript)
	}
}

func TestLoadScript_EmptySessionIDError(t *testing.T) {
	c := New("http://localhost")
	_, err := c.LoadScript(context.Background(), Session{}, "x")
	if !errors.Is(err, ErrEmptySessionID) {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadScript_EmptyScriptError(t *testing.T) {
	c := New("http://localhost")
	_, err := c.LoadScript(context.Background(), Session{ID: "s"}, "")
	if !errors.Is(err, ErrEmptyScript) {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadScript_EmptyEndpointError(t *testing.T) {
	c := &Client{}
	_, err := c.LoadScript(context.Background(), Session{ID: "s"}, "x")
	if !errors.Is(err, ErrEmptyEndpoint) {
		t.Fatalf("err = %v", err)
	}
}

// ---------------------------------------------------------------------------
// Call
// ---------------------------------------------------------------------------

func TestCall_HappyPathDecodesResult(t *testing.T) {
	srv, state := mockBridge()
	defer srv.Close()
	c := New(srv.URL)
	sess, _ := c.Attach(context.Background(), "com.x", AttachPackage)
	sc, _ := c.LoadScript(context.Background(), sess, "rpc.exports = {}")
	var result string
	err := c.Call(context.Background(), sess, sc, "ping", []any{"hello", 42}, &result)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result != "pong-ping" {
		t.Fatalf("result = %q, want pong-ping", result)
	}
	if state.calledMethod != "ping" {
		t.Fatalf("bridge saw method=%q", state.calledMethod)
	}
	if len(state.calledArgs) != 2 {
		t.Fatalf("args len = %d, want 2", len(state.calledArgs))
	}
}

func TestCall_NilOutIgnored(t *testing.T) {
	srv, _ := mockBridge()
	defer srv.Close()
	c := New(srv.URL)
	sess, _ := c.Attach(context.Background(), "com.x", AttachPackage)
	sc, _ := c.LoadScript(context.Background(), sess, "rpc.exports = {}")
	// Passing out=nil should succeed without decoding.
	if err := c.Call(context.Background(), sess, sc, "ping", nil, nil); err != nil {
		t.Fatalf("Call with nil out: %v", err)
	}
}

func TestCall_InvalidJSONResult(t *testing.T) {
	// Bridge returns valid-JSON but result doesn't match `out`
	// type — Unmarshal should fail.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/sessions") {
			_ = json.NewEncoder(w).Encode(map[string]string{"session_id": "s"})
			return
		}
		if strings.HasSuffix(path, "/scripts") {
			_ = json.NewEncoder(w).Encode(map[string]string{"script_id": "sc"})
			return
		}
		// Returns {"result": "not-a-number"} when caller expects int.
		_ = json.NewEncoder(w).Encode(map[string]any{"result": "not-a-number"})
	}))
	defer srv.Close()
	c := New(srv.URL)
	sess, _ := c.Attach(context.Background(), "x", AttachPackage)
	sc, _ := c.LoadScript(context.Background(), sess, "x")
	var n int
	err := c.Call(context.Background(), sess, sc, "ping", nil, &n)
	if err == nil || !strings.Contains(err.Error(), "decode result") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestCall_EmptyMethodError(t *testing.T) {
	c := New("http://localhost")
	err := c.Call(context.Background(), Session{ID: "s"}, Script{ID: "sc"}, "", nil, nil)
	if !errors.Is(err, ErrEmptyMethod) {
		t.Fatalf("err = %v, want ErrEmptyMethod", err)
	}
}

func TestCall_EmptyScriptIDError(t *testing.T) {
	c := New("http://localhost")
	err := c.Call(context.Background(), Session{ID: "s"}, Script{}, "m", nil, nil)
	if !errors.Is(err, ErrEmptyScriptID) {
		t.Fatalf("err = %v, want ErrEmptyScriptID", err)
	}
}

func TestCall_EmptySessionIDError(t *testing.T) {
	c := New("http://localhost")
	err := c.Call(context.Background(), Session{}, Script{ID: "sc"}, "m", nil, nil)
	if !errors.Is(err, ErrEmptySessionID) {
		t.Fatalf("err = %v, want ErrEmptySessionID", err)
	}
}

func TestCall_EmptyEndpointError(t *testing.T) {
	c := &Client{}
	err := c.Call(context.Background(), Session{ID: "s"}, Script{ID: "sc"}, "m", nil, nil)
	if !errors.Is(err, ErrEmptyEndpoint) {
		t.Fatalf("err = %v, want ErrEmptyEndpoint", err)
	}
}

// ---------------------------------------------------------------------------
// Detach
// ---------------------------------------------------------------------------

func TestDetach_HappyPath(t *testing.T) {
	srv, state := mockBridge()
	defer srv.Close()
	c := New(srv.URL)
	sess, _ := c.Attach(context.Background(), "com.x", AttachPackage)
	if err := c.Detach(context.Background(), sess); err != nil {
		t.Fatalf("Detach: %v", err)
	}
	if state.detachedID != sess.ID {
		t.Fatalf("bridge saw detach id %q, want %q", state.detachedID, sess.ID)
	}
}

func TestDetach_AlreadyDetachedReturnsNoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	c := New(srv.URL)
	if err := c.Detach(context.Background(), Session{ID: "gone"}); err != nil {
		t.Fatalf("404 should be treated as already-detached: %v", err)
	}
}

func TestDetach_HTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "broken", http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := New(srv.URL)
	err := c.Detach(context.Background(), Session{ID: "s"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("expected HTTP 500, got %v", err)
	}
}

func TestDetach_EmptySessionError(t *testing.T) {
	c := New("http://localhost")
	if err := c.Detach(context.Background(), Session{}); !errors.Is(err, ErrEmptySessionID) {
		t.Fatalf("err = %v", err)
	}
}

func TestDetach_EmptyEndpointError(t *testing.T) {
	c := &Client{}
	if err := c.Detach(context.Background(), Session{ID: "s"}); !errors.Is(err, ErrEmptyEndpoint) {
		t.Fatalf("err = %v", err)
	}
}

func TestDetach_InvalidEndpointURL(t *testing.T) {
	c := &Client{Endpoint: "ht!tp://bad\x00url"}
	if err := c.Detach(context.Background(), Session{ID: "s"}); err == nil {
		t.Fatal("invalid URL must error")
	}
}

// ---------------------------------------------------------------------------
// Transport errors + timeouts
// ---------------------------------------------------------------------------

func TestClient_HTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := New(srv.URL)
	_, err := c.Attach(context.Background(), "x", AttachPackage)
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("expected HTTP 503, got %v", err)
	}
}

func TestClient_MalformedJSONError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	c := New(srv.URL)
	_, err := c.Attach(context.Background(), "x", AttachPackage)
	if err == nil {
		t.Fatal("malformed JSON should fail")
	}
}

func TestClient_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer srv.Close()
	c := New(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Attach(ctx, "x", AttachPackage); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestClient_InvalidEndpointURLError(t *testing.T) {
	c := &Client{Endpoint: "ht!tp://bad\x00url"}
	if _, err := c.Attach(context.Background(), "x", AttachPackage); err == nil {
		t.Fatal("invalid URL should fail")
	}
}

// ---------------------------------------------------------------------------
// Marshal edge case — Call with unmarshallable args
// ---------------------------------------------------------------------------

func TestCall_MarshalErrorPropagates(t *testing.T) {
	c := New("http://localhost")
	// Channels are not JSON-marshallable — Marshal will fail.
	ch := make(chan int)
	err := c.Call(context.Background(), Session{ID: "s"}, Script{ID: "sc"}, "m", []any{ch}, nil)
	if err == nil {
		t.Fatal("unmarshallable arg should fail")
	}
}

// ---------------------------------------------------------------------------
// Custom HTTPClient
// ---------------------------------------------------------------------------

func TestClient_CustomHTTPClient(t *testing.T) {
	srv, _ := mockBridge()
	defer srv.Close()
	c := New(srv.URL)
	c.HTTPClient = &http.Client{Timeout: 1 * time.Second}
	if _, err := c.Attach(context.Background(), "x", AttachPackage); err != nil {
		t.Fatalf("custom HTTPClient: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Ensure the io package is actually used in a read path (keeps
// coverage meaningful across error branches).
// ---------------------------------------------------------------------------

func TestClient_HTTPErrorBodyReadable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server went boom"))
	}))
	defer srv.Close()
	c := New(srv.URL)
	_, err := c.Attach(context.Background(), "x", AttachPackage)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "server went boom") {
		t.Fatalf("body not in error: %v", err)
	}
	// Keep io import referenced.
	_ = io.EOF
}
