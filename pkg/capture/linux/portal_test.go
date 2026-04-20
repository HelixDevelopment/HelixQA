// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	dbus "github.com/godbus/dbus/v5"
)

// fakeCaller records every invocation and returns scripted responses.
type fakeCaller struct {
	mu           sync.Mutex
	portalCalls  []portalCall
	immCalls     []portalCall
	portalResps  []portalResp
	immRespBody  []any
	immRespErr   error
	closed       bool
	closeErr     error
}

type portalCall struct {
	Dest, Path, Iface, Method string
	Args                      []any
}

type portalResp struct {
	status  uint32
	results map[string]any
	err     error
}

func (f *fakeCaller) CallPortal(ctx context.Context, dest, path, iface, method string, args ...any) (uint32, map[string]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.portalCalls = append(f.portalCalls, portalCall{Dest: dest, Path: path, Iface: iface, Method: method, Args: append([]any(nil), args...)})
	if len(f.portalResps) == 0 {
		return 0, map[string]any{}, nil
	}
	r := f.portalResps[0]
	f.portalResps = f.portalResps[1:]
	return r.status, r.results, r.err
}

func (f *fakeCaller) CallImmediate(ctx context.Context, dest, path, iface, method string, args ...any) ([]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.immCalls = append(f.immCalls, portalCall{Dest: dest, Path: path, Iface: iface, Method: method, Args: append([]any(nil), args...)})
	return f.immRespBody, f.immRespErr
}

func (f *fakeCaller) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return f.closeErr
}

// --- CreateSession ---

func TestPortal_CreateSession_Success(t *testing.T) {
	fc := &fakeCaller{
		portalResps: []portalResp{{
			status:  0,
			results: map[string]any{"session_handle": "/org/freedesktop/portal/desktop/session/1"},
		}},
	}
	p := NewPortal(fc)
	sess, err := p.CreateSession(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sess != "/org/freedesktop/portal/desktop/session/1" {
		t.Errorf("session = %q", sess)
	}
	if got := len(fc.portalCalls); got != 1 {
		t.Fatalf("calls = %d", got)
	}
	c := fc.portalCalls[0]
	if c.Dest != portalDestination || c.Path != portalObjectPath || c.Iface != portalScreenCastIface || c.Method != "CreateSession" {
		t.Errorf("wrong call target: %+v", c)
	}
	// Options arg must include handle_token + session_handle_token.
	if len(c.Args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(c.Args))
	}
	opts, ok := c.Args[0].(map[string]dbus.Variant)
	if !ok {
		t.Fatalf("args[0] = %T, want map[string]dbus.Variant", c.Args[0])
	}
	for _, key := range []string{"handle_token", "session_handle_token"} {
		v, present := opts[key]
		if !present {
			t.Errorf("missing %q in CreateSession options", key)
			continue
		}
		if s, ok := v.Value().(string); !ok || !strings.HasPrefix(s, "helixqa") {
			t.Errorf("%q = %v, want helixqa-prefixed string", key, v.Value())
		}
	}
}

func TestPortal_CreateSession_TokensAreUnique(t *testing.T) {
	fc := &fakeCaller{
		portalResps: []portalResp{
			{status: 0, results: map[string]any{"session_handle": "s1"}},
			{status: 0, results: map[string]any{"session_handle": "s2"}},
		},
	}
	p := NewPortal(fc)
	if _, err := p.CreateSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := p.CreateSession(context.Background()); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range fc.portalCalls {
		opts := c.Args[0].(map[string]dbus.Variant)
		tok := opts["handle_token"].Value().(string)
		if seen[tok] {
			t.Errorf("duplicate handle_token: %q", tok)
		}
		seen[tok] = true
	}
}

