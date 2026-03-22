// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package issuedetector

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"digital.vasic.llmorchestrator/pkg/agent"
	"digital.vasic.visionengine/pkg/analyzer"
	"digital.vasic.visionengine/pkg/graph"

	"digital.vasic.helixqa/pkg/session"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAgent implements agent.Agent for testing.
type mockAgent struct {
	mu        sync.Mutex
	id        string
	name      string
	responses []agent.Response
	callIdx   int
	failErr   error
}

func newMockAgent(responses ...agent.Response) *mockAgent {
	return &mockAgent{
		id:        "agent-test",
		name:      "test-agent",
		responses: responses,
	}
}

func (m *mockAgent) ID() string                    { return m.id }
func (m *mockAgent) Name() string                  { return m.name }
func (m *mockAgent) Start(_ context.Context) error { return nil }
func (m *mockAgent) Stop(_ context.Context) error  { return nil }
func (m *mockAgent) IsRunning() bool               { return true }

func (m *mockAgent) Health(_ context.Context) agent.HealthStatus {
	return agent.HealthStatus{Healthy: true, CheckedAt: time.Now()}
}

func (m *mockAgent) Send(
	_ context.Context, _ string,
) (agent.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failErr != nil {
		return agent.Response{}, m.failErr
	}
	if m.callIdx < len(m.responses) {
		resp := m.responses[m.callIdx]
		m.callIdx++
		return resp, nil
	}
	return agent.Response{Content: "[]"}, nil
}

func (m *mockAgent) SendStream(
	_ context.Context, _ string,
) (<-chan agent.StreamChunk, error) {
	ch := make(chan agent.StreamChunk, 1)
	ch <- agent.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockAgent) SendWithAttachments(
	_ context.Context, _ string, _ []agent.Attachment,
) (agent.Response, error) {
	return agent.Response{}, nil
}

func (m *mockAgent) OutputDir() string { return "/tmp" }

func (m *mockAgent) Capabilities() agent.AgentCapabilities {
	return agent.AgentCapabilities{Vision: true}
}

func (m *mockAgent) SupportsVision() bool { return true }

func (m *mockAgent) ModelInfo() agent.ModelInfo {
	return agent.ModelInfo{ID: "model-1"}
}

// mockAnalyzer implements analyzer.Analyzer for testing.
type mockAnalyzer struct{}

func (m *mockAnalyzer) AnalyzeScreen(
	_ context.Context, _ []byte,
) (analyzer.ScreenAnalysis, error) {
	return analyzer.ScreenAnalysis{
		ScreenID: "screen-1", Title: "Test",
	}, nil
}

func (m *mockAnalyzer) CompareScreens(
	_ context.Context, _, _ []byte,
) (analyzer.ScreenDiff, error) {
	return analyzer.ScreenDiff{}, nil
}

func (m *mockAnalyzer) DetectElements(
	_ []byte,
) ([]analyzer.UIElement, error) {
	return nil, nil
}

func (m *mockAnalyzer) DetectText(
	_ []byte,
) ([]analyzer.TextRegion, error) {
	return nil, nil
}

func (m *mockAnalyzer) IdentifyScreen(
	_ context.Context, _ []byte,
) (analyzer.ScreenIdentity, error) {
	return analyzer.ScreenIdentity{}, nil
}

func (m *mockAnalyzer) DetectIssues(
	_ context.Context, _ []byte,
) ([]analyzer.VisualIssue, error) {
	return nil, nil
}

// --- Category Tests ---

func TestAllCategories(t *testing.T) {
	cats := AllCategories()
	assert.Len(t, cats, 6)
	assert.Contains(t, cats, CategoryVisual)
	assert.Contains(t, cats, CategoryCrash)
}

func TestValidCategory(t *testing.T) {
	assert.True(t, ValidCategory("visual"))
	assert.True(t, ValidCategory("ux"))
	assert.True(t, ValidCategory("accessibility"))
	assert.True(t, ValidCategory("functional"))
	assert.True(t, ValidCategory("performance"))
	assert.True(t, ValidCategory("crash"))
	assert.False(t, ValidCategory("unknown"))
	assert.False(t, ValidCategory(""))
}

func TestAllSeverities(t *testing.T) {
	sevs := AllSeverities()
	assert.Len(t, sevs, 4)
	assert.Equal(t, SeverityCritical, sevs[0])
}

