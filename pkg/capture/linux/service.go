// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"fmt"

	"digital.vasic.helixqa/pkg/bridge/dbusportal"
	"digital.vasic.helixqa/pkg/capture/frames"
)

// ServiceConfig is the operator-facing knob set for Linux capture. The zero
// value (other than Width/Height which are required) produces a sensible
// default stack: backend auto-detected from env, production DBusCaller
// through the shared session bus, HelixQA-native sidecars (helixqa-
// capture-linux, helixqa-kmsgrab, helixqa-x11grab) looked up on PATH.
type ServiceConfig struct {
	// Required.
	Width  int
	Height int

	// BackendOverride wins over HELIX_LINUX_CAPTURE and XDG_SESSION_TYPE.
	// BackendAuto (the zero value) lets the router probe.
	BackendOverride Backend

	// ChannelBuffer sizes the captured-frame channel; 0 falls back to 32.
	ChannelBuffer int

	// X11Grab-specific tuning.
	Display string // "" -> $DISPLAY -> ":0"
	FPS     int    // 0 -> 30

	// Portal-specific tuning.
	SelectSources SelectSourcesOptions
	ParentWindow  string

	// Sidecar binary overrides. Empty strings select the documented
	// defaults (DefaultPortalSidecarBinary / DefaultKMSGrabSidecarBinary /
	// DefaultX11GrabSidecarBinary).
	PortalSidecarBinary  string
	KMSGrabSidecarBinary string
	X11GrabSidecarBinary string
}

// NewDefaultSource is the one-liner most callers want. It wires:
//
//   - production dbusportal.DBusCallerFactory (shared session bus)
//   - PortalFactory over ScreenCast (launches helixqa-capture-linux)
//   - KMSGrabFactory (launches helixqa-kmsgrab)
//   - X11GrabFactory (launches helixqa-x11grab)
//
// ...into a single Config and returns whatever Source the resolved Backend
// picks. Callers follow up with Source.Start(ctx) + range over
// Source.Frames(); Stop() on shutdown.
//
// No Go-side CGO is involved; failures that would usually surface as a nil
// factory (ErrUnsupportedBackend) are impossible here because every backend
// is wired.
func NewDefaultSource(cfg ServiceConfig) (Source, error) {
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("%w: NewDefaultSource requires Width/Height (%dx%d)", ErrInvalidConfig, cfg.Width, cfg.Height)
	}
	return NewSource(cfg.toConfig(dbusportal.DBusCallerFactory))
}

// NewDefaultSourceWithCallerFactory is the same as NewDefaultSource except
// callers can override the CallerFactory — useful for tests that want a
// fake Caller, or for integration environments that bring their own bus.
func NewDefaultSourceWithCallerFactory(cfg ServiceConfig, factory dbusportal.CallerFactory) (Source, error) {
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("%w: width/height required (%dx%d)", ErrInvalidConfig, cfg.Width, cfg.Height)
	}
	if factory == nil {
		return nil, fmt.Errorf("%w: NewDefaultSourceWithCallerFactory: nil factory", ErrInvalidConfig)
	}
	return NewSource(cfg.toConfig(factory))
}

// toConfig builds a fully-populated Config from the ServiceConfig. Separated
// out so tests can inspect the Config shape without needing to actually
// spawn a Source.
func (cfg ServiceConfig) toConfig(callerFactory dbusportal.CallerFactory) Config {
	return Config{
		BackendOverride: cfg.BackendOverride,
		Width:           cfg.Width,
		Height:          cfg.Height,
		ChannelBuffer:   cfg.ChannelBuffer,

		PortalFactory: NewPortalFactory(PortalConfig{
			CallerFactory: callerFactory,
			SelectSources: cfg.SelectSources,
			ParentWindow:  cfg.ParentWindow,
			SidecarBinary: cfg.PortalSidecarBinary,
		}),
		KMSGrabFactory: NewKMSGrabFactory(KMSGrabConfig{
			SidecarBinary: cfg.KMSGrabSidecarBinary,
		}),
		X11GrabFactory: NewX11GrabFactory(X11GrabConfig{
			SidecarBinary: cfg.X11GrabSidecarBinary,
			Display:       cfg.Display,
			FPS:           cfg.FPS,
		}),
	}
}

// CollectFrames is a small convenience that drains Source.Frames for at
// most max frames or until ctx fires / the channel closes. Useful in
// tests + demos; production pipelines typically consume Frames() directly.
func CollectFrames(ctx context.Context, src Source, max int) []frames.Frame {
	out := make([]frames.Frame, 0, max)
	ch := src.Frames()
	for len(out) < max {
		select {
		case <-ctx.Done():
			return out
		case f, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, f)
		}
	}
	return out
}
