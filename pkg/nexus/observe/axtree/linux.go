// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package axtree

import (
	"context"
	"errors"
	"fmt"
	"image"
	"strings"
	"sync"

	dbus "github.com/godbus/dbus/v5"
)

// AT-SPI2 service / path / interface constants. The AT-SPI bus is
// discovered at runtime via `org.a11y.Bus.GetAddress` on the session bus.
const (
	a11yBusService = "org.a11y.Bus"
	a11yBusPath    = "/org/a11y/bus"
	a11yBusIface   = "org.a11y.Bus"

	atspiRootPath = "/org/a11y/atspi/accessible/root"

	atspiAccessibleIface = "org.a11y.atspi.Accessible"
	atspiComponentIface  = "org.a11y.atspi.Component"
	atspiValueIface      = "org.a11y.atspi.Value"

	// AT-SPI2 coordinate type — ATSPI_COORD_TYPE_SCREEN = 0.
	atspiCoordTypeScreen = uint32(0)
)

// LinuxBus is the narrow D-Bus abstraction LinuxSnapshotter consumes.
// Production code uses LinuxBusGodbus wrapping two *dbus.Conn (session +
// a11y); tests inject a fake bus that returns scripted trees.
type LinuxBus interface {
	// GetRoot returns the destination + object path for the AT-SPI2 root
	// accessible. The destination is the connection name of the registry
	// process (org.a11y.atspi.Registry), discovered once at connect time.
	GetRoot(ctx context.Context) (dest string, path dbus.ObjectPath, err error)

	// GetChildren returns the (dest, path) pairs of every child of the
	// given accessible. AT-SPI2 `GetChildren` returns an array of
	// (string, object_path) tuples.
	GetChildren(ctx context.Context, dest string, path dbus.ObjectPath) ([]Accessible, error)

	// GetProps reads the accessibility-relevant properties of a single
	// accessible. Callers can ignore individual errors — props carries
	// zero values for any field the backend failed to report.
	GetProps(ctx context.Context, dest string, path dbus.ObjectPath) (NodeProps, error)

	// Close releases both underlying connections (session + a11y).
	Close() error
}

// Accessible identifies an AT-SPI2 object: (connection name, object path).
type Accessible struct {
	Dest string
	Path dbus.ObjectPath
}

// NodeProps carries the raw accessibility-relevant fields of a single
// AT-SPI2 Accessible. Converted to a *Node by LinuxSnapshotter.Snapshot.
type NodeProps struct {
	Role     string // normalized to ARIA name
	Name     string
	Value    string
	Bounds   image.Rectangle
	Enabled  bool
	Focused  bool
	Selected bool
}

// LinuxSnapshotter walks the AT-SPI2 accessibility tree over D-Bus and
// emits a materialized *Node tree.
type LinuxSnapshotter struct {
	bus    LinuxBus
	mu     sync.Mutex
	closed bool
}

// NewLinuxSnapshotter binds to the session + a11y buses via
// LinuxBusGodbus.Connect. For tests, NewLinuxSnapshotterWithBus injects a
// mock LinuxBus directly.
func NewLinuxSnapshotter(ctx context.Context) (*LinuxSnapshotter, error) {
	bus, err := ConnectLinuxBus(ctx)
	if err != nil {
		return nil, fmt.Errorf("axtree/linux: connect: %w", err)
	}
	return NewLinuxSnapshotterWithBus(bus), nil
}

// NewLinuxSnapshotterWithBus wires a snapshotter to an arbitrary LinuxBus
// implementation. Useful for tests and for callers that want to share a
// D-Bus connection across multiple components.
func NewLinuxSnapshotterWithBus(bus LinuxBus) *LinuxSnapshotter {
	return &LinuxSnapshotter{bus: bus}
}

