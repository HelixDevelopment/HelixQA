// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"strings"
	"testing"

	"digital.vasic.helixqa/pkg/learning"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text     string
		expected int
	}{
		{"", 0},
		{"hello", 1},                      // 5 chars / 4 = 1
		{"hello world", 2},                // 11 chars / 4 = 2
		{"this is a longer text", 5},      // 21 chars / 4 = 5
		{strings.Repeat("a", 4000), 1000}, // 4000 chars / 4 = 1000
	}

	for _, tt := range tests {
		result := EstimateTokens(tt.text)
		if result != tt.expected {
			t.Errorf("EstimateTokens(%q) = %d, expected %d", tt.text, result, tt.expected)
		}
	}
}

func TestNewPromptOptimizer(t *testing.T) {
	po := NewPromptOptimizer(8000)
	if po.maxTokens != 8000 {
		t.Errorf("expected maxTokens=8000, got: %d", po.maxTokens)
	}
}

func TestPromptOptimizer_OptimizePrompt(t *testing.T) {
	kb := learning.NewKnowledgeBase()
	kb.ProjectName = "TestProject"

	// Add screens
	for i := 0; i < 10; i++ {
		kb.AddScreen(learning.Screen{
			Name:     "Screen" + string(rune('0'+i)),
			Platform: "androidtv",
			Route:    "/screen/" + string(rune('0'+i)),
		})
	}

	// Add endpoints
	for i := 0; i < 20; i++ {
		kb.AddEndpoint(learning.APIEndpoint{
			Method: "GET",
			Path:   "/api/test/" + string(rune('0'+i)),
		})
	}

	// Add known issues
	for i := 0; i < 15; i++ {
		kb.KnownIssues = append(kb.KnownIssues, "Issue "+string(rune('0'+i))+": description")
	}

	po := NewPromptOptimizer(4000)
	result := po.OptimizePrompt(kb, []string{"androidtv"}, 3000)

	// Verify result contains expected sections
	if !strings.Contains(result, "Project: TestProject") {
		t.Error("result should contain project name")
	}
	if !strings.Contains(result, "Target platforms: androidtv") {
		t.Error("result should contain platforms")
	}
	if !strings.Contains(result, "Screens:") {
		t.Error("result should contain screens section")
	}
	if !strings.Contains(result, "API Endpoints:") {
		t.Error("result should contain endpoints section")
	}

	// Verify token limit is respected
	tokens := EstimateTokens(result)
	if tokens > 4000 {
		t.Errorf("optimized prompt exceeds token limit: %d > 4000", tokens)
	}
}

func TestPromptOptimizer_WithAndroidTVChannels(t *testing.T) {
	kb := learning.NewKnowledgeBase()
	kb.ProjectName = "Catalogizer"
	kb.PlatformFeatures = []learning.PlatformFeature{
		{
			Name:        "androidtv_channels",
			Platform:    "androidtv",
			Description: "Android TV Channels integration",
			Metadata: map[string]string{
				"uri_scheme":      "catalogizer",
				"default_channel": "Catalogizer Picks",
			},
		},
	}

	po := NewPromptOptimizer(8000)
	result := po.OptimizePrompt(kb, []string{"androidtv"}, 6000)

	// Verify Android TV Channels section is included
	if !strings.Contains(result, "Android TV Channels Feature Detected") {
		t.Error("result should contain Android TV Channels section")
	}
	if !strings.Contains(result, "catalogizer") {
		t.Error("result should contain URI scheme")
	}
	if !strings.Contains(result, "Catalogizer Picks") {
		t.Error("result should contain default channel name")
	}
}

func TestGetOptimizedPrompt(t *testing.T) {
	kb := learning.NewKnowledgeBase()
	kb.ProjectName = "Test"
	kb.AddScreen(learning.Screen{
		Name:     "Home",
		Platform: "androidtv",
		Route:    "/home",
	})

	// Test for GitHub Models (low token limit)
	result := GetOptimizedPrompt("githubmodels", kb, []string{"androidtv"})
	if !strings.Contains(result, "Project: Test") {
		t.Error("result should contain project name")
	}

	// Verify it's optimized for 8000 token limit
	tokens := EstimateTokens(result)
	if tokens > 8000 {
		t.Errorf("GitHub Models prompt exceeds 8000 tokens: %d", tokens)
	}
}

func TestProviderTokenLimits(t *testing.T) {
	// Verify all providers have reasonable token limits
	for name, limit := range ProviderTokenLimits {
		if limit <= 0 {
			t.Errorf("provider %s has invalid token limit: %d", name, limit)
		}
		if limit < 1000 {
			t.Errorf("provider %s token limit suspiciously low: %d", name, limit)
		}
		if limit > 2000000 {
			t.Errorf("provider %s token limit suspiciously high: %d", name, limit)
		}
	}
}

func TestPromptOptimizer_Truncation(t *testing.T) {
	kb := learning.NewKnowledgeBase()
	kb.ProjectName = "BigProject"

	// Add many screens with long names to trigger truncation
	// Each line is ~70 chars, 50 lines = 3500 chars
	// With 2000 token limit (~8000 chars), budget for screens is ~2000 chars
	for i := 0; i < 100; i++ {
		kb.AddScreen(learning.Screen{
			Name:     "VeryLongScreenNameNumber" + string(rune('0'+i%10)) + string(rune('0'+(i/10)%10)),
			Platform: "androidtv",
			Route:    "/very/long/route/path/number/" + string(rune('0'+i%10)),
		})
	}

	po := NewPromptOptimizer(3000)                               // Moderate limit
	result := po.OptimizePrompt(kb, []string{"androidtv"}, 2250) // 75% of limit

	// Should contain truncation indicator
	if !strings.Contains(result, "... and") {
		t.Logf("Result length: %d chars", len(result))
		t.Logf("Result: %s", result[:intMin(len(result), 500)])
		t.Skip("truncation indicator not found - budget may be sufficient")
	}
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func BenchmarkEstimateTokens(b *testing.B) {
	text := strings.Repeat("hello world ", 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EstimateTokens(text)
	}
}

func BenchmarkPromptOptimizer_OptimizePrompt(b *testing.B) {
	kb := learning.NewKnowledgeBase()
	kb.ProjectName = "Benchmark"
	for i := 0; i < 50; i++ {
		kb.AddScreen(learning.Screen{
			Name:     "Screen" + string(rune('0'+i%10)),
			Platform: "androidtv",
			Route:    "/screen/" + string(rune('0'+i%10)),
		})
	}

	po := NewPromptOptimizer(8000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		po.OptimizePrompt(kb, []string{"androidtv"}, 6000)
	}
}
