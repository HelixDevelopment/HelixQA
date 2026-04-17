// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package planning

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/learning"
	"digital.vasic.helixqa/pkg/llm"
)

// systemPrompt instructs the LLM to generate comprehensive test cases
// as a JSON array of PlannedTest objects.
const systemPrompt = `You are an expert QA engineer. Your job is to generate
comprehensive test cases for the given project based on its screens, API
endpoints, and known issues.

Return ONLY a valid JSON array of test case objects. Each object must have:
- "id": string (unique, e.g. "GEN-001")
- "name": string (short, descriptive)
- "description": string (what this test validates)
- "category": string (one of: functional, edge_case, integration, security, performance)
- "priority": integer (1=critical, 2=high, 3=medium, 4=low)
- "platforms": array of strings (e.g. ["web", "android"])
- "screen": string (the screen or area under test)
- "steps": array of strings (ordered test steps)
- "expected": string (expected outcome)

IMPORTANT: For Android TV apps, you MUST include comprehensive test cases for:
- Android TV Home Screen Channels (default channel, category channels)
- Watch Next row integration (continue watching, next episode)
- Deep link handling from channels (use the project's own URI scheme, e.g. "<scheme>://media/{id}?type={type}")
- WorkManager periodic sync
- Channel cleanup on logout

Do not include any explanation, markdown, or text outside the JSON array.`

// TestPlanGenerator generates a TestPlan by querying an LLM with a
// structured prompt built from a KnowledgeBase.
type TestPlanGenerator struct {
	provider llm.Provider
}

// NewTestPlanGenerator returns a TestPlanGenerator backed by the given
// LLM provider.
func NewTestPlanGenerator(provider llm.Provider) *TestPlanGenerator {
	return &TestPlanGenerator{provider: provider}
}

// Generate builds a prompt from the KnowledgeBase, calls the LLM, and
// parses the response into a TestPlan. On parse failure it returns an
// empty plan (graceful degradation) — never an error for malformed LLM
// output. Only hard infrastructure errors (context cancelled, provider
// unreachable) are returned as errors.
func (g *TestPlanGenerator) Generate(
	ctx context.Context,
	kb *learning.KnowledgeBase,
	platforms []string,
) (*TestPlan, error) {
	// Use optimized prompt with reduced context for better compatibility
	// with providers that have token limits (GitHub Models, Groq, etc.)
	prompt := buildOptimizedPrompt(kb, platforms)

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: prompt},
	}

	resp, err := g.provider.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("planning: LLM chat failed: %w", err)
	}

	tests := g.parseTests(resp.Content)
	for i := range tests {
		tests[i].IsNew = true
	}

	// Inject mandatory Android TV Channels tests if platform is androidtv
	// and channels feature is detected in codebase
	if g.shouldInjectAndroidTVChannelsTests(platforms, kb) {
		channelSpec := g.buildChannelFeatureSpec(kb)
		tests = InjectAndroidTVChannelsTests(tests, platforms, channelSpec.AppName, channelSpec.DeepLink.Scheme)
	}

	newCount := 0
	for _, t := range tests {
		if t.IsNew {
			newCount++
		}
	}

	plan := &TestPlan{
		SessionID:     fmt.Sprintf("session-%d", time.Now().UnixNano()),
		Generated:     time.Now().UTC().Format(time.RFC3339),
		TotalTests:    len(tests),
		ExistingTests: 0,
		NewTests:      newCount,
		Platforms:     platforms,
		Tests:         tests,
	}

	return plan, nil
}

// shouldInjectAndroidTVChannelsTests checks if we should inject channel tests
func (g *TestPlanGenerator) shouldInjectAndroidTVChannelsTests(platforms []string, kb *learning.KnowledgeBase) bool {
	if !HasAndroidTVChannelsSupport(platforms) {
		return false
	}
	
	// Check if channels feature is detected in codebase
	for _, f := range kb.PlatformFeatures {
		if f.Name == "androidtv_channels" && f.Platform == "androidtv" {
			return true
		}
	}
	
	return false
}

// buildChannelFeatureSpec builds a ChannelFeatureSpec from detected features
func (g *TestPlanGenerator) buildChannelFeatureSpec(kb *learning.KnowledgeBase) ChannelFeatureSpec {
	spec := DefaultChannelFeatureSpec(kb.ProjectName, "", strings.ToLower(kb.ProjectName))
	
	// Override with detected metadata from codebase
	for _, f := range kb.PlatformFeatures {
		if f.Name == "androidtv_channels" {
			if scheme, ok := f.Metadata["uri_scheme"]; ok && scheme != "" {
				spec.DeepLink.Scheme = scheme
			}
			if name, ok := f.Metadata["default_channel"]; ok && name != "" {
				spec.DefaultChannelName = name
			}
			if f.Metadata["has_watch_next"] == "true" {
				spec.WatchNext.Enabled = true
			}
			if f.Metadata["has_deep_linking"] == "true" {
				spec.DeepLink.SupportsUnauthenticated = false
			}
		}
	}
	
	return spec
}

