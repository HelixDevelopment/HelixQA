// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package axtree

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/go-json-experiment/json/jsontext"
)

// ---------------------------------------------------------------------------
// Fake WebFetcher
// ---------------------------------------------------------------------------

type fakeWebFetcher struct {
	nodes   []*accessibility.Node
	err     error
	closeFn int
}

func (f *fakeWebFetcher) Fetch(ctx context.Context) ([]*accessibility.Node, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.nodes, nil
}

func (f *fakeWebFetcher) Close() error { f.closeFn++; return nil }

// ---------------------------------------------------------------------------
// Fixture builders
// ---------------------------------------------------------------------------

func axValueOfString(s string) *accessibility.Value {
	b, _ := json.Marshal(s)
	return &accessibility.Value{Type: accessibility.ValueTypeString, Value: jsontext.Value(b)}
}

// sampleCDPTree returns a 5-node CDP AX tree:
//
//	RootWebArea "HelixQA Demo"
//	├── NavigationBar
//	│   └── link "Docs"
//	├── button "Sign In"
//	└── textbox "Search" (value="admin")
func sampleCDPTree() []*accessibility.Node {
	return []*accessibility.Node{
		{NodeID: "1", Role: axValueOfString("RootWebArea"), Name: axValueOfString("HelixQA Demo"),
			ChildIDs: []accessibility.NodeID{"2", "3", "4"}},
		{NodeID: "2", ParentID: "1", Role: axValueOfString("navigation"), Name: axValueOfString(""),
			ChildIDs: []accessibility.NodeID{"5"}},
		{NodeID: "3", ParentID: "1", Role: axValueOfString("button"), Name: axValueOfString("Sign In"),
			Properties: []*accessibility.Property{
				{Name: "focused", Value: axValueOfString("true")},
			}},
		{NodeID: "4", ParentID: "1", Role: axValueOfString("textbox"), Name: axValueOfString("Search"),
			Value: axValueOfString("admin"),
			Properties: []*accessibility.Property{
				{Name: "disabled", Value: axValueOfString("false")},
			}},
		{NodeID: "5", ParentID: "2", Role: axValueOfString("link"), Name: axValueOfString("Docs")},
	}
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestWebSnapshotter_Snapshot_FullTree(t *testing.T) {
	snap := NewWebSnapshotterWithFetcher(&fakeWebFetcher{nodes: sampleCDPTree()})
	defer snap.Close()

	root, err := snap.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if root.Platform != PlatformWeb {
		t.Fatalf("Platform = %q, want %q", root.Platform, PlatformWeb)
	}
	if root.Role != "document" { // RootWebArea → document
		t.Fatalf("root role = %q, want document", root.Role)
	}
	if root.Name != "HelixQA Demo" {
		t.Fatalf("root Name = %q", root.Name)
	}
	if root.RawID != "1" {
		t.Fatalf("root RawID = %q", root.RawID)
	}
	if c := root.CountDescendants(); c != 5 {
		t.Fatalf("descendants = %d, want 5", c)
	}
}

func TestWebSnapshotter_Snapshot_AttributesTranslated(t *testing.T) {
	snap := NewWebSnapshotterWithFetcher(&fakeWebFetcher{nodes: sampleCDPTree()})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())

	btn := root.Find(func(n *Node) bool { return n.Role == "button" })
	if btn == nil {
		t.Fatal("button not found")
	}
	if btn.Name != "Sign In" {
		t.Fatalf("button Name = %q", btn.Name)
	}
	if !btn.Focused {
		t.Fatal("button focused flag not propagated")
	}

	tb := root.Find(func(n *Node) bool { return n.Role == "textbox" })
	if tb == nil || tb.Value != "admin" {
		t.Fatalf("textbox Value = %+v", tb)
	}
	if !tb.Enabled {
		t.Fatal("textbox should be Enabled (disabled=false inverts)")
	}
}

func TestWebSnapshotter_Snapshot_MultiRootWrapped(t *testing.T) {
	// Two root-less nodes — should wrap in synthetic application root.
	nodes := []*accessibility.Node{
		{NodeID: "a", Role: axValueOfString("RootWebArea"), Name: axValueOfString("Frame1")},
		{NodeID: "b", Role: axValueOfString("RootWebArea"), Name: axValueOfString("Frame2")},
	}
	snap := NewWebSnapshotterWithFetcher(&fakeWebFetcher{nodes: nodes})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	if root.Role != "application" {
		t.Fatalf("multi-root wrapper role = %q", root.Role)
	}
	if len(root.Children) != 2 {
		t.Fatalf("wrapper children = %d, want 2", len(root.Children))
	}
}

