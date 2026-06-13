// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package recordingqa

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"digital.vasic.helixqa/pkg/conduit"
)

// writeFile writes content and returns the path (test helper).
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// goodVideo is a VideoValidator that reports a goal-achieved recording:
// every expected reply matched, no error text, a real frames artefact.
func goodVideo(framesPath string) VideoValidator {
	return VideoValidatorFunc(func(_ context.Context, _ string, expected []string) (VideoResult, error) {
		return VideoResult{
			MatchedReplies: expected,
			AnalyzedFrames: framesPath,
			Detail:         "all expected replies rendered; no error overlay",
		}, nil
	})
}

// TestValidate_HappyPath_Pass: a non-empty mp4, an analyzer that matched
// every reply with no error text + a real frames artefact, and a clean
// stderr log → PASS with all three evidence paths captured.
func TestValidate_HappyPath_Pass(t *testing.T) {
	dir := t.TempDir()
	mp4 := writeFile(t, dir, "session.mp4", "\x00\x00fake-but-nonempty-mp4-bytes")
	frames := writeFile(t, dir, "frames.jsonl", `{"frame":1,"ocr":["What is 2+2?","4"]}`)
	logp := writeFile(t, dir, "tui.stderr.log", "INFO provider=ollama healthy\nINFO model llama3.2 ready\nINFO reply rendered\n")

	v := &Validator{Video: goodVideo(frames)}
	res := v.Validate(context.Background(), Spec{
		ChallengeID:     "tui-rec-001",
		RecordingPath:   mp4,
		StderrLogPath:   logp,
		ExpectedReplies: []string{"4"},
		RequireLog:      true,
	})

	if res.Verdict != conduit.VerdictPass {
		t.Fatalf("want PASS, got %s (%s)", res.Verdict, res.Reason)
	}
	// Anti-bluff: a PASS MUST cite every captured-evidence path.
	wantEvidence := map[string]bool{mp4: false, frames: false, logp: false}
	for _, p := range res.EvidencePaths {
		if _, ok := wantEvidence[p]; ok {
			wantEvidence[p] = true
		}
	}
	for p, seen := range wantEvidence {
		if !seen {
			t.Errorf("PASS missing evidence path: %s", p)
		}
	}
}

// TestValidate_ZeroByteRecording_Fail: the canonical §11.4.5 0-byte-mp4
// bluff MUST come out FAIL even though a (would-be) good analyzer is wired.
func TestValidate_ZeroByteRecording_Fail(t *testing.T) {
	dir := t.TempDir()
	mp4 := writeFile(t, dir, "empty.mp4", "") // 0 bytes
	frames := writeFile(t, dir, "frames.jsonl", `{"frame":1}`)
	logp := writeFile(t, dir, "tui.stderr.log", "INFO ok\n")

	v := &Validator{Video: goodVideo(frames)}
	res := v.Validate(context.Background(), Spec{
		ChallengeID:   "tui-rec-zero",
		RecordingPath: mp4,
		StderrLogPath: logp,
	})
	if res.Verdict != conduit.VerdictFail {
		t.Fatalf("0-byte mp4 must FAIL, got %s (%s)", res.Verdict, res.Reason)
	}
}

// TestValidate_MissingRecording_Fail: a missing mp4 → FAIL.
func TestValidate_MissingRecording_Fail(t *testing.T) {
	dir := t.TempDir()
	v := &Validator{Video: goodVideo("x")}
	res := v.Validate(context.Background(), Spec{
		ChallengeID:   "tui-rec-missing",
		RecordingPath: filepath.Join(dir, "does-not-exist.mp4"),
	})
	if res.Verdict != conduit.VerdictFail {
		t.Fatalf("missing mp4 must FAIL, got %s (%s)", res.Verdict, res.Reason)
	}
}

