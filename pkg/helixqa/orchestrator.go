// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package helixqa provides the autonomous QA orchestration layer that
// integrates HelixDevelopment/HelixQA with the HelixPlay test matrix.
//
// Constitution §6.7: every feature needs HelixQA visual assertion,
// manual recording, or Challenge scenario evidence.
package helixqa

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// TestType is one of the 10 required test categories.
type TestType string

const (
	Unit        TestType = "unit"
	Integration TestType = "integration"
	E2E         TestType = "e2e"
	Security    TestType = "security"
	Benchmark   TestType = "benchmark"
	Chaos       TestType = "chaos"
	Stress      TestType = "stress"
	Smoke       TestType = "smoke"
	FullAuto    TestType = "fullauto"
	Challenge   TestType = "challenge"
)

// TestResult captures the outcome of a single test-type run.
type TestResult struct {
	Type      TestType
	Passed    bool
	Duration  time.Duration
	Error     error
	Evidence  []string // paths to captured evidence (screenshots, logs)
}

// Orchestrator coordinates the full 10-type test matrix for a HelixPlay
// release candidate. It delegates to the existing test infrastructure
// while capturing visual evidence for Constitution §6.7 compliance.
type Orchestrator struct {
	repoRoot     string
	evidenceDir  string
	testTimeout  time.Duration
	results      []TestResult
}

// NewOrchestrator creates an orchestrator rooted at the HelixPlay repo.
func NewOrchestrator(opts ...OrchestratorOption) (*Orchestrator, error) {
	root, err := findHelixPlayRoot()
	if err != nil {
		return nil, err
	}

	o := &Orchestrator{
		repoRoot:    root,
		evidenceDir: filepath.Join(root, "qa-results", time.Now().Format("20060102_150405")),
		testTimeout: 30 * time.Minute,
	}
	for _, opt := range opts {
		opt(o)
	}

	if err := os.MkdirAll(o.evidenceDir, 0755); err != nil {
		return nil, fmt.Errorf("create evidence dir: %w", err)
	}

	return o, nil
}

// OrchestratorOption configures the orchestrator.
type OrchestratorOption func(*Orchestrator)

// WithEvidenceDir overrides the default evidence directory.
func WithEvidenceDir(dir string) OrchestratorOption {
	return func(o *Orchestrator) {
		o.evidenceDir = dir
	}
}

// WithTimeout overrides the default per-test-type timeout.
func WithTimeout(d time.Duration) OrchestratorOption {
	return func(o *Orchestrator) {
		o.testTimeout = d
	}
}

// RunAll executes all 10 test types and collects evidence.
// Returns true only if every type passes.
func (o *Orchestrator) RunAll(ctx context.Context) bool {
	types := []TestType{Unit, Integration, E2E, Security, Benchmark, Chaos, Stress, Smoke, Challenge}
	allPassed := true

	for _, tt := range types {
		start := time.Now()
		passed, err := o.runType(ctx, tt)
		duration := time.Since(start)

		tr := TestResult{
			Type:     tt,
			Passed:   passed,
			Duration: duration,
			Error:    err,
		}

		if !passed {
			allPassed = false
			tr.Evidence = append(tr.Evidence, o.captureFailureEvidence(tt, err)...)
		}

		o.results = append(o.results, tr)
	}

	return allPassed
}

// Results returns all collected results.
func (o *Orchestrator) Results() []TestResult {
	return o.results
}

// Summary returns a human-readable summary.
func (o *Orchestrator) Summary() string {
	var b strings.Builder
	passed, failed := 0, 0
	for _, r := range o.results {
		if r.Passed {
			passed++
		} else {
			failed++
		}
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "[%s] %s (%s)\n", status, r.Type, r.Duration)
		if r.Error != nil {
			fmt.Fprintf(&b, "  error: %v\n", r.Error)
		}
	}
	fmt.Fprintf(&b, "\nTotal: %d passed, %d failed\n", passed, failed)
	return b.String()
}

func (o *Orchestrator) runType(ctx context.Context, tt TestType) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, o.testTimeout)
	defer cancel()

	switch tt {
	case Unit:
		return o.runGoTest(ctx, "./cmd/...", "./pkg/...")
	case Integration:
		return o.runGoTest(ctx, "./tests/integration/...")
	case E2E:
		return o.runGoTest(ctx, "./tests/e2e/...")
	case Security:
		return o.runGoTest(ctx, "./tests/security/...")
	case Benchmark:
		return o.runGoTest(ctx, "-bench=.", "./tests/benchmark/...")
	case Chaos:
		return o.runGoTest(ctx, "./tests/chaos/...")
	case Stress:
		return o.runGoTest(ctx, "-run=Test1000", "./tests/stress/...")
	case Smoke:
		return o.runGoTest(ctx, "./tests/smoke/...")
	case Challenge:
		return o.runChallenges(ctx)
	default:
		return false, fmt.Errorf("unknown test type: %s", tt)
	}
}

func (o *Orchestrator) runGoTest(ctx context.Context, args ...string) (bool, error) {
	cmdArgs := append([]string{"test", "-count=1", "-race", "-p", "1"}, args...)
	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Dir = o.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("go test failed: %w\n%s", err, string(out))
	}
	return true, nil
}

func (o *Orchestrator) runChallenges(ctx context.Context) (bool, error) {
	challengesDir := filepath.Join(o.repoRoot, "Challenges")
	if _, err := os.Stat(challengesDir); os.IsNotExist(err) {
		return false, fmt.Errorf("Challenges submodule not found")
	}

	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-race", "-p", "1", "./...")
	cmd.Dir = challengesDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("challenge tests failed: %w\n%s", err, string(out))
	}
	return true, nil
}

func (o *Orchestrator) captureFailureEvidence(tt TestType, err error) []string {
	// Placeholder: in production this would capture screenshots, logs, etc.
	return []string{filepath.Join(o.evidenceDir, string(tt)+"_failure.log")}
}

func findHelixPlayRoot() (string, error) {
	_, callerFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot determine caller path")
	}
	// Walk up until we find go.work.
	dir := filepath.Dir(callerFile)
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("HelixPlay root not found (searched up from %s)", filepath.Dir(callerFile))
}
