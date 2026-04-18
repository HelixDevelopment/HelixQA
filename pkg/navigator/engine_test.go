// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package navigator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"digital.vasic.llmorchestrator/pkg/agent"
	"digital.vasic.visionengine/pkg/analyzer"
	"digital.vasic.visionengine/pkg/graph"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAgent implements agent.Agent for testing.
type mockAgent struct {
	id        string
	name      string
	responses []agent.Response
	callCount int
	running   bool
	vision    bool
	modelInfo agent.ModelInfo
}

func newMockAgent(name string) *mockAgent {
	return &mockAgent{
		id:      "agent-" + name,
		name:    name,
		running: true,
		vision:  true,
		modelInfo: agent.ModelInfo{
			ID:       "model-1",
			Provider: "test",
			Name:     "test-model",
		},
	}
}

func (m *mockAgent) ID() string   { return m.id }
func (m *mockAgent) Name() string { return m.name }

func (m *mockAgent) Start(_ context.Context) error {
	m.running = true
	return nil
}

func (m *mockAgent) Stop(_ context.Context) error {
	m.running = false
	return nil
}

func (m *mockAgent) IsRunning() bool { return m.running }

func (m *mockAgent) Health(_ context.Context) agent.HealthStatus {
	return agent.HealthStatus{
		AgentID: m.id, AgentName: m.name,
		Healthy: true, CheckedAt: time.Now(),
	}
}

func (m *mockAgent) Send(
	_ context.Context, _ string,
) (agent.Response, error) {
	idx := m.callCount
	m.callCount++
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return agent.Response{Content: "ok"}, nil
}

func (m *mockAgent) SendStream(
	_ context.Context, _ string,
) (<-chan agent.StreamChunk, error) {
	ch := make(chan agent.StreamChunk, 1)
	ch <- agent.StreamChunk{Content: "ok", Done: true}
	close(ch)
	return ch, nil
}

func (m *mockAgent) SendWithAttachments(
	_ context.Context, _ string, _ []agent.Attachment,
) (agent.Response, error) {
	return agent.Response{Content: "ok"}, nil
}

func (m *mockAgent) OutputDir() string { return "/tmp/agent" }

func (m *mockAgent) Capabilities() agent.AgentCapabilities {
	return agent.AgentCapabilities{
		Vision:    m.vision,
		Streaming: true,
		MaxTokens: 100000,
	}
}

func (m *mockAgent) SupportsVision() bool { return m.vision }
func (m *mockAgent) ModelInfo() agent.ModelInfo {
	return m.modelInfo
}

// mockAnalyzer implements analyzer.Analyzer for testing.
type mockAnalyzer struct {
	analyses []analyzer.ScreenAnalysis
	diffs    []analyzer.ScreenDiff
	idx      int
	failErr  error
}

func newMockAnalyzer() *mockAnalyzer {
	return &mockAnalyzer{
		analyses: []analyzer.ScreenAnalysis{
			{
				ScreenID:    "screen-main",
				Title:       "Main Screen",
				Description: "The main application screen",
				Elements: []analyzer.UIElement{
					{Type: "button", Label: "Settings", Clickable: true},
				},
				Navigable: []analyzer.Action{
					{Type: "click", Target: "Settings"},
				},
			},
		},
	}
}

func (m *mockAnalyzer) AnalyzeScreen(
	_ context.Context, _ []byte,
) (analyzer.ScreenAnalysis, error) {
	if m.failErr != nil {
		return analyzer.ScreenAnalysis{}, m.failErr
	}
	idx := m.idx % len(m.analyses)
	m.idx++
	return m.analyses[idx], nil
}

func (m *mockAnalyzer) CompareScreens(
	_ context.Context, _, _ []byte,
) (analyzer.ScreenDiff, error) {
	if len(m.diffs) > 0 {
		return m.diffs[0], nil
	}
	return analyzer.ScreenDiff{Similarity: 0.5}, nil
}

func (m *mockAnalyzer) DetectElements(
	_ []byte,
) ([]analyzer.UIElement, error) {
	return []analyzer.UIElement{
		{Type: "button", Label: "OK", Clickable: true},
	}, nil
}

func (m *mockAnalyzer) DetectText(
	_ []byte,
) ([]analyzer.TextRegion, error) {
	return []analyzer.TextRegion{
		{Text: "Hello", Confidence: 0.95},
	}, nil
}

