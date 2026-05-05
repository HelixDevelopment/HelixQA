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
// Fake AndroidDumper
// ---------------------------------------------------------------------------

type fakeDumper struct {
	xml       []byte
	err       error
	closeFn   int
}

func (f *fakeDumper) Dump(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.xml, nil
}

func (f *fakeDumper) Close() error { f.closeFn++; return nil }

// ---------------------------------------------------------------------------
// Canned UIAutomator dump — mirrors the shape emitted by a real device.
// ---------------------------------------------------------------------------

const sampleDump = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<hierarchy rotation="0">
  <node index="0" text="" resource-id="" class="android.widget.FrameLayout"
        package="com.example" content-desc="" enabled="true" focused="false"
        selected="false" bounds="[0,0][1080,2400]">
    <node index="0" text="" resource-id="android:id/action_bar" class="androidx.appcompat.widget.Toolbar"
          package="com.example" content-desc="Main toolbar" enabled="true" focused="false"
          selected="false" bounds="[0,80][1080,224]">
      <node index="0" text="HelixQA" resource-id="" class="android.widget.TextView"
            package="com.example" content-desc="" enabled="true" focused="false"
            selected="false" bounds="[48,120][400,184]"/>
    </node>
    <node index="1" text="Sign in" resource-id="com.example:id/login_button" class="android.widget.Button"
          package="com.example" content-desc="" enabled="true" focused="true"
          selected="false" bounds="[400,1200][680,1280]"/>
    <node index="2" text="" resource-id="com.example:id/search" class="android.widget.EditText"
          package="com.example" content-desc="Search field" enabled="true" focused="false"
          selected="true" bounds="[80,1400][1000,1480]"/>
  </node>
</hierarchy>
`

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestAndroidSnapshotter_Snapshot_FullTree(t *testing.T) {
	snap := NewAndroidSnapshotterWithDumper(&fakeDumper{xml: []byte(sampleDump)})
	defer snap.Close()

	root, err := snap.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if root == nil {
		t.Fatal("nil root")
	}
	if root.Platform != PlatformAndroid {
		t.Fatalf("Platform = %q, want %q", root.Platform, PlatformAndroid)
	}
	if got := root.CountDescendants(); got != 5 {
		t.Fatalf("descendant count = %d, want 5", got)
	}
	if root.Value != "rotation=0" {
		t.Fatalf("root.Value = %q, want rotation=0", root.Value)
	}
}

func TestAndroidSnapshotter_Snapshot_AttributesTranslated(t *testing.T) {
	snap := NewAndroidSnapshotterWithDumper(&fakeDumper{xml: []byte(sampleDump)})
	defer snap.Close()

	root, _ := snap.Snapshot(context.Background())
	btn := root.Find(func(n *Node) bool { return n.Role == "button" })
	if btn == nil {
		t.Fatal("button not found")
	}
	if btn.Name != "Sign in" {
		t.Fatalf("button name = %q, want 'Sign in'", btn.Name)
	}
	if btn.RawID != "com.example:id/login_button" {
		t.Fatalf("button RawID = %q", btn.RawID)
	}
	if !btn.Enabled || !btn.Focused {
		t.Fatalf("button flags wrong: %+v", btn)
	}
	if btn.Bounds != image.Rect(400, 1200, 680, 1280) {
		t.Fatalf("button bounds = %v", btn.Bounds)
	}

	// content-desc falls back to Name when text is empty.
	tb := root.Find(func(n *Node) bool { return n.Role == "textbox" })
	if tb == nil || tb.Name != "Search field" || !tb.Selected {
		t.Fatalf("textbox wrong: %+v", tb)
	}
}

func TestAndroidSnapshotter_Snapshot_MultipleTopLevelWrapped(t *testing.T) {
	// Some system dumps have multiple hierarchy children (e.g. status
	// bar + app window). The snapshotter wraps them in a synthetic
	// application-role root.
	xml := `<?xml version="1.0"?>
<hierarchy rotation="1">
  <node class="android.widget.FrameLayout" text="Status" bounds="[0,0][1080,80]"/>
  <node class="android.widget.FrameLayout" text="App" bounds="[0,80][1080,2400]"/>
