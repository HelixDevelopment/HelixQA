// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	dbus "github.com/godbus/dbus/v5"
)

// Portal destination + object paths (xdg-desktop-portal ScreenCast spec).
const (
	portalDestination      = "org.freedesktop.portal.Desktop"
	portalObjectPath       = "/org/freedesktop/portal/desktop"
	portalScreenCastIface  = "org.freedesktop.portal.ScreenCast"
	portalRequestInterface = "org.freedesktop.portal.Request"
)

// StreamSourceType is the `types` bitmask accepted by SelectSources (spec §1).
type StreamSourceType uint32

const (
	// StreamSourceMonitor selects physical outputs / monitors.
	StreamSourceMonitor StreamSourceType = 1
	// StreamSourceWindow selects application windows (where supported).
	StreamSourceWindow StreamSourceType = 2
	// StreamSourceVirtual requests a virtual source (KDE/wlroots only).
	StreamSourceVirtual StreamSourceType = 4
)

// CursorMode is the `cursor_mode` enum accepted by SelectSources (spec §1).
type CursorMode uint32

const (
	// CursorHidden hides the cursor in the stream.
	CursorHidden CursorMode = 1
	// CursorEmbedded draws the cursor into the stream's pixel data.
	CursorEmbedded CursorMode = 2
	// CursorMetadata delivers cursor position via PipeWire metadata.
	CursorMetadata CursorMode = 4
)

// PersistMode selects whether the portal remembers the selection across calls.
type PersistMode uint32

const (
	// PersistNever prompts the user on every Start.
	PersistNever PersistMode = 0
	// PersistAppWindow remembers for the lifetime of the calling app.
	PersistAppWindow PersistMode = 1
	// PersistForever remembers permanently until the user revokes consent.
	PersistForever PersistMode = 2
)

// Stream is one entry from ScreenCast Start's "streams" response.
type Stream struct {
	// NodeID is the PipeWire node the capture sidecar will attach to.
	NodeID uint32
	// Metadata holds any portal-provided properties (position, size, source_type,
	// mapping_id). Keys are portal-defined; consumers typically only need
	// "size" (ax) and "source_type" (u).
	Metadata map[string]any
}

// StartResult is the decoded return from ScreenCast.Start.
type StartResult struct {
	Streams []Stream
	// RestoreToken, if non-empty, can be reused on a subsequent CreateSession
	// to skip the user-consent dialog (requires portal support for "persist").
	RestoreToken string
}

// Caller abstracts the godbus Request/Response handshake the portal uses.
// Production code uses dbusCaller (in portal_dbus.go) which holds a live
// *dbus.Conn; tests inject a fake that records invocations and returns
// scripted responses.
//
// CallPortal performs a portal method call that returns a Request object
// (CreateSession, SelectSources, Start): the implementation MUST add a
// Response signal match for the request path, make the call, wait for the
// signal, and return (status, results). status==0 means success per
// org.freedesktop.portal.Request.
//
// CallImmediate performs a method call whose return body is the result
// (OpenPipeWireRemote): no Response signal waits. The returned []any is the
// raw godbus Body slice so callers can extract UnixFDs without rewrapping.
type Caller interface {
	CallPortal(
		ctx context.Context,
		dest, path, iface, method string,
		args ...any,
	) (status uint32, results map[string]any, err error)

	CallImmediate(
		ctx context.Context,
		dest, path, iface, method string,
		args ...any,
	) (raw []any, err error)

	Close() error
}

// Portal wraps a Caller and exposes the four ScreenCast operations HelixQA
// needs: CreateSession, SelectSources, Start, OpenPipeWireRemote.
type Portal struct {
	caller Caller
	// handleTokenCounter serialises unique "handle_token" values per Portal
	// instance — portal spec requires every call to include a fresh token.
	handleTokenCounter atomic.Uint64
	// sessionTokenCounter does the same for "session_handle_token".
	sessionTokenCounter atomic.Uint64
}

// NewPortal returns a Portal driven by caller.
func NewPortal(caller Caller) *Portal { return &Portal{caller: caller} }

// ErrPortalStatus is returned when the portal emits a non-zero Response
// status (1 == user cancelled, 2 == other failure).
type ErrPortalStatus struct {
	Method string
	Status uint32
	Result map[string]any
}

