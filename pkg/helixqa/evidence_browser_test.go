// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Round-61 §11.4 anti-bluff tests: browser-screenshot capture path.

package helixqa

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hasChromeBinary reports whether a Chromium/Chrome binary is on PATH.
// chromedp requires a real browser binary; in test hosts without one
// the screenshot tests are SKIP-OK rather than FAIL — the absence is
// loud (skip with marker), not a silent PASS. Mirrors round-43 GPU
// integration SKIP pattern.
func hasChromeBinary() bool {
	for _, bin := range []string{"chromium", "chromium-browser", "google-chrome", "chrome", "google-chrome-stable"} {
		if _, err := exec.LookPath(bin); err == nil {
			return true
		}
	}
	return false
}

// TestCaptureFailureEvidence_WithBrowserURL_CapturesScreenshot uses
// httptest.NewServer with a deterministic HTML page and asserts the
// orchestrator writes a non-empty screenshot.png to disk.
//
// SKIP-OK: #HELIXQA-EVIDENCE-CHROMEDP-NO-CHROME — test hosts without
// Chromium/Chrome cannot exercise chromedp lifecycle; the screenshot
// integration test SKIPs rather than FAILs in that environment.
func TestCaptureFailureEvidence_WithBrowserURL_CapturesScreenshot(t *testing.T) {
	if !hasChromeBinary() {
		t.Skip("SKIP-OK: #HELIXQA-EVIDENCE-CHROMEDP-NO-CHROME — no Chromium/Chrome binary on PATH; chromedp screenshot integration cannot run")
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body><h1>round-61 capture target</h1><p>evidence ok</p></body></html>"))
	}))
	defer ts.Close()

	t.Setenv(EnvBrowserURL, ts.URL)

	o := newOrchestratorWithTempEvidenceDir(t)
	paths, capErr := o.captureFailureEvidence(E2E, errors.New("e2e probe failed"), []byte("FAIL: probe\n"))
	require.NoError(t, capErr, "screenshot capture against live httptest server should succeed; got %v", capErr)
	require.NotEmpty(t, paths)

	var screenshotPath string
	for _, p := range paths {
		if filepath.Base(p) == "screenshot.png" {
			screenshotPath = p
			break
		}
	}
	require.NotEmpty(t, screenshotPath, "captureFailureEvidence MUST include screenshot.png in returned paths (got %v)", paths)

	info, err := os.Stat(screenshotPath)
	require.NoError(t, err, "round-61 anti-bluff backstop: returned screenshot.png MUST exist on disk")
	assert.False(t, info.IsDir(), "screenshot.png MUST be a regular file")
	assert.Greater(t, info.Size(), int64(100), "screenshot.png MUST be a non-trivial PNG, not a zero-byte bluff (got %d bytes)", info.Size())

	// Sanity: first 8 bytes are the PNG magic.
	raw, err := os.ReadFile(screenshotPath)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(raw), 8)
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	assert.Equal(t, pngMagic, raw[:8], "screenshot.png MUST start with the PNG magic bytes")
}

// TestCaptureFailureEvidence_BrowserUnreachable_ReturnsSentinel asserts
// that when HELIX_QA_BROWSER_URL points at an unreachable port, the
// orchestrator surfaces ErrEvidenceCaptureScreenshotFailed via errors.Is
// AND does NOT fabricate a screenshot path.
func TestCaptureFailureEvidence_BrowserUnreachable_ReturnsSentinel(t *testing.T) {
	if !hasChromeBinary() {
		t.Skip("SKIP-OK: #HELIXQA-EVIDENCE-CHROMEDP-NO-CHROME — no Chromium/Chrome binary on PATH; chromedp lifecycle cannot fire to produce the expected sentinel")
	}

	// Port 9 is the discard port — guaranteed-unreachable for HTTP.
	t.Setenv(EnvBrowserURL, "http://127.0.0.1:9/round-61-unreachable")

	o := newOrchestratorWithTempEvidenceDir(t)
	paths, capErr := o.captureFailureEvidence(E2E, errors.New("e2e probe failed"), []byte("FAIL: probe\n"))

	require.Error(t, capErr, "unreachable browser MUST surface an error from captureFailureEvidence")
	assert.True(t, errors.Is(capErr, ErrEvidenceCaptureScreenshotFailed), "unreachable browser error MUST wrap ErrEvidenceCaptureScreenshotFailed (got %v)", capErr)

	// stdout.log + error.log + env.json + manifest.json should still be
	// captured even though screenshot failed (partial-success path).
	for _, p := range paths {
		assert.NotEqual(t, "screenshot.png", filepath.Base(p), "round-29/58 anti-bluff backstop: screenshot.png MUST NOT appear in returned paths when capture failed (got %v)", paths)
	}
}

// TestCaptureBrowserScreenshot_NoBrowserURL_ReturnsSentinel asserts the
// no-context contract: when HELIX_QA_BROWSER_URL is unset, the capturer
// returns ErrEvidenceCaptureNoBrowserContext and an empty path string.
// This is the round-58 honesty contract surfaced at the round-61 layer.
func TestCaptureBrowserScreenshot_NoBrowserURL_ReturnsSentinel(t *testing.T) {
	// Explicitly unset HELIX_QA_BROWSER_URL for this test.
	t.Setenv(EnvBrowserURL, "")

	tmp := t.TempDir()
	path, err := captureBrowserScreenshot(t.Context(), tmp)

	assert.Empty(t, path, "no-context capture MUST NOT fabricate a path")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEvidenceCaptureNoBrowserContext), "no-context capture MUST return ErrEvidenceCaptureNoBrowserContext (got %v)", err)
}

// TestCaptureBrowserScreenshot_OperatorSkipNoChrome_ReturnsSentinel
// asserts the operator escape hatch: when HELIX_QA_BROWSER_SKIP_NO_CHROME
// is set, the capturer SKIPs with the no-context sentinel regardless of
// HELIX_QA_BROWSER_URL — letting the rest of the evidence pipeline run.
func TestCaptureBrowserScreenshot_OperatorSkipNoChrome_ReturnsSentinel(t *testing.T) {
	t.Setenv(EnvBrowserURL, "http://example.invalid/")
	t.Setenv(EnvBrowserSkipNoChrome, "1")

	tmp := t.TempDir()
	path, err := captureBrowserScreenshot(t.Context(), tmp)

	assert.Empty(t, path, "skip-no-chrome MUST NOT fabricate a path")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEvidenceCaptureNoBrowserContext), "operator skip MUST surface ErrEvidenceCaptureNoBrowserContext (got %v)", err)
}

// TestIsBrowserCaptureSentinel asserts the predicate matches every
// round-61 browser-capture sentinel and rejects unrelated errors.
func TestIsBrowserCaptureSentinel(t *testing.T) {
	for _, s := range browserCaptureSentinels {
		assert.True(t, isBrowserCaptureSentinel(s), "isBrowserCaptureSentinel MUST recognise %v", s)
	}
	assert.False(t, isBrowserCaptureSentinel(nil), "nil MUST NOT be classified as a sentinel")
	assert.False(t, isBrowserCaptureSentinel(errors.New("unrelated")), "unrelated error MUST NOT be classified as a sentinel")
}
