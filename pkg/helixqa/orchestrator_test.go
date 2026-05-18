// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package helixqa

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOrchestrator(t *testing.T) {
	o, err := NewOrchestrator()
	require.NoError(t, err)
	require.NotNil(t, o)
	assert.NotEmpty(t, o.repoRoot)
	assert.NotEmpty(t, o.evidenceDir)
}

func TestOrchestratorResults(t *testing.T) {
	o, err := NewOrchestrator()
	require.NoError(t, err)

	// Manually inject a result.
	o.results = append(o.results, TestResult{
		Type:   Unit,
		Passed: true,
	})

	results := o.Results()
	require.Len(t, results, 1)
	assert.Equal(t, Unit, results[0].Type)
	assert.True(t, results[0].Passed)
}

func TestOrchestratorSummary(t *testing.T) {
	o, err := NewOrchestrator()
	require.NoError(t, err)

	o.results = []TestResult{
		{Type: Unit, Passed: true, Duration: 1000000000},
		{Type: Smoke, Passed: false, Error: assert.AnError},
	}

	summary := o.Summary()
	assert.Contains(t, summary, "PASS")
	assert.Contains(t, summary, "FAIL")
	assert.Contains(t, summary, "1 passed, 1 failed")
}

// newOrchestratorWithTempEvidenceDir is a test helper that returns an
// Orchestrator whose evidenceDir points at a fresh per-test temp
// directory. We bypass NewOrchestrator() to avoid the go.work auto-
// discovery (which is environment-dependent) — we only need the
// captureFailureEvidence pipeline under test, not the discovery layer.
func newOrchestratorWithTempEvidenceDir(t *testing.T) *Orchestrator {
	t.Helper()
	dir := t.TempDir()
	o := &Orchestrator{
		repoRoot:    dir,
		evidenceDir: filepath.Join(dir, "qa-results"),
	}
	require.NoError(t, os.MkdirAll(o.evidenceDir, 0o755))
	return o
}

// TestCaptureFailureEvidence_NoCaptureContext_ReturnsSentinel is the
// round-29 §11.4 anti-bluff regression test, tightened for round-58.
// When NO subprocess output AND NO wrapping error AND no other capture
// source is available, the orchestrator MUST return the sentinel
// (preserving round-29 behaviour for the genuinely-empty case) and MUST
// NOT fabricate any path.
//
// Constitutional anchors: CONST-035 (anti-bluff), CONST-050(A)
// (no-fakes-beyond-unit-tests), Article XI §11.9 (forensic anchor).
func TestCaptureFailureEvidence_NoCaptureContext_ReturnsSentinel(t *testing.T) {
	o := newOrchestratorWithTempEvidenceDir(t)

	// No stdout, no original error → no capture mechanism applicable.
	paths, capErr := o.captureFailureEvidence(Unit, nil, nil)

	assert.Empty(t, paths, "captureFailureEvidence MUST NOT fabricate evidence paths when nothing real is available to capture")
	require.Error(t, capErr, "captureFailureEvidence MUST surface the gap, not silently pretend evidence was captured")
	assert.True(t, errors.Is(capErr, ErrEvidenceCaptureNotWired), "the surfaced error MUST be ErrEvidenceCaptureNotWired (got %v)", capErr)
}

// TestCaptureFailureEvidence_StdoutPresent_CapturesToFile asserts the
// round-58 wiring: when subprocess stdout/stderr bytes ARE available,
// the orchestrator writes them to <evidenceDir>/<type>/<ts>/stdout.log
// and returns a path that EXISTS on disk + contains the exact bytes
// passed in. No fabrication.
func TestCaptureFailureEvidence_StdoutPresent_CapturesToFile(t *testing.T) {
	o := newOrchestratorWithTempEvidenceDir(t)

	stdout := []byte("FAIL: TestSomething\n--- FAIL: TestSomething (0.01s)\n    expected 4, got 5\n")
	paths, capErr := o.captureFailureEvidence(E2E, errors.New("go test failed: exit status 1"), stdout)

	require.NoError(t, capErr, "captureFailureEvidence with stdout present should succeed without error (got %v)", capErr)
	require.NotEmpty(t, paths, "captureFailureEvidence with stdout present MUST return at least one path")

	// Locate the stdout.log we expect to have been written.
	var stdoutPath string
	for _, p := range paths {
		if filepath.Base(p) == "stdout.log" {
			stdoutPath = p
			break
		}
	}
	require.NotEmpty(t, stdoutPath, "captureFailureEvidence MUST include stdout.log in returned paths (got %v)", paths)

	// Anti-bluff: the file MUST exist on disk AND contain the exact bytes.
	info, err := os.Stat(stdoutPath)
	require.NoError(t, err, "round-58 anti-bluff backstop: returned stdout.log MUST exist on disk")
	assert.False(t, info.IsDir(), "stdout.log MUST be a regular file, not a directory")

	got, err := os.ReadFile(stdoutPath)
	require.NoError(t, err)
	assert.Equal(t, stdout, got, "stdout.log MUST contain the exact bytes captureFailureEvidence was given — no transformation")
}

