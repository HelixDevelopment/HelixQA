// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package axtree

import (
	"context"
	"errors"
	"fmt"
	"image"
	"strconv"
	"strings"
	"sync"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"
)

// WebFetcher is the narrow contract WebSnapshotter consumes. The
// production implementation (ChromedpFetcher) drives a headless or
// attached Chromium over the CDP Accessibility.getFullAXTree call;
// tests inject a fake that returns scripted []*accessibility.Node
// slices.
type WebFetcher interface {
	Fetch(ctx context.Context) ([]*accessibility.Node, error)
	Close() error
}

// ChromedpFetcher is the production WebFetcher. It attaches to an
// already-running Chromium DevTools endpoint (typically spawned by
// chromedp.NewExecAllocator before the snapshotter runs) and invokes
// Accessibility.getFullAXTree via chromedp.Run.
type ChromedpFetcher struct {
	// AllocCtx is the chromedp allocator context — obtained from
	// chromedp.NewExecAllocator or chromedp.NewRemoteAllocator.
	// Required.
	AllocCtx context.Context

	// TabCtx is the per-tab chromedp context the fetcher reuses
	// across Fetch calls. If zero, a new tab context is created from
	// AllocCtx on the first Fetch and retained for the fetcher's
	// lifetime.
	TabCtx    context.Context
	tabCancel context.CancelFunc

	mu sync.Mutex
}

// Fetch runs Accessibility.getFullAXTree on the target browser tab
// and returns the resulting AXNodes. Concurrent Fetch calls serialize
// through the tab mutex — CDP is inherently single-channel per tab.
func (f *ChromedpFetcher) Fetch(ctx context.Context) ([]*accessibility.Node, error) {
	if f.AllocCtx == nil {
		return nil, errors.New("axtree/web: ChromedpFetcher.AllocCtx must be set")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.TabCtx == nil {
		// Lazily build a tab context the first time we fetch.
		tabCtx, cancel := chromedp.NewContext(f.AllocCtx)
		f.TabCtx = tabCtx
		f.tabCancel = cancel
	}

	var nodes []*accessibility.Node
	err := chromedp.Run(f.TabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		var callErr error
		nodes, callErr = accessibility.GetFullAXTree().Do(ctx)
		return callErr
	}))
	if err != nil {
		return nil, fmt.Errorf("%w: getFullAXTree: %v", ErrNotAvailable, err)
	}
	return nodes, nil
}

// Close cancels the per-tab context if one was created. Idempotent.
func (f *ChromedpFetcher) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.tabCancel != nil {
		f.tabCancel()
		f.tabCancel = nil
	}
	return nil
}

// WebSnapshotter fetches a CDP AXTree via a WebFetcher and builds
// a materialized *Node tree with Platform = PlatformWeb. Since CDP
// returns nodes as a flat list with parent/child IDs, the snapshotter
// walks the list once to index by NodeID, then builds a tree by
// following ChildIDs from the (parent-less) root.
type WebSnapshotter struct {
	fetcher WebFetcher
	mu      sync.Mutex
	closed  bool
}

// NewWebSnapshotter binds to a ChromedpFetcher over the given
// allocator context. For tests, NewWebSnapshotterWithFetcher injects
// a mock.
func NewWebSnapshotter(allocCtx context.Context) *WebSnapshotter {
	return &WebSnapshotter{fetcher: &ChromedpFetcher{AllocCtx: allocCtx}}
}

// NewWebSnapshotterWithFetcher wires a snapshotter to any WebFetcher
// implementation.
func NewWebSnapshotterWithFetcher(f WebFetcher) *WebSnapshotter {
	return &WebSnapshotter{fetcher: f}
}

// Snapshot fetches the AX tree and materializes it into *Node form.
func (s *WebSnapshotter) Snapshot(ctx context.Context) (*Node, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrSnapshotClosed
	}
	s.mu.Unlock()

	nodes, err := s.fetcher.Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("axtree/web: Fetch: %w", err)
	}
	return buildWebTree(nodes)
}

// Close releases the fetcher. Idempotent.
func (s *WebSnapshotter) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.fetcher.Close()
}

// ---------------------------------------------------------------------------
// AX-tree assembly
// ---------------------------------------------------------------------------

