// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Runner executes a Bank's cases against a device.
type Runner struct {
	ADB         *ADB
	Bank        *Bank
	Package     string
	EvidenceDir string
	Timeout     time.Duration
}

// Result is the outcome of one Case.
type Result struct {
	CaseID       string       `json:"case_id"`
	CaseName     string       `json:"case_name"`
	Passed       bool         `json:"passed"`
	FailedStep   int          `json:"failed_step,omitempty"` // 0 == not failed
	FailReason   string       `json:"fail_reason,omitempty"`
	Steps        []StepResult `json:"steps"`
	StartTime    time.Time    `json:"start_time"`
	EndTime      time.Time    `json:"end_time"`
	EvidencePath string       `json:"evidence_path"`
}

// StepResult is the outcome of one Step.
type StepResult struct {
	Index       int           `json:"index"`
	Description string        `json:"description"`
	Action      string        `json:"action"` // e.g. "tap_text:QuickNote"
	Passed      bool          `json:"passed"`
	Error       string        `json:"error,omitempty"`
	Evidence    string        `json:"evidence,omitempty"` // path to relevant artifact
	Duration    time.Duration `json:"duration"`
}

// Run executes the bank. Each case runs in sequence; a case failure
// records the failure + the relevant evidence but does NOT abort the
// remaining cases. Returns the per-case results.
func (r *Runner) Run() []*Result {
	deadline := time.Now().Add(r.Timeout)
	results := make([]*Result, 0, len(r.Bank.Cases))

	for _, c := range r.Bank.Cases {
		if time.Now().After(deadline) {
			fmt.Fprintf(os.Stderr, "TIMEOUT: bank run exceeded %s, skipping remaining cases\n", r.Timeout)
			break
		}
		results = append(results, r.runCase(c))
	}

	// Persist results.json under EvidenceDir for downstream tooling.
	resPath := filepath.Join(r.EvidenceDir, "results.json")
	if data, err := json.MarshalIndent(results, "", "  "); err == nil {
		_ = os.WriteFile(resPath, data, 0o644)
	}

	return results
}

func (r *Runner) runCase(c Case) *Result {
	res := &Result{
		CaseID:    c.ID,
		CaseName:  c.Name,
		StartTime: time.Now(),
	}
	caseDir := filepath.Join(r.EvidenceDir, c.ID)
	_ = os.MkdirAll(caseDir, 0o755)
	res.EvidencePath = caseDir

	fmt.Println()
	fmt.Printf("=== Case %s: %s ===\n", c.ID, c.Name)

	for i, step := range c.Steps {
		sr := r.runStep(i, step, caseDir)
		res.Steps = append(res.Steps, sr)
		if !sr.Passed {
			res.Passed = false
			res.FailedStep = i + 1
			res.FailReason = sr.Error
			res.EndTime = time.Now()
			fmt.Printf("  ✗ step %d FAILED: %s\n", i+1, sr.Error)
			return res
		}
		fmt.Printf("  ✓ step %d (%s) %s\n", i+1, sr.Action, sr.Duration.Round(time.Millisecond))
	}
	res.Passed = true
	res.EndTime = time.Now()
	fmt.Printf("=== Case %s: PASSED in %s ===\n",
		c.ID, res.EndTime.Sub(res.StartTime).Round(time.Millisecond))
	return res
}

