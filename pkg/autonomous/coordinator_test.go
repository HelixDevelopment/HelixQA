// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"testing"
	"time"

	"digital.vasic.docprocessor/pkg/coverage"
	"digital.vasic.docprocessor/pkg/feature"
	"digital.vasic.llmorchestrator/pkg/agent"
	"digital.vasic.visionengine/pkg/analyzer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testAgent implements agent.Agent for testing.
type testAgent struct {
	id   string
	name string
}

func newTestAgent(id, name string) *testAgent {
	return &testAgent{id: id, name: name}
}

func (a *testAgent) ID() string                    { return a.id }
func (a *testAgent) Name() string                  { return a.name }
func (a *testAgent) Start(_ context.Context) error { return nil }
func (a *testAgent) Stop(_ context.Context) error  { return nil }
func (a *testAgent) IsRunning() bool               { return true }

func (a *testAgent) Health(_ context.Context) agent.HealthStatus {
	return agent.HealthStatus{
		AgentID: a.id, AgentName: a.name,
		Healthy: true, CheckedAt: time.Now(),
	}
}

func (a *testAgent) Send(_ context.Context, _ string) (agent.Response, error) {
	return agent.Response{Content: "[]"}, nil
}

func (a *testAgent) SendStream(_ context.Context, _ string) (<-chan agent.StreamChunk, error) {
	ch := make(chan agent.StreamChunk, 1)
	ch <- agent.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (a *testAgent) SendWithAttachments(_ context.Context, _ string, _ []agent.Attachment) (agent.Response, error) {
	return agent.Response{}, nil
}

func (a *testAgent) OutputDir() string { return "/tmp/agent" }

func (a *testAgent) Capabilities() agent.AgentCapabilities {
	return agent.AgentCapabilities{Vision: true, Streaming: true, MaxTokens: 100000}
}

func (a *testAgent) SupportsVision() bool { return true }

func (a *testAgent) ModelInfo() agent.ModelInfo {
	return agent.ModelInfo{ID: "model-1", Provider: "test"}
}

// testAnalyzer implements analyzer.Analyzer for testing.
type testAnalyzer struct{}

func (t *testAnalyzer) AnalyzeScreen(_ context.Context, _ []byte) (analyzer.ScreenAnalysis, error) {
	return analyzer.ScreenAnalysis{ScreenID: "main", Title: "Main"}, nil
}

func (t *testAnalyzer) CompareScreens(_ context.Context, _, _ []byte) (analyzer.ScreenDiff, error) {
	return analyzer.ScreenDiff{}, nil
}

func (t *testAnalyzer) DetectElements(_ []byte) ([]analyzer.UIElement, error) {
	return nil, nil
}

func (t *testAnalyzer) DetectText(_ []byte) ([]analyzer.TextRegion, error) {
	return nil, nil
}

func (t *testAnalyzer) IdentifyScreen(_ context.Context, _ []byte) (analyzer.ScreenIdentity, error) {
	return analyzer.ScreenIdentity{}, nil
}

func (t *testAnalyzer) DetectIssues(_ context.Context, _ []byte) ([]analyzer.VisualIssue, error) {
	return nil, nil
}

func TestDefaultSessionConfig(t *testing.T) {
	cfg := DefaultSessionConfig()
	assert.NotEmpty(t, cfg.SessionID)
	assert.Equal(t, "qa-results", cfg.OutputDir)
	assert.Len(t, cfg.Platforms, 3)
	assert.Equal(t, 2*time.Hour, cfg.Timeout)
	assert.Equal(t, 0.90, cfg.CoverageTarget)
	assert.True(t, cfg.CuriosityEnabled)
	assert.Equal(t, 30*time.Minute, cfg.CuriosityTimeout)
}

func TestNewSessionCoordinator(t *testing.T) {
	cfg := DefaultSessionConfig()
	pool := agent.NewPool()
	viz := &testAnalyzer{}
	fm := feature.NewFeatureMap("/tmp/project")
	cov := coverage.NewTracker()

	sc := NewSessionCoordinator(cfg, pool, viz, fm, cov)
	assert.NotNil(t, sc)
	assert.Equal(t, StatusIdle, sc.Status())
	assert.NotNil(t, sc.PhaseManager())
	assert.NotNil(t, sc.Session())
}

func TestSessionCoordinator_Pause_NotRunning(t *testing.T) {
	cfg := DefaultSessionConfig()
	sc := NewSessionCoordinator(
		cfg, agent.NewPool(), &testAnalyzer{},
		feature.NewFeatureMap(""), coverage.NewTracker(),
	)
	err := sc.Pause(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot pause")
}

func TestSessionCoordinator_Resume_NotPaused(t *testing.T) {
	cfg := DefaultSessionConfig()
	sc := NewSessionCoordinator(
		cfg, agent.NewPool(), &testAnalyzer{},
		feature.NewFeatureMap(""), coverage.NewTracker(),
	)
	err := sc.Resume(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot resume")
}

func TestSessionCoordinator_Cancel(t *testing.T) {
	cfg := DefaultSessionConfig()
	sc := NewSessionCoordinator(
		cfg, agent.NewPool(), &testAnalyzer{},
		feature.NewFeatureMap(""), coverage.NewTracker(),
	)
	err := sc.Cancel(context.Background())
	require.NoError(t, err)
	assert.Equal(t, StatusCanceled, sc.Status())
}

func TestSessionCoordinator_Progress_Idle(t *testing.T) {
	cfg := DefaultSessionConfig()
	sc := NewSessionCoordinator(
		cfg, agent.NewPool(), &testAnalyzer{},
		feature.NewFeatureMap(""), coverage.NewTracker(),
	)

	progress := sc.Progress()
	assert.Equal(t, StatusIdle, progress.Status)
	assert.Equal(t, cfg.SessionID, progress.SessionID)
	assert.Equal(t, 0.0, progress.OverallProgress)
}

func TestSessionCoordinator_Run_WithAgents(t *testing.T) {
	cfg := DefaultSessionConfig()
	cfg.Platforms = []string{"android"}
	cfg.Timeout = 5 * time.Second
	cfg.CuriosityEnabled = false
	cfg.CuriosityTimeout = 100 * time.Millisecond

	pool := agent.NewPool()
	ag := newTestAgent("agent-1", "claude")
	require.NoError(t, pool.Register(ag))

	fm := feature.NewFeatureMap("/tmp")
	fm.AddFeature(feature.Feature{
		ID:        "feat-test",
		Name:      "Test Feature",
		Platforms: []string{"android"},
		TestSteps: []feature.TestStep{
			{Order: 1, Action: "click button", Expected: "screen changes"},
		},
	})

	cov := coverage.NewTracker()
	cov.RegisterFeature(
		coverage.Feature{ID: "feat-test", Name: "Test Feature"},
		[]string{"android"},
	)

	sc := NewSessionCoordinator(
		cfg, pool, &testAnalyzer{}, fm, cov,
	)

	result, err := sc.Run(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, StatusComplete, result.Status)
	assert.NotEmpty(t, result.SessionID)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.NotEmpty(t, result.Phases)
	assert.NotNil(t, result.PlatformResults["android"])
}

func TestSessionCoordinator_Run_NoAgents(t *testing.T) {
	cfg := DefaultSessionConfig()
	cfg.Platforms = []string{"android"}
	cfg.Timeout = 2 * time.Second

	pool := agent.NewPool()
	// No agents registered — Acquire will block until timeout.

	sc := NewSessionCoordinator(
		cfg, pool, &testAnalyzer{},
		feature.NewFeatureMap(""), coverage.NewTracker(),
	)

	_, err := sc.Run(context.Background())
	assert.Error(t, err)
	assert.Equal(t, StatusFailed, sc.Status())
}

func TestSessionCoordinator_Run_AlreadyRunning(t *testing.T) {
	cfg := DefaultSessionConfig()
	cfg.Timeout = 100 * time.Millisecond

	sc := NewSessionCoordinator(
		cfg, agent.NewPool(), &testAnalyzer{},
		feature.NewFeatureMap(""), coverage.NewTracker(),
	)

	// Manually set status.
	sc.mu.Lock()
	sc.status = StatusRunning
	sc.mu.Unlock()

	_, err := sc.Run(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "running")
}

func TestSessionCoordinator_Run_SkipCuriosity(t *testing.T) {
	cfg := DefaultSessionConfig()
	cfg.Platforms = []string{"desktop"}
	cfg.Timeout = 5 * time.Second
	cfg.CuriosityEnabled = false

	pool := agent.NewPool()
	require.NoError(t, pool.Register(newTestAgent("a1", "claude")))

	sc := NewSessionCoordinator(
		cfg, pool, &testAnalyzer{},
		feature.NewFeatureMap(""), coverage.NewTracker(),
	)

	result, err := sc.Run(context.Background())
	require.NoError(t, err)

	// Curiosity phase should be skipped.
	for _, p := range result.Phases {
		if p.Name == "curiosity" {
			assert.Equal(t, PhaseSkipped, p.Status)
		}
	}
}

func TestSessionStatus_Constants(t *testing.T) {
	assert.Equal(t, SessionStatus("idle"), StatusIdle)
	assert.Equal(t, SessionStatus("running"), StatusRunning)
	assert.Equal(t, SessionStatus("paused"), StatusPaused)
	assert.Equal(t, SessionStatus("complete"), StatusComplete)
	assert.Equal(t, SessionStatus("failed"), StatusFailed)
	assert.Equal(t, SessionStatus("canceled"), StatusCanceled)
}

func TestNoopExecutor(t *testing.T) {
	e := &noopExecutor{}
	ctx := context.Background()

	assert.NoError(t, e.Click(ctx, 0, 0))
	assert.NoError(t, e.Type(ctx, "test"))
	assert.NoError(t, e.Scroll(ctx, "down", 100))
	assert.NoError(t, e.LongPress(ctx, 0, 0))
	assert.NoError(t, e.Swipe(ctx, 0, 0, 100, 100))
	assert.NoError(t, e.KeyPress(ctx, "Enter"))
	assert.NoError(t, e.Back(ctx))
	assert.NoError(t, e.Home(ctx))

	data, err := e.Screenshot(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
}
