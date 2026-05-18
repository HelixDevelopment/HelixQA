// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Round-61 §11.4 anti-bluff tests: container-log capture path.

package helixqa

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hasContainerCLI reports whether docker or podman is on PATH.
func hasContainerCLI() bool {
	for _, cli := range containerCLIs {
		if _, err := exec.LookPath(cli); err == nil {
			return true
		}
	}
	return false
}

// TestCaptureContainerLogs_NoContainerID_ReturnsSentinel asserts the
// no-context contract: when HELIX_QA_CONTAINER_ID is unset the capturer
// returns ErrEvidenceCaptureNoContainerContext and an empty path.
func TestCaptureContainerLogs_NoContainerID_ReturnsSentinel(t *testing.T) {
	t.Setenv(EnvContainerID, "")

	tmp := t.TempDir()
	path, err := captureContainerLogs(t.Context(), tmp)

	assert.Empty(t, path, "no-context capture MUST NOT fabricate a path")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEvidenceCaptureNoContainerContext), "no-context capture MUST return ErrEvidenceCaptureNoContainerContext (got %v)", err)
}

// TestCaptureContainerLogs_CLIMissing_ReturnsSentinel uses HELIX_QA_CONTAINER_CLI
// override + a deliberately-nonexistent binary to force LookPath failure
// for every candidate and asserts ErrEvidenceCaptureContainerLogsFailed.
func TestCaptureContainerLogs_CLIMissing_ReturnsSentinel(t *testing.T) {
	t.Setenv(EnvContainerID, "round-61-test-container")
	// Override the CLI search list to a guaranteed-nonexistent binary.
	t.Setenv(EnvContainerCLI, "/nonexistent/bin/round-61-no-such-cli")

	tmp := t.TempDir()
	path, err := captureContainerLogs(t.Context(), tmp)

	assert.Empty(t, path, "missing CLI MUST NOT fabricate a path")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEvidenceCaptureContainerLogsFailed), "missing CLI MUST return ErrEvidenceCaptureContainerLogsFailed (got %v)", err)
}

// TestCaptureContainerLogs_CLIPathHidden_ReturnsSentinel scrubs PATH
// to hide docker/podman binaries from LookPath and asserts the
// failure sentinel fires. Belt-and-braces against the override test.
func TestCaptureContainerLogs_CLIPathHidden_ReturnsSentinel(t *testing.T) {
	t.Setenv(EnvContainerID, "round-61-test-container")
	// Empty PATH guarantees LookPath fails for both docker AND podman.
	t.Setenv("PATH", "")

	tmp := t.TempDir()
	path, err := captureContainerLogs(t.Context(), tmp)

	assert.Empty(t, path, "scrubbed PATH MUST NOT fabricate a path")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrEvidenceCaptureContainerLogsFailed), "scrubbed PATH MUST return ErrEvidenceCaptureContainerLogsFailed (got %v)", err)
}

// TestCaptureContainerLogs_WithRealContainer is an env-gated integration
// test that hits a REAL docker/podman daemon. SKIP-OK marker fires when
// the environment is not set up. The test sets HELIX_QA_CONTAINER_ID to
// the operator-provided container ID and asserts container.log appears
// on disk with non-zero header bytes.
//
// SKIP-OK: #HELIXQA-EVIDENCE-CONTAINER-REAL-ROUND61 — requires operator
// to set HELIXQA_TEST_REAL_CONTAINER_ID to a live container ID with
// docker or podman daemon running.
func TestCaptureContainerLogs_WithRealContainer(t *testing.T) {
	containerID := os.Getenv("HELIXQA_TEST_REAL_CONTAINER_ID")
	if containerID == "" {
		t.Skip("SKIP-OK: #HELIXQA-EVIDENCE-CONTAINER-REAL-ROUND61 — HELIXQA_TEST_REAL_CONTAINER_ID unset; real-container integration test skipped")
	}
	if !hasContainerCLI() {
		t.Skip("SKIP-OK: #HELIXQA-EVIDENCE-CONTAINER-REAL-ROUND61 — no docker/podman on PATH; real-container integration test cannot run")
	}

	t.Setenv(EnvContainerID, containerID)

	tmp := t.TempDir()
	path, err := captureContainerLogs(t.Context(), tmp)
	require.NoError(t, err, "real-container log capture should succeed; got %v", err)
	require.NotEmpty(t, path)

	info, err := os.Stat(path)
	require.NoError(t, err, "round-61 anti-bluff backstop: container.log MUST exist on disk")
	assert.False(t, info.IsDir())
	assert.Greater(t, info.Size(), int64(0), "container.log header alone is non-empty even if container produced no logs")

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "# helixqa container-log capture (round-61)", "container.log MUST carry the round-61 header")
	assert.Contains(t, string(raw), containerID, "container.log header MUST identify the captured container_id")
}

