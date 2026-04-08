// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package reproduce provides automated bug reproduction
// capabilities for HelixQA. When the Analyze phase finds
// a bug, BugReproducer replays the action sequence that
// led to the bug and uses LLM vision to confirm whether
// the bug is reproducible.
package reproduce

import (
	"context"
	"fmt"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/llm"
	"digital.vasic.helixqa/pkg/navigator"
)

// defaultMaxRetries is the default number of reproduction
// attempts before giving up.
const defaultMaxRetries = 3

// defaultActionDelay is the pause between actions during
// replay to let the UI settle.
// REDUCED for FLASHING FAST performance (was 1s).
const defaultActionDelay = 200 * time.Millisecond

// defaultScreenshotDelay is the pause after all actions
// are replayed before taking the verification screenshot.
// REDUCED for FLASHING FAST performance (was 500ms).
const defaultScreenshotDelay = 100 * time.Millisecond

// BugReproducer attempts to reproduce bugs found during
// analysis by replaying the action sequence that led to
// the bug and using LLM vision to confirm reproduction.
type BugReproducer struct {
	executor        navigator.ActionExecutor
	provider        llm.Provider
	maxRetries      int
	actionDelay     time.Duration
	screenshotDelay time.Duration
}

// Option configures a BugReproducer.
type Option func(*BugReproducer)

// WithMaxRetries sets the maximum number of reproduction
// attempts. Default is 3.
func WithMaxRetries(n int) Option {
	return func(br *BugReproducer) {
		if n > 0 {
			br.maxRetries = n
		}
	}
}

// WithActionDelay sets the pause between replayed actions.
// Default is 1 second.
func WithActionDelay(d time.Duration) Option {
	return func(br *BugReproducer) {
		if d >= 0 {
			br.actionDelay = d
		}
	}
}

// WithScreenshotDelay sets the pause after the last action
// before taking the verification screenshot. Default is
// 500ms.
func WithScreenshotDelay(d time.Duration) Option {
	return func(br *BugReproducer) {
		if d >= 0 {
			br.screenshotDelay = d
		}
	}
}

// NewBugReproducer creates a BugReproducer with the given
// executor and LLM provider. The executor performs UI
// actions (taps, types, scrolls) and the provider analyses
// screenshots to confirm reproduction.
func NewBugReproducer(
	executor navigator.ActionExecutor,
	provider llm.Provider,
	opts ...Option,
) *BugReproducer {
	br := &BugReproducer{
		executor:        executor,
		provider:        provider,
		maxRetries:      defaultMaxRetries,
		actionDelay:     defaultActionDelay,
		screenshotDelay: defaultScreenshotDelay,
	}
	for _, opt := range opts {
		opt(br)
	}
	return br
}

// ReproductionResult captures the outcome of a bug
// reproduction attempt.
type ReproductionResult struct {
	// BugID is the identifier of the bug being reproduced.
	BugID string `json:"bug_id"`

	// Reproduced is true if the bug was confirmed present.
	Reproduced bool `json:"reproduced"`

	// Attempts is the number of reproduction tries made.
	Attempts int `json:"attempts"`

	// ActionSequence records the actions that were replayed.
	ActionSequence []Action `json:"action_sequence"`

	// Screenshots holds paths or identifiers of screenshots
	// captured during reproduction.
	Screenshots []string `json:"screenshots,omitempty"`

	// Evidence is the LLM's description of the reproduced
	// state when the bug was confirmed.
	Evidence string `json:"evidence,omitempty"`

	// Duration is the total time spent on reproduction.
	Duration time.Duration `json:"duration"`

	// Error holds any error that prevented reproduction
	// from completing.
	Error string `json:"error,omitempty"`
}

