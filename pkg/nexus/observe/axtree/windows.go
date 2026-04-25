// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package axtree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// WindowsFetcher emits a raw JSON snapshot of the frontmost Windows
// app's UI Automation tree. The default implementation is an HTTP
// client against a local Windows sidecar
// (cmd/helixqa-axtree-windows/, future) that walks the IUIAutomation
// tree via the Microsoft UIA COM API and returns the JSON described
// by windowsNode. Tests inject a fake that returns canned JSON.
//
// Same HTTP-over-sidecar pattern as pkg/agent/omniparser and
// pkg/nexus/observe/axtree/darwin — the Go host stays CGO/COM-free
// while the Windows sidecar handles the private COM surface with
// full platform APIs.
type WindowsFetcher interface {
	Fetch(ctx context.Context) ([]byte, error)
	Close() error
}

// WindowsHTTPFetcher is the production WindowsFetcher. Same shape
// as DarwinHTTPFetcher (see darwin.go) — a 5-second default timeout
// HTTP GET against the configured URL.
type WindowsHTTPFetcher struct {
	URL        string       // required
	HTTPClient *http.Client // default 5s timeout
}

// Fetch GETs the sidecar URL and returns the body.
func (f *WindowsHTTPFetcher) Fetch(ctx context.Context) ([]byte, error) {
	if f.URL == "" {
		return nil, errors.New("axtree/windows: WindowsHTTPFetcher.URL must be set")
	}
	client := f.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotAvailable, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotAvailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("axtree/windows: HTTP %d: %s", resp.StatusCode, string(body))
	}
	return io.ReadAll(resp.Body)
}

// Close is a no-op for the HTTP fetcher.
func (*WindowsHTTPFetcher) Close() error { return nil }

// WindowsSnapshotter walks the Windows UIA tree via a WindowsFetcher
// and emits a materialized *Node tree with Platform = PlatformWindows.
type WindowsSnapshotter struct {
	fetcher WindowsFetcher
	mu      sync.Mutex
	closed  bool
}

// NewWindowsSnapshotter binds to a WindowsHTTPFetcher at the given
// URL. For tests, NewWindowsSnapshotterWithFetcher injects a mock.
func NewWindowsSnapshotter(url string) *WindowsSnapshotter {
	return &WindowsSnapshotter{fetcher: &WindowsHTTPFetcher{URL: url}}
}

// NewWindowsSnapshotterWithFetcher wires a snapshotter to any
// WindowsFetcher implementation.
func NewWindowsSnapshotterWithFetcher(f WindowsFetcher) *WindowsSnapshotter {
	return &WindowsSnapshotter{fetcher: f}
}

// Snapshot calls the WindowsFetcher and parses the JSON tree.
func (s *WindowsSnapshotter) Snapshot(ctx context.Context) (*Node, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrSnapshotClosed
	}
	s.mu.Unlock()

	raw, err := s.fetcher.Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("axtree/windows: Fetch: %w", err)
	}
	return parseWindowsDump(raw)
}

// Close releases the fetcher. Idempotent.
func (s *WindowsSnapshotter) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.fetcher.Close()
}

// ---------------------------------------------------------------------------
// Windows UIA JSON → Node parsing
// ---------------------------------------------------------------------------

// windowsNode mirrors the JSON shape the Windows sidecar emits for
// each IUIAutomationElement. ControlType accepts either a symbolic
// name ("Button", "Window") or the UIA numeric ID as a string
// ("50000") — both forms are mapped by windowsControlTypeToARIA.
type windowsNode struct {
	ControlType       string           `json:"controlType"`
	Name              string           `json:"name"`
	AutomationID      string           `json:"automationId"`
	ClassName         string           `json:"className"`
	Value             string           `json:"value"`
	HelpText          string           `json:"helpText"`
	IsEnabled         bool             `json:"isEnabled"`
	HasKeyboardFocus  bool             `json:"hasKeyboardFocus"`
	IsSelected        bool             `json:"isSelected"`
	BoundingRectangle [4]float64       `json:"boundingRectangle"` // [x, y, w, h]
	Children          []windowsNode    `json:"children"`
}

// parseWindowsDump accepts either a JSON object (single root) or a
// JSON array (multiple top-level windows, rare on Windows but
// possible with MDI or attached overlays).
func parseWindowsDump(raw []byte) (*Node, error) {
	trimmed := []byte(strings.TrimSpace(string(raw)))
	if len(trimmed) == 0 {
		return nil, ErrNoRoot
	}

	switch trimmed[0] {
	case '[':
		var arr []windowsNode
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return nil, err
		}
		if len(arr) == 0 {
			return nil, ErrNoRoot
		}
		if len(arr) == 1 {
			return convertWindowsNode(arr[0]), nil
		}
		children := make([]*Node, 0, len(arr))
		for _, c := range arr {
			children = append(children, convertWindowsNode(c))
		}
		return &Node{
			Role:     "application",
			Name:     "windows-ui",
			Platform: PlatformWindows,
			Children: children,
		}, nil
	case '{':
		var obj windowsNode
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return nil, err
		}
		return convertWindowsNode(obj), nil
	default:
		return nil, fmt.Errorf("axtree/windows: expected JSON object or array, got %q", trimmed[0])
	}
}

