// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package recordingqa orchestrates the validation of a RECORDED ARTIFACT
// (an mp4 screen recording of a session) against the goal it was supposed
// to achieve, and surfaces a structured PASS / FAIL / SKIP verdict with
// captured-evidence paths.
//
// MOTIVATION (the §11.4 hole this closes). A test that records a session
// to mp4 and then asserts PASS because "the recording file exists" is the
// canonical §11.4 / §11.4.2 / §11.4.5 PASS-bluff: a 0-byte mp4, a frozen
// first frame, or an error overlay in the captured frames all slip through
// a presence-only check. This package raises the bar to: the recorded
// session GENUINELY achieved its goal — the prompts received real replies
// AND no error/warning/panic surfaced during the interaction.
//
// CANONICAL USE CASE (the consumer the conductor wires). HelixCode records
// TUI LLM-chat sessions to mp4 (prompts typed, model replies rendered on
// the terminal). recordingqa ORCHESTRATES the validation of each such
// recording by composing TWO independent oracles, both consumer-injected:
//
//  1. A VIDEO validator (the "Panoptic" / recording-analyzer oracle) — does
//     frame extraction + OCR over the mp4 and reports whether the EXPECTED
//     replies are present on-screen and whether any ERROR TEXT is on-screen.
//     HelixQA already ships such an analyzer at cmd/recording-analyzer
//     (frame OCR + §11.4.107 liveness). recordingqa never hardcodes which
//     binary runs — the consumer injects a VideoValidator (e.g. a wrapper
//     around `recording-analyzer --post-analyze` or the conductor's
//     `panoptic-validate-recording <mp4>` hook).
//
//  2. A LOG oracle — greps the session's stderr log for error / warning /
//     panic / "no provider" / "0 models" / "unhealthy" patterns that mean
//     the LLM interaction was broken even if a frame happened to render.
//     The pattern set is consumer-supplied (DefaultErrorPatterns ships a
//     starting set the consumer may replace) so HelixQA stays project-
//     agnostic per CONST-051(B).
//
// DECOUPLING (CONST-051(B) / §11.4.28). This package carries NO HelixCode /
// ATMOSphere knowledge: no hardcoded binary path, no hardcoded log path, no
// hardcoded model id, no hardcoded prompt. Every concrete value arrives via
// the Spec the consumer constructs at runtime. It depends only on the
// stdlib + the in-repo conduit package for the §11.4.116 sync channel.
//
// ANTI-BLUFF (§11.4.50 / §11.4.69 / §11.4.98). A PASS is permitted ONLY
// when ALL hold: the mp4 exists and is non-empty; the injected
// VideoValidator returns OK with every expected reply matched and NO error
// text matched; the stderr log exists and contains NONE of the error
// patterns. A green verdict on a broken recording (0-byte mp4, missing
// reply, error overlay, panic in the log) is a §11.4 bluff and MUST come
// out FAIL — the package's tests include a deliberately-bad recording and
// a stubbed bad-analysis result that both force FAIL.
package recordingqa

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"digital.vasic.helixqa/pkg/conduit"
)