// buildPrompt constructs a human-readable prompt that includes the
// project summary, target platforms, screens, API endpoints, and known
// issues discovered by the knowledge base.
func (g *TestPlanGenerator) buildPrompt(
	kb *learning.KnowledgeBase,
	platforms []string,
) string {
	var sb strings.Builder

	sb.WriteString("Project: ")
	sb.WriteString(kb.ProjectName)
	sb.WriteString("\n")

	sb.WriteString("Target platforms: ")
	sb.WriteString(strings.Join(platforms, ", "))
	sb.WriteString("\n\n")

	sb.WriteString("Summary: ")
	sb.WriteString(kb.Summary())
	sb.WriteString("\n\n")

	if len(kb.Screens) > 0 {
		sb.WriteString("Screens:\n")
		for _, s := range kb.Screens {
			sb.WriteString(fmt.Sprintf(
				"  - %s (%s) route=%s\n",
				s.Name, s.Platform, s.Route,
			))
		}
		sb.WriteString("\n")
	}

	if len(kb.APIEndpoints) > 0 {
		sb.WriteString("API Endpoints:\n")
		for _, ep := range kb.APIEndpoints {
			sb.WriteString(fmt.Sprintf(
				"  - %s %s (handler: %s)\n",
				ep.Method, ep.Path, ep.Handler,
			))
		}
		sb.WriteString("\n")
	}

	if len(kb.KnownIssues) > 0 {
		sb.WriteString("Known Issues:\n")
		for _, issue := range kb.KnownIssues {
			sb.WriteString(fmt.Sprintf("  - %s\n", issue))
		}
		sb.WriteString("\n")
	}

	// Add Android TV Channels feature info if detected
	for _, f := range kb.PlatformFeatures {
		if f.Name == "androidtv_channels" {
			sb.WriteString("\n--- Android TV Channels Feature Detected ---\n")
			sb.WriteString(fmt.Sprintf("Feature: %s\n", f.Description))
			sb.WriteString(fmt.Sprintf("Implementation files: %d\n", len(f.SourceFiles)))
			for k, v := range f.Metadata {
				sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
			}
			sb.WriteString("\nREQUIRED: Include comprehensive Android TV Channels test cases covering:\n")
			sb.WriteString("- Default channel creation and content\n")
			sb.WriteString("- Category channels (movies, tv shows, etc.)\n")
			sb.WriteString("- Watch Next row (continue watching, next episode)\n")
			sb.WriteString("- Deep link handling from home screen\n")
			sb.WriteString("- Channel sync (WorkManager, launch, manual)\n")
			sb.WriteString("- Cleanup on logout\n")
		}
	}

	sb.WriteString(
		"\nGenerate a comprehensive set of test cases covering " +
			"functional correctness, edge cases, and integration " +
			"scenarios for these platforms and screens.\n",
	)

	return sb.String()
}

// parseTests extracts a JSON array of PlannedTest from the LLM response.
// It handles responses wrapped in markdown code fences, deduplicates tests
// by name (case-insensitive), and returns an empty slice (not nil) on any
// parse failure.
func (g *TestPlanGenerator) parseTests(content string) []PlannedTest {
	content = strings.TrimSpace(content)

	// Strip markdown code fences if present.
	if idx := strings.Index(content, "```"); idx != -1 {
		// Find the end of the opening fence line.
		start := strings.Index(content[idx:], "\n")
		if start != -1 {
			content = content[idx+start+1:]
		}
		// Strip trailing fence.
		if end := strings.LastIndex(content, "```"); end != -1 {
			content = content[:end]
		}
		content = strings.TrimSpace(content)
	}

	// Extract the outermost JSON array.
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start == -1 || end == -1 || end < start {
		return []PlannedTest{}
	}
	content = content[start : end+1]

	var tests []PlannedTest
	if err := json.Unmarshal([]byte(content), &tests); err != nil {
		return []PlannedTest{}
	}

	// DEDUPLICATION: Ensure same test name never appears twice.
	// Case-insensitive matching to catch variations like "Login Test" vs "login test".
	seen := make(map[string]bool)
	unique := make([]PlannedTest, 0, len(tests))
	duplicates := 0
	
	for _, t := range tests {
		key := strings.ToLower(strings.TrimSpace(t.Name))
		if key == "" {
			continue // Skip tests with empty names
		}
		if seen[key] {
			duplicates++
			continue // Skip duplicate
		}
		seen[key] = true
		unique = append(unique, t)
	}
	
	if duplicates > 0 {
		fmt.Printf("  [planner] deduplicated %d duplicate test(s)\n", duplicates)
	}

	return unique
}

// buildOptimizedPrompt creates a compact prompt optimized for providers
// with strict token limits (GitHub Models 8k, Groq 12k, etc.)
func buildOptimizedPrompt(kb *learning.KnowledgeBase, platforms []string) string {
	var sb strings.Builder

	sb.WriteString("Project: ")
	sb.WriteString(kb.ProjectName)
	sb.WriteString("\n")

	sb.WriteString("Platforms: ")
	sb.WriteString(strings.Join(platforms, ", "))
	sb.WriteString("\n\n")

	// Compact summary instead of full details
	sb.WriteString(fmt.Sprintf("Screens: %d | Endpoints: %d | Issues: %d\n",
		len(kb.Screens), len(kb.APIEndpoints), len(kb.KnownIssues)))

	// Add only first 10 screens (most important)
	if len(kb.Screens) > 0 {
		sb.WriteString("\nKey Screens:\n")
		limit := 10
		if len(kb.Screens) < limit {
			limit = len(kb.Screens)
		}
		for i := 0; i < limit; i++ {
			s := kb.Screens[i]
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", s.Name, s.Platform))
		}
		if len(kb.Screens) > limit {
			sb.WriteString(fmt.Sprintf("... and %d more\n", len(kb.Screens)-limit))
		}
	}

	// Add Android TV Channels feature if detected
	for _, f := range kb.PlatformFeatures {
		if f.Name == "androidtv_channels" {
			sb.WriteString("\n[Android TV Channels Detected]\n")
			sb.WriteString("REQUIRED: Test default channel, category channels, Watch Next, deep links, sync\n")
			break
		}
	}

	sb.WriteString("\nGenerate test cases covering functional, edge, integration scenarios.")

	return sb.String()
}
