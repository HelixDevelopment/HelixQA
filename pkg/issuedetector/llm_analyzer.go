// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package issuedetector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"digital.vasic.llmorchestrator/pkg/agent"
)

// LLMIssueAnalyzer uses LLM to classify and analyze issues
type LLMIssueAnalyzer struct {
	agent   agent.Agent
	prompts *PromptTemplates
}

// PromptTemplates contains LLM prompts for issue analysis
type PromptTemplates struct {
	AnalysisPrompt string
	UXPrompt       string
	VisualPrompt   string
	FixPrompt      string
}

// DefaultPrompts returns default prompt templates
func DefaultPrompts() *PromptTemplates {
	return &PromptTemplates{
		AnalysisPrompt: `Analyze this QA test result and identify any issues.

Before State: %s
After State: %s
Action Performed: %s
Platform: %s

Look for:
1. Visual bugs (truncation, overlap, misalignment)
2. UX issues (confusing UI, missing feedback)
3. Functional bugs (unexpected behavior)
4. Performance issues (slow response)
5. Crashes or errors

Return a JSON array of issues found (empty if none).
MANDATORY: Every issue MUST include specific, measurable acceptance_criteria describing exactly how a fix will be validated.

[{
  "category": "visual|ux|accessibility|functional|performance|crash",
  "severity": "critical|high|medium|low",
  "title": "brief issue title",
  "description": "detailed description",
  "acceptance_criteria": "specific, measurable criteria to verify the fix",
  "confidence": 0.0-1.0,
  "suggestion": "recommended fix"
}]`,
		UXPrompt: `Evaluate the UX of this screen:

Screen: %s
Elements: %v
User Flow: %s

Identify UX issues such as:
- Confusing navigation
- Missing feedback
- Unclear labels
- Poor information hierarchy
- Accessibility problems

Return UX issues as JSON array.
MANDATORY: Every issue MUST include specific, measurable acceptance_criteria describing exactly how a fix will be validated.`,
		VisualPrompt: `Analyze this screenshot for visual bugs:

Description: %s
Expected UI: %s

Look for:
- Text truncation
- Element overlap
- Misalignment
- Color/contrast issues
- Responsive layout problems

Return visual issues as JSON array.
MANDATORY: Every issue MUST include specific, measurable acceptance_criteria describing exactly how a fix will be validated.`,
		FixPrompt: `Suggest a fix for this issue:

Title: %s
Description: %s
Category: %s
Platform: %s

Provide specific, actionable fix recommendations.`,
	}
}

// NewLLMIssueAnalyzer creates a new LLM issue analyzer
func NewLLMIssueAnalyzer(ag agent.Agent) *LLMIssueAnalyzer {
	return &LLMIssueAnalyzer{
		agent:   ag,
		prompts: DefaultPrompts(),
	}
}

// AnalyzeAction analyzes a user action for issues
func (a *LLMIssueAnalyzer) AnalyzeAction(
	ctx context.Context,
	before, after []byte,
	action, platform string,
) ([]Issue, error) {
	prompt := fmt.Sprintf(
		a.prompts.AnalysisPrompt,
		string(before),
		string(after),
		action,
		platform,
	)

	resp, err := a.agent.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM analysis: %w", err)
	}

	var issues []Issue
	if err := json.Unmarshal([]byte(resp.Content), &issues); err != nil {
		// If JSON parsing fails, create a single issue with the raw response
		return []Issue{{
			ID:          generateIssueID(),
			Category:    CategoryFunctional,
			Severity:    SeverityMedium,
			Title:       "Potential Issue Detected",
			Description: resp.Content,
			Platform:    platform,
			Confidence:  0.5,
			Timestamp:   time.Now(),
		}}, nil
	}

	// Add IDs and timestamps
	for i := range issues {
		if issues[i].ID == "" {
			issues[i].ID = generateIssueID()
		}
		issues[i].Timestamp = time.Now()
		issues[i].Platform = platform
	}

	return issues, nil
}

