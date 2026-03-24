// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package issuedetector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"digital.vasic.llmorchestrator/pkg/agent"
	"digital.vasic.visionengine/pkg/analyzer"
	"digital.vasic.visionengine/pkg/graph"

	"digital.vasic.helixqa/pkg/session"
)

// IssueDetector uses LLM agents to detect bugs during
// autonomous QA sessions. It analyzes screen transitions,
// navigation patterns, and accessibility properties.
type IssueDetector struct {
	agent    agent.Agent
	analyzer analyzer.Analyzer
	session  *session.SessionRecorder
	issues   []Issue
	counter  int
	mu       sync.Mutex
}

// NewIssueDetector creates an IssueDetector with all dependencies.
func NewIssueDetector(
	ag agent.Agent,
	az analyzer.Analyzer,
	sess *session.SessionRecorder,
) *IssueDetector {
	return &IssueDetector{
		agent:    ag,
		analyzer: az,
		session:  sess,
		issues:   make([]Issue, 0, 16),
	}
}

// AnalyzeAction compares before and after screen states to
// detect issues caused by an action.
func (id *IssueDetector) AnalyzeAction(
	ctx context.Context,
	before, after analyzer.ScreenAnalysis,
	action analyzer.Action,
) ([]Issue, error) {
	prompt := fmt.Sprintf(
		actionAnalysisPrompt,
		action.Type,
		before.Title,
		len(before.Elements),
		before.Description,
		len(after.Elements),
		after.Description,
	)

	resp, err := id.agent.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("analyze action: %w", err)
	}

	issues := id.parseIssues(resp.Content)
	for i := range issues {
		issues[i].Platform = id.platform()
		issues[i].ScreenID = after.ScreenID
	}

	id.recordIssues(issues)
	return issues, nil
}

// AnalyzeUX analyzes the navigation graph for UX issues.
func (id *IssueDetector) AnalyzeUX(
	ctx context.Context,
	navGraph graph.NavigationGraph,
) ([]Issue, error) {
	screens := navGraph.Screens()
	transitions := navGraph.Transitions()
	coverage := navGraph.Coverage()
	unvisited := navGraph.UnvisitedScreens()

	unvisitedStr := "none"
	if len(unvisited) > 0 {
		unvisitedStr = strings.Join(unvisited, ", ")
	}

	prompt := fmt.Sprintf(
		uxAnalysisPrompt,
		len(screens),
		len(transitions),
		coverage*100,
		unvisitedStr,
	)

	resp, err := id.agent.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("analyze ux: %w", err)
	}

	issues := id.parseIssues(resp.Content)
	for i := range issues {
		issues[i].Category = CategoryUX
		issues[i].Platform = id.platform()
	}

	id.recordIssues(issues)
	return issues, nil
}

// AnalyzeAccessibility analyzes a screen for accessibility issues.
func (id *IssueDetector) AnalyzeAccessibility(
	ctx context.Context,
	screen analyzer.ScreenAnalysis,
) ([]Issue, error) {
	clickable := 0
	var elemDetails strings.Builder
	for _, e := range screen.Elements {
		if e.Clickable {
			clickable++
		}
		fmt.Fprintf(&elemDetails,
			"- %s: %q (clickable=%v, confidence=%.2f)\n",
			e.Type, e.Label, e.Clickable, e.Confidence,
		)
	}

	prompt := fmt.Sprintf(
		accessibilityAnalysisPrompt,
		screen.Title,
		screen.ScreenID,
		len(screen.Elements),
		clickable,
		len(screen.TextRegions),
		elemDetails.String(),
	)

	resp, err := id.agent.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("analyze accessibility: %w", err)
	}

	issues := id.parseIssues(resp.Content)
	for i := range issues {
		issues[i].Category = CategoryAccessibility
		issues[i].Platform = id.platform()
		issues[i].ScreenID = screen.ScreenID
	}

	id.recordIssues(issues)
	return issues, nil
}

// RecordIssue records an issue in the session timeline.
func (id *IssueDetector) RecordIssue(issue Issue) {
	id.mu.Lock()
	defer id.mu.Unlock()

	if id.session != nil {
		id.session.RecordEvent(session.TimelineEvent{
			Type:        session.EventIssue,
			Platform:    issue.Platform,
			ScreenID:    issue.ScreenID,
			Description: fmt.Sprintf("Issue: %s", issue.Title),
			IssueID:     issue.ID,
		})
	}
}

// Issues returns a copy of all detected issues.
func (id *IssueDetector) Issues() []Issue {
	id.mu.Lock()
	defer id.mu.Unlock()

	result := make([]Issue, len(id.issues))
	copy(result, id.issues)
	return result
}

// IssueCount returns the total number of detected issues.
func (id *IssueDetector) IssueCount() int {
	id.mu.Lock()
	defer id.mu.Unlock()
	return len(id.issues)
}

// IssuesByCategory returns issues filtered by category.
func (id *IssueDetector) IssuesByCategory(
	cat IssueCategory,
) []Issue {
	id.mu.Lock()
	defer id.mu.Unlock()

	var result []Issue
	for _, iss := range id.issues {
		if iss.Category == cat {
			result = append(result, iss)
		}
	}
	return result
}

// IssuesBySeverity returns issues filtered by severity.
func (id *IssueDetector) IssuesBySeverity(
	sev IssueSeverity,
) []Issue {
	id.mu.Lock()
	defer id.mu.Unlock()

	var result []Issue
	for _, iss := range id.issues {
		if iss.Severity == sev {
			result = append(result, iss)
		}
	}
	return result
}

// parseIssues extracts issues from an LLM response string.
func (id *IssueDetector) parseIssues(content string) []Issue {
	// Try to find JSON array in the response.
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start < 0 || end < 0 || end <= start {
		return nil
	}

	jsonStr := content[start : end+1]
	var raw []struct {
		Category    string  `json:"category"`
		Severity    string  `json:"severity"`
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Suggestion  string  `json:"suggestion"`
		Confidence  float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil
	}

	// Allocate IDs under the lock to avoid races on counter.
	id.mu.Lock()
	issues := make([]Issue, 0, len(raw))
	for _, r := range raw {
		if r.Title == "" && r.Description == "" {
			continue
		}
		id.counter++
		issue := Issue{
			ID:          fmt.Sprintf("ISS-%04d", id.counter),
			Category:    IssueCategory(r.Category),
			Severity:    IssueSeverity(r.Severity),
			Title:       r.Title,
			Description: r.Description,
			Suggestion:  r.Suggestion,
			Confidence:  r.Confidence,
		}
		if !ValidCategory(string(issue.Category)) {
			issue.Category = CategoryFunctional
		}
		if !ValidSeverity(string(issue.Severity)) {
			issue.Severity = SeverityMedium
		}
		issues = append(issues, issue)
	}
	id.mu.Unlock()

	return issues
}

// recordIssues appends issues to the detector's internal list.
// Must NOT be called while mu is held.
func (id *IssueDetector) recordIssues(issues []Issue) {
	if len(issues) == 0 {
		return
	}
	id.mu.Lock()
	defer id.mu.Unlock()
	id.issues = append(id.issues, issues...)
}

// platform returns a platform string based on the session or
// a default.
func (id *IssueDetector) platform() string {
	if id.session != nil {
		platforms := id.session.VideoPlatforms()
		if len(platforms) > 0 {
			return platforms[0]
		}
	}
	return "unknown"
}
