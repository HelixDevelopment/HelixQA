// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package axtree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os/exec"
	"strings"
	"sync"
)

// IDBDumper emits a raw iOS UI hierarchy dump for a specific device
// via Facebook's idb (https://fbidb.io). The default implementation
// shells out to `idb ui describe-all --udid <UDID> --json`, which
// walks the running app's AXUIElement tree on the device and prints
// a JSON array of accessibility nodes to stdout. Tests inject a
// fake that returns canned JSON.
type IDBDumper interface {
	Dump(ctx context.Context) ([]byte, error)
	Close() error
}

// IDBShellDumper is the production IDBDumper — shells out to the
// `idb` binary. UDID is mandatory (targets one device; multi-device
// hosts must pick which to inspect).
type IDBShellDumper struct {
	UDID string // e.g. "00008030-001234567890123A"
	// Path is the idb binary location. Empty → looked up on PATH.
	Path string
}

// Dump runs `idb ui describe-all --udid <UDID> --json` and returns
// stdout. Wraps errors with ErrNotAvailable so callers can cleanly
// detect "no idb / device offline" environments.
func (d *IDBShellDumper) Dump(ctx context.Context) ([]byte, error) {
	if d.UDID == "" {
		return nil, errors.New("axtree/ios: IDBShellDumper.UDID must be set")
	}
	idb := d.Path
	if idb == "" {
		idb = "idb"
	}
	cmd := exec.CommandContext(ctx, idb, "ui", "describe-all", "--udid", d.UDID, "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotAvailable, err)
	}
	return out, nil
}

// Close is a no-op for the shell dumper.
func (*IDBShellDumper) Close() error { return nil }

// IOSSnapshotter fetches the idb UI dump via an IDBDumper and parses
// it into a *Node tree with Platform = PlatformIOS.
type IOSSnapshotter struct {
	dumper IDBDumper
	mu     sync.Mutex
	closed bool
}

// NewIOSSnapshotter binds to an IDBShellDumper targeting the given
// UDID. For tests, NewIOSSnapshotterWithDumper injects a mock
// IDBDumper directly.
func NewIOSSnapshotter(udid string) *IOSSnapshotter {
	return &IOSSnapshotter{dumper: &IDBShellDumper{UDID: udid}}
}

// NewIOSSnapshotterWithDumper wires a snapshotter to any IDBDumper
// implementation.
func NewIOSSnapshotterWithDumper(d IDBDumper) *IOSSnapshotter {
	return &IOSSnapshotter{dumper: d}
}

// Snapshot parses idb's JSON output into a *Node tree. idb's
// describe-all emits either a single root object or a JSON array
// of siblings (multiple top-level windows); the parser handles both,
// wrapping an array in a synthetic application-role root.
func (s *IOSSnapshotter) Snapshot(ctx context.Context) (*Node, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrSnapshotClosed
	}
	s.mu.Unlock()

	raw, err := s.dumper.Dump(ctx)
	if err != nil {
		return nil, fmt.Errorf("axtree/ios: Dump: %w", err)
	}
	root, err := parseIDBDump(raw)
	if err != nil {
		return nil, fmt.Errorf("axtree/ios: parse: %w", err)
	}
	return root, nil
}

// Close releases the dumper. Idempotent.
func (s *IOSSnapshotter) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.dumper.Close()
}

// ---------------------------------------------------------------------------
// idb JSON → Node parsing
// ---------------------------------------------------------------------------

// idbNode mirrors the relevant fields of idb's JSON output. idb
// actually emits a bit more (subrole, identifier, custom_actions…)
// but HelixQA only needs the fields that map to axtree.Node.
type idbNode struct {
	Type       string    `json:"type"`
	Title      string    `json:"title"`
	Label      string    `json:"label"`
	Value      string    `json:"value"`
	Identifier string    `json:"identifier"`
	Enabled    bool      `json:"enabled"`
	Focused    bool      `json:"focused"`
	Selected   bool      `json:"selected"`
	Frame      idbFrame  `json:"frame"`
	Children   []idbNode `json:"children"`
}

