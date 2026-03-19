// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
)

func TestVideoReference_Validate_Valid(t *testing.T) {
	vr := &VideoReference{
		VideoPath:   "/videos/android-session.mp4",
		Timestamp:   14*time.Minute + 32*time.Second,
		Description: "Navigation to settings",
	}
	assert.NoError(t, vr.Validate())
}

func TestVideoReference_Validate_MissingPath(t *testing.T) {
	vr := &VideoReference{
		Timestamp: 5 * time.Second,
	}
	err := vr.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "video path")
}

func TestVideoReference_Validate_NegativeTimestamp(t *testing.T) {
	vr := &VideoReference{
		VideoPath: "/video.mp4",
		Timestamp: -1 * time.Second,
	}
	err := vr.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-negative")
}

func TestVideoReference_Validate_ZeroTimestamp(t *testing.T) {
	vr := &VideoReference{
		VideoPath: "/video.mp4",
		Timestamp: 0,
	}
	assert.NoError(t, vr.Validate())
}

func TestVideoReference_Validate_EmptyDescription(t *testing.T) {
	vr := &VideoReference{
		VideoPath:   "/video.mp4",
		Timestamp:   10 * time.Second,
		Description: "",
	}
	assert.NoError(t, vr.Validate())
}

func TestLLMSuggestedFix_Validate_Valid(t *testing.T) {
	sf := &LLMSuggestedFix{
		Description:   "Add nil check before accessing list",
		CodeSnippet:   "if formats != nil {\n  // use formats\n}",
		Confidence:    0.85,
		AffectedFiles: []string{"FormatRegistry.kt"},
	}
	assert.NoError(t, sf.Validate())
}

func TestLLMSuggestedFix_Validate_MissingDescription(t *testing.T) {
	sf := &LLMSuggestedFix{
		Confidence: 0.5,
	}
	err := sf.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

func TestLLMSuggestedFix_Validate_InvalidConfidence(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
	}{
		{"negative", -0.1},
		{"too_high", 1.1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &LLMSuggestedFix{
				Description: "fix something",
				Confidence:  tt.confidence,
			}
			err := sf.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "confidence")
		})
	}
}

func TestLLMSuggestedFix_Validate_BoundaryConfidence(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
	}{
		{"zero", 0.0},
		{"one", 1.0},
		{"mid", 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &LLMSuggestedFix{
				Description: "a fix",
				Confidence:  tt.confidence,
			}
			assert.NoError(t, sf.Validate())
		})
	}
}

func TestLLMSuggestedFix_Validate_NoCodeSnippet(t *testing.T) {
	sf := &LLMSuggestedFix{
		Description: "Increase timeout value",
		Confidence:  0.7,
	}
	assert.NoError(t, sf.Validate())
}

func TestLLMSuggestedFix_Validate_NoAffectedFiles(t *testing.T) {
	sf := &LLMSuggestedFix{
		Description: "General refactoring",
		Confidence:  0.6,
	}
	assert.NoError(t, sf.Validate())
}

func TestTicket_WithVideoRefs_Nil(t *testing.T) {
	ticket := &Ticket{
		ID:        "HQA-0001",
		Title:     "Simple issue",
		Severity:  SeverityLow,
		Platform:  config.PlatformWeb,
		CreatedAt: time.Now(),
		VideoRefs: nil,
	}
	assert.Nil(t, ticket.VideoRefs)
}

func TestTicket_WithVideoRefs_Present(t *testing.T) {
	ticket := &Ticket{
		ID:       "HQA-0001",
		Title:    "Issue with video",
		Severity: SeverityMedium,
		Platform: config.PlatformAndroid,
		VideoRefs: []*VideoReference{
			{
				VideoPath:   "/videos/android.mp4",
				Timestamp:   2*time.Minute + 30*time.Second,
				Description: "Navigation to crash",
			},
			{
				VideoPath:   "/videos/android.mp4",
				Timestamp:   3*time.Minute + 15*time.Second,
				Description: "Crash visible",
			},
		},
		CreatedAt: time.Now(),
	}
	assert.Len(t, ticket.VideoRefs, 2)
}

func TestTicket_WithSuggestedFix_Nil(t *testing.T) {
	ticket := &Ticket{
		ID:           "HQA-0001",
		Title:        "No fix",
		Severity:     SeverityLow,
		Platform:     config.PlatformWeb,
		SuggestedFix: nil,
		CreatedAt:    time.Now(),
	}
	assert.Nil(t, ticket.SuggestedFix)
}