func (m *mockAnalyzer) IdentifyScreen(
	_ context.Context, _ []byte,
) (analyzer.ScreenIdentity, error) {
	return analyzer.ScreenIdentity{
		ID: "screen-1", Name: "Test Screen",
	}, nil
}

func (m *mockAnalyzer) DetectIssues(
	_ context.Context, _ []byte,
) ([]analyzer.VisualIssue, error) {
	return nil, nil
}

// mockExecutor implements ActionExecutor for testing.
type mockExecutor struct {
	clicks     []mockClick
	types      []string
	scrolls    []string
	backs      int
	homes      int
	screenshot []byte
	failErr    error
}

type mockClick struct{ x, y int }

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		screenshot: []byte("SCREENSHOT-DATA"),
	}
}

func (m *mockExecutor) Click(_ context.Context, x, y int) error {
	if m.failErr != nil {
		return m.failErr
	}
	m.clicks = append(m.clicks, mockClick{x, y})
	return nil
}

func (m *mockExecutor) Type(_ context.Context, text string) error {
	if m.failErr != nil {
		return m.failErr
	}
	m.types = append(m.types, text)
	return nil
}

func (m *mockExecutor) Scroll(_ context.Context, dir string, _ int) error {
	if m.failErr != nil {
		return m.failErr
	}
	m.scrolls = append(m.scrolls, dir)
	return nil
}

func (m *mockExecutor) Clear(_ context.Context) error {
	return m.failErr
}

func (m *mockExecutor) LongPress(_ context.Context, _, _ int) error {
	return m.failErr
}

func (m *mockExecutor) Swipe(_ context.Context, _, _, _, _ int) error {
	return m.failErr
}

func (m *mockExecutor) KeyPress(_ context.Context, _ string) error {
	return m.failErr
}

func (m *mockExecutor) Back(_ context.Context) error {
	if m.failErr != nil {
		return m.failErr
	}
	m.backs++
	return nil
}

func (m *mockExecutor) Home(_ context.Context) error {
	if m.failErr != nil {
		return m.failErr
	}
	m.homes++
	return nil
}

func (m *mockExecutor) Screenshot(_ context.Context) ([]byte, error) {
	if m.failErr != nil {
		return nil, m.failErr
	}
	return m.screenshot, nil
}

// --- NavigationEngine Tests ---

func TestNewNavigationEngine(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()

	ne := NewNavigationEngine(ag, az, ex, g)
	assert.NotNil(t, ne)
	assert.NotNil(t, ne.State())
	assert.NotNil(t, ne.Graph())
}

func TestNavigationEngine_PerformAction_Click(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()

	ne := NewNavigationEngine(ag, az, ex, g)
	action := analyzer.Action{Type: "click", Target: "100,200"}

	result, err := ne.PerformAction(context.Background(), action)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "click", result.Action)
	assert.Greater(t, result.Duration, time.Duration(0))

	require.Len(t, ex.clicks, 1)
	assert.Equal(t, 100, ex.clicks[0].x)
	assert.Equal(t, 200, ex.clicks[0].y)
}

func TestNavigationEngine_PerformAction_Type(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	action := analyzer.Action{Type: "type", Value: "hello"}
	result, err := ne.PerformAction(context.Background(), action)
	require.NoError(t, err)
	assert.True(t, result.Success)
	require.Len(t, ex.types, 1)
	assert.Equal(t, "hello", ex.types[0])
}

func TestNavigationEngine_PerformAction_Back(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	action := analyzer.Action{Type: "back"}
	result, err := ne.PerformAction(context.Background(), action)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, ex.backs)
}

func TestNavigationEngine_PerformAction_Home(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	action := analyzer.Action{Type: "home"}
	result, err := ne.PerformAction(context.Background(), action)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, ex.homes)
}

func TestNavigationEngine_PerformAction_UnknownType(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	action := analyzer.Action{Type: "teleport"}
	result, err := ne.PerformAction(context.Background(), action)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown action type")
	assert.False(t, result.Success)
}

func TestNavigationEngine_PerformAction_ExecutorError(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	ex.failErr = fmt.Errorf("device disconnected")
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	action := analyzer.Action{Type: "click", Target: "50,50"}
	result, err := ne.PerformAction(context.Background(), action)
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "device disconnected")
}