// Bug describes a bug to reproduce.
type Bug struct {
	// ID is a unique identifier for the bug.
	ID string `json:"id"`

	// Description explains what the bug is.
	Description string `json:"description"`

	// ActionSequence is the list of actions that led to
	// the bug being observed.
	ActionSequence []Action `json:"action_sequence"`

	// OriginalScreen is a path to the screenshot where
	// the bug was originally found.
	OriginalScreen string `json:"original_screen,omitempty"`

	// Severity indicates the bug priority.
	Severity string `json:"severity"`

	// Platform is the platform where the bug was found.
	Platform string `json:"platform,omitempty"`
}

// Validate checks that the bug has the minimum fields
// needed for reproduction.
func (b Bug) Validate() error {
	if b.ID == "" {
		return fmt.Errorf("reproduce: bug ID is required")
	}
	if b.Description == "" {
		return fmt.Errorf(
			"reproduce: bug %s description is required",
			b.ID,
		)
	}
	return nil
}

// Action describes a single UI action to replay.
type Action struct {
	// Type is the action type: "click", "type", "scroll",
	// "swipe", "key_press", "back", "home", "long_press",
	// "clear", or "wait".
	Type string `json:"type"`

	// Value is the action parameter. Meaning depends on
	// Type:
	//   click:      "x,y"
	//   type:       text to enter
	//   scroll:     "direction,amount"
	//   swipe:      "fromX,fromY,toX,toY"
	//   key_press:  key name
	//   long_press: "x,y"
	//   clear:      (ignored)
	//   back:       (ignored)
	//   home:       (ignored)
	//   wait:       duration string (e.g. "2s")
	Value string `json:"value"`
}

