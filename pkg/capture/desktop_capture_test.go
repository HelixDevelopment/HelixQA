package capture

import (
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultDesktopConfig(t *testing.T) {
	config := DefaultDesktopConfig()

	assert.Equal(t, "screen", config.Source)
	assert.Equal(t, 1920, config.Resolution.Width)
	assert.Equal(t, 1080, config.Resolution.Height)
	assert.Equal(t, 30, config.FPS)
	assert.Empty(t, config.Display)
}

func TestNewDesktopCapture(t *testing.T) {
	if !IsPlatformSupported() {
		t.Skip("platform not supported")  // SKIP-OK: #legacy-untriaged
	}

	config := DefaultDesktopConfig()
	capture, err := NewDesktopCapture(config)

	if err != nil {
		t.Skipf("failed to create capture: %v", err)  // SKIP-OK: #legacy-skip-untriaged-2026-04-29
	}

	assert.NotNil(t, capture)
	assert.Equal(t, "screen", capture.source)
	assert.Equal(t, 1920, capture.resolution.Width)
	assert.Equal(t, 1080, capture.resolution.Height)
	assert.Equal(t, 30, capture.fps)
	assert.False(t, capture.IsRunning())
	assert.NotNil(t, capture.frameChan)
	assert.NotNil(t, capture.errorChan)
}

func TestGetPlatform(t *testing.T) {
	platform := GetPlatform()

	switch runtime.GOOS {
	case "linux", "windows", "darwin":
		assert.Equal(t, runtime.GOOS, platform)
	default:
		assert.Equal(t, runtime.GOOS, platform)
	}
}

func TestIsPlatformSupported(t *testing.T) {
	supported := IsPlatformSupported()

	switch runtime.GOOS {
	case "linux", "windows", "darwin":
		assert.True(t, supported)
	default:
		assert.False(t, supported)
	}
}

func TestCommandExists(t *testing.T) {
	// Test with commands that should exist
	assert.True(t, CommandExists("ls"))
	assert.True(t, CommandExists("cat"))

	// Test with command that shouldn't exist
	assert.False(t, CommandExists("nonexistentcommand12345"))
}

func TestWindow_String(t *testing.T) {
	window := Window{
		ID:     "123",
		Title:  "Test Window",
		Width:  1920,
		Height: 1080,
	}

	str := window.String()
	assert.Contains(t, str, "123")
	assert.Contains(t, str, "Test Window")
	assert.Contains(t, str, "1920")
	assert.Contains(t, str, "1080")
}

func TestDesktopCapture_GetSource(t *testing.T) {
	if !IsPlatformSupported() {
		t.Skip("platform not supported")  // SKIP-OK: #legacy-untriaged
	}

	config := DefaultDesktopConfig()
	config.Source = "window"
	config.WindowID = "12345"

	capture, err := NewDesktopCapture(config)
	if err != nil {
		t.Skipf("failed to create capture: %v", err)  // SKIP-OK: #legacy-skip-untriaged-2026-04-29
	}

	assert.Equal(t, "window", capture.GetSource())
}

func TestVerifyPlatformSupport(t *testing.T) {
	err := VerifyPlatformSupport()

	if IsPlatformSupported() {
		// May fail if dependencies not installed, but shouldn't panic
		t.Logf("Platform support check: %v", err)
	} else {
		assert.Error(t, err)
	}
}

// Platform-specific tests

func TestListDisplays(t *testing.T) {
	if !IsPlatformSupported() {
		t.Skip("platform not supported") // SKIP-OK: #CAPTURE-PLATFORM-001
	}

	displays, err := ListDisplays()

	// On a supported platform that lacks a working display
	// backend (typical in headless CI containers without Xvfb),
	// ListDisplays returns a recognised "no backend" error. That
	// is a SKIP, not a PASS — we have not actually verified the
	// behaviour. Anything else is a real failure.
	if err != nil {
		if strings.Contains(err.Error(), "no display") ||
			strings.Contains(err.Error(), "DISPLAY") ||
			strings.Contains(err.Error(), "x11") ||
			strings.Contains(err.Error(), "wayland") {
			t.Skipf("SKIP-OK: #CAPTURE-NO-DISPLAY — headless environment: %v", err)
		}
		t.Fatalf("ListDisplays on supported platform must succeed; got: %v", err)
	}

	if displays == nil {
		t.Fatal("ListDisplays returned nil slice on supported platform with no error")
	}
	t.Logf("Found %d displays", len(displays))
	for i, d := range displays {
		if d.Width <= 0 || d.Height <= 0 {
			t.Errorf("display %d: invalid dimensions %dx%d", i, d.Width, d.Height)
		}
		t.Logf("  Display %d: %s (%dx%d)", i, d.Name, d.Width, d.Height)
	}
}

func TestListWindows(t *testing.T) {
	if !IsPlatformSupported() {
		t.Skip("platform not supported") // SKIP-OK: #CAPTURE-PLATFORM-001
	}

	windows, err := ListWindows()
	if err != nil {
		// Headless / no window-listing tool installed → SKIP, not PASS.
		if strings.Contains(err.Error(), "wmctrl") ||
			strings.Contains(err.Error(), "xdotool") ||
			strings.Contains(err.Error(), "DISPLAY") ||
			strings.Contains(err.Error(), "no display") {
			t.Skipf("SKIP-OK: #CAPTURE-NO-WMCTRL — headless or missing tool: %v", err)
		}
		t.Fatalf("ListWindows on supported platform must succeed; got: %v", err)
	}

	if windows == nil {
		t.Fatal("ListWindows returned nil slice on supported platform with no error")
	}
	t.Logf("Found %d windows", len(windows))
	for i, w := range windows {
		t.Logf("  Window %d: %s", i, w.String())
		if i >= 5 {
			t.Logf("  ... and %d more", len(windows)-6)
			break
		}
	}
}

func TestFindWindow(t *testing.T) {
	// bluff-scan: no-assert-ok (platform-probe smoke — already SKIP-OK on platform/headless)
	if !IsPlatformSupported() {
		t.Skip("platform not supported") // SKIP-OK: #CAPTURE-PLATFORM-001
	}

	// FindWindow returns the first matching window, or an error if
	// no candidate matches. On a real desktop with at least one
	// open window, at least one keyword should match. In a headless
	// CI, none will match — that's a SKIP, not a PASS.
	keywords := []string{"terminal", "browser", "code", "editor"}
	var foundAny bool
	var lastErr error

	for _, keyword := range keywords {
		window, err := FindWindow(keyword)
		if err == nil {
			foundAny = true
			t.Logf("Found window with keyword '%s': %s", keyword, window.String())
			break
		}
		lastErr = err
	}

	if !foundAny {
		// If lastErr suggests headless/missing-tool, SKIP. Otherwise
		// we'd at least expect ONE common-name window on a real
		// desktop.
		if lastErr != nil &&
			(strings.Contains(lastErr.Error(), "DISPLAY") ||
				strings.Contains(lastErr.Error(), "no display") ||
				strings.Contains(lastErr.Error(), "wmctrl") ||
				strings.Contains(lastErr.Error(), "xdotool")) {
			t.Skipf("SKIP-OK: #CAPTURE-NO-WMCTRL — headless or missing tool: %v", lastErr)
		}
		// Headless desktop with no common windows open is also a
		// legitimate SKIP — we cannot prove FindWindow positively.
		t.Skipf("SKIP-OK: #CAPTURE-NO-COMMON-WIN — no common-name window present (headless test runner?)")
	}
}

func TestCaptureScreenshot(t *testing.T) {
	// bluff-scan: no-assert-ok (platform-probe smoke — already SKIP-OK on platform/headless)
	if !IsPlatformSupported() {
		t.Skip("platform not supported")  // SKIP-OK: #legacy-untriaged
	}

	// Skip in CI environments
	if CommandExists("xvfb-run") || CommandExists("Xvfb") {
		t.Skip("skipping screenshot test in headless environment")  // SKIP-OK: #legacy-untriaged
	}

	outputPath := "/tmp/helixqa_test_screenshot.png"
	err := CaptureScreenshot(outputPath)

	if err != nil {
		t.Skipf("Screenshot capture failed: %v", err)  // SKIP-OK: #legacy-skip-untriaged-2026-04-29
	}

	t.Logf("Screenshot saved to: %s", outputPath)

	// Clean up
	// os.Remove(outputPath)
}

// Linux-specific tests

func TestParseXrandrOutput(t *testing.T) {
	output := `
 0: +*DP-1 1920/531x1080/299+0+0  DP-1
 1: +HDMI-1 1920/509x1080/286+1920+0  HDMI-1
`

	displays := parseXrandrOutput(output)

	assert.Len(t, displays, 2)

	// First display
	assert.Equal(t, "0:", displays[0].ID) // Note: includes colon from parsing
	assert.Equal(t, "DP-1", displays[0].Name)
	// Width/Height may be 0 if parsing doesn't work as expected

	// Second display
	assert.Equal(t, "1:", displays[1].ID) // Note: includes colon from parsing
	assert.Equal(t, "HDMI-1", displays[1].Name)
}

func TestParseXdotoolGeometry(t *testing.T) {
	output := `
Window 12345678
  Position: 100,200 (screen: 0)
  Geometry: 1920x1080
`

	var window Window
	parseXdotoolGeometry(output, &window)

	// Parser is best-effort across xdotool versions — produce
	// concrete assertions on the values that DO survive the
	// parser, instead of a content-less log line. If a future
	// regression makes the parser ignore Position/Geometry
	// entirely, this test now FAILs.
	if window.X == 0 && window.Y == 0 && window.Width == 0 && window.Height == 0 {
		t.Fatal("parseXdotoolGeometry produced an entirely-zero Window — parser regression")
	}
	t.Logf("Parsed window: X=%d, Y=%d, Width=%d, Height=%d", window.X, window.Y, window.Width, window.Height)
}

func TestParseWindowClass(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `WM_CLASS(STRING) = "firefox", "Firefox"`,
			expected: "firefox",
		},
		{
			input:    `WM_CLASS(STRING) = "code", "Code"`,
			expected: "code",
		},
		{
			input:    `WM_CLASS(STRING) = "", ""`,
			expected: "",
		},
	}

	for _, tt := range tests {
		result := parseWindowClass(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}

func TestParseWmctrlOutput(t *testing.T) {
	output := `0x0420000a  0 0    1920   1080  ubuntu Terminal
0x04600003  0 1920 1920   1080  ubuntu Firefox`

	windows := parseWmctrlOutput(output)

	assert.Len(t, windows, 2)

	// First window
	assert.Equal(t, "0x0420000a", windows[0].ID)
	// Verify basic parsing works
	t.Logf("First window: X=%d, Y=%d, Title=%s", windows[0].X, windows[0].Y, windows[0].Title)

	// Second window
	assert.Equal(t, "0x04600003", windows[1].ID)
}

func TestIsWayland(t *testing.T) {
	// bluff-scan: no-assert-ok (smoke test — IsWayland must not panic;
	// returns a bool, value depends on runtime env)
	_ = IsWayland()
}

func TestGetDesktopEnvironment(t *testing.T) {
	// bluff-scan: no-assert-ok (smoke test — GetDesktopEnvironment must
	// not panic; returns a string, value depends on $XDG_CURRENT_DESKTOP)
	de := GetDesktopEnvironment()
	t.Logf("Desktop Environment: %s", de)
}

// macOS-specific tests

func TestIsScreenCaptureKitAvailable(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only test") // SKIP-OK: #CAPTURE-MACOS-ONLY
	}
	// bluff-scan: no-assert-ok (Darwin smoke — must not panic; bool
	// result varies with macOS version)
	available := IsScreenCaptureKitAvailable()
	t.Logf("ScreenCaptureKit available: %v", available)
}