func TestValidSeverity(t *testing.T) {
	assert.True(t, ValidSeverity("critical"))
	assert.True(t, ValidSeverity("high"))
	assert.True(t, ValidSeverity("medium"))
	assert.True(t, ValidSeverity("low"))
	assert.False(t, ValidSeverity("extreme"))
	assert.False(t, ValidSeverity(""))
}

func TestCategoryConstants(t *testing.T) {
	assert.Equal(t, IssueCategory("visual"), CategoryVisual)
	assert.Equal(t, IssueCategory("ux"), CategoryUX)
	assert.Equal(t, IssueCategory("accessibility"), CategoryAccessibility)
	assert.Equal(t, IssueCategory("functional"), CategoryFunctional)
	assert.Equal(t, IssueCategory("performance"), CategoryPerformance)
	assert.Equal(t, IssueCategory("crash"), CategoryCrash)
}

func TestSeverityConstants(t *testing.T) {
	assert.Equal(t, IssueSeverity("critical"), SeverityCritical)
	assert.Equal(t, IssueSeverity("high"), SeverityHigh)
	assert.Equal(t, IssueSeverity("medium"), SeverityMedium)
	assert.Equal(t, IssueSeverity("low"), SeverityLow)
}

// --- Detector Tests ---

func TestNewIssueDetector(t *testing.T) {
	ag := newMockAgent()
	az := &mockAnalyzer{}
	sess := session.NewSessionRecorder("test", "/tmp")

	det := NewIssueDetector(ag, az, sess)
	assert.NotNil(t, det)
	assert.Equal(t, 0, det.IssueCount())
	assert.Empty(t, det.Issues())
}

func TestIssueDetector_AnalyzeAction_NoIssues(t *testing.T) {
	ag := newMockAgent(agent.Response{Content: "[]"})
	az := &mockAnalyzer{}
	sess := session.NewSessionRecorder("test", "/tmp")

	det := NewIssueDetector(ag, az, sess)

	before := analyzer.ScreenAnalysis{
		Title:    "Before",
		Elements: []analyzer.UIElement{{Type: "button"}},
	}
	after := analyzer.ScreenAnalysis{
		Title:    "After",
		Elements: []analyzer.UIElement{{Type: "button"}},
	}
	action := analyzer.Action{Type: "click", Target: "button"}

	issues, err := det.AnalyzeAction(
		context.Background(), before, after, action,
	)
	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestIssueDetector_AnalyzeAction_WithIssues(t *testing.T) {
	jsonResp := `[{"category":"visual","severity":"medium","title":"Button truncated","description":"The save button text is cut off","suggestion":"Use wrap_content"}]`
	ag := newMockAgent(agent.Response{Content: jsonResp})
	az := &mockAnalyzer{}
	sess := session.NewSessionRecorder("test", "/tmp")

	det := NewIssueDetector(ag, az, sess)

	before := analyzer.ScreenAnalysis{Title: "Settings"}
	after := analyzer.ScreenAnalysis{
		ScreenID: "screen-settings",
		Title:    "Settings",
	}
	action := analyzer.Action{Type: "scroll", Value: "down"}

	issues, err := det.AnalyzeAction(
		context.Background(), before, after, action,
	)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, CategoryVisual, issues[0].Category)
	assert.Equal(t, SeverityMedium, issues[0].Severity)
	assert.Equal(t, "Button truncated", issues[0].Title)
	assert.Equal(t, "screen-settings", issues[0].ScreenID)
	assert.Equal(t, 1, det.IssueCount())
}

func TestIssueDetector_AnalyzeAction_AgentError(t *testing.T) {
	ag := newMockAgent()
	ag.failErr = fmt.Errorf("agent timeout")
	az := &mockAnalyzer{}
	det := NewIssueDetector(ag, az, nil)

	_, err := det.AnalyzeAction(
		context.Background(),
		analyzer.ScreenAnalysis{},
		analyzer.ScreenAnalysis{},
		analyzer.Action{},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "analyze action")
}

