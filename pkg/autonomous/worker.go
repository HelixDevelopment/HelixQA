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
	"digital.vasic.visionengine/pkg/graph"

	"digital.vasic.docprocessor/pkg/coverage"
	"digital.vasic.docprocessor/pkg/feature"

	"digital.vasic.helixqa/pkg/issuedetector"
	"digital.vasic.helixqa/pkg/navigator"
	"digital.vasic.helixqa/pkg/session"
)

// PlatformWorker executes both doc-driven and curiosity-driven
// testing phases for a single platform.
type PlatformWorker struct {
	platform      string
	agent         agent.Agent
	analyzer      analyzer.Analyzer
	navigator     *navigator.NavigationEngine
	issueDetector *issuedetector.IssueDetector
	coverage      coverage.CoverageTracker
	navGraph      graph.NavigationGraph
	session       *session.SessionRecorder
	executor      navigator.ActionExecutor
	mu            sync.Mutex
}

// PlatformWorkerConfig holds configuration for creating a
// PlatformWorker.
type PlatformWorkerConfig struct {
	Platform      string
	Agent         agent.Agent
	Analyzer      analyzer.Analyzer
	Executor      navigator.ActionExecutor
	Coverage      coverage.CoverageTracker
	Session       *session.SessionRecorder
	TicketGen     interface{} // *ticket.Generator
}

// NewPlatformWorker creates a PlatformWorker with all
// required dependencies.
func NewPlatformWorker(cfg PlatformWorkerConfig) *PlatformWorker {
	g := graph.NewNavigationGraph()
	nav := navigator.NewNavigationEngine(
		cfg.Agent, cfg.Analyzer, cfg.Executor, g,
	)
	id := issuedetector.NewIssueDetector(
		cfg.Agent, cfg.Analyzer, nil, cfg.Session,
	)

	return &PlatformWorker{
		platform:      cfg.Platform,
		agent:         cfg.Agent,
		analyzer:      cfg.Analyzer,
		navigator:     nav,
		issueDetector: id,
		coverage:      cfg.Coverage,
		navGraph:      g,
		session:       cfg.Session,
		executor:      cfg.Executor,
	}
}

// Platform returns the platform name.
func (pw *PlatformWorker) Platform() string {
	return pw.platform
}

// RunDocDriven verifies documented features by executing
// their test steps.
func (pw *PlatformWorker) RunDocDriven(
	ctx context.Context,
	features []feature.Feature,
) ([]StepResult, error) {
	var results []StepResult

	for _, feat := range features {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		pw.session.RecordEvent(session.TimelineEvent{
			Type:        session.EventAction,
			Platform:    pw.platform,
			Description: fmt.Sprintf("Verifying feature: %s", feat.Name),
			FeatureID:   feat.ID,
		})

		stepResults := pw.verifyFeature(ctx, feat)
		results = append(results, stepResults...)

		// Determine overall feature result.
		allPassed := true
		for _, sr := range stepResults {
			if !sr.Success {
				allPassed = false
				break
			}
		}

		if allPassed {
			pw.coverage.MarkVerified(
				feat.ID, pw.platform,
				coverage.Evidence{
					Timestamp:   time.Now(),
					Description: "Verified via autonomous QA",
				},
			)
		} else {
			pw.coverage.MarkFailed(
				feat.ID, pw.platform,
				coverage.Issue{
					Type:        "functional",
					Severity:    "medium",
					Title:       fmt.Sprintf("Feature %s failed", feat.Name),
					Description: "One or more test steps failed",
				},
			)
		}
	}
	return results, nil
}

// RunCuriosityDriven performs free exploration to discover
// untested areas and edge cases.
func (pw *PlatformWorker) RunCuriosityDriven(
	ctx context.Context,
	timeout time.Duration,
) ([]StepResult, error) {
	var results []StepResult
	deadline := time.After(timeout)

	pw.session.RecordEvent(session.TimelineEvent{
		Type:        session.EventPhaseChange,
		Platform:    pw.platform,
		Description: "Starting curiosity-driven exploration",
	})

	for {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		case <-deadline:
			return results, nil
		default:
		}

		exploreResult, err := pw.navigator.ExploreUnknown(ctx)
		if err != nil {
			results = append(results, StepResult{
				Action:  "explore",
				Success: false,
				Error:   err.Error(),
			})
			continue
		}

		results = append(results, StepResult{
			Action:  "explore",
			Success: true,
			Duration: exploreResult.Duration,
		})

		// Check coverage — stop early if high enough.
		if pw.navGraph.Coverage() >= 0.95 {
			break
		}
	}
	return results, nil
}

// IssueDetector returns the issue detector.
func (pw *PlatformWorker) IssueDetector() *issuedetector.IssueDetector {
	return pw.issueDetector
}

// Navigator returns the navigation engine.
func (pw *PlatformWorker) Navigator() *navigator.NavigationEngine {
	return pw.navigator
}

// NavGraph returns the navigation graph.
func (pw *PlatformWorker) NavGraph() graph.NavigationGraph {
	return pw.navGraph
}

// verifyFeature executes all test steps for a feature.
func (pw *PlatformWorker) verifyFeature(
	ctx context.Context,
	feat feature.Feature,
) []StepResult {
	var results []StepResult

	for i, step := range feat.TestSteps {
		select {
		case <-ctx.Done():
			return results
		default:
		}

		start := time.Now()
		action := analyzer.Action{
			Type:  "click",
			Value: step.Action,
		}

		result, err := pw.navigator.PerformAction(ctx, action)
		sr := StepResult{
			FeatureID: feat.ID,
			StepIndex: i,
			Action:    step.Action,
			Duration:  time.Since(start),
		}

		if err != nil {
			sr.Error = err.Error()
			sr.Success = false
		} else {
			sr.Success = result.Success
			if !result.Success {
				sr.Error = result.Error
			}
		}

		results = append(results, sr)
	}
	return results
}