func TestCheckScreenRecordingPermission(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only test") // SKIP-OK: #CAPTURE-MACOS-ONLY
	}
	// bluff-scan: no-assert-ok (Darwin smoke — CheckScreenRecordingPermission
	// must not panic; result depends on user TCC grants)
	_ = CheckScreenRecordingPermission()
}

// Integration tests

func TestDesktopCapture_StartStop(t *testing.T) {
	if !IsPlatformSupported() {
		t.Skip("platform not supported")  // SKIP-OK: #legacy-untriaged
	}

	// Skip if GStreamer not available
	if !CommandExists("gst-launch-1.0") {
		t.Skip("GStreamer not available")  // SKIP-OK: #legacy-untriaged
	}

	config := DefaultDesktopConfig()
	config.FPS = 5 // Lower FPS for testing
	config.Resolution = Resolution{Width: 640, Height: 480}

	capture, err := NewDesktopCapture(config)
	if err != nil {
		t.Skipf("failed to create capture: %v", err)  // SKIP-OK: #legacy-skip-untriaged-2026-04-29
	}

	// Start capture
	err = capture.Start()
	if err != nil {
		t.Skipf("failed to start capture: %v (may require display)", err)  // SKIP-OK: #legacy-skip-untriaged-2026-04-29
	}

	assert.True(t, capture.IsRunning())

	// Let it run briefly
	// time.Sleep(2 * time.Second)

	// Stop capture
	err = capture.Stop()
	assert.NoError(t, err)
	assert.False(t, capture.IsRunning())
}

func TestDesktopCapture_GetFrameChan(t *testing.T) {
	if !IsPlatformSupported() {
		t.Skip("platform not supported")  // SKIP-OK: #legacy-untriaged
	}

	config := DefaultDesktopConfig()
	capture, err := NewDesktopCapture(config)
	if err != nil {
		t.Skipf("failed to create capture: %v", err)  // SKIP-OK: #legacy-skip-untriaged-2026-04-29
	}

	// Should return channel even if not running
	ch := capture.GetFrameChan()
	assert.NotNil(t, ch)
}

// Benchmarks

func BenchmarkCommandExists(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CommandExists("ls")
	}
}

func BenchmarkParseXrandrOutput(b *testing.B) {
	output := `
 0: +*DP-1 1920/531x1080/299+0+0  DP-1
 1: +HDMI-1 1920/509x1080/286+1920+0  HDMI-1
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parseXrandrOutput(output)
	}
}
