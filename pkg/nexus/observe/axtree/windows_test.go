// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package axtree

import (
	"context"
	"errors"
	"image"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Fake WindowsFetcher
// ---------------------------------------------------------------------------

type fakeWindowsFetcher struct {
	json    []byte
	err     error
	closeFn int
}

func (f *fakeWindowsFetcher) Fetch(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.json, nil
}

func (f *fakeWindowsFetcher) Close() error { f.closeFn++; return nil }

// ---------------------------------------------------------------------------
// Canned Windows UIA JSON
// ---------------------------------------------------------------------------

const sampleWindowsDump = `{
  "controlType": "Window",
  "name": "HelixQA",
  "automationId": "main-window",
  "className": "HwndWrapper",
  "value": "",
  "helpText": "",
  "isEnabled": true,
  "hasKeyboardFocus": false,
  "isSelected": false,
  "boundingRectangle": [0, 0, 1920, 1080],
  "children": [
    {
      "controlType": "Pane",
      "name": "",
      "automationId": "root-pane",
      "className": "Pane",
      "isEnabled": true,
      "boundingRectangle": [0, 0, 1920, 80],
      "children": []
    },
    {
      "controlType": "Button",
      "name": "Sign In",
      "automationId": "login-btn",
      "className": "Button",
      "isEnabled": true,
      "hasKeyboardFocus": true,
      "boundingRectangle": [100, 400, 200, 50],
      "children": []
    },
    {
      "controlType": "Edit",
      "name": "",
      "helpText": "Username",
      "value": "admin",
      "automationId": "username-field",
      "className": "Edit",
      "isEnabled": true,
      "isSelected": true,
      "boundingRectangle": [100, 200, 800, 30],
      "children": []
    }
  ]
}`

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestWindowsSnapshotter_Snapshot_FullTree(t *testing.T) {
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte(sampleWindowsDump)})
	defer snap.Close()
	root, err := snap.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if root.Platform != PlatformWindows {
		t.Fatalf("Platform = %q, want %q", root.Platform, PlatformWindows)
	}
	if root.Role != "window" {
		t.Fatalf("root role = %q, want window", root.Role)
	}
	if root.RawID != "main-window" {
		t.Fatalf("RawID = %q", root.RawID)
	}
	if c := root.CountDescendants(); c != 4 {
		t.Fatalf("descendants = %d, want 4", c)
	}
}

func TestWindowsSnapshotter_Snapshot_AttributesTranslated(t *testing.T) {
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte(sampleWindowsDump)})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())

	btn := root.Find(func(n *Node) bool { return n.Role == "button" })
	if btn == nil {
		t.Fatal("button not found")
	}
	if btn.Name != "Sign In" || btn.RawID != "login-btn" || !btn.Focused || !btn.Enabled {
		t.Fatalf("button fields wrong: %+v", btn)
	}
	if btn.Bounds != image.Rect(100, 400, 300, 450) {
		t.Fatalf("button bounds = %v", btn.Bounds)
	}

	// Edit box — Name empty, falls back to HelpText "Username".
	tb := root.Find(func(n *Node) bool { return n.Role == "textbox" })
	if tb == nil || tb.Name != "Username" || tb.Value != "admin" || !tb.Selected {
		t.Fatalf("textbox fields wrong: %+v", tb)
	}
}

func TestWindowsSnapshotter_Snapshot_NamePrecedence(t *testing.T) {
	// Name empty, HelpText empty, AutomationID present → Name falls
	// back to AutomationID.
	dump := `{
  "controlType": "Button",
  "name": "",
  "helpText": "",
  "automationId": "fallback-id",
  "className": "Button",
  "isEnabled": true,
  "boundingRectangle": [0, 0, 40, 40]
}`
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte(dump)})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	if root.Name != "fallback-id" {
		t.Fatalf("Name fallback = %q, want fallback-id", root.Name)
	}
}

func TestWindowsSnapshotter_Snapshot_RawIDFallsBackToClassName(t *testing.T) {
	dump := `{
  "controlType": "Custom",
  "name": "Widget",
  "className": "CustomWidget",
  "isEnabled": true,
  "boundingRectangle": [0, 0, 10, 10]
}`
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte(dump)})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	if root.RawID != "CustomWidget:Widget" {
		t.Fatalf("RawID fallback = %q", root.RawID)
	}
}

