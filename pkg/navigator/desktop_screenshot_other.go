//go:build !linux && !darwin

// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"fmt"
	"runtime"

	"digital.vasic.helixqa/pkg/detector"
)

// captureDesktopScreenshot is the fallback for platforms without an
// implemented native desktop-capture path (e.g. Windows, the BSDs).
//
// §11.4.81(C) honest kernel/platform gap: rather than silently returning no
// frames, it reports a clear error naming the current OS so the caller can
// degrade or SKIP-with-reason instead of treating an empty capture as success
// (anti-bluff §11.4.69 — no fail-open). Adding a native path here (e.g. a
// Windows GDI/DXGI grab) lifts the gap for that platform.
func captureDesktopScreenshot(
	_ context.Context, _ detector.CommandRunner,
) ([]byte, error) {
	return nil, fmt.Errorf(
		"desktop screenshot not implemented on %s (§11.4.81 gap: no native capture path)",
		runtime.GOOS,
	)
}