// Snapshot walks the AT-SPI2 tree from the root Accessible and returns a
// fully-materialized *Node tree.
func (s *LinuxSnapshotter) Snapshot(ctx context.Context) (*Node, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrSnapshotClosed
	}
	s.mu.Unlock()

	dest, rootPath, err := s.bus.GetRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("axtree/linux: GetRoot: %w", err)
	}
	if dest == "" || rootPath == "" {
		return nil, ErrNoRoot
	}

	return s.walk(ctx, Accessible{Dest: dest, Path: rootPath})
}

// walk materializes the subtree rooted at a. The recursion respects
// ctx cancellation; any per-child GetProps / GetChildren error is
// wrapped with the offending object path for diagnostics.
func (s *LinuxSnapshotter) walk(ctx context.Context, a Accessible) (*Node, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	props, err := s.bus.GetProps(ctx, a.Dest, a.Path)
	if err != nil {
		return nil, fmt.Errorf("axtree/linux: GetProps(%s): %w", a.Path, err)
	}
	children, err := s.bus.GetChildren(ctx, a.Dest, a.Path)
	if err != nil {
		return nil, fmt.Errorf("axtree/linux: GetChildren(%s): %w", a.Path, err)
	}

	node := &Node{
		Role:     props.Role,
		Name:     props.Name,
		Value:    props.Value,
		Bounds:   props.Bounds,
		Enabled:  props.Enabled,
		Focused:  props.Focused,
		Selected: props.Selected,
		Platform: PlatformLinux,
		RawID:    string(a.Path),
		Children: make([]*Node, 0, len(children)),
	}

	for _, c := range children {
		child, err := s.walk(ctx, c)
		if err != nil {
			return nil, err
		}
		node.Children = append(node.Children, child)
	}
	return node, nil
}

// Close releases the underlying LinuxBus and is idempotent.
func (s *LinuxSnapshotter) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.bus.Close()
}

// ---------------------------------------------------------------------------
// godbus-backed production LinuxBus. Wired only on Linux hosts with an
// active a11y bus; returns ErrNotAvailable when the a11y bus cannot be
// discovered.
// ---------------------------------------------------------------------------

// linuxBusGodbus connects to both the session bus (to resolve the a11y bus
// address) and the a11y bus itself (for the actual accessibility calls).
type linuxBusGodbus struct {
	session *dbus.Conn
	a11y    *dbus.Conn
}

// ConnectLinuxBus attempts the two-step connection dance that every
// AT-SPI2 client follows:
//
//  1. Connect to the session bus.
//  2. Call org.a11y.Bus.GetAddress on the session bus — the returned
//     address is the separate AT-SPI2 bus (typically a UNIX socket).
//  3. Dial that address and authenticate.
//
// Any step failure returns ErrNotAvailable wrapped with the underlying
// cause, so callers can cleanly detect "no accessibility bus here".
func ConnectLinuxBus(ctx context.Context) (LinuxBus, error) {
	session, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("%w: session bus: %v", ErrNotAvailable, err)
	}

	var addr string
	obj := session.Object(a11yBusService, a11yBusPath)
	call := obj.CallWithContext(ctx, a11yBusIface+".GetAddress", 0)
	if call.Err != nil {
		session.Close()
		return nil, fmt.Errorf("%w: a11y bus discovery: %v", ErrNotAvailable, call.Err)
	}
	if err := call.Store(&addr); err != nil {
		session.Close()
		return nil, fmt.Errorf("%w: a11y bus address: %v", ErrNotAvailable, err)
	}

	a11y, err := dbus.Dial(addr)
	if err != nil {
		session.Close()
		return nil, fmt.Errorf("%w: dial a11y bus: %v", ErrNotAvailable, err)
	}
	if err := a11y.Auth(nil); err != nil {
		session.Close()
		a11y.Close()
		return nil, fmt.Errorf("%w: auth a11y bus: %v", ErrNotAvailable, err)
	}
	if err := a11y.Hello(); err != nil {
		session.Close()
		a11y.Close()
		return nil, fmt.Errorf("%w: hello a11y bus: %v", ErrNotAvailable, err)
	}

	return &linuxBusGodbus{session: session, a11y: a11y}, nil
}