// VideoOptions carries the consumer-supplied tuning the video oracle (the
// sibling Panoptic `recvalidate` validator, the in-house recording-analyzer,
// or any injected analyzer) needs to validate a chat-TUI recording WITHOUT
// false positives. Every field is consumer DATA — recordingqa never
// hardcodes a UI string (CONST-051(B) / §11.4.28); the bank YAML / the Spec
// supplies the values and recordingqa forwards them unchanged.
//
// The two fields mirror the Panoptic recvalidate options the conductor
// wires:
//
//   - ChromeLinePatterns (Panoptic CLI `--chrome-pattern`, repeatable):
//     consumer-supplied case-insensitive regexes matching ambient UI chrome
//     lines (sidebar labels / status panels / command lists) that the video
//     oracle MUST EXCLUDE from reply-prose counting. Without them, a chat-TUI
//     recording whose sidebar reads "Commands  Models  Settings" can be
//     mis-counted as assistant reply prose — a §11.4.137-class chrome-as-reply
//     bluff. Empty ⇒ the analyzer applies no chrome exclusion (generic chat).
//
//   - ReplyMarkers (Panoptic CLI `--reply-marker`, repeatable): assistant-turn
//     prefixes (e.g. "AI:") that anchor where the real reply begins, so the
//     oracle counts prose AFTER the marker, not the user's own echoed prompt
//     or chrome above it. Empty ⇒ the analyzer falls back to its generic
//     chat defaults.
//
// recordingqa does NOT interpret either list — it forwards them verbatim to
// the injected VideoValidator, which maps them onto the Panoptic recvalidate
// `ChromeLinePatterns` / `ReplyMarkers` options (CLI flags or Go API).
type VideoOptions struct {
	// ExpectedReplies are the reply fragments each prompt should have
	// produced (or generic non-empty-reply markers) — the video oracle
	// must match every one for a PASS.
	ExpectedReplies []string

	// ChromeLinePatterns are consumer-supplied regexes (raw strings) for
	// ambient UI chrome lines to EXCLUDE from reply-prose counting.
	ChromeLinePatterns []string

	// ReplyMarkers are the assistant-turn prefixes that anchor the reply.
	ReplyMarkers []string
}

// VideoValidator is the consumer-injected oracle that analyzes the mp4 of
// a recorded session (frame extraction + OCR — the "Panoptic" /
// recording-analyzer role). HelixQA never constructs a concrete one; the
// consumer supplies an implementation that shells out to its analyzer (or
// to the conductor's `panoptic-validate-recording <mp4>` hook, OR the
// sibling Panoptic `recvalidate` validator) and maps the result into a
// VideoResult.
//
// Validate is given the absolute mp4 path and the VideoOptions the consumer
// declared (expected replies + chrome-line patterns + reply markers). It
// returns a VideoResult describing what the analyzer found. A non-nil error
// means the analyzer COULD NOT RUN (tool absent, mp4 unreadable) —
// recordingqa maps that to SKIP-with-reason per §11.4.3, never a fake PASS.
type VideoValidator interface {
	Validate(ctx context.Context, mp4Path string, opts VideoOptions) (VideoResult, error)
}

// VideoValidatorFunc adapts a plain function to VideoValidator.
type VideoValidatorFunc func(ctx context.Context, mp4Path string, opts VideoOptions) (VideoResult, error)

// Validate implements VideoValidator.
func (f VideoValidatorFunc) Validate(ctx context.Context, mp4Path string, opts VideoOptions) (VideoResult, error) {
	return f(ctx, mp4Path, opts)
}

// VideoResult is the structured outcome the injected analyzer reports for
// one recording. It is intentionally analyzer-agnostic: the consumer's
// VideoValidator maps its tool's native output into these fields.
type VideoResult struct {
	// MatchedReplies lists the expected-reply fragments the analyzer
	// actually found rendered on-screen (OCR hit). For a goal-achieved
	// recording this MUST cover every expected reply.
	MatchedReplies []string

	// MissingReplies lists the expected-reply fragments the analyzer did
	// NOT find on any frame. Non-empty here means the recording did not
	// achieve its goal — at least one prompt got no visible reply.
	MissingReplies []string

	// ErrorTextFound lists any error/obstruction strings the analyzer
	// detected ON-SCREEN (error overlay, "no provider", stack trace text
	// rendered in the terminal). Non-empty here is a hard FAIL — the user
	// saw an error, not a reply.
	ErrorTextFound []string

	// AnalyzedFrames is the artefact path the analyzer wrote (extracted
	// frames dir / findings JSONL). Captured as evidence per §11.4.5.
	// MUST be a real, non-empty path for a PASS.
	AnalyzedFrames string

	// Detail is a free-form human-readable summary from the analyzer.
	Detail string
}

