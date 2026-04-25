// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package axtree

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// AndroidDumper emits a raw UI Automator dump for a specific device. The
// default implementation shells out to `adb -s <serial> exec-out uiautomator
// dump /dev/tty` which returns the XML tree on stdout without touching
// /sdcard. Tests inject a fake that returns canned XML.
type AndroidDumper interface {
	// Dump fetches the current UI hierarchy. ctx cancellation must
	// interrupt the underlying subprocess or HTTP call.
	Dump(ctx context.Context) ([]byte, error)
	Close() error
}

// ADBDumper is the production AndroidDumper: it shells out via the `adb`
// binary on the operator's path. The serial field is mandatory — multi-
// device hosts must always target a specific device, and a common QA bug
// is "accidentally dumped the wrong device" when serial is empty.
type ADBDumper struct {
	Serial string
	// Path is the adb binary location. Empty → looked up on PATH.
	Path string
}

// Dump runs `adb -s <Serial> exec-out uiautomator dump /dev/tty` and
// returns stdout (the UI XML). Returns ErrNotAvailable wrapped with the
// underlying error if adb is not on the path or the device is offline.
func (d *ADBDumper) Dump(ctx context.Context) ([]byte, error) {
	if d.Serial == "" {
		return nil, errors.New("axtree/android: ADBDumper.Serial must be set")
	}
	adb := d.Path
	if adb == "" {
		adb = "adb"
	}
	cmd := exec.CommandContext(ctx, adb, "-s", d.Serial, "exec-out", "uiautomator", "dump", "/dev/tty")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotAvailable, err)
	}
	// uiautomator dump prints a status line before the XML — strip
	// everything before the first `<?xml` marker.
	if i := strings.Index(string(out), "<?xml"); i >= 0 {
		out = out[i:]
	}
	return out, nil
}

// Close is a no-op for the ADB dumper. Present so AndroidDumper matches
// the same lifecycle shape as LinuxBus.
func (*ADBDumper) Close() error { return nil }

// AndroidSnapshotter fetches the UIAutomator dump via an AndroidDumper
// and parses it into a *Node tree with Platform = PlatformAndroid.
type AndroidSnapshotter struct {
	dumper AndroidDumper
	mu     sync.Mutex
	closed bool
}

// NewAndroidSnapshotter binds to an ADBDumper targeting the given
// device serial. For tests, NewAndroidSnapshotterWithDumper injects a
// mock AndroidDumper directly.
func NewAndroidSnapshotter(serial string) *AndroidSnapshotter {
	return &AndroidSnapshotter{dumper: &ADBDumper{Serial: serial}}
}

// NewAndroidSnapshotterWithDumper wires a snapshotter to any
// AndroidDumper implementation — useful for tests and for callers that
// want to share a dumper across multiple snapshots.
func NewAndroidSnapshotterWithDumper(d AndroidDumper) *AndroidSnapshotter {
	return &AndroidSnapshotter{dumper: d}
}

// Snapshot pulls the current UI hierarchy from the device and returns a
// materialized *Node tree. The top-level Android hierarchy element
// becomes the Node root; the `rotation` attribute is preserved in the
// root's Value.
func (s *AndroidSnapshotter) Snapshot(ctx context.Context) (*Node, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrSnapshotClosed
	}
	s.mu.Unlock()

	raw, err := s.dumper.Dump(ctx)
	if err != nil {
		return nil, fmt.Errorf("axtree/android: Dump: %w", err)
	}
	root, err := parseAndroidDump(raw)
	if err != nil {
		return nil, fmt.Errorf("axtree/android: parse: %w", err)
	}
	return root, nil
}

// Close releases the dumper. Idempotent.
func (s *AndroidSnapshotter) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.dumper.Close()
}

// ---------------------------------------------------------------------------
// XML → Node parsing
// ---------------------------------------------------------------------------

type xmlHierarchy struct {
	XMLName  xml.Name  `xml:"hierarchy"`
	Rotation string    `xml:"rotation,attr"`
	Nodes    []xmlNode `xml:"node"`
}

type xmlNode struct {
	Index       string    `xml:"index,attr"`
	Text        string    `xml:"text,attr"`
	ResourceID  string    `xml:"resource-id,attr"`
	Class       string    `xml:"class,attr"`
	Package     string    `xml:"package,attr"`
	ContentDesc string    `xml:"content-desc,attr"`
	Checkable   string    `xml:"checkable,attr"`
	Checked     string    `xml:"checked,attr"`
	Clickable   string    `xml:"clickable,attr"`
	Enabled     string    `xml:"enabled,attr"`
	Focusable   string    `xml:"focusable,attr"`
	Focused     string    `xml:"focused,attr"`
	Scrollable  string    `xml:"scrollable,attr"`
	Selected    string    `xml:"selected,attr"`
	Bounds      string    `xml:"bounds,attr"`
	Children    []xmlNode `xml:"node"`
}