// TestCaptureFailureEvidence_ReturnedPathsExistOnDisk is the paired-mutation
// anti-bluff backstop for round-58: every path the orchestrator returns
// MUST pass os.Stat verification in the test. The round-29 bluff was
// EXACTLY "return a path string that does not exist on disk", so this
// test mechanically guards against any regression of that pattern.
func TestCaptureFailureEvidence_ReturnedPathsExistOnDisk(t *testing.T) {
	o := newOrchestratorWithTempEvidenceDir(t)

	stdout := []byte("--- FAIL: TestX (0.01s)\n")
	paths, capErr := o.captureFailureEvidence(Integration, errors.New("integration test failed"), stdout)

	require.NoError(t, capErr)
	require.NotEmpty(t, paths)

	for _, p := range paths {
		info, err := os.Stat(p)
		require.NoError(t, err, "round-58 anti-bluff backstop: returned path %s MUST exist on disk (round-29 bluff was 'return path that does not exist')", p)
		assert.False(t, info.IsDir(), "returned path %s MUST be a regular file, not a directory", p)
		// Returned paths MUST also be absolute or rooted inside the evidence
		// directory — never an arbitrary string the orchestrator dreamed up.
		assert.True(t, strings.HasPrefix(p, o.evidenceDir), "returned path %s MUST live inside evidenceDir %s", p, o.evidenceDir)
	}
}

// TestCaptureFailureEvidence_ManifestRoundtripJSON asserts the manifest
// is well-formed JSON, references the same paths returned to the caller,
// and survives a parse → serialise → parse round-trip without data loss.
// A downstream §11.4.2 gate is expected to consume this manifest, so its
// schema stability is part of the round-58 contract.
func TestCaptureFailureEvidence_ManifestRoundtripJSON(t *testing.T) {
	o := newOrchestratorWithTempEvidenceDir(t)

	stdout := []byte("PANIC: runtime error\n")
	originalErr := errors.New("go test failed: exit status 2")
	paths, capErr := o.captureFailureEvidence(Chaos, originalErr, stdout)
	require.NoError(t, capErr)
	require.NotEmpty(t, paths)

	var manifestPath string
	for _, p := range paths {
		if filepath.Base(p) == "manifest.json" {
			manifestPath = p
			break
		}
	}
	require.NotEmpty(t, manifestPath, "captureFailureEvidence MUST emit manifest.json in returned paths (got %v)", paths)

	raw, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var manifest EvidenceManifest
	require.NoError(t, json.Unmarshal(raw, &manifest), "manifest.json MUST be parseable JSON")
	assert.Equal(t, string(Chaos), manifest.TestType)
	assert.Equal(t, originalErr.Error(), manifest.OriginalError)
	assert.Contains(t, manifest.Capturer, "round-58", "Capturer field MUST identify the round-58 wiring for forensic attribution")
	assert.NotEmpty(t, manifest.Paths, "manifest.Paths MUST list captured artefacts")
	assert.False(t, manifest.FailedAt.IsZero(), "manifest.FailedAt MUST be set")

	// Re-serialise + re-parse to assert stability of the schema (no map
	// ordering surprises, no time-format loss).
	reBytes, err := json.Marshal(manifest)
	require.NoError(t, err)
	var manifest2 EvidenceManifest
	require.NoError(t, json.Unmarshal(reBytes, &manifest2))
	assert.Equal(t, manifest.TestType, manifest2.TestType)
	assert.Equal(t, manifest.OriginalError, manifest2.OriginalError)
	assert.Equal(t, len(manifest.Paths), len(manifest2.Paths))
}

