// Package a11y audits Helix Nexus-driven UI screens against WCAG 2.2 A,
// AA, and AAA plus Section 508 via a vendored axe-core runner. The
// auditor is pure Go; it delegates actual axe execution to the browser
// Engine (chromedp/rod) through a small JS injection.
package a11y
