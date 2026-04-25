// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package axtree

import (
	"context"
	"errors"
	"image"
)

// Platform identifies which accessibility backend produced a Node. Used by
// higher layers to decide which RawID format is in play and which platform-
// specific actions are available.
type Platform string

const (
	PlatformLinux   Platform = "linux-atspi2"
	PlatformWeb     Platform = "web-cdp"
	PlatformAndroid Platform = "android-uia2"
	PlatformDarwin  Platform = "darwin-ax"
	PlatformWindows Platform = "windows-uiautomation"
	PlatformIOS     Platform = "ios-idb"
	// PlatformTUI is defined in tui.go (the TUI backend is a
	// later addition; it lives in its own file to avoid
	// reshuffling this Platform block).
)

// Node is the unified accessibility-tree entry across every platform
// HelixQA targets. The cross-platform fields (Role, Name, Bounds, Enabled,
// Focused, Selected) are what the grounding VLM and the target-resolution
// code consume; RawID carries the platform-native identifier for callbacks
// (click, focus, inspect).
type Node struct {
	// Role is the accessibility role (e.g. "button", "link", "textbox").
	// Normalized to the ARIA vocabulary — AT-SPI2 codes and UiAutomator2
	// class names are translated into the same set.
	Role string

	// Name is the accessible name (aria-label, text content, or platform-
	// specific fallback). Never nil, possibly empty.
	Name string

	// Value is the current value for controls that carry one (slider
	// position, textbox contents, checkbox state as "true"/"false").
	// Empty for leaf nodes that have no value semantics.
	Value string

	// Bounds is the screen-space rectangle the node occupies, or the
	// zero rectangle if the backend did not report one.
	Bounds image.Rectangle

	// Enabled / Focused / Selected flags — defaults false if the
	// backend did not surface the state.
	Enabled  bool
	Focused  bool
	Selected bool

	// Children are immediate descendants. The tree is materialized
	// eagerly at snapshot time; callers wanting lazy traversal should
	// call Snapshot again with a different root path.
	Children []*Node

	// Platform identifies which backend produced this Node.
	Platform Platform

	// RawID is the opaque platform-native identifier (AT-SPI2 object
	// path, CDP backendNodeId, UiAutomator2 class+instance path, etc).
	// Round-tripped by the action-resolution layer to target the right
	// widget.
	RawID string
}

// Walk invokes visitor on this node and every descendant in depth-first
// pre-order. Returning a non-nil error stops the walk early and propagates
// the error to the caller.
func (n *Node) Walk(visitor func(*Node) error) error {
	if n == nil {
		return nil
	}
	if err := visitor(n); err != nil {
		return err
	}
	for _, c := range n.Children {
		if err := c.Walk(visitor); err != nil {
			return err
		}
	}
	return nil
}

// Find returns the first descendant (including this node) for which
// match returns true, or nil if none match.
func (n *Node) Find(match func(*Node) bool) *Node {
	var found *Node
	n.Walk(func(nd *Node) error {
		if found != nil {
			return nil
		}
		if match(nd) {
			found = nd
		}
		return nil
	})
	return found
}

// CountDescendants returns 1 (self) plus every descendant.
func (n *Node) CountDescendants() int {
	if n == nil {
		return 0
	}
	c := 1
	for _, child := range n.Children {
		c += child.CountDescendants()
	}
	return c
}

// Snapshotter is the cross-platform accessibility-tree capture interface.
// Each platform provides one implementation (LinuxSnapshotter via AT-SPI2,
// WebSnapshotter via CDP, AndroidSnapshotter via UiAutomator2, …).
type Snapshotter interface {
	// Snapshot walks the accessibility tree rooted at the platform-
	// specific default root and returns a fully-materialized Node tree.
	// Implementations honor ctx cancellation for long traversals.
	Snapshot(ctx context.Context) (*Node, error)

	// Close releases any backend resources (D-Bus connection, CDP
	// client, ADB session, …). Safe to call multiple times.
	Close() error
}

// Sentinel errors shared by every Snapshotter implementation.
var (
	ErrNotAvailable   = errors.New("axtree: accessibility backend not available on this platform")
	ErrNoRoot         = errors.New("axtree: backend did not report a root accessible")
	ErrSnapshotClosed = errors.New("axtree: Snapshotter already closed")
)
