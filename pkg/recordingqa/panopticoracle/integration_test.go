// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Package panopticoracle integration test: drives the REAL recorded ensemble
// mp4 end-to-end THROUGH the HelixQA recordingqa bank (TRV-ENSEMBLE-001) + the
// real Panoptic recvalidate oracle, proving the "reuse HelixQA" loop is closed:
//
//	banks/tui-recording-validation.yaml (TRV-ENSEMBLE-001 options as DATA)
//	  → pkg/recordingqa.Spec
//	    → pkg/recordingqa.Validator (the orchestrator)
//	      → panopticoracle adapter (this package)
//	        → Panoptic recvalidate CLI (ffmpeg + tesseract OCR over the mp4)
//	          → JSON report → recordingqa verdict.
//
// HONEST SKIP (§11.4.3 / §11.4.123). The test SKIPs with a clear reason — never
// a fake PASS — when any prerequisite is genuinely absent: the recorded mp4,
// ffmpeg, tesseract, the sibling Panoptic checkout, or the bank file. The
// prerequisite paths are integration-local (the sibling repo layout) and may be
// overridden via env so the test stays decoupled (CONST-051(B)).
//
// Run with:
//
//	go test -tags=integration -run TestEnsembleRecording_ThroughBank_RealPanoptic \
//	  -v -count=1 -timeout 600s ./pkg/recordingqa/panopticoracle/...
package panopticoracle

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/conduit"
	"digital.vasic.helixqa/pkg/recordingqa"
	"digital.vasic.helixqa/pkg/testbank"
)

const (
	bankEntryID = "TRV-ENSEMBLE-001"
	// envVideo overrides the recorded mp4 path; defaults to the conductor's
	// canonical ensemble recording.
	envVideo = "HELIXQA_ENSEMBLE_MP4"
	// envPanopticDir overrides the sibling Panoptic checkout dir.
	envPanopticDir = "HELIXQA_PANOPTIC_DIR"
	// envBank overrides the bank file path.
	envBank = "HELIXQA_RECORDING_BANK"
)

