// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-conduit-demo drives a realistic QA-session shape
// through a real conduit.Writer, capturing real evidence files, so the
// end-to-end conductor channel can be validated with a real producer
// + the real helixqa-conduit-monitor consumer running concurrently.
//
// This is NOT a mock: it exercises the actual conduit.Writer / Sink
// emit path, writes actual evidence artefacts to disk, and produces
// the actual JSONL stream + status snapshot a live HelixQA session
// would. It exists so the channel can be validated without a flashed
// device (which another stream owns).
//
//	helixqa-conduit-demo -dir <out> [-pace 250ms]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"digital.vasic.helixqa/pkg/conduit"
)

func main() {
	dir := flag.String("dir", ".", "output directory for the channel + evidence")
	pace := flag.Duration("pace", 250*time.Millisecond, "delay between emitted events")
	flag.Parse()

	evidenceDir := filepath.Join(*dir, "evidence")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	w, err := conduit.NewWriter(conduit.Config{Session: "demo-session", Dir: *dir})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer w.Close()
	fmt.Printf("producer: stream=%s status=%s\n", w.StreamPath(), w.StatusPath())

	step := func() { time.Sleep(*pace) }

	conduit.SessionStart(w, "demo-session", map[string]any{"platforms": []string{"android"}})
	step()

	// Phase: setup → doc-driven (two challenges) → report.
	conduit.PhaseStart(w, "setup", "android")
	conduit.Logf(w, "setup", "selected vision provider + built feature map")
	step()
	conduit.PhaseComplete(w, "setup", 500*time.Millisecond)
	step()

	conduit.PhaseStart(w, "doc-driven", "android")

	// Challenge 1 — PASS with real captured evidence.
	conduit.ChallengeStart(w, "TC-001-login", "android")
	conduit.ChallengeStep(w, "TC-001-login", "launch app via launcher icon")
	conduit.VisionCall(w, "doc-driven", 320*time.Millisecond, map[string]any{"model": "vision-x", "screens": 1})
	step()
	shot := filepath.Join(evidenceDir, "TC-001-login.screenshot.png")
	writeEvidence(shot, fakePNG())
	conduit.EvidenceCaptured(w, "TC-001-login", "screenshot", shot)
	conduit.LLMCall(w, "doc-driven", 410*time.Millisecond, map[string]any{"model": "chat-y", "tokens": 128})
	conduit.ChallengeVerdict(w, "TC-001-login", conduit.VerdictPass, "")
	step()

	// Challenge 2 — SKIP with a closed-set reason (geo-restricted).
	conduit.ChallengeStart(w, "TC-002-stream-geo", "android")
	conduit.ChallengeStep(w, "TC-002-stream-geo", "probe content API reachability")
	conduit.ChallengeVerdict(w, "TC-002-stream-geo", conduit.VerdictSkip, "geo_restricted")
	step()

	conduit.PhaseComplete(w, "doc-driven", 2*time.Second)
	step()

	conduit.PhaseStart(w, "report", "android")
	report := filepath.Join(evidenceDir, "qa-report.json")
	writeEvidence(report, []byte(`{"pass":1,"skip":1,"fail":0}`))
	conduit.EvidenceCaptured(w, "", "json", report)
	conduit.PhaseComplete(w, "report", 200*time.Millisecond)
	step()

	conduit.SessionEnd(w, "demo-session", conduit.VerdictPass, "1 pass, 1 skip, 0 fail")

	if err := w.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "producer error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("producer: done")
}

func writeEvidence(path string, data []byte) {
	_ = os.WriteFile(path, data, 0o644)
}

// fakePNG returns a tiny but non-empty byte slice standing in for a
// real screenshot artefact (non-zero size is the point — the monitor
// reports 0-byte evidence as a bluff signal).
func fakePNG() []byte {
	return []byte("\x89PNG\r\n\x1a\n-helixqa-conduit-demo-evidence-")
}
