// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package panopticoracle is a concrete recordingqa.VideoValidator that drives
// the sibling Panoptic `recvalidate` video oracle (frame extraction + OCR over
// the mp4 via ffmpeg + tesseract) and maps its JSON report into a
// recordingqa.VideoResult.
//
// WHY THIS PACKAGE EXISTS (closing the "reuse HelixQA" loop). pkg/recordingqa
// defines the recording-validation CONTRACT via a consumer-injected
// VideoValidator interface, but ships NO concrete oracle — by design, so HelixQA
// stays project-agnostic (CONST-051(B) / §11.4.28). This adapter is the REAL
// oracle the contract was built for: it shells out to Panoptic recvalidate,
// forwards the recordingqa VideoOptions (expected replies + ChromeLinePatterns +
// ReplyMarkers) as recvalidate CLI flags verbatim, parses the structured JSON
// report Panoptic emits, and folds it back into the recordingqa.VideoResult the
// orchestrator's PASS/FAIL/SKIP logic consumes. With this, the TRV-ENSEMBLE-001
// bank entry's options genuinely validate a real recorded mp4 end-to-end:
// bank YAML → recordingqa.Spec → recordingqa.Validator → THIS adapter →
// Panoptic recvalidate → JSON verdict.
//
// DECOUPLING (CONST-051(B) / §11.4.28). This package carries NO hardcoded
// absolute path to Panoptic and NO HelixCode/ATMOSphere knowledge. The Panoptic
// invocation is fully INJECTED via Config.Command + Config.Dir — the consumer
// (or an integration helper) supplies how to reach Panoptic (a prebuilt binary
// on PATH, `go run .` in the sibling checkout, a container entrypoint, …). The
// adapter only knows the recvalidate FLAG GRAMMAR + JSON SHAPE, which are
// Panoptic's stable public contract, not a consumer detail.
//
// ANTI-BLUFF (§11.4.3 / §11.4.69 / §11.4.123). When the Panoptic command cannot
// run (binary absent, ffmpeg/tesseract missing, the recvalidate run itself
// erroring before producing a report), Validate returns a non-nil error so the
// recordingqa orchestrator maps it to an honest SKIP-with-reason per §11.4.3 —
// NEVER a fake PASS. A report that ran but FAILed (a missing reply, an error
// token on screen, the model not visible) is surfaced as a VideoResult with a
// non-empty MissingReplies / ErrorTextFound so the orchestrator FAILs — the
// failure is REAL evidence, not a tool gap. The frames-dir Panoptic wrote (or,
// when absent, the JSON report itself) is cited as the AnalyzedFrames evidence
// path the orchestrator requires for a PASS (§11.4.69).
package panopticoracle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"digital.vasic.helixqa/pkg/recordingqa"
)

// Config injects everything the adapter needs to reach + drive the Panoptic
// recvalidate oracle. Every field is consumer DATA — no value is hardcoded into
// shipped logic (CONST-051(B)).
type Config struct {
	// Command is the argv that invokes Panoptic. The adapter APPENDS the
	// `recvalidate` subcommand + its flags. Examples:
	//   - prebuilt binary on PATH:        []string{"panoptic"}
	//   - prebuilt binary, explicit path: []string{"/path/to/panoptic"}
	//   - go run in the sibling checkout: []string{"go", "run", "."} (+ Dir)
	// REQUIRED — an empty Command makes Validate return an error (→ SKIP).
	Command []string

	// Dir is the working directory for Command (e.g. the Panoptic checkout
	// root when Command is `go run .`). Empty ⇒ the current process dir.
	Dir string

	// Model is the intended model/ensemble name that MUST appear on screen
	// (recvalidate `--model`). Empty ⇒ no model-visibility assertion.
	Model string

	// FramesDir, when non-empty, is passed as recvalidate `--frames-dir` +
	// `--keep-frames` so the extracted frames are retained as the §11.4.69
	// captured-evidence artefact. Empty ⇒ recvalidate uses a temp dir (the
	// adapter then cites the JSON report path as evidence instead).
	FramesDir string

	// ExtraErrorTokens are additional case-insensitive error phrases to flag
	// (recvalidate `--error-token`, repeatable). Optional.
	ExtraErrorTokens []string

	// Env, when non-nil, REPLACES the child process environment. Nil ⇒
	// inherit the parent environment (the common case so PATH resolves
	// ffmpeg/tesseract/go).
	Env []string

	// JSONOutPath, when non-empty, is passed as recvalidate `--json-out` so
	// the report is written to a known file (also captured as evidence).
	// Empty ⇒ the adapter reads the report from the command's stdout.
	JSONOutPath string

	// PreprocessVF, when non-empty, is an ffmpeg `-vf` filter chain the adapter
	// applies to a TEMPORARY copy of the mp4 BEFORE handing it to Panoptic, so
	// the OCR engine sees frames it can read. The canonical use is `negate` for
	// dark-background terminal/TUI recordings: tesseract expects dark text on a
	// light background, so a light-on-dark TUI frame OCRs to EMPTY text and every
	// reply/model assertion then fails on a recording whose content is plainly
	// visible to a human (FACT: confirmed on the ensemble recording — the dark
	// frame yields 0 OCR chars, `negate` yields the full transcript incl. the
	// real `AI:` replies). This is INPUT adaptation owned by the consumer, NOT a
	// verdict change: the PASS/FAIL still comes entirely from Panoptic's analysis
	// of the (now-readable) frames. Empty ⇒ the original mp4 is passed unchanged.
	//
	// PreprocessTool is the ffmpeg-class binary used for the transform; empty ⇒
	// "ffmpeg" on PATH. The transcoded copy is written under a temp dir and
	// removed when Validate returns. If the preprocess step cannot run, Validate
	// returns an error so the orchestrator SKIPs honestly (§11.4.3).
	PreprocessVF   string
	PreprocessTool string

	// ErrorScopeReplies, when true, passes recvalidate `--error-scope-replies`
	// so the built-in error/warning scan is restricted to the assistant-reply
	// regions and incidental terminal scrollback (pre-session startup warnings,
	// redis/log noise) cannot false-FAIL a STRUCTURAL-presence bank. Consumer
	// DATA (CONST-051(B)) — default false leaves the whole-frame scan unchanged;
	// genuine errors inside a reply still FAIL under either mode.
	ErrorScopeReplies bool
}