func TestTicket_WithSuggestedFix_Present(t *testing.T) {
	ticket := &Ticket{
		ID:       "HQA-0001",
		Title:    "Issue with fix",
		Severity: SeverityHigh,
		Platform: config.PlatformDesktop,
		SuggestedFix: &LLMSuggestedFix{
			Description:   "Use wrap_content instead of fixed width",
			CodeSnippet:   "android:layout_width=\"wrap_content\"",
			Confidence:    0.92,
			AffectedFiles: []string{"res/layout/settings.xml"},
		},
		CreatedAt: time.Now(),
	}
	require.NotNil(t, ticket.SuggestedFix)
	assert.InDelta(t, 0.92, ticket.SuggestedFix.Confidence, 0.001)
}

func TestRenderMarkdown_WithVideoRefs(t *testing.T) {
	gen := New()
	ticket := &Ticket{
		ID:       "HQA-0042",
		Title:    "Button text truncated on settings",
		Severity: SeverityMedium,
		Platform: config.PlatformAndroid,
		VideoRefs: []*VideoReference{
			{
				VideoPath:   "/videos/android-session.mp4",
				Timestamp:   14*time.Minute + 32*time.Second,
				Description: "Navigating to settings",
			},
			{
				VideoPath:   "/videos/android-session.mp4",
				Timestamp:   14*time.Minute + 47*time.Second,
				Description: "Truncation visible",
			},
		},
		CreatedAt: time.Now(),
	}

	md := string(gen.RenderMarkdown(ticket))

	assert.Contains(t, md, "## Video References")
	assert.Contains(t, md, "android-session.mp4")
	assert.Contains(t, md, "14:32")
	assert.Contains(t, md, "Navigating to settings")
	assert.Contains(t, md, "14:47")
	assert.Contains(t, md, "Truncation visible")
}

func TestRenderMarkdown_WithSuggestedFix(t *testing.T) {
	gen := New()
	ticket := &Ticket{
		ID:       "HQA-0043",
		Title:    "OOM during image load",
		Severity: SeverityCritical,
		Platform: config.PlatformAndroid,
		SuggestedFix: &LLMSuggestedFix{
			Description: "Add memory limit check before decoding",
			CodeSnippet: "val options = BitmapFactory.Options()\n" +
				"options.inJustDecodeBounds = true",
			Confidence:    0.88,
			AffectedFiles: []string{"ImageDecoder.kt", "ImageCache.kt"},
		},
		CreatedAt: time.Now(),
	}

	md := string(gen.RenderMarkdown(ticket))

	assert.Contains(t, md, "## Suggested Fix")
	assert.Contains(t, md, "**Confidence:** 88%")
	assert.Contains(t, md, "memory limit check")
	assert.Contains(t, md, "BitmapFactory")
	assert.Contains(t, md, "**Affected files:**")
	assert.Contains(t, md, "ImageDecoder.kt")
	assert.Contains(t, md, "ImageCache.kt")
}

func TestRenderMarkdown_WithBothVideoAndFix(t *testing.T) {
	gen := New()
	ticket := &Ticket{
		ID:       "HQA-0044",
		Title:    "Full enhanced ticket",
		Severity: SeverityHigh,
		Platform: config.PlatformDesktop,
		VideoRefs: []*VideoReference{
			{
				VideoPath: "/videos/desktop.mp4",
				Timestamp: 5*time.Minute + 10*time.Second,
			},
		},
		SuggestedFix: &LLMSuggestedFix{
			Description: "Increase buffer size",
			Confidence:  0.75,
		},
		CreatedAt: time.Now(),
	}

	md := string(gen.RenderMarkdown(ticket))

	assert.Contains(t, md, "## Video References")
	assert.Contains(t, md, "5:10")
	assert.Contains(t, md, "## Suggested Fix")
	assert.Contains(t, md, "**Confidence:** 75%")
	assert.Contains(t, md, "Increase buffer size")
}

