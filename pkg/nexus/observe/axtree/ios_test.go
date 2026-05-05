// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package axtree

import (
	"context"
	"errors"
	"image"
	"testing"
)

// ---------------------------------------------------------------------------
// Fake IDBDumper
// ---------------------------------------------------------------------------

type fakeIDBDumper struct {
	json     []byte
	err      error
	closeFn  int
}

func (f *fakeIDBDumper) Dump(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.json, nil
}

func (f *fakeIDBDumper) Close() error { f.closeFn++; return nil }

// ---------------------------------------------------------------------------
// Canned idb dump — mirrors what `idb ui describe-all` produces.
// ---------------------------------------------------------------------------

const sampleIDBDump = `{
  "type": "Application",
  "title": "HelixQA Demo",
  "label": "",
  "value": "",
  "identifier": "com.example.helixqa",
  "enabled": true,
  "focused": false,
  "selected": false,
  "frame": {"x": 0, "y": 0, "width": 375, "height": 667},
  "children": [
    {
      "type": "NavigationBar",
      "title": "Home",
      "label": "Navigation bar",
      "frame": {"x": 0, "y": 20, "width": 375, "height": 44},
      "enabled": true,
      "children": []
    },
    {
      "type": "Button",
      "title": "Sign In",
      "label": "Sign In button",
      "identifier": "login-button",
      "frame": {"x": 100, "y": 400, "width": 175, "height": 50},
      "enabled": true,
      "focused": true,
      "children": []
    },
    {
      "type": "TextField",
      "title": "",
      "label": "Username",
      "value": "admin",
      "identifier": "username-field",
      "frame": {"x": 20, "y": 200, "width": 335, "height": 44},
      "enabled": true,
      "selected": true,
      "children": []
    }
  ]
}`

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestIOSSnapshotter_Snapshot_FullTree(t *testing.T) {
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte(sampleIDBDump)})
	defer snap.Close()

	root, err := snap.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if root.Platform != PlatformIOS {
		t.Fatalf("Platform = %q, want %q", root.Platform, PlatformIOS)
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

func TestIOSSnapshotter_Snapshot_AttributesTranslated(t *testing.T) {
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte(sampleIDBDump)})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())

	btn := root.Find(func(n *Node) bool { return n.Role == "button" })
	if btn == nil {
		t.Fatal("button not found")
	}
	if btn.Name != "Sign In button" {
		t.Fatalf("button name = %q, want 'Sign In button' (label takes precedence over title)", btn.Name)
	}
	if btn.RawID != "login-button" {
		t.Fatalf("button RawID = %q", btn.RawID)
	}
	if !btn.Enabled || !btn.Focused {
		t.Fatalf("button flags wrong: %+v", btn)
	}
	if btn.Bounds != image.Rect(100, 400, 275, 450) {
		t.Fatalf("button bounds = %v, want (100,400)-(275,450)", btn.Bounds)
	}

	tf := root.Find(func(n *Node) bool { return n.Role == "textbox" })
	if tf == nil || tf.Name != "Username" || tf.Value != "admin" || !tf.Selected {
		t.Fatalf("textfield wrong: %+v", tf)
	}
}

func TestIOSSnapshotter_Snapshot_NameFallsBackToTitle(t *testing.T) {
	// idb may emit a node without a label — Name should fall back
	// to title.
	dump := `{
  "type": "Button",
  "title": "OK",
  "label": "",
  "frame": {"x": 0, "y": 0, "width": 50, "height": 30},
  "enabled": true
}`
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte(dump)})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	if root.Name != "OK" {
		t.Fatalf("Name fallback = %q, want OK", root.Name)
	}
}

func TestIOSSnapshotter_Snapshot_RawIDFallsBackToTypeTitle(t *testing.T) {
	// No identifier → fallback to "type:title" so action resolution
	// has some handle.
	dump := `{
  "type": "Button",
  "title": "Close",
  "frame": {"x": 0, "y": 0, "width": 40, "height": 40},
  "enabled": true
}`
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte(dump)})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	if root.RawID != "Button:Close" {
		t.Fatalf("RawID fallback = %q, want Button:Close", root.RawID)
	}
}

// ---------------------------------------------------------------------------
// Multiple top-level windows
// ---------------------------------------------------------------------------

func TestIOSSnapshotter_Snapshot_JSONArrayWrapped(t *testing.T) {
	// iOS sometimes emits an array (e.g., app window + alert
	// overlay).
	arr := `[
    {"type":"Window","title":"Main","frame":{"x":0,"y":0,"width":375,"height":667},"enabled":true,"children":[]},
    {"type":"Alert","title":"Confirm","frame":{"x":50,"y":200,"width":275,"height":100},"enabled":true,"children":[]}
  ]`
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte(arr)})
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

func TestIOSSnapshotter_Snapshot_SingleElementArrayUnwrapped(t *testing.T) {
	arr := `[{"type":"Window","title":"Main","frame":{"x":0,"y":0,"width":375,"height":667},"enabled":true,"children":[]}]`
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte(arr)})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	if root.Role != "window" {
		t.Fatalf("single-element array should return the element directly, not wrap. Role = %q", root.Role)
	}
}