// TestValidate_BadAnalysis_ErrorOverlay_Fail: ANTI-BLUFF paired mutation.
// The mp4 is non-empty and the log is clean, but the injected analyzer
// reports an error overlay on-screen (the user saw an error, not a reply).
// A green verdict here would be the exact §11.4 bluff — it MUST FAIL.
func TestValidate_BadAnalysis_ErrorOverlay_Fail(t *testing.T) {
	dir := t.TempDir()
	mp4 := writeFile(t, dir, "session.mp4", "nonempty")
	frames := writeFile(t, dir, "frames.jsonl", `{"frame":1,"ocr":["no provider configured"]}`)
	logp := writeFile(t, dir, "tui.stderr.log", "INFO started\n")

	badVideo := VideoValidatorFunc(func(_ context.Context, _ string, _ []string) (VideoResult, error) {
		return VideoResult{
			ErrorTextFound: []string{"no provider configured"},
			AnalyzedFrames: frames,
			Detail:         "error overlay rendered on terminal",
		}, nil
	})
	v := &Validator{Video: badVideo}
	res := v.Validate(context.Background(), Spec{
		ChallengeID:   "tui-rec-erroverlay",
		RecordingPath: mp4,
		StderrLogPath: logp,
	})
	if res.Verdict != conduit.VerdictFail {
		t.Fatalf("error-overlay recording must FAIL, got %s (%s)", res.Verdict, res.Reason)
	}
}

// TestValidate_BadAnalysis_MissingReply_Fail: ANTI-BLUFF. The analyzer
// reports the expected reply was NOT found on any frame — the prompt got no
// visible reply, so the goal was not achieved → FAIL.
func TestValidate_BadAnalysis_MissingReply_Fail(t *testing.T) {
	dir := t.TempDir()
	mp4 := writeFile(t, dir, "session.mp4", "nonempty")
	frames := writeFile(t, dir, "frames.jsonl", `{"frame":1,"ocr":["prompt typed but no reply"]}`)
	logp := writeFile(t, dir, "tui.stderr.log", "INFO ok\n")

	missingReplyVideo := VideoValidatorFunc(func(_ context.Context, _ string, expected []string) (VideoResult, error) {
		return VideoResult{
			MissingReplies: expected, // none matched
			AnalyzedFrames: frames,
		}, nil
	})
	v := &Validator{Video: missingReplyVideo}
	res := v.Validate(context.Background(), Spec{
		ChallengeID:     "tui-rec-noreply",
		RecordingPath:   mp4,
		StderrLogPath:   logp,
		ExpectedReplies: []string{"4"},
	})
	if res.Verdict != conduit.VerdictFail {
		t.Fatalf("missing-reply recording must FAIL, got %s (%s)", res.Verdict, res.Reason)
	}
}

// TestValidate_DirtyLog_Fail: ANTI-BLUFF. The mp4 is fine and the analyzer
// reports goal-achieved, but the stderr log contains a panic / "no provider"
// line — the LLM interaction was broken → FAIL.
func TestValidate_DirtyLog_Fail(t *testing.T) {
	dir := t.TempDir()
	mp4 := writeFile(t, dir, "session.mp4", "nonempty")
	frames := writeFile(t, dir, "frames.jsonl", `{"frame":1,"ocr":["4"]}`)
	logp := writeFile(t, dir, "tui.stderr.log",
		"INFO started\npanic: runtime error: nil provider\nWARNING 0 models available\n")

	v := &Validator{Video: goodVideo(frames)}
	res := v.Validate(context.Background(), Spec{
		ChallengeID:     "tui-rec-dirtylog",
		RecordingPath:   mp4,
		StderrLogPath:   logp,
		ExpectedReplies: []string{"4"},
	})
	if res.Verdict != conduit.VerdictFail {
		t.Fatalf("dirty-log recording must FAIL, got %s (%s)", res.Verdict, res.Reason)
	}
	if len(res.Log.Matches) == 0 {
		t.Errorf("expected log oracle to record at least one match")
	}
}

