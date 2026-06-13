// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package panopticoracle

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"digital.vasic.helixqa/pkg/conduit"
	"digital.vasic.helixqa/pkg/recordingqa"
)

// TestBuildArgs_ForwardsOptionsVerbatim asserts the recordingqa VideoOptions
// (expected replies + reply markers + chrome patterns) are forwarded to
// recvalidate as the correct repeatable flags, in the documented order, and
// that the cfg fields (model / frames-dir / json-out) become their flags. This
// pins the FLAG GRAMMAR contract the adapter relies on.
func TestBuildArgs_ForwardsOptionsVerbatim(t *testing.T) {
	v := &validator{cfg: Config{
		Command:     []string{"go", "run", "."},
		Model:       "ensemble",
		FramesDir:   "/tmp/frames",
		JSONOutPath: "/tmp/out.json",
	}}
	opts := recordingqa.VideoOptions{
		ExpectedReplies:    []string{"ensemble", "second", "third"},
		ReplyMarkers:       []string{"AI:"},
		ChromeLinePatterns: []string{`(?i)commands\s+models\s+settings`, `(?i)helix agent ensemble`},
	}
	got := v.buildArgs("/tmp/v2.mp4", opts)

	mustContainPair := func(flag, val string) {
		t.Helper()
		for i := 0; i+1 < len(got); i++ {
			if got[i] == flag && got[i+1] == val {
				return
			}
		}
		t.Fatalf("expected flag pair %q %q in args %v", flag, val, got)
	}

	if got[0] != "recvalidate" {
		t.Fatalf("first arg must be the recvalidate subcommand, got %q", got[0])
	}
	mustContainPair("--video", "/tmp/v2.mp4")
	mustContainPair("--model", "ensemble")
	mustContainPair("--frames-dir", "/tmp/frames")
	mustContainPair("--json-out", "/tmp/out.json")
	mustContainPair("--prompt", "ensemble")
	mustContainPair("--prompt", "second")
	mustContainPair("--prompt", "third")
	mustContainPair("--reply-marker", "AI:")
	mustContainPair("--chrome-pattern", `(?i)commands\s+models\s+settings`)
	mustContainPair("--chrome-pattern", `(?i)helix agent ensemble`)

	// --keep-frames must accompany --frames-dir so frames are retained.
	foundKeep := false
	for _, a := range got {
		if a == "--keep-frames" {
			foundKeep = true
		}
	}
	if !foundKeep {
		t.Errorf("--frames-dir without --keep-frames would not retain evidence; args=%v", got)
	}
}

// TestMapReport_Pass: a PASS report → every expected reply matched, no error
// text, a real frames-dir evidence path.
func TestMapReport_Pass(t *testing.T) {
	dir := t.TempDir()
	framesDir := filepath.Join(dir, "frames")
	if err := os.Mkdir(framesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(framesDir, "frame_1.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	v := &validator{cfg: Config{}}
	rep := &report{
		Pass:       true,
		FrameCount: 95,
		FramesDir:  framesDir,
		Checks: []checkResult{
			{Name: "no_error_tokens", Pass: true},
			{Name: "reply_to_prompt_1", Pass: true},
			{Name: "intended_model_selected", Pass: true},
		},
	}
	opts := recordingqa.VideoOptions{ExpectedReplies: []string{"ensemble", "second", "third"}}
	res := v.mapReport(rep, opts, rep.FramesDir)

	if len(res.MissingReplies) != 0 {
		t.Errorf("PASS report must have no missing replies, got %v", res.MissingReplies)
	}
	if len(res.ErrorTextFound) != 0 {
		t.Errorf("PASS report must have no error text, got %v", res.ErrorTextFound)
	}
	if len(res.MatchedReplies) != 3 {
		t.Errorf("PASS report must mark all 3 expected replies matched, got %v", res.MatchedReplies)
	}
	if res.AnalyzedFrames != framesDir {
		t.Errorf("expected frames dir as evidence path, got %q", res.AnalyzedFrames)
	}
}

// TestMapReport_MissingReply_Fail: a FAIL report whose reply check failed maps
// to MissingReplies so the recordingqa orchestrator FAILs — this is the
// anti-bluff seam: a broken recording surfaces as FAIL, never PASS.
func TestMapReport_MissingReply_Fail(t *testing.T) {
	v := &validator{cfg: Config{}}
	rep := &report{
		Pass:       false,
		FrameCount: 40,
		VideoPath:  "/tmp/v2.mp4",
		Checks: []checkResult{
			{Name: "no_error_tokens", Pass: true},
			{Name: "reply_to_prompt_2", Pass: false, Detail: "no prose after AI: for prompt 2"},
			{Name: "intended_model_selected", Pass: true},
		},
	}
	opts := recordingqa.VideoOptions{ExpectedReplies: []string{"ensemble", "second", "third"}}
	res := v.mapReport(rep, opts, rep.VideoPath)

	if len(res.MissingReplies) == 0 {
		t.Fatalf("FAIL report with a failed reply check must report missing replies")
	}

	// End-to-end through the recordingqa orchestrator: the adapter's FAIL
	// must drive the verdict to FAIL, not PASS.
	dir := t.TempDir()
	mp4 := filepath.Join(dir, "session.mp4")
	if err := os.WriteFile(mp4, []byte("nonempty-mp4"), 0o644); err != nil {
		t.Fatal(err)
	}
	logp := filepath.Join(dir, "tui.stderr.log")
	if err := os.WriteFile(logp, []byte("INFO ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub := recordingqa.VideoValidatorFunc(func(_ context.Context, _ string, o recordingqa.VideoOptions) (recordingqa.VideoResult, error) {
		return v.mapReport(rep, o, rep.VideoPath), nil
	})
	orch := &recordingqa.Validator{Video: stub}
	got := orch.Validate(context.Background(), recordingqa.Spec{
		ChallengeID:     "panopticoracle-fail-map",
		RecordingPath:   mp4,
		StderrLogPath:   logp,
		ExpectedReplies: opts.ExpectedReplies,
	})
	if got.Verdict != conduit.VerdictFail {
		t.Fatalf("a recvalidate FAIL must drive the orchestrator to FAIL, got %s (%s)", got.Verdict, got.Reason)
	}
}

// TestMapReport_ErrorToken_Fail: a FAIL report whose error-token check failed
// surfaces the on-screen error text → orchestrator FAILs.
func TestMapReport_ErrorToken_Fail(t *testing.T) {
	v := &validator{cfg: Config{}}
	rep := &report{
		Pass:      false,
		VideoPath: "/tmp/v2.mp4",
		Checks: []checkResult{
			{Name: "no_error_tokens", Pass: false, Evidence: "no provider configured"},
		},
	}
	res := v.mapReport(rep, recordingqa.VideoOptions{}, rep.VideoPath)
	if len(res.ErrorTextFound) == 0 {
		t.Fatalf("a failed error-token check must surface the on-screen error text")
	}
}

// TestNew_EmptyCommand_Errors: an empty Command cannot reach Panoptic → Validate
// returns an error so the orchestrator SKIPs honestly (§11.4.3), never PASSes.
func TestNew_EmptyCommand_Errors(t *testing.T) {
	v := New(Config{})
	_, err := v.Validate(context.Background(), "/tmp/v2.mp4", recordingqa.VideoOptions{})
	if err == nil {
		t.Fatalf("empty Command must error (→ orchestrator SKIP), got nil")
	}
}