func TestEnsembleRecording_ThroughBank_RealPanoptic(t *testing.T) {
	// --- prerequisite: ffmpeg + tesseract (the recvalidate OCR engine) ---
	for _, bin := range []string{"ffmpeg", "tesseract"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("SKIP (§11.4.3): %s not on PATH — Panoptic recvalidate OCR cannot run", bin) // SKIP-OK: #env-ffmpeg-tesseract-missing
		}
	}

	// --- prerequisite: the recorded mp4 ---
	mp4 := os.Getenv(envVideo)
	if mp4 == "" {
		mp4 = "/tmp/helix_recordings/video2-helix-agent-ensemble.mp4"
	}
	if fi, err := os.Stat(mp4); err != nil || fi.Size() == 0 {
		t.Skipf("SKIP (§11.4.3): recorded ensemble mp4 absent/empty at %s (set %s to override)", mp4, envVideo) // SKIP-OK: #fixture-ensemble-mp4-absent
	}

	// --- prerequisite: the sibling Panoptic checkout (integration: local sibling) ---
	panopticDir := os.Getenv(envPanopticDir)
	if panopticDir == "" {
		// integration: local sibling — helix_qa and panoptic are siblings under
		// <repo-root>/submodules/. Resolve from this test file's location.
		wd, _ := os.Getwd() // .../helix_qa/pkg/recordingqa/panopticoracle
		panopticDir = filepath.Clean(filepath.Join(wd, "..", "..", "..", "..", "panoptic"))
	}
	if _, err := os.Stat(filepath.Join(panopticDir, "main.go")); err != nil {
		t.Skipf("SKIP (§11.4.3): sibling Panoptic checkout not found at %s (set %s to override)", panopticDir, envPanopticDir) // SKIP-OK: #dep-sibling-panoptic-absent
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("SKIP (§11.4.3): go toolchain not on PATH — cannot `go run` Panoptic") // SKIP-OK: #env-go-toolchain-missing
	}

	// --- load the TRV-ENSEMBLE-001 bank entry + extract recvalidate options ---
	bankPath := os.Getenv(envBank)
	if bankPath == "" {
		wd, _ := os.Getwd()
		bankPath = filepath.Clean(filepath.Join(wd, "..", "..", "..", "banks", "tui-recording-validation.yaml"))
	}
	if _, err := os.Stat(bankPath); err != nil {
		t.Skipf("SKIP (§11.4.3): recording-validation bank not found at %s (set %s to override)", bankPath, envBank) // SKIP-OK: #fixture-recording-bank-absent
	}
	bf, err := testbank.LoadFile(bankPath)
	if err != nil {
		t.Fatalf("load bank %s: %v", bankPath, err)
	}
	entry := findCase(bf, bankEntryID)
	if entry == nil {
		t.Fatalf("bank entry %s not found in %s", bankEntryID, bankPath)
	}

	opts := extractRecvalidateOptions(t, entry)
	// The real end-to-end Panoptic run drives the OCR-LITERAL prompt substrings
	// (expected_prompts) — recvalidate `--prompt` locates each on screen and
	// asserts a real post-"AI:" reply follows. These flow through the
	// recordingqa Spec.ExpectedReplies → VideoOptions.ExpectedReplies → the
	// adapter's `--prompt` flags.
	if len(opts.expectedPrompts) == 0 {
		t.Fatalf("bank entry %s carried no expected_prompts — cannot assert real replies on the recording", bankEntryID)
	}
	t.Logf("bank %s options: prompts=%v markers=%v chrome=%d-patterns model=%q preprocess=%q",
		bankEntryID, opts.expectedPrompts, opts.replyMarkers, len(opts.chromePatterns), opts.expectedModel, opts.preprocessVF)

	// --- build the REAL Panoptic-backed VideoValidator (decoupled: go run .) ---
	framesDir := filepath.Join(t.TempDir(), "frames_ensemble")
	jsonOut := filepath.Join(t.TempDir(), "frames_ensemble_findings.json")
	oracle := New(Config{
		Command:      []string{"go", "run", "."},
		Dir:          panopticDir,
		Model:        opts.expectedModel,
		FramesDir:    framesDir,
		JSONOutPath:  jsonOut,
		PreprocessVF: opts.preprocessVF, // bank-supplied input adaptation (e.g. negate)
	})

	// --- run the bank entry's options through the recordingqa orchestrator ---
	// No stderr log was captured alongside this recording, so the log oracle is
	// opted out (RequireLog=false). The video oracle (real Panoptic) carries the
	// PASS/FAIL verdict — the point of this test.
	orch := &recordingqa.Validator{Video: oracle}
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Minute)
	defer cancel()

	res := orch.Validate(ctx, recordingqa.Spec{
		ChallengeID:        entry.EffectiveChallengeID(),
		RecordingPath:      mp4,
		ExpectedReplies:    opts.expectedPrompts,
		ReplyMarkers:       opts.replyMarkers,
		ChromeLinePatterns: opts.chromePatterns,
		RequireLog:         false,
	})

	t.Logf("recordingqa verdict: %s — %s", res.Verdict, res.Reason)
	t.Logf("video oracle detail: %s", res.Video.Detail)
	t.Logf("matched replies: %v", res.Video.MatchedReplies)
	t.Logf("evidence paths: %v", res.EvidencePaths)

	if res.Verdict == conduit.VerdictSkip {
		t.Skipf("SKIP (§11.4.3): Panoptic oracle could not run end-to-end: %s", res.Reason) // SKIP-OK: #dep-panoptic-oracle-skip
	}
	if res.Verdict != conduit.VerdictPass {
		t.Fatalf("the real ensemble recording must PASS through the bank + Panoptic oracle, got %s (%s)",
			res.Verdict, res.Reason)
	}
	// Anti-bluff: a PASS MUST cite the mp4 + the analyzed-frames artefact.
	sawMp4, sawFrames := false, false
	for _, p := range res.EvidencePaths {
		if p == mp4 {
			sawMp4 = true
		}
		if p == res.Video.AnalyzedFrames && res.Video.AnalyzedFrames != "" {
			sawFrames = true
		}
	}
	if !sawMp4 {
		t.Errorf("PASS must cite the recorded mp4 as evidence; got %v", res.EvidencePaths)
	}
	if !sawFrames {
		t.Errorf("PASS must cite the Panoptic analyzed-frames artefact as evidence; got %v", res.EvidencePaths)
	}
	if len(res.Video.MatchedReplies) != len(opts.expectedPrompts) {
		t.Errorf("expected all %d prompt-replies matched, got %v", len(opts.expectedPrompts), res.Video.MatchedReplies)
	}
}

func findCase(bf *testbank.BankFile, id string) *testbank.TestCase {
	for i := range bf.TestCases {
		if bf.TestCases[i].ID == id {
			return &bf.TestCases[i]
		}
	}
	return nil
}

// bankOptions is the typed view of metadata.recvalidate_options that drives the
// real Panoptic run — proving the bank entry's DATA drives the validation, not a
// re-hardcode in the test.
type bankOptions struct {
	replyMarkers    []string
	chromePatterns  []string
	expectedReplies []string // ordinal reply markers (forwarding-contract tests)
	expectedPrompts []string // OCR-literal prompt substrings (drive the real run)
	expectedModel   string
	preprocessVF    string
}

// extractRecvalidateOptions reads metadata.recvalidate_options.{reply_markers,
// chrome_line_patterns,expected_replies,expected_prompts,expected_model_visible,
// preprocess_vf} from the bank entry.
func extractRecvalidateOptions(t *testing.T, tc *testbank.TestCase) bankOptions {
	t.Helper()
	raw, ok := tc.Metadata["recvalidate_options"]
	if !ok {
		t.Fatalf("bank entry %s has no metadata.recvalidate_options", tc.ID)
	}
	m, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("metadata.recvalidate_options is not a map (got %T)", raw)
	}
	out := bankOptions{
		replyMarkers:    toStringSlice(m["reply_markers"]),
		chromePatterns:  toStringSlice(m["chrome_line_patterns"]),
		expectedReplies: toStringSlice(m["expected_replies"]),
		expectedPrompts: toStringSlice(m["expected_prompts"]),
	}
	if s, ok := m["expected_model_visible"].(string); ok {
		out.expectedModel = s
	}
	if s, ok := m["preprocess_vf"].(string); ok {
		out.preprocessVF = s
	}
	return out
}

func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