func (b *linuxBusGodbus) GetRoot(ctx context.Context) (string, dbus.ObjectPath, error) {
	// AT-SPI2's root is always at atspiRootPath under the destination of
	// the registry process. We resolve the registry connection name once
	// via the well-known name org.a11y.atspi.Registry.
	const regDest = "org.a11y.atspi.Registry"
	obj := b.a11y.Object(regDest, atspiRootPath)
	// Ping the root with GetRole to confirm it exists.
	var role uint32
	err := obj.CallWithContext(ctx, atspiAccessibleIface+".GetRole", 0).Store(&role)
	if err != nil {
		return "", "", err
	}
	return regDest, atspiRootPath, nil
}

func (b *linuxBusGodbus) GetChildren(ctx context.Context, dest string, path dbus.ObjectPath) ([]Accessible, error) {
	obj := b.a11y.Object(dest, path)
	var count int32
	if err := obj.CallWithContext(ctx, atspiAccessibleIface+".GetChildCount", 0).Store(&count); err != nil {
		return nil, err
	}
	out := make([]Accessible, 0, count)
	for i := int32(0); i < count; i++ {
		var child []any
		if err := obj.CallWithContext(ctx, atspiAccessibleIface+".GetChildAtIndex", 0, i).Store(&child); err != nil {
			return nil, err
		}
		// AT-SPI2 returns (s, o) — a string dest + an object path.
		if len(child) != 2 {
			return nil, fmt.Errorf("axtree/linux: GetChildAtIndex returned %d values, want 2", len(child))
		}
		cDest, ok := child[0].(string)
		if !ok {
			return nil, errors.New("axtree/linux: GetChildAtIndex returned non-string dest")
		}
		cPath, ok := child[1].(dbus.ObjectPath)
		if !ok {
			return nil, errors.New("axtree/linux: GetChildAtIndex returned non-ObjectPath path")
		}
		out = append(out, Accessible{Dest: cDest, Path: cPath})
	}
	return out, nil
}

func (b *linuxBusGodbus) GetProps(ctx context.Context, dest string, path dbus.ObjectPath) (NodeProps, error) {
	var props NodeProps

	obj := b.a11y.Object(dest, path)

	// Role. AT-SPI2 returns a uint32 role code; translate to ARIA.
	var roleCode uint32
	if err := obj.CallWithContext(ctx, atspiAccessibleIface+".GetRole", 0).Store(&roleCode); err == nil {
		props.Role = atspiRoleToARIA(roleCode)
	}

	// Name (property). AT-SPI2 accessibles expose `Name` via
	// org.freedesktop.DBus.Properties.
	var nameV dbus.Variant
	if err := obj.CallWithContext(ctx,
		"org.freedesktop.DBus.Properties.Get", 0,
		atspiAccessibleIface, "Name").Store(&nameV); err == nil {
		if s, ok := nameV.Value().(string); ok {
			props.Name = s
		}
	}

	// Bounds via Component interface.
	var extents []int32
	if err := obj.CallWithContext(ctx,
		atspiComponentIface+".GetExtents", 0,
		atspiCoordTypeScreen).Store(&extents); err == nil && len(extents) == 4 {
		props.Bounds = image.Rect(
			int(extents[0]), int(extents[1]),
			int(extents[0]+extents[2]), int(extents[1]+extents[3]),
		)
	}

	// State set — enabled/focused/selected. AT-SPI2 returns an array of
	// two uint32 bitmasks.
	var stateMask []uint32
	if err := obj.CallWithContext(ctx,
		atspiAccessibleIface+".GetState", 0).Store(&stateMask); err == nil && len(stateMask) == 2 {
		const (
			stateEnabled  = 10 // ATSPI_STATE_ENABLED
			stateFocused  = 12 // ATSPI_STATE_FOCUSED
			stateSelected = 28 // ATSPI_STATE_SELECTED
		)
		props.Enabled = stateBit(stateMask, stateEnabled)
		props.Focused = stateBit(stateMask, stateFocused)
		props.Selected = stateBit(stateMask, stateSelected)
	}

	// Value for controls that carry one.
	var val dbus.Variant
	if err := obj.CallWithContext(ctx,
		"org.freedesktop.DBus.Properties.Get", 0,
		atspiValueIface, "CurrentValue").Store(&val); err == nil {
		props.Value = fmt.Sprintf("%v", val.Value())
	}

	return props, nil
}

