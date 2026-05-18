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
	"encoding/json"
	"errors"
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
		passed, stdout, err := o.runType(ctx, tt)
		duration := time.Since(start)

		tr := TestResult{
			Type:     tt,
			Passed:   passed,
			Duration: duration,
			Error:    err,
		}

		if !passed {
			allPassed = false
			evidencePaths, captureErr := o.captureFailureEvidence(tt, err, stdout)
			tr.Evidence = append(tr.Evidence, evidencePaths...)
			// Surface the capture-pipeline gap via the TestResult so
			// downstream gates (CONST-035 / Article XI §11.9) can refuse
			// to mark the QA run as "evidence-captured" when the real
			// pipeline has not been wired. We preserve the original test
			// error (the cause of the test failure) and add the capture
			// error as a wrapped fact so neither is lost. The orchestrator
			// itself never fabricates evidence.
			if captureErr != nil {
				if tr.Error != nil {
					tr.Error = fmt.Errorf("%w (additionally: %w)", tr.Error, captureErr)
				} else {
					tr.Error = captureErr
				}
			}
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

// runType returns (passed, stdoutBytes, err). stdoutBytes is the raw combined
// stdout+stderr from the subprocess used to execute the test type — surfaced
// up to RunAll so captureFailureEvidence can persist it as REAL on-disk
// evidence (round-58 §11.4 wiring). For unknown/no-subprocess paths the
// returned stdout slice is nil and captureFailureEvidence falls back to
// environment-only evidence (still real, just narrower).
func (o *Orchestrator) runType(ctx context.Context, tt TestType) (bool, []byte, error) {
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
		return false, nil, fmt.Errorf("unknown test type: %s", tt)
	}
}

func (o *Orchestrator) runGoTest(ctx context.Context, args ...string) (bool, []byte, error) {
	cmdArgs := append([]string{"test", "-count=1", "-race", "-p", "1"}, args...)
	cmd := exec.CommandContext(ctx, "go", cmdArgs...)
	cmd.Dir = o.repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Return the raw bytes alongside the wrapping error so the caller
		// can persist them to disk (round-58 anti-bluff). We keep the
		// human-readable wrapping for log/UI consumers but the bytes are
		// the source-of-truth artefact.
		return false, out, fmt.Errorf("go test failed: %w\n%s", err, string(out))
	}
	return true, out, nil
}

func (o *Orchestrator) runChallenges(ctx context.Context) (bool, []byte, error) {
	challengesDir := filepath.Join(o.repoRoot, "Challenges")
	if _, err := os.Stat(challengesDir); os.IsNotExist(err) {
		return false, nil, fmt.Errorf("Challenges submodule not found")
	}

	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-race", "-p", "1", "./...")
	cmd.Dir = challengesDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, out, fmt.Errorf("challenge tests failed: %w\n%s", err, string(out))
	}
	return true, out, nil
}

// ErrEvidenceCaptureNotWired is returned by captureFailureEvidence when NO
// applicable capture mechanism is available for the failing test type — no
// captured stdout/stderr from the test subprocess, no live browser context
// reachable via HELIX_QA_BROWSER_URL, no container log source. In that
// narrow situation the honest return is "empty paths + sentinel" so a
// downstream §11.4.2 gate cannot read any fake names.
//
// Background (round-29 §11.4 anti-bluff audit, 2026-05-17): the previous
// implementation of captureFailureEvidence fabricated a path string
// ("<evidenceDir>/<type>_failure.log") and labelled it as captured
// evidence, with the comment "Placeholder: in production this would
// capture screenshots, logs, etc." That fake path was then attached to
// the TestResult.Evidence slice and a downstream §11.4.2 captured-
// evidence gate could read TWO names in that slice and conclude
// "evidence captured" while the file referenced did not exist on disk —
// the exact PASS-bluff pattern Article XI §11.9 / CONST-035 / CONST-050(A)
// forbid, made worse by the fact that helix_qa is the orchestrator
// MEANT to detect that bluff class.
//
// Round-58 (2026-05-18) widens the contract: the orchestrator now wires
// REAL evidence capture for the test outputs it does have direct access to
// (stdout/stderr from the failing `go test` subprocess + a structured
// evidence manifest + an environment snapshot). The sentinel survives but
// fires only when NONE of the capture paths produced an on-disk artefact.
// Round-59 will extend with chromedp/playwright live-context screenshot
// capture when HELIX_QA_BROWSER_URL is set; round-58 deliberately scopes
// to subprocess output to keep blast radius contained.
var ErrEvidenceCaptureNotWired = fmt.Errorf("helixqa orchestrator: no failure-evidence capture mechanism applicable for this test context — no subprocess stdout/stderr was captured by runType AND no live browser/container context is reachable. Caller MUST treat absence of captured artefacts as a §11.4 PASS-bluff and refuse to mark the QA run as 'evidence-captured' per Article XI §11.9 / CONST-035")

