// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package linux

import (
	"fmt"

	"digital.vasic.helixqa/pkg/capture/frames"
)

// DefaultKMSGrabSidecarBinary is the operator-installed helper that requires
// `setcap cap_sys_admin+ep` (granted by the operator at install time — NO
// runtime privilege escalation). See OpenClawing4.md §5.1.1.
const DefaultKMSGrabSidecarBinary = "helixqa-kmsgrab"

// KMSGrabConfig drives NewKMSGrabFactory.
type KMSGrabConfig struct {
	// SidecarBinary defaults to DefaultKMSGrabSidecarBinary when empty.
	SidecarBinary string

	// ExtraArgs are passed to the sidecar (e.g. --connector HDMI-A-1).
	ExtraArgs []string

	// Runner is the sidecar process spawner. Nil defaults to ExecRunner.
	Runner Runner
}

// NewKMSGrabFactory returns a BackendFactory that spawns the kmsgrab sidecar
// directly — no D-Bus / portal handshake — and exposes its envelope stream
// as a Source. The sidecar is expected to own its own DRM capability grant
// (setcap); HelixQA never elevates at runtime.
func NewKMSGrabFactory(kc KMSGrabConfig) BackendFactory {
	return func(cfg Config) (Source, error) {
		if cfg.Width <= 0 || cfg.Height <= 0 {
			return nil, fmt.Errorf("%w: bad dimensions (%dx%d)", ErrInvalidConfig, cfg.Width, cfg.Height)
		}
		bin := kc.SidecarBinary
		if bin == "" {
			bin = DefaultKMSGrabSidecarBinary
		}
		runnerCfg := SidecarConfig{
			Binary:        bin,
			Args:          append([]string(nil), kc.ExtraArgs...),
			Source:        "kmsgrab",
			Width:         cfg.Width,
			Height:        cfg.Height,
			Format:        frames.FormatH264AnnexB,
			ChannelBuffer: cfg.ChannelBuffer,
			Runner:        kc.Runner,
		}
		runner, err := NewSidecarRunner(runnerCfg)
		if err != nil {
			return nil, err
		}
		return WrapSidecarAsSource(runner, BackendKMSGrab), nil
	}
}