// LogScanResult is the outcome of scanning the session's stderr log for
// error/warning/panic patterns.
type LogScanResult struct {
	// Matches lists "pattern => line" hits found in the log. Non-empty
	// here forces FAIL: the LLM interaction logged a real problem.
	Matches []string

	// LogPath echoes the scanned log path (captured as evidence).
	LogPath string
}

// Verdict re-exports the conduit closed PASS/FAIL/SKIP vocabulary so a
// caller need not import conduit directly.
type Verdict = conduit.Verdict

// Result is the full structured outcome of validating one recorded
// artefact: the final verdict, the reason, the two oracle results, and the
// captured-evidence paths.
type Result struct {
	// ChallengeID is the stable handle for this validation (so a conduit
	// verdict / tracker ticket can cross-reference it).
	ChallengeID string

	// Verdict is PASS / FAIL / SKIP after BOTH oracles.
	Verdict Verdict

	// Reason carries the SKIP / FAIL detail (closed-set-ish, greppable).
	Reason string

	// Video / Log are the two oracle results.
	Video VideoResult
	Log   LogScanResult

	// EvidencePaths lists every captured-evidence artefact this verdict
	// rests on (the mp4, the analyzed-frames dir, the stderr log). For a
	// PASS, every entry MUST be a real, non-empty path (§11.4.69).
	EvidencePaths []string
}

// Spec is everything the consumer supplies to validate ONE recorded
// session. Every field is consumer data — no HelixCode/ATMOSphere knowledge
// lives here (CONST-051(B)).
type Spec struct {
	// ChallengeID is the stable handle the consumer assigns.
	ChallengeID string

	// RecordingPath is the absolute path to the session mp4.
	RecordingPath string

	// StderrLogPath is the absolute path to the session's stderr log (the
	// TUI's stderr capture). Empty disables the log oracle ONLY if
	// RequireLog is false; when RequireLog is true an empty/missing log is
	// a FAIL (the consumer asserted a log MUST exist).
	StderrLogPath string

	// ExpectedReplies are the reply fragments each prompt should have
	// produced (or generic non-empty-reply markers). The video oracle must
	// match every one of these for a PASS. Empty means "no per-reply
	// assertion" — the consumer relies on the analyzer's own goal logic.
	ExpectedReplies []string

	// ChromeLinePatterns are consumer-supplied case-insensitive regexes
	// (raw strings) matching ambient chat-TUI chrome lines (sidebar labels
	// / status panels / command lists) that the video oracle MUST EXCLUDE
	// from reply-prose counting (Panoptic recvalidate `--chrome-pattern`,
	// repeatable). Forwarded verbatim to the injected VideoValidator via
	// VideoOptions; recordingqa never interprets them (CONST-051(B)). Empty
	// ⇒ no chrome exclusion (generic chat).
	ChromeLinePatterns []string

	// ReplyMarkers are the assistant-turn prefixes (e.g. "AI:") that anchor
	// where the real reply begins (Panoptic recvalidate `--reply-marker`,
	// repeatable). Forwarded verbatim to the injected VideoValidator via
	// VideoOptions. Empty ⇒ the analyzer applies its generic chat defaults.
	ReplyMarkers []string

	// ErrorPatterns are the regexes the log oracle treats as failure
	// evidence. When nil, DefaultErrorPatterns is used. The consumer may
	// replace it entirely.
	ErrorPatterns []*regexp.Regexp

	// RequireLog makes a missing/empty stderr log a FAIL (default: a
	// missing log is a FAIL too — see Validate; the field exists so a
	// consumer can opt the log oracle out by setting StderrLogPath="" AND
	// RequireLog=false, e.g. when the TUI logs to the same mp4-side file).
	RequireLog bool
}

