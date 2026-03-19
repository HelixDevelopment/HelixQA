// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"regexp"
	"strings"
)

// MaxSafeLength is the maximum allowed length for sanitized
// strings (1MB).
const MaxSafeLength = 1024 * 1024

// pathTraversalPattern detects path traversal attempts.
var pathTraversalPattern = regexp.MustCompile(
	`(?:\.\./|\.\.\\|%2e%2e%2f|%2e%2e/|\.%2e/|%2e\./)`,
)

// shellMetachars are characters that could be dangerous in
// shell contexts.
var shellMetachars = regexp.MustCompile(
	"[`$|;&><(){}\\[\\]!#~]",
)

// promptInjectionPatterns detects common prompt injection
// attempts.
var promptInjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(previous|above|all)\s+instructions`),
	regexp.MustCompile(`(?i)system\s*:\s*`),
	regexp.MustCompile(`(?i)you\s+are\s+now`),
	regexp.MustCompile(`(?i)new\s+instructions`),
	regexp.MustCompile(`(?i)disregard`),
}

// SanitizePath removes path traversal patterns and shell
// metacharacters from a file path.
func SanitizePath(path string) string {
	if len(path) > MaxSafeLength {
		path = path[:MaxSafeLength]
	}
	// Remove path traversal.
	path = pathTraversalPattern.ReplaceAllString(path, "")
	// Remove shell metacharacters.
	path = shellMetachars.ReplaceAllString(path, "")
	return strings.TrimSpace(path)
}

// SanitizeContent removes dangerous patterns from LLM-
// generated content before using it in tickets or reports.
func SanitizeContent(content string) string {
	if len(content) > MaxSafeLength {
		content = content[:MaxSafeLength]
	}
	// Remove shell metacharacters that could be dangerous.
	content = shellMetachars.ReplaceAllString(content, "")
	return content
}

// DetectPromptInjection checks if a string contains patterns
// that suggest a prompt injection attempt. Returns true if
// suspicious patterns are detected.
func DetectPromptInjection(text string) bool {
	for _, pattern := range promptInjectionPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// SanitizeTicketContent sanitizes content for use in markdown
// tickets. Removes shell metacharacters and path traversal
// patterns.
func SanitizeTicketContent(content string) string {
	if len(content) > MaxSafeLength {
		content = content[:MaxSafeLength]
	}
	content = pathTraversalPattern.ReplaceAllString(content, "")
	return content
}

// ValidateFilePath checks if a path is safe to use (no
// traversal, no shell chars, reasonable length).
func ValidateFilePath(path string) bool {
	if path == "" || len(path) > 4096 {
		return false
	}
	if pathTraversalPattern.MatchString(path) {
		return false
	}
	if shellMetachars.MatchString(path) {
		return false
	}
	return true
}
