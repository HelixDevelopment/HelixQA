// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// session.go — the autonomous vision-nav driving loop.
//
// Session composes the three existing seams (Provider, an action
// executor, an Explorer) THROUGH INTERFACES so it stays decoupled and
// unit-testable. Per step it: captures a screenshot, builds an
// Observation, asks the Provider to Decide, dispatches the decided
// action through the executor, asks the Explorer to CaptureFinding,
// then checks whether a registered ScreenGoal is satisfied.
//
// §11.4.52 anti-bluff verdict. A run is PASS only when BOTH hold:
//   (a) the goal was reached — by EITHER an OCR-backed match (a captured
//       Evidence whose OCRSnapshot contains a registered ScreenGoal) OR a
//       provider-vision match (the vision Provider, having SEEN the
//       step's screenshot, set Decision.GoalReached). The provider-vision
//       source needs NO external OCR host, AND
//   (b) the screen demonstrably CHANGED between consecutive steps
//       (screenshot delta above a floor). A run whose every consecutive
//       screenshot pair is effectively identical is a zero-screen-delta
//       run and is auto-FAIL — the executor's actions did nothing
//       observable, so any goal "match" would be unearned.
//
// The ScreenActor interface is defined HERE (the consumer), not in
// pkg/navigator, so this package does not import pkg/navigator and no
// import cycle is possible. pkg/navigator's ADBExecutor satisfies it
// structurally (Screenshot(ctx)([]byte,error) + a Dispatch shim).

package visionnav

import (
	"context"
	"fmt"
)

// ScreenActor is the small executor contract the Session needs. Kept
// local + minimal (consumer-defined interface, Go idiom) so the Session
// never imports a concrete executor package. An adapter over
// pkg/navigator.ADBExecutor (Screenshot + a per-action dispatch) is one
// line in the caller; tests supply a fake.
type ScreenActor interface {
	// Screenshot returns the current screen as image bytes. Used both
	// to feed the Provider an Observation and to compute the screen
	// delta that backs the §11.4.52 zero-delta auto-FAIL.
	Screenshot(ctx context.Context) ([]byte, error)
	// Dispatch performs the action chosen by the Provider (the action
	// grammar is opaque to the Session — the executor interprets it).
	Dispatch(ctx context.Context, action string) error
}

// SessionConfig wires a Session's collaborators. All fields required.
type SessionConfig struct {
	// Provider decides the next action each step.
	Provider Provider
	// Actor performs screenshots + actions.
	Actor ScreenActor
	// Explorer turns a step into validated, persisted Evidence.
	Explorer Explorer
	// Target is the registered thing under exploration.
	Target Target
	// MaxSteps caps the loop (must be ≥ 1).
	MaxSteps int
}

// Validate checks the config is complete before a Session runs.
func (c *SessionConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("visionnav: nil SessionConfig")
	}
	if c.Provider == nil {
		return fmt.Errorf("visionnav: SessionConfig.Provider is nil")
	}
	if c.Actor == nil {
		return fmt.Errorf("visionnav: SessionConfig.Actor is nil")
	}
	if c.Explorer == nil {
		return fmt.Errorf("visionnav: SessionConfig.Explorer is nil")
	}
	if c.MaxSteps < 1 {
		return fmt.Errorf("visionnav: SessionConfig.MaxSteps %d invalid (want >= 1)", c.MaxSteps)
	}
	return c.Target.Validate()
}

// Session drives the autonomous exploration loop.
type Session struct {
	cfg SessionConfig
}

// NewSession returns a Session for the given config, validating it
// up-front so a malformed Session can't be constructed.
func NewSession(cfg SessionConfig) (*Session, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Session{cfg: cfg}, nil
}

// SessionResult is the outcome of Run.
type SessionResult struct {
	// Passed is the §11.4.52 verdict: goal reached AND screen changed.
	Passed bool
	// GoalReached is true if the goal was reached at any step — by either
	// an OCR-backed Evidence match OR a provider-vision Decision.GoalReached.
	GoalReached bool
	// ScreenChanged is true if any consecutive screenshot pair differed
	// above the delta floor. Zero across all steps is the auto-FAIL.
	ScreenChanged bool
	// Steps is how many loop iterations executed.
	Steps int
	// Reason is a human-readable closure cause.
	Reason string
	// Evidence is the per-step captured-evidence records (in order).
	Evidence []*Evidence
}

