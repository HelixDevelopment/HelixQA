// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-bank-session runs a REAL test bank through the
// pkg/testbank.Dispatcher evidence-ledger — replacing the STUB session
// runner (run_release_gate_helixqa_session.sh) that bypassed the Go
// Dispatcher and ignored each Challenge's declared required_evidence
// (AUDIT GAP B / G7 / G8).
//
// For every TestCase in the bank that declares dispatches_to and/or
// required_evidence, this driver:
//
//  1. runs dispatches_to via a CONSUMER-INJECTED DeviceExec — here an
//     `adb -s <serial> shell sh <on-device-test>` wrapper (the real
//     device test that produces the captured evidence). The script's
//     real exit code drives step-1 of the verdict.
//  2. enforces the §11.4.69 evidence ledger via the CONTENT-ASSERTING
//     resolver (pkg/testbank.ContentAssertingResolver) — every
//     required_evidence token's CONTENT must match its declared
//     assertion (not merely exist non-empty). A non-empty-but-WRONG
//     artefact (empty Arvus Codec-In-Use, stereo collapse, frozen frame,
//     wrong display, pale render, silent captured WAV) FAILs.
//  3. records a §11.4.116 conduit verdict + per-artefact evidence event
//     to an append-only JSONL stream + an atomically-rewritten status
//     snapshot the conductor can tail.
//
// Project-agnostic per CONST-051 (§11.4.28): every ATMOSphere fact
// (device serial, bank path, on-device test directory, evidence base
// directory) is a CLI flag. HelixQA's Dispatcher + ContentAssertingResolver
// hardcode nothing. The grammar of required_evidence tokens is generic;
// the bank declares the codec regex / JSON key / channel threshold.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/conduit"
	"digital.vasic.helixqa/pkg/testbank"
)

// row is one Challenge's outcome in the machine-readable summary.
type row struct {
	Challenge string   `json:"challenge"`
	Verdict   string   `json:"verdict"`
	Reason    string   `json:"reason,omitempty"`
	Missing   []string `json:"missing_evidence,omitempty"`
	ExitCode  int      `json:"exit_code"`
}

// recordResult folds a dispatch result into the running counters + rows
// and prints a one-line verdict (shared by the dispatch + dry-run paths).
func recordResult(res testbank.DispatchResult, pass, fail, skip *int, rows *[]row) {
	switch res.Verdict {
	case conduit.VerdictPass:
		*pass++
	case conduit.VerdictFail:
		*fail++
	default:
		*skip++
	}
	*rows = append(*rows, row{
		Challenge: res.ChallengeID,
		Verdict:   string(res.Verdict),
		Reason:    res.Reason,
		Missing:   res.MissingEvidence,
		ExitCode:  res.ExitCode,
	})
	fmt.Printf("[%-4s] %s  %s\n", res.Verdict, res.ChallengeID, res.Reason)
}