func TestIssueDetector_AnalyzeUX(t *testing.T) {
	jsonResp := `[{"category":"ux","severity":"high","title":"Dead end screen","description":"Settings has no back navigation"}]`
	ag := newMockAgent(agent.Response{Content: jsonResp})
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	g := graph.NewNavigationGraph()
	g.AddScreen(analyzer.ScreenIdentity{ID: "screen-a", Name: "A"})
	g.AddScreen(analyzer.ScreenIdentity{ID: "screen-b", Name: "B"})
	g.AddTransition("screen-a", "screen-b", analyzer.Action{Type: "click"})
	g.SetCurrent("screen-a")

	issues, err := det.AnalyzeUX(context.Background(), g)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, CategoryUX, issues[0].Category)
}

func TestIssueDetector_AnalyzeUX_AgentError(t *testing.T) {
	ag := newMockAgent()
	ag.failErr = fmt.Errorf("timeout")
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	g := graph.NewNavigationGraph()
	_, err := det.AnalyzeUX(context.Background(), g)
	assert.Error(t, err)
}

func TestIssueDetector_AnalyzeAccessibility(t *testing.T) {
	jsonResp := `[{"category":"accessibility","severity":"high","title":"Low contrast","description":"Header text has 2.1:1 contrast ratio"}]`
	ag := newMockAgent(agent.Response{Content: jsonResp})
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	screen := analyzer.ScreenAnalysis{
		ScreenID: "screen-main",
		Title:    "Main Screen",
		Elements: []analyzer.UIElement{
			{Type: "button", Label: "Save", Clickable: true, Confidence: 0.9},
		},
		TextRegions: []analyzer.TextRegion{
			{Text: "Welcome", Confidence: 0.95},
		},
	}

	issues, err := det.AnalyzeAccessibility(context.Background(), screen)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, CategoryAccessibility, issues[0].Category)
	assert.Equal(t, "screen-main", issues[0].ScreenID)
}

func TestIssueDetector_AnalyzeAccessibility_AgentError(t *testing.T) {
	ag := newMockAgent()
	ag.failErr = fmt.Errorf("timeout")
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	_, err := det.AnalyzeAccessibility(
		context.Background(),
		analyzer.ScreenAnalysis{},
	)
	assert.Error(t, err)
}

func TestIssueDetector_IssuesByCategory(t *testing.T) {
	jsonResp := `[
		{"category":"visual","severity":"medium","title":"A"},
		{"category":"ux","severity":"low","title":"B"},
		{"category":"visual","severity":"high","title":"C"}
	]`
	ag := newMockAgent(agent.Response{Content: jsonResp})
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	_, err := det.AnalyzeAction(
		context.Background(),
		analyzer.ScreenAnalysis{},
		analyzer.ScreenAnalysis{},
		analyzer.Action{},
	)
	require.NoError(t, err)

	visual := det.IssuesByCategory(CategoryVisual)
	assert.Len(t, visual, 2)

	ux := det.IssuesByCategory(CategoryUX)
	assert.Len(t, ux, 1)
}

func TestIssueDetector_IssuesBySeverity(t *testing.T) {
	jsonResp := `[
		{"category":"visual","severity":"high","title":"A"},
		{"category":"ux","severity":"high","title":"B"},
		{"category":"visual","severity":"low","title":"C"}
	]`
	ag := newMockAgent(agent.Response{Content: jsonResp})
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	_, err := det.AnalyzeAction(
		context.Background(),
		analyzer.ScreenAnalysis{},
		analyzer.ScreenAnalysis{},
		analyzer.Action{},
	)
	require.NoError(t, err)

	high := det.IssuesBySeverity(SeverityHigh)
	assert.Len(t, high, 2)

	low := det.IssuesBySeverity(SeverityLow)
	assert.Len(t, low, 1)
}

func TestIssueDetector_ParseIssues_InvalidJSON(t *testing.T) {
	ag := newMockAgent(agent.Response{Content: "no json here"})
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	issues, err := det.AnalyzeAction(
		context.Background(),
		analyzer.ScreenAnalysis{},
		analyzer.ScreenAnalysis{},
		analyzer.Action{},
	)
	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestIssueDetector_ParseIssues_InvalidCategory(t *testing.T) {
	jsonResp := `[{"category":"unknown_cat","severity":"medium","title":"X"}]`
	ag := newMockAgent(agent.Response{Content: jsonResp})
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	issues, err := det.AnalyzeAction(
		context.Background(),
		analyzer.ScreenAnalysis{},
		analyzer.ScreenAnalysis{},
		analyzer.Action{},
	)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	// Should default to functional.
	assert.Equal(t, CategoryFunctional, issues[0].Category)
}

func TestIssueDetector_ParseIssues_InvalidSeverity(t *testing.T) {
	jsonResp := `[{"category":"visual","severity":"extreme","title":"X"}]`
	ag := newMockAgent(agent.Response{Content: jsonResp})
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	issues, err := det.AnalyzeAction(
		context.Background(),
		analyzer.ScreenAnalysis{},
		analyzer.ScreenAnalysis{},
		analyzer.Action{},
	)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, SeverityMedium, issues[0].Severity)
}