func (e *ErrPortalStatus) Error() string {
	return fmt.Sprintf("linux/capture: portal %s returned status=%d results=%v", e.Method, e.Status, e.Result)
}

// IsUserCancelled reports whether err is a portal status=1 (user dismissed
// the consent dialog). Useful for distinguishing "operator said no" from
// technical failures.
func IsUserCancelled(err error) bool {
	var s *ErrPortalStatus
	if errors.As(err, &s) {
		return s.Status == 1
	}
	return false
}

// CreateSession opens a portal ScreenCast session. Returns the session object
// path that SelectSources / Start / OpenPipeWireRemote need.
func (p *Portal) CreateSession(ctx context.Context) (string, error) {
	options := map[string]dbus.Variant{
		"handle_token":         dbus.MakeVariant(p.nextHandleToken("helixqa")),
		"session_handle_token": dbus.MakeVariant(p.nextSessionToken("helixqa")),
	}
	status, results, err := p.caller.CallPortal(
		ctx, portalDestination, portalObjectPath, portalScreenCastIface,
		"CreateSession", options,
	)
	if err != nil {
		return "", fmt.Errorf("linux/capture: CreateSession: %w", err)
	}
	if status != 0 {
		return "", &ErrPortalStatus{Method: "CreateSession", Status: status, Result: results}
	}
	sessPath, _ := results["session_handle"].(string)
	if sessPath == "" {
		return "", errors.New("linux/capture: CreateSession response missing session_handle")
	}
	return sessPath, nil
}

// SelectSourcesOptions carries the arguments ScreenCast.SelectSources accepts
// beyond the mandatory `handle_token`. Callers use zero values for defaults.
type SelectSourcesOptions struct {
	// Types selects which source kinds to offer (bitmask of StreamSourceMonitor,
	// Window, Virtual). Zero means "monitor" only.
	Types StreamSourceType
	// Multiple=true allows the user to pick more than one source in the dialog.
	Multiple bool
	// CursorMode selects whether / how the cursor is rendered into the stream.
	// Zero falls back to CursorHidden.
	CursorMode CursorMode
	// Persist requests that the portal remember this selection.
	Persist PersistMode
	// RestoreToken restores a prior selection when Persist > 0 previously
	// yielded a token; empty means "no prior session to restore".
	RestoreToken string
}

// SelectSources registers the desired capture surface(s) on the session.
func (p *Portal) SelectSources(ctx context.Context, sessionPath string, opts SelectSourcesOptions) error {
	if sessionPath == "" {
		return errors.New("linux/capture: SelectSources: empty sessionPath")
	}
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(p.nextHandleToken("helixqa")),
	}
	types := opts.Types
	if types == 0 {
		types = StreamSourceMonitor
	}
	options["types"] = dbus.MakeVariant(uint32(types))
	options["multiple"] = dbus.MakeVariant(opts.Multiple)
	mode := opts.CursorMode
	if mode == 0 {
		mode = CursorHidden
	}
	options["cursor_mode"] = dbus.MakeVariant(uint32(mode))
	if opts.Persist != PersistNever {
		options["persist_mode"] = dbus.MakeVariant(uint32(opts.Persist))
	}
	if opts.RestoreToken != "" {
		options["restore_token"] = dbus.MakeVariant(opts.RestoreToken)
	}
	status, results, err := p.caller.CallPortal(
		ctx, portalDestination, sessionPath, portalScreenCastIface,
		"SelectSources", options,
	)
	if err != nil {
		return fmt.Errorf("linux/capture: SelectSources: %w", err)
	}
	if status != 0 {
		return &ErrPortalStatus{Method: "SelectSources", Status: status, Result: results}
	}
	return nil
}

