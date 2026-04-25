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

// DarwinFetcher emits a raw JSON snapshot of the frontmost macOS
// app's accessibility tree. The default implementation is an HTTP
// client against a local Swift sidecar
// (cmd/helixqa-axtree-darwin/, future) that walks the AXUIElement
// tree via the macOS Accessibility API and returns the JSON tree
// described by darwinNode. Tests inject a fake that returns canned
// JSON.
//
// The Swift-sidecar-over-HTTP pattern (same as pkg/agent/omniparser)
// keeps the HelixQA Go host CGO-free AND avoids the need for a
// second Go client that talks directly to the macOS C API — the
// Swift sidecar handles the Accessibility entitlement + private API
// surface, the Go client handles the JSON.
type DarwinFetcher interface {
	Fetch(ctx context.Context) ([]byte, error)
	Close() error
}

// DarwinHTTPFetcher is the production DarwinFetcher. It GETs the
// configured sidecar URL and returns the body. Zero-value HTTPClient
// uses a 5-second timeout (local sidecar latency budget).
type DarwinHTTPFetcher struct {
	// URL is the Swift sidecar snapshot endpoint, e.g.
	// "http://127.0.0.1:17420/snapshot". Required.
	URL string

	// HTTPClient is the underlying transport. Default: 5s timeout.
	HTTPClient *http.Client
}

// Fetch GETs the sidecar URL and returns the body bytes.
func (f *DarwinHTTPFetcher) Fetch(ctx context.Context) ([]byte, error) {
	if f.URL == "" {
		return nil, errors.New("axtree/darwin: DarwinHTTPFetcher.URL must be set")
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
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("axtree/darwin: HTTP %d: %s", resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

// Close is a no-op for the HTTP fetcher.
func (*DarwinHTTPFetcher) Close() error { return nil }

// DarwinSnapshotter walks the macOS accessibility tree via a
// DarwinFetcher and emits a materialized *Node tree with
// Platform = PlatformDarwin.
type DarwinSnapshotter struct {
	fetcher DarwinFetcher
	mu      sync.Mutex
	closed  bool
}

// NewDarwinSnapshotter binds to a DarwinHTTPFetcher at the given
// URL. For tests, NewDarwinSnapshotterWithFetcher injects a mock.
func NewDarwinSnapshotter(url string) *DarwinSnapshotter {
	return &DarwinSnapshotter{fetcher: &DarwinHTTPFetcher{URL: url}}
}

// NewDarwinSnapshotterWithFetcher wires a snapshotter to any
// DarwinFetcher implementation.
func NewDarwinSnapshotterWithFetcher(f DarwinFetcher) *DarwinSnapshotter {
	return &DarwinSnapshotter{fetcher: f}
}

// Snapshot calls the DarwinFetcher and parses the JSON tree.
func (s *DarwinSnapshotter) Snapshot(ctx context.Context) (*Node, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrSnapshotClosed
	}
	s.mu.Unlock()

	raw, err := s.fetcher.Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("axtree/darwin: Fetch: %w", err)
	}
	root, err := parseDarwinDump(raw)
	if err != nil {
		return nil, fmt.Errorf("axtree/darwin: parse: %w", err)
	}
	return root, nil
}

// Close releases the fetcher. Idempotent.
func (s *DarwinSnapshotter) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.fetcher.Close()
}

// ---------------------------------------------------------------------------
// macOS Accessibility JSON → Node parsing
// ---------------------------------------------------------------------------

// darwinNode mirrors the fields the Swift sidecar emits for each
// AXUIElement. The sidecar documentation governs the exact wire
// format; this struct captures the minimum the Go layer needs.
type darwinNode struct {
	Role        string       `json:"role"`        // AXRole, e.g. "AXButton"
	Title       string       `json:"title"`       // AXTitle
	Description string       `json:"description"` // AXDescription
	Value       string       `json:"value"`       // AXValue (string form)
	Identifier  string       `json:"identifier"`  // AXIdentifier
	Enabled     bool         `json:"enabled"`
	Focused     bool         `json:"focused"`
	Selected    bool         `json:"selected"`
	Frame       darwinFrame  `json:"frame"`
	Children    []darwinNode `json:"children"`
}