// buildWebTree converts CDP's flat []*accessibility.Node list into a
// tree by following parent/child links. Ignores nodes flagged as
// Ignored=true (CDP marks these; skipping them keeps the Node tree
// focused on the user-visible accessibility surface).
func buildWebTree(nodes []*accessibility.Node) (*Node, error) {
	if len(nodes) == 0 {
		return nil, ErrNoRoot
	}

	// Index by NodeID for child lookups.
	byID := make(map[accessibility.NodeID]*accessibility.Node, len(nodes))
	for _, n := range nodes {
		byID[n.NodeID] = n
	}

	// Find the root — the node with an empty ParentID. If multiple
	// candidates exist (rare; iframe/frame boundaries), the first
	// one wins and the rest are attached as top-level children
	// under a synthetic application root.
	var roots []*accessibility.Node
	for _, n := range nodes {
		if n.ParentID == "" {
			roots = append(roots, n)
		}
	}
	if len(roots) == 0 {
		return nil, ErrNoRoot
	}

	// Cycle guard: the map tracks which NodeIDs we've converted so a
	// malformed tree with cycles doesn't recurse infinitely.
	visited := make(map[accessibility.NodeID]bool, len(nodes))

	if len(roots) == 1 {
		return convertWebNode(roots[0], byID, visited), nil
	}
	// Multi-root case — wrap in synthetic application node.
	children := make([]*Node, 0, len(roots))
	for _, r := range roots {
		children = append(children, convertWebNode(r, byID, visited))
	}
	return &Node{
		Role:     "application",
		Name:     "web-root",
		Platform: PlatformWeb,
		Children: children,
	}, nil
}

// convertWebNode recursively materializes a CDP AXNode into *Node
// form. visited short-circuits cycles.
func convertWebNode(n *accessibility.Node, byID map[accessibility.NodeID]*accessibility.Node, visited map[accessibility.NodeID]bool) *Node {
	if visited[n.NodeID] {
		return nil
	}
	visited[n.NodeID] = true

	role := ""
	if n.Role != nil {
		role = axValueString(n.Role)
	}
	name := ""
	if n.Name != nil {
		name = axValueString(n.Name)
	}
	value := ""
	if n.Value != nil {
		value = axValueString(n.Value)
	}

	out := &Node{
		Role:     normalizeWebRole(role),
		Name:     name,
		Value:    value,
		Enabled:  axBoolProperty(n, "disabled", true), // "disabled" inverted
		Focused:  axBoolProperty(n, "focused", false),
		Selected: axBoolProperty(n, "selected", false),
		Platform: PlatformWeb,
		RawID:    string(n.NodeID),
	}

	// CDP doesn't emit a geometric frame in Accessibility; leave
	// Bounds at the zero rectangle — downstream layers that need
	// bounds call DOM.getBoxModel separately.
	out.Bounds = image.Rectangle{}

	for _, childID := range n.ChildIDs {
		child, ok := byID[childID]
		if !ok {
			continue
		}
		if child.Ignored {
			// Hoist the grand-children of ignored wrappers; CDP
			// sometimes nests significant content under aria-hidden
			// containers that we still want to traverse.
			for _, gid := range child.ChildIDs {
				if g, ok := byID[gid]; ok {
					if c := convertWebNode(g, byID, visited); c != nil {
						out.Children = append(out.Children, c)
					}
				}
			}
			continue
		}
		if c := convertWebNode(child, byID, visited); c != nil {
			out.Children = append(out.Children, c)
		}
	}
	return out
}

// axValueString safely extracts the string form of an AXValue. CDP
// AXValue.Value is a json.RawMessage; we take the best-effort
// unquoted string representation, since HelixQA only consumes these
// for display/identity purposes.
func axValueString(v *accessibility.Value) string {
	if v == nil || len(v.Value) == 0 {
		return ""
	}
	s := strings.TrimSpace(string(v.Value))
	// RawMessage round-trips as JSON — strip surrounding quotes for
	// string types; numeric / boolean forms pass through as their
	// raw textual representation.
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		unq, err := strconv.Unquote(s)
		if err == nil {
			return unq
		}
		return s[1 : len(s)-1]
	}
	return s
}

// axBoolProperty extracts a named boolean property from the AXNode
// properties list. If the named property is missing, returns
// defaultValue; if present, returns its bool form.
//
// AXValue.Value is a JSON-encoded payload — the bool may arrive as
// `true` (native JSON boolean) or `"true"` (JSON string, which the
// string-valued AXValue type emits for text-rendered booleans).
// axValueString unwraps either form.
//
// The `invert` path: "disabled=true" → Enabled=false, so the
// Enabled field maps from NOT disabled.
func axBoolProperty(n *accessibility.Node, name string, defaultValue bool) bool {
	for _, p := range n.Properties {
		if string(p.Name) != name {
			continue
		}
		if p.Value == nil {
			continue
		}
		s := axValueString(p.Value)
		b := strings.EqualFold(s, "true")
		// For "disabled" property, Enabled = !disabled.
		if name == "disabled" {
			return !b
		}
		return b
	}
	return defaultValue
}

// normalizeWebRole maps CDP's raw role strings to the ARIA vocabulary
// HelixQA shares across platforms. CDP already emits ARIA role names
// for most elements ("button", "link", "textbox"), so the mapping is
// mostly a passthrough with a few light normalizations.
func normalizeWebRole(role string) string {
	switch role {
	case "":
		return ""
	case "StaticText":
		return "text"
	case "RootWebArea", "WebView":
		return "document"
	case "genericContainer":
		return "group"
	}
	return role
}

// Compile-time guards.
var (
	_ Snapshotter = (*WebSnapshotter)(nil)
	_ WebFetcher  = (*ChromedpFetcher)(nil)
)
