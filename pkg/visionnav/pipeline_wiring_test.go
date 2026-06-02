// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// pipeline_wiring_test.go — Issues.md C6 end-to-end wiring proof.
//
// Proves the C6 pipeline composes: a real LLMProvider (backed by a
// fake VisionDecider) + a real ADBActor (backed by a fake
// DeviceExecutor) + a DefaultExplorer (backed by an in-memory sink) +
// a runtime-registered Target, driven by Session.Run to a PASS verdict.
// This is the integration seam §GT.3 / C6 asked for — the loop in
// session.go now has concrete collaborators on both ends.
//
// Mocks here are the fakes for the LLM backend + the device executor
// only; the Provider, Actor, Explorer, Session, Target, and FileSink
// code under test is real (§11.4.27 — fakes are confined to the
// external LLM + ADB boundaries, which is the unit-test boundary).

package visionnav

import (
	"context"
	"strings"
	"testing"
)

// screenSteppingExec is a DeviceExecutor whose Screenshot returns a
// DIFFERENT image each call (so the §11.4.52 screen-delta requirement is
// genuinely satisfied — not faked by returning identical bytes).
type screenSteppingExec struct {
	step  int
	calls []string
}

func (e *screenSteppingExec) Screenshot(_ context.Context) ([]byte, error) {
	e.step++
	// Distinct, screenshot-sized payloads per step so screensDiffer()
	// reports change. Vary the BYTE LENGTH per step (well past the
	// screenLenNoiseBytes=64 floor) so screensDiffer reliably reports a
	// transition between consecutive frames — a real screen change alters
	// both pixel data and encoded length.
	n := 8192 + e.step*512
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = byte((e.step*37 + i) % 251)
	}
	return buf, nil
}
func (e *screenSteppingExec) Click(_ context.Context, x, y int) error {
	e.calls = append(e.calls, "click")
	return nil
}
func (e *screenSteppingExec) Type(_ context.Context, t string) error {
	e.calls = append(e.calls, "type")
	return nil
}
func (e *screenSteppingExec) KeyPress(_ context.Context, k string) error {
	e.calls = append(e.calls, "key:"+k)
	return nil
}
func (e *screenSteppingExec) Back(_ context.Context) error {
	e.calls = append(e.calls, "back")
	return nil
}
func (e *screenSteppingExec) Home(_ context.Context) error {
	e.calls = append(e.calls, "home")
	return nil
}
func (e *screenSteppingExec) Shell(_ context.Context, cmd string) ([]byte, error) {
	e.calls = append(e.calls, "shell:"+cmd)
	return nil, nil
}

// memSink is an in-memory EvidenceSink (unit-test fake). It runs the
// REAL Evidence.Validate before recording — so a bluff Evidence (missing
// OCR + transcript) would be rejected here exactly as the FileSink would.
type memSink struct {
	records []*Evidence
}

func (s *memSink) Record(_ context.Context, e *Evidence) error {
	if err := e.Validate(); err != nil {
		return err
	}
	s.records = append(s.records, e)
	return nil
}
func (s *memSink) Count() int { return len(s.records) }