func main() {
	var (
		bankPath    = flag.String("bank", "", "path to the test bank YAML (REQUIRED)")
		serial      = flag.String("serial", "", "adb device serial (REQUIRED for dispatch)")
		repoRoot    = flag.String("repo-root", ".", "consumer repo root; on-device test paths in the bank are resolved relative to it for push")
		evidenceDir = flag.String("evidence-dir", "", "base directory for required_evidence relative path tokens (REQUIRED for the ledger)")
		sessionDir  = flag.String("session-dir", "", "directory for the conduit JSONL + status snapshot (REQUIRED)")
		domain      = flag.String("domain", "", "only run cases tagged with this domain (e.g. audio); empty = all")
		dispatchTO  = flag.Duration("dispatch-timeout", 5*time.Minute, "per-Challenge dispatch timeout")
		dryRun      = flag.Bool("dry-run", false, "do not execute dispatches_to scripts; only enforce the evidence ledger over existing artefacts")
		adbBin      = flag.String("adb", "adb", "adb binary on PATH")
	)
	flag.Parse()

	if *bankPath == "" || *evidenceDir == "" || *sessionDir == "" {
		fmt.Fprintln(os.Stderr, "helixqa-bank-session: --bank, --evidence-dir and --session-dir are required")
		os.Exit(2)
	}
	if !*dryRun && *serial == "" {
		fmt.Fprintln(os.Stderr, "helixqa-bank-session: --serial is required (or use --dry-run to enforce the ledger only)")
		os.Exit(2)
	}

	bank, err := testbank.LoadFile(*bankPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "helixqa-bank-session: load bank %q: %v\n", *bankPath, err)
		os.Exit(2)
	}
	cases := bank.TestCases
	if *domain != "" {
		cases = testbank.FilterByDomain(cases, *domain)
	}

	if err := os.MkdirAll(*sessionDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "helixqa-bank-session: mkdir session dir: %v\n", err)
		os.Exit(2)
	}
	sess := time.Now().UTC().Format("20060102T150405Z")
	w, err := conduit.NewWriter(conduit.Config{Session: sess, Dir: *sessionDir})
	if err != nil {
		fmt.Fprintf(os.Stderr, "helixqa-bank-session: conduit writer: %v\n", err)
		os.Exit(2)
	}
	defer w.Close()

	conduit.SessionStart(w, sess, map[string]any{
		"bank":      *bankPath,
		"serial":    *serial,
		"domain":    *domain,
		"cases":     len(cases),
		"dry_run":   *dryRun,
		"evidence":  *evidenceDir,
		"validator": "ContentAssertingResolver", // the §11.4.69 content gate, not size-only
	})

	// Consumer-injected DeviceExec: push the on-device test then run it via
	// adb. The bank's dispatches_to is a repo-relative path to a test_*.sh.
	var execHook testbank.DeviceExec
	if *dryRun {
		// Ledger-only mode: no script runs; the verdict rests entirely on
		// the content-asserting evidence over already-captured artefacts.
		execHook = nil
	} else {
		execHook = testbank.DeviceExecFunc(func(ctx context.Context, dispatchesTo string) (string, int, error) {
			return runOnDeviceTest(ctx, *adbBin, *serial, *repoRoot, dispatchesTo)
		})
	}

	disp := &testbank.Dispatcher{
		Exec:     execHook,
		Evidence: testbank.ContentAssertingResolver{BaseDir: *evidenceDir},
		Conduit:  w,
		Timeout:  *dispatchTO,
	}

	var rows []row
	var pass, fail, skip int

	for i := range cases {
		tc := &cases[i]
		if tc.DispatchesTo == "" && len(tc.RequiredEvidence) == 0 {
			continue // not a dispatch case; the ordinary step executor owns it
		}
		if *dryRun {
			// Ledger-only mode: do NOT re-run the on-device script; enforce
			// the content-asserting evidence ledger over already-captured
			// artefacts. Skip cases that have no evidence to assert (they
			// would only run a script we are intentionally not running).
			if len(tc.RequiredEvidence) == 0 {
				continue
			}
			tcCopy := *tc
			tcCopy.DispatchesTo = ""
			res := disp.Run(context.Background(), &tcCopy)
			recordResult(res, &pass, &fail, &skip, &rows)
			continue
		}
		res := disp.Run(context.Background(), tc)
		recordResult(res, &pass, &fail, &skip, &rows)
	}

	// Write a machine-readable summary alongside the conduit stream.
	summary := map[string]any{
		"session": sess,
		"bank":    *bankPath,
		"pass":    pass,
		"fail":    fail,
		"skip":    skip,
		"cases":   len(rows),
		"rows":    rows,
	}
	if b, err := json.MarshalIndent(summary, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(*sessionDir, "session_summary.json"), b, 0o644)
	}

	overall := conduit.VerdictPass
	detail := fmt.Sprintf("%d pass, %d fail, %d skip", pass, fail, skip)
	if fail > 0 {
		overall = conduit.VerdictFail
	}
	conduit.SessionEnd(w, sess, overall, detail)

	fmt.Printf("\nHelixQA bank session: %s\n", *sessionDir)
	fmt.Printf("  stream:  %s\n", w.StreamPath())
	fmt.Printf("  verdict: PASS=%d FAIL=%d SKIP=%d\n", pass, fail, skip)
	if fail > 0 {
		os.Exit(1)
	}
}

// runOnDeviceTest pushes the consumer's on-device test to the device and
// runs it, returning combined output + exit code. The test is responsible
// for writing the captured evidence the bank's required_evidence asserts.
func runOnDeviceTest(ctx context.Context, adbBin, serial, repoRoot, dispatchesTo string) (string, int, error) {
	// Trust boundary: dispatchesTo comes from the bank YAML (consumer-
	// authored config, same trust level as a Makefile target), serial +
	// adbBin from operator CLI flags. Args are passed as SEPARATE argv
	// elements to exec (NOT via a shell), so there is no shell-injection
	// surface. We still sanitise the on-device basename to a strict
	// allowlist so a malformed bank entry cannot smuggle path separators
	// or metacharacters into the device path.
	base := filepath.Base(dispatchesTo)
	if !safeTestBasename(base) {
		return "", -1, fmt.Errorf("refusing unsafe dispatches_to basename %q (allow [A-Za-z0-9._-], must end .sh)", base)
	}
	local := dispatchesTo
	if !filepath.IsAbs(local) {
		local = filepath.Join(repoRoot, dispatchesTo)
	}
	devPath := "/data/local/tmp/tests/" + base

	// push (best-effort: if the file is already on-device this still works)
	if _, err := os.Stat(local); err == nil {
		if out, perr := adbRun(ctx, adbBin, "-s", serial, "push", local, devPath); perr != nil {
			return string(out), -1, fmt.Errorf("adb push %s: %w", base, perr)
		}
	}
	out, err := adbRun(ctx, adbBin, "-s", serial, "shell", "sh "+devPath)
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
			err = nil // ordinary non-zero exit is reported via code, not error
		} else {
			return string(out), -1, err
		}
	}
	// adb shell does not always propagate the remote exit code; if the
	// output ends with an explicit marker the test prints, prefer it.
	if code == 0 && strings.Contains(string(out), "ANTI_BLUFF_FAIL") {
		code = 1
	}
	return string(out), code, nil
}

// adbRun is the single audited exec site (same pattern as the shipped
// cmd/helixqa-bridge execRunner.Run). `name` is the operator-supplied adb
// binary CLI flag; args are argv-separated (NO shell interpretation), so
// there is no shell-injection surface. The on-device test basename is
// allowlist-validated by safeTestBasename before reaching here.
func adbRun(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// safeTestBasename allows only [A-Za-z0-9._-], requires a .sh suffix, and
// forbids leading dots / path separators — so a malformed bank dispatches_to
// cannot smuggle path traversal or metacharacters into the device path.
func safeTestBasename(b string) bool {
	if b == "" || strings.HasPrefix(b, ".") || !strings.HasSuffix(b, ".sh") {
		return false
	}
	for _, r := range b {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}
