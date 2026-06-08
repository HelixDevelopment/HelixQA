//go:build darwin

// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"fmt"
	"os"

	"digital.vasic.helixqa/pkg/detector"
)

// captureDesktopScreenshot grabs the full desktop on macOS via the native
// `screencapture` tool (there is no X11 on darwin, so ImageMagick `import`
// fails — see §11.4.81). `screencapture` has no stdout mode, so it writes a
// temporary PNG which we read back and remove. The returned bytes are the
// real on-screen capture (positive sink-side evidence per §11.4.69).
//
// This is the macOS half of the §11.4.81 cross-platform split.
func captureDesktopScreenshot(
	ctx context.Context, runner detector.CommandRunner,
) ([]byte, error) {
	tmp, err := os.CreateTemp("", "helixqa-screencap-*.png")
	if err != nil {
		return nil, fmt.Errorf("darwin screenshot tempfile: %w", err)
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(path) }()

	// -x: silent (no capture sound). -t png: PNG format. Capture all displays
	// to the file. screencapture writes nothing to stdout.
	if _, err := runner.Run(ctx, "screencapture", "-x", "-t", "png", path); err != nil {
		return nil, fmt.Errorf("darwin screencapture: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("darwin screenshot read: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("darwin screencapture produced an empty file")
	}
	return data, nil
}