func TestWebSnapshotter_Snapshot_IgnoredNodesHoistGrandchildren(t *testing.T) {
	// Root contains an Ignored wrapper; the grandchild should become
	// a direct child of root.
	nodes := []*accessibility.Node{
		{NodeID: "r", Role: axValueOfString("RootWebArea"), Name: axValueOfString("Root"),
			ChildIDs: []accessibility.NodeID{"ign"}},
		{NodeID: "ign", ParentID: "r", Ignored: true, ChildIDs: []accessibility.NodeID{"leaf"}},
		{NodeID: "leaf", ParentID: "ign", Role: axValueOfString("button"), Name: axValueOfString("Hidden button")},
	}
	snap := NewWebSnapshotterWithFetcher(&fakeWebFetcher{nodes: nodes})
	defer snap.Close()
	root, _ := snap.Snapshot(context.Background())
	// Root should have one child (the button) — the ignored wrapper
	// is elided but its grandchild is hoisted up.
	if len(root.Children) != 1 {
		t.Fatalf("root children = %d, want 1 (ignored wrapper elided)", len(root.Children))
	}
	if root.Children[0].Role != "button" {
		t.Fatalf("hoisted child role = %q, want button", root.Children[0].Role)
	}
}

func TestWebSnapshotter_Snapshot_CycleGuard(t *testing.T) {
	// Malformed tree with cycle: a → b → a. Cycle guard prevents
	// infinite recursion and returns a finite tree.
	nodes := []*accessibility.Node{
		{NodeID: "a", Role: axValueOfString("RootWebArea"), ChildIDs: []accessibility.NodeID{"b"}},
		{NodeID: "b", ParentID: "a", Role: axValueOfString("group"), ChildIDs: []accessibility.NodeID{"a"}},
	}
	snap := NewWebSnapshotterWithFetcher(&fakeWebFetcher{nodes: nodes})
	defer snap.Close()
	root, err := snap.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	// Should not hang; root + b are visited once each.
	if root.CountDescendants() != 2 {
		t.Fatalf("cycle descendants = %d, want 2", root.CountDescendants())
	}
}

// ---------------------------------------------------------------------------
// Role normalization
// ---------------------------------------------------------------------------

