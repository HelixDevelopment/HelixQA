// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.llmorchestrator/pkg/agent"
)

// IssueData represents issue data for ticket generation
// This avoids import cycle with issuedetector package
type IssueData struct {
	ID          string
	Title       string
	Description string
	Category    string
	Severity    string
	Platform    string
	ScreenID    string
	Confidence  float64
}

// EnhancedGenerator creates comprehensive tickets with LLM analysis
type EnhancedGenerator struct {
	*Generator
	agent       agent.Agent
	sessionInfo *SessionInfo
	templates   *TicketTemplates
}

// SessionInfo holds context about the testing session
type SessionInfo struct {
	SessionID  string
	StartTime  time.Time
	Platforms  []string
	Features   []string
	Coverage   float64
	VideoPaths map[string]string
}

// TicketTemplates contains templates for ticket generation
type TicketTemplates struct {
	TitleTemplate       string
	DescriptionTemplate string
	StepsTemplate       string
	ExpectedTemplate    string
}

// DefaultTemplates returns default ticket templates
func DefaultTemplates() *TicketTemplates {
	return &TicketTemplates{
		TitleTemplate:       "[%s] %s on %s",
		DescriptionTemplate: "Issue detected during autonomous QA session %s on platform %s.",
		StepsTemplate:       "1. Navigate to %s\n2. %s\n3. Observe issue",
		ExpectedTemplate:    "%s should function correctly without errors.",
	}
}

// NewEnhancedGenerator creates an enhanced ticket generator
func NewEnhancedGenerator(agent agent.Agent, opts ...Option) *EnhancedGenerator {
	return &EnhancedGenerator{
		Generator:   New(opts...),
		agent:       agent,
		templates:   DefaultTemplates(),
		sessionInfo: &SessionInfo{},
	}
}

// SetSessionInfo sets the session context
func (g *EnhancedGenerator) SetSessionInfo(info *SessionInfo) {
	g.sessionInfo = info
}

// GenerateFromIssueData creates a comprehensive ticket from issue data
func (g *EnhancedGenerator) GenerateFromIssueData(
	ctx context.Context,
	issue IssueData,
	screenshots []string,
	videoTimestamp time.Duration,
) (*Ticket, error) {
	g.counter++

	ticket := &Ticket{
		ID:               fmt.Sprintf("HQA-%04d", g.counter),
		Title:            g.generateTitle(issue),
		Severity:         Severity(issue.Severity),
		Platform:         config.Platform(issue.Platform),
		CreatedAt:        time.Now(),
		Description:      issue.Description,
		Screenshots:      screenshots,
		Labels:           []string{issue.Category, issue.Platform},
		ExpectedBehavior: g.generateExpected(issue),
		ActualBehavior:   issue.Description,
	}

	// Generate steps to reproduce using LLM if available
	if g.agent != nil {
		steps, err := g.generateSteps(ctx, issue)
		if err == nil {
			ticket.StepsToReproduce = steps
		}

		// Get LLM suggested fix
		fix, err := g.generateFix(ctx, issue)
		if err == nil {
			ticket.SuggestedFix = fix
		}
	}

	// Add video reference if available
	if g.sessionInfo != nil && g.sessionInfo.VideoPaths != nil {
		if videoPath, ok := g.sessionInfo.VideoPaths[issue.Platform]; ok {
			ticket.VideoRefs = append(ticket.VideoRefs, &VideoReference{
				VideoPath:   videoPath,
				Timestamp:   videoTimestamp,
				Description: fmt.Sprintf("Issue: %s", issue.Title),
			})
		}
	}

	return ticket, nil
}

// GenerateBatch creates tickets from multiple issues
func (g *EnhancedGenerator) GenerateBatch(
	ctx context.Context,
	issues []IssueData,
	screenshots map[string][]string,
	videoOffsets map[string]time.Duration,
) ([]*Ticket, error) {
	tickets := make([]*Ticket, 0, len(issues))

	for _, issue := range issues {
		screens := screenshots[issue.ID]
		if screens == nil {
			screens = []string{}
		}

		offset := videoOffsets[issue.ID]

		ticket, err := g.GenerateFromIssueData(ctx, issue, screens, offset)
		if err != nil {
			continue // Skip failed tickets
		}

		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

// generateTitle creates a ticket title from an issue
func (g *EnhancedGenerator) generateTitle(issue IssueData) string {
	return fmt.Sprintf(
		g.templates.TitleTemplate,
		strings.ToUpper(issue.Severity),
		issue.Title,
		issue.Platform,
	)
}

// generateExpected creates expected behavior text
func (g *EnhancedGenerator) generateExpected(issue IssueData) string {
	return fmt.Sprintf(g.templates.ExpectedTemplate, issue.Title)
}

// generateSteps uses LLM to generate reproduction steps
func (g *EnhancedGenerator) generateSteps(
	ctx context.Context,
	issue IssueData,
) ([]string, error) {
	if g.agent == nil {
		return []string{
			fmt.Sprintf("Navigate to %s", issue.ScreenID),
			"Perform relevant actions",
			"Observe the issue",
		}, nil
	}

	prompt := fmt.Sprintf(`
Generate clear reproduction steps for this issue:

Title: %s
Description: %s
Category: %s
Platform: %s
Screen: %s

Provide 3-5 numbered steps to reproduce this issue.
`, issue.Title, issue.Description, issue.Category, issue.Platform, issue.ScreenID)

	resp, err := g.agent.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Parse steps from response (assuming numbered list)
	lines := strings.Split(resp.Content, "\n")
	steps := make([]string, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && (strings.HasPrefix(line, "1.") ||
			strings.HasPrefix(line, "2.") ||
			strings.HasPrefix(line, "3.") ||
			strings.HasPrefix(line, "4.") ||
			strings.HasPrefix(line, "5.")) {
			// Remove the number prefix
			parts := strings.SplitN(line, ".", 2)
			if len(parts) > 1 {
				steps = append(steps, strings.TrimSpace(parts[1]))
			}
		}
	}

	if len(steps) == 0 {
		// Fallback if parsing fails
		steps = []string{resp.Content}
	}

	return steps, nil
}

// generateFix uses LLM to suggest a fix
func (g *EnhancedGenerator) generateFix(
	ctx context.Context,
	issue IssueData,
) (*LLMSuggestedFix, error) {
	if g.agent == nil {
		return nil, fmt.Errorf("no agent available")
	}

	prompt := fmt.Sprintf(`
Analyze this issue and suggest a fix:

Title: %s
Description: %s
Category: %s
Platform: %s

Provide:
1. A clear description of the fix
2. Confidence level (0.0-1.0)
3. Any affected files (if applicable)

Format as JSON:
{
  "description": "fix description",
  "confidence": 0.8,
  "affected_files": ["file1.go", "file2.go"]
}
`, issue.Title, issue.Description, issue.Category, issue.Platform)

	resp, err := g.agent.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Try to parse JSON response
	var fix LLMSuggestedFix
	if err := json.Unmarshal([]byte(resp.Content), &fix); err != nil {
		// Fallback: use entire response as description
		fix = LLMSuggestedFix{
			Description: resp.Content,
			Confidence:  0.5,
		}
	}

	return &fix, nil
}

// WriteTicketWithMetadata writes a ticket with additional metadata
func (g *EnhancedGenerator) WriteTicketWithMetadata(
	ticket *Ticket,
	metadata map[string]string,
) (string, error) {
	// Add metadata as labels
	for key, value := range metadata {
		ticket.Labels = append(ticket.Labels, fmt.Sprintf("%s:%s", key, value))
	}

	return g.WriteTicket(ticket)
}