// checkResult mirrors Panoptic recvalidate's per-check JSON shape.
type checkResult struct {
	Name     string `json:"name"`
	Pass     bool   `json:"pass"`
	Detail   string `json:"detail"`
	Evidence string `json:"evidence,omitempty"`
}

// report mirrors the Panoptic recvalidate Report JSON shape (the fields this
// adapter consumes). Panoptic owns the canonical struct; this is the stable
// wire contract the CLI emits.
type report struct {
	Pass           bool          `json:"pass"`
	Skipped        bool          `json:"skipped"`
	SkipReason     string        `json:"skip_reason,omitempty"`
	VideoPath      string        `json:"video_path"`
	FrameCount     int           `json:"frame_count"`
	FramesDir      string        `json:"frames_dir,omitempty"`
	AggregatedText string        `json:"aggregated_text"`
	Checks         []checkResult `json:"checks"`
}

// New builds a recordingqa.VideoValidator backed by the Panoptic recvalidate
// CLI per cfg.
func New(cfg Config) recordingqa.VideoValidator {
	return &validator{cfg: cfg}
}

type validator struct {
	cfg Config
}

// Validate implements recordingqa.VideoValidator. It runs Panoptic recvalidate
// over mp4Path with the forwarded opts, parses the JSON report, and maps it
// into a recordingqa.VideoResult.
//
// Error vs FAIL discipline (§11.4.3): a non-nil error means the oracle COULD
// NOT RUN (→ orchestrator SKIP). A VideoResult with MissingReplies /
// ErrorTextFound means the oracle RAN and the recording FAILed its goal
// (→ orchestrator FAIL). recvalidate's own `skipped` (tools absent) is mapped
// to an error so the orchestrator SKIPs honestly rather than fake-PASS.
func (v *validator) Validate(ctx context.Context, mp4Path string, opts recordingqa.VideoOptions) (recordingqa.VideoResult, error) {
	if len(v.cfg.Command) == 0 {
		return recordingqa.VideoResult{}, fmt.Errorf("panopticoracle: empty Config.Command (no way to invoke Panoptic)")
	}

	// Optional input adaptation: transcode the mp4 through an ffmpeg -vf filter
	// (e.g. `negate` for dark-background TUI recordings) so the OCR engine sees
	// readable frames. The transform never changes the verdict — Panoptic still
	// does all the analysis; it only makes light-on-dark text OCR-able.
	videoForPanoptic := mp4Path
	if v.cfg.PreprocessVF != "" {
		pre, cleanup, err := v.preprocess(ctx, mp4Path)
		if err != nil {
			return recordingqa.VideoResult{}, fmt.Errorf("panopticoracle: preprocess (%s) failed: %w", v.cfg.PreprocessVF, err)
		}
		defer cleanup()
		videoForPanoptic = pre
	}

	args := v.buildArgs(videoForPanoptic, opts)

	// #nosec G204 — Command is consumer-injected config, not user input; the
	// remaining args are flag names + the consumer's own option values.
	cmd := exec.CommandContext(ctx, v.cfg.Command[0], append(append([]string{}, v.cfg.Command[1:]...), args...)...)
	cmd.Dir = v.cfg.Dir
	if v.cfg.Env != nil {
		cmd.Env = v.cfg.Env
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	// recvalidate exits non-zero on a genuine FAIL but STILL writes a report
	// (to --json-out, or stdout). A FAIL is NOT a run error — it is real
	// evidence. So we always try to obtain a report first; only when no
	// report is obtainable do we treat runErr as a tool-could-not-run error.
	rep, repErr := v.loadReport(&stdout)
	if repErr != nil {
		// No parseable report — the oracle could not produce a verdict.
		// Surface the run error + stderr so the SKIP reason is forensic.
		detail := strings.TrimSpace(stderr.String())
		if runErr != nil {
			return recordingqa.VideoResult{}, fmt.Errorf("panopticoracle: recvalidate did not produce a report: %w (stderr: %s)", runErr, truncate(detail, 400))
		}
		return recordingqa.VideoResult{}, fmt.Errorf("panopticoracle: %w (stderr: %s)", repErr, truncate(detail, 400))
	}

	// recvalidate self-reported SKIP (ffmpeg/tesseract absent, etc.): honest
	// SKIP per §11.4.3, never a fake PASS — surface as an error.
	if rep.Skipped {
		reason := rep.SkipReason
		if reason == "" {
			reason = "panoptic recvalidate skipped (tools unavailable)"
		}
		return recordingqa.VideoResult{}, fmt.Errorf("panopticoracle: %s", reason)
	}

	return v.mapReport(rep, opts, mp4Path), nil
}

// preprocess transcodes mp4Path through the configured ffmpeg -vf filter into a
// temp .mp4 and returns its path + a cleanup func. The transform is OCR-input
// adaptation only (e.g. `negate`), never a verdict change.
func (v *validator) preprocess(ctx context.Context, mp4Path string) (string, func(), error) {
	tool := v.cfg.PreprocessTool
	if tool == "" {
		tool = "ffmpeg"
	}
	tmpDir, err := os.MkdirTemp("", "panopticoracle-pre-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	out := filepath.Join(tmpDir, "preprocessed.mp4")

	// #nosec G204 — tool + filter are consumer-injected config, not user input.
	cmd := exec.CommandContext(ctx, tool,
		"-hide_banner", "-loglevel", "error",
		"-i", mp4Path,
		"-vf", v.cfg.PreprocessVF,
		"-c:v", "libx264", "-preset", "ultrafast",
		out, "-y",
	)
	if v.cfg.Env != nil {
		cmd.Env = v.cfg.Env
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("%w (stderr: %s)", err, truncate(strings.TrimSpace(stderr.String()), 300))
	}
	if fi, err := os.Stat(out); err != nil || fi.Size() == 0 {
		cleanup()
		return "", func() {}, fmt.Errorf("preprocess produced no output at %s", out)
	}
	return out, cleanup, nil
}

// buildArgs assembles the recvalidate flag list from cfg + opts. The opts lists
// are forwarded VERBATIM as repeatable flags — the adapter never interprets a
// UI string (CONST-051(B)).
func (v *validator) buildArgs(mp4Path string, opts recordingqa.VideoOptions) []string {
	args := []string{"recvalidate", "--video", mp4Path}

	if v.cfg.Model != "" {
		args = append(args, "--model", v.cfg.Model)
	}
	if v.cfg.FramesDir != "" {
		args = append(args, "--frames-dir", v.cfg.FramesDir, "--keep-frames")
	}
	if v.cfg.JSONOutPath != "" {
		args = append(args, "--json-out", v.cfg.JSONOutPath)
	}
	if v.cfg.ErrorScopeReplies {
		args = append(args, "--error-scope-replies")
	}

	// Expected replies → repeatable --prompt (recvalidate treats each as an
	// expected post-reply-marker prose fragment).
	for _, r := range opts.ExpectedReplies {
		args = append(args, "--prompt", r)
	}
	// Reply markers → repeatable --reply-marker.
	for _, m := range opts.ReplyMarkers {
		args = append(args, "--reply-marker", m)
	}
	// Chrome line patterns → repeatable --chrome-pattern.
	for _, p := range opts.ChromeLinePatterns {
		args = append(args, "--chrome-pattern", p)
	}
	// Extra error tokens → repeatable --error-token.
	for _, t := range v.cfg.ExtraErrorTokens {
		args = append(args, "--error-token", t)
	}

	return args
}

// loadReport returns the parsed recvalidate report. It prefers the
// --json-out file when configured, falling back to the command's stdout.
func (v *validator) loadReport(stdout *bytes.Buffer) (*report, error) {
	var raw []byte
	if v.cfg.JSONOutPath != "" {
		data, err := os.ReadFile(v.cfg.JSONOutPath)
		if err != nil {
			return nil, fmt.Errorf("read json-out %s: %w", v.cfg.JSONOutPath, err)
		}
		raw = data
	} else {
		raw = bytes.TrimSpace(stdout.Bytes())
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("empty recvalidate report")
	}
	var rep report
	if err := json.Unmarshal(raw, &rep); err != nil {
		return nil, fmt.Errorf("parse recvalidate report json: %w", err)
	}
	return &rep, nil
}

// mapReport folds a recvalidate report into a recordingqa.VideoResult.
//
//   - Every expected reply Panoptic confirmed (the overall report PASS implies
//     each --prompt was matched as post-marker prose) → MatchedReplies.
//   - On overall FAIL, the unmatched expected replies → MissingReplies, and any
//     failing error-token check detail → ErrorTextFound, so the orchestrator
//     FAILs with a forensic reason.
//   - The frames dir (kept) or the JSON report path is the AnalyzedFrames
//     evidence path the orchestrator requires for a PASS (§11.4.69).
func (v *validator) mapReport(rep *report, opts recordingqa.VideoOptions, originalVideo string) recordingqa.VideoResult {
	res := recordingqa.VideoResult{
		AnalyzedFrames: v.evidencePath(rep, originalVideo),
		Detail:         summarize(rep),
	}

	// Collect per-check signals.
	var errTexts []string
	replyCheckFailed := false
	modelCheckFailed := false
	for _, c := range rep.Checks {
		if c.Pass {
			continue
		}
		name := strings.ToLower(c.Name)
		switch {
		case strings.Contains(name, "error"):
			// no_error_tokens FAILed → an error token was on screen.
			ev := c.Evidence
			if ev == "" {
				ev = c.Detail
			}
			errTexts = append(errTexts, strings.TrimSpace(ev))
		case strings.Contains(name, "model"):
			modelCheckFailed = true
		case strings.Contains(name, "repl") || strings.Contains(name, "prompt"):
			replyCheckFailed = true
		default:
			// Any other failing check is surfaced as a generic failure note
			// so the orchestrator's FAIL reason is forensic.
			replyCheckFailed = true
		}
	}

	if rep.Pass {
		// Overall PASS: every expected reply was matched as post-marker prose,
		// no error tokens, model visible. Mark every expected reply matched.
		res.MatchedReplies = append([]string{}, opts.ExpectedReplies...)
		res.ErrorTextFound = nil
		return res
	}

	// Overall FAIL. Surface the failure as REAL evidence (not a tool gap).
	res.ErrorTextFound = errTexts
	if replyCheckFailed || modelCheckFailed || len(opts.ExpectedReplies) > 0 {
		// At least one assertion failed; treat the expected replies as
		// unverified so the orchestrator FAILs with missing_replies. When the
		// failure was purely the model-visibility check, still report the
		// replies as missing so the FAIL is not silently swallowed.
		res.MissingReplies = append([]string{}, opts.ExpectedReplies...)
		if len(res.MissingReplies) == 0 {
			// No per-reply assertion was configured but the report still
			// FAILed — encode the failure as an error text so the
			// orchestrator does not PASS.
			res.ErrorTextFound = append(res.ErrorTextFound, "recvalidate report FAIL: "+summarize(rep))
		}
	}
	return res
}

// evidencePath returns the captured-evidence path the orchestrator cites for a
// PASS: the kept frames dir if recvalidate retained one, else the configured
// JSON report file, else (last resort) the video path itself so the evidence
// field is never empty when a real report was produced.
func (v *validator) evidencePath(rep *report, originalVideo string) string {
	if rep.FramesDir != "" {
		if fi, err := os.Stat(rep.FramesDir); err == nil && fi.IsDir() {
			return rep.FramesDir
		}
	}
	if v.cfg.FramesDir != "" {
		if fi, err := os.Stat(v.cfg.FramesDir); err == nil && fi.IsDir() {
			return v.cfg.FramesDir
		}
	}
	if v.cfg.JSONOutPath != "" {
		if fi, err := os.Stat(v.cfg.JSONOutPath); err == nil && fi.Size() > 0 {
			return v.cfg.JSONOutPath
		}
	}
	// Last resort: the ORIGINAL mp4 (rep.VideoPath may be the cleaned-up
	// preprocess temp copy, which would not exist when cited as evidence).
	return originalVideo
}

func summarize(rep *report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "panoptic recvalidate: pass=%t frames=%d", rep.Pass, rep.FrameCount)
	for _, c := range rep.Checks {
		fmt.Fprintf(&b, "; %s=%t", c.Name, c.Pass)
	}
	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
