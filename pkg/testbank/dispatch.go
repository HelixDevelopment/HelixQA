// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/conduit"
)

// This file wires the schema dispatch metadata (ChallengeID,
// DispatchesTo, Domains, RequiredEvidence — see schema.go) into a
// runnable mechanism:
//
//   1. A generic dispatch executor that runs a case's DispatchesTo
//      script via a CONSUMER-INJECTED exec hook, captures its exit
//      code + output, and records a conduit verdict + evidence event.
//   2. An evidence-ledger gate (§11.4.69) that forbids a PASS unless
//      every RequiredEvidence token resolves to a real, non-empty
//      artefact.
//
// HelixQA stays project-agnostic per CONST-051 (§11.4.28): it never
// hardcodes a device serial, package id, script path, or evidence
// token. The consuming project supplies the DeviceExec implementation
// and all bank data at runtime.

// DeviceExec is the consumer-injected command runner. The consuming
// project provides the implementation — e.g. an `adb -s <serial>
// shell` wrapper, a local `os/exec` runner, an SSH executor, or a
// container exec. HelixQA never constructs a concrete one; it only
// calls through this interface so no ATMOSphere/device knowledge
// leaks into the framework.
type DeviceExec interface {
	// Exec runs the given command line and returns the combined
	// stdout+stderr output and the process exit code. A non-nil error
	// is returned only for execution failures the runner could not
	// translate into an exit code (spawn failure, context cancelled);
	// an ordinary non-zero exit is reported via exitCode with a nil
	// error so the dispatch executor can map it to a FAIL verdict.
	Exec(ctx context.Context, command string) (output string, exitCode int, err error)
}

// DeviceExecFunc adapts a plain function to the DeviceExec interface,
// so a consumer can inject a closure without declaring a type.
type DeviceExecFunc func(ctx context.Context, command string) (string, int, error)

// Exec implements DeviceExec.
func (f DeviceExecFunc) Exec(ctx context.Context, command string) (string, int, error) {
	return f(ctx, command)
}

// EvidenceResolver locates evidence artefacts for a single
// RequiredEvidence token. The consuming project decides what a token
// means (a glob under a run directory, a conduit evidence key, a sink
// probe report path, ...). HelixQA only requires that a satisfied
// token map to at least one path that exists and is non-empty.
//
// Resolve returns the matched artefact paths for the token. An empty
// slice (or a slice whose entries are all missing/empty) means the
// token is unsatisfied. A non-nil error is treated as unsatisfied
// (the token failed to resolve) and surfaced in the missing list.
type EvidenceResolver interface {
	Resolve(token string) (paths []string, err error)
}

// EvidenceResolverFunc adapts a plain function to EvidenceResolver.
type EvidenceResolverFunc func(token string) ([]string, error)

// Resolve implements EvidenceResolver.
func (f EvidenceResolverFunc) Resolve(token string) ([]string, error) { return f(token) }

// GlobEvidenceResolver is a generic resolver that treats each token
// as a filepath glob, optionally rooted under BaseDir. It is the
// reference implementation HelixQA ships; the consuming project may
// supply its own. A token is satisfied when at least one glob match
// exists AND is a non-empty regular file (the anti-bluff condition —
// a 0-byte mp4 is NOT evidence).
type GlobEvidenceResolver struct {
	// BaseDir, when non-empty, is prepended to relative tokens so a
	// bank can use short tokens (e.g. "codec-state.json") resolved
	// against a per-run evidence directory the consumer chooses.
	BaseDir string
}

// Resolve implements EvidenceResolver by globbing the token (rooted
// at BaseDir for relative tokens) and returning only matches that
// exist and are non-empty.
func (g GlobEvidenceResolver) Resolve(token string) ([]string, error) {
	pattern := token
	if g.BaseDir != "" && !filepath.IsAbs(token) {
		pattern = filepath.Join(g.BaseDir, token)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("evidence glob %q invalid: %w", pattern, err)
	}
	var nonEmpty []string
	for _, m := range matches {
		if fi, statErr := os.Stat(m); statErr == nil && !fi.IsDir() && fi.Size() > 0 {
			nonEmpty = append(nonEmpty, m)
		}
	}
	return nonEmpty, nil
}

