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
// Fake DarwinFetcher
// ---------------------------------------------------------------------------

type fakeDarwinFetcher struct {
	json    []byte
	err     error
	closeFn int
}

func (f *fakeDarwinFetcher) Fetch(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.json, nil
}

func (f *fakeDarwinFetcher) Close() error { f.closeFn++; return nil }

// ---------------------------------------------------------------------------
// Canned macOS AX dump — mirrors the Swift sidecar's JSON output.
// ---------------------------------------------------------------------------

const sampleDarwinDump = `{
  "role": "AXApplication",
  "title": "HelixQA",
  "description": "HelixQA",
  "value": "",
  "identifier": "com.example.helixqa",
  "enabled": true,
  "focused": false,
  "selected": false,
  "frame": {"x": 0, "y": 0, "width": 1440, "height": 900},
  "children": [
    {
      "role": "AXWindow",
      "title": "Main",
      "description": "",
      "identifier": "main-window",
      "frame": {"x": 100, "y": 100, "width": 1240, "height": 700},
      "enabled": true,
      "focused": true,
      "children": [
        {
          "role": "AXButton",
          "title": "Sign In",
          "description": "Primary action",
          "identifier": "login-btn",
          "frame": {"x": 500, "y": 400, "width": 240, "height": 60},
          "enabled": true,
          "focused": false,
          "selected": false,
          "children": []
        },
        {
          "role": "AXTextField",
          "title": "",
          "description": "Username",
          "value": "admin",
          "identifier": "username-field",
          "frame": {"x": 200, "y": 200, "width": 840, "height": 30},
          "enabled": true,
          "focused": true,
          "selected": false,
          "children": []
        }
      ]
    }
  ]
}`

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestDarwinSnapshotter_Snapshot_FullTree(t *testing.T) {
	snap := NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{json: []byte(sampleDarwinDump)})
	defer snap.Close()

	root, err := snap.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if root.Platform != PlatformDarwin {
		t.Fatalf("Platform = %q, want %q", root.Platform, PlatformDarwin)
	}
	if root.Role != "application" {
		t.Fatalf("root role = %q, want application", root.Role)
	}
	if root.RawID != "com.example.helixqa" {
		t.Fatalf("root RawID = %q", root.RawID)
	}
	if c := root.CountDescendants(); c != 4 {
		t.Fatalf("descendants = %d, want 4", c)
	}
}

func TestDarwinSnapshotter_Snapshot_AttributesTranslated(t *testing.T) {
	snap := NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{json: []byte(sampleDarwinDump)})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())

	btn := root.Find(func(n *Node) bool { return n.Role == "button" })
	if btn == nil {
		t.Fatal("button not found")
	}
	if btn.Name != "Sign In" {
		t.Fatalf("button Name = %q, want 'Sign In' (AXTitle precedence)", btn.Name)
	}
	if btn.RawID != "login-btn" {
		t.Fatalf("button RawID = %q", btn.RawID)
	}
	if !btn.Enabled {
		t.Fatal("button should be enabled")
	}
	if btn.Bounds != image.Rect(500, 400, 740, 460) {
		t.Fatalf("button bounds = %v, want (500,400)-(740,460)", btn.Bounds)
	}

	tf := root.Find(func(n *Node) bool { return n.Role == "textbox" })
	if tf == nil {
		t.Fatal("textbox not found")
	}
	// AXTitle is empty → Name falls back to AXDescription "Username".
	if tf.Name != "Username" {
		t.Fatalf("textbox Name = %q, want 'Username' (description fallback)", tf.Name)
	}
	if tf.Value != "admin" {
		t.Fatalf("textbox Value = %q", tf.Value)
	}
	if !tf.Focused {
		t.Fatal("textbox should be focused")
	}
}

func TestDarwinSnapshotter_Snapshot_RawIDFallsBackToRoleTitle(t *testing.T) {
	dump := `{
  "role": "AXButton",
  "title": "Close",
  "frame": {"x": 0, "y": 0, "width": 40, "height": 40},
  "enabled": true
}`
	snap := NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{json: []byte(dump)})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	if root.RawID != "AXButton:Close" {
		t.Fatalf("RawID fallback = %q, want AXButton:Close", root.RawID)
	}
}

// ---------------------------------------------------------------------------
// Array vs object top-level tolerance
// ---------------------------------------------------------------------------