type darwinFrame struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// parseDarwinDump parses the Swift sidecar's JSON output. Like the
// iOS parser, it accepts either a single root object or an array of
// top-level windows; empty input surfaces ErrNoRoot.
func parseDarwinDump(raw []byte) (*Node, error) {
	trimmed := []byte(strings.TrimSpace(string(raw)))
	if len(trimmed) == 0 {
		return nil, ErrNoRoot
	}

	switch trimmed[0] {
	case '[':
		var arr []darwinNode
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return nil, err
		}
		if len(arr) == 0 {
			return nil, ErrNoRoot
		}
		if len(arr) == 1 {
			return convertDarwinNode(arr[0]), nil
		}
		children := make([]*Node, 0, len(arr))
		for _, c := range arr {
			children = append(children, convertDarwinNode(c))
		}
		return &Node{
			Role:     "application",
			Name:     "darwin-ui",
			Platform: PlatformDarwin,
			Children: children,
		}, nil

	case '{':
		var obj darwinNode
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return nil, err
		}
		return convertDarwinNode(obj), nil

	default:
		return nil, fmt.Errorf("axtree/darwin: expected JSON object or array, got %q", trimmed[0])
	}
}

// convertDarwinNode materializes a single darwinNode (recursively)
// into *Node. Name precedence: title → description, matching macOS
// AX conventions where AXTitle is the primary accessible name.
func convertDarwinNode(n darwinNode) *Node {
	name := strings.TrimSpace(n.Title)
	if name == "" {
		name = strings.TrimSpace(n.Description)
	}

	bounds := image.Rect(
		int(n.Frame.X),
		int(n.Frame.Y),
		int(n.Frame.X+n.Frame.Width),
		int(n.Frame.Y+n.Frame.Height),
	)

	rawID := n.Identifier
	if rawID == "" {
		rawID = n.Role + ":" + n.Title
	}

	out := &Node{
		Role:     darwinRoleToARIA(n.Role),
		Name:     name,
		Value:    n.Value,
		Bounds:   bounds,
		Enabled:  n.Enabled,
		Focused:  n.Focused,
		Selected: n.Selected,
		Platform: PlatformDarwin,
		RawID:    rawID,
	}
	for _, child := range n.Children {
		out.Children = append(out.Children, convertDarwinNode(child))
	}
	return out
}

// darwinRoleToARIA maps macOS AXRole strings to the normalized
// ARIA vocabulary. macOS role names are more granular than
// iOS/UIKit — AXSheet, AXPopover, AXDrawer etc. — so the map is
// larger. Unknown roles fall through as-is with the AX prefix
// stripped (AXFoo → Foo).
func darwinRoleToARIA(role string) string {
	switch role {
	case "AXApplication":
		return "application"
	case "AXWindow":
		return "window"
	case "AXSheet", "AXDialog":
		return "dialog"
	case "AXPopover":
		return "dialog"
	case "AXButton", "AXPopUpButton", "AXDisclosureTriangle":
		return "button"
	case "AXStaticText":
		return "text"
	case "AXTextField", "AXSecureTextField", "AXSearchField":
		return "textbox"
	case "AXTextArea":
		return "textbox"
	case "AXImage":
		return "image"
	case "AXCheckBox":
		return "checkbox"
	case "AXRadioButton":
		return "radio"
	case "AXSwitch":
		return "switch"
	case "AXProgressIndicator":
		return "progressbar"
	case "AXSlider":
		return "slider"
	case "AXStepper":
		return "spinbutton"
	case "AXComboBox":
		return "combobox"
	case "AXList", "AXOutline":
		return "list"
	case "AXTable":
		return "table"
	case "AXRow":
		return "row"
	case "AXCell":
		return "cell"
	case "AXColumn":
		return "columnheader"
	case "AXGroup":
		return "group"
	case "AXScrollArea":
		return "list"
	case "AXToolbar":
		return "toolbar"
	case "AXMenuBar":
		return "menubar"
	case "AXMenu":
		return "menu"
	case "AXMenuItem", "AXMenuButton":
		return "menuitem"
	case "AXTabGroup":
		return "tablist"
	case "AXRadioGroup":
		return "radiogroup"
	case "AXLink":
		return "link"
	case "AXWebArea":
		return "document"
	case "AXBrowser":
		return "document"
	case "AXSplitGroup":
		return "group"
	case "AXSplitter":
		return "separator"
	case "AXDrawer":
		return "complementary"
	case "AXHelpTag":
		return "tooltip"
	}
	// Strip the AX prefix so unknown roles stay readable.
	if strings.HasPrefix(role, "AX") {
		return strings.ToLower(role[2:])
	}
	return role
}

// Compile-time guards.
var (
	_ Snapshotter   = (*DarwinSnapshotter)(nil)
	_ DarwinFetcher = (*DarwinHTTPFetcher)(nil)
)
