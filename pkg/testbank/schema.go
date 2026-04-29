// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package testbank provides QA-specific test bank management
// with YAML-based test case definitions. It bridges to the
// Challenges framework's bank package for execution while
// adding QA metadata like platform targeting, priority, and
// documentation references.
package testbank

import (
	"strings"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.helixqa/pkg/config"
)

// Priority levels for test cases.
type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityHigh     Priority = "high"
	PriorityMedium   Priority = "medium"
	PriorityLow      Priority = "low"
)

// TestCase is a QA-specific test definition that extends the
// Challenges Definition with platform, priority, and
// documentation references.
type TestCase struct {
	// ID uniquely identifies the test case.
	ID string `yaml:"id" json:"id"`

	// Name is the human-readable test name.
	Name string `yaml:"name" json:"name"`

	// Description explains what this test validates.
	Description string `yaml:"description" json:"description"`

	// Category groups related tests (e.g., "functional",
	// "edge_case", "integration", "security").
	Category string `yaml:"category" json:"category"`

	// Priority indicates test importance for scheduling.
	Priority Priority `yaml:"priority" json:"priority"`

	// Platforms specifies which platforms this test targets.
	// Empty means all platforms.
	Platforms []config.Platform `yaml:"platforms" json:"platforms"`

	// Steps lists the ordered test steps to execute.
	Steps []TestStep `yaml:"steps" json:"steps"`

	// Dependencies lists test case IDs that must pass first.
	Dependencies []string `yaml:"dependencies" json:"dependencies"`

	// DocumentationRefs links to relevant docs for
	// inconsistency detection.
	DocumentationRefs []DocRef `yaml:"documentation_refs" json:"documentation_refs"`

	// Tags provides free-form labels for filtering.
	Tags []string `yaml:"tags" json:"tags"`

	// EstimatedDuration is the expected execution time.
	EstimatedDuration string `yaml:"estimated_duration" json:"estimated_duration"`

	// ExpectedResult describes the expected outcome.
	ExpectedResult string `yaml:"expected_result" json:"expected_result"`

	// AllowForegroundLeave opts this test out of the structured-phase
	// foreground-drift guard. Set to true for tests that intentionally
	// exercise system overlays or the launcher (voice search, tv
	// channels, watch-next-row), where drifting to `ihq`, Google
	// Katniss, IPTV Pro, RuTube, etc. is the test subject, not a
	// bug. Defaults to false — tests MUST stay in the target app.
	AllowForegroundLeave bool `yaml:"allow_foreground_leave,omitempty" json:"allow_foreground_leave,omitempty"`

	// RequiresEnv lists environment variables that MUST be set for
	// this test case to run. If any are unset or empty, the executor
	// SKIP-OKs the entire test case rather than failing — Article XI
	// §11.5 demands honest skips for env-dependent tests, not silent
	// fails (or worse, silent passes).
	//
	// Examples:
	//   requires_env: [HELIXQA_LAB_HAS_4K_MEDIA]   // for 4K-on-1080p tests
	//   requires_env: [HELIXQA_LAB_HAS_USB_KBD]    // for external-keyboard tests
	//   requires_env: [HELIXQA_LAB_HAS_SOUNDBAR]   // for audio-delay tests
	//   requires_env: [HELIXQA_LAB_FULL_HARDWARE]  // generic catch-all
	//
	// Operators set the var(s) in their .env (or HelixQA/.env) when
	// the corresponding hardware is available. Lab installs without
	// the hardware see honest SKIP-OK results — anti-bluff compliant.
	RequiresEnv []string `yaml:"requires_env,omitempty" json:"requires_env,omitempty"`
}

// ActionType identifies the type of action to execute.
type ActionType string