func TestNavigationEngine_PerformAction_UpdatesState(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	ne.State().SetCurrentScreen("screen-a")
	action := analyzer.Action{Type: "click", Target: "50,50"}

	_, err := ne.PerformAction(context.Background(), action)
	require.NoError(t, err)

	assert.Equal(t, 1, ne.State().ActionCount())
}

func TestNavigationEngine_GoBack(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	err := ne.GoBack(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, ex.backs)
}

func TestNavigationEngine_GoHome(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	err := ne.GoHome(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, ex.homes)
}

func TestNavigationEngine_CurrentScreen(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	analysis, err := ne.CurrentScreen(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, analysis)
	assert.Equal(t, "Main Screen", analysis.Title)
}

func TestNavigationEngine_CurrentScreen_ScreenshotError(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	ex.failErr = fmt.Errorf("no display")
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	_, err := ne.CurrentScreen(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "screenshot")
}

func TestNavigationEngine_CurrentScreen_AnalysisError(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	az.failErr = fmt.Errorf("analysis timeout")
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	_, err := ne.CurrentScreen(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "analysis")
}

func TestNavigationEngine_NavigateTo(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()

	// Set up a simple graph: A -> B via click.
	screenA := analyzer.ScreenIdentity{ID: "screen-a", Name: "A"}
	screenB := analyzer.ScreenIdentity{ID: "screen-b", Name: "B"}
	g.AddScreen(screenA)
	g.AddScreen(screenB)
	g.AddTransition("screen-a", "screen-b", analyzer.Action{
		Type: "click", Target: "100,100",
	})
	g.SetCurrent("screen-a")

	ne := NewNavigationEngine(ag, az, ex, g)
	err := ne.NavigateTo(context.Background(), "screen-b")
	require.NoError(t, err)

	// Should have performed one click.
	require.Len(t, ex.clicks, 1)
}

func TestNavigationEngine_NavigateTo_NoPath(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()

	screenA := analyzer.ScreenIdentity{ID: "screen-a", Name: "A"}
	screenB := analyzer.ScreenIdentity{ID: "screen-b", Name: "B"}
	g.AddScreen(screenA)
	g.AddScreen(screenB)
	g.SetCurrent("screen-a")
	// No transition added.

	ne := NewNavigationEngine(ag, az, ex, g)
	err := ne.NavigateTo(context.Background(), "screen-b")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "navigate to")
}

func TestNavigationEngine_ExploreUnknown(t *testing.T) {
	ag := newMockAgent("claude")
	ag.responses = []agent.Response{
		{
			Content: "I'll click Settings",
			Actions: []agent.Action{
				{Type: "click", Target: "100,200"},
			},
		},
	}
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	result, err := ne.ExploreUnknown(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.ActionsPerformed)
}

func TestNavigationEngine_ExploreUnknown_ScreenshotFails(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	ex.failErr = fmt.Errorf("no display")
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	_, err := ne.ExploreUnknown(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "explore screenshot")
}

func TestParseCoordinates(t *testing.T) {
	tests := []struct {
		input   string
		expectX int
		expectY int
	}{
		{"100,200", 100, 200},
		{"0,0", 0, 0},
		{"invalid", 0, 0},
		{"", 0, 0},
		{"50,", 50, 0},
	}

	for _, tc := range tests {
		x, y := parseCoordinates(tc.input)
		assert.Equal(t, tc.expectX, x, "input: %s", tc.input)
		assert.Equal(t, tc.expectY, y, "input: %s", tc.input)
	}
}

func TestNavigationEngine_PerformAction_Scroll(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	action := analyzer.Action{Type: "scroll", Value: "down"}
	result, err := ne.PerformAction(context.Background(), action)
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestNavigationEngine_PerformAction_KeyPress(t *testing.T) {
	ag := newMockAgent("claude")
	az := newMockAnalyzer()
	ex := newMockExecutor()
	g := graph.NewNavigationGraph()
	ne := NewNavigationEngine(ag, az, ex, g)

	action := analyzer.Action{Type: "key_press", Value: "Enter"}
	result, err := ne.PerformAction(context.Background(), action)
	require.NoError(t, err)
	assert.True(t, result.Success)
}
