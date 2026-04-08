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
	prompt := g.buildPrompt(kb, platforms)

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

	sb.WriteString(
		"Generate a comprehensive set of test cases covering " +
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
