// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"digital.vasic.helixqa/pkg/capture/frames"
)

// Backend identifies a concrete Linux capture source.
type Backend int

const (
	// BackendAuto means: probe HELIX_LINUX_CAPTURE first, then XDG_SESSION_TYPE.
	BackendAuto Backend = iota
	// BackendPortal uses xdg-desktop-portal ScreenCast + helixqa-capture-linux.
	BackendPortal
	// BackendKMSGrab uses the capability-granted helixqa-kmsgrab sidecar.
	BackendKMSGrab
	// BackendX11Grab is the legacy ffmpeg x11grab fallback. Only valid when
	// XDG_SESSION_TYPE=x11 (or on XWayland).
	BackendX11Grab
)

// String renders Backend as a stable, lowercase token (for logs + env values).
func (b Backend) String() string {
	switch b {
	case BackendPortal:
		return "portal"
	case BackendKMSGrab:
		return "kmsgrab"
	case BackendX11Grab:
		return "x11grab"
	default:
		return "auto"
	}
}

// ParseBackend turns an env-var string into a Backend. Empty or unknown values
// return BackendAuto so the router probes the environment.
func ParseBackend(s string) Backend {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "portal", "pipewire":
		return BackendPortal
	case "kmsgrab", "kms":
		return BackendKMSGrab
	case "x11grab", "x11":
		return BackendX11Grab
	default:
		return BackendAuto
	}
}

// ErrUnsupportedBackend is returned when the selected Backend cannot be
// constructed on the current host (e.g. kmsgrab without the setcap'd sidecar,
// or x11grab on a pure Wayland session).
var ErrUnsupportedBackend = errors.New("linux/capture: backend unsupported on this host")

// Source represents one running Linux capture source. Implementations:
// portalSource (Phase 1 M9), kmsgrabSource (future), x11grabSource (future),
// and sidecarSource (a thin alias over SidecarRunner for tests + generic use).
type Source interface {
	// Start launches the underlying source. After Start returns nil, Frames
	// is live. Calling Start twice returns an error.
	Start(ctx context.Context) error
	// Frames returns the read-only channel of captured frames.
	Frames() <-chan frames.Frame
	// Stop terminates the source. Idempotent; safe from any goroutine.
	Stop() error
	// Backend reports which backend produced this Source.
	Backend() Backend
}

// Config drives NewSource. Only the Backend-specific fields for the chosen
// Backend are consulted; the rest are ignored.
type Config struct {
	// BackendOverride takes precedence over environment variables. Leave
	// BackendAuto to let NewSource probe the environment.
	BackendOverride Backend

	// Width / Height / ChannelBuffer are passed through to the underlying
	// SidecarRunner. Width+Height are REQUIRED (no sane default for the
	// encoded resolution).
	Width         int
	Height        int
	ChannelBuffer int

	// Environment lookups. nil defaults to os.LookupEnv so production code
	// just reads the real environment; tests inject a map-backed lookup.
	LookupEnv func(string) (string, bool)

	// Factories give the router hooks for backend construction. Leaving a
	// factory nil is valid — the router returns ErrUnsupportedBackend for
	// that backend. Production assembly wires the real factories; tests
	// inject stubs that return a pre-constructed fake Source.
	PortalFactory   BackendFactory
	KMSGrabFactory  BackendFactory
	X11GrabFactory  BackendFactory
}

// BackendFactory constructs one Source. Called at most once per NewSource.
type BackendFactory func(cfg Config) (Source, error)

// ResolveBackend returns the effective Backend to use given cfg and the
// current environment. Uses the documented precedence:
//
//  1. cfg.BackendOverride, if not BackendAuto
//  2. HELIX_LINUX_CAPTURE environment variable (parsed via ParseBackend)
//  3. XDG_SESSION_TYPE: "wayland" → BackendPortal, "x11" → BackendX11Grab
//  4. BackendPortal (safest default)
func ResolveBackend(cfg Config) Backend {
	if cfg.BackendOverride != BackendAuto {
		return cfg.BackendOverride
	}
	lookup := cfg.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	if v, ok := lookup("HELIX_LINUX_CAPTURE"); ok {
		if b := ParseBackend(v); b != BackendAuto {
			return b
		}
	}
	if v, ok := lookup("XDG_SESSION_TYPE"); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "wayland":
			return BackendPortal
		case "x11", "tty":
			return BackendX11Grab
		}
	}
	return BackendPortal
}

// NewSource resolves the backend from cfg and returns a constructed Source.
// Returns ErrUnsupportedBackend when the chosen backend has no factory wired.
func NewSource(cfg Config) (Source, error) {
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("%w: width/height required (%dx%d)", ErrInvalidConfig, cfg.Width, cfg.Height)
	}
	backend := ResolveBackend(cfg)
	var factory BackendFactory
	switch backend {
	case BackendPortal:
		factory = cfg.PortalFactory
	case BackendKMSGrab:
		factory = cfg.KMSGrabFactory
	case BackendX11Grab:
		factory = cfg.X11GrabFactory
	default:
		return nil, fmt.Errorf("linux/capture: unknown backend %d", backend)
	}
	if factory == nil {
		return nil, fmt.Errorf("%w: backend=%s (no factory wired)", ErrUnsupportedBackend, backend)
	}
	src, err := factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("linux/capture: construct %s: %w", backend, err)
	}
	return src, nil
}

// WrapSidecarAsSource adapts a SidecarRunner into a Source — used by
// portalSource / kmsgrabSource / tests to avoid reimplementing the
// Start/Frames/Stop wiring four times.
//
// The Source.Backend() value tags which wrapper produced the Source.
func WrapSidecarAsSource(runner *SidecarRunner, backend Backend) Source {
	return &sidecarSource{runner: runner, backend: backend}
}

type sidecarSource struct {
	runner  *SidecarRunner
	backend Backend
}

func (s *sidecarSource) Start(ctx context.Context) error { return s.runner.Start(ctx) }
func (s *sidecarSource) Frames() <-chan frames.Frame     { return s.runner.Frames() }
func (s *sidecarSource) Stop() error                     { return s.runner.Stop() }
func (s *sidecarSource) Backend() Backend                { return s.backend }
