// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package axtree

import (
	"context"
	"errors"
	"image"
	"testing"

	dbus "github.com/godbus/dbus/v5"
)

// ---------------------------------------------------------------------------
// Fake LinuxBus — used by every test in this file.
// ---------------------------------------------------------------------------

// fakeAccessible models a single AT-SPI2 node in the scripted tree.
type fakeAccessible struct {
	path     dbus.ObjectPath
	props    NodeProps
	children []dbus.ObjectPath
}

type fakeLinuxBus struct {
	root        dbus.ObjectPath
	nodes       map[dbus.ObjectPath]fakeAccessible
	closeCalls  int
	rootError   error
	propsError  map[dbus.ObjectPath]error
	childError  map[dbus.ObjectPath]error
	ctxHook     func(ctx context.Context) error
}

func (f *fakeLinuxBus) GetRoot(ctx context.Context) (string, dbus.ObjectPath, error) {
	if f.rootError != nil {
		return "", "", f.rootError
	}
	// Note: ctxHook deliberately skipped here — GetRoot is the first
	// call and tests that want to exercise walk's own ctx.Err() path
	// need the root fetch to succeed first.
	return "org.a11y.atspi.Registry", f.root, nil
}

func (f *fakeLinuxBus) GetChildren(ctx context.Context, dest string, path dbus.ObjectPath) ([]Accessible, error) {
	if err := f.childError[path]; err != nil {
		return nil, err
	}
	if f.ctxHook != nil {
		if err := f.ctxHook(ctx); err != nil {
			return nil, err
		}
	}
	n, ok := f.nodes[path]
	if !ok {
		return nil, errors.New("fakeLinuxBus: unknown path " + string(path))
	}
	out := make([]Accessible, 0, len(n.children))
	for _, c := range n.children {
		out = append(out, Accessible{Dest: dest, Path: c})
	}
	return out, nil
}

func (f *fakeLinuxBus) GetProps(ctx context.Context, dest string, path dbus.ObjectPath) (NodeProps, error) {
	if err := f.propsError[path]; err != nil {
		return NodeProps{}, err
	}
	n, ok := f.nodes[path]
	if !ok {
		return NodeProps{}, errors.New("fakeLinuxBus: unknown path " + string(path))
	}
	return n.props, nil
}

func (f *fakeLinuxBus) Close() error {
	f.closeCalls++
	return nil
}