func TestRenderMarkdown_VideoRefNoDescription(t *testing.T) {
	gen := New()
	ticket := &Ticket{
		ID:       "HQA-0045",
		Title:    "Issue",
		Severity: SeverityLow,
		Platform: config.PlatformWeb,
		VideoRefs: []*VideoReference{
			{
				VideoPath: "/videos/web.mp4",
				Timestamp: 30 * time.Second,
			},
		},
		CreatedAt: time.Now(),
	}

	md := string(gen.RenderMarkdown(ticket))

	assert.Contains(t, md, "0:30")
	// No dash before empty description.
	assert.NotContains(t, md, "0:30 —")
}

func TestRenderMarkdown_SuggestedFixNoCode(t *testing.T) {
	gen := New()
	ticket := &Ticket{
		ID:       "HQA-0046",
		Title:    "Issue",
		Severity: SeverityMedium,
		Platform: config.PlatformAndroid,
		SuggestedFix: &LLMSuggestedFix{
			Description: "Refactor error handling",
			Confidence:  0.6,
		},
		CreatedAt: time.Now(),
	}

	md := string(gen.RenderMarkdown(ticket))

	assert.Contains(t, md, "## Suggested Fix")
	assert.Contains(t, md, "Refactor error handling")
	// No code block when snippet is empty.
	lineCount := 0
	for _, line := range splitLines(md) {
		if line == "```" {
			lineCount++
		}
	}
	assert.Equal(t, 0, lineCount,
		"should have no code blocks without snippet")
}

func TestRenderMarkdown_WithoutVideoOrFix(t *testing.T) {
	gen := New()
	ticket := &Ticket{
		ID:        "HQA-0047",
		Title:     "Plain ticket",
		Severity:  SeverityLow,
		Platform:  config.PlatformWeb,
		CreatedAt: time.Now(),
	}

	md := string(gen.RenderMarkdown(ticket))

	assert.NotContains(t, md, "## Video References")
	assert.NotContains(t, md, "## Suggested Fix")
	assert.Contains(t, md, "Generated by HelixQA")
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		d        time.Duration
		expected string
	}{
		{"zero", 0, "0:00"},
		{"seconds", 30 * time.Second, "0:30"},
		{"one_minute", time.Minute, "1:00"},
		{"minutes_seconds",
			14*time.Minute + 32*time.Second, "14:32"},
		{"large", time.Hour + 5*time.Minute + 3*time.Second,
			"65:03"},
		{"one_second", time.Second, "0:01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected,
				formatDuration(tt.d))
		})
	}
}

func TestMultipleVideoRefs_RenderOrder(t *testing.T) {
	gen := New()
	ticket := &Ticket{
		ID:       "HQA-0048",
		Title:    "Multiple refs",
		Severity: SeverityMedium,
		Platform: config.PlatformAndroid,
		VideoRefs: []*VideoReference{
			{VideoPath: "/a.mp4", Timestamp: time.Minute,
				Description: "First"},
			{VideoPath: "/b.mp4", Timestamp: 2 * time.Minute,
				Description: "Second"},
			{VideoPath: "/c.mp4", Timestamp: 3 * time.Minute,
				Description: "Third"},
		},
		CreatedAt: time.Now(),
	}

	md := string(gen.RenderMarkdown(ticket))

	// All three should appear.
	assert.Contains(t, md, "/a.mp4")
	assert.Contains(t, md, "/b.mp4")
	assert.Contains(t, md, "/c.mp4")
	assert.Contains(t, md, "First")
	assert.Contains(t, md, "Second")
	assert.Contains(t, md, "Third")
}

func TestSuggestedFix_MultipleAffectedFiles(t *testing.T) {
	gen := New()
	ticket := &Ticket{
		ID:       "HQA-0049",
		Title:    "Multi-file fix",
		Severity: SeverityHigh,
		Platform: config.PlatformDesktop,
		SuggestedFix: &LLMSuggestedFix{
			Description: "Fix across multiple files",
			Confidence:  0.9,
			AffectedFiles: []string{
				"src/main/FormatRegistry.kt",
				"src/main/TextParser.kt",
				"src/main/DocumentCache.kt",
				"src/test/FormatRegistryTest.kt",
			},
		},
		CreatedAt: time.Now(),
	}

	md := string(gen.RenderMarkdown(ticket))

	assert.Contains(t, md, "FormatRegistry.kt")
	assert.Contains(t, md, "TextParser.kt")
	assert.Contains(t, md, "DocumentCache.kt")
	assert.Contains(t, md, "FormatRegistryTest.kt")
}

// splitLines splits a string into lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
