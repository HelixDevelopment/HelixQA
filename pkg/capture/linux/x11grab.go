// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"fmt"

	"digital.vasic.helixqa/pkg/capture/frames"
)

// DefaultX11GrabSidecarBinary is the operator-installed Go sidecar that runs
// `ffmpeg -f x11grab -i $DISPLAY -f h264 …` and wraps ffmpeg's raw H.264
// Annex-B output in the envelope format documented in doc.go.
//
// The sidecar is *trivially small* (it is essentially a Go NAL-unit splitter
// plus the envelope framer around ffmpeg's stdout) but it must exist on the
// host — the Go process never shells out to ffmpeg directly, so the envelope
// contract stays intact for every Linux backend.
//
// If the binary is not present on PATH, Runner.Start returns an "exec" error
// that surfaces clearly in Source.Start's chain — no silent misbehaviour.
const DefaultX11GrabSidecarBinary = "helixqa-x11grab"

// DefaultX11GrabDisplay is the fallback X11 display when Config does not
// supply one. Most desktop sessions are ":0"; headless operators set the
// DISPLAY environment variable or pass it via ExtraArgs.
const DefaultX11GrabDisplay = ":0"

// X11GrabConfig drives NewX11GrabFactory.
type X11GrabConfig struct {
	// SidecarBinary defaults to DefaultX11GrabSidecarBinary when empty.
	SidecarBinary string

	// Display is forwarded to the sidecar as "--display <value>" when
	// non-empty. Empty falls back to DefaultX11GrabDisplay at runtime.
	Display string

	// FPS, when non-zero, is forwarded as "--fps <n>"; zero lets the
	// sidecar choose (typically 30).
	FPS int

	// ExtraArgs are passed after the HelixQA-constructed args (display, fps).
	// Useful for per-deployment flags like "--region 0,0,1920,1080".
	ExtraArgs []string

	// Runner is the sidecar process spawner. Nil defaults to ExecRunner.
	Runner Runner
}

// NewX11GrabFactory returns a BackendFactory that spawns helixqa-x11grab
// directly — no D-Bus / portal handshake. The sidecar owns its own ffmpeg
// invocation and the NAL-unit framing; the Go host only speaks the envelope
// wire format.
//
// This is the legacy fallback path for X11 sessions where Wayland / portal
// is not available. When libei + portal work everywhere we intend to deploy,
// this factory becomes a no-op — but it remains for compatibility.
func NewX11GrabFactory(xc X11GrabConfig) BackendFactory {
	return func(cfg Config) (Source, error) {
		if cfg.Width <= 0 || cfg.Height <= 0 {
			return nil, fmt.Errorf("%w: bad dimensions (%dx%d)", ErrInvalidConfig, cfg.Width, cfg.Height)
		}
		bin := xc.SidecarBinary
		if bin == "" {
			bin = DefaultX11GrabSidecarBinary
		}
		args := make([]string, 0, 4+len(xc.ExtraArgs))
		display := xc.Display
		if display == "" {
			display = DefaultX11GrabDisplay
		}
		args = append(args, "--display", display)
		if xc.FPS > 0 {
			args = append(args, "--fps", fmt.Sprintf("%d", xc.FPS))
		}
		args = append(args, xc.ExtraArgs...)
		runnerCfg := SidecarConfig{
			Binary:        bin,
			Args:          args,
			Source:        "x11grab",
			Width:         cfg.Width,
			Height:        cfg.Height,
			Format:        frames.FormatH264AnnexB,
			ChannelBuffer: cfg.ChannelBuffer,
			Runner:        xc.Runner,
		}
		runner, err := NewSidecarRunner(runnerCfg)
		if err != nil {
			return nil, err
		}
		return WrapSidecarAsSource(runner, BackendX11Grab), nil
	}
}