// buildFakeBus scripts a small three-level tree:
//
//	root (application)
//	├── window "Main"
//	│   ├── button "OK"
//	│   └── textbox "Search"
//	└── dialog "Dialog"
func buildFakeBus() *fakeLinuxBus {
	root := dbus.ObjectPath("/org/a11y/atspi/accessible/root")
	win := dbus.ObjectPath("/org/a11y/atspi/accessible/window")
	ok := dbus.ObjectPath("/org/a11y/atspi/accessible/ok")
	tb := dbus.ObjectPath("/org/a11y/atspi/accessible/textbox")
	dlg := dbus.ObjectPath("/org/a11y/atspi/accessible/dialog")

	return &fakeLinuxBus{
		root: root,
		nodes: map[dbus.ObjectPath]fakeAccessible{
			root: {
				path:  root,
				props: NodeProps{Role: "application", Name: "HelixQA", Enabled: true},
				children: []dbus.ObjectPath{win, dlg},
			},
			win: {
				path:  win,
				props: NodeProps{Role: "window", Name: "Main", Bounds: image.Rect(0, 0, 1920, 1080), Enabled: true, Focused: true},
				children: []dbus.ObjectPath{ok, tb},
			},
			ok: {
				path:  ok,
				props: NodeProps{Role: "button", Name: "OK", Bounds: image.Rect(10, 10, 80, 30), Enabled: true},
			},
			tb: {
				path:  tb,
				props: NodeProps{Role: "textbox", Name: "Search", Value: "hello", Bounds: image.Rect(100, 10, 300, 30), Enabled: true, Focused: false},
			},
			dlg: {
				path:  dlg,
				props: NodeProps{Role: "dialog", Name: "Dialog", Enabled: false, Selected: true},
			},
		},
		propsError: map[dbus.ObjectPath]error{},
		childError: map[dbus.ObjectPath]error{},
	}
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestLinuxSnapshotter_Snapshot_FullTree(t *testing.T) {
	bus := buildFakeBus()
	snap := NewLinuxSnapshotterWithBus(bus)
	defer snap.Close()

	root, err := snap.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if root == nil {
		t.Fatal("nil root")
	}
	if root.Role != "application" || root.Name != "HelixQA" {
		t.Fatalf("root = (role=%q, name=%q), want (application, HelixQA)", root.Role, root.Name)
	}
	if got := root.CountDescendants(); got != 5 {
		t.Fatalf("descendant count = %d, want 5", got)
	}
	if root.Platform != PlatformLinux {
		t.Fatalf("Platform = %q, want %q", root.Platform, PlatformLinux)
	}
	if !strHas(root.RawID, "/root") {
		t.Fatalf("RawID = %q, want to contain /root", root.RawID)
	}
}

func strHas(s, sub string) bool {
	return len(s) >= len(sub) && indexOfSubstring(s, sub) >= 0
}

func indexOfSubstring(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestLinuxSnapshotter_Snapshot_PropertiesPropagate(t *testing.T) {
	bus := buildFakeBus()
	snap := NewLinuxSnapshotterWithBus(bus)
	defer snap.Close()

	root, err := snap.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	btn := root.Find(func(n *Node) bool { return n.Role == "button" })
	if btn == nil {
		t.Fatal("button not found")
	}
	if btn.Name != "OK" {
		t.Fatalf("button name = %q, want OK", btn.Name)
	}
	if !btn.Enabled {
		t.Fatal("button should be enabled")
	}
	if btn.Bounds != image.Rect(10, 10, 80, 30) {
		t.Fatalf("button bounds = %v, want (10,10)-(80,30)", btn.Bounds)
	}

	tb := root.Find(func(n *Node) bool { return n.Role == "textbox" })
	if tb == nil || tb.Value != "hello" {
		t.Fatalf("textbox value = %q, want hello", tb.Value)
	}

	dlg := root.Find(func(n *Node) bool { return n.Role == "dialog" })
	if dlg == nil || dlg.Enabled || !dlg.Selected {
		t.Fatalf("dialog state wrong: %+v", dlg)
	}
}

// ---------------------------------------------------------------------------
// Walk + Find + CountDescendants
// ---------------------------------------------------------------------------

func TestNode_Walk_VisitsEveryNodeDepthFirstPreOrder(t *testing.T) {
	bus := buildFakeBus()
	snap := NewLinuxSnapshotterWithBus(bus)
	root, _ := snap.Snapshot(context.Background())

	var visited []string
	root.Walk(func(n *Node) error {
		visited = append(visited, n.Name)
		return nil
	})
	want := []string{"HelixQA", "Main", "OK", "Search", "Dialog"}
	if len(visited) != len(want) {
		t.Fatalf("visited %d nodes, want %d: %v", len(visited), len(want), visited)
	}
	for i, n := range visited {
		if n != want[i] {
			t.Fatalf("visited[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestNode_Walk_PropagatesError(t *testing.T) {
	bus := buildFakeBus()
	snap := NewLinuxSnapshotterWithBus(bus)
	root, _ := snap.Snapshot(context.Background())

	sentinel := errors.New("stop")
	err := root.Walk(func(n *Node) error {
		if n.Role == "button" {
			return sentinel
		}
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Walk err = %v, want %v", err, sentinel)
	}
}

func TestNode_Walk_NilNodeIsNoop(t *testing.T) {
	var n *Node
	if err := n.Walk(func(*Node) error { return errors.New("should not fire") }); err != nil {
		t.Fatalf("Walk on nil returned err %v, want nil", err)
	}
}

func TestNode_Find_ReturnsNilOnNoMatch(t *testing.T) {
	bus := buildFakeBus()
	snap := NewLinuxSnapshotterWithBus(bus)
	root, _ := snap.Snapshot(context.Background())
	if got := root.Find(func(*Node) bool { return false }); got != nil {
		t.Fatalf("Find with no match = %+v, want nil", got)
	}
}

func TestNode_Find_StopsAtFirstMatch(t *testing.T) {
	bus := buildFakeBus()
	snap := NewLinuxSnapshotterWithBus(bus)
	root, _ := snap.Snapshot(context.Background())
	count := 0
	root.Find(func(n *Node) bool {
		count++
		return n.Role == "window"
	})
	// Find should stop after "window" (second visit): HelixQA → Main.
	if count != 2 {
		t.Fatalf("Find visited %d nodes, want 2", count)
	}
}

func TestNode_CountDescendants_Nil(t *testing.T) {
	var n *Node
	if c := n.CountDescendants(); c != 0 {
		t.Fatalf("nil count = %d, want 0", c)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestLinuxSnapshotter_Close_Idempotent(t *testing.T) {
	bus := buildFakeBus()
	snap := NewLinuxSnapshotterWithBus(bus)
	if err := snap.Close(); err != nil {
		t.Fatalf("Close #1: %v", err)
	}
	if err := snap.Close(); err != nil {
		t.Fatalf("Close #2: %v", err)
	}
	if bus.closeCalls != 1 {
		t.Fatalf("bus.Close() called %d times, want 1", bus.closeCalls)
	}
}

func TestLinuxSnapshotter_Snapshot_AfterCloseReturnsError(t *testing.T) {
	bus := buildFakeBus()
	snap := NewLinuxSnapshotterWithBus(bus)
	snap.Close()
	if _, err := snap.Snapshot(context.Background()); err != ErrSnapshotClosed {
		t.Fatalf("Snapshot after Close = %v, want ErrSnapshotClosed", err)
	}
}

func TestLinuxSnapshotter_Snapshot_GetRootError(t *testing.T) {
	bus := buildFakeBus()
	bus.rootError = errors.New("boom")
	snap := NewLinuxSnapshotterWithBus(bus)
	if _, err := snap.Snapshot(context.Background()); err == nil || !strHas(err.Error(), "GetRoot") {
		t.Fatalf("GetRoot error not propagated: %v", err)
	}
}

func TestLinuxSnapshotter_Snapshot_EmptyRoot(t *testing.T) {
	bus := &fakeLinuxBus{
		root:  "",
		nodes: map[dbus.ObjectPath]fakeAccessible{},
	}
	snap := NewLinuxSnapshotterWithBus(bus)
	if _, err := snap.Snapshot(context.Background()); err != ErrNoRoot {
		t.Fatalf("empty-root = %v, want ErrNoRoot", err)
	}
}

func TestLinuxSnapshotter_Snapshot_ContextCanceled(t *testing.T) {
	// Pre-cancel the context. GetRoot in our fake bus ignores ctxHook,
	// so it returns the root successfully — then walk() immediately
	// checks ctx.Err() and bails out on line 126.
	bus := buildFakeBus()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snap := NewLinuxSnapshotterWithBus(bus)
	if _, err := snap.Snapshot(ctx); err == nil {
		t.Fatal("Snapshot with canceled ctx should fail")
	}
}

func TestLinuxSnapshotter_Snapshot_GetPropsError(t *testing.T) {
	bus := buildFakeBus()
	buttonPath := dbus.ObjectPath("/org/a11y/atspi/accessible/ok")
	bus.propsError[buttonPath] = errors.New("prop fail")
	snap := NewLinuxSnapshotterWithBus(bus)
	if _, err := snap.Snapshot(context.Background()); err == nil || !strHas(err.Error(), "GetProps") {
		t.Fatalf("GetProps error not propagated: %v", err)
	}
}

func TestLinuxSnapshotter_Snapshot_GetChildrenError(t *testing.T) {
	bus := buildFakeBus()
	bus.childError[bus.root] = errors.New("child fail")
	snap := NewLinuxSnapshotterWithBus(bus)
	if _, err := snap.Snapshot(context.Background()); err == nil || !strHas(err.Error(), "GetChildren") {
		t.Fatalf("GetChildren error not propagated: %v", err)
	}
}

// ---------------------------------------------------------------------------
// AT-SPI role translation
// ---------------------------------------------------------------------------

func TestAtspiRoleToARIA_KnownRoles(t *testing.T) {
	// Exhaustive coverage — every case in the switch.
	cases := map[uint32]string{
		7:   "application",
		9:   "article",
		12:  "button",
		25:  "combobox",
		44:  "dialog",
		45:  "document",
		50:  "form",
		54:  "heading",
		58:  "image",
		63:  "link",
		68:  "list",
		69:  "listitem",
		72:  "menu",
		73:  "menubar",
		74:  "menuitem",
		88:  "progressbar",
		90:  "radio",
		93:  "scrollbar",
		95:  "slider",
		109: "table",
		110: "cell",
		112: "rowheader",
		113: "columnheader",
		117: "textbox",
		128: "toolbar",
		130: "tooltip",
		134: "window",
		136: "header",
		137: "footer",
		138: "paragraph",
		158: "tab",
		159: "tabpanel",
	}
	for code, name := range cases {
		if got := atspiRoleToARIA(code); got != name {
			t.Errorf("role %d: got %q, want %q", code, got, name)
		}
	}
}

func TestAtspiRoleToARIA_UnknownCodeFallback(t *testing.T) {
	if got := atspiRoleToARIA(9999); got != "role-9999" {
		t.Errorf("unknown code = %q, want role-9999", got)
	}
}

func TestRoleCodeFromName_RoundTrip(t *testing.T) {
	for _, name := range []string{"application", "button", "image", "link", "slider", "textbox", "window"} {
		code := roleCodeFromName(name)
		if code == 0 {
			t.Errorf("roleCodeFromName(%q) = 0", name)
			continue
		}
		if back := atspiRoleToARIA(code); back != name {
			t.Errorf("round-trip %q → %d → %q", name, code, back)
		}
	}
}

func TestRoleCodeFromName_CaseInsensitive(t *testing.T) {
	if roleCodeFromName("BUTTON") != 12 {
		t.Fatal("case-insensitive lookup failed")
	}
}

func TestRoleCodeFromName_Unknown(t *testing.T) {
	if roleCodeFromName("totally-not-a-role") != 0 {
		t.Fatal("unknown role name should return 0")
	}
}

// ---------------------------------------------------------------------------
// stateBit
// ---------------------------------------------------------------------------

func TestStateBit_ReadsCorrectBit(t *testing.T) {
	// AT-SPI state is a two-uint32 bitmask. bit 12 is in word 0, bit 12.
	mask := []uint32{1 << 12, 0}
	if !stateBit(mask, 12) {
		t.Fatal("bit 12 not read in word 0")
	}
	// bit 40 is in word 1, bit 8.
	mask2 := []uint32{0, 1 << 8}
	if !stateBit(mask2, 40) {
		t.Fatal("bit 40 not read in word 1")
	}
	// Wrong mask shape → false.
	if stateBit([]uint32{1}, 0) {
		t.Fatal("single-word mask should return false")
	}
	// Out-of-range bit → false.
	if stateBit(mask, -1) || stateBit(mask, 64) {
		t.Fatal("out-of-range bits should return false")
	}
}

// ---------------------------------------------------------------------------
// Interface conformance
// ---------------------------------------------------------------------------

func TestLinuxSnapshotter_SatisfiesSnapshotterInterface(t *testing.T) {
	var s Snapshotter = NewLinuxSnapshotterWithBus(buildFakeBus())
	defer s.Close()
	if _, err := s.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot via interface: %v", err)
	}
}