// DispatchResult is the outcome of running one bank case through the
// dispatch executor + evidence-ledger gate.
type DispatchResult struct {
	// CaseID / ChallengeID identify the case (ChallengeID is the
	// stable handle per EffectiveChallengeID).
	CaseID      string
	ChallengeID string

	// Verdict is the final PASS/FAIL/SKIP outcome after BOTH the
	// dispatched-script exit code AND the evidence-ledger gate.
	Verdict conduit.Verdict

	// ExitCode is the dispatched script's exit code (0 unless a
	// DispatchesTo script ran and exited non-zero). Negative means
	// "no script ran" (case had no DispatchesTo).
	ExitCode int

	// Output is the dispatched script's combined output (may be
	// truncated by the caller before logging — HelixQA does not
	// truncate here so the consumer keeps full evidence).
	Output string

	// MissingEvidence lists the RequiredEvidence tokens that did NOT
	// resolve to a real, non-empty artefact. Non-empty here forces a
	// FAIL even if the script exited 0 (the §11.4.69 hole this gate
	// closes).
	MissingEvidence []string

	// EvidencePaths maps each satisfied token to the artefact paths
	// that satisfied it (for the conductor / report).
	EvidencePaths map[string][]string

	// Reason carries a SKIP / failure detail (closed-set reasons
	// preferred per §11.4.69 for SKIP).
	Reason string

	// Duration is the dispatched-script wall-clock time.
	Duration time.Duration
}

// Dispatcher runs bank cases that declare a DispatchesTo script and
// enforces the RequiredEvidence ledger. Both the command runner and
// the evidence resolver are consumer-injected, so the Dispatcher
// carries no project knowledge. A nil Conduit is tolerated (events
// are simply not emitted).
type Dispatcher struct {
	// Exec runs the DispatchesTo script. Required when any dispatched
	// case is run; a nil Exec makes Run return a SKIP for cases that
	// declare DispatchesTo (honest — the case could not be driven).
	Exec DeviceExec

	// Evidence resolves RequiredEvidence tokens. When nil, a case
	// that declares RequiredEvidence cannot be satisfied and FAILs
	// with every token reported missing (no resolver = no evidence).
	Evidence EvidenceResolver

	// Conduit, when non-nil, receives challenge_start / evidence /
	// challenge_verdict events for the live conductor channel
	// (§11.4.116). Nil is fine for unit tests and headless runs.
	Conduit conduit.Sink

	// Timeout bounds a single dispatched script. Zero means no
	// HelixQA-imposed timeout (the consumer's DeviceExec / context
	// governs).
	Timeout time.Duration
}

