// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package libei

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	dbus "github.com/godbus/dbus/v5"

	"digital.vasic.helixqa/pkg/bridge/dbusportal"
)

// portalRemoteDesktopIface is the xdg-desktop-portal RemoteDesktop interface
// name. The object path + destination are shared with other portal clients
// and live in dbusportal.PortalDestination / PortalObjectPath.
const portalRemoteDesktopIface = "org.freedesktop.portal.RemoteDesktop"

// DeviceType is the bitmask accepted by RemoteDesktop.SelectDevices.
type DeviceType uint32

const (
	// DeviceKeyboard emulates keyboard events (press / release / modifier masks).
	DeviceKeyboard DeviceType = 1
	// DevicePointer emulates relative pointer motion + buttons + scroll.
	DevicePointer DeviceType = 2
	// DeviceTouchscreen emulates absolute-positioned touch events (where supported).
	DeviceTouchscreen DeviceType = 4
)

// PersistMode mirrors the ScreenCast enum: whether the portal should
// remember a device selection across calls.
type PersistMode uint32

const (
	// PersistNever prompts the user every time.
	PersistNever PersistMode = 0
	// PersistAppWindow remembers for the lifetime of the calling app.
	PersistAppWindow PersistMode = 1
	// PersistForever remembers until the user revokes consent.
	PersistForever PersistMode = 2
)

// SelectDevicesOptions carries the arguments RemoteDesktop.SelectDevices
// accepts beyond the mandatory handle_token. Callers use zero values for
// defaults.
type SelectDevicesOptions struct {
	// Types is a bitmask of DeviceKeyboard / DevicePointer / DeviceTouchscreen.
	// Zero falls back to DeviceKeyboard | DevicePointer — the common QA case.
	Types DeviceType
	// Persist requests the portal remember this selection.
	Persist PersistMode
	// RestoreToken, if non-empty, tries to restore a prior selection without
	// re-prompting. Requires Persist > 0 on the session that yielded the token.
	RestoreToken string
}

// StartResult is the decoded return from RemoteDesktop.Start.
type StartResult struct {
	// ChosenDevices is what the portal confirmed the session may emulate.
	// Callers MUST respect this — writing events for devices the portal
	// did not grant is a protocol violation.
	ChosenDevices DeviceType
	// RestoreToken is non-empty only when Persist > 0 was requested and
	// the portal honoured it.
	RestoreToken string
}

// Portal wraps a dbusportal.Caller and drives the RemoteDesktop handshake.
// Construct via NewPortal; plug into higher-level input drivers that consume
// the FD returned by ConnectToEIS.
type Portal struct {
	caller              dbusportal.Caller
	handleTokenCounter  atomic.Uint64
	sessionTokenCounter atomic.Uint64
}

// NewPortal returns a Portal driven by caller.
func NewPortal(caller dbusportal.Caller) *Portal { return &Portal{caller: caller} }

// CreateSession opens a portal RemoteDesktop session. Returns the session
// object path that SelectDevices / Start / ConnectToEIS need.
func (p *Portal) CreateSession(ctx context.Context) (string, error) {
	options := map[string]dbus.Variant{
		"handle_token":         dbus.MakeVariant(p.nextHandleToken()),
		"session_handle_token": dbus.MakeVariant(p.nextSessionToken()),
	}
	status, results, err := p.caller.CallPortal(
		ctx, dbusportal.PortalDestination, dbusportal.PortalObjectPath, portalRemoteDesktopIface,
		"CreateSession", options,
	)
	if err != nil {
		return "", fmt.Errorf("libei: CreateSession: %w", err)
	}
	if status != 0 {
		return "", &dbusportal.ErrPortalStatus{Method: "CreateSession", Status: status, Result: results}
	}
	sessPath, _ := results["session_handle"].(string)
	if sessPath == "" {
		return "", errors.New("libei: CreateSession response missing session_handle")
	}
	return sessPath, nil
}

