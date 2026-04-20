// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"context"
	"fmt"

	"digital.vasic.helixqa/pkg/bridge/scrcpy"
)

// DirectServiceConfig is the one-liner operator-facing knob set that
// combines a scrcpy.ServerConfig with DirectSource tuning. Callers supply
// the ServerConfig (Runner, Launcher, Serial, JarLocalPath, ServerVersion)
// and the capture dimensions; the service starts the server, wires a
// DirectSource, and returns a ready-to-consume source whose Stop forwards
// to Server.Stop (closing all sockets + removing the ADB reverse forward).
type DirectServiceConfig struct {
	// ServerConfig is the full scrcpy.ServerConfig the service will hand
	// to scrcpy.StartServer. Required.
	Server scrcpy.ServerConfig

	// Width / Height are attached to every emitted frames.Frame. Required.
	Width  int
	Height int

	// ChannelBuffer sizes DirectSource's output channel (0 -> 64 default).
	ChannelBuffer int

	// IncludeConfig forwards SPS/PPS config packets as Frames (normally
	// skipped because they carry no visible frame).
	IncludeConfig bool
}

// NewDirectFromServerConfig is the production-grade entry point for
// Android scrcpy-direct capture. Steps:
//
//  1. scrcpy.StartServer(ctx, cfg.Server) — push JAR, adb reverse, launch
//     app_process, accept video+audio+control sockets.
//  2. NewDirectSource wraps the resulting Server in a DirectSource.
//  3. DirectSource.Start(ctx) kicks off the pump goroutine.
//
// On success returns the DirectSource already started — iterate
// src.Frames() for captured frames; call src.Stop() when done (which
// tears down the underlying Server).
//
// On any error, all partially-acquired resources are released.
func NewDirectFromServerConfig(ctx context.Context, cfg DirectServiceConfig) (*DirectSource, error) {
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("%w: width/height required (%dx%d)", ErrDirectConfig, cfg.Width, cfg.Height)
	}
	// Ensure control is enabled so HelixQA can send input events. Audio is
	// opt-in per ServerConfig.
	if !cfg.Server.EnableControl {
		cfg.Server.EnableControl = true
	}
	srv, err := scrcpy.StartServer(ctx, cfg.Server)
	if err != nil {
		return nil, fmt.Errorf("android/direct: StartServer: %w", err)
	}
	src, err := NewDirectSource(DirectConfig{
		Server:        srv,
		Width:         cfg.Width,
		Height:        cfg.Height,
		ChannelBuffer: cfg.ChannelBuffer,
		IncludeConfig: cfg.IncludeConfig,
	})
	if err != nil {
		_ = srv.Stop()
		return nil, fmt.Errorf("android/direct: NewDirectSource: %w", err)
	}
	if err := src.Start(ctx); err != nil {
		_ = srv.Stop()
		return nil, fmt.Errorf("android/direct: Start: %w", err)
	}
	return src, nil
}

// MustBeEnabled is a convenience assertion for callers that gate direct
// capture on HELIX_SCRCPY_DIRECT=1. Returns a clear error when the env var
// is missing so higher-level code surfaces a proper operator-action
// message rather than silently falling back.
func MustBeEnabled(lookup func(string) (string, bool)) error {
	if IsDirectEnabled(lookup) {
		return nil
	}
	return fmt.Errorf("android/direct: HELIX_SCRCPY_DIRECT=1 is required; current env does not match")
}