func TestDarwinSnapshotter_Snapshot_JSONArrayWrapped(t *testing.T) {
	arr := `[
    {"role":"AXWindow","title":"Main","frame":{"x":0,"y":0,"width":1440,"height":900},"enabled":true,"children":[]},
    {"role":"AXDialog","title":"Confirm","frame":{"x":500,"y":400,"width":400,"height":200},"enabled":true,"children":[]}
  ]`
	snap := NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{json: []byte(arr)})
	defer snap.Close()
	root, err := snap.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if root.Role != "application" {
		t.Fatalf("wrapper role = %q, want application", root.Role)
	}
	if len(root.Children) != 2 {
		t.Fatalf("wrapper children = %d, want 2", len(root.Children))
	}
}

func TestDarwinSnapshotter_Snapshot_SingleElementArrayUnwrapped(t *testing.T) {
	arr := `[{"role":"AXWindow","title":"Main","frame":{"x":0,"y":0,"width":1440,"height":900},"enabled":true,"children":[]}]`
	snap := NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{json: []byte(arr)})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	if root.Role != "window" {
		t.Fatalf("single-element array should return the element directly, not wrap. Role = %q", root.Role)
	}
}

// ---------------------------------------------------------------------------
// macOS AXRole → ARIA mapping
// ---------------------------------------------------------------------------

func TestDarwinRoleToARIA_KnownRoles(t *testing.T) {
	cases := map[string]string{
		"AXApplication":        "application",
		"AXWindow":             "window",
		"AXSheet":              "dialog",
		"AXDialog":             "dialog",
		"AXPopover":            "dialog",
		"AXButton":             "button",
		"AXPopUpButton":        "button",
		"AXDisclosureTriangle": "button",
		"AXStaticText":         "text",
		"AXTextField":          "textbox",
		"AXSecureTextField":    "textbox",
		"AXSearchField":        "textbox",
		"AXTextArea":           "textbox",
		"AXImage":              "image",
		"AXCheckBox":           "checkbox",
		"AXRadioButton":        "radio",
		"AXSwitch":             "switch",
		"AXProgressIndicator":  "progressbar",
		"AXSlider":             "slider",
		"AXStepper":            "spinbutton",
		"AXComboBox":           "combobox",
		"AXList":               "list",
		"AXOutline":            "list",
		"AXTable":              "table",
		"AXRow":                "row",
		"AXCell":               "cell",
		"AXColumn":             "columnheader",
		"AXGroup":              "group",
		"AXScrollArea":         "list",
		"AXToolbar":            "toolbar",
		"AXMenuBar":            "menubar",
		"AXMenu":               "menu",
		"AXMenuItem":           "menuitem",
		"AXMenuButton":         "menuitem",
		"AXTabGroup":           "tablist",
		"AXRadioGroup":         "radiogroup",
		"AXLink":               "link",
		"AXWebArea":            "document",
		"AXBrowser":            "document",
		"AXSplitGroup":         "group",
		"AXSplitter":           "separator",
		"AXDrawer":             "complementary",
		"AXHelpTag":            "tooltip",
	}
	for in, want := range cases {
		if got := darwinRoleToARIA(in); got != want {
			t.Errorf("darwinRoleToARIA(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDarwinRoleToARIA_UnknownStripsAXPrefix(t *testing.T) {
	// Unknown AX role → lowercase of the stripped name (AXFoo → foo).
	if got := darwinRoleToARIA("AXCustomWidget"); got != "customwidget" {
		t.Fatalf("unknown role fallback = %q, want customwidget", got)
	}
}

func TestDarwinRoleToARIA_NonAXPrefixPassesThrough(t *testing.T) {
	if got := darwinRoleToARIA("Unusual"); got != "Unusual" {
		t.Fatalf("non-AX role = %q", got)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestDarwinSnapshotter_Snapshot_EmptyDumpError(t *testing.T) {
	snap := NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{json: []byte("")})
	if _, err := snap.Snapshot(context.Background()); !errors.Is(err, ErrNoRoot) {
		t.Fatalf("empty: %v, want ErrNoRoot", err)
	}
}

func TestDarwinSnapshotter_Snapshot_EmptyArrayError(t *testing.T) {
	snap := NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{json: []byte("[]")})
	if _, err := snap.Snapshot(context.Background()); !errors.Is(err, ErrNoRoot) {
		t.Fatalf("empty array: %v, want ErrNoRoot", err)
	}
}

func TestDarwinSnapshotter_Snapshot_MalformedJSON(t *testing.T) {
	snap := NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{json: []byte("{not json")})
	if _, err := snap.Snapshot(context.Background()); err == nil {
		t.Fatal("malformed should fail")
	}
}

func TestDarwinSnapshotter_Snapshot_MalformedArray(t *testing.T) {
	snap := NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{json: []byte("[malformed")})
	if _, err := snap.Snapshot(context.Background()); err == nil {
		t.Fatal("malformed array should fail")
	}
}

func TestDarwinSnapshotter_Snapshot_UnexpectedJSONKind(t *testing.T) {
	snap := NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{json: []byte(`"just a string"`)})
	if _, err := snap.Snapshot(context.Background()); err == nil {
		t.Fatal("scalar should fail")
	}
}