func (b *linuxBusGodbus) Close() error {
	var firstErr error
	if b.a11y != nil {
		if err := b.a11y.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if b.session != nil {
		if err := b.session.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// stateBit reads bit i from the two-uint32 AT-SPI state mask.
func stateBit(mask []uint32, bit int) bool {
	if bit < 0 || bit >= 64 || len(mask) != 2 {
		return false
	}
	word := bit / 32
	b := bit % 32
	return mask[word]&(uint32(1)<<b) != 0
}

// atspiRoleToARIA maps a subset of the AT-SPI2 role codes to the
// ARIA-normalized role names HelixQA uses cross-platform. Codes not
// listed are returned as "role-<N>" so they remain identifiable without
// a translation; over time we add rows as we encounter them.
//
// Reference: at-spi2-core/atspi/atspi-types.h AtspiRole enum.
func atspiRoleToARIA(code uint32) string {
	switch code {
	case 7:
		return "application"
	case 9:
		return "article"
	case 12:
		return "button"
	case 25:
		return "combobox"
	case 44:
		return "dialog"
	case 45:
		return "document"
	case 50:
		return "form"
	case 54:
		return "heading"
	case 58:
		return "image"
	case 63:
		return "link"
	case 68:
		return "list"
	case 69:
		return "listitem"
	case 72:
		return "menu"
	case 73:
		return "menubar"
	case 74:
		return "menuitem"
	case 88:
		return "progressbar"
	case 90:
		return "radio"
	case 93:
		return "scrollbar"
	case 95:
		return "slider"
	case 109:
		return "table"
	case 110:
		return "cell"
	case 112:
		return "rowheader"
	case 113:
		return "columnheader"
	case 117:
		return "textbox"
	case 128:
		return "toolbar"
	case 130:
		return "tooltip"
	case 134:
		return "window"
	case 136:
		return "header"
	case 137:
		return "footer"
	case 138:
		return "paragraph"
	case 158:
		return "tab"
	case 159:
		return "tabpanel"
	}
	return fmt.Sprintf("role-%d", code)
}

// roleCodeFromName is the inverse of atspiRoleToARIA — useful for test
// fixtures and for constructing synthetic NodeProps in unit tests.
func roleCodeFromName(name string) uint32 {
	m := map[string]uint32{
		"application":  7,
		"article":      9,
		"button":       12,
		"combobox":     25,
		"dialog":       44,
		"document":     45,
		"form":         50,
		"heading":      54,
		"image":        58,
		"link":         63,
		"list":         68,
		"listitem":     69,
		"menu":         72,
		"menubar":      73,
		"menuitem":     74,
		"progressbar":  88,
		"radio":        90,
		"scrollbar":    93,
		"slider":       95,
		"table":        109,
		"cell":         110,
		"rowheader":    112,
		"columnheader": 113,
		"textbox":      117,
		"toolbar":      128,
		"tooltip":      130,
		"window":       134,
		"header":       136,
		"footer":       137,
		"paragraph":    138,
		"tab":          158,
		"tabpanel":     159,
	}
	if code, ok := m[strings.ToLower(name)]; ok {
		return code
	}
	return 0
}

// Compile-time guards.
var (
	_ Snapshotter = (*LinuxSnapshotter)(nil)
	_ LinuxBus    = (*linuxBusGodbus)(nil)
)