func TestIssueDetector_ParseIssues_EmptyItems(t *testing.T) {
	jsonResp := `[{"category":"visual","severity":"medium","title":"","description":""}]`
	ag := newMockAgent(agent.Response{Content: jsonResp})
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	issues, err := det.AnalyzeAction(
		context.Background(),
		analyzer.ScreenAnalysis{},
		analyzer.ScreenAnalysis{},
		analyzer.Action{},
	)
	require.NoError(t, err)
	assert.Empty(t, issues)
}

func TestIssueDetector_ParseIssues_JSONInText(t *testing.T) {
	content := `Here are the issues I found:
[{"category":"visual","severity":"low","title":"Misaligned icon","description":"The home icon is 2px off"}]
That's all I see.`
	ag := newMockAgent(agent.Response{Content: content})
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	issues, err := det.AnalyzeAction(
		context.Background(),
		analyzer.ScreenAnalysis{},
		analyzer.ScreenAnalysis{},
		analyzer.Action{},
	)
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "Misaligned icon", issues[0].Title)
}

func TestIssueDetector_IDSequence(t *testing.T) {
	jsonResp := `[{"category":"visual","severity":"low","title":"A"},{"category":"ux","severity":"low","title":"B"}]`
	ag := newMockAgent(
		agent.Response{Content: jsonResp},
		agent.Response{Content: `[{"category":"crash","severity":"critical","title":"C"}]`},
	)
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	issues1, _ := det.AnalyzeAction(
		context.Background(),
		analyzer.ScreenAnalysis{},
		analyzer.ScreenAnalysis{},
		analyzer.Action{},
	)
	require.Len(t, issues1, 2)
	assert.Equal(t, "ISS-0001", issues1[0].ID)
	assert.Equal(t, "ISS-0002", issues1[1].ID)

	issues2, _ := det.AnalyzeAction(
		context.Background(),
		analyzer.ScreenAnalysis{},
		analyzer.ScreenAnalysis{},
		analyzer.Action{},
	)
	require.Len(t, issues2, 1)
	assert.Equal(t, "ISS-0003", issues2[0].ID)
}

func TestIssueDetector_Issues_ReturnsCopy(t *testing.T) {
	jsonResp := `[{"category":"visual","severity":"low","title":"X"}]`
	ag := newMockAgent(agent.Response{Content: jsonResp})
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	det.AnalyzeAction(
		context.Background(),
		analyzer.ScreenAnalysis{},
		analyzer.ScreenAnalysis{},
		analyzer.Action{},
	)

	issues := det.Issues()
	issues[0].Title = "modified"

	original := det.Issues()
	assert.Equal(t, "X", original[0].Title)
}

func TestPromptVersion(t *testing.T) {
	assert.Equal(t, "v1", promptVersion)
}

// Stress test: concurrent issue recording.
func TestIssueDetector_Stress_ConcurrentAnalyze(t *testing.T) {
	const goroutines = 10
	const opsPerGoroutine = 10

	responses := make([]agent.Response, goroutines*opsPerGoroutine)
	for i := range responses {
		responses[i] = agent.Response{
			Content: fmt.Sprintf(
				`[{"category":"visual","severity":"low","title":"Issue %d"}]`, i,
			),
		}
	}
	ag := newMockAgent(responses...)
	det := NewIssueDetector(ag, &mockAnalyzer{}, nil)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				det.AnalyzeAction(
					context.Background(),
					analyzer.ScreenAnalysis{},
					analyzer.ScreenAnalysis{},
					analyzer.Action{},
				)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, goroutines*opsPerGoroutine, det.IssueCount())
}