// ---------------------------------------------------------------------------
// Array top-level
// ---------------------------------------------------------------------------

func TestWindowsSnapshotter_Snapshot_JSONArrayWrapped(t *testing.T) {
	arr := `[
    {"controlType":"Window","name":"Main","boundingRectangle":[0,0,1920,1080],"isEnabled":true,"children":[]},
    {"controlType":"Window","name":"Overlay","boundingRectangle":[500,400,400,200],"isEnabled":true,"children":[]}
  ]`
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte(arr)})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	if root.Role != "application" {
		t.Fatalf("wrapper role = %q", root.Role)
	}
	if len(root.Children) != 2 {
		t.Fatalf("wrapper children = %d", len(root.Children))
	}
}

func TestWindowsSnapshotter_Snapshot_SingleElementArrayUnwrapped(t *testing.T) {
	arr := `[{"controlType":"Window","name":"Main","boundingRectangle":[0,0,1920,1080],"isEnabled":true,"children":[]}]`
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte(arr)})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	if root.Role != "window" {
		t.Fatalf("single-element array unwrap role = %q", root.Role)
	}
}

// ---------------------------------------------------------------------------
// Windows ControlType → ARIA mapping
// ---------------------------------------------------------------------------

func TestWindowsControlTypeToARIA_SymbolicNames(t *testing.T) {
	cases := map[string]string{
		"Button":      "button",
		"Window":      "window",
		"Pane":        "group",
		"Edit":        "textbox",
		"Text":        "text",
		"Document":    "document",
		"Image":       "image",
		"CheckBox":    "checkbox",
		"RadioButton": "radio",
		"ComboBox":    "combobox",
		"List":        "list",
		"ListItem":    "listitem",
		"Tree":        "tree",
		"TreeItem":    "treeitem",
		"Table":       "table",
		"DataGrid":    "grid",
		"Menu":        "menu",
		"MenuBar":     "menubar",
		"MenuItem":    "menuitem",
		"Tab":         "tablist",
		"TabItem":     "tab",
		"ProgressBar": "progressbar",
		"Slider":      "slider",
		"Spinner":     "spinbutton",
		"ToolBar":     "toolbar",
		"Hyperlink":   "link",
		"Group":       "group",
		"Header":      "header",
		"HeaderItem":  "columnheader",
		"Separator":   "separator",
		"ScrollBar":   "scrollbar",
		"Calendar":    "grid",
		"Custom":      "group",
	}
	for in, want := range cases {
		if got := windowsControlTypeToARIA(in); got != want {
			t.Errorf("%q = %q, want %q", in, got, want)
		}
	}
}

func TestWindowsControlTypeToARIA_NumericIDs(t *testing.T) {
	cases := map[string]string{
		"50000": "button",
		"50002": "checkbox",
		"50003": "combobox",
		"50004": "textbox",
		"50006": "image",
		"50008": "list",
		"50009": "menu",
		"50011": "menuitem",
		"50012": "progressbar",
		"50015": "slider",
		"50018": "tablist",
		"50020": "text",
		"50021": "toolbar",
		"50023": "tree",
		"50026": "group",
		"50030": "document",
		"50032": "window",
		"50033": "group",
		"50038": "grid",
	}
	for in, want := range cases {
		if got := windowsControlTypeToARIA(in); got != want {
			t.Errorf("numeric %q = %q, want %q", in, got, want)
		}
	}
}

func TestWindowsControlTypeToARIA_UnknownPassesThrough(t *testing.T) {
	if got := windowsControlTypeToARIA("UnknownCustomControl"); got != "UnknownCustomControl" {
		t.Fatalf("unknown fallback = %q", got)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestWindowsSnapshotter_Snapshot_EmptyDumpError(t *testing.T) {
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte("")})
	if _, err := snap.Snapshot(context.Background()); !errors.Is(err, ErrNoRoot) {
		t.Fatalf("empty: %v, want ErrNoRoot", err)
	}
}

func TestWindowsSnapshotter_Snapshot_EmptyArrayError(t *testing.T) {
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte("[]")})
	if _, err := snap.Snapshot(context.Background()); !errors.Is(err, ErrNoRoot) {
		t.Fatalf("empty array: %v, want ErrNoRoot", err)
	}
}