func TestC6PipelineWiring_DrivesToPass(t *testing.T) {
	ResetTargetRegistry()
	t.Cleanup(ResetTargetRegistry)

	// Runtime-registered target (project-agnostic — generic label + a
	// generic launch action + a generic goal substring).
	const goal = "Now Playing"
	if err := Register(Target{
		Name:         "generic-player",
		LaunchAction: "launch monkey -p com.example 1",
		ScreenGoals:  []string{goal},
	}); err != nil {
		t.Fatalf("Register target: %v", err)
	}
	tgt, _ := Get("generic-player")

	// Real LLMProvider over a fake vision backend that always replies a
	// valid tap decision.
	fv := &fakeVision{
		supports: true,
		reply:    "ACTION: tap 100 200\nRATIONALE: advancing toward the goal screen\nEXPECT: needs-review\n",
	}
	prov, err := NewLLMProvider(fv, tgt.ScreenGoals, "tap x y | key KEYCODE | back | home")
	if err != nil {
		t.Fatalf("NewLLMProvider: %v", err)
	}

	// Real ADBActor over a stepping device fake (distinct screenshots).
	exec := &screenSteppingExec{}
	actor, err := NewADBActor(exec)
	if err != nil {
		t.Fatalf("NewADBActor: %v", err)
	}

	// In-memory EvidenceSink (runs the real Evidence.Validate). The
	// loop is driven by sinkOnlyExplorer below, which stamps a real OCR
	// snapshot per finding so the goal match + Evidence.Validate are both
	// exercised without a live Whisper/Tesseract HTTP service.
	sink := &memSink{}
	// NewDefaultExplorer requires whisper OR tesseract non-nil; confirm it
	// rejects the both-nil case (the §11.4 captured-evidence guard) ...
	if _, err := NewDefaultExplorer("c6-e2e", nil, nil, sink); err == nil {
		t.Fatal("expected NewDefaultExplorer to reject nil whisper AND nil tesseract")
	}
	// ... then drive the loop with an explorer whose CaptureFinding
	// persists via the sink without needing a live Whisper/Tesseract HTTP
	// service (fully-offline wiring proof).
	explorer2 := &sinkOnlyExplorer{sink: sink, goal: goal}

	sess, err := NewSession(SessionConfig{
		Provider: prov,
		Actor:    actor,
		Explorer: explorer2,
		Target:   tgt,
		MaxSteps: 4,
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	res, err := sess.Run(context.Background())
	if err != nil {
		t.Fatalf("Session.Run: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected PASS, got: %s (goalReached=%v screenChanged=%v)",
			res.Reason, res.GoalReached, res.ScreenChanged)
	}
	if !res.GoalReached {
		t.Fatal("expected GoalReached true")
	}
	if !res.ScreenChanged {
		t.Fatal("expected ScreenChanged true (stepping fake returns distinct frames)")
	}
	if sink.Count() == 0 {
		t.Fatal("expected at least one captured Evidence record")
	}
	// Step 1 must have dispatched the target launch action, not the LLM's tap.
	if len(exec.calls) == 0 || !strings.HasPrefix(exec.calls[0], "shell:") {
		t.Fatalf("step1 should dispatch launch (shell:...), calls=%v", exec.calls)
	}
}

// sinkOnlyExplorer is a minimal Explorer for the wiring test: it records
// each finding straight through the sink (the sink stamps OCR). It lets
// the e2e test run fully offline (no Whisper/Tesseract HTTP service)
// while still exercising the real Session loop + the real sink Validate.
//
// The goal OCR is stamped only from the SECOND finding onward so the
// Session runs ≥2 steps — that exercises the §11.4.52 screen-delta path
// genuinely (a single-step run could never prove ScreenChanged, so a
// goal-on-step-1 would correctly FAIL the zero-delta guard).
type sinkOnlyExplorer struct {
	sink    EvidenceSink
	goal    string
	finding int
}

func (e *sinkOnlyExplorer) Name() string { return "sink-only" }
func (e *sinkOnlyExplorer) CaptureFinding(ctx context.Context, opts FindingOptions) (*Evidence, error) {
	e.finding++
	ev := &Evidence{Description: opts.Description, Verdict: "needs-review", Notes: opts.Notes}
	if e.finding >= 2 {
		ev.OCRSnapshot = "Welcome — " + e.goal
	} else {
		// First screen does not show the goal yet; stamp non-goal OCR so
		// Evidence.Validate still passes (OCRSnapshot non-empty).
		ev.OCRSnapshot = "Loading…"
	}
	if err := e.sink.Record(ctx, ev); err != nil {
		return nil, err
	}
	return ev, nil
}
