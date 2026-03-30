// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleUIXML returns a minimal but realistic UI automator
// dump for testing.
func sampleUIXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<hierarchy rotation="0">
  <node class="android.widget.FrameLayout" text=""
        clickable="false" focused="false" enabled="true"
        bounds="[0,0][1920,1080]">
    <node class="android.widget.TextView" text="Home"
          clickable="false" focused="true" enabled="true"
          resource-id="com.app:id/title"
          bounds="[100,50][400,100]" content-desc=""
          selected="false" />
    <node class="android.widget.Button" text="Browse"
          clickable="true" focused="false" enabled="true"
          resource-id="com.app:id/btn_browse"
          bounds="[100,200][400,260]" content-desc=""
          selected="false" />
    <node class="android.widget.Button" text="Settings"
          clickable="true" focused="false" enabled="true"
          resource-id="com.app:id/btn_settings"
          bounds="[100,300][400,360]" content-desc=""
          selected="false" />
    <node class="android.widget.ImageView" text=""
          clickable="true" focused="false" enabled="true"
          resource-id="com.app:id/logo"
          bounds="[800,50][900,150]"
          content-desc="App Logo" selected="false" />
    <node class="android.widget.Button" text=""
          clickable="true" focused="false" enabled="false"
          bounds="[100,400][400,460]"
          content-desc="Disabled" selected="false" />
  </node>
</hierarchy>`
}

// TestSummarizeUITree_ValidXML verifies that SummarizeUITree
// extracts focused, text, and clickable elements.
func TestSummarizeUITree_ValidXML(t *testing.T) {
	summary := SummarizeUITree(sampleUIXML())

	// Should contain the focused element.
	assert.Contains(t, summary, "FOCUSED:")
	assert.Contains(t, summary, "Home")
	assert.Contains(t, summary, "com.app:id/title")

	// Should contain text entries.
	assert.Contains(t, summary, "TEXT:")
	assert.Contains(t, summary, "Browse")
	assert.Contains(t, summary, "Settings")
	assert.Contains(t, summary, "App Logo")

	// Should contain clickable+enabled elements.
	assert.Contains(t, summary, "CLICKABLE:")
	assert.Contains(t, summary, "Browse")
	assert.Contains(t, summary, "Settings")
	assert.Contains(t, summary, "App Logo")

	// Disabled button should NOT appear in clickables.
	// It may appear in TEXT (from content-desc), but the
	// CLICKABLE section must exclude disabled elements.
	clickableLine := ""
	for _, line := range strings.Split(summary, "\n") {
		if strings.HasPrefix(line, "CLICKABLE:") {
			clickableLine = line
			break
		}
	}
	assert.NotContains(t, clickableLine, "Disabled")
}

// TestSummarizeUITree_EmptyXML verifies the empty case.
func TestSummarizeUITree_EmptyXML(t *testing.T) {
	summary := SummarizeUITree("")
	assert.Equal(t, "(empty UI tree)", summary)
}

// TestSummarizeUITree_InvalidXML verifies that invalid XML
// returns a parse error message.
func TestSummarizeUITree_InvalidXML(t *testing.T) {
	summary := SummarizeUITree("<not valid xml")
	assert.Equal(t, "(UI tree parse error)", summary)
}

// TestSummarizeUITree_NoElements verifies that an empty
// hierarchy returns the empty marker.
func TestSummarizeUITree_NoElements(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<hierarchy rotation="0"></hierarchy>`
	summary := SummarizeUITree(xml)
	assert.Equal(t, "(empty UI tree)", summary)
}

// TestSummarizeUITree_ManyTexts verifies that text entries
// are limited to 20.
func TestSummarizeUITree_ManyTexts(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`<hierarchy rotation="0"><node class="root"
		text="" clickable="false" focused="false" enabled="true"
		bounds="[0,0][100,100]">`)
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf(
			`<node class="tv" text="Item%d"
			 clickable="false" focused="false" enabled="true"
			 bounds="[0,0][100,100]" content-desc=""
			 selected="false" />`, i,
		))
	}
	sb.WriteString(`</node></hierarchy>`)

	summary := SummarizeUITree(sb.String())
	assert.Contains(t, summary, "...")
}

// TestSummarizeUITree_ManyClickables verifies that clickable
// elements are limited to 15.
func TestSummarizeUITree_ManyClickables(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`<hierarchy rotation="0"><node class="root"
		text="" clickable="false" focused="false" enabled="true"
		bounds="[0,0][100,100]">`)
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf(
			`<node class="btn" text="Btn%d"
			 clickable="true" focused="false" enabled="true"
			 bounds="[%d,0][%d,50]" content-desc=""
			 selected="false" />`, i, i*50, i*50+50,
		))
	}
	sb.WriteString(`</node></hierarchy>`)

	summary := SummarizeUITree(sb.String())
	assert.Contains(t, summary, "5 more")
}