// parseAndroidDump converts UIAutomator dump XML into an axtree.Node
// tree. The top-level <hierarchy> wraps one or more <node> children —
// the usual case is a single application-level node, so we return that
// directly. If there are multiple top-level nodes (rare), we wrap them
// in a synthetic root.
func parseAndroidDump(raw []byte) (*Node, error) {
	var h xmlHierarchy
	if err := xml.Unmarshal(raw, &h); err != nil {
		return nil, err
	}
	if len(h.Nodes) == 0 {
		return nil, ErrNoRoot
	}
	if len(h.Nodes) == 1 {
		root := convertXMLNode(h.Nodes[0])
		root.Value = "rotation=" + h.Rotation
		return root, nil
	}
	children := make([]*Node, 0, len(h.Nodes))
	for _, c := range h.Nodes {
		children = append(children, convertXMLNode(c))
	}
	return &Node{
		Role:     "application",
		Name:     "hierarchy",
		Value:    "rotation=" + h.Rotation,
		Platform: PlatformAndroid,
		Children: children,
	}, nil
}

// convertXMLNode materializes a single <node> (recursively) into *Node.
func convertXMLNode(x xmlNode) *Node {
	name := strings.TrimSpace(x.Text)
	if name == "" {
		name = strings.TrimSpace(x.ContentDesc)
	}
	n := &Node{
		Role:     androidClassToARIA(x.Class),
		Name:     name,
		Value:    "", // left blank at leaf level; rotations live only on the root
		Bounds:   parseAndroidBounds(x.Bounds),
		Enabled:  boolAttr(x.Enabled),
		Focused:  boolAttr(x.Focused),
		Selected: boolAttr(x.Selected),
		Platform: PlatformAndroid,
		RawID:    x.ResourceID,
	}
	if n.RawID == "" {
		// Fall back to the class + index when no resource-id exists
		// (common in system dialogs and custom views) so action
		// resolution always has *some* handle.
		n.RawID = x.Class + ":" + x.Index
	}
	for _, child := range x.Children {
		n.Children = append(n.Children, convertXMLNode(child))
	}
	return n
}

var boundsRegexp = regexp.MustCompile(`^\[(-?\d+),(-?\d+)\]\[(-?\d+),(-?\d+)\]$`)

// parseAndroidBounds converts the UIAutomator "[x1,y1][x2,y2]" bounds
// syntax to image.Rectangle. Returns the zero rect on malformed input.
func parseAndroidBounds(s string) image.Rectangle {
	m := boundsRegexp.FindStringSubmatch(s)
	if len(m) != 5 {
		return image.Rectangle{}
	}
	x1, _ := strconv.Atoi(m[1])
	y1, _ := strconv.Atoi(m[2])
	x2, _ := strconv.Atoi(m[3])
	y2, _ := strconv.Atoi(m[4])
	return image.Rect(x1, y1, x2, y2)
}

// boolAttr parses a UIAutomator boolean attribute ("true"/"false").
// Anything other than "true" is false — matches UIAutomator's behavior
// of emitting the literal strings.
func boolAttr(s string) bool { return s == "true" }

// androidClassToARIA maps the common android.widget / android.view class
// names to the normalized ARIA role vocabulary. Unknown classes fall
// back to the raw class name so the grounding VLM can still reason
// about them; over time we extend the table as new widgets appear.
func androidClassToARIA(cls string) string {
	base := cls
	if i := strings.LastIndex(cls, "."); i >= 0 {
		base = cls[i+1:]
	}
	switch base {
	case "FrameLayout", "LinearLayout", "RelativeLayout", "ConstraintLayout",
		"CoordinatorLayout", "ViewGroup", "View":
		return "group"
	case "Button", "ImageButton", "MaterialButton":
		return "button"
	case "TextView", "AppCompatTextView", "MaterialTextView":
		return "text"
	case "EditText", "AppCompatEditText", "TextInputEditText":
		return "textbox"
	case "ImageView", "AppCompatImageView":
		return "image"
	case "CheckBox", "AppCompatCheckBox":
		return "checkbox"
	case "RadioButton", "AppCompatRadioButton":
		return "radio"
	case "Switch", "SwitchMaterial", "SwitchCompat":
		return "switch"
	case "ProgressBar":
		return "progressbar"
	case "SeekBar":
		return "slider"
	case "Toolbar":
		return "toolbar"
	case "Spinner":
		return "combobox"
	case "RecyclerView", "ListView", "ScrollView", "HorizontalScrollView":
		return "list"
	case "ViewPager", "ViewPager2":
		return "tabpanel"
	case "TabLayout", "TabWidget":
		return "tablist"
	case "WebView":
		return "document"
	case "DrawerLayout":
		return "navigation"
	}
	return base
}

// Compile-time guards.
var (
	_ Snapshotter   = (*AndroidSnapshotter)(nil)
	_ AndroidDumper = (*ADBDumper)(nil)
)
