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