</hierarchy>`

	snap := NewAndroidSnapshotterWithDumper(&fakeDumper{xml: []byte(xml)})
	defer snap.Close()
	root, err := snap.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if root.Role != "application" {
		t.Fatalf("wrapper role = %q, want application", root.Role)
	}
	if root.Name != "hierarchy" {
		t.Fatalf("wrapper name = %q", root.Name)
	}
	if root.Value != "rotation=1" {
		t.Fatalf("wrapper value = %q", root.Value)
	}
	if len(root.Children) != 2 {
		t.Fatalf("wrapper children = %d, want 2", len(root.Children))
	}
}

// ---------------------------------------------------------------------------
// XML → Node conversion unit tests
// ---------------------------------------------------------------------------

func TestParseAndroidBounds_HappyPath(t *testing.T) {
	if got := parseAndroidBounds("[10,20][100,200]"); got != image.Rect(10, 20, 100, 200) {
		t.Fatalf("happy path = %v", got)
	}
}

func TestParseAndroidBounds_NegativeCoords(t *testing.T) {
	if got := parseAndroidBounds("[-5,-10][50,60]"); got != image.Rect(-5, -10, 50, 60) {
		t.Fatalf("negative coords = %v", got)
	}
}

func TestParseAndroidBounds_Malformed(t *testing.T) {
	for _, s := range []string{"", "[0,0]", "nonsense", "[a,b][c,d]"} {
		if got := parseAndroidBounds(s); got != (image.Rectangle{}) {
			t.Errorf("malformed %q → %v, want zero rect", s, got)
		}
	}
}

func TestBoolAttr(t *testing.T) {
	if !boolAttr("true") {
		t.Fatal(`"true" → false`)
	}
	for _, s := range []string{"false", "True", "TRUE", "", "1", "yes"} {
		if boolAttr(s) {
			t.Errorf(`%q → true`, s)
		}
	}
}

func TestAndroidClassToARIA_KnownClasses(t *testing.T) {
	cases := map[string]string{
		"android.widget.FrameLayout":                  "group",
		"android.widget.Button":                       "button",
		"androidx.appcompat.widget.AppCompatButton":   "AppCompatButton", // not in table → fallback to bare name
		"android.widget.EditText":                     "textbox",
		"android.widget.ImageView":                    "image",
		"android.widget.CheckBox":                     "checkbox",
		"android.widget.RadioButton":                  "radio",
		"android.widget.Switch":                       "switch",
		"android.widget.ProgressBar":                  "progressbar",
		"android.widget.SeekBar":                      "slider",
		"androidx.appcompat.widget.Toolbar":           "toolbar",
		"android.widget.Spinner":                      "combobox",
		"androidx.recyclerview.widget.RecyclerView":   "list",
		"androidx.viewpager2.widget.ViewPager2":       "tabpanel",
		"com.google.android.material.tabs.TabLayout": "tablist",
		"android.webkit.WebView":                      "document",
		"androidx.drawerlayout.widget.DrawerLayout":   "navigation",
	}
	for cls, want := range cases {
		if got := androidClassToARIA(cls); got != want {
			t.Errorf("androidClassToARIA(%q) = %q, want %q", cls, got, want)
		}
	}
}

func TestAndroidClassToARIA_UnknownClassFallback(t *testing.T) {
	if got := androidClassToARIA("com.example.MyCustomWidget"); got != "MyCustomWidget" {
		t.Fatalf("unknown class fallback = %q", got)
	}
	// No dot in the class name — returns the full string as-is.
	if got := androidClassToARIA("UnqualifiedName"); got != "UnqualifiedName" {
		t.Fatalf("unqualified class = %q", got)
	}
}

func TestConvertXMLNode_UsesClassIndexWhenResourceIDMissing(t *testing.T) {
	x := xmlNode{
		Class:  "android.widget.TextView",
		Index:  "3",
		Text:   "Hello",
		Bounds: "[0,0][100,50]",
	}
	n := convertXMLNode(x)
	if n.RawID != "android.widget.TextView:3" {
		t.Fatalf("RawID fallback = %q", n.RawID)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestAndroidSnapshotter_Snapshot_DumperError(t *testing.T) {
	snap := NewAndroidSnapshotterWithDumper(&fakeDumper{err: errors.New("boom")})
	if _, err := snap.Snapshot(context.Background()); err == nil || !strHas(err.Error(), "Dump") {
		t.Fatalf("dumper error not propagated: %v", err)
	}
}

func TestAndroidSnapshotter_Snapshot_MalformedXML(t *testing.T) {
	snap := NewAndroidSnapshotterWithDumper(&fakeDumper{xml: []byte("not xml")})
	if _, err := snap.Snapshot(context.Background()); err == nil || !strHas(err.Error(), "parse") {
		t.Fatalf("malformed XML error not propagated: %v", err)
	}
}

func TestAndroidSnapshotter_Snapshot_EmptyHierarchy(t *testing.T) {
	xml := `<?xml version="1.0"?><hierarchy rotation="0"></hierarchy>`
	snap := NewAndroidSnapshotterWithDumper(&fakeDumper{xml: []byte(xml)})
	if _, err := snap.Snapshot(context.Background()); err == nil {
		t.Fatal("empty hierarchy must fail")
	}
}

func TestAndroidSnapshotter_Snapshot_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snap := NewAndroidSnapshotterWithDumper(&fakeDumper{xml: []byte(sampleDump)})
	if _, err := snap.Snapshot(ctx); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestAndroidSnapshotter_Close_Idempotent(t *testing.T) {
	d := &fakeDumper{xml: []byte(sampleDump)}
	snap := NewAndroidSnapshotterWithDumper(d)
	snap.Close()
	snap.Close()
	if d.closeFn != 1 {
		t.Fatalf("dumper.Close called %d times, want 1", d.closeFn)
	}
}

func TestAndroidSnapshotter_Snapshot_AfterCloseReturnsError(t *testing.T) {
	snap := NewAndroidSnapshotterWithDumper(&fakeDumper{xml: []byte(sampleDump)})
	snap.Close()
	if _, err := snap.Snapshot(context.Background()); err != ErrSnapshotClosed {
		t.Fatalf("post-Close Snapshot = %v, want ErrSnapshotClosed", err)
	}
}

// ---------------------------------------------------------------------------
// ADBDumper — serial validation (can't test the adb exec path without a
// device, but we can at least validate the guard).
// ---------------------------------------------------------------------------

func TestADBDumper_EmptySerialError(t *testing.T) {
	d := &ADBDumper{}
	if _, err := d.Dump(context.Background()); err == nil {
		t.Fatal("empty serial must error")
	}
	// Close is a no-op and should never error.
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

func TestAndroidSnapshotter_SatisfiesSnapshotterInterface(t *testing.T) {
	var s Snapshotter = NewAndroidSnapshotterWithDumper(&fakeDumper{xml: []byte(sampleDump)})
	defer s.Close()
	if _, err := s.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot via interface: %v", err)
	}
}
