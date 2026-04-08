// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"fmt"
	"strings"

	"digital.vasic.helixqa/pkg/learning"
)

// PromptOptimizer reduces prompt size for providers with token limits
type PromptOptimizer struct {
	maxTokens int
}

// NewPromptOptimizer creates a prompt optimizer with a token limit
func NewPromptOptimizer(maxTokens int) *PromptOptimizer {
	return &PromptOptimizer{
		maxTokens: maxTokens,
	}
}

// ProviderTokenLimits defines maximum token limits for providers
var ProviderTokenLimits = map[string]int{
	"google":       1000000, // Gemini has very high limits
	"anthropic":    200000,
	"openai":       128000,
	"githubmodels": 8000, // GitHub Models has strict 8k limit
	"groq":         12000,
	"astica":       8000,
	"nvidia":       16000,
	"mistral":      32000,
	"ollama":       128000,
}

// EstimateTokens estimates token count from text (1 token ≈ 4 chars)
func EstimateTokens(text string) int {
	return len(text) / 4
}

// OptimizePrompt reduces KnowledgeBase context to fit within token limits
func (po *PromptOptimizer) OptimizePrompt(kb *learning.KnowledgeBase, platforms []string, targetTokens int) string {
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

	// Calculate remaining budget
	currentEstimate := EstimateTokens(sb.String())
	remainingBudget := targetTokens - currentEstimate - 1000 // Reserve 1k for response

	// Add screens (highest priority)
	if len(kb.Screens) > 0 && remainingBudget > 500 {
		screenText := po.formatScreens(kb.Screens, remainingBudget/4)
		sb.WriteString(screenText)
		remainingBudget -= EstimateTokens(screenText)
	}

	// Add API endpoints
	if len(kb.APIEndpoints) > 0 && remainingBudget > 500 {
		endpointText := po.formatEndpoints(kb.APIEndpoints, remainingBudget/4)
		sb.WriteString(endpointText)
		remainingBudget -= EstimateTokens(endpointText)
	}

	// Add known issues (important for testing)
	if len(kb.KnownIssues) > 0 && remainingBudget > 300 {
		issueText := po.formatIssues(kb.KnownIssues, remainingBudget/4)
		sb.WriteString(issueText)
		remainingBudget -= EstimateTokens(issueText)
	}

	// Add platform features (Android TV Channels, etc.)
	if len(kb.PlatformFeatures) > 0 && remainingBudget > 300 {
		featureText := po.formatPlatformFeatures(kb.PlatformFeatures, remainingBudget/4)
		sb.WriteString(featureText)
	}

	sb.WriteString(
		"\nGenerate a comprehensive set of test cases covering " +
			"functional correctness, edge cases, and integration scenarios.\n",
	)

	return sb.String()
}

// formatScreens formats screens with budget limit
func (po *PromptOptimizer) formatScreens(screens []learning.Screen, charBudget int) string {
	var sb strings.Builder
	sb.WriteString("Screens:\n")

	charCount := 0
	for _, s := range screens {
		line := fmt.Sprintf("  - %s (%s) route=%s\n", s.Name, s.Platform, s.Route)
		if charCount+len(line) > charBudget {
			sb.WriteString(fmt.Sprintf("  ... and %d more screens\n", len(screens)-charCount/50))
			break
		}
		sb.WriteString(line)
		charCount += len(line)
	}
	sb.WriteString("\n")
	return sb.String()
}

// formatEndpoints formats API endpoints with budget limit
func (po *PromptOptimizer) formatEndpoints(endpoints []learning.APIEndpoint, charBudget int) string {
	var sb strings.Builder
	sb.WriteString("API Endpoints:\n")

	charCount := 0
	for _, ep := range endpoints {
		line := fmt.Sprintf("  - %s %s\n", ep.Method, ep.Path)
		if charCount+len(line) > charBudget {
			sb.WriteString(fmt.Sprintf("  ... and %d more endpoints\n", len(endpoints)-charCount/30))
			break
		}
		sb.WriteString(line)
		charCount += len(line)
	}
	sb.WriteString("\n")
	return sb.String()
}

// formatIssues formats known issues with budget limit
func (po *PromptOptimizer) formatIssues(issues []string, charBudget int) string {
	var sb strings.Builder
	sb.WriteString("Known Issues:\n")

	charCount := 0
	for _, issue := range issues {
		line := fmt.Sprintf("  - %s\n", issue)
		if charCount+len(line) > charBudget {
			sb.WriteString(fmt.Sprintf("  ... and %d more issues\n", len(issues)-charCount/50))
			break
		}
		sb.WriteString(line)
		charCount += len(line)
	}
	sb.WriteString("\n")
	return sb.String()
}

// formatPlatformFeatures formats platform features with budget limit
func (po *PromptOptimizer) formatPlatformFeatures(features []learning.PlatformFeature, charBudget int) string {
	var sb strings.Builder

	charCount := 0
	for _, f := range features {
		if f.Name == "androidtv_channels" {
			line := fmt.Sprintf("\n--- Android TV Channels Feature Detected ---\n"+
				"Feature: %s\n", f.Description)
			if charCount+len(line) > charBudget {
				break
			}
			sb.WriteString(line)
			charCount += len(line)

			// Add metadata
			for k, v := range f.Metadata {
				metaLine := fmt.Sprintf("  %s: %s\n", k, v)
				if charCount+len(metaLine) > charBudget {
					break
				}
				sb.WriteString(metaLine)
				charCount += len(metaLine)
			}

			// Add required tests section
			sb.WriteString("\nREQUIRED: Include comprehensive Android TV Channels test cases\n")
		}
	}

	return sb.String()
}

// GetOptimizedPrompt builds an optimized prompt for a specific provider
func GetOptimizedPrompt(providerName string, kb *learning.KnowledgeBase, platforms []string) string {
	// Get token limit for provider
	tokenLimit, ok := ProviderTokenLimits[providerName]
	if !ok {
		tokenLimit = 8000 // Default conservative limit
	}

	// Reserve tokens for response
	targetTokens := tokenLimit * 3 / 4 // Use 75% for prompt, 25% for response

	optimizer := NewPromptOptimizer(targetTokens)
	return optimizer.OptimizePrompt(kb, platforms, targetTokens)
}
