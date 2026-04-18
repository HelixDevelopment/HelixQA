// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"fmt"
	"sync"
	"time"

	"digital.vasic.llmorchestrator/pkg/agent"
	"digital.vasic.visionengine/pkg/analyzer"

	"digital.vasic.docprocessor/pkg/coverage"
	"digital.vasic.docprocessor/pkg/feature"

	"digital.vasic.helixqa/pkg/session"
)

// SessionConfig holds the configuration for an autonomous QA
// session.
type SessionConfig struct {
	// SessionID is a unique identifier for this session.
	SessionID string

	// OutputDir is where results and evidence are stored.
	OutputDir string

	// Platforms lists the platforms to test.
	Platforms []string

	// Timeout is the maximum session duration.
	Timeout time.Duration

	// CoverageTarget is the desired coverage (0-1).
	CoverageTarget float64

	// CuriosityEnabled enables the curiosity-driven phase.
	CuriosityEnabled bool

	// CuriosityTimeout limits the curiosity phase duration.
	CuriosityTimeout time.Duration
}

// DefaultSessionConfig returns a SessionConfig with sensible defaults.
func DefaultSessionConfig() *SessionConfig {
	return &SessionConfig{
		SessionID:        fmt.Sprintf("helix-%d", time.Now().Unix()),
		OutputDir:        "qa-results",
		Platforms:        []string{"android", "desktop", "web"},
		Timeout:          2 * time.Hour,
		CoverageTarget:   0.90,
		CuriosityEnabled: true,
		CuriosityTimeout: 30 * time.Minute,
	}
}

// SessionCoordinator manages the full lifecycle of an
// autonomous QA session across multiple platforms.
type SessionCoordinator struct {
	config          *SessionConfig
	orchestrator    agent.AgentPool
	visionEngine    analyzer.Analyzer
	featureMap      *feature.FeatureMap
	executorFactory ExecutorFactory
	workers         map[string]*PlatformWorker
	phaseManager    *PhaseManager
	session         *session.SessionRecorder
	coverage        coverage.CoverageTracker
	status          SessionStatus
	mu              sync.Mutex
}

// SessionOption configures a SessionCoordinator.
type SessionOption func(*SessionCoordinator)

// WithExecutorFactory sets the executor factory used to create
// platform-specific ActionExecutors during setup. If not set,
// a NoopExecutorFactory is used (suitable for testing).
func WithExecutorFactory(f ExecutorFactory) SessionOption {
	return func(sc *SessionCoordinator) {
		sc.executorFactory = f
	}
}

// NewSessionCoordinator creates a SessionCoordinator with
// the given configuration and dependencies. Use SessionOption
// values to inject an ExecutorFactory for real platform
// interaction.
func NewSessionCoordinator(
	cfg *SessionConfig,
	pool agent.AgentPool,
	viz analyzer.Analyzer,
	fm *feature.FeatureMap,
	cov coverage.CoverageTracker,
	opts ...SessionOption,
) *SessionCoordinator {
	sc := &SessionCoordinator{
		config:          cfg,
		orchestrator:    pool,
		visionEngine:    viz,
		featureMap:      fm,
		executorFactory: &NoopExecutorFactory{},
		workers:         make(map[string]*PlatformWorker),
		phaseManager:    NewPhaseManager(),
		session: session.NewSessionRecorder(
			cfg.SessionID, cfg.OutputDir,
		),
		coverage: cov,
		status:   StatusIdle,
	}
	for _, opt := range opts {
		opt(sc)
	}
	return sc
}

