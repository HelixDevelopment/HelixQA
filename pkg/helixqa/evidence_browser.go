// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Round-61 §11.4 anti-bluff extension (2026-05-18): browser screenshot
// capturer for captureFailureEvidence. Closes the round-58 deferred item.
//
// Verbatim operator mandate (2026-05-19, preserved per CONST-049 §11.4.17):
//
//	"all existing tests and Challenges do work in anti-bluff manner - they
//	 MUST confirm that all tested codebase really works as expected! We had
//	 been in position that all tests do execute with success and all
//	 Challenges as well, but in reality the most of the features does not
//	 work and can't be used! This MUST NOT be the case and execution of
//	 tests and Challenges MUST guarantee the quality, the completition and
//	 full usability by end users of the product!"
//
// Constitutional anchors: CONST-035 (anti-bluff), CONST-042 (no secret
// leak — env whitelist preserved), CONST-050(A)+(B) (no fakes beyond unit
// tests, 100% test-type coverage), Article XI §11.9 (forensic anchor).

package helixqa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/chromedp"
)

// EnvBrowserURL is the environment variable consulted by the screenshot
// capturer to decide whether a live browser context is available. When
// empty the capturer SKIPs (returns ErrEvidenceCaptureNoBrowserContext)
// — round-58's honesty contract: no context, no fabrication.
const EnvBrowserURL = "HELIX_QA_BROWSER_URL"

// EnvBrowserSkipNoChrome lets the operator (or a SKIP-OK test) force the
// capturer to bypass the chromedp lifecycle even when HELIX_QA_BROWSER_URL
// is set — useful when the test host genuinely lacks any Chrome/Chromium
// binary and the operator wants the rest of the evidence pipeline to keep
// running without surfacing a screenshot sentinel.
const EnvBrowserSkipNoChrome = "HELIX_QA_BROWSER_SKIP_NO_CHROME"

// ErrEvidenceCaptureNoBrowserContext indicates the caller asked the
// orchestrator to capture a browser screenshot but no live browser
// context was available (HELIX_QA_BROWSER_URL unset AND no explicit
// BrowserContext supplied). Per round-58's honesty contract the
// orchestrator surfaces this gap rather than fabricating a screenshot
// path or silently returning no artefact when the caller expected one.
var ErrEvidenceCaptureNoBrowserContext = fmt.Errorf("helixqa orchestrator: no browser context for screenshot capture — HELIX_QA_BROWSER_URL is unset and no explicit BrowserContext was supplied; the orchestrator refuses to fabricate a screenshot path per round-29/58 anti-bluff backstop")

// ErrEvidenceCaptureScreenshotFailed indicates the chromedp lifecycle
// (allocator → context → navigate → full-screenshot) failed for the
// configured browser URL. Wraps the chromedp error so the caller can
// errors.Is the sentinel and inspect the underlying cause.
var ErrEvidenceCaptureScreenshotFailed = fmt.Errorf("helixqa orchestrator: chromedp screenshot capture failed — the browser context was attempted but did not produce on-disk screenshot bytes; round-29/58 anti-bluff backstop: no fake path returned")

// captureBrowserScreenshot navigates a headless Chromium instance to the
// URL given by HELIX_QA_BROWSER_URL and writes a full-page PNG screenshot
// to <perTypeDir>/screenshot.png. The caller passes the per-failure
// evidence directory already created by captureFailureEvidence.
//
// Return contract (preserves round-58 honesty):
//   - On success: returns the on-disk path (already verified via
//     pathExistsOnDisk by the caller's existing wrapper, but ALSO
//     verified here as a local backstop) and nil error.
//   - On no-context: returns "" and ErrEvidenceCaptureNoBrowserContext.
//   - On chromedp lifecycle failure: returns "" and an error wrapping
//     ErrEvidenceCaptureScreenshotFailed (errors.Is matches).
//
// The orchestrator NEVER returns a path it has not directly confirmed
// via os.Stat in the same call — same backstop as round-58's stdout.log
// path.
func captureBrowserScreenshot(ctx context.Context, perTypeDir string) (string, error) {
	url := os.Getenv(EnvBrowserURL)
	if url == "" {
		return "", ErrEvidenceCaptureNoBrowserContext
	}

	// Operator escape hatch: when the test host has no Chrome/Chromium
	// binary we let the operator skip the chromedp lifecycle entirely.
	// The caller treats this as "no screenshot captured, but no bluff
	// either" — the absence is honest because the environment knob is
	// loud, not silent. Pairs with the SKIP-OK marker on the matching
	// integration test.
	if os.Getenv(EnvBrowserSkipNoChrome) != "" {
		return "", fmt.Errorf("%w: HELIX_QA_BROWSER_SKIP_NO_CHROME set — operator-explicit skip", ErrEvidenceCaptureNoBrowserContext)
	}

	// Bound the entire chromedp dance to a reasonable ceiling so a
	// hung browser cannot stall the orchestrator's failure-evidence
	// pipeline. 15 s is generous for navigate + full-screenshot but
	// well below the orchestrator's per-test timeout.
	captureCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Match pkg/nexus/browser/chromedp_driver.go's hardened-flag set so
	// the orchestrator's screenshot runs in the same headless posture as
	// the production browser engine.
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Headless,
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(captureCtx, allocOpts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	// CaptureScreenshot produces PNG bytes (chromedp's FullScreenshot
	// returns JPEG when called with a quality argument — we want a
	// lossless artefact for forensic review, so CaptureScreenshot is
	// the right primitive here).
	var pngBuf []byte
	runErr := chromedp.Run(browserCtx,
		chromedp.Navigate(url),
		chromedp.CaptureScreenshot(&pngBuf),
	)
	if runErr != nil {
		return "", fmt.Errorf("%w: %v (url=%s)", ErrEvidenceCaptureScreenshotFailed, runErr, url)
	}
	if len(pngBuf) == 0 {
		// Defence in depth: chromedp returned nil but the buffer is
		// empty. Treat as a screenshot failure rather than writing a
		// zero-byte PNG (a §11.4 PASS-bluff in the round-29 sense).
		return "", fmt.Errorf("%w: chromedp returned no bytes for url=%s", ErrEvidenceCaptureScreenshotFailed, url)
	}

	screenshotPath := filepath.Join(perTypeDir, "screenshot.png")
	if err := os.WriteFile(screenshotPath, pngBuf, 0o644); err != nil {
		return "", fmt.Errorf("%w: write %s: %v", ErrEvidenceCaptureScreenshotFailed, screenshotPath, err)
	}
	if !pathExistsOnDisk(screenshotPath) {
		return "", fmt.Errorf("%w: post-write os.Stat verification failed for %s", ErrEvidenceCaptureStatVerificationFailed, screenshotPath)
	}
	return screenshotPath, nil
}

// browserCaptureSentinels enumerates the sentinels captureBrowserScreenshot
// can return, used by callers that want to errors.Is across the round-61
// surface. Kept as a package-level slice (not exported function) so the
// surface is easy to extend in round-62+.
var browserCaptureSentinels = []error{
	ErrEvidenceCaptureNoBrowserContext,
	ErrEvidenceCaptureScreenshotFailed,
	ErrEvidenceCaptureStatVerificationFailed,
}

// isBrowserCaptureSentinel reports whether err matches any of the
// round-61 browser-capture sentinels. Provided for callers/tests that
// want a single predicate rather than chaining errors.Is.
func isBrowserCaptureSentinel(err error) bool {
	if err == nil {
		return false
	}
	for _, s := range browserCaptureSentinels {
		if errors.Is(err, s) {
			return true
		}
	}
	return false
}