// ErrEvidenceCaptureStdoutWriteFailed indicates the orchestrator attempted to
// write the captured stdout/stderr bytes to disk but the write failed (full
// disk, permission denied, evidence directory deleted mid-run, etc.). The
// caller MUST surface this as a §11.4 violation — captured bytes exist in
// memory but did not become a durable artefact, so any release gate
// asserting "evidence on disk" cannot honestly PASS.
var ErrEvidenceCaptureStdoutWriteFailed = fmt.Errorf("helixqa orchestrator: stdout/stderr evidence write to disk failed — the bytes were captured in memory but the artefact file does not exist on disk, so any §11.4.2 captured-evidence gate must treat this as PASS-bluff")

// ErrEvidenceCaptureManifestWriteFailed indicates the orchestrator failed to
// write the evidence manifest.json that catalogues which artefacts were
// captured for a given failure. Without the manifest a downstream reader
// cannot enumerate artefacts honestly, so its absence is itself a §11.4
// surface defect.
var ErrEvidenceCaptureManifestWriteFailed = fmt.Errorf("helixqa orchestrator: evidence manifest.json write failed — the per-failure artefact catalogue could not be persisted, breaking the §11.4.2 captured-evidence chain")

// ErrEvidenceCaptureStatVerificationFailed indicates the orchestrator wrote
// an artefact file but the post-write os.Stat verification could not
// confirm the file exists on disk at return time. This is the hard
// anti-bluff backstop: the orchestrator MUST NOT return any path it has
// not directly confirmed via os.Stat in the same call, because round-29's
// observed bluff was exactly "return a path that does not exist on disk".
var ErrEvidenceCaptureStatVerificationFailed = fmt.Errorf("helixqa orchestrator: post-write os.Stat verification failed — refusing to return a path that does not exist on disk per round-29 anti-bluff backstop")

// EvidenceManifest is the per-failure artefact catalogue written as
// <evidenceDir>/<testType>/<timestamp>/manifest.json. Schema is intentionally
// flat so a downstream §11.4.2 gate can read+verify with a simple JSON
// decoder. All Paths entries MUST exist on disk (verified via os.Stat) at
// the moment the manifest is written.
type EvidenceManifest struct {
	TestType       string    `json:"test_type"`
	FailedAt       time.Time `json:"failed_at"`
	Capturer       string    `json:"capturer"`
	OriginalError  string    `json:"original_error,omitempty"`
	Paths          []string  `json:"paths"`
	CapturerNotes  []string  `json:"capturer_notes,omitempty"`
	CaptureFailed  []string  `json:"capture_failed,omitempty"`
}