func TestWindowsSnapshotter_Snapshot_MalformedJSON(t *testing.T) {
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte("{not json")})
	if _, err := snap.Snapshot(context.Background()); err == nil {
		t.Fatal("malformed JSON should fail")
	}
}

func TestWindowsSnapshotter_Snapshot_MalformedArray(t *testing.T) {
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte("[malformed")})
	if _, err := snap.Snapshot(context.Background()); err == nil {
		t.Fatal("malformed array should fail")
	}
}

func TestWindowsSnapshotter_Snapshot_ScalarJSONError(t *testing.T) {
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte(`"just a string"`)})
	if _, err := snap.Snapshot(context.Background()); err == nil {
		t.Fatal("scalar should fail")
	}
}

func TestWindowsSnapshotter_Snapshot_FetcherError(t *testing.T) {
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{err: errors.New("sidecar down")})
	if _, err := snap.Snapshot(context.Background()); err == nil {
		t.Fatal("fetcher error should propagate")
	}
}

func TestWindowsSnapshotter_Snapshot_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte(sampleWindowsDump)})
	if _, err := snap.Snapshot(ctx); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestWindowsSnapshotter_Close_Idempotent(t *testing.T) {
	f := &fakeWindowsFetcher{json: []byte(sampleWindowsDump)}
	snap := NewWindowsSnapshotterWithFetcher(f)
	snap.Close()
	snap.Close()
	if f.closeFn != 1 {
		t.Fatalf("fetcher.Close called %d times, want 1", f.closeFn)
	}
}

func TestWindowsSnapshotter_Snapshot_AfterCloseReturnsError(t *testing.T) {
	snap := NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte(sampleWindowsDump)})
	snap.Close()
	if _, err := snap.Snapshot(context.Background()); err != ErrSnapshotClosed {
		t.Fatalf("post-Close = %v, want ErrSnapshotClosed", err)
	}
}

// ---------------------------------------------------------------------------
// WindowsHTTPFetcher — httptest end-to-end
// ---------------------------------------------------------------------------

func TestWindowsHTTPFetcher_ReturnsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sampleWindowsDump))
	}))
	defer srv.Close()
	f := &WindowsHTTPFetcher{URL: srv.URL}
	body, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(string(body), "HelixQA") {
		t.Fatal("body missing HelixQA marker")
	}
}

func TestWindowsHTTPFetcher_HTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	f := &WindowsHTTPFetcher{URL: srv.URL}
	_, err := f.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("HTTP 503 not propagated: %v", err)
	}
}

func TestWindowsHTTPFetcher_EmptyURLError(t *testing.T) {
	f := &WindowsHTTPFetcher{}
	if _, err := f.Fetch(context.Background()); err == nil {
		t.Fatal("empty URL must error")
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close = %v, want nil", err)
	}
}

func TestWindowsHTTPFetcher_InvalidURLError(t *testing.T) {
	f := &WindowsHTTPFetcher{URL: "ht!tp://bad\x00url"}
	if _, err := f.Fetch(context.Background()); err == nil {
		t.Fatal("invalid URL must error")
	}
}

func TestWindowsSnapshotter_EndToEndViaHTTPFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sampleWindowsDump))
	}))
	defer srv.Close()
	snap := NewWindowsSnapshotter(srv.URL)
	defer snap.Close()
	root, err := snap.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if root.Role != "window" {
		t.Fatalf("root role = %q", root.Role)
	}
}

// ---------------------------------------------------------------------------
// Factory + interface conformance
// ---------------------------------------------------------------------------

func TestNewWindowsSnapshotter_UsesHTTPFetcher(t *testing.T) {
	s := NewWindowsSnapshotter("http://localhost:17421/snapshot")
	f, ok := s.fetcher.(*WindowsHTTPFetcher)
	if !ok {
		t.Fatalf("fetcher type = %T, want *WindowsHTTPFetcher", s.fetcher)
	}
	if f.URL != "http://localhost:17421/snapshot" {
		t.Fatalf("URL = %q", f.URL)
	}
}

func TestWindowsSnapshotter_SatisfiesSnapshotterInterface(t *testing.T) {
	var s Snapshotter = NewWindowsSnapshotterWithFetcher(&fakeWindowsFetcher{json: []byte(sampleWindowsDump)})
	defer s.Close()
	if _, err := s.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot via interface: %v", err)
	}
}
