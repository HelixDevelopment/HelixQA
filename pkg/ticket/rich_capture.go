// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	"fmt"
	"strings"
	"time"
)

// RichCaptureInput groups every piece of evidence a rich ticket
// needs. Every field is optional; the capture helper populates
// whatever the caller supplies.
type RichCaptureInput struct {
	// SessionID ties this ticket to qa-results/session-<id>.
	SessionID string

	// StepNumber is the 1-based iteration index inside the Agent
	// run that produced this failure.
	StepNumber int

	// FailedAt is when the failure was first observed. Used to
	// pin VideoTimestamp against the session's video origin.
	FailedAt time.Time

	// VideoOriginAt is when the video recording started. The
	// capture helper subtracts this from FailedAt to produce the
	// mm:ss VideoTimestamp.
	VideoOriginAt time.Time

	// VideoPath is the filesystem path of the video recording.
	// Written into VideoRefs[0].
	VideoPath string

	// BeforeScreenshot / AfterScreenshot are the paired pre/post
	// frames. Either may be empty.
	BeforeScreenshot string
	AfterScreenshot  string

	// LLMReasoning is the planner's evaluation + memory + next_goal
	// transcript across the last N steps before the failure. Each
	// string is one step.
	LLMReasoning []string

	// StackTrace is the native stack (Java / Go / JS) captured at
	// failure time.
	StackTrace string

	// ReproductionBankTemplate is a YAML stub the caller can paste
	// into a fixes-validation bank so the scenario becomes a
	// permanent regression guard. The Bank field on the ticket
	// records the eventual destination.
	ReproductionBankTemplate string
	ReproductionBank         string
}

// CaptureRich populates the OpenClawing2 Phase 7 rich-evidence
// fields on an existing Ticket. Returns t for chaining.
//
// The helper is deliberately forgiving: if any source field on the
// input is zero, the corresponding field on the ticket stays
// untouched, so legacy tickets that only have Screenshots + Logs
// upgrade gracefully when a caller starts supplying more evidence.
func CaptureRich(t *Ticket, input RichCaptureInput) *Ticket {
	if t == nil {
		return nil
	}
	if input.SessionID != "" {
		t.SessionID = input.SessionID
	}
	if input.StepNumber > 0 {
		t.StepNumber = input.StepNumber
	}
	if !input.FailedAt.IsZero() && !input.VideoOriginAt.IsZero() && input.FailedAt.After(input.VideoOriginAt) {
		delta := input.FailedAt.Sub(input.VideoOriginAt)
		t.VideoTimestamp = formatMMSS(delta)
	}
	if input.VideoPath != "" {
		t.VideoRefs = append(t.VideoRefs, &VideoReference{
			VideoPath: input.VideoPath,
		})
	}
	if input.BeforeScreenshot != "" {
		t.BeforeScreenshotPath = input.BeforeScreenshot
		// Also append to the Screenshots slice so reviewers who
		// scan Screenshots still see both frames.
		t.Screenshots = append(t.Screenshots, input.BeforeScreenshot)
	}
	if input.AfterScreenshot != "" {
		t.AfterScreenshotPath = input.AfterScreenshot
		t.Screenshots = append(t.Screenshots, input.AfterScreenshot)
	}
	if len(input.LLMReasoning) > 0 {
		t.LLMReasoningTranscript = append([]string{}, input.LLMReasoning...)
	}
	if input.StackTrace != "" && t.StackTrace == "" {
		t.StackTrace = input.StackTrace
	}
	if input.ReproductionBank != "" {
		t.ReproductionBank = input.ReproductionBank
	}
	return t
}

// RenderRichMarkdown formats every rich-evidence field into a
// reviewer-friendly markdown block. The section is safe to append
// to any existing ticket body — it emits nothing when the ticket
// carries no rich fields.
func RenderRichMarkdown(t *Ticket) string {
	if t == nil {
		return ""
	}
	if !hasAnyRichField(t) {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## Evidence\n\n")
	if t.SessionID != "" {
		fmt.Fprintf(&b, "- **Session ID:** `%s`\n", t.SessionID)
	}
	if t.StepNumber > 0 {
		fmt.Fprintf(&b, "- **Step number:** %d\n", t.StepNumber)
	}
	if t.VideoTimestamp != "" {
		fmt.Fprintf(&b, "- **Video timestamp:** %s\n", t.VideoTimestamp)
	}
	if len(t.VideoRefs) > 0 {
		b.WriteString("- **Videos:**\n")
		for _, v := range t.VideoRefs {
			fmt.Fprintf(&b, "  - [%s](%s)\n", v.VideoPath, v.VideoPath)
		}
	}
	if t.BeforeScreenshotPath != "" || t.AfterScreenshotPath != "" {
		b.WriteString("- **Screenshots:**\n")
		if t.BeforeScreenshotPath != "" {
			fmt.Fprintf(&b, "  - Before: `%s`\n", t.BeforeScreenshotPath)
		}
		if t.AfterScreenshotPath != "" {
			fmt.Fprintf(&b, "  - After: `%s`\n", t.AfterScreenshotPath)
		}
	}
	if t.StackTrace != "" {
		b.WriteString("\n### Stack trace\n\n```\n")
		b.WriteString(t.StackTrace)
		b.WriteString("\n```\n")
	}
	if len(t.LLMReasoningTranscript) > 0 {
		b.WriteString("\n### LLM reasoning transcript\n\n")
		for i, line := range t.LLMReasoningTranscript {
			fmt.Fprintf(&b, "%d. %s\n", i+1, line)
		}
	}
	if t.ReproductionBank != "" {
		fmt.Fprintf(&b, "\n### Reproduction bank\n\nRegistered at `%s`.\n", t.ReproductionBank)
	}
	return b.String()
}

// MandatoryRichFields returns the field names that a production-
// ready ticket MUST carry. The capture CLI runs this over newly
// emitted tickets and fails the QA campaign if any required field
// is still empty.
func MandatoryRichFields() []string {
	return []string{
		"SessionID",
		"StepNumber",
		"BeforeScreenshotPath",
		"AfterScreenshotPath",
		"LLMReasoningTranscript",
	}
}

// VerifyMandatory returns the names of mandatory rich fields that
// are empty on t. An empty slice means the ticket is complete.
func VerifyMandatory(t *Ticket) []string {
	if t == nil {
		return MandatoryRichFields()
	}
	var missing []string
	if t.SessionID == "" {
		missing = append(missing, "SessionID")
	}
	if t.StepNumber == 0 {
		missing = append(missing, "StepNumber")
	}
	if t.BeforeScreenshotPath == "" {
		missing = append(missing, "BeforeScreenshotPath")
	}
	if t.AfterScreenshotPath == "" {
		missing = append(missing, "AfterScreenshotPath")
	}
	if len(t.LLMReasoningTranscript) == 0 {
		missing = append(missing, "LLMReasoningTranscript")
	}
	return missing
}

func hasAnyRichField(t *Ticket) bool {
	return t.SessionID != "" || t.StepNumber > 0 || t.VideoTimestamp != "" ||
		len(t.VideoRefs) > 0 || t.BeforeScreenshotPath != "" ||
		t.AfterScreenshotPath != "" || t.StackTrace != "" ||
		len(t.LLMReasoningTranscript) > 0 || t.ReproductionBank != ""
}

func formatMMSS(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d.Seconds())
	mm := total / 60
	ss := total % 60
	return fmt.Sprintf("%02d:%02d", mm, ss)
}