// TestSummarizeUITree_DeduplicateTexts verifies that
// duplicate text entries are deduplicated.
func TestSummarizeUITree_DeduplicateTexts(t *testing.T) {
	xml := `<hierarchy rotation="0">
	<node class="root" text="" clickable="false" focused="false"
		  enabled="true" bounds="[0,0][100,100]">
	  <node class="tv" text="Duplicate" clickable="false"
	        focused="false" enabled="true"
	        bounds="[0,0][100,50]" content-desc=""
	        selected="false" />
	  <node class="tv" text="Duplicate" clickable="false"
	        focused="false" enabled="true"
	        bounds="[0,50][100,100]" content-desc=""
	        selected="false" />
	  <node class="tv" text="Unique" clickable="false"
	        focused="false" enabled="true"
	        bounds="[0,100][100,150]" content-desc=""
	        selected="false" />
	</node>
	</hierarchy>`

	summary := SummarizeUITree(xml)
	// Count occurrences of "Duplicate" in the TEXT section.
	textLine := ""
	for _, line := range strings.Split(summary, "\n") {
		if strings.HasPrefix(line, "TEXT:") {
			textLine = line
			break
		}
	}
	count := strings.Count(textLine, "Duplicate")
	assert.Equal(t, 1, count, "Duplicate should appear once")
}

// TestDualScreenCapturer_CaptureDualScreen_Success verifies
// the happy path where both screenshot and UI dump succeed.
func TestDualScreenCapturer_CaptureDualScreen_Success(
	t *testing.T,
) {
	runner := newMockRunner()

	// First call: screencap
	runner.response = []byte("PNG-DATA")

	capturer := NewDualScreenCapturer(runner)
	result, err := capturer.CaptureDualScreen(
		context.Background(), "emulator-5554",
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []byte("PNG-DATA"), result.Screenshot)
}

// TestDualScreenCapturer_CaptureDualScreen_ScreenshotFail
// verifies that a screenshot failure returns an error.
func TestDualScreenCapturer_CaptureDualScreen_ScreenshotFail(
	t *testing.T,
) {
	runner := newMockRunner()
	runner.failOn["adb"] = fmt.Errorf("device offline")

	capturer := NewDualScreenCapturer(runner)
	result, err := capturer.CaptureDualScreen(
		context.Background(), "emulator-5554",
	)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "screenshot")
}

// TestDualScreenCapturer_CaptureDualScreen_UIFail verifies
// that a UI dump failure returns a partial result with the
// screenshot.
func TestDualScreenCapturer_CaptureDualScreen_UIFail(
	t *testing.T,
) {
	// We need the first call (screencap) to succeed and
	// the second call (uiautomator) to fail.
	callCount := 0
	runner := &sequentialMockRunner{
		responses: []mockResponse{
			{data: []byte("PNG-DATA"), err: nil},
			{data: nil, err: fmt.Errorf("dump failed")},
		},
		callCount: &callCount,
	}

	capturer := NewDualScreenCapturer(runner)
	result, err := capturer.CaptureDualScreen(
		context.Background(), "emulator-5554",
	)
	require.NoError(t, err) // partial result, not error
	require.NotNil(t, result)
	assert.Equal(t, []byte("PNG-DATA"), result.Screenshot)
	assert.Equal(t, "", result.UITree)
	assert.Contains(t, result.Combined, "unavailable")
}

// TestCaptureUITree_StripTrailer verifies that the
// "UI hierchary" trailer is stripped from the dump output.
func TestCaptureUITree_StripTrailer(t *testing.T) {
	runner := newMockRunner()
	xmlData := sampleUIXML()
	runner.response = []byte(
		xmlData + "\nUI hierchary dumped to: /dev/tty",
	)

	result, err := captureUITree(
		context.Background(), runner, "emulator-5554",
	)
	require.NoError(t, err)
	assert.NotContains(t, result, "UI hierchary")
	assert.Contains(t, result, "<hierarchy")
}

// TestCaptureUITree_NullRootNode verifies that a "null root
// node" response is treated as an error.
func TestCaptureUITree_NullRootNode(t *testing.T) {
	runner := newMockRunner()
	runner.response = []byte("null root node")

	_, err := captureUITree(
		context.Background(), runner, "emulator-5554",
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "null root node")
}

// sequentialMockRunner returns different responses for
// successive calls.
type sequentialMockRunner struct {
	responses []mockResponse
	callCount *int
}

type mockResponse struct {
	data []byte
	err  error
}

func (m *sequentialMockRunner) Run(
	_ context.Context, _ string, _ ...string,
) ([]byte, error) {
	idx := *m.callCount
	*m.callCount++
	if idx >= len(m.responses) {
		return nil, fmt.Errorf("no more mock responses")
	}
	r := m.responses[idx]
	return r.data, r.err
}
