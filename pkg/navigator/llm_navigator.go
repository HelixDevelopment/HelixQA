// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"digital.vasic.llmorchestrator/pkg/agent"
	"digital.vasic.visionengine/pkg/analyzer"
)

// LLMNavigator uses LLM reasoning for intelligent navigation
type LLMNavigator struct {
	agent    agent.Agent
	analyzer analyzer.Analyzer
	graph    *NavigationGraph
	state    *StateTracker
	executor ActionExecutor
	history  *NavigationHistory
}

// NavigationGraph tracks discovered screens and transitions
type NavigationGraph struct {
	screens     map[string]*Screen
	transitions map[string][]Transition
}

// Screen represents a discovered application screen
type Screen struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Features    []string `json:"features"`
	Platforms   []string `json:"platforms"`
	Visited     bool     `json:"visited"`
	VisitCount  int      `json:"visit_count"`
}

// Transition represents a navigation between screens
type Transition struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Action    string `json:"action"`
	Success   bool   `json:"success"`
	Timestamp int64  `json:"timestamp"`
}

// NavigationHistory maintains navigation history
type NavigationHistory struct {
	paths []NavigationPath
	mu    sync.Mutex
}

// NavigationPath represents a computed path to reach a target
type NavigationPath struct {
	Start         string   `json:"start"`
	Destination   string   `json:"destination"`
	Actions       []string `json:"actions"`
	EstimatedTime int      `json:"estimated_time_ms"`
	Confidence    float64  `json:"confidence"`
}

// NewLLMNavigator creates a new LLM-powered navigator
func NewLLMNavigator(ag agent.Agent, an analyzer.Analyzer, exec ActionExecutor) *LLMNavigator {
	return &LLMNavigator{
		agent:    ag,
		analyzer: an,
		graph:    NewNavigationGraph(),
		state:    NewStateTracker(),
		executor: exec,
		history:  &NavigationHistory{},
	}
}

// NavigateTo navigates to a specific screen or feature
func (n *LLMNavigator) NavigateTo(ctx context.Context, target string) error {
	// Check if we already know how to reach this screen
	targetScreen := n.graph.GetScreen(target)
	if targetScreen == nil {
		// Ask LLM to infer the navigation path
		path, err := n.inferPath(ctx, target)
		if err != nil {
			return fmt.Errorf("infer path to %s: %w", target, err)
		}
		targetScreen = n.graph.GetScreen(path.Destination)
	}

	// Compute shortest path from current screen
	current := n.state.CurrentScreen()
	path := n.graph.ShortestPath(current, targetScreen.ID)

	// Execute navigation actions
	for _, action := range path.Actions {
		if err := n.executeAction(ctx, action); err != nil {
			return fmt.Errorf("execute action %s: %w", action, err)
		}

		// Update state
		n.state.RecordAction(current, targetScreen.ID, action, true)
		current = targetScreen.ID
	}

	return nil
}

// inferPath asks the LLM to suggest a navigation path
func (n *LLMNavigator) inferPath(ctx context.Context, target string) (*NavigationPath, error) {
	currentScreen := n.state.CurrentScreen()
	knownScreens := n.graph.GetKnownScreenNames()

	prompt := fmt.Sprintf(`
I need to navigate to "%s".

Current screen: %s
Known screens: %v

What navigation actions should I take? Return a JSON object with:
{
  "destination": "screen_id",
  "actions": ["action1", "action2"],
  "confidence": 0.0-1.0
}
`, target, currentScreen, knownScreens)

	resp, err := n.agent.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	var path NavigationPath
	if err := json.Unmarshal([]byte(resp.Content), &path); err != nil {
		return nil, fmt.Errorf("parse path: %w", err)
	}

	return &path, nil
}

// executeAction performs a navigation action. Only `back` and `home`
// are dispatched without explicit coordinates — every other action MUST
// carry its target coordinates / element identifier and be invoked
// through the typed ActionExecutor API directly. Returning an explicit
// error closes the §11.4 stub-bluff: callers that previously got `nil`
// for an unsupported action (and assumed success) now see the failure.
func (n *LLMNavigator) executeAction(ctx context.Context, action string) error {
	switch action {
	case "back":
		return n.executor.Back(ctx)
	case "home":
		return n.executor.Home(ctx)
	default:
		return fmt.Errorf("navigator.executeAction: unsupported coordinate-free action %q (only \"back\" and \"home\" dispatch without coordinates; for click/type/scroll/swipe/longpress/keypress, invoke ActionExecutor directly with the target coordinates / key)", action)
	}
}

// NewNavigationGraph creates a new empty navigation graph
func NewNavigationGraph() *NavigationGraph {
	return &NavigationGraph{
		screens:     make(map[string]*Screen),
		transitions: make(map[string][]Transition),
	}
}

// AddScreen adds a screen to the graph
func (g *NavigationGraph) AddScreen(screen *Screen) {
	g.screens[screen.ID] = screen
}

// GetScreen retrieves a screen by ID
func (g *NavigationGraph) GetScreen(id string) *Screen {
	return g.screens[id]
}

// GetKnownScreenNames returns all known screen names
func (g *NavigationGraph) GetKnownScreenNames() []string {
	names := make([]string, 0, len(g.screens))
	for name := range g.screens {
		names = append(names, name)
	}
	return names
}

// AddTransition records a directed edge `from → to` via `action`. Multiple
// transitions are tolerated; ShortestPath always picks the shortest by
// edge count (BFS).
func (g *NavigationGraph) AddTransition(from, to, action string) {
	g.transitions[from] = append(g.transitions[from], Transition{
		From:   from,
		To:     to,
		Action: action,
	})
}

// ShortestPath computes the shortest path between two screens via BFS over
// the recorded transitions. Returns nil if `to` is unreachable from `from`
// (or if either screen is unknown). The previous "return single-step
// stub" behavior was a §11.4 PASS-bluff — any test asserting multi-hop
// navigation would PASS against a fabricated one-step path.
func (g *NavigationGraph) ShortestPath(from, to string) *NavigationPath {
	if from == to {
		return &NavigationPath{Start: from, Destination: to}
	}
	if _, ok := g.screens[from]; !ok {
		return nil
	}
	if _, ok := g.screens[to]; !ok {
		return nil
	}

	type queueItem struct {
		screen  string
		actions []string
	}
	visited := map[string]bool{from: true}
	queue := []queueItem{{screen: from, actions: nil}}
	for len(queue) > 0 {
		head := queue[0]
		queue = queue[1:]
		for _, t := range g.transitions[head.screen] {
			if visited[t.To] {
				continue
			}
			next := append(append([]string(nil), head.actions...), t.Action)
			if t.To == to {
				return &NavigationPath{
					Start:       from,
					Destination: to,
					Actions:     next,
				}
			}
			visited[t.To] = true
			queue = append(queue, queueItem{screen: t.To, actions: next})
		}
	}
	return nil
}
