// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// definition_challenge_hxc011_test.go — HXC-011 regression suite.
//
// HXC-011: the helix_qa runner's `run` path, on the `desktop` platform,
// loaded a test-bank's cases but did NOT shell out to a case's
// `action:` shell command — it emitted a hollow sub-microsecond
// PASSED / SKIPPED metadata row instead of really executing the
// command. That is a §11.4 / CONST-035 PASS-bluff IN THE QA RUNNER
// ITSELF: a green (or honest-looking SKIP) line with no runtime
// evidence.
//
// These tests are RED-first per Rule 7 / §11.4.43 TDD-fix. They MUST
// FAIL against the pre-fix definitionChallenge (which unconditionally
// returned Status=Skipped and never ran any action), and MUST PASS
// after the fix wires real os/exec execution of desktop-platform
// shell actions into the runner.
//
// Anti-bluff posture (§11.4.69): the positive-evidence artefact is a
// sentinel file the bank action writes to disk. The test reads that
// file back — a hollow metadata-only PASS cannot produce it.
package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"digital.vasic.challenges/pkg/challenge"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/testbank"
)

// writeDesktopBank writes a HelixQA test-bank YAML to a temp file and
// returns its path. The bank has a single desktop-platform case whose
// step carries the supplied `shell:` action.
func writeDesktopBank(t *testing.T, id, name, shellAction string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hxc011.bank.yaml")
	yaml := "version: \"1.0\"\n" +
		"name: \"HXC-011 desktop bank\"\n" +
		"description: \"desktop-platform shell-action regression bank\"\n" +
		"test_cases:\n" +
		"  - id: " + id + "\n" +
		"    name: \"" + name + "\"\n" +
		"    category: functional\n" +
		"    priority: critical\n" +
		"    platforms: [desktop]\n" +
		"    steps:\n" +
		"      - name: \"run desktop shell action\"\n" +
		"        action: \"shell: " + shellAction + "\"\n" +
		"        expected: \"command runs and exits 0\"\n"
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0600))
	return path
}

// TestHXC011_DesktopShellAction_ActuallyExecutes is the primary RED
// test. It runs a desktop-platform bank case whose `shell:` action
// writes a sentinel file. The test asserts the sentinel file really
// exists on disk after the run — proof the runner shelled out.
//
// Pre-fix: definitionChallenge.Execute never ran the action, the
// sentinel file is never created, this test FAILS.
// Post-fix: the runner executes the action via os/exec, the file
// exists, this test PASSES.
func TestHXC011_DesktopShellAction_ActuallyExecutes(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "hxc011-sentinel.txt")
	// The action writes a known payload to the sentinel path.
	action := "printf HXC011-REAL-EXECUTION > " + sentinel
	bankPath := writeDesktopBank(t, "HXC-011-EXEC",
		"desktop shell action executes", action)

	cfg := &config.Config{
		Banks:         []string{bankPath},
		Platforms:     []config.Platform{config.PlatformDesktop},
		OutputDir:     t.TempDir(),
		Speed:         config.SpeedNormal,
		ReportFormat:  config.ReportMarkdown,
		ValidateSteps: false,
		Timeout:       2 * time.Minute,
		StepTimeout:   30 * time.Second,
	}
	orch := New(cfg)
	result, err := orch.Run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result.Report)
	require.Equal(t, 1, result.Report.TotalChallenges)

	// ANTI-BLUFF positive evidence: the sentinel file MUST exist and
	// carry the payload. A hollow metadata-only PASS/SKIP cannot
	// produce this artefact.
	data, readErr := os.ReadFile(sentinel)
	require.NoError(t, readErr,
		"HXC-011: desktop bank action did not execute — "+
			"sentinel file was never written (hollow PASS/SKIP bluff)")
	require.Equal(t, "HXC011-REAL-EXECUTION", string(data),
		"sentinel payload mismatch — action did not run correctly")

	// The case ran a real command that exited 0 — it MUST score PASS,
	// not a hollow SKIP.
	require.Equal(t, 1, result.Report.PassedChallenges,
		"successful desktop shell action must score PASS")
	require.Zero(t, result.Report.FailedChallenges)
}

// TestHXC011_DesktopFailingAction_ScoresFAIL proves the runner can no
// longer bluff: a desktop bank case whose `shell:` action exits
// non-zero MUST score FAIL, never PASS.
//
// Pre-fix: definitionChallenge.Execute never ran the action and
// returned Skipped (promoted toward Passed) — a deliberately-broken
// action produced a non-FAIL row. FAILS.
// Post-fix: the real exit code drives a FAIL verdict. PASSES.
func TestHXC011_DesktopFailingAction_ScoresFAIL(t *testing.T) {
	// `false` always exits 1; pre-fix this would not be FAIL.
	bankPath := writeDesktopBank(t, "HXC-011-FAIL",
		"desktop failing action scores FAIL",
		"exit 17")

	cfg := &config.Config{
		Banks:         []string{bankPath},
		Platforms:     []config.Platform{config.PlatformDesktop},
		OutputDir:     t.TempDir(),
		Speed:         config.SpeedNormal,
		ReportFormat:  config.ReportMarkdown,
		ValidateSteps: false,
		Timeout:       2 * time.Minute,
		StepTimeout:   30 * time.Second,
	}
	orch := New(cfg)
	result, err := orch.Run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result.Report)
	require.Equal(t, 1, result.Report.TotalChallenges)

	require.Equal(t, 1, result.Report.FailedChallenges,
		"HXC-011: a desktop shell action that exits non-zero MUST "+
			"score FAIL — the runner can no longer bluff a PASS")
	require.Zero(t, result.Report.PassedChallenges,
		"a failing action must NOT be counted as a pass")
}

// TestHXC011_DesktopShellAction_RealDuration proves the executed case
// carries a real (non-sub-microsecond) wall-clock duration. The
// forensic symptom of the HXC-011 bluff was a sub-µs PASSED row — a
// metadata-only result that never blocked on a real subprocess.
func TestHXC011_DesktopShellAction_RealDuration(t *testing.T) {
	// sleep 0.05s so a real subprocess produces a measurable duration.
	bankPath := writeDesktopBank(t, "HXC-011-DUR",
		"desktop action has real duration",
		"sleep 0.05")

	tc := loadSingleTestCase(t, bankPath)
	dc := newDefinitionChallengeForPlatform(
		tc.ToDefinition(), tc, config.PlatformDesktop)
	start := time.Now()
	res, err := dc.Execute(context.Background())
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, challenge.StatusPassed, res.Status)
	require.GreaterOrEqual(t, res.Duration, 40*time.Millisecond,
		"HXC-011: executed desktop action must carry a real "+
			"wall-clock duration, not a sub-µs metadata stamp")
	require.GreaterOrEqual(t, elapsed, 40*time.Millisecond)
}

// loadSingleTestCase loads a bank file and returns its single case.
func loadSingleTestCase(t *testing.T, path string) *testbank.TestCase {
	t.Helper()
	bf, err := testbank.LoadFile(path)
	require.NoError(t, err)
	require.Len(t, bf.TestCases, 1)
	return &bf.TestCases[0]
}
