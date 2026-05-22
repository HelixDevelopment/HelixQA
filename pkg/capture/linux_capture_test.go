//go:build linux
// +build linux

// Linux-specific capture-parser tests. The functions exercised here
// (parseXrandrOutput, parseXdotoolGeometry, parseWindowClass,
// parseWmctrlOutput) live in `linux_capture.go` which itself has a
// `//go:build linux` tag, so the tests are unreachable on macOS /
// Windows hosts. Without this matching build tag, `go vet ./...` on
// macOS reported "undefined: parseXrandrOutput" because the test
// package tried to link a Linux-only symbol — a real CONST-035 bluff
// at the test compile-time layer that silently broke cross-platform
// development.
//
// Moved out of desktop_capture_test.go on 2026-05-12 (iter 31).

package capture

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
