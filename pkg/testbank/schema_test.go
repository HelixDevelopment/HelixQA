// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAction_ADBShell(t *testing.T) {
	s := TestStep{Action: "adb_shell: am start -n com.catalogizer.androidtv/.ui.MainActivity"}
	at, val := s.ParseAction()
	assert.Equal(t, ActionTypeADBShell, at)
	assert.Equal(t, "am start -n com.catalogizer.androidtv/.ui.MainActivity", val)
}

func TestParseAction_Keypress(t *testing.T) {
	s := TestStep{Action: "keypress: KEYCODE_DPAD_CENTER"}
	at, val := s.ParseAction()
	assert.Equal(t, ActionTypeKeyPress, at)
	assert.Equal(t, "KEYCODE_DPAD_CENTER", val)
}

func TestParseAction_Text(t *testing.T) {
	s := TestStep{Action: "text: admin123"}
	at, val := s.ParseAction()
	assert.Equal(t, ActionTypeText, at)
	assert.Equal(t, "admin123", val)
}

func TestParseAction_Sleep(t *testing.T) {
	s := TestStep{Action: "sleep: 5000"}
	at, val := s.ParseAction()
	assert.Equal(t, ActionTypeSleep, at)
	assert.Equal(t, "5000", val)
}

func TestParseAction_Tap(t *testing.T) {
	s := TestStep{Action: "tap: 960,540"}
	at, val := s.ParseAction()
	assert.Equal(t, ActionTypeTap, at)
	assert.Equal(t, "960,540", val)
}

func TestParseAction_ScreenshotStandalone(t *testing.T) {
	// This was the bug: "screenshot" without colon was treated as description
	s := TestStep{Action: "screenshot"}
	at, _ := s.ParseAction()
	assert.Equal(t, ActionTypeScreenshot, at, "standalone 'screenshot' must be recognized as screenshot action")
}

func TestParseAction_ScreenshotWithColon(t *testing.T) {
	s := TestStep{Action: "screenshot:"}
	at, _ := s.ParseAction()
	assert.Equal(t, ActionTypeScreenshot, at)
}

func TestParseAction_ScreenshotCaseInsensitive(t *testing.T) {
	s := TestStep{Action: "Screenshot"}
	at, _ := s.ParseAction()
	assert.Equal(t, ActionTypeScreenshot, at, "case-insensitive 'Screenshot' must be recognized")
}

func TestParseAction_PlaybackCheck(t *testing.T) {
	s := TestStep{Action: "playback_check: com.catalogizer.androidtv"}
	at, val := s.ParseAction()
	assert.Equal(t, ActionTypePlaybackCheck, at)
	assert.Equal(t, "com.catalogizer.androidtv", val)
}

func TestParseAction_FrameDiff(t *testing.T) {
	s := TestStep{Action: "frame_diff: 2000"}
	at, val := s.ParseAction()
	assert.Equal(t, ActionTypeFrameDiff, at)
	assert.Equal(t, "2000", val)
}

func TestParseAction_Description(t *testing.T) {
	s := TestStep{Action: "Navigate to the login screen and enter credentials"}
	at, val := s.ParseAction()
	assert.Equal(t, ActionTypeDescription, at)
	assert.Equal(t, "Navigate to the login screen and enter credentials", val)
}

func TestParseAction_Empty(t *testing.T) {
	s := TestStep{Action: ""}
	at, _ := s.ParseAction()
	assert.Equal(t, ActionTypeDescription, at)
}

func TestParseAction_TODO(t *testing.T) {
	s := TestStep{Action: "# TODO: Convert to executable - launch app"}
	at, _ := s.ParseAction()
	assert.Equal(t, ActionTypeDescription, at, "TODO placeholders remain as descriptions")
}

func TestParseAction_HTTP(t *testing.T) {
	cases := []struct {
		in, val string
	}{
		{"http: GET /api/v1/health", "GET /api/v1/health"},
		{"http: POST /api/v1/auth/login", "POST /api/v1/auth/login"},
		{"http: DELETE /api/v1/users/42", "DELETE /api/v1/users/42"},
	}
	for _, tc := range cases {
		s := TestStep{Action: tc.in}
		at, val := s.ParseAction()
		assert.Equal(t, ActionTypeHTTP, at, "input %q", tc.in)
		assert.Equal(t, tc.val, val, "input %q", tc.in)
	}
}

func TestParseAction_Assert(t *testing.T) {
	cases := []struct {
		in, val string
	}{
		{"assert: status_eq: 200", "status_eq: 200"},
		{"assert: json_path_eq: $.id = 42", "json_path_eq: $.id = 42"},
		{"assert: body_contains: hello", "body_contains: hello"},
		{"assert: header_eq: Content-Type = application/json", "header_eq: Content-Type = application/json"},
	}
	for _, tc := range cases {
		s := TestStep{Action: tc.in}
		at, val := s.ParseAction()
		assert.Equal(t, ActionTypeAssert, at, "input %q", tc.in)
		assert.Equal(t, tc.val, val, "input %q", tc.in)
	}
}

// TestParseAction_HTTPRoundTripsBodyHeaders ensures the new
// step fields (Body / Headers / ExpectStatus / ExpectJSONPath /
// ExpectBodyContains / AuthMode) survive YAML round-trips. This
// is the structural contract the bank converter relies on.
func TestParseAction_HTTPRoundTripsBodyHeaders(t *testing.T) {
	s := TestStep{
		Action:             "http: POST /api/v1/auth/login",
		Body:               map[string]string{"username": "admin", "password": "admin123"},
		Headers:            map[string]string{"X-Test": "1"},
		AuthMode:           "none",
		ExpectStatus:       200,
		ExpectJSONPath:     "$.session_token",
		ExpectBodyContains: "session",
	}
	at, val := s.ParseAction()
	assert.Equal(t, ActionTypeHTTP, at)
	assert.Equal(t, "POST /api/v1/auth/login", val)
	// All structured fields preserved.
	assert.NotNil(t, s.Body)
	assert.Equal(t, 200, s.ExpectStatus)
	assert.Equal(t, "$.session_token", s.ExpectJSONPath)
	assert.Equal(t, "session", s.ExpectBodyContains)
	assert.Equal(t, "1", s.Headers["X-Test"])
	assert.Equal(t, "none", s.AuthMode)
}