func TestDarwinSnapshotter_Snapshot_FetcherError(t *testing.T) {
	snap := NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{err: errors.New("sidecar down")})
	if _, err := snap.Snapshot(context.Background()); err == nil {
		t.Fatal("fetcher error should propagate")
	}
}

func TestDarwinSnapshotter_Snapshot_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snap := NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{json: []byte(sampleDarwinDump)})
	if _, err := snap.Snapshot(ctx); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestDarwinSnapshotter_Close_Idempotent(t *testing.T) {
	f := &fakeDarwinFetcher{json: []byte(sampleDarwinDump)}
	snap := NewDarwinSnapshotterWithFetcher(f)
	snap.Close()
	snap.Close()
	if f.closeFn != 1 {
		t.Fatalf("fetcher.Close called %d times, want 1", f.closeFn)
	}
}

func TestDarwinSnapshotter_Snapshot_AfterCloseReturnsError(t *testing.T) {
	snap := NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{json: []byte(sampleDarwinDump)})
	snap.Close()
	if _, err := snap.Snapshot(context.Background()); err != ErrSnapshotClosed {
		t.Fatalf("post-Close = %v, want ErrSnapshotClosed", err)
	}
}

// ---------------------------------------------------------------------------
// DarwinHTTPFetcher — live HTTP via httptest
// ---------------------------------------------------------------------------

func TestDarwinHTTPFetcher_ReturnsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/snapshot" {
			http.Error(w, "nope", http.StatusNotFound)
			return
		}
		w.Write([]byte(sampleDarwinDump))
	}))
	defer srv.Close()

	f := &DarwinHTTPFetcher{URL: srv.URL + "/snapshot"}
	body, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(string(body), "HelixQA") {
		t.Fatalf("body missing marker: %q", string(body)[:50])
	}
}

func TestDarwinHTTPFetcher_HTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "sidecar busy", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	f := &DarwinHTTPFetcher{URL: srv.URL}
	_, err := f.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("HTTP 503 not propagated: %v", err)
	}
}

func TestDarwinHTTPFetcher_EmptyURLError(t *testing.T) {
	f := &DarwinHTTPFetcher{}
	if _, err := f.Fetch(context.Background()); err == nil {
		t.Fatal("empty URL must error")
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close = %v, want nil", err)
	}
}

func TestDarwinHTTPFetcher_InvalidURLError(t *testing.T) {
	f := &DarwinHTTPFetcher{URL: "ht!tp://bad\x00url"}
	if _, err := f.Fetch(context.Background()); err == nil {
		t.Fatal("invalid URL must error")
	}
}

// ---------------------------------------------------------------------------
// End-to-end: DarwinSnapshotter over a real httptest-backed HTTP fetcher.
// ---------------------------------------------------------------------------

func TestDarwinSnapshotter_EndToEndViaHTTPFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sampleDarwinDump))
	}))
	defer srv.Close()

	snap := NewDarwinSnapshotter(srv.URL)
	defer snap.Close()
	root, err := snap.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if root.Role != "application" {
		t.Fatalf("root role = %q", root.Role)
	}
}

// ---------------------------------------------------------------------------
// Factory constructor + interface conformance
// ---------------------------------------------------------------------------

func TestDarwinSnapshotter_SatisfiesSnapshotterInterface(t *testing.T) {
	var s Snapshotter = NewDarwinSnapshotterWithFetcher(&fakeDarwinFetcher{json: []byte(sampleDarwinDump)})
	defer s.Close()
	if _, err := s.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot via interface: %v", err)
	}
}
