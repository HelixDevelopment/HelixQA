// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Round-61 §11.4 anti-bluff extension (2026-05-18): container-log capturer
// for captureFailureEvidence. Closes the round-58 deferred item alongside
// the browser screenshot capturer in evidence_browser.go.
//
// Verbatim operator mandate (2026-05-19, preserved per CONST-049 §11.4.17):
//
//	"all existing tests and Challenges do work in anti-bluff manner - they
//	 MUST confirm that all tested codebase really works as expected! We had
//	 been in position that all tests do execute with success and all
//	 Challenges as well, but in reality the most of the features does not
//	 work and can't be used! This MUST NOT be the case and execution of
//	 tests and Challenges MUST guarantee the quality, the completition and
//	 full usability by end users of the product!"
//
// Constitutional anchors: CONST-035 (anti-bluff), CONST-042 (no secret
// leak — container logs may contain sensitive data, caller is responsible
// for redacting before publishing artefacts), CONST-050(A)+(B) (no fakes
// beyond unit tests, 100% test-type coverage), Article XI §11.9 (forensic
// anchor).

package helixqa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// EnvContainerID is the environment variable consulted by the container-
// log capturer to decide whether a container context is available. When
// empty the capturer SKIPs (returns ErrEvidenceCaptureNoContainerContext)
// — round-58's honesty contract: no context, no fabrication.
const EnvContainerID = "HELIX_QA_CONTAINER_ID"

// containerCLIs is the ordered list of container-CLI binaries the
// capturer tries. docker first (most common in CI), podman second
// (preferred on rootless Linux desktops per parent CLAUDE.md). The
// list is intentionally short — if the operator uses a different
// runtime they set HELIX_QA_CONTAINER_CLI to override.
var containerCLIs = []string{"docker", "podman"}

// EnvContainerCLI lets the operator override the default CLI search
// order (e.g. point at /usr/local/bin/nerdctl or a vendored binary).
// When set, the capturer tries ONLY this binary, no fallback.
const EnvContainerCLI = "HELIX_QA_CONTAINER_CLI"

// ErrEvidenceCaptureNoContainerContext indicates the caller asked the
// orchestrator to capture container logs but no container context was
// available (HELIX_QA_CONTAINER_ID unset AND no explicit ContainerContext
// supplied). Same anti-bluff posture as ErrEvidenceCaptureNoBrowserContext.
var ErrEvidenceCaptureNoContainerContext = fmt.Errorf("helixqa orchestrator: no container context for log capture — HELIX_QA_CONTAINER_ID is unset and no explicit ContainerContext was supplied; the orchestrator refuses to fabricate a container.log path per round-29/58 anti-bluff backstop")

// ErrEvidenceCaptureContainerLogsFailed indicates the docker/podman exec
// failed (no CLI on PATH, non-zero exit, container missing, etc.).
// Wraps the underlying exec error so callers can errors.Is the sentinel
// and inspect the cause.
var ErrEvidenceCaptureContainerLogsFailed = fmt.Errorf("helixqa orchestrator: container-log capture failed — neither docker nor podman could produce on-disk log bytes for the configured HELIX_QA_CONTAINER_ID; round-29/58 anti-bluff backstop: no fake path returned")

// captureContainerLogs invokes `docker logs <id>` (then `podman logs
// <id>` as fallback) and writes the combined output to
// <perTypeDir>/container.log. The caller passes the per-failure
// evidence directory already created by captureFailureEvidence.
//
// Return contract (preserves round-58 honesty):
//   - On success: returns the on-disk path verified via pathExistsOnDisk
//     and nil error.
//   - On no-context: returns "" and ErrEvidenceCaptureNoContainerContext.
//   - On exec failure (all candidates): returns "" and an error wrapping
//     ErrEvidenceCaptureContainerLogsFailed (errors.Is matches).
//
// The orchestrator NEVER returns a path it has not directly confirmed
// via os.Stat in the same call — same backstop as round-58's stdout.log.
func captureContainerLogs(ctx context.Context, perTypeDir string) (string, error) {
	containerID := os.Getenv(EnvContainerID)
	if containerID == "" {
		return "", ErrEvidenceCaptureNoContainerContext
	}

	// Bound the entire exec dance to a reasonable ceiling so a hung
	// container CLI cannot stall the orchestrator's failure-evidence
	// pipeline. 10 s is generous for `docker logs <id>` on a healthy
	// daemon and well below the orchestrator's per-test timeout.
	captureCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Build CLI candidate list — honour the operator override first.
	candidates := containerCLIs
	if override := os.Getenv(EnvContainerCLI); override != "" {
		candidates = []string{override}
	}

	var attemptErrors []error
	var logBytes []byte
	var usedCLI string
	for _, cli := range candidates {
		path, lookErr := exec.LookPath(cli)
		if lookErr != nil {
			attemptErrors = append(attemptErrors, fmt.Errorf("LookPath(%s): %w", cli, lookErr))
			continue
		}
		cmd := exec.CommandContext(captureCtx, path, "logs", containerID)
		out, err := cmd.CombinedOutput()
		if err != nil {
			attemptErrors = append(attemptErrors, fmt.Errorf("%s logs %s: %w (output=%q)", cli, containerID, err, truncateForError(out)))
			continue
		}
		logBytes = out
		usedCLI = cli
		break
	}

	if usedCLI == "" {
		// Every candidate failed — surface the combined error so the
		// caller's downstream §11.4.2 gate sees both signals.
		return "", fmt.Errorf("%w: tried %v: %w", ErrEvidenceCaptureContainerLogsFailed, candidates, errors.Join(attemptErrors...))
	}

	containerLogPath := filepath.Join(perTypeDir, "container.log")
	// Even an empty log is still informative ("this container produced
	// nothing"). We persist what we received, but pathExistsOnDisk plus
	// a non-zero header line guarantees the artefact is real.
	header := fmt.Sprintf("# helixqa container-log capture (round-61)\n# cli=%s container_id=%s captured_at=%s bytes=%d\n", usedCLI, containerID, time.Now().UTC().Format(time.RFC3339), len(logBytes))
	combined := append([]byte(header), logBytes...)
	if err := os.WriteFile(containerLogPath, combined, 0o644); err != nil {
		return "", fmt.Errorf("%w: write %s: %v", ErrEvidenceCaptureContainerLogsFailed, containerLogPath, err)
	}
	if !pathExistsOnDisk(containerLogPath) {
		return "", fmt.Errorf("%w: post-write os.Stat verification failed for %s", ErrEvidenceCaptureStatVerificationFailed, containerLogPath)
	}
	return containerLogPath, nil
}

// truncateForError clips command-output bytes to a manageable length
// for error messages. Full output still lives in the on-disk container.log
// when capture succeeds; this is purely for failure-path log clarity.
func truncateForError(out []byte) string {
	const maxLen = 256
	if len(out) <= maxLen {
		return string(out)
	}
	return string(out[:maxLen]) + "...[truncated]"
}

// containerCaptureSentinels enumerates the sentinels captureContainerLogs
// can return. Mirrors browserCaptureSentinels in evidence_browser.go.
var containerCaptureSentinels = []error{
	ErrEvidenceCaptureNoContainerContext,
	ErrEvidenceCaptureContainerLogsFailed,
	ErrEvidenceCaptureStatVerificationFailed,
}

// isContainerCaptureSentinel reports whether err matches any of the
// round-61 container-capture sentinels.
func isContainerCaptureSentinel(err error) bool {
	if err == nil {
		return false
	}
	for _, s := range containerCaptureSentinels {
		if errors.Is(err, s) {
			return true
		}
	}
	return false
}
