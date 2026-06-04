//go:build linux

// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"fmt"

	"digital.vasic.helixqa/pkg/detector"
)

// captureDesktopScreenshot grabs the full desktop on Linux/X11 via
// ImageMagick's `import`, streaming PNG bytes on stdout. This is the Linux
// half of the §11.4.81 cross-platform split; the macOS counterpart lives in
// desktop_screenshot_darwin.go and the fallback in desktop_screenshot_other.go.
func captureDesktopScreenshot(
	ctx context.Context, runner detector.CommandRunner,
) ([]byte, error) {
	data, err := runner.Run(ctx, "import", "-window", "root", "png:-")
	if err != nil {
		return nil, fmt.Errorf("x11 screenshot: %w", err)
	}
	return data, nil
}