func TestPortal_CreateSession_StatusNonZero(t *testing.T) {
	fc := &fakeCaller{portalResps: []portalResp{{status: 1, results: map[string]any{}}}}
	p := NewPortal(fc)
	_, err := p.CreateSession(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsUserCancelled(err) {
		t.Errorf("IsUserCancelled = false; err = %v", err)
	}
	// Non-cancelled statuses do not trip IsUserCancelled.
	fc.portalResps = []portalResp{{status: 2, results: map[string]any{}}}
	_, err = p.CreateSession(context.Background())
	if IsUserCancelled(err) {
		t.Error("status=2 should not be reported as user-cancelled")
	}
}

func TestPortal_CreateSession_MissingHandle(t *testing.T) {
	fc := &fakeCaller{portalResps: []portalResp{{status: 0, results: map[string]any{}}}}
	p := NewPortal(fc)
	_, err := p.CreateSession(context.Background())
	if err == nil || !strings.Contains(err.Error(), "session_handle") {
		t.Errorf("got %v", err)
	}
}

func TestPortal_CreateSession_CallerError(t *testing.T) {
	boom := errors.New("dbus down")
	fc := &fakeCaller{portalResps: []portalResp{{err: boom}}}
	p := NewPortal(fc)
	_, err := p.CreateSession(context.Background())
	if !errors.Is(err, boom) {
		t.Errorf("want boom, got %v", err)
	}
}

// --- SelectSources ---

func TestPortal_SelectSources_ArgumentShape(t *testing.T) {
	fc := &fakeCaller{portalResps: []portalResp{{status: 0, results: map[string]any{}}}}
	p := NewPortal(fc)
	opts := SelectSourcesOptions{
		Types:        StreamSourceMonitor | StreamSourceWindow,
		Multiple:     true,
		CursorMode:   CursorEmbedded,
		Persist:      PersistForever,
		RestoreToken: "tok-123",
	}
	if err := p.SelectSources(context.Background(), "/session/42", opts); err != nil {
		t.Fatal(err)
	}
	if len(fc.portalCalls) != 1 {
		t.Fatalf("calls=%d", len(fc.portalCalls))
	}
	c := fc.portalCalls[0]
	if c.Path != "/session/42" || c.Method != "SelectSources" {
		t.Errorf("wrong call: %+v", c)
	}
	o := c.Args[0].(map[string]dbus.Variant)
	wantU := map[string]uint32{
		"types":        uint32(StreamSourceMonitor | StreamSourceWindow),
		"cursor_mode":  uint32(CursorEmbedded),
		"persist_mode": uint32(PersistForever),
	}
	for k, want := range wantU {
		if got, ok := o[k].Value().(uint32); !ok || got != want {
			t.Errorf("option %q: got %v, want %v", k, o[k].Value(), want)
		}
	}
	if m, _ := o["multiple"].Value().(bool); !m {
		t.Errorf("multiple not set")
	}
	if rt, _ := o["restore_token"].Value().(string); rt != "tok-123" {
		t.Errorf("restore_token = %q", rt)
	}
}

func TestPortal_SelectSources_Defaults(t *testing.T) {
	fc := &fakeCaller{portalResps: []portalResp{{status: 0, results: map[string]any{}}}}
	p := NewPortal(fc)
	if err := p.SelectSources(context.Background(), "/s", SelectSourcesOptions{}); err != nil {
		t.Fatal(err)
	}
	o := fc.portalCalls[0].Args[0].(map[string]dbus.Variant)
	if o["types"].Value().(uint32) != uint32(StreamSourceMonitor) {
		t.Errorf("default types != MONITOR")
	}
	if o["cursor_mode"].Value().(uint32) != uint32(CursorHidden) {
		t.Errorf("default cursor_mode != hidden")
	}
	if _, present := o["persist_mode"]; present {
		t.Errorf("persist_mode should be omitted when PersistNever")
	}
	if _, present := o["restore_token"]; present {
		t.Errorf("restore_token should be omitted when empty")
	}
}

func TestPortal_SelectSources_EmptySessionPath(t *testing.T) {
	p := NewPortal(&fakeCaller{})
	if err := p.SelectSources(context.Background(), "", SelectSourcesOptions{}); err == nil {
		t.Error("empty sessionPath should error")
	}
}

// --- Start ---

func TestPortal_Start_DecodesStreams(t *testing.T) {
	streamEntry := []any{
		uint32(42),
		map[string]dbus.Variant{
			"size":        dbus.MakeVariant([]int32{1920, 1080}),
			"source_type": dbus.MakeVariant(uint32(1)),
		},
	}
	fc := &fakeCaller{
		portalResps: []portalResp{{
			status: 0,
			results: map[string]any{
				"streams":       []any{streamEntry},
				"restore_token": "restore-xyz",
			},
		}},
	}
	p := NewPortal(fc)
	res, err := p.Start(context.Background(), "/s", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.RestoreToken != "restore-xyz" {
		t.Errorf("RestoreToken = %q", res.RestoreToken)
	}
	if len(res.Streams) != 1 {
		t.Fatalf("streams = %d", len(res.Streams))
	}
	s := res.Streams[0]
	if s.NodeID != 42 {
		t.Errorf("NodeID = %d", s.NodeID)
	}
	if srcType, ok := s.Metadata["source_type"].(uint32); !ok || srcType != 1 {
		t.Errorf("source_type in metadata = %v", s.Metadata["source_type"])
	}
}

func TestPortal_Start_NoStreams(t *testing.T) {
	fc := &fakeCaller{portalResps: []portalResp{{
		status:  0,
		results: map[string]any{"streams": []any{}},
	}}}
	p := NewPortal(fc)
	_, err := p.Start(context.Background(), "/s", "")
	if err == nil || !strings.Contains(err.Error(), "no streams") {
		t.Errorf("want 'no streams', got %v", err)
	}
}

func TestPortal_Start_ParentWindowPassedThrough(t *testing.T) {
	fc := &fakeCaller{
		portalResps: []portalResp{{
			status: 0,
			results: map[string]any{
				"streams": []any{[]any{uint32(1), map[string]dbus.Variant{}}},
			},
		}},
	}
	p := NewPortal(fc)
	if _, err := p.Start(context.Background(), "/s", "x11:0x12345"); err != nil {
		t.Fatal(err)
	}
	if got := fc.portalCalls[0].Args[0]; got != "x11:0x12345" {
		t.Errorf("parentWindow arg = %v, want x11:0x12345", got)
	}
}

// --- OpenPipeWireRemote ---

func TestPortal_OpenPipeWireRemote_ReturnsFile(t *testing.T) {
	// Generate a real FD (a pipe read-end) so os.NewFile wraps something
	// valid.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	// Close read-end at cleanup; portal.OpenPipeWireRemote owns the FD it
	// receives and its returned *os.File holds it. We'll explicitly .Close
	// the returned file below.
	defer r.Close()
	defer w.Close()

	fc := &fakeCaller{
		immRespBody: []any{dbus.UnixFD(int32(w.Fd()))},
	}
	p := NewPortal(fc)
	f, err := p.OpenPipeWireRemote(context.Background(), "/session")
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatal("nil file")
	}
	if uintptr(f.Fd()) != w.Fd() {
		t.Errorf("fd mismatch: %d vs %d", f.Fd(), w.Fd())
	}
	// Don't close f — it duplicates the underlying FD; the test's deferred
	// Close on w covers cleanup.
}

func TestPortal_OpenPipeWireRemote_WrongBodyType(t *testing.T) {
	fc := &fakeCaller{immRespBody: []any{"not-an-fd"}}
	p := NewPortal(fc)
	_, err := p.OpenPipeWireRemote(context.Background(), "/session")
	if err == nil || !strings.Contains(err.Error(), "UnixFD") {
		t.Errorf("want UnixFD type error, got %v", err)
	}
}

func TestPortal_OpenPipeWireRemote_EmptyBody(t *testing.T) {
	fc := &fakeCaller{immRespBody: nil}
	p := NewPortal(fc)
	_, err := p.OpenPipeWireRemote(context.Background(), "/session")
	if err == nil || !strings.Contains(err.Error(), "no body") {
		t.Errorf("want empty-body error, got %v", err)
	}
}

func TestPortal_OpenPipeWireRemote_EmptySession(t *testing.T) {
	p := NewPortal(&fakeCaller{})
	if _, err := p.OpenPipeWireRemote(context.Background(), ""); err == nil {
		t.Error("empty sessionPath should error")
	}
}

// --- Close ---

func TestPortal_Close_CallsCallerClose(t *testing.T) {
	fc := &fakeCaller{}
	p := NewPortal(fc)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if !fc.closed {
		t.Error("caller.Close not invoked")
	}
}

func TestPortal_Close_Nil(t *testing.T) {
	var p *Portal
	if err := p.Close(); err != nil {
		t.Errorf("nil receiver close: %v", err)
	}
}

// --- parseStreams handles both raw-interface and already-typed inputs ---

func TestParseStreams_AlreadyTyped(t *testing.T) {
	in := []Stream{{NodeID: 1}, {NodeID: 2}}
	got := parseStreams(in)
	if len(got) != 2 || got[0].NodeID != 1 {
		t.Errorf("got %+v", got)
	}
}

func TestParseStreams_UnknownType(t *testing.T) {
	if got := parseStreams("not-a-slice"); got != nil {
		t.Errorf("want nil, got %v", got)
	}
}

func TestParseStreams_SkipsMalformedEntries(t *testing.T) {
	in := []any{
		"not a tuple",
		[]any{uint32(1), map[string]dbus.Variant{}},  // valid
		[]any{"wrong-type-for-nodeid", map[string]dbus.Variant{}},
		[]any{uint32(2)}, // tuple too short
	}
	got := parseStreams(in)
	if len(got) != 1 || got[0].NodeID != 1 {
		t.Errorf("got %+v", got)
	}
}

func TestDecodeVariantMap(t *testing.T) {
	in := map[string]dbus.Variant{
		"a": dbus.MakeVariant(uint32(5)),
		"b": dbus.MakeVariant("hello"),
	}
	got := decodeVariantMap(in)
	if got["a"].(uint32) != 5 || got["b"].(string) != "hello" {
		t.Errorf("got %+v", got)
	}
	// Nil / wrong type -> empty map (non-nil).
	if got := decodeVariantMap("nope"); got == nil || len(got) != 0 {
		t.Errorf("unexpected: %v", got)
	}
}