type idbFrame struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// parseIDBDump accepts either a JSON object (single root) or a JSON
// array (multiple top-level windows). Returns the materialized
// *Node tree.
func parseIDBDump(raw []byte) (*Node, error) {
	trimmed := []byte(strings.TrimSpace(string(raw)))
	if len(trimmed) == 0 {
		return nil, ErrNoRoot
	}

	switch trimmed[0] {
	case '[':
		var arr []idbNode
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return nil, err
		}
		if len(arr) == 0 {
			return nil, ErrNoRoot
		}
		if len(arr) == 1 {
			n := convertIDBNode(arr[0])
			return n, nil
		}
		children := make([]*Node, 0, len(arr))
		for _, c := range arr {
			children = append(children, convertIDBNode(c))
		}
		return &Node{
			Role:     "application",
			Name:     "ios-ui",
			Platform: PlatformIOS,
			Children: children,
		}, nil

	case '{':
		var obj idbNode
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return nil, err
		}
		return convertIDBNode(obj), nil

	default:
		return nil, fmt.Errorf("axtree/ios: expected JSON object or array, got %q", trimmed[0])
	}
}

// convertIDBNode materializes a single idbNode (recursively) into
// *Node. Naming precedence matches AX conventions: AXLabel ("label")
// is the accessibility-primary name, falling back to title.
func convertIDBNode(n idbNode) *Node {
	name := strings.TrimSpace(n.Label)
	if name == "" {
		name = strings.TrimSpace(n.Title)
	}

	bounds := image.Rect(
		int(n.Frame.X),
		int(n.Frame.Y),
		int(n.Frame.X+n.Frame.Width),
		int(n.Frame.Y+n.Frame.Height),
	)

	rawID := n.Identifier
	if rawID == "" {
		// Fall back to type + title so action resolution always has
		// a handle, even on widgets without accessibility identifiers.
		rawID = n.Type + ":" + n.Title
	}

	out := &Node{
		Role:     iosTypeToARIA(n.Type),
		Name:     name,
		Value:    n.Value,
		Bounds:   bounds,
		Enabled:  n.Enabled,
		Focused:  n.Focused,
		Selected: n.Selected,
		Platform: PlatformIOS,
		RawID:    rawID,
	}
	for _, child := range n.Children {
		out.Children = append(out.Children, convertIDBNode(child))
	}
	return out
}

// iosTypeToARIA maps idb's `type` field (which mirrors AXUIElement
// role strings) to the normalized ARIA vocabulary. Unknown types
// fall through as-is.
func iosTypeToARIA(t string) string {
	switch t {
	case "Application":
		return "application"
	case "Window":
		return "window"
	case "Button", "ButtonElement":
		return "button"
	case "StaticText", "Text":
		return "text"
	case "TextField", "SecureTextField", "TextFieldElement":
		return "textbox"
	case "TextView":
		return "textbox"
	case "Image", "Icon":
		return "image"
	case "Switch":
		return "switch"
	case "Slider":
		return "slider"
	case "ProgressIndicator":
		return "progressbar"
	case "Picker", "PickerWheel":
		return "combobox"
	case "Cell", "TableCell":
		return "cell"
	case "Table":
		return "table"
	case "CollectionView":
		return "list"
	case "ScrollView":
		return "list"
	case "NavigationBar":
		return "navigation"
	case "Toolbar":
		return "toolbar"
	case "TabBar":
		return "tablist"
	case "Tab":
		return "tab"
	case "Alert":
		return "dialog"
	case "Link":
		return "link"
	case "WebView":
		return "document"
	case "SearchField":
		return "textbox"
	case "Other":
		return "group"
	}
	return t
}

// Compile-time guards.
var (
	_ Snapshotter = (*IOSSnapshotter)(nil)
	_ IDBDumper   = (*IDBShellDumper)(nil)
)
