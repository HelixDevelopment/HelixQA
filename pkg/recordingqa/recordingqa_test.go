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
	return VideoValidatorFunc(func(_ context.Context, _ string, opts VideoOptions) (VideoResult, error) {
		return VideoResult{
			MatchedReplies: opts.ExpectedReplies,
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

	badVideo := VideoValidatorFunc(func(_ context.Context, _ string, _ VideoOptions) (VideoResult, error) {
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

	missingReplyVideo := VideoValidatorFunc(func(_ context.Context, _ string, opts VideoOptions) (VideoResult, error) {
		return VideoResult{
			MissingReplies: opts.ExpectedReplies, // none matched
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
	unavailable := VideoValidatorFunc(func(_ context.Context, _ string, _ VideoOptions) (VideoResult, error) {
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
	noEvidenceVideo := VideoValidatorFunc(func(_ context.Context, _ string, opts VideoOptions) (VideoResult, error) {
		return VideoResult{MatchedReplies: opts.ExpectedReplies, AnalyzedFrames: ""}, nil
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

// ---------------------------------------------------------------------------
// Option-forwarding tests (ChromeLinePatterns + ReplyMarkers).
//
// SCOPE NOTE (§11.4.3 honesty). These tests do NOT run a real OCR / ffmpeg
// pass — that lives in the injected VideoValidator (the Panoptic recvalidate
// validator or cmd/recording-analyzer, which the conductor wires). What
// recordingqa OWNS, and what these tests pin, is the FORWARDING contract: the
// consumer-supplied ChromeLinePatterns + ReplyMarkers on the Spec MUST reach
// the injected VideoValidator verbatim via VideoOptions. To prove that without
// a real OCR run, the injected validator here is a faithful chrome-aware STUB
// that reproduces the Panoptic recvalidate semantics over a per-frame text
// fixture: it counts a frame as a real reply only when, AFTER applying the
// supplied ReplyMarkers + ChromeLinePatterns, a prose line survives. If the
// forwarding is dropped, the stub sees no patterns, treats ambient chrome as a
// reply, and the golden-bad case PASSes — which the paired mutation asserts
// FAILs the test. The full OCR end-to-end is the bank's dispatch path.
// ---------------------------------------------------------------------------

// chromeAwareVideo is a faithful stub of the Panoptic recvalidate video
// oracle. It receives the per-frame OCR text fixture and reproduces
// recvalidate's chrome-exclusion + reply-marker semantics using the
// VideoOptions FORWARDED by recordingqa. It is NOT a real OCR run — it exists
// to prove the option-forwarding contract (see SCOPE NOTE above).
//
// Logic per frame line, mirroring Panoptic recvalidate:
//   - a line matching ANY ChromeLinePatterns regex is ambient chrome → excluded
//     from reply-prose counting;
//   - the real reply is the prose AFTER the first ReplyMarkers prefix on a
//     surviving (non-chrome) line; if ReplyMarkers is empty, any surviving
//     prose line counts (generic-chat default);
//   - error tokens ("error", "no provider", "panic") on a surviving line are
//     reported as ErrorTextFound.
// It reports each expectedReply as matched only if it appears in a counted
// reply line. A reply that survives ONLY because chrome was (wrongly) counted
// is the §11.4.137 bluff this stub is built to expose.
func chromeAwareVideo(framesPath string, frameLines []string) VideoValidator {
	return VideoValidatorFunc(func(_ context.Context, _ string, opts VideoOptions) (VideoResult, error) {
		chromeREs := make([]*regexp.Regexp, 0, len(opts.ChromeLinePatterns))
		for _, p := range opts.ChromeLinePatterns {
			chromeREs = append(chromeREs, regexp.MustCompile("(?i)"+p))
		}
		isChrome := func(line string) bool {
			for _, re := range chromeREs {
				if re.MatchString(line) {
					return true
				}
			}
			return false
		}
		// replyProse returns the prose part of a line after the first reply
		// marker, or the whole line when no markers are configured.
		replyProse := func(line string) (string, bool) {
			if len(opts.ReplyMarkers) == 0 {
				return line, true
			}
			for _, m := range opts.ReplyMarkers {
				if idx := indexOfMarker(line, m); idx >= 0 {
					return line[idx+len(m):], true
				}
			}
			return "", false
		}

		var replyText []string
		var errs []string
		for _, line := range frameLines {
			if isChrome(line) {
				continue // ambient chrome excluded from reply counting
			}
			prose, ok := replyProse(line)
			if !ok {
				continue // no reply marker on this surviving line
			}
			lower := toLower(prose)
			for _, tok := range []string{"error", "no provider", "panic"} {
				if containsStr(lower, tok) {
					errs = append(errs, prose)
				}
			}
			replyText = append(replyText, prose)
		}

		joined := joinLines(replyText)
		var matched, missing []string
		for _, want := range opts.ExpectedReplies {
			if containsStr(joined, want) {
				matched = append(matched, want)
			} else {
				missing = append(missing, want)
			}
		}
		return VideoResult{
			MatchedReplies: matched,
			MissingReplies: missing,
			ErrorTextFound: errs,
			AnalyzedFrames: framesPath,
			Detail:         "chrome-aware recvalidate stub",
		}, nil
	})
}

// TestValidate_ChromeAndMarkersForwarded_GoldenGood_Pass: a chat-TUI
// recording whose ambient sidebar chrome ("Commands  Models  Settings") sits
// above a REAL "AI:" reply. With the chrome patterns + reply marker supplied
// on the Spec and FORWARDED to the oracle, the chrome is excluded and the
// real reply ("4") is counted → PASS.
func TestValidate_ChromeAndMarkersForwarded_GoldenGood_Pass(t *testing.T) {
	dir := t.TempDir()
	mp4 := writeFile(t, dir, "session.mp4", "nonempty-mp4")
	frames := writeFile(t, dir, "frames.jsonl", `{"frame":1}`)
	logp := writeFile(t, dir, "tui.stderr.log", "INFO provider=ollama healthy\nINFO model llama3.2 ready\n")

	frameLines := []string{
		"Commands   Models   Settings", // sidebar chrome
		"Helix Agent ensemble  |  llama3.2", // status panel chrome
		"You: What is 2+2?",                 // user echo (not a reply marker)
		"AI: The answer is 4.",              // the REAL reply
	}
	v := &Validator{Video: chromeAwareVideo(frames, frameLines)}

	res := v.Validate(context.Background(), Spec{
		ChallengeID:        "tui-rec-chrome-good",
		RecordingPath:      mp4,
		StderrLogPath:      logp,
		ExpectedReplies:    []string{"4"},
		ReplyMarkers:       []string{"AI:"},
		ChromeLinePatterns: []string{`commands\s+models\s+settings`, `helix agent ensemble`},
		RequireLog:         true,
	})
	if res.Verdict != conduit.VerdictPass {
		t.Fatalf("golden-good (real AI: reply, chrome excluded) must PASS, got %s (%s)", res.Verdict, res.Reason)
	}
}

// TestValidate_ChromeAndMarkersForwarded_GoldenBad_Fail: a recording where
// the model NEVER produced a reply — there is NO "AI:" reply line at all. The
// expected token "4" appears ONLY inside an ambient chrome line
// ("Models  Settings  Help   (4 models loaded)"). The FAIL therefore hinges
// PURELY on chrome exclusion:
//
//   - With ChromeLinePatterns FORWARDED → the chrome line is excluded, NO
//     surviving line carries an "AI:" reply, the expected token "4" is NOT in
//     any counted reply → MissingReplies non-empty → FAIL (correct: the model
//     never replied).
//   - With the forwarding DROPPED → the chrome line is (wrongly) counted as
//     reply prose, "4" is "matched" inside "4 models loaded" → PASS (the
//     §11.4.137 chrome-as-reply bluff).
//
// The verdict here depends ONLY on whether ChromeLinePatterns reached the
// oracle — nothing else (no error-token interplay, no marker on the chrome
// line). The sub-test below is the paired §1.1 mutation: it re-runs the SAME
// Spec but with the Spec's ChromeLinePatterns blanked (reproducing exactly the
// production regression "Spec.ChromeLinePatterns not threaded into
// VideoOptions") and asserts the golden-bad recording then bluffs PASS —
// proving the FAIL above genuinely depends on the forwarding.
func TestValidate_ChromeAndMarkersForwarded_GoldenBad_Fail(t *testing.T) {
	dir := t.TempDir()
	mp4 := writeFile(t, dir, "session.mp4", "nonempty-mp4")
	frames := writeFile(t, dir, "frames.jsonl", `{"frame":1}`)
	logp := writeFile(t, dir, "tui.stderr.log", "INFO started\n")

	// On-screen content: an assistant turn ("AI:" marker) whose CONTENT is an
	// ambient command/menu chrome strip that happens to contain the token "4"
	// (inside "4 models loaded") — this is the §11.4.137 chrome-as-reply case:
	// the marker is present but the "reply" is really chrome, not a dialogue
	// answer. The model did NOT actually answer the prompt. The chrome pattern
	// matches this exact line, so chrome exclusion is the ONLY thing that
	// distinguishes a correct FAIL from a bluff PASS.
	frameLines := []string{
		"You: What is 2+2?",                              // user echo
		"AI:  Models  Settings  Help   (4 models loaded)", // marker present, but content is chrome
	}
	expected := []string{"4"}
	markers := []string{"AI:"}
	chrome := []string{`models\s+settings\s+help`}

	baseSpec := func(chromePatterns []string) Spec {
		return Spec{
			ChallengeID:        "tui-rec-chrome-bad",
			RecordingPath:      mp4,
			StderrLogPath:      logp,
			ExpectedReplies:    expected,
			ReplyMarkers:       markers,
			ChromeLinePatterns: chromePatterns,
		}
	}

	v := &Validator{Video: chromeAwareVideo(frames, frameLines)}

	// --- Golden-bad with chrome patterns supplied (MUST FAIL) ---
	res := v.Validate(context.Background(), baseSpec(chrome))
	if res.Verdict != conduit.VerdictFail {
		t.Fatalf("golden-bad (no real reply; token only in excluded chrome) must FAIL, got %s (%s)",
			res.Verdict, res.Reason)
	}

	// --- PAIRED §1.1 MUTATION: Spec.ChromeLinePatterns dropped ---
	// Reproduce the production regression "Spec.ChromeLinePatterns never
	// reaches VideoOptions" by passing an empty chrome list through the SAME
	// production Validate path. The chrome line is now counted as prose, "4"
	// matches inside it, and the broken recording bluffs PASS — proving the
	// FAIL above depends solely on the chrome-pattern forwarding.
	t.Run("mutation_drop_chrome_forwarding_bluffs_pass", func(t *testing.T) {
		mutRes := v.Validate(context.Background(), baseSpec(nil))
		if mutRes.Verdict != conduit.VerdictPass {
			t.Fatalf("MUTATION-PROOF: with chrome patterns dropped, the golden-bad recording "+
				"must bluff PASS (proving the golden-bad FAIL depends on chrome forwarding), got %s (%s)",
				mutRes.Verdict, mutRes.Reason)
		}
	})
}

// TestValidate_ReplyMarkersForwarded_GoldenBad_Fail: isolates the
// ReplyMarkers forwarding. The expected token "4" appears ONLY in the USER's
// echoed prompt line ("You: ... 4 ..."); the real assistant turn ("AI: ...")
// is an apology with no answer. The FAIL hinges PURELY on reply-marker
// anchoring:
//
//   - With ReplyMarkers=["AI:"] FORWARDED → only prose AFTER "AI:" is counted
//     as the reply; the user line is NOT a reply, so "4" is NOT matched →
//     MissingReplies non-empty → FAIL (correct: the model didn't answer).
//   - With ReplyMarkers DROPPED → the generic-chat default counts EVERY line
//     as prose; the user's own "...is it 4?" line matches "4" → PASS (the
//     bluff: the user's prompt is mistaken for the model's reply).
//
// The paired §1.1 sub-test blanks Spec.ReplyMarkers (the production
// "ReplyMarkers not threaded into VideoOptions" regression) and asserts the
// recording then bluffs PASS — proving the FAIL depends on marker forwarding.
func TestValidate_ReplyMarkersForwarded_GoldenBad_Fail(t *testing.T) {
	dir := t.TempDir()
	mp4 := writeFile(t, dir, "session.mp4", "nonempty-mp4")
	frames := writeFile(t, dir, "frames.jsonl", `{"frame":1}`)
	logp := writeFile(t, dir, "tui.stderr.log", "INFO ok\n")

	// The token "4" is in the USER echo only; the AI turn gives no answer.
	frameLines := []string{
		"You: is the answer 4?",                 // user echo carries the token
		"AI: I'm not sure, could you rephrase?", // real reply has no "4"
	}
	expected := []string{"4"}

	baseSpec := func(markers []string) Spec {
		return Spec{
			ChallengeID:     "tui-rec-marker-bad",
			RecordingPath:   mp4,
			StderrLogPath:   logp,
			ExpectedReplies: expected,
			ReplyMarkers:    markers,
		}
	}

	v := &Validator{Video: chromeAwareVideo(frames, frameLines)}

	res := v.Validate(context.Background(), baseSpec([]string{"AI:"}))
	if res.Verdict != conduit.VerdictFail {
		t.Fatalf("golden-bad (token only in user echo; AI gave no answer) must FAIL, got %s (%s)",
			res.Verdict, res.Reason)
	}

	t.Run("mutation_drop_reply_marker_forwarding_bluffs_pass", func(t *testing.T) {
		mutRes := v.Validate(context.Background(), baseSpec(nil))
		if mutRes.Verdict != conduit.VerdictPass {
			t.Fatalf("MUTATION-PROOF: with reply markers dropped, the user's echoed prompt "+
				"is mistaken for the reply so the recording bluffs PASS (proving the FAIL "+
				"depends on reply-marker forwarding), got %s (%s)", mutRes.Verdict, mutRes.Reason)
		}
	})
}

// --- tiny string helpers (no extra imports) ---

func toLower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func containsStr(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func indexOfMarker(line, marker string) int {
	for i := 0; i+len(marker) <= len(line); i++ {
		if line[i:i+len(marker)] == marker {
			return i
		}
	}
	return -1
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

// toolAbsentErr is a minimal error type for the analyzer-unavailable test.
type toolAbsentErr struct{}

func (e *toolAbsentErr) Error() string { return "ffprobe not found on PATH" }