// SelectDevices registers the desired emulation surface (keyboard / pointer /
// touchscreen) on the session.
func (p *Portal) SelectDevices(ctx context.Context, sessionPath string, opts SelectDevicesOptions) error {
	if sessionPath == "" {
		return errors.New("libei: SelectDevices: empty sessionPath")
	}
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(p.nextHandleToken()),
	}
	types := opts.Types
	if types == 0 {
		types = DeviceKeyboard | DevicePointer
	}
	options["types"] = dbus.MakeVariant(uint32(types))
	if opts.Persist != PersistNever {
		options["persist_mode"] = dbus.MakeVariant(uint32(opts.Persist))
	}
	if opts.RestoreToken != "" {
		options["restore_token"] = dbus.MakeVariant(opts.RestoreToken)
	}
	status, results, err := p.caller.CallPortal(
		ctx, dbusportal.PortalDestination, sessionPath, portalRemoteDesktopIface,
		"SelectDevices", options,
	)
	if err != nil {
		return fmt.Errorf("libei: SelectDevices: %w", err)
	}
	if status != 0 {
		return &dbusportal.ErrPortalStatus{Method: "SelectDevices", Status: status, Result: results}
	}
	return nil
}

// Start presents the consent dialog (if needed) and commits the session.
// parentWindow identifies the X11 / Wayland parent for dialog placement;
// empty is fine for headless QA.
func (p *Portal) Start(ctx context.Context, sessionPath, parentWindow string) (StartResult, error) {
	if sessionPath == "" {
		return StartResult{}, errors.New("libei: Start: empty sessionPath")
	}
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(p.nextHandleToken()),
	}
	status, results, err := p.caller.CallPortal(
		ctx, dbusportal.PortalDestination, sessionPath, portalRemoteDesktopIface,
		"Start", parentWindow, options,
	)
	if err != nil {
		return StartResult{}, fmt.Errorf("libei: Start: %w", err)
	}
	if status != 0 {
		return StartResult{}, &dbusportal.ErrPortalStatus{Method: "Start", Status: status, Result: results}
	}
	out := StartResult{}
	if tok, ok := results["restore_token"].(string); ok {
		out.RestoreToken = tok
	}
	if dev, ok := results["devices"].(uint32); ok {
		out.ChosenDevices = DeviceType(dev)
	}
	// The portal MAY decline all devices (returning 0 bitmask) — surface
	// that as an error rather than silently continuing, since downstream
	// input drivers will write events and hit protocol violations.
	if out.ChosenDevices == 0 {
		return StartResult{}, errors.New("libei: Start response granted no devices")
	}
	return out, nil
}

// ConnectToEIS returns a *os.File wrapping the Unix socket the EI client
// must speak to emulate input. The portal spec names this method
// ConnectToEIS (ConnectTo-EmulatedInputServer).
//
// The returned file owns the FD — callers use it with net.FileConn or
// hand it to an EI client; it MUST be Closed by exactly one owner when
// the session ends.
func (p *Portal) ConnectToEIS(ctx context.Context, sessionPath string) (*os.File, error) {
	if sessionPath == "" {
		return nil, errors.New("libei: ConnectToEIS: empty sessionPath")
	}
	options := map[string]dbus.Variant{}
	raw, err := p.caller.CallImmediate(
		ctx, dbusportal.PortalDestination, sessionPath, portalRemoteDesktopIface,
		"ConnectToEIS", options,
	)
	if err != nil {
		return nil, fmt.Errorf("libei: ConnectToEIS: %w", err)
	}
	if len(raw) == 0 {
		return nil, errors.New("libei: ConnectToEIS returned no body")
	}
	fd, ok := raw[0].(dbus.UnixFD)
	if !ok {
		return nil, fmt.Errorf("libei: ConnectToEIS body[0] = %T, want dbus.UnixFD", raw[0])
	}
	return os.NewFile(uintptr(fd), "helixqa-libei-eis"), nil
}

// Close releases the underlying Caller. Safe to call more than once.
func (p *Portal) Close() error {
	if p == nil || p.caller == nil {
		return nil
	}
	return p.caller.Close()
}

// --- token generation ---

func (p *Portal) nextHandleToken() string {
	return fmt.Sprintf("helixqa_libei_%d", p.handleTokenCounter.Add(1))
}

func (p *Portal) nextSessionToken() string {
	return fmt.Sprintf("helixqa_libei_session_%d", p.sessionTokenCounter.Add(1))
}
