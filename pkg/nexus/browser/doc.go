// Package browser is the Nexus browser automation engine. It layers
// OpenClaw-style role-based element referencing and AI-friendly error
// translation on top of chromedp and go-rod, with a unified Engine
// facade that satisfies nexus.Adapter.
//
// The driver implementations are guarded by Go build tags so the default
// build of HelixQA does not require a Chromium binary:
//
//	nexus_chromedp  enables chromedp driver
//	nexus_rod       enables go-rod driver
//
// Refer to docs/nexus/browser.md for the full architecture and the
// migration path from the existing Playwright adapter.
package browser