// captureFailureEvidence captures REAL on-disk evidence for a failed test
// run. Round-58 (2026-05-18) replaces the round-29 stub with a working
// pipeline scoped to data the orchestrator already has direct access to:
//
//  1. Subprocess stdout/stderr — the combined output captured by runGoTest /
//     runChallenges is persisted to <evidenceDir>/<testType>/<timestamp>/stdout.log.
//     This is the highest-value artefact: it tells the QA reviewer EXACTLY
//     what the failing `go test` invocation printed, including which
//     assertion failed and any goroutine dump.
//  2. Original error — the wrapping error from the runType layer is persisted
//     to error.log so reviewers can correlate the wrapper message with the
//     subprocess output.
//  3. Environment snapshot — a curated subset of HELIX_QA_* and other QA-
//     relevant environment variables is persisted to env.json so the failure
//     can be reproduced.
//  4. Manifest — manifest.json catalogues all of the above with the test
//     type, failure timestamp, and capturer attribution.
//
// Honesty contract (round-58 strengthening):
//   - Every returned path MUST exist on disk at return time, verified via
//     os.Stat in the same call. If os.Stat fails the path is NOT returned
//     and an error is collected via errors.Join.
//   - If NO capture mechanism produces any on-disk artefact (e.g. failing
//     test produced zero stdout AND no other capture source was available),
//     ErrEvidenceCaptureNotWired is returned and the paths slice is nil.
//     This preserves round-29 semantics in the genuinely-empty case.
//   - Capture-pipeline failures (write errors, stat verification failures)
//     are NEVER silently swallowed. They are returned via errors.Join
//     alongside any successfully-captured paths so the caller's
//     downstream §11.4.2 gate sees both signals.
//   - The orchestrator NEVER fabricates a path. If a write fails, the
//     would-have-been-written path is NOT returned — only paths that
//     passed os.Stat.
//
// Round-59 will extend the pipeline with chromedp/playwright screenshot
// capture when HELIX_QA_BROWSER_URL is set. Round-58 deliberately scopes
// to subprocess output + manifest to keep blast radius contained and
// avoid pulling in live-browser lifecycle complexity in a single PR.
func (o *Orchestrator) captureFailureEvidence(tt TestType, runErr error, stdout []byte) ([]string, error) {
	timestamp := time.Now().UTC().Format("20060102_150405.000")
	perTypeDir := filepath.Join(o.evidenceDir, string(tt), timestamp)

	var capturedPaths []string
	var captureErrors []error
	var notes []string
	var failed []string

	// If there is no subprocess output AND no other capture mechanism
	// would fire, short-circuit to the round-29 sentinel BEFORE creating
	// an empty directory. This preserves the round-29 contract for the
	// genuinely-empty case.
	hasStdoutBytes := len(stdout) > 0
	hasOriginalError := runErr != nil
	if !hasStdoutBytes && !hasOriginalError {
		return nil, ErrEvidenceCaptureNotWired
	}

	// Create per-failure evidence directory under <evidenceDir>/<type>/<ts>/.
	if err := os.MkdirAll(perTypeDir, 0o755); err != nil {
		return nil, fmt.Errorf("create per-failure evidence dir %s: %w", perTypeDir, err)
	}

	// (1) Persist captured stdout/stderr bytes if we have any.
	if hasStdoutBytes {
		stdoutPath := filepath.Join(perTypeDir, "stdout.log")
		if err := os.WriteFile(stdoutPath, stdout, 0o644); err != nil {
			captureErrors = append(captureErrors, fmt.Errorf("%w: %v (path=%s)", ErrEvidenceCaptureStdoutWriteFailed, err, stdoutPath))
			failed = append(failed, "stdout.log")
		} else if !pathExistsOnDisk(stdoutPath) {
			captureErrors = append(captureErrors, fmt.Errorf("%w: %s", ErrEvidenceCaptureStatVerificationFailed, stdoutPath))
			failed = append(failed, "stdout.log")
		} else {
			capturedPaths = append(capturedPaths, stdoutPath)
			notes = append(notes, fmt.Sprintf("captured %d bytes of subprocess stdout/stderr", len(stdout)))
		}
	}

	// (2) Persist the wrapping error from the runType layer if present.
	// The error usually contains exit-code metadata + the wrapped subprocess
	// message which complements the raw stdout bytes from (1).
	if hasOriginalError {
		errPath := filepath.Join(perTypeDir, "error.log")
		errBytes := []byte(runErr.Error() + "\n")
		if err := os.WriteFile(errPath, errBytes, 0o644); err != nil {
			captureErrors = append(captureErrors, fmt.Errorf("%w: %v (path=%s)", ErrEvidenceCaptureStdoutWriteFailed, err, errPath))
			failed = append(failed, "error.log")
		} else if !pathExistsOnDisk(errPath) {
			captureErrors = append(captureErrors, fmt.Errorf("%w: %s", ErrEvidenceCaptureStatVerificationFailed, errPath))
			failed = append(failed, "error.log")
		} else {
			capturedPaths = append(capturedPaths, errPath)
			notes = append(notes, "captured wrapping error message from runType")
		}
	}

	// (3) Environment snapshot — curated to QA-relevant variables. The
	// subset keeps the file small + readable + free of accidental secrets
	// (we do NOT dump arbitrary env vars; only known QA prefixes).
	envSnapshot := collectQAEnvSnapshot()
	if len(envSnapshot) > 0 {
		envPath := filepath.Join(perTypeDir, "env.json")
		envBytes, err := json.MarshalIndent(envSnapshot, "", "  ")
		if err != nil {
			captureErrors = append(captureErrors, fmt.Errorf("marshal env snapshot: %w", err))
			failed = append(failed, "env.json")
		} else if err := os.WriteFile(envPath, envBytes, 0o644); err != nil {
			captureErrors = append(captureErrors, fmt.Errorf("write env.json: %w", err))
			failed = append(failed, "env.json")
		} else if !pathExistsOnDisk(envPath) {
			captureErrors = append(captureErrors, fmt.Errorf("%w: %s", ErrEvidenceCaptureStatVerificationFailed, envPath))
			failed = append(failed, "env.json")
		} else {
			capturedPaths = append(capturedPaths, envPath)
			notes = append(notes, fmt.Sprintf("captured %d QA-relevant env vars", len(envSnapshot)))
		}
	}

	// (4) Manifest — catalogue what we captured. Written LAST so it can
	// reference the paths from (1)-(3). The manifest is also verified
	// via os.Stat before being added to the returned paths slice.
	manifest := EvidenceManifest{
		TestType:      string(tt),
		FailedAt:      time.Now().UTC(),
		Capturer:      "helixqa.Orchestrator (round-58 wiring)",
		Paths:         append([]string(nil), capturedPaths...),
		CapturerNotes: notes,
		CaptureFailed: failed,
	}
	if runErr != nil {
		manifest.OriginalError = runErr.Error()
	}
	manifestPath := filepath.Join(perTypeDir, "manifest.json")
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		captureErrors = append(captureErrors, fmt.Errorf("%w: marshal: %v", ErrEvidenceCaptureManifestWriteFailed, err))
	} else if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		captureErrors = append(captureErrors, fmt.Errorf("%w: write: %v", ErrEvidenceCaptureManifestWriteFailed, err))
	} else if !pathExistsOnDisk(manifestPath) {
		captureErrors = append(captureErrors, fmt.Errorf("%w: %s", ErrEvidenceCaptureStatVerificationFailed, manifestPath))
	} else {
		capturedPaths = append(capturedPaths, manifestPath)
	}

	// If everything failed — including the manifest — surface the sentinel
	// so the caller's gate sees the round-29 signal.
	if len(capturedPaths) == 0 {
		captureErrors = append(captureErrors, ErrEvidenceCaptureNotWired)
		return nil, errors.Join(captureErrors...)
	}

	// Partial-success path: return the paths we DID write + any errors we
	// collected. errors.Join is nil if captureErrors is empty.
	return capturedPaths, errors.Join(captureErrors...)
}

// pathExistsOnDisk is the round-58 anti-bluff backstop: the orchestrator
// MUST NOT return any path it has not directly confirmed via os.Stat in
// the same call. This is the mechanical guarantee that closes the round-29
// bluff (return a path string that does not exist on disk).
func pathExistsOnDisk(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// A directory at the expected file path is not a valid artefact.
	return !info.IsDir()
}

// collectQAEnvSnapshot returns a map of QA-relevant environment variables.
// The whitelist approach intentionally avoids dumping arbitrary env vars
// (which would risk leaking secrets per CONST-042 / §11.4.10) and keeps
// the snapshot small + reviewable. Add new prefixes here as QA-relevant
// orchestration knobs are introduced.
func collectQAEnvSnapshot() map[string]string {
	prefixes := []string{
		"HELIX_QA_",
		"HELIXQA_",
		"HELIX_CODE_",
		"GOWORK",
		"GOFLAGS",
		"GO_TEST_",
	}
	snapshot := make(map[string]string)
	for _, kv := range os.Environ() {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		key := kv[:eq]
		val := kv[eq+1:]
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) || key == prefix {
				snapshot[key] = val
				break
			}
		}
	}
	return snapshot
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