// ---------------------------------------------------------------------------
// iOS type → ARIA mapping
// ---------------------------------------------------------------------------

func TestIOSTypeToARIA_KnownTypes(t *testing.T) {
	cases := map[string]string{
		"Application":       "application",
		"Window":            "window",
		"Button":            "button",
		"ButtonElement":     "button",
		"StaticText":        "text",
		"Text":              "text",
		"TextField":         "textbox",
		"SecureTextField":   "textbox",
		"TextFieldElement":  "textbox",
		"TextView":          "textbox",
		"SearchField":       "textbox",
		"Image":             "image",
		"Icon":              "image",
		"Switch":            "switch",
		"Slider":            "slider",
		"ProgressIndicator": "progressbar",
		"Picker":            "combobox",
		"PickerWheel":       "combobox",
		"Cell":              "cell",
		"TableCell":         "cell",
		"Table":             "table",
		"CollectionView":    "list",
		"ScrollView":        "list",
		"NavigationBar":     "navigation",
		"Toolbar":           "toolbar",
		"TabBar":            "tablist",
		"Tab":               "tab",
		"Alert":             "dialog",
		"Link":              "link",
		"WebView":           "document",
		"Other":             "group",
	}
	for in, want := range cases {
		if got := iosTypeToARIA(in); got != want {
			t.Errorf("iosTypeToARIA(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIOSTypeToARIA_UnknownPassesThrough(t *testing.T) {
	if got := iosTypeToARIA("CustomThingy"); got != "CustomThingy" {
		t.Fatalf("unknown type fallback = %q", got)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestIOSSnapshotter_Snapshot_EmptyDumpError(t *testing.T) {
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte("")})
	_, err := snap.Snapshot(context.Background())
	if !errors.Is(err, ErrNoRoot) {
		t.Fatalf("empty dump: %v, want ErrNoRoot", err)
	}
}

func TestIOSSnapshotter_Snapshot_WhitespaceOnlyDumpError(t *testing.T) {
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte("   \n\t  ")})
	_, err := snap.Snapshot(context.Background())
	if !errors.Is(err, ErrNoRoot) {
		t.Fatalf("whitespace dump: %v, want ErrNoRoot", err)
	}
}

func TestIOSSnapshotter_Snapshot_EmptyArrayError(t *testing.T) {
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte("[]")})
	_, err := snap.Snapshot(context.Background())
	if !errors.Is(err, ErrNoRoot) {
		t.Fatalf("empty array: %v, want ErrNoRoot", err)
	}
}

func TestIOSSnapshotter_Snapshot_MalformedJSON(t *testing.T) {
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte("{not json")})
	_, err := snap.Snapshot(context.Background())
	if err == nil {
		t.Fatal("malformed JSON should fail")
	}
}

func TestIOSSnapshotter_Snapshot_MalformedArray(t *testing.T) {
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte("[malformed")})
	_, err := snap.Snapshot(context.Background())
	if err == nil {
		t.Fatal("malformed array should fail")
	}
}

func TestIOSSnapshotter_Snapshot_UnexpectedJSONKind(t *testing.T) {
	// A JSON scalar / string is neither object nor array.
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte(`"just a string"`)})
	_, err := snap.Snapshot(context.Background())
	if err == nil {
		t.Fatal("scalar JSON should fail")
	}
}

func TestIOSSnapshotter_Snapshot_DumperError(t *testing.T) {
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{err: errors.New("device offline")})
	_, err := snap.Snapshot(context.Background())
	if err == nil {
		t.Fatal("dumper error should propagate")
	}
}

func TestIOSSnapshotter_Snapshot_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte(sampleIDBDump)})
	if _, err := snap.Snapshot(ctx); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestIOSSnapshotter_Close_Idempotent(t *testing.T) {
	d := &fakeIDBDumper{json: []byte(sampleIDBDump)}
	snap := NewIOSSnapshotterWithDumper(d)
	snap.Close()
	snap.Close()
	if d.closeFn != 1 {
		t.Fatalf("dumper.Close called %d times, want 1", d.closeFn)
	}
}

func TestIOSSnapshotter_Snapshot_AfterCloseReturnsError(t *testing.T) {
	snap := NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte(sampleIDBDump)})
	snap.Close()
	if _, err := snap.Snapshot(context.Background()); err != ErrSnapshotClosed {
		t.Fatalf("post-Close = %v, want ErrSnapshotClosed", err)
	}
}

// ---------------------------------------------------------------------------
// IDBShellDumper — UDID validation + Close
// ---------------------------------------------------------------------------

func TestIDBShellDumper_EmptyUDIDError(t *testing.T) {
	d := &IDBShellDumper{}
	if _, err := d.Dump(context.Background()); err == nil {
		t.Fatal("empty UDID must error")
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// Factory constructor
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Interface conformance
// ---------------------------------------------------------------------------

func TestIOSSnapshotter_SatisfiesSnapshotterInterface(t *testing.T) {
	var s Snapshotter = NewIOSSnapshotterWithDumper(&fakeIDBDumper{json: []byte(sampleIDBDump)})
	defer s.Close()
	if _, err := s.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot via interface: %v", err)
	}
}