// TestCaptureFailureEvidence_OnlyErrorPresent_StillCaptures asserts a
// failure with NO stdout bytes but WITH a wrapping error still produces
// real on-disk evidence (error.log + manifest.json), NOT the sentinel.
// This is the "test type exited via err path before producing stdout"
// case — still informative, still captured.
func TestCaptureFailureEvidence_OnlyErrorPresent_StillCaptures(t *testing.T) {
	o := newOrchestratorWithTempEvidenceDir(t)

	paths, capErr := o.captureFailureEvidence(Smoke, errors.New("Challenges submodule not found"), nil)
	require.NoError(t, capErr, "error-only capture should succeed (got %v)", capErr)
	require.NotEmpty(t, paths, "error-only capture MUST still produce on-disk evidence")

	var errPath string
	for _, p := range paths {
		if filepath.Base(p) == "error.log" {
			errPath = p
			break
		}
	}
	require.NotEmpty(t, errPath, "error-only capture MUST emit error.log")

	got, err := os.ReadFile(errPath)
	require.NoError(t, err)
	assert.Contains(t, string(got), "Challenges submodule not found")
}

// TestCaptureFailureEvidence_EnvSnapshotCaptured asserts QA-relevant
// environment variables are persisted to env.json when present, and that
// non-whitelisted vars are NOT leaked into the snapshot (CONST-042
// secret-leak prevention).
func TestCaptureFailureEvidence_EnvSnapshotCaptured(t *testing.T) {
	o := newOrchestratorWithTempEvidenceDir(t)

	t.Setenv("HELIX_QA_TEST_MARKER", "round-58-marker-value")
	t.Setenv("HELIX_QA_BROWSER_URL", "http://localhost:9222")
	// Non-whitelisted secret that MUST NOT appear in env.json.
	t.Setenv("MY_SECRET_API_KEY", "secret-do-not-leak")

	paths, capErr := o.captureFailureEvidence(Unit, errors.New("unit test failed"), []byte("FAIL\n"))
	require.NoError(t, capErr)
	require.NotEmpty(t, paths)

	var envPath string
	for _, p := range paths {
		if filepath.Base(p) == "env.json" {
			envPath = p
			break
		}
	}
	require.NotEmpty(t, envPath, "env.json MUST be in returned paths when HELIX_QA_* vars are set")

	raw, err := os.ReadFile(envPath)
	require.NoError(t, err)
	envStr := string(raw)
	assert.Contains(t, envStr, "HELIX_QA_TEST_MARKER")
	assert.Contains(t, envStr, "round-58-marker-value")
	assert.Contains(t, envStr, "HELIX_QA_BROWSER_URL")
	// Secret-leak guard.
	assert.NotContains(t, envStr, "MY_SECRET_API_KEY", "env snapshot MUST NOT leak non-whitelisted env vars (CONST-042 / §11.4.10)")
	assert.NotContains(t, envStr, "secret-do-not-leak", "env snapshot MUST NOT leak non-whitelisted secret values")
}

// TestPathExistsOnDisk_DirectoryNotAccepted asserts the round-58
// anti-bluff backstop refuses to validate a directory as a captured
// artefact. A directory at the expected file path indicates a write
// failure or filesystem corruption — both bluffs in the round-29 sense
// (path exists but is not the captured-bytes file the manifest claims).
func TestPathExistsOnDisk_DirectoryNotAccepted(t *testing.T) {
	dir := t.TempDir()
	// pathExistsOnDisk must reject the directory itself.
	assert.False(t, pathExistsOnDisk(dir), "pathExistsOnDisk MUST refuse a directory — it is not a captured-bytes artefact")
	// pathExistsOnDisk must reject a non-existent path.
	assert.False(t, pathExistsOnDisk(filepath.Join(dir, "does-not-exist.log")), "pathExistsOnDisk MUST refuse a non-existent path")
	// pathExistsOnDisk must accept a real file.
	realFile := filepath.Join(dir, "real.log")
	require.NoError(t, os.WriteFile(realFile, []byte("x"), 0o644))
	assert.True(t, pathExistsOnDisk(realFile), "pathExistsOnDisk MUST accept a real on-disk file")
}