const (
	// ActionTypeDescription is a text-only action (legacy, non-executable).
	ActionTypeDescription ActionType = "description"
	// ActionTypeADBShell executes an ADB shell command.
	ActionTypeADBShell ActionType = "adb_shell"
	// ActionTypeSleep waits for a specified duration.
	ActionTypeSleep ActionType = "sleep"
	// ActionTypeScreenshot captures a screenshot.
	ActionTypeScreenshot ActionType = "screenshot"
	// ActionTypeKeyPress simulates a key press.
	ActionTypeKeyPress ActionType = "keypress"
	// ActionTypeTap taps at coordinates.
	ActionTypeTap ActionType = "tap"
	// ActionTypeSwipe performs a swipe gesture.
	ActionTypeSwipe ActionType = "swipe"
	// ActionTypeText enters text.
	ActionTypeText ActionType = "text"
	// ActionTypePlaybackCheck queries Android `dumpsys media_session`
	// and verifies that at least one media session for the given
	// package (or any package if the value is "*") is in the
	// PlaybackState PLAYING (state=3). Used to confirm a test case
	// that pressed a Play button actually caused playback to start.
	// Value format: "<package>" or "<package>:<minState>" where
	// minState is the minimum PlaybackState integer to accept
	// (default 3 = PLAYING).
	ActionTypePlaybackCheck ActionType = "playback_check"
	// ActionTypeFrameDiff captures a screenshot, waits the given
	// number of milliseconds, captures a second screenshot, and
	// returns success if the two frames differ by more than the
	// similarity threshold. Used to confirm video playback is
	// actually rendering (not a frozen first frame). Value format:
	// "<waitMs>" — defaults to 2000 ms.
	ActionTypeFrameDiff ActionType = "frame_diff"
	// ActionTypeHTTP performs an HTTP request against a backend
	// API and asserts on the response. Value format:
	// "<METHOD> <PATH>" e.g. "GET /api/v1/health" or
	// "POST /api/v1/auth/login". Body, headers, expected status,
	// and JSON-path assertions are supplied via the dedicated
	// TestStep fields (Body, Headers, ExpectStatus,
	// ExpectJSONPath, ExpectBodyContains, AuthMode). The base URL
	// comes from the executor's HTTPBaseURL config.
	//
	// Added 2026-04-29 to close the BLUFF-HELIXQA-BANKS-REWRITE-001
	// gap: prior banks (full-qa-api, full-qa-web, atmosphere) used
	// prose ActionTypeDescription strings that the executor could
	// not run, producing 4034 PROSE_HELIXQA_ACTION findings in the
	// bluff audit. With this type, those banks become structurally
	// executable.
	ActionTypeHTTP ActionType = "http"
	// ActionTypeAssert evaluates a structured assertion against
	// the most recent HTTP response, captured screenshot, or
	// device state. Value format: "<kind>: <expr>" — kinds are
	// status_eq, json_path_eq, body_contains, header_eq.
	// Used in tandem with ActionTypeHTTP for complex assertions
	// that span multiple checks; simpler one-shot expectations
	// can ride on the HTTP step's ExpectStatus / ExpectJSONPath
	// / ExpectBodyContains fields directly.
	ActionTypeAssert ActionType = "assert"
	// ActionTypePlaywright drives a web browser via the Playwright
	// adapter for deterministic UI test bank steps. Value format:
	//   "<verb> <selector|target>"
	// where verb is one of:
	//   navigate <url>            — open a URL
	//   click    <selector>       — click an element
	//   fill     <selector> <txt> — type into an input
	//   waitFor  <selector>       — wait for element
	//   assertVisible    <selector>
	//   assertNotVisible <selector>
	//   press    <key>            — keyboard press (e.g. Enter)
	//
	// Selectors are Playwright selectors (CSS, text=, role=, etc).
	//
	// Added 2026-04-29 for BLUFF-HELIXQA-BANKS-REWRITE-001 step 3
	// to bring full-qa-web.json (572 prose entries) under the
	// structured-action umbrella. The current executor stub
	// records the step as structurally valid but needs the
	// Playwright runtime wired in via PlaywrightCLIAdapter from
	// the Challenges submodule (separate PR — banks are
	// converted ahead of the runtime so a single integration
	// commit closes the gap). Until then ActionTypePlaywright
	// steps SKIP with PLAYWRIGHT-RUNTIME-PENDING rather than
	// false-PASS, keeping Article XI compliance.
	ActionTypePlaywright ActionType = "playwright"
)

// TestStep is a single step within a test case.
type TestStep struct {
	// Name identifies this step.
	Name string `yaml:"name" json:"name"`

	// Action describes what to do.
	// For executable actions, use format: "type: value"
	// Examples:
	//   "adb_shell: input keyevent KEYCODE_ENTER"
	//   "sleep: 5000" (milliseconds)
	//   "screenshot"
	//   "keypress: KEYCODE_DPAD_DOWN"
	//   "text: admin"
	Action string `yaml:"action" json:"action"`

	// Expected describes the expected outcome.
	Expected string `yaml:"expected" json:"expected"`

	// Platform limits this step to a specific platform.
	// Empty means it applies to all.
	Platform config.Platform `yaml:"platform,omitempty" json:"platform,omitempty"`

	// Timeout is the maximum time to wait for this step (in seconds).
	// Default is 30 seconds.
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// VisionVerify enables LLM vision verification of the result.
	VisionVerify bool `yaml:"vision_verify,omitempty" json:"vision_verify,omitempty"`

	// Body is the HTTP request body for ActionTypeHTTP. Strings
	// are sent verbatim; map/slice values are JSON-encoded.
	Body any `yaml:"body,omitempty" json:"body,omitempty"`

	// Headers carries extra HTTP headers for ActionTypeHTTP.
	// Common defaults (Accept, Content-Type for JSON bodies) are
	// added by the executor unless overridden here.
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// AuthMode selects how authentication is applied to an
	// ActionTypeHTTP step. Recognized values:
	//   "none"      — no Authorization header (default)
	//   "admin"     — log in as admin once per session, attach
	//                 Bearer <session_token>
	//   "as:<user>" — log in as the named user (creds resolved
	//                 from the executor's UserCredentials map)
	//   "raw:<token>" — attach Bearer <token> verbatim
	AuthMode string `yaml:"auth,omitempty" json:"auth,omitempty"`

	// ExpectStatus is the expected HTTP status code (e.g. 200,
	// 201, 401). Zero means "do not check status".
	ExpectStatus int `yaml:"expect_status,omitempty" json:"expect_status,omitempty"`

	// ExpectJSONPath is a single JSON-path expression that must
	// resolve to a non-null value in the HTTP response body.
	// Example: "$.session_token". Empty means "do not check".
	ExpectJSONPath string `yaml:"expect_json_path,omitempty" json:"expect_json_path,omitempty"`

	// ExpectBodyContains is a substring that must appear in the
	// HTTP response body (case-sensitive). Empty means
	// "do not check".
	ExpectBodyContains string `yaml:"expect_body_contains,omitempty" json:"expect_body_contains,omitempty"`
}