func (r *Runner) runStep(idx int, step Step, caseDir string) StepResult {
	start := time.Now()
	sr := StepResult{
		Index:       idx + 1,
		Description: step.Description,
	}

	defer func() {
		sr.Duration = time.Since(start)
	}()

	// Wait
	if step.Wait != "" {
		d, err := time.ParseDuration(step.Wait)
		if err != nil {
			sr.Action = "wait"
			sr.Error = fmt.Sprintf("invalid wait duration %q: %v", step.Wait, err)
			return sr
		}
		time.Sleep(d)
		sr.Action = "wait:" + step.Wait
		sr.Passed = true
		return sr
	}

	// LaunchActivity
	if step.LaunchActivity != "" {
		sr.Action = "launch_activity:" + step.LaunchActivity
		if err := r.ADB.LaunchActivity(step.LaunchActivity); err != nil {
			sr.Error = err.Error()
			return sr
		}
		sr.Passed = true
		return sr
	}

	// ForceStop
	if step.ForceStop != "" {
		sr.Action = "force_stop:" + step.ForceStop
		if err := r.ADB.ForceStop(step.ForceStop); err != nil {
			sr.Error = err.Error()
			return sr
		}
		sr.Passed = true
		return sr
	}

	// TapText / TapDesc / TapXY
	if step.TapText != "" {
		sr.Action = "tap_text:" + step.TapText
		hier, err := r.ADB.Dump()
		if err != nil {
			sr.Error = "dump failed: " + err.Error()
			return sr
		}
		node := hier.FindByText(step.TapText)
		if node == nil {
			sr.Error = fmt.Sprintf("no node with text=%q", step.TapText)
			return sr
		}
		x, y, ok := node.Center()
		if !ok {
			sr.Error = fmt.Sprintf("bad bounds for text=%q: %q", step.TapText, node.Bounds)
			return sr
		}
		if err := r.ADB.Tap(x, y); err != nil {
			sr.Error = err.Error()
			return sr
		}
		sr.Evidence = fmt.Sprintf("center=(%d,%d) bounds=%s", x, y, node.Bounds)
		sr.Passed = true
		return sr
	}
	if step.TapDesc != "" {
		sr.Action = "tap_desc:" + step.TapDesc
		hier, err := r.ADB.Dump()
		if err != nil {
			sr.Error = "dump failed: " + err.Error()
			return sr
		}
		node := hier.FindByDesc(step.TapDesc)
		if node == nil {
			sr.Error = fmt.Sprintf("no node with content-desc=%q", step.TapDesc)
			return sr
		}
		x, y, ok := node.Center()
		if !ok {
			sr.Error = fmt.Sprintf("bad bounds for desc=%q: %q", step.TapDesc, node.Bounds)
			return sr
		}
		if err := r.ADB.Tap(x, y); err != nil {
			sr.Error = err.Error()
			return sr
		}
		sr.Evidence = fmt.Sprintf("center=(%d,%d) bounds=%s", x, y, node.Bounds)
		sr.Passed = true
		return sr
	}
	if step.TapXY != "" {
		sr.Action = "tap_xy:" + step.TapXY
		var x, y int
		if _, err := fmt.Sscanf(step.TapXY, "%d,%d", &x, &y); err != nil {
			sr.Error = "bad tap_xy " + step.TapXY
			return sr
		}
		if err := r.ADB.Tap(x, y); err != nil {
			sr.Error = err.Error()
			return sr
		}
		sr.Passed = true
		return sr
	}

	// TypeText
	if step.TypeText != "" {
		sr.Action = "type_text:" + step.TypeText
		if err := r.ADB.Type(step.TypeText); err != nil {
			sr.Error = err.Error()
			return sr
		}
		sr.Passed = true
		return sr
	}

	// Assertions
	if step.AssertTextPresent != "" {
		sr.Action = "assert_text_present:" + step.AssertTextPresent
		ok, evPath, err := r.assertUI(caseDir, idx, func(h *UIHierarchy) bool {
			return h.HasText(step.AssertTextPresent)
		}, step.WaitFor)
		if !ok {
			sr.Error = fmt.Sprintf("text %q never appeared", step.AssertTextPresent)
			if err != nil {
				sr.Error += ": " + err.Error()
			}
			sr.Evidence = evPath
			return sr
		}
		sr.Evidence = evPath
		sr.Passed = true
		return sr
	}
	if step.AssertDescPresent != "" {
		sr.Action = "assert_desc_present:" + step.AssertDescPresent
		ok, evPath, err := r.assertUI(caseDir, idx, func(h *UIHierarchy) bool {
			return h.HasDesc(step.AssertDescPresent)
		}, step.WaitFor)
		if !ok {
			sr.Error = fmt.Sprintf("content-desc %q never appeared", step.AssertDescPresent)
			if err != nil {
				sr.Error += ": " + err.Error()
			}
			sr.Evidence = evPath
			return sr
		}
		sr.Evidence = evPath
		sr.Passed = true
		return sr
	}
	if step.AssertActivityCurrent != "" {
		sr.Action = "assert_activity_current:" + step.AssertActivityCurrent
		got, err := r.ADB.CurrentActivity()
		if err != nil {
			sr.Error = err.Error()
			return sr
		}
		if got != step.AssertActivityCurrent {
			sr.Error = fmt.Sprintf("current activity %q != expected %q", got, step.AssertActivityCurrent)
			return sr
		}
		sr.Evidence = "mFocusedApp=" + got
		sr.Passed = true
		return sr
	}

	sr.Action = "(empty)"
	sr.Error = "step has no action field set"
	return sr
}

// assertUI polls the UI hierarchy until `match` returns true OR timeout.
// On every poll a fresh dump is taken; the LAST dump and a screenshot
// are persisted to caseDir so failures have positive captured evidence.
func (r *Runner) assertUI(caseDir string, stepIdx int, match func(*UIHierarchy) bool, waitFor string) (bool, string, error) {
	dur := 5 * time.Second
	if waitFor != "" {
		if d, err := time.ParseDuration(waitFor); err == nil {
			dur = d
		}
	}
	deadline := time.Now().Add(dur)
	var lastHier *UIHierarchy
	var lastErr error
	for time.Now().Before(deadline) {
		h, err := r.ADB.Dump()
		if err != nil {
			lastErr = err
			time.Sleep(300 * time.Millisecond)
			continue
		}
		lastHier = h
		if match(h) {
			// Success — persist evidence (a passing dump + screenshot
			// per CONST-035 §11.4.2 captured-evidence floor).
			dumpPath := filepath.Join(caseDir, fmt.Sprintf("step%02d-dump-pass.xml", stepIdx+1))
			persistDump(h, dumpPath)
			shotPath := filepath.Join(caseDir, fmt.Sprintf("step%02d-screen-pass.png", stepIdx+1))
			_ = r.ADB.Screenshot(shotPath)
			return true, shotPath, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	// Failure — persist the LAST dump + a screenshot so the operator
	// can see why the assertion never matched.
	if lastHier != nil {
		dumpPath := filepath.Join(caseDir, fmt.Sprintf("step%02d-dump-fail.xml", stepIdx+1))
		persistDump(lastHier, dumpPath)
	}
	shotPath := filepath.Join(caseDir, fmt.Sprintf("step%02d-screen-fail.png", stepIdx+1))
	_ = r.ADB.Screenshot(shotPath)
	return false, shotPath, lastErr
}

func persistDump(h *UIHierarchy, path string) {
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, "# Concrete-runner UI dump (post-step)")
	fmt.Fprintln(f, "# Format: TEXT|CONTENT-DESC|CLASS|BOUNDS")
	for _, n := range h.Nodes {
		fmt.Fprintf(f, "%q|%q|%q|%s\n", n.Text, n.ContentDesc, n.Class, n.Bounds)
	}
}