// TestValidate_RequiredLogMissing_Fail: when RequireLog is set, a missing
// stderr log is a FAIL (cannot prove the interaction was clean).
func TestValidate_RequiredLogMissing_Fail(t *testing.T) {
	dir := t.TempDir()
	mp4 := writeFile(t, dir, "session.mp4", "nonempty")
	frames := writeFile(t, dir, "frames.jsonl", `{"frame":1}`)

	v := &Validator{Video: goodVideo(frames)}
	res := v.Validate(context.Background(), Spec{
		ChallengeID:   "tui-rec-nolog",
		RecordingPath: mp4,
		StderrLogPath: filepath.Join(dir, "absent.log"),
		RequireLog:    true,
	})
	if res.Verdict != conduit.VerdictFail {
		t.Fatalf("required-but-missing log must FAIL, got %s (%s)", res.Verdict, res.Reason)
	}
}

// TestValidate_AnalyzerUnavailable_Skip: an analyzer that cannot run (tool
// absent / unreadable mp4) → honest SKIP-with-reason per §11.4.3, NEVER a
// fake PASS.
func TestValidate_AnalyzerUnavailable_Skip(t *testing.T) {
	dir := t.TempDir()
	mp4 := writeFile(t, dir, "session.mp4", "nonempty")
	unavailable := VideoValidatorFunc(func(_ context.Context, _ string, _ []string) (VideoResult, error) {
		return VideoResult{}, &toolAbsentErr{}
	})
	v := &Validator{Video: unavailable}
	res := v.Validate(context.Background(), Spec{
		ChallengeID:   "tui-rec-skip",
		RecordingPath: mp4,
	})
	if res.Verdict != conduit.VerdictSkip {
		t.Fatalf("analyzer-absent must SKIP, got %s (%s)", res.Verdict, res.Reason)
	}
}

// TestValidate_NoFramesArtefact_Fail: ANTI-BLUFF. The analyzer reports all
// replies matched and no error text, but cites NO analyzed-frames artefact —
// a PASS without evidence is a §11.4.69 bluff → FAIL.
func TestValidate_NoFramesArtefact_Fail(t *testing.T) {
	dir := t.TempDir()
	mp4 := writeFile(t, dir, "session.mp4", "nonempty")
	logp := writeFile(t, dir, "tui.stderr.log", "INFO ok\n")
	noEvidenceVideo := VideoValidatorFunc(func(_ context.Context, _ string, expected []string) (VideoResult, error) {
		return VideoResult{MatchedReplies: expected, AnalyzedFrames: ""}, nil
	})
	v := &Validator{Video: noEvidenceVideo}
	res := v.Validate(context.Background(), Spec{
		ChallengeID:     "tui-rec-noevidence",
		RecordingPath:   mp4,
		StderrLogPath:   logp,
		ExpectedReplies: []string{"4"},
	})
	if res.Verdict != conduit.VerdictFail {
		t.Fatalf("no-frames-evidence PASS-bluff must FAIL, got %s (%s)", res.Verdict, res.Reason)
	}
}

// TestValidate_CustomErrorPatterns: the consumer can replace the error
// pattern set (project-agnostic per CONST-051(B)).
func TestValidate_CustomErrorPatterns(t *testing.T) {
	dir := t.TempDir()
	mp4 := writeFile(t, dir, "session.mp4", "nonempty")
	frames := writeFile(t, dir, "frames.jsonl", `{"frame":1,"ocr":["4"]}`)
	logp := writeFile(t, dir, "tui.stderr.log", "INFO quota_exceeded from provider\n")

	v := &Validator{Video: goodVideo(frames)}
	res := v.Validate(context.Background(), Spec{
		ChallengeID:     "tui-rec-custom",
		RecordingPath:   mp4,
		StderrLogPath:   logp,
		ExpectedReplies: []string{"4"},
		ErrorPatterns:   []*regexp.Regexp{regexp.MustCompile(`(?i)quota_exceeded`)},
	})
	if res.Verdict != conduit.VerdictFail {
		t.Fatalf("custom error pattern must FAIL, got %s (%s)", res.Verdict, res.Reason)
	}
}

// toolAbsentErr is a minimal error type for the analyzer-unavailable test.
type toolAbsentErr struct{}

func (e *toolAbsentErr) Error() string { return "ffprobe not found on PATH" }