// TestIsContainerCaptureSentinel asserts the predicate matches every
// round-61 container-capture sentinel and rejects unrelated errors.
func TestIsContainerCaptureSentinel(t *testing.T) {
	for _, s := range containerCaptureSentinels {
		assert.True(t, isContainerCaptureSentinel(s), "isContainerCaptureSentinel MUST recognise %v", s)
	}
	assert.False(t, isContainerCaptureSentinel(nil), "nil MUST NOT be classified as a sentinel")
	assert.False(t, isContainerCaptureSentinel(errors.New("unrelated")), "unrelated error MUST NOT be classified as a sentinel")
}

// TestCaptureFailureEvidence_ContainerCLIMissing_ReturnsSentinel
// exercises the dispatcher path: when HELIX_QA_CONTAINER_ID is set but
// no CLI can find a binary, captureFailureEvidence's errors.Join MUST
// include ErrEvidenceCaptureContainerLogsFailed AND no container.log
// path is returned. The unrelated artefacts (stdout.log, error.log,
// env.json, manifest.json) MUST still be captured.
func TestCaptureFailureEvidence_ContainerCLIMissing_ReturnsSentinel(t *testing.T) {
	t.Setenv(EnvContainerID, "round-61-test-container")
	t.Setenv(EnvContainerCLI, "/nonexistent/bin/round-61-no-such-cli")

	o := newOrchestratorWithTempEvidenceDir(t)
	paths, capErr := o.captureFailureEvidence(Integration, errors.New("integration test failed"), []byte("FAIL\n"))

	require.Error(t, capErr, "missing container CLI MUST surface an error")
	assert.True(t, errors.Is(capErr, ErrEvidenceCaptureContainerLogsFailed), "missing CLI error MUST wrap ErrEvidenceCaptureContainerLogsFailed (got %v)", capErr)

	for _, p := range paths {
		assert.NotEqual(t, "container.log", filepath.Base(p), "round-29/58 anti-bluff: container.log MUST NOT appear in returned paths when capture failed (got %v)", paths)
	}

	// Partial-success path: stdout.log + error.log + manifest.json MUST
	// still be captured. env.json is conditional on QA-prefixed env vars
	// being present; HELIX_QA_CONTAINER_ID is set so env.json should
	// also appear.
	bases := make(map[string]bool)
	for _, p := range paths {
		bases[filepath.Base(p)] = true
	}
	assert.True(t, bases["stdout.log"], "stdout.log MUST still appear in partial-success returned paths")
	assert.True(t, bases["error.log"], "error.log MUST still appear in partial-success returned paths")
	assert.True(t, bases["manifest.json"], "manifest.json MUST still appear in partial-success returned paths")
}

// TestCaptureFailureEvidence_PartialFailure_ReturnsArtefactsPlusErrorsJoin
// asserts the dispatcher returns successfully-captured artefacts AND
// surfaces capture failures via errors.Join — exactly the round-58
// honesty contract carried into round-61.
func TestCaptureFailureEvidence_PartialFailure_ReturnsArtefactsPlusErrorsJoin(t *testing.T) {
	// Set container context BUT make sure the CLI is unreachable,
	// so container capture fails while everything else succeeds.
	t.Setenv(EnvContainerID, "round-61-test-container")
	t.Setenv(EnvContainerCLI, "/nonexistent/bin/round-61-no-such-cli")

	o := newOrchestratorWithTempEvidenceDir(t)
	stdout := []byte("--- FAIL: TestX (0.01s)\n  partial-failure scenario\n")
	paths, capErr := o.captureFailureEvidence(Chaos, errors.New("chaos test failed"), stdout)

	// Expect at least stdout.log + error.log + manifest.json present
	// AND capErr non-nil containing the container sentinel via errors.Join.
	require.NotEmpty(t, paths)
	require.Error(t, capErr)
	assert.True(t, errors.Is(capErr, ErrEvidenceCaptureContainerLogsFailed))

	stdoutFound := false
	for _, p := range paths {
		if strings.HasSuffix(p, "stdout.log") {
			stdoutFound = true
			info, err := os.Stat(p)
			require.NoError(t, err, "round-58 backstop: every returned path MUST os.Stat")
			assert.Equal(t, int64(len(stdout)), info.Size(), "stdout.log MUST contain exactly the captured bytes")
		}
	}
	assert.True(t, stdoutFound, "stdout.log MUST still appear in partial-success returned paths")
}
