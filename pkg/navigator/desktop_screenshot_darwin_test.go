//go:build darwin

// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/detector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// macOS half of the §11.4.81 desktop-screenshot split. This is an
// INTEGRATION test (CONST-050: real system, no mock) — it drives the real
// `screencapture` tool through the real CommandRunner and asserts the
// returned bytes are a genuine, non-trivial PNG (§11.4.81(B) positive
// captured per-OS evidence; §11.4.69 sink-side video_display proof).
func TestX11Executor_Screenshot_Darwin_RealCapture(t *testing.T) {
	if _, err := exec.LookPath("screencapture"); err != nil {
		t.Skip("SKIP-OK: screencapture not on PATH (§11.4.3 topology gate)")
	}

	runner := detector.NewExecRunner()
	executor := NewX11Executor("", runner)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, err := executor.Screenshot(ctx)
	// screencapture needs Screen-Recording permission; if denied it errors
	// or yields an empty file. Treat a clean environment gap as SKIP, not a
	// false FAIL (§11.4.3), but a present-and-working capture MUST validate.
	if err != nil {
		t.Skipf("SKIP-OK: screencapture unavailable/denied in this environment: %v", err)
	}

	require.NotEmpty(t, data, "real screencapture must return non-empty bytes")
	// PNG magic number: 89 50 4E 47 0D 0A 1A 0A.
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	require.True(t, len(data) >= len(pngMagic), "captured data too short to be a PNG")
	assert.Equal(t, pngMagic, data[:len(pngMagic)],
		"captured bytes must carry the PNG magic header (real on-screen capture)")
	assert.Greater(t, len(data), 1024,
		"a real desktop PNG should be substantially larger than 1 KiB")
	assert.True(t, bytes.HasSuffix(data, []byte{0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82}),
		"captured PNG must end with the IEND chunk (complete, non-truncated image)")
}