// DefaultErrorPatterns is a STARTING set of error/warning/panic patterns a
// consumer may use as-is or replace. It is deliberately conservative and
// case-insensitive. The patterns match the closed set the task names:
// error, warning, panic, "no provider", "0 models", "unhealthy", plus the
// "simulated" anti-bluff marker (a simulated reply is a broken interaction).
func DefaultErrorPatterns() []*regexp.Regexp {
	raw := []string{
		`(?i)\bpanic\b`,
		`(?i)\bfatal\b`,
		`(?i)\berror\b`,
		`(?i)\bwarning\b`,
		`(?i)no provider`,
		`(?i)\b0 models\b`,
		`(?i)no models`,
		`(?i)unhealthy`,
		`(?i)connection refused`,
		`(?i)\bsimulated\b`, // a simulated reply is a broken LLM interaction
	}
	out := make([]*regexp.Regexp, 0, len(raw))
	for _, r := range raw {
		out = append(out, regexp.MustCompile(r))
	}
	return out
}

// Validator orchestrates one recorded-artefact validation. The video oracle
// is consumer-injected; the log oracle is built in (a regex scan). A nil
// Conduit is tolerated (no sync-channel events emitted).
type Validator struct {
	// Video is the consumer-injected analyzer. REQUIRED — a nil Video
	// makes Validate return SKIP (the recording could not be analyzed).
	Video VideoValidator

	// Conduit, when non-nil, receives challenge_start / evidence /
	// challenge_verdict events for the §11.4.116 live sync channel.
	Conduit conduit.Sink
}

// Validate runs both oracles for one Spec and returns the structured
// verdict. Decision order (each step can only LOWER the verdict, never
// raise a FAIL back to PASS):
//
//  1. mp4 presence: missing/0-byte → FAIL (the §11.4.5 0-byte-mp4 bluff).
//  2. video oracle: analyzer cannot run → SKIP-with-reason (§11.4.3);
//     analyzer ran but found error text on-screen OR a missing expected
//     reply OR no analyzed-frames artefact → FAIL.
//  3. log oracle: log missing (when required) → FAIL; any error pattern
//     matched → FAIL.
//  4. otherwise PASS, citing the mp4 + analyzed-frames + log as evidence.
func (v *Validator) Validate(ctx context.Context, spec Spec) Result {
	res := Result{ChallengeID: spec.ChallengeID}

	if v.Conduit != nil {
		conduit.ChallengeStart(v.Conduit, spec.ChallengeID, "recording")
	}

	// Step 1: mp4 presence (the 0-byte / missing bluff).
	if fi, err := os.Stat(spec.RecordingPath); err != nil {
		res.Verdict = conduit.VerdictFail
		res.Reason = "recording_missing: " + spec.RecordingPath
		return v.finish(res)
	} else if fi.Size() == 0 {
		res.Verdict = conduit.VerdictFail
		res.Reason = "recording_zero_bytes: " + spec.RecordingPath
		return v.finish(res)
	}
	res.EvidencePaths = append(res.EvidencePaths, spec.RecordingPath)

	// Step 2: video oracle.
	if v.Video == nil {
		res.Verdict = conduit.VerdictSkip
		res.Reason = "no_video_validator_injected"
		return v.finish(res)
	}
	// Forward the consumer's chrome-line patterns + reply markers to the
	// injected video oracle (which maps them onto the Panoptic recvalidate
	// ChromeLinePatterns / ReplyMarkers options). recordingqa passes them
	// through verbatim — it never interprets a UI string (CONST-051(B)).
	vr, err := v.Video.Validate(ctx, spec.RecordingPath, VideoOptions{
		ExpectedReplies:    spec.ExpectedReplies,
		ChromeLinePatterns: spec.ChromeLinePatterns,
		ReplyMarkers:       spec.ReplyMarkers,
	})
	res.Video = vr
	if err != nil {
		// Analyzer could not run — honest SKIP, never a fake PASS.
		res.Verdict = conduit.VerdictSkip
		res.Reason = "video_validator_unavailable: " + err.Error()
		return v.finish(res)
	}
	if len(vr.ErrorTextFound) > 0 {
		res.Verdict = conduit.VerdictFail
		res.Reason = "error_text_on_screen: " + strings.Join(vr.ErrorTextFound, ",")
		return v.finish(res)
	}
	if len(vr.MissingReplies) > 0 {
		res.Verdict = conduit.VerdictFail
		res.Reason = "missing_replies: " + strings.Join(vr.MissingReplies, ",")
		return v.finish(res)
	}
	// A PASS-eligible video result MUST cite a real analyzed-frames
	// artefact (§11.4.69 — evidence, not absence-of-error).
	if !fileNonEmpty(vr.AnalyzedFrames) {
		res.Verdict = conduit.VerdictFail
		res.Reason = "no_analyzed_frames_evidence"
		return v.finish(res)
	}
	res.EvidencePaths = append(res.EvidencePaths, vr.AnalyzedFrames)

	// Step 3: log oracle.
	logResult, logVerdict, logReason := scanLog(spec)
	res.Log = logResult
	if logVerdict == conduit.VerdictFail {
		res.Verdict = conduit.VerdictFail
		res.Reason = logReason
		if logResult.LogPath != "" {
			res.EvidencePaths = append(res.EvidencePaths, logResult.LogPath)
		}
		return v.finish(res)
	}
	if logResult.LogPath != "" {
		res.EvidencePaths = append(res.EvidencePaths, logResult.LogPath)
	}

	// Step 4: all gates cleared.
	res.Verdict = conduit.VerdictPass
	res.Reason = "goal_achieved: replies_matched + no_error_text + clean_log"
	return v.finish(res)
}