// Run executes the full session lifecycle through all 4 phases.
func (sc *SessionCoordinator) Run(
	ctx context.Context,
) (*SessionResult, error) {
	sc.mu.Lock()
	if sc.status != StatusIdle {
		sc.mu.Unlock()
		return nil, fmt.Errorf(
			"session is %s, expected idle", sc.status,
		)
	}
	sc.status = StatusRunning
	sc.mu.Unlock()

	result := &SessionResult{
		SessionID:       sc.config.SessionID,
		Status:          StatusRunning,
		StartTime:       time.Now(),
		PlatformResults: make(map[string]*PlatformResult),
	}

	ctx, cancel := context.WithTimeout(ctx, sc.config.Timeout)
	defer cancel()

	// Phase 1: Setup.
	if err := sc.runSetup(ctx); err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		sc.finalize(result)
		return result, err
	}

	// Phase 2: Doc-Driven Verification.
	if err := sc.runDocDriven(ctx, result); err != nil {
		result.Error = err.Error()
		// Continue to report phase even on failure.
	}

	// Phase 3: Curiosity-Driven Exploration.
	if sc.config.CuriosityEnabled {
		if err := sc.runCuriosityDriven(ctx, result); err != nil {
			result.Error = err.Error()
		}
	} else {
		_ = sc.phaseManager.Skip("curiosity")
	}

	// Phase 4: Report & Cleanup.
	sc.runReport(ctx, result)

	sc.finalize(result)
	return result, nil
}

// Pause pauses the session.
func (sc *SessionCoordinator) Pause(_ context.Context) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.status != StatusRunning {
		return fmt.Errorf("cannot pause: status is %s", sc.status)
	}
	sc.status = StatusPaused
	return nil
}

// Resume resumes a paused session.
func (sc *SessionCoordinator) Resume(_ context.Context) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.status != StatusPaused {
		return fmt.Errorf("cannot resume: status is %s", sc.status)
	}
	sc.status = StatusRunning
	return nil
}

// Cancel cancels the session.
func (sc *SessionCoordinator) Cancel(_ context.Context) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.status = StatusCanceled
	return nil
}

// Status returns the current session status.
func (sc *SessionCoordinator) Status() SessionStatus {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.status
}

// Progress returns a real-time progress report.
func (sc *SessionCoordinator) Progress() ProgressReport {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	current := sc.phaseManager.Current()
	phases := sc.phaseManager.All()

	completed := 0
	for _, p := range phases {
		if p.Status == PhaseCompleted || p.Status == PhaseSkipped {
			completed++
		}
	}

	platformStatus := make(map[string]string)
	for platform, w := range sc.workers {
		platformStatus[platform] = fmt.Sprintf(
			"%d issues", w.IssueDetector().IssueCount(),
		)
	}

	totalIssues := 0
	for _, w := range sc.workers {
		totalIssues += w.IssueDetector().IssueCount()
	}

	overall := 0.0
	if len(phases) > 0 {
		overall = float64(completed) / float64(len(phases))
	}

	return ProgressReport{
		SessionID:       sc.config.SessionID,
		Status:          sc.status,
		CurrentPhase:    current.Name,
		PhaseProgress:   current.Progress,
		OverallProgress: overall,
		PlatformStatus:  platformStatus,
		IssuesFound:     totalIssues,
	}
}

// PhaseManager returns the phase manager.
func (sc *SessionCoordinator) PhaseManager() *PhaseManager {
	return sc.phaseManager
}

// Session returns the session recorder.
func (sc *SessionCoordinator) Session() *session.SessionRecorder {
	return sc.session
}

// runSetup initializes workers for each platform.
func (sc *SessionCoordinator) runSetup(
	ctx context.Context,
) error {
	if err := sc.phaseManager.Start("setup"); err != nil {
		return err
	}

	for _, platform := range sc.config.Platforms {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Acquire an agent from the pool.
		ag, err := sc.orchestrator.Acquire(ctx, agent.AgentRequirements{
			NeedsVision: true,
		})
		if err != nil {
			_ = sc.phaseManager.Fail("setup", err)
			return fmt.Errorf(
				"acquire agent for %s: %w", platform, err,
			)
		}

		// Create platform-specific executor via the factory.
		exec, err := sc.executorFactory.Create(platform)
		if err != nil {
			_ = sc.phaseManager.Fail("setup", err)
			return fmt.Errorf(
				"create executor for %s: %w", platform, err,
			)
		}

		worker := NewPlatformWorker(PlatformWorkerConfig{
			Platform: platform,
			Agent:    ag,
			Analyzer: sc.visionEngine,
			Executor: exec,
			Coverage: sc.coverage,
			Session:  sc.session,
		})

		sc.mu.Lock()
		sc.workers[platform] = worker
		sc.mu.Unlock()

		// Start video recording.
		_ = sc.session.StartRecording(platform)
	}

	return sc.phaseManager.Complete("setup")
}

