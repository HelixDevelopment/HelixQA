// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"fmt"
	"time"

	"digital.vasic.llmorchestrator/pkg/agent"
	"digital.vasic.visionengine/pkg/analyzer"
	"digital.vasic.visionengine/pkg/graph"
)

// NavigationEngine orchestrates UI navigation by combining
// LLM agent decisions, vision analysis, and platform-specific
// action execution. It maintains a navigation graph and state
// tracker for path-finding and coverage tracking.
type NavigationEngine struct {
	agent    agent.Agent
	analyzer analyzer.Analyzer
	executor ActionExecutor
	graph    graph.NavigationGraph
	state    *StateTracker
}

// NewNavigationEngine creates a NavigationEngine with all
// required dependencies.
func NewNavigationEngine(
	ag agent.Agent,
	az analyzer.Analyzer,
	exec ActionExecutor,
	navGraph graph.NavigationGraph,
) *NavigationEngine {
	return &NavigationEngine{
		agent:    ag,
		analyzer: az,
		executor: exec,
		graph:    navGraph,
		state:    NewStateTracker(),
	}
}

// NavigateTo attempts to navigate to the named target screen
// by looking up a path in the navigation graph and executing
// the required actions.
func (ne *NavigationEngine) NavigateTo(
	ctx context.Context, target string,
) error {
	path, err := ne.graph.PathTo(target)
	if err != nil {
		return fmt.Errorf("navigate to %s: %w", target, err)
	}

	for _, transition := range path {
		result, execErr := ne.PerformAction(
			ctx, transition.Action,
		)
		if execErr != nil {
			return fmt.Errorf(
				"action %s failed: %w",
				transition.Action.Type, execErr,
			)
		}
		if !result.Success {
			return fmt.Errorf(
				"action %s did not succeed: %s",
				transition.Action.Type, result.Error,
			)
		}
	}
	return nil
}

// PerformAction executes a single action and returns the
// result including whether the screen changed.
func (ne *NavigationEngine) PerformAction(
	ctx context.Context, action analyzer.Action,
) (*ActionResult, error) {
	start := time.Now()
	result := &ActionResult{
		Action: action.Type,
	}

	prevScreen := ne.state.CurrentScreen()

	var err error
	switch action.Type {
	case "click":
		x, y := parseCoordinates(action.Target)
		err = ne.executor.Click(ctx, x, y)
	case "type":
		err = ne.executor.Type(ctx, action.Value)
	case "scroll":
		amount := 300 // default scroll amount
		err = ne.executor.Scroll(ctx, action.Value, amount)
	case "long_press":
		x, y := parseCoordinates(action.Target)
		err = ne.executor.LongPress(ctx, x, y)
	case "swipe":
		err = ne.executor.Scroll(ctx, action.Value, 500)
	case "key_press":
		err = ne.executor.KeyPress(ctx, action.Value)
	case "back":
		err = ne.executor.Back(ctx)
	case "home":
		err = ne.executor.Home(ctx)
	default:
		err = fmt.Errorf("unknown action type: %s", action.Type)
	}

	result.Duration = time.Since(start)

	if err != nil {
		result.Error = err.Error()
		ne.state.RecordAction(
			prevScreen, "", action.Type, false,
		)
		return result, err
	}

	result.Success = true
	ne.state.RecordAction(
		prevScreen, prevScreen, action.Type, true,
	)
	return result, nil
}

// ExploreUnknown asks the LLM to suggest an action to explore
// undiscovered screens, then executes it.
func (ne *NavigationEngine) ExploreUnknown(
	ctx context.Context,
) (*ExploreResult, error) {
	start := time.Now()
	result := &ExploreResult{}

	// Take a screenshot.
	ssData, err := ne.executor.Screenshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("explore screenshot: %w", err)
	}

	// Analyze the screen.
	analysis, err := ne.analyzer.AnalyzeScreen(ctx, ssData)
	if err != nil {
		return nil, fmt.Errorf("explore analysis: %w", err)
	}

	// Ask agent for an action.
	prompt := fmt.Sprintf(
		"You are on screen %q with %d navigable actions. "+
			"Pick the most interesting action to explore "+
			"undiscovered areas. Reply with a JSON action object.",
		analysis.Title, len(analysis.Navigable),
	)
	resp, err := ne.agent.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("explore agent: %w", err)
	}

	// Try to perform the first suggested action.
	if len(resp.Actions) > 0 {
		action := analyzer.Action{
			Type:   resp.Actions[0].Type,
			Target: resp.Actions[0].Target,
			Value:  resp.Actions[0].Value,
		}
		_, execErr := ne.PerformAction(ctx, action)
		result.ActionsPerformed++
		if execErr == nil {
			result.ScreensDiscovered++
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// CurrentScreen analyzes the current visible screen and
// returns its analysis.
func (ne *NavigationEngine) CurrentScreen(
	ctx context.Context,
) (*analyzer.ScreenAnalysis, error) {
	ssData, err := ne.executor.Screenshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}
	analysis, err := ne.analyzer.AnalyzeScreen(ctx, ssData)
	if err != nil {
		return nil, fmt.Errorf("analysis: %w", err)
	}
	return &analysis, nil
}

// GoBack sends the back action.
func (ne *NavigationEngine) GoBack(
	ctx context.Context,
) error {
	return ne.executor.Back(ctx)
}

// GoHome sends the home action.
func (ne *NavigationEngine) GoHome(
	ctx context.Context,
) error {
	return ne.executor.Home(ctx)
}

// State returns the state tracker.
func (ne *NavigationEngine) State() *StateTracker {
	return ne.state
}

// Graph returns the navigation graph.
func (ne *NavigationEngine) Graph() graph.NavigationGraph {
	return ne.graph
}

// parseCoordinates extracts x,y from a "X,Y" string.
// Returns 0,0 on failure.
func parseCoordinates(target string) (int, int) {
	var x, y int
	_, _ = fmt.Sscanf(target, "%d,%d", &x, &y)
	return x, y
}