// scanLog runs the log oracle for one Spec. Returns the scan result plus a
// (verdict, reason) pair: FAIL when the log is required-but-missing or any
// error pattern matched, otherwise PASS (the caller folds it in).
func scanLog(spec Spec) (LogScanResult, conduit.Verdict, string) {
	var out LogScanResult
	out.LogPath = spec.StderrLogPath

	if spec.StderrLogPath == "" {
		if spec.RequireLog {
			return out, conduit.VerdictFail, "stderr_log_required_but_unset"
		}
		return out, conduit.VerdictPass, ""
	}
	data, err := os.ReadFile(spec.StderrLogPath)
	if err != nil {
		// A required log that cannot be read is a FAIL; an optional one
		// missing is tolerated.
		if spec.RequireLog {
			return out, conduit.VerdictFail, "stderr_log_unreadable: " + err.Error()
		}
		return out, conduit.VerdictPass, ""
	}

	patterns := spec.ErrorPatterns
	if patterns == nil {
		patterns = DefaultErrorPatterns()
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		for _, p := range patterns {
			if p.MatchString(trimmed) {
				out.Matches = append(out.Matches, fmt.Sprintf("%s => %s", p.String(), truncateLine(trimmed)))
			}
		}
	}
	sort.Strings(out.Matches)
	if len(out.Matches) > 0 {
		return out, conduit.VerdictFail, "log_error_patterns: " + strings.Join(out.Matches, " | ")
	}
	return out, conduit.VerdictPass, ""
}

// finish emits the conduit verdict (+ one evidence_captured per path) and
// returns the result. Nil-conduit-safe.
func (v *Validator) finish(res Result) Result {
	if v.Conduit == nil {
		return res
	}
	if res.Verdict == conduit.VerdictPass {
		for _, p := range res.EvidencePaths {
			conduit.EvidenceCaptured(v.Conduit, res.ChallengeID, "recording_evidence", p)
		}
	}
	conduit.ChallengeVerdict(v.Conduit, res.ChallengeID, res.Verdict, res.Reason)
	return res
}

// fileNonEmpty reports whether path names an existing, non-empty file OR a
// non-empty directory (the analyzer may write a frames dir). Empty path,
// missing target, or 0-byte file → false (not evidence).
func fileNonEmpty(path string) bool {
	if path == "" {
		return false
	}
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	if fi.IsDir() {
		entries, err := os.ReadDir(path)
		return err == nil && len(entries) > 0
	}
	return fi.Size() > 0
}

// truncateLine bounds a matched log line so the reason string stays sane.
func truncateLine(s string) string {
	const max = 160
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