// AnalyzeScreen analyzes a screen for UX issues
func (a *LLMIssueAnalyzer) AnalyzeScreen(
	ctx context.Context,
	screenID string,
	elements []string,
	userFlow string,
	platform string,
) ([]Issue, error) {
	prompt := fmt.Sprintf(
		a.prompts.UXPrompt,
		screenID,
		elements,
		userFlow,
	)

	resp, err := a.agent.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("UX analysis: %w", err)
	}

	var issues []Issue
	if err := json.Unmarshal([]byte(resp.Content), &issues); err != nil {
		return nil, fmt.Errorf("parse UX issues: %w", err)
	}

	for i := range issues {
		if issues[i].ID == "" {
			issues[i].ID = generateIssueID()
		}
		issues[i].Timestamp = time.Now()
		issues[i].Platform = platform
		issues[i].ScreenID = screenID
	}

	return issues, nil
}

// AnalyzeVisual analyzes screenshots for visual bugs
func (a *LLMIssueAnalyzer) AnalyzeVisual(
	ctx context.Context,
	description string,
	expectedUI string,
	platform string,
) ([]Issue, error) {
	prompt := fmt.Sprintf(
		a.prompts.VisualPrompt,
		description,
		expectedUI,
	)

	resp, err := a.agent.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("visual analysis: %w", err)
	}

	var issues []Issue
	if err := json.Unmarshal([]byte(resp.Content), &issues); err != nil {
		return nil, fmt.Errorf("parse visual issues: %w", err)
	}

	for i := range issues {
		if issues[i].ID == "" {
			issues[i].ID = generateIssueID()
		}
		issues[i].Timestamp = time.Now()
		issues[i].Platform = platform
	}

	return issues, nil
}

// SuggestFix generates a fix suggestion for an issue
func (a *LLMIssueAnalyzer) SuggestFix(ctx context.Context, issue Issue) (string, error) {
	prompt := fmt.Sprintf(
		a.prompts.FixPrompt,
		issue.Title,
		issue.Description,
		issue.Category,
		issue.Platform,
	)

	resp, err := a.agent.Send(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("generate fix: %w", err)
	}

	return resp.Content, nil
}

// BatchAnalyze analyzes multiple actions/screens in batch
func (a *LLMIssueAnalyzer) BatchAnalyze(
	ctx context.Context,
	items []AnalysisItem,
) (map[string][]Issue, error) {
	results := make(map[string][]Issue)

	for _, item := range items {
		var issues []Issue
		var err error

		switch item.Type {
		case "action":
			issues, err = a.AnalyzeAction(ctx, item.Before, item.After, item.Action, item.Platform)
		case "screen":
			issues, err = a.AnalyzeScreen(ctx, item.ScreenID, item.Elements, item.UserFlow, item.Platform)
		case "visual":
			issues, err = a.AnalyzeVisual(ctx, item.Description, item.ExpectedUI, item.Platform)
		}

		if err != nil {
			results[item.ID] = []Issue{{
				ID:          generateIssueID(),
				Category:    CategoryFunctional,
				Severity:    SeverityMedium,
				Title:       "Analysis Error",
				Description: fmt.Sprintf("Failed to analyze %s: %v", item.ID, err),
				Platform:    item.Platform,
				Confidence:  0.0,
			}}
			continue
		}

		results[item.ID] = issues
	}

	return results, nil
}

// AnalysisItem represents an item to be analyzed
type AnalysisItem struct {
	ID          string
	Type        string // action, screen, visual
	Platform    string
	Before      []byte
	After       []byte
	Action      string
	ScreenID    string
	Elements    []string
	UserFlow    string
	Description string
	ExpectedUI  string
}

// generateIssueID generates a unique issue ID
func generateIssueID() string {
	return fmt.Sprintf("HQA-%d", time.Now().UnixNano())
}