// convertWindowsNode materializes a single windowsNode (recursively)
// into *Node. Name precedence: Name → HelpText → AutomationID
// (UIA conventions — Name is the primary accessible name, HelpText
// is the tooltip/description, AutomationID is the programmatic ID).
func convertWindowsNode(n windowsNode) *Node {
	name := strings.TrimSpace(n.Name)
	if name == "" {
		name = strings.TrimSpace(n.HelpText)
	}
	if name == "" {
		name = strings.TrimSpace(n.AutomationID)
	}

	// UIA bounding rectangle is [x, y, width, height].
	x := int(n.BoundingRectangle[0])
	y := int(n.BoundingRectangle[1])
	w := int(n.BoundingRectangle[2])
	h := int(n.BoundingRectangle[3])
	bounds := image.Rect(x, y, x+w, y+h)

	rawID := n.AutomationID
	if rawID == "" {
		// Fall back to ClassName + Name so action resolution always
		// has a handle on widgets that lack an AutomationID (custom
		// controls, DirectUI, etc.).
		rawID = n.ClassName + ":" + n.Name
	}

	out := &Node{
		Role:     windowsControlTypeToARIA(n.ControlType),
		Name:     name,
		Value:    n.Value,
		Bounds:   bounds,
		Enabled:  n.IsEnabled,
		Focused:  n.HasKeyboardFocus,
		Selected: n.IsSelected,
		Platform: PlatformWindows,
		RawID:    rawID,
	}
	for _, child := range n.Children {
		out.Children = append(out.Children, convertWindowsNode(child))
	}
	return out
}

// windowsControlTypeToARIA maps UIA ControlType strings to the
// normalized ARIA vocabulary. Handles both the symbolic name
// ("Button") and the numeric ID ("50000") — Windows sidecars in the
// wild emit either form.
func windowsControlTypeToARIA(ct string) string {
	switch ct {
	// Symbolic names (Microsoft Docs canonical forms).
	case "Button":
		return "button"
	case "Window":
		return "window"
	case "Pane":
		return "group"
	case "Edit":
		return "textbox"
	case "Text":
		return "text"
	case "Document":
		return "document"
	case "Image":
		return "image"
	case "CheckBox":
		return "checkbox"
	case "RadioButton":
		return "radio"
	case "ComboBox":
		return "combobox"
	case "List":
		return "list"
	case "ListItem":
		return "listitem"
	case "Tree":
		return "tree"
	case "TreeItem":
		return "treeitem"
	case "Table":
		return "table"
	case "DataGrid":
		return "grid"
	case "Menu":
		return "menu"
	case "MenuBar":
		return "menubar"
	case "MenuItem":
		return "menuitem"
	case "Tab":
		return "tablist"
	case "TabItem":
		return "tab"
	case "ProgressBar":
		return "progressbar"
	case "Slider":
		return "slider"
	case "Spinner":
		return "spinbutton"
	case "ToolBar":
		return "toolbar"
	case "Hyperlink":
		return "link"
	case "Group":
		return "group"
	case "Header":
		return "header"
	case "HeaderItem":
		return "columnheader"
	case "Separator":
		return "separator"
	case "ScrollBar":
		return "scrollbar"
	case "Calendar":
		return "grid"
	case "Custom":
		return "group"

	// Numeric IDs (UIAutomationCore.h ControlTypeIds). See
	// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-controltype-ids.
	case "50000":
		return "button"
	case "50002":
		return "checkbox"
	case "50003":
		return "combobox"
	case "50004":
		return "textbox"
	case "50006":
		return "image"
	case "50008":
		return "list"
	case "50009":
		return "menu"
	case "50011":
		return "menuitem"
	case "50012":
		return "progressbar"
	case "50015":
		return "slider"
	case "50018":
		return "tablist"
	case "50020":
		return "text"
	case "50021":
		return "toolbar"
	case "50023":
		return "tree"
	case "50026":
		return "group"
	case "50030":
		return "document"
	case "50032":
		return "window"
	case "50033":
		return "group"
	case "50038":
		return "grid"
	}

	// Unknown control type: return it as-is so the grounding layer
	// can still reason about custom controls.
	return ct
}

// Compile-time guards.
var (
	_ Snapshotter    = (*WindowsSnapshotter)(nil)
	_ WindowsFetcher = (*WindowsHTTPFetcher)(nil)
)