// Reproduce attempts to reproduce a bug by replaying its
// action sequence and using LLM vision to confirm. It
// tries up to maxRetries times, returning on the first
// successful reproduction or after all attempts are
// exhausted.
//
// The process for each attempt:
//  1. Replay each action in the bug's ActionSequence.
//  2. Wait for the UI to settle.
//  3. Take a screenshot.
//  4. Ask the LLM vision provider to compare the screenshot
//     with the bug description.
//  5. If the LLM confirms the bug is visible, mark as
//     reproduced.
func (br *BugReproducer) Reproduce(
	ctx context.Context,
	bug Bug,
) (*ReproductionResult, error) {
	if err := bug.Validate(); err != nil {
		return nil, err
	}

	start := time.Now()
	result := &ReproductionResult{
		BugID:          bug.ID,
		ActionSequence: bug.ActionSequence,
	}

	for attempt := 0; attempt < br.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			result.Duration = time.Since(start)
			result.Error = ctx.Err().Error()
			return result, ctx.Err()
		default:
		}

		result.Attempts++

		// Replay each action in sequence.
		replayErr := br.replayActions(
			ctx, bug.ActionSequence,
		)
		if replayErr != nil {
			// Action replay failed; try again on next
			// attempt.
			continue
		}

		// Wait for the UI to settle after the last action.
		select {
		case <-ctx.Done():
			result.Duration = time.Since(start)
			result.Error = ctx.Err().Error()
			return result, ctx.Err()
		case <-time.After(br.screenshotDelay):
		}

		// Take a screenshot of the current state.
		screenshot, err := br.executor.Screenshot(ctx)
		if err != nil {
			continue
		}

		screenshotID := fmt.Sprintf(
			"repro-%s-attempt-%d", bug.ID, attempt+1,
		)
		result.Screenshots = append(
			result.Screenshots, screenshotID,
		)

		// Ask the LLM to compare.
		confirmed, evidence := br.confirmBug(
			ctx, screenshot, bug.Description,
		)
		if confirmed {
			result.Reproduced = true
			result.Evidence = evidence
			result.Duration = time.Since(start)
			return result, nil
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// ReproduceBatch attempts to reproduce multiple bugs,
// returning results for each. It processes bugs
// sequentially and respects context cancellation between
// bugs.
func (br *BugReproducer) ReproduceBatch(
	ctx context.Context,
	bugs []Bug,
) ([]*ReproductionResult, error) {
	results := make([]*ReproductionResult, 0, len(bugs))

	for _, bug := range bugs {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		result, err := br.Reproduce(ctx, bug)
		if err != nil {
			// Context cancellation — stop processing.
			if ctx.Err() != nil {
				return results, err
			}
			// Validation error — record and continue.
			results = append(results, &ReproductionResult{
				BugID: bug.ID,
				Error: err.Error(),
			})
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// HighSeverityBugs filters a bug list to only include
// bugs with "critical" or "high" severity. This is useful
// for prioritising reproduction of the most impactful
// bugs.
func HighSeverityBugs(bugs []Bug) []Bug {
	var result []Bug
	for _, b := range bugs {
		sev := strings.ToLower(b.Severity)
		if sev == "critical" || sev == "high" {
			result = append(result, b)
		}
	}
	return result
}

// replayActions executes the action sequence on the
// executor, pausing between each action.
func (br *BugReproducer) replayActions(
	ctx context.Context,
	actions []Action,
) error {
	for _, action := range actions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := executeAction(
			ctx, br.executor, action,
		); err != nil {
			return fmt.Errorf(
				"replay action %s: %w", action.Type, err,
			)
		}

		// Pause between actions.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(br.actionDelay):
		}
	}
	return nil
}

// confirmBug asks the LLM provider to compare a screenshot
// with the bug description and returns whether the bug is
// confirmed and the LLM's explanation.
func (br *BugReproducer) confirmBug(
	ctx context.Context,
	screenshot []byte,
	description string,
) (bool, string) {
	if !br.provider.SupportsVision() {
		return false, ""
	}

	prompt := fmt.Sprintf(
		"Compare this screenshot with the bug description: "+
			"%q. Is the bug visible in this screenshot? "+
			"Answer YES or NO followed by a brief explanation.",
		description,
	)

	resp, err := br.provider.Vision(ctx, screenshot, prompt)
	if err != nil {
		return false, ""
	}

	content := strings.TrimSpace(resp.Content)
	upper := strings.ToUpper(content)

	if strings.HasPrefix(upper, "YES") ||
		strings.Contains(upper, "YES") {
		return true, content
	}

	return false, content
}

// executeAction dispatches a single Action to the
// ActionExecutor based on the action type.
func executeAction(
	ctx context.Context,
	exec navigator.ActionExecutor,
	action Action,
) error {
	switch strings.ToLower(action.Type) {
	case "click", "tap":
		var x, y int
		if _, err := fmt.Sscanf(
			action.Value, "%d,%d", &x, &y,
		); err != nil {
			return fmt.Errorf("parse click coords: %w", err)
		}
		return exec.Click(ctx, x, y)

	case "type", "text", "input":
		return exec.Type(ctx, action.Value)

	case "clear":
		return exec.Clear(ctx)

	case "scroll":
		var direction string
		var amount int
		if _, err := fmt.Sscanf(
			action.Value, "%s %d", &direction, &amount,
		); err != nil {
			// Default scroll amount.
			direction = action.Value
			amount = 500
		}
		return exec.Scroll(ctx, direction, amount)

	case "swipe":
		var fromX, fromY, toX, toY int
		if _, err := fmt.Sscanf(
			action.Value, "%d,%d,%d,%d",
			&fromX, &fromY, &toX, &toY,
		); err != nil {
			return fmt.Errorf("parse swipe coords: %w", err)
		}
		return exec.Swipe(ctx, fromX, fromY, toX, toY)

	case "key_press", "keypress", "key":
		return exec.KeyPress(ctx, action.Value)

	case "long_press", "longpress":
		var x, y int
		if _, err := fmt.Sscanf(
			action.Value, "%d,%d", &x, &y,
		); err != nil {
			return fmt.Errorf(
				"parse long_press coords: %w", err,
			)
		}
		return exec.LongPress(ctx, x, y)

	case "back":
		return exec.Back(ctx)

	case "home":
		return exec.Home(ctx)

	case "wait":
		d, err := time.ParseDuration(action.Value)
		if err != nil {
			d = 1 * time.Second
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
		return nil

	default:
		return fmt.Errorf(
			"unknown action type: %s", action.Type,
		)
	}
}