func TestNormalizeWebRole(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"StaticText":       "text",
		"RootWebArea":      "document",
		"WebView":          "document",
		"genericContainer": "group",
		"button":           "button", // passthrough
		"link":             "link",   // passthrough
	}
	for in, want := range cases {
		if got := normalizeWebRole(in); got != want {
			t.Errorf("normalizeWebRole(%q) = %q, want %q", in, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func TestAxValueString_Variants(t *testing.T) {
	// nil and empty value → empty string.
	if got := axValueString(nil); got != "" {
		t.Fatalf("nil = %q", got)
	}
	if got := axValueString(&accessibility.Value{}); got != "" {
		t.Fatalf("empty = %q", got)
	}
	// JSON-quoted string → unquoted.
	if got := axValueString(axValueOfString("hello")); got != "hello" {
		t.Fatalf("string = %q", got)
	}
	// Non-string value passes through (numeric).
	v := &accessibility.Value{Value: jsontext.Value("42")}
	if got := axValueString(v); got != "42" {
		t.Fatalf("numeric = %q", got)
	}
}

func TestAxBoolProperty(t *testing.T) {
	n := &accessibility.Node{
		Properties: []*accessibility.Property{
			{Name: "focused", Value: axValueOfString("true")},
			{Name: "selected", Value: axValueOfString("false")},
			{Name: "disabled", Value: axValueOfString("true")},
		},
	}
	if !axBoolProperty(n, "focused", false) {
		t.Error("focused should be true")
	}
	if axBoolProperty(n, "selected", true) {
		t.Error("selected should be false (explicit false)")
	}
	// disabled=true → Enabled=false (invert).
	if axBoolProperty(n, "disabled", true) {
		t.Error("disabled=true should map to Enabled=false")
	}
	// Missing property → default.
	if !axBoolProperty(n, "unknown", true) {
		t.Error("unknown property should return default=true")
	}
	// Property with nil Value → default (not processed).
	n2 := &accessibility.Node{
		Properties: []*accessibility.Property{{Name: "focused", Value: nil}},
	}
	if axBoolProperty(n2, "focused", false) {
		t.Error("nil value should fall through to default")
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestWebSnapshotter_Snapshot_EmptyTreeError(t *testing.T) {
	snap := NewWebSnapshotterWithFetcher(&fakeWebFetcher{nodes: nil})
	_, err := snap.Snapshot(context.Background())
	if !errors.Is(err, ErrNoRoot) {
		t.Fatalf("empty: %v, want ErrNoRoot", err)
	}
}

func TestWebSnapshotter_Snapshot_NoRootNodeError(t *testing.T) {
	// All nodes have ParentID — no candidate root.
	nodes := []*accessibility.Node{
		{NodeID: "1", ParentID: "ghost", Role: axValueOfString("group")},
	}
	snap := NewWebSnapshotterWithFetcher(&fakeWebFetcher{nodes: nodes})
	_, err := snap.Snapshot(context.Background())
	if !errors.Is(err, ErrNoRoot) {
		t.Fatalf("no-root: %v, want ErrNoRoot", err)
	}
}

func TestWebSnapshotter_Snapshot_FetcherError(t *testing.T) {
	snap := NewWebSnapshotterWithFetcher(&fakeWebFetcher{err: errors.New("browser gone")})
	_, err := snap.Snapshot(context.Background())
	if err == nil {
		t.Fatal("fetcher error should propagate")
	}
}

func TestWebSnapshotter_Snapshot_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	snap := NewWebSnapshotterWithFetcher(&fakeWebFetcher{nodes: sampleCDPTree()})
	if _, err := snap.Snapshot(ctx); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

func TestWebSnapshotter_Close_Idempotent(t *testing.T) {
	f := &fakeWebFetcher{nodes: sampleCDPTree()}
	snap := NewWebSnapshotterWithFetcher(f)
	snap.Close()
	snap.Close()
	if f.closeFn != 1 {
		t.Fatalf("fetcher.Close called %d times, want 1", f.closeFn)
	}
}

func TestWebSnapshotter_Snapshot_AfterCloseReturnsError(t *testing.T) {
	snap := NewWebSnapshotterWithFetcher(&fakeWebFetcher{nodes: sampleCDPTree()})
	snap.Close()
	if _, err := snap.Snapshot(context.Background()); err != ErrSnapshotClosed {
		t.Fatalf("post-Close = %v, want ErrSnapshotClosed", err)
	}
}

// ---------------------------------------------------------------------------
// ChromedpFetcher — validation (no live browser; just guard paths).
// ---------------------------------------------------------------------------

func TestChromedpFetcher_NilAllocCtxError(t *testing.T) {
	f := &ChromedpFetcher{}
	if _, err := f.Fetch(context.Background()); err == nil {
		t.Fatal("nil AllocCtx must error")
	}
}

func TestChromedpFetcher_CloseIsIdempotent(t *testing.T) {
	f := &ChromedpFetcher{}
	// Close before Fetch — no tab was built; should be a no-op.
	if err := f.Close(); err != nil {
		t.Fatalf("Close on virgin fetcher: %v", err)
	}
	// Second call also clean.
	if err := f.Close(); err != nil {
		t.Fatalf("Second Close: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Constructor + interface conformance
// ---------------------------------------------------------------------------

func TestNewWebSnapshotter_UsesChromedpFetcher(t *testing.T) {
	s := NewWebSnapshotter(context.Background())
	f, ok := s.fetcher.(*ChromedpFetcher)
	if !ok {
		t.Fatalf("fetcher type = %T, want *ChromedpFetcher", s.fetcher)
	}
	if f.AllocCtx == nil {
		t.Fatal("AllocCtx not wired")
	}
}

func TestWebSnapshotter_SatisfiesSnapshotterInterface(t *testing.T) {
	var s Snapshotter = NewWebSnapshotterWithFetcher(&fakeWebFetcher{nodes: sampleCDPTree()})
	defer s.Close()
	if _, err := s.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot via interface: %v", err)
	}
}