// Run executes a single bank case's dispatch + evidence-ledger and
// returns the resulting verdict. The decision order is:
//
//   1. If the case declares DispatchesTo, run it. A non-zero exit (or
//      a spawn error) is a FAIL — no evidence check needed, the
//      Challenge body itself failed.
//   2. Enforce RequiredEvidence: every declared token MUST resolve to
//      a real, non-empty artefact. Any missing → FAIL with the list.
//   3. Otherwise PASS, citing the satisfied evidence.
//
// A case with neither DispatchesTo nor RequiredEvidence is a no-op
// for the dispatcher and returns VerdictSkip with reason
// "no_dispatch_metadata" so the caller routes it through the ordinary
// step executor instead (the dispatcher only owns cases that opt in).
func (d *Dispatcher) Run(ctx context.Context, tc *TestCase) DispatchResult {
	res := DispatchResult{
		CaseID:        tc.ID,
		ChallengeID:   tc.EffectiveChallengeID(),
		ExitCode:      -1,
		EvidencePaths: map[string][]string{},
	}

	if tc.DispatchesTo == "" && len(tc.RequiredEvidence) == 0 {
		res.Verdict = conduit.VerdictSkip
		res.Reason = "no_dispatch_metadata"
		return res
	}

	if d.Conduit != nil {
		conduit.ChallengeStart(d.Conduit, res.ChallengeID, domainsCSV(tc.Domains))
	}

	// Step 1: run the dispatched script (if any).
	if tc.DispatchesTo != "" {
		if d.Exec == nil {
			res.Verdict = conduit.VerdictSkip
			res.Reason = "no_device_exec_injected"
			d.emitVerdict(res)
			return res
		}
		runCtx := ctx
		var cancel context.CancelFunc
		if d.Timeout > 0 {
			runCtx, cancel = context.WithTimeout(ctx, d.Timeout)
			defer cancel()
		}
		start := time.Now()
		out, code, err := d.Exec.Exec(runCtx, tc.DispatchesTo)
		res.Duration = time.Since(start)
		res.Output = out
		res.ExitCode = code
		if err != nil {
			res.Verdict = conduit.VerdictFail
			res.Reason = "dispatch_exec_error: " + err.Error()
			d.emitVerdict(res)
			return res
		}
		if code != 0 {
			res.Verdict = conduit.VerdictFail
			res.Reason = fmt.Sprintf("dispatch_exit_%d", code)
			d.emitVerdict(res)
			return res
		}
	}

	// Step 2: enforce the evidence ledger (§11.4.69). A PASS is only
	// permitted when EVERY required token resolves to real evidence.
	if len(tc.RequiredEvidence) > 0 {
		missing := d.checkEvidence(tc, &res)
		if len(missing) > 0 {
			res.MissingEvidence = missing
			res.Verdict = conduit.VerdictFail
			res.Reason = "missing_required_evidence: " + strings.Join(missing, ",")
			d.emitVerdict(res)
			return res
		}
	}

	// Step 3: all gates cleared.
	res.Verdict = conduit.VerdictPass
	d.emitVerdict(res)
	return res
}

// checkEvidence resolves every RequiredEvidence token, records the
// satisfied paths (emitting evidence_captured events), and returns the
// sorted list of unsatisfied tokens.
func (d *Dispatcher) checkEvidence(tc *TestCase, res *DispatchResult) []string {
	var missing []string
	for _, token := range tc.RequiredEvidence {
		if d.Evidence == nil {
			missing = append(missing, token)
			continue
		}
		paths, err := d.Evidence.Resolve(token)
		if err != nil || len(paths) == 0 {
			missing = append(missing, token)
			continue
		}
		res.EvidencePaths[token] = paths
		if d.Conduit != nil {
			// Emit one evidence_captured per resolved artefact so the
			// conductor can audit each path (anti-bluff).
			for _, p := range paths {
				conduit.EvidenceCaptured(d.Conduit, res.ChallengeID, "required_evidence", p)
			}
		}
	}
	sort.Strings(missing)
	return missing
}

// emitVerdict folds the result into a conduit challenge_verdict event
// (nil-safe).
func (d *Dispatcher) emitVerdict(res DispatchResult) {
	if d.Conduit == nil {
		return
	}
	conduit.ChallengeVerdict(d.Conduit, res.ChallengeID, res.Verdict, res.Reason)
}

// domainsCSV joins domain labels for the conduit "platform"/area slot.
func domainsCSV(domains []string) string {
	return strings.Join(domains, ",")
}

// FilterByDomain returns the subset of cases tagged with the given
// domain label (case-insensitive). An empty domain returns all cases.
// Supports §11.4.119 single-resource-owner partitioning and §11.4.72
// audio-first dispatch — the labels are opaque consumer strings.
func FilterByDomain(cases []TestCase, domain string) []TestCase {
	if domain == "" {
		return cases
	}
	var out []TestCase
	for i := range cases {
		if cases[i].HasDomain(domain) {
			out = append(out, cases[i])
		}
	}
	return out
}