// Run executes the loop. It returns a non-nil *SessionResult describing
// the verdict even when the verdict is FAIL; err is non-nil only for
// infrastructure failures (screenshot/dispatch/provider/explorer errors)
// that abort the loop, not for a clean FAIL verdict.
func (s *Session) Run(ctx context.Context) (*SessionResult, error) {
	res := &SessionResult{}
	var prevShot []byte

	for step := 1; step <= s.cfg.MaxSteps; step++ {
		res.Steps = step

		shot, err := s.cfg.Actor.Screenshot(ctx)
		if err != nil {
			return res, fmt.Errorf("visionnav: step %d: screenshot: %w", step, err)
		}

		// Screen-delta accounting (§11.4.52 (b)). The first step has no
		// predecessor, so it cannot establish change by itself.
		if prevShot != nil && screensDiffer(prevShot, shot) {
			res.ScreenChanged = true
		}
		prevShot = shot

		// Feed the just-captured screen frame to the Provider so a
		// vision-driven Provider (e.g. LLMProvider) can actually SEE the
		// screen it is deciding about. Without this the Provider is blind
		// — the C6 wiring defect this field closes.
		obs := Observation{StepNumber: step, LastImageBytes: shot}
		if len(res.Evidence) > 0 {
			obs.LastEvidence = res.Evidence[len(res.Evidence)-1]
		}

		decision, err := s.cfg.Provider.Decide(ctx, obs)
		if err != nil {
			return res, fmt.Errorf("visionnav: step %d: provider decide: %w", step, err)
		}
		if err := decision.Validate(); err != nil {
			return res, fmt.Errorf("visionnav: step %d: %w", step, err)
		}

		// On step 1 the LaunchAction is what brings the target on-screen.
		// Subsequent steps dispatch what the Provider decided.
		action := decision.Action
		if step == 1 {
			action = s.cfg.Target.LaunchAction
		}
		if err := s.cfg.Actor.Dispatch(ctx, action); err != nil {
			return res, fmt.Errorf("visionnav: step %d: dispatch %q: %w", step, action, err)
		}

		ev, err := s.cfg.Explorer.CaptureFinding(ctx, FindingOptions{
			Description: fmt.Sprintf("step %d: %s", step, decision.Action),
			Verdict:     "needs-review",
			Notes:       decision.Rationale,
			// Carry the Provider's recorded reasoning into the finding so an
			// Explorer that has no OCR/audio host (the provider-vision path)
			// can still produce §11.4-valid captured Evidence.
			ProviderRationale: decision.Rationale,
		})
		if err != nil {
			return res, fmt.Errorf("visionnav: step %d: capture finding: %w", step, err)
		}
		res.Evidence = append(res.Evidence, ev)

		// Goal detection has TWO independent sources, either of which marks
		// the goal reached:
		//   (a) OCR-backed match — a captured Evidence.OCRSnapshot contains a
		//       registered ScreenGoal. Active only when an OCR host populated
		//       the snapshot. This is the §11.4.52 OCR path.
		//   (b) Provider-vision match — the vision-capable Provider, having
		//       SEEN this step's screenshot, set decision.GoalReached. This
		//       path needs NO external OCR host: the model's own decision on
		//       the screen it was fed is the stop signal.
		// Neither path can manufacture a PASS: (a) requires a real OCR match,
		// (b) requires the Provider to affirmatively confirm the goal.
		if (ev != nil && evidenceMatchesGoal(ev, s.cfg.Target.ScreenGoals)) ||
			decision.GoalReached {
			res.GoalReached = true
			break
		}
	}

	// §11.4.52 anti-bluff verdict: BOTH conditions required.
	res.Passed = res.GoalReached && res.ScreenChanged
	switch {
	case !res.GoalReached:
		res.Reason = fmt.Sprintf("FAIL: no ScreenGoal reached within %d steps", res.Steps)
	case !res.ScreenChanged:
		res.Reason = "FAIL: zero-screen-delta across all steps " +
			"(actions produced no observable change — goal match is unearned)"
	default:
		res.Reason = fmt.Sprintf("PASS: ScreenGoal reached at step %d with observed screen change", res.Steps)
	}
	return res, nil
}

// evidenceMatchesGoal reports whether the Evidence's OCR snapshot
// contains any of the registered ScreenGoals (case-sensitive substring).
func evidenceMatchesGoal(ev *Evidence, goals []string) bool {
	if ev == nil || ev.OCRSnapshot == "" {
		return false
	}
	for _, g := range goals {
		if g != "" && containsSubstring(ev.OCRSnapshot, g) {
			return true
		}
	}
	return false
}

// containsSubstring is a tiny strings.Contains to keep the import set
// minimal and intent obvious at the call site.
func containsSubstring(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// screensDiffer reports whether two screenshots represent a different
// screen. It reuses the sampled-byte concept from pkg/navigator's
// isUniformImage/absDiff (kept local to avoid a cross-package import):
// sample bytes at four spread offsets in each image and treat the
// screens as differing if any sampled pair differs beyond the noise
// threshold, OR if the byte lengths differ materially.
//
// This is deliberately conservative: a real screen transition changes
// pixel data and almost always the PNG byte length; a no-op tap leaves
// both effectively identical. The threshold tolerates compression jitter.
func screensDiffer(a, b []byte) bool {
	// Materially different encoded length ⇒ different screen.
	if absInt(len(a)-len(b)) > screenLenNoiseBytes {
		return true
	}
	if len(a) < screenSampleStart+4 || len(b) < screenSampleStart+4 {
		// Too small to sample reliably; fall back to exact compare.
		return !bytesEqual(a, b)
	}
	// Sample at four spread offsets bounded by the shorter image.
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	offsets := []int{
		screenSampleStart,
		screenSampleStart + (n-screenSampleStart)/4,
		screenSampleStart + (n-screenSampleStart)/2,
		screenSampleStart + 3*(n-screenSampleStart)/4,
	}
	for _, o := range offsets {
		if o >= n {
			continue
		}
		if absByte(a[o], b[o]) > screenSampleThreshold {
			return true
		}
	}
	return false
}

const (
	// screenSampleStart skips the PNG header (mirrors navigator's 33).
	screenSampleStart = 33
	// screenSampleThreshold tolerates compression jitter per sample byte.
	screenSampleThreshold = byte(10)
	// screenLenNoiseBytes: encoded-length deltas at/under this are noise.
	screenLenNoiseBytes = 64
)

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func absByte(a, b byte) byte {
	if a > b {
		return a - b
	}
	return b - a
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