// runDocDriven runs the document-driven verification phase.
func (sc *SessionCoordinator) runDocDriven(
	ctx context.Context,
	result *SessionResult,
) error {
	if err := sc.phaseManager.Start("doc-driven"); err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for platform, worker := range sc.workers {
		features := sc.featureMap.FeaturesForPlatform(platform)
		if len(features) == 0 {
			continue
		}

		wg.Add(1)
		go func(p string, w *PlatformWorker, feats []feature.Feature) {
			defer wg.Done()

			stepResults, err := w.RunDocDriven(ctx, feats)

			mu.Lock()
			defer mu.Unlock()

			pr := &PlatformResult{
				Platform:          p,
				IssuesFound:       w.IssueDetector().IssueCount(),
				ScreensDiscovered: len(w.NavGraph().Screens()),
				Coverage:          w.NavGraph().Coverage(),
			}

			for _, sr := range stepResults {
				if sr.Success {
					pr.FeaturesVerified++
				} else {
					pr.FeaturesFailed++
				}
			}

			result.PlatformResults[p] = pr

			if err != nil && firstErr == nil {
				firstErr = err
			}
		}(platform, worker, features)
	}

	wg.Wait()

	if firstErr != nil {
		_ = sc.phaseManager.Fail("doc-driven", firstErr)
		return firstErr
	}
	return sc.phaseManager.Complete("doc-driven")
}

// runCuriosityDriven runs the curiosity-driven exploration phase.
func (sc *SessionCoordinator) runCuriosityDriven(
	ctx context.Context,
	result *SessionResult,
) error {
	if err := sc.phaseManager.Start("curiosity"); err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, worker := range sc.workers {
		wg.Add(1)
		go func(w *PlatformWorker) {
			defer wg.Done()
			_, _ = w.RunCuriosityDriven(
				ctx, sc.config.CuriosityTimeout,
			)
		}(worker)
	}

	wg.Wait()
	return sc.phaseManager.Complete("curiosity")
}

// runReport generates the final report.
func (sc *SessionCoordinator) runReport(
	_ context.Context,
	result *SessionResult,
) {
	_ = sc.phaseManager.Start("report")

	// Stop all recordings.
	for _, platform := range sc.config.Platforms {
		_, _ = sc.session.StopRecording(platform)
	}

	// Collect all issues.
	for _, w := range sc.workers {
		result.Issues = append(result.Issues, w.IssueDetector().Issues()...)
	}

	result.Timeline = sc.session.ExportTimeline()
	result.Phases = sc.phaseManager.All()

	if sc.coverage != nil {
		report := sc.coverage.Coverage()
		result.CoverageOverall = report.OverallPct
	}

	_ = sc.phaseManager.Complete("report")
}

// finalize completes the session result.
func (sc *SessionCoordinator) finalize(result *SessionResult) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if result.Error != "" {
		result.Status = StatusFailed
		sc.status = StatusFailed
	} else {
		result.Status = StatusComplete
		sc.status = StatusComplete
	}
}

// noopExecutor is a no-op ActionExecutor used as a placeholder.
type noopExecutor struct{}

func (n *noopExecutor) Click(_ context.Context, _, _ int) error { return nil }
func (n *noopExecutor) Type(_ context.Context, _ string) error  { return nil }
func (n *noopExecutor) Clear(_ context.Context) error           { return nil }
func (n *noopExecutor) Scroll(_ context.Context, _ string, _ int) error {
	return nil
}
func (n *noopExecutor) LongPress(_ context.Context, _, _ int) error { return nil }
func (n *noopExecutor) Swipe(_ context.Context, _, _, _, _ int) error {
	return nil
}
func (n *noopExecutor) KeyPress(_ context.Context, _ string) error { return nil }
func (n *noopExecutor) Back(_ context.Context) error               { return nil }
func (n *noopExecutor) Home(_ context.Context) error               { return nil }
func (n *noopExecutor) Screenshot(_ context.Context) ([]byte, error) {
	return []byte("mock-screenshot"), nil
}