// ParseAction parses the action string and returns the type and value.
// Format: "type: value" or just "description" for legacy text actions.
func (ts *TestStep) ParseAction() (ActionType, string) {
	if ts.Action == "" {
		return ActionTypeDescription, ""
	}

	// Handle standalone "screenshot" keyword (no colon needed)
	trimmed := strings.TrimSpace(ts.Action)
	if strings.EqualFold(trimmed, "screenshot") {
		return ActionTypeScreenshot, ""
	}

	// Check for explicit type prefix
	if idx := strings.Index(ts.Action, ":"); idx > 0 {
		prefix := ts.Action[:idx]
		value := strings.TrimSpace(ts.Action[idx+1:])

		switch ActionType(prefix) {
		case ActionTypeADBShell:
			return ActionTypeADBShell, value
		case ActionTypeSleep:
			return ActionTypeSleep, value
		case ActionTypeScreenshot:
			return ActionTypeScreenshot, ""
		case ActionTypeKeyPress:
			return ActionTypeKeyPress, value
		case ActionTypeTap:
			return ActionTypeTap, value
		case ActionTypeSwipe:
			return ActionTypeSwipe, value
		case ActionTypeText:
			return ActionTypeText, value
		case ActionTypePlaybackCheck:
			return ActionTypePlaybackCheck, value
		case ActionTypeFrameDiff:
			return ActionTypeFrameDiff, value
		case ActionTypeHTTP:
			return ActionTypeHTTP, value
		case ActionTypeAssert:
			return ActionTypeAssert, value
		case ActionTypePlaywright:
			return ActionTypePlaywright, value
		}
	}

	// Legacy: treat as description
	return ActionTypeDescription, ts.Action
}

// DocRef references a documentation source for consistency
// verification.
type DocRef struct {
	// Type is the doc type (e.g., "user_guide", "api_spec",
	// "video_course", "architecture").
	Type string `yaml:"type" json:"type"`

	// Section is the specific section or page reference.
	Section string `yaml:"section" json:"section"`

	// Path is the file path or URL.
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
}

// BankFile represents the YAML structure of a test bank file.
type BankFile struct {
	// Version is the bank file format version.
	Version string `yaml:"version" json:"version"`

	// Name identifies this bank.
	Name string `yaml:"name" json:"name"`

	// Description explains the bank's purpose.
	Description string `yaml:"description" json:"description"`

	// TestCases holds all test cases in this bank.
	TestCases []TestCase `yaml:"test_cases" json:"test_cases"`

	// Metadata holds arbitrary key-value pairs.
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// ToDefinition converts a TestCase to a Challenges Definition
// for execution by the runner.
func (tc *TestCase) ToDefinition() *challenge.Definition {
	deps := make([]challenge.ID, len(tc.Dependencies))
	for i, d := range tc.Dependencies {
		deps[i] = challenge.ID(d)
	}

	return &challenge.Definition{
		ID:                challenge.ID(tc.ID),
		Name:              tc.Name,
		Description:       tc.Description,
		Category:          tc.Category,
		Dependencies:      deps,
		EstimatedDuration: tc.EstimatedDuration,
	}
}

// AppliesToPlatform returns true if this test case targets the
// given platform. An empty Platforms list means all platforms.
func (tc *TestCase) AppliesToPlatform(p config.Platform) bool {
	if len(tc.Platforms) == 0 {
		return true
	}
	for _, tp := range tc.Platforms {
		if tp == p || tp == config.PlatformAll {
			return true
		}
	}
	return false
}

// IsValid returns an error message if the test case has
// missing required fields, or empty string if valid.
func (tc *TestCase) IsValid() string {
	if tc.ID == "" {
		return "test case missing ID"
	}
	if tc.Name == "" {
		return "test case " + tc.ID + " missing name"
	}
	return ""
}