// Start presents the consent dialog (if needed) and commits the session.
// parentWindow identifies the X11 / Wayland parent for dialog placement;
// empty string is fine for headless QA runs where no parent UI exists.
func (p *Portal) Start(ctx context.Context, sessionPath, parentWindow string) (StartResult, error) {
	if sessionPath == "" {
		return StartResult{}, errors.New("linux/capture: Start: empty sessionPath")
	}
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(p.nextHandleToken("helixqa")),
	}
	status, results, err := p.caller.CallPortal(
		ctx, portalDestination, sessionPath, portalScreenCastIface,
		"Start", parentWindow, options,
	)
	if err != nil {
		return StartResult{}, fmt.Errorf("linux/capture: Start: %w", err)
	}
	if status != 0 {
		return StartResult{}, &ErrPortalStatus{Method: "Start", Status: status, Result: results}
	}
	out := StartResult{}
	if tok, ok := results["restore_token"].(string); ok {
		out.RestoreToken = tok
	}
	out.Streams = parseStreams(results["streams"])
	if len(out.Streams) == 0 {
		return StartResult{}, errors.New("linux/capture: Start response had no streams")
	}
	return out, nil
}

// parseStreams decodes the "streams" field from ScreenCast Start. The spec
// shape is `a(ua{sv})` — array of (uint32 node_id, vardict metadata).
// godbus may hand the value as either an already-typed Go slice or a raw
// []interface{}; we handle both.
func parseStreams(v any) []Stream {
	switch raw := v.(type) {
	case []Stream:
		return raw
	case []any:
		return parseStreamsAny(raw)
	}
	return nil
}

func parseStreamsAny(raw []any) []Stream {
	out := make([]Stream, 0, len(raw))
	for _, entry := range raw {
		s, ok := parseStreamEntry(entry)
		if !ok {
			continue
		}
		out = append(out, s)
	}
	return out
}

func parseStreamEntry(entry any) (Stream, bool) {
	// Expect a tuple: (u, a{sv}). godbus surfaces this as []any{uint32, map[string]dbus.Variant}.
	tuple, ok := entry.([]any)
	if !ok {
		return Stream{}, false
	}
	if len(tuple) < 2 {
		return Stream{}, false
	}
	nodeID, ok := tuple[0].(uint32)
	if !ok {
		return Stream{}, false
	}
	meta := decodeVariantMap(tuple[1])
	return Stream{NodeID: nodeID, Metadata: meta}, true
}

// decodeVariantMap converts a map[string]dbus.Variant (or equivalent raw form)
// into a plain map[string]any so downstream code doesn't import godbus.
func decodeVariantMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]dbus.Variant:
		out := make(map[string]any, len(m))
		for k, vv := range m {
			out[k] = vv.Value()
		}
		return out
	case map[string]any:
		return m
	}
	return map[string]any{}
}

// OpenPipeWireRemote returns an *os.File holding the PipeWire socket the
// sidecar should use for the PipeWire remote. The file owns the underlying
// FD — callers pass it to exec.Cmd.ExtraFiles and MUST NOT close it first.
func (p *Portal) OpenPipeWireRemote(ctx context.Context, sessionPath string) (*os.File, error) {
	if sessionPath == "" {
		return nil, errors.New("linux/capture: OpenPipeWireRemote: empty sessionPath")
	}
	options := map[string]dbus.Variant{}
	raw, err := p.caller.CallImmediate(
		ctx, portalDestination, sessionPath, portalScreenCastIface,
		"OpenPipeWireRemote", options,
	)
	if err != nil {
		return nil, fmt.Errorf("linux/capture: OpenPipeWireRemote: %w", err)
	}
	if len(raw) == 0 {
		return nil, errors.New("linux/capture: OpenPipeWireRemote returned no body")
	}
	fd, ok := raw[0].(dbus.UnixFD)
	if !ok {
		return nil, fmt.Errorf("linux/capture: OpenPipeWireRemote body[0] = %T, want dbus.UnixFD", raw[0])
	}
	// dbus.UnixFD is already an int32-compatible handle owned by this process.
	return os.NewFile(uintptr(fd), "helixqa-pipewire-remote"), nil
}

// Close releases the underlying Caller (closes the D-Bus connection in the
// production path). Safe to call multiple times.
func (p *Portal) Close() error {
	if p == nil || p.caller == nil {
		return nil
	}
	return p.caller.Close()
}

// --- token generation helpers ---

func (p *Portal) nextHandleToken(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, p.handleTokenCounter.Add(1))
}

func (p *Portal) nextSessionToken(prefix string) string {
	return fmt.Sprintf("%s_session_%d", prefix, p.sessionTokenCounter.Add(1))
}
