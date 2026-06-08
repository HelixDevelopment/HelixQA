// Package orchestrator — definition_challenge.go.
//
// definitionChallenge bridges bank-loaded *challenge.Definition records
// into the runnable challenge.Challenge interface so the orchestrator
// can dispatch them through challenges/pkg/runner. Before this
// adapter existed, helixqa loaded YAML definitions into the bank
// but never registered them with registry.Default — so the
// orchestrator's per-definition dispatch loop in runPlatform always
// short-circuited (o.runner == nil OR registry.Get returned not-found),
// producing the "PASS-bluff detected: 0 challenges actually executed"
// honest-fail in cmd/helixqa/main.go.
//
// HXC-011 (close-out — desktop-platform real execution):
// definitionChallenge.Execute USED to unconditionally return
// Status=Skipped — it never shelled out to a bank case's `action:`
// command on any platform. For the `desktop` platform that is a
// §11.4 / CONST-035 PASS-bluff IN THE QA RUNNER ITSELF: a green
// (or honest-looking SKIP) line with no runtime evidence. The runner
// loaded the cases but never ran them.
//
// CONST-035 / §11.4.69 posture after HXC-011:
//   - A definitionChallenge that carries one or more executable
//     desktop-platform steps (`action: "shell: <cmd>"`) RUNS them
//     via os/exec, captures the real exit code + combined output,
//     and scores PASS only when every step exits 0. A non-zero exit
//     scores FAIL — the runner can no longer bluff a PASS.
//   - A definitionChallenge with NO step the desktop platform can
//     execute (prose-only steps, or steps that need an Android / UI
//     topology) returns an honest Status=Skipped with an explicit
//     reason — never a PASS. SKIP is for environment limitations and
//     MUST always carry an explicit reason (§11.4.3).
//
// CONST-051(B): the challenges submodule stays decoupled — this fix
// lives entirely in helix_qa. The wrapper carries the executable
// TestCase (loaded by pkg/testbank, which HelixQA owns) so the
// generic challenges/pkg/bank loader needs no project-specific
// `steps` field.
package orchestrator

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/registry"

	"digital.vasic.helixqa/pkg/config"
	"digital.vasic.helixqa/pkg/testbank"
	"digital.vasic.helixqa/pkg/validator"
	"digital.vasic.helixqa/pkg/visionnav"
)

// AndroidVisionContext carries the collaborators a definitionChallenge
// needs to DRIVE a real Android device through the existing vision-nav
// loop (pkg/visionnav.Session) for a bank case on the `android`
// platform. It is OPTIONAL: when nil (no device serial OR no vision
// Provider was configured for the run), the wrapper falls back to the
// honest Skipped path exactly as before — never a fake PASS.
//
// Decoupling (CONST-051(B) / §11.4.27): the orchestrator MAY import
// both pkg/visionnav and pkg/navigator and build the actor adapter,
// because the orchestrator is HelixQA-internal glue. pkg/visionnav
// itself never imports pkg/navigator — the ScreenActor passed in here
// is built by the caller (cmd/helixqa or a test fake).
type AndroidVisionContext struct {
	// Provider decides the next action each step (e.g. a
	// pkg/llm.BridgedCLIProvider wrapped via visionnav.NewLLMProvider,
	// or a test fake). Required for the Android branch to run.
	Provider visionnav.Provider

	// Actor performs screenshots + dispatches actions against the real
	// device (e.g. visionnav.NewADBActor over a pkg/navigator.ADBExecutor,
	// or a test fake). Required.
	Actor visionnav.ScreenActor

	// Explorer turns each step into validated, persisted Evidence whose
	// OCRSnapshot the Session matches against the bank case's success
	// criterion. Required — without it no Evidence is captured and the
	// run could never honestly reach PASS.
	Explorer visionnav.Explorer

	// MaxSteps caps the vision loop per bank case. Must be ≥ 1; the
	// caller supplies a sane default (the orchestrator uses a small
	// constant) when unset.
	MaxSteps int

	// Serial is the ADB device serial the Actor targets. Recorded into
	// RecordedActions for forensic traceability; not used to drive the
	// loop (the Actor already holds it).
	Serial string
}

// valid reports whether the context carries everything the Android
// branch needs. A partially-wired context (e.g. a Provider but no
// Actor) means the run was misconfigured — the wrapper treats that as
// "not wired" and honestly skips rather than half-running.
func (a *AndroidVisionContext) valid() bool {
	return a != nil &&
		a.Provider != nil &&
		a.Actor != nil &&
		a.Explorer != nil &&
		a.MaxSteps >= 1
}

// definitionWrapperSkipSentinel is the prefix written into
// RecordedActions by definitionChallenge.Execute so the orchestrator
// can detect, after the runner has merged the result, that the
// original wrapper intended a Skipped status. See
// restoreSkippedFromDefinitionWrapper for the round-82 anti-bluff
// rationale.
const definitionWrapperSkipSentinel = "skip-reason:"

// restoreSkippedFromDefinitionWrapper restores Status=Skipped on a
// challenge.Result produced by the definitionChallenge wrapper after
// the challenges-runner's executeChallenge merge logic has overridden
// the wrapper's intent. See round-82 commit body for the full
// causal chain — the short version: the runner only preserves
// Failed/TimedOut/Error from the inner Execute call; Skipped with a
// passing assertion gets silently promoted to Passed, producing the
// canonical CONST-035 / Article XI §11.9 PASS-bluff (declarative-only
// definitions with no real backend dispatch report success to the
// end user).
//
// The signal that triggers restoration is the
// definitionWrapperSkipSentinel string at the start of any
// RecordedAction entry — only the wrapper writes that exact prefix.
// Non-wrapper results pass through untouched.
//
// Anti-bluff posture: this restores the HONEST skip, it does not
// hide a real PASS. The wrapper itself is what produced the
// sentinel; a definitionChallenge that REALLY executed desktop
// shell actions does NOT emit the sentinel, so the runner's Passed
// (or Failed) status stands.
func restoreSkippedFromDefinitionWrapper(r *challenge.Result) {
	if r == nil {
		return
	}
	if r.Status != challenge.StatusPassed {
		return
	}
	for _, action := range r.RecordedActions {
		if strings.HasPrefix(action, definitionWrapperSkipSentinel) {
			r.Status = challenge.StatusSkipped
			return
		}
	}
}

// testCasePlatformMatches reports whether tc targets the given run
// platform. A nil case, an empty Platforms list, or a list containing
// config.PlatformAll matches every platform. This is the standalone
// twin of (*definitionChallenge).platformMatches for the orchestrator
// aggregation call site, where the wrapper itself is no longer in
// scope (only the TestCase + platform are).
func testCasePlatformMatches(tc *testbank.TestCase, platform config.Platform) bool {
	if tc == nil || len(tc.Platforms) == 0 {
		return true
	}
	for _, p := range tc.Platforms {
		if p == config.PlatformAll || p == platform {
			return true
		}
	}
	return false
}

// promoteSkippedToPassed upgrades a SKIPPED challenge result to PASSED
// when (and only when) all of the following hold:
//
//   - the challenge result is non-nil and currently StatusSkipped;
//   - the wrapper skipped because it had no directly-executable
//     `shell:` step (the definitionWrapperSkipSentinel is present) —
//     i.e. the steps ran through the step-validation / LLM-bridge path
//     instead of the wrapper's os/exec path;
//   - the bank case targets the platform being run
//     (testCasePlatformMatches) — a case pinned to a non-matching
//     platform stays SKIPPED and is never promoted;
//   - the corresponding step-validation result PASSED.
//
// This closes the §107 reporting bluff where every challenge reported
// SKIPPED (and the summary read "0/N passed") even though every step
// in the "### Step Validation" table PASSED. A challenge whose steps
// all passed on a matching platform IS a pass, and the aggregation
// must reflect that.
//
// Anti-bluff posture: promotion requires a genuinely-passing step
// result. A SKIPPED challenge with no step result, a FAILED/ERROR step
// result, or a platform mismatch is left SKIPPED untouched — the
// function never invents a pass. As a hard guard, the desktop platform
// is NEVER promoted: desktop has no persistent app, so the step
// validator only proves crash-absence, which is not evidence a CLI
// command ran correctly — desktop cases must earn PASSED via real
// `shell:` execution. Promotion therefore serves only UI/app platforms
// (android/androidtv/web) where post-action crash-absence is the
// intended validation signal.
func promoteSkippedToPassed(
	cr *challenge.Result,
	sr *validator.StepResult,
	tc *testbank.TestCase,
	platform config.Platform,
) {
	if cr == nil || cr.Status != challenge.StatusSkipped {
		return
	}
	// Only promote wrapper-emitted honest skips, never some other
	// component's deliberate skip.
	if !hasSkipSentinel(cr) {
		return
	}
	// §107 anti-bluff hard guard: the desktop platform has NO persistent
	// application, so the step validator's signal is pure crash-absence
	// (det.CheckApp finds nothing running → StepPassed). Promoting a desktop
	// SKIP to PASSED on that vacuous signal would manufacture a PASS that no
	// command ever earned — exactly the metadata-only/absence-of-error
	// PASS-bluff the covenant forbids. Desktop cases MUST earn PASSED through
	// real `shell:` execution (executeDesktopShellSteps captures the real exit
	// code + output); a desktop case with only prose steps stays honestly
	// SKIPPED with the "convert prose steps to `action: \"shell: <cmd>\"`"
	// reason. Never promote on desktop.
	if platform == config.PlatformDesktop {
		return
	}
	if !testCasePlatformMatches(tc, platform) {
		return
	}
	// §107 anti-bluff wider guard (parity-audit 2026-05-30): the bank-runner
	// path executes ONLY `shell:` steps (executableShellSteps filters to
	// ActionTypeShell). The step validator that gates this promotion only
	// proves crash-absence — it NEVER runs a case's `http:` / `assert:` /
	// `tap:` / `adb_shell:` / … asserting steps. So promoting a SKIP whose
	// case carries any such asserting step would manufacture a PASS for
	// assertions that never executed — the same bluff the desktop guard
	// closes, but on every UI platform. Only cases whose steps are PURELY
	// observational (description / screenshot / sleep — where post-action
	// crash-absence IS the intended signal) may be promoted; a case with a
	// real asserting step stays honestly SKIPPED.
	if caseHasUnrunAssertingStep(tc) {
		return
	}
	if sr == nil || sr.Status != validator.StepPassed {
		return
	}
	cr.Status = challenge.StatusPassed
}

// caseHasUnrunAssertingStep reports whether the test case carries any step
// whose action genuinely asserts something the bank-runner path does NOT
// execute (everything except the observational set description/screenshot/
// sleep, and `shell:` which the bank runner DOES run — so a sentinel-SKIPPED
// case never has shell steps anyway). If true, crash-absence is not evidence
// the case passed and it must not be promoted. A nil/empty case is treated as
// having no asserting step (nothing to run → safe to promote on the prior
// guards).
func caseHasUnrunAssertingStep(tc *testbank.TestCase) bool {
	if tc == nil {
		return false
	}
	for i := range tc.Steps {
		step := tc.Steps[i]
		at, _ := step.ParseAction()
		switch at {
		case testbank.ActionTypeDescription,
			testbank.ActionTypeScreenshot,
			testbank.ActionTypeSleep,
			testbank.ActionTypeShell:
			// observational, or shell (run by the bank runner) — not a bluff
			continue
		default:
			// http / assert / tap / swipe / keypress / text / adb_shell /
			// playback_check / frame_diff / playwright — a real asserting
			// action the bank runner never executed.
			return true
		}
	}
	return false
}

// hasSkipSentinel reports whether the result carries the
// definitionChallenge wrapper's skip sentinel in its RecordedActions.
func hasSkipSentinel(r *challenge.Result) bool {
	if r == nil {
		return false
	}
	for _, action := range r.RecordedActions {
		if strings.HasPrefix(action, definitionWrapperSkipSentinel) {
			return true
		}
	}
	return false
}

// newDefinitionRegistry returns a fresh, non-shared registry that the
// orchestrator can populate with definitionChallenge wrappers. Using a
// dedicated per-run registry (rather than registry.Default) keeps
// concurrent helixqa invocations from polluting each other's
// challenge namespaces.
func newDefinitionRegistry() registry.Registry {
	return registry.NewRegistry()
}

// definitionChallenge wraps a *challenge.Definition so it satisfies
// the challenge.Challenge interface.
//
// When the orchestrator has the executable bank case for this
// definition (testCase != nil) AND a target platform that can run
// its steps, Execute genuinely runs them. Otherwise Execute returns
// an honest Skipped result with an explicit reason.
type definitionChallenge struct {
	def *challenge.Definition
	cfg *challenge.Config

	// testCase is the executable bank case backing this definition,
	// when the orchestrator could load it via pkg/testbank. nil for
	// definitions loaded only through the generic challenges/pkg/bank
	// loader (which drops the `steps` array).
	testCase *testbank.TestCase

	// platform is the target platform this wrapper executes against.
	// Set by the orchestrator per runPlatform iteration.
	platform config.Platform

	// androidCtx, when non-nil and valid(), lets Execute DRIVE a real
	// Android device through the pkg/visionnav loop for this bank case.
	// nil for desktop/web runs and for android runs where no device +
	// vision Provider was configured (Execute then honestly skips).
	androidCtx *AndroidVisionContext
}

// newDefinitionChallenge constructs a wrapper with no executable
// backing — Execute will honestly skip.
func newDefinitionChallenge(def *challenge.Definition) *definitionChallenge {
	return &definitionChallenge{def: def}
}

// newDefinitionChallengeForPlatform constructs a wrapper that carries
// the executable bank case and the target platform, so Execute can
// genuinely run the case's actions when the platform supports them.
func newDefinitionChallengeForPlatform(
	def *challenge.Definition,
	tc *testbank.TestCase,
	platform config.Platform,
) *definitionChallenge {
	return &definitionChallenge{
		def:      def,
		testCase: tc,
		platform: platform,
	}
}

// newDefinitionChallengeForAndroid constructs an android-platform
// wrapper carrying the executable bank case AND the vision-nav context
// so Execute drives the real device. A nil/invalid androidCtx is
// preserved verbatim — Execute then falls through to the honest skip.
func newDefinitionChallengeForAndroid(
	def *challenge.Definition,
	tc *testbank.TestCase,
	androidCtx *AndroidVisionContext,
) *definitionChallenge {
	return &definitionChallenge{
		def:        def,
		testCase:   tc,
		platform:   config.PlatformAndroid,
		androidCtx: androidCtx,
	}
}

// platformMatches reports whether this wrapper's bank case targets
// the platform the wrapper is running against. A case with no
// declared platforms (or with config.PlatformAll) matches every
// platform; otherwise the run platform must appear in the case's
// Platforms list.
//
// When no executable bank case is available (testCase == nil) the
// definition is declarative-only and not pinned to any platform, so
// it is treated as matching — the skip in that situation is "no
// backend dispatcher", not "wrong platform".
func (d *definitionChallenge) platformMatches() bool {
	if d.testCase == nil {
		return true
	}
	if len(d.testCase.Platforms) == 0 {
		return true
	}
	for _, p := range d.testCase.Platforms {
		if p == config.PlatformAll || p == d.platform {
			return true
		}
	}
	return false
}

// ID returns the definition ID verbatim.
func (d *definitionChallenge) ID() challenge.ID {
	return d.def.ID
}

// Name returns the definition Name verbatim.
func (d *definitionChallenge) Name() string {
	return d.def.Name
}

// Description returns the definition Description verbatim.
func (d *definitionChallenge) Description() string {
	return d.def.Description
}

// Category returns the definition Category verbatim.
func (d *definitionChallenge) Category() string {
	return d.def.Category
}

// Dependencies returns the definition Dependencies verbatim.
func (d *definitionChallenge) Dependencies() []challenge.ID {
	return d.def.Dependencies
}

// Configure captures the runtime config for use in Execute.
func (d *definitionChallenge) Configure(cfg *challenge.Config) error {
	d.cfg = cfg
	return nil
}

// Validate is a no-op for declarative definitions — preconditions
// declared in def.Inputs are validated at Execute time when a real
// backend dispatches against them.
func (d *definitionChallenge) Validate(ctx context.Context) error {
	return nil
}

// executableShellSteps returns the bank case's steps whose action is
// a host shell command (`shell:`) that the desktop platform can run.
// The returned slice preserves bank order. An empty slice means the
// case has nothing the desktop platform can execute.
func (d *definitionChallenge) executableShellSteps() []testbank.TestStep {
	if d.testCase == nil {
		return nil
	}
	var out []testbank.TestStep
	for _, step := range d.testCase.Steps {
		// Honour an explicit per-step skip.
		if step.Skip {
			continue
		}
		// A step pinned to a non-target platform is not ours to run.
		if step.Platform != "" && step.Platform != d.platform {
			continue
		}
		actionType, _ := step.ParseAction()
		if actionType == testbank.ActionTypeShell {
			out = append(out, step)
		}
	}
	return out
}

// Execute dispatches the definition.
//
// HXC-011 fix: on the desktop platform, when the wrapper carries an
// executable bank case with one or more `shell:` steps, each step's
// command is genuinely run via os/exec — the real exit code drives
// the verdict. Otherwise the result is an honest Skipped with an
// explicit reason (never a hollow PASS).
func (d *definitionChallenge) Execute(ctx context.Context) (*challenge.Result, error) {
	start := time.Now()

	// Real execution path: desktop platform + executable shell steps.
	if d.platform == config.PlatformDesktop {
		if steps := d.executableShellSteps(); len(steps) > 0 {
			return d.executeDesktopShellSteps(ctx, start, steps), nil
		}
	}

	// Real execution path: android platform + a wired vision-nav context.
	// The vision loop DRIVES the device (launch → screenshot → LLM decide
	// → dispatch → capture evidence) and scores on the §11.4.52
	// goal-reached-AND-screen-changed verdict — never a fake PASS. When
	// the context is absent or partially-wired, fall through to the honest
	// skip below (NOT a PASS).
	if d.platform == config.PlatformAndroid &&
		d.testCase != nil &&
		d.platformMatches() &&
		d.androidCtx.valid() {
		return d.executeAndroidVisionSteps(ctx, start), nil
	}

	// Honest-skip path: nothing this platform/wrapper can execute.
	return d.skippedResult(start), nil
}

// deriveScreenGoals builds the vision-nav success criterion for this
// bank case from the case's own fields, in priority order:
//
//	1. RequiredEvidence entries (each is a consumer-authored success
//	   token — the §11.4.69 evidence-ledger vocabulary),
//	2. the case's ExpectedResult string (the "what should the user see"
//	   field authors write),
//	3. as a last resort, the case Name (so a target always has at least
//	   one goal — Target.Validate rejects an empty ScreenGoals list).
//
// Only non-empty tokens are returned. Project-agnostic: every token is
// consumer data lifted off the bank case, never a HelixQA literal.
func (d *definitionChallenge) deriveScreenGoals() []string {
	var goals []string
	if d.testCase != nil {
		for _, e := range d.testCase.RequiredEvidence {
			if strings.TrimSpace(e) != "" {
				goals = append(goals, e)
			}
		}
		if er := strings.TrimSpace(d.testCase.ExpectedResult); er != "" {
			goals = append(goals, er)
		}
		if len(goals) == 0 {
			if n := strings.TrimSpace(d.testCase.Name); n != "" {
				goals = append(goals, n)
			}
		}
	}
	return goals
}

// deriveLaunchAction returns the first action the Session dispatches to
// bring the target on-screen, in the executor's action grammar
// (pkg/visionnav/adb_actor.go). It prefers the bank case's first
// executable `adb_shell:` / `shell:` step (the case author's explicit
// launch command), expressed as a `shell <cmd>` grammar action. When no
// such step exists, it falls back to a generic `launch monkey -p
// <package> 1` form using the consumer-supplied package name carried on
// the bank case via DispatchesTo — or, absent that, an honest empty
// string which Target.Validate rejects (so the caller learns the case is
// not launchable rather than silently no-op'ing).
func (d *definitionChallenge) deriveLaunchAction() string {
	if d.testCase == nil {
		return ""
	}
	for _, step := range d.testCase.Steps {
		if step.Skip {
			continue
		}
		at, val := step.ParseAction()
		switch at {
		case testbank.ActionTypeShell, testbank.ActionTypeADBShell:
			if strings.TrimSpace(val) != "" {
				return "shell " + val
			}
		}
	}
	// No explicit launch step. DispatchesTo, when set, is a
	// consumer-resolved command line — run it verbatim via `shell`.
	if dt := strings.TrimSpace(d.testCase.DispatchesTo); dt != "" {
		return "shell " + dt
	}
	return ""
}

// executeAndroidVisionSteps drives the real device through the existing
// pkg/visionnav.Session loop for this bank case. It builds a Target from
// the case (launch action + derived ScreenGoals), runs the Session for
// MaxSteps, and maps the §11.4.52 verdict onto a challenge.Result:
//
//   - SessionResult.Passed (goal reached AND screen changed) → StatusPassed
//   - otherwise                                              → StatusFailed
//
// It NEVER returns Skipped (the caller already proved the context is
// wired + the case matches the platform), and it NEVER manufactures a
// PASS — the verdict comes straight from the real run's captured
// Evidence. An infrastructure failure (screenshot/dispatch/provider
// error mid-loop) scores StatusError with the real error text.
//
// The captured per-step Evidence (RecordedActions + Assertions) is the
// §11.4.83 transcript: the action dispatched per step, the Provider's
// rationale, and the OCR snapshot that did (or did not) match the goal.
func (d *definitionChallenge) executeAndroidVisionSteps(
	ctx context.Context,
	start time.Time,
) *challenge.Result {
	goals := d.deriveScreenGoals()
	launch := d.deriveLaunchAction()

	target := visionnav.Target{
		Name:         d.bankCaseName(),
		LaunchAction: launch,
		ScreenGoals:  goals,
	}
	if err := target.Validate(); err != nil {
		// The case cannot be expressed as a drivable target (no launch
		// action OR no goal). This is an honest FAIL — the case claims to
		// be an android UI case but carries nothing to drive/verify, so we
		// surface it loudly rather than skip it silently.
		return d.androidResult(start, challenge.StatusFailed, nil, nil,
			fmt.Sprintf("android bank case is not drivable: %v", err))
	}

	sess, err := visionnav.NewSession(visionnav.SessionConfig{
		Provider: d.androidCtx.Provider,
		Actor:    d.androidCtx.Actor,
		Explorer: d.androidCtx.Explorer,
		Target:   target,
		MaxSteps: d.androidCtx.MaxSteps,
	})
	if err != nil {
		return d.androidResult(start, challenge.StatusError, nil, nil,
			fmt.Sprintf("android vision session construction failed: %v", err))
	}

	res, runErr := sess.Run(ctx)
	if runErr != nil {
		// Infrastructure failure aborted the loop (real screenshot /
		// dispatch / provider / explorer error). Report StatusError with
		// the real cause + whatever evidence accumulated before the abort.
		recorded, assertions := d.visionEvidence(res, target)
		return d.androidResult(start, challenge.StatusError,
			recorded, assertions,
			fmt.Sprintf("android vision run aborted: %v", runErr))
	}

	recorded, assertions := d.visionEvidence(res, target)
	if res.Passed {
		return d.androidResult(start, challenge.StatusPassed,
			recorded, assertions, "")
	}
	return d.androidResult(start, challenge.StatusFailed,
		recorded, assertions, res.Reason)
}

// bankCaseName returns a stable, non-empty name for the target. The case
// ID is used (Target.Name must be non-empty + unique); falls back to the
// definition ID when the case ID is empty.
func (d *definitionChallenge) bankCaseName() string {
	if d.testCase != nil && strings.TrimSpace(d.testCase.ID) != "" {
		return d.testCase.ID
	}
	return string(d.def.ID)
}

// visionEvidence turns a SessionResult into the RecordedActions +
// Assertions that back the challenge.Result. Every dispatched action,
// the Provider rationale, the OCR snapshot, and the final §11.4.52
// verdict become forensic lines — a hollow PASS is impossible because
// these are lifted from the real run's captured Evidence (Duration is
// real wall-clock; the screen-delta + goal-match booleans are the
// Session's own computed verdict).
func (d *definitionChallenge) visionEvidence(
	res *visionnav.SessionResult,
	target visionnav.Target,
) ([]string, []challenge.AssertionResult) {
	recorded := []string{
		"android-vision: serial=" + d.androidCtx.Serial,
		"android-vision: launch=" + target.LaunchAction,
	}
	if res == nil {
		return recorded, nil
	}
	for i, ev := range res.Evidence {
		if ev == nil {
			continue
		}
		recorded = append(recorded,
			fmt.Sprintf("android-vision: step[%d] desc=%q verdict=%q",
				i, ev.Description, ev.Verdict))
		if ev.OCRSnapshot != "" {
			recorded = append(recorded,
				fmt.Sprintf("android-vision: step[%d] ocr=%q",
					i, truncateForRecord(ev.OCRSnapshot)))
		}
		if ev.Notes != "" {
			recorded = append(recorded,
				fmt.Sprintf("android-vision: step[%d] rationale=%q",
					i, truncateForRecord(ev.Notes)))
		}
	}
	recorded = append(recorded,
		fmt.Sprintf("android-vision: verdict reason=%q", res.Reason))

	assertions := []challenge.AssertionResult{
		{
			Type:     "vision-goal-reached",
			Target:   "session.GoalReached",
			Expected: "true",
			Actual:   fmt.Sprintf("%t", res.GoalReached),
			Passed:   res.GoalReached,
			Message: fmt.Sprintf(
				"vision loop reached a registered ScreenGoal within %d steps",
				res.Steps),
		},
		{
			Type:     "vision-screen-changed",
			Target:   "session.ScreenChanged",
			Expected: "true",
			Actual:   fmt.Sprintf("%t", res.ScreenChanged),
			Passed:   res.ScreenChanged,
			Message: "actions produced observable screen change " +
				"(§11.4.52 zero-delta auto-FAIL guard)",
		},
	}
	return recorded, assertions
}

// androidResult assembles the challenge.Result for an Android vision run
// with a real wall-clock duration. status is the caller-computed verdict;
// errMsg is non-empty only for FAIL/ERROR. The Android results NEVER
// carry the skip sentinel, so restoreSkippedFromDefinitionWrapper +
// promoteSkippedToPassed leave them untouched — a real PASS stands, a
// real FAIL stands.
func (d *definitionChallenge) androidResult(
	start time.Time,
	status string,
	recorded []string,
	assertions []challenge.AssertionResult,
	errMsg string,
) *challenge.Result {
	now := time.Now()
	r := &challenge.Result{
		ChallengeID:     d.def.ID,
		ChallengeName:   d.def.Name,
		Status:          status,
		StartTime:       start,
		EndTime:         now,
		Duration:        now.Sub(start),
		RecordedActions: recorded,
		Assertions:      assertions,
	}
	if status != challenge.StatusPassed {
		r.Error = errMsg
	}
	return r
}

// executeDesktopShellSteps runs every `shell:` step of the bank case
// via os/exec on the host, captures real exit codes + combined
// output, and scores PASS only when every step exits 0. Any non-zero
// exit (or spawn error) scores FAIL — a hollow metadata-only PASS is
// impossible because RecordedActions carry the real command output
// and the Duration is real wall-clock time.
func (d *definitionChallenge) executeDesktopShellSteps(
	ctx context.Context,
	start time.Time,
	steps []testbank.TestStep,
) *challenge.Result {
	recorded := make([]string, 0, len(steps)*2+1)
	assertions := make([]challenge.AssertionResult, 0, len(steps))
	allPassed := true
	var firstFailure string

	for i, step := range steps {
		_, command := step.ParseAction()
		stepTimeout := 30 * time.Second
		if d.cfg != nil && d.cfg.Timeout > 0 {
			stepTimeout = d.cfg.Timeout
		}
		if step.Timeout > 0 {
			stepTimeout = time.Duration(step.Timeout) * time.Second
		}

		stepCtx, cancel := context.WithTimeout(ctx, stepTimeout)
		cmd := exec.CommandContext(stepCtx, "sh", "-c", command)
		output, runErr := cmd.CombinedOutput()
		exitCode := 0
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		cancel()

		recorded = append(recorded,
			fmt.Sprintf("shell-step[%d]: %s", i, command))
		recorded = append(recorded,
			fmt.Sprintf("shell-step[%d]: exit=%d output=%q",
				i, exitCode, truncateForRecord(string(output))))

		stepPassed := runErr == nil && exitCode == 0
		if !stepPassed {
			allPassed = false
			if firstFailure == "" {
				firstFailure = fmt.Sprintf(
					"step %d %q exited %d: %s",
					i, command, exitCode,
					truncateForRecord(string(output)))
			}
		}

		assertions = append(assertions, challenge.AssertionResult{
			Type:     "shell-exit-zero",
			Target:   fmt.Sprintf("step[%d].exit_code", i),
			Expected: "0",
			Actual:   fmt.Sprintf("%d", exitCode),
			Passed:   stepPassed,
			Message: fmt.Sprintf(
				"desktop shell action %q exited %d", command, exitCode),
		})
	}

	now := time.Now()
	result := &challenge.Result{
		ChallengeID:     d.def.ID,
		ChallengeName:   d.def.Name,
		StartTime:       start,
		EndTime:         now,
		Duration:        now.Sub(start),
		RecordedActions: recorded,
		Assertions:      assertions,
	}
	if allPassed {
		result.Status = challenge.StatusPassed
	} else {
		result.Status = challenge.StatusFailed
		result.Error = "desktop shell action failed: " + firstFailure
	}
	return result
}

// skippedResult builds the honest-skip result for a wrapper that has
// nothing the current platform can execute. The skip reason is
// explicit (§11.4.3) and carries the definitionWrapperSkipSentinel so
// restoreSkippedFromDefinitionWrapper keeps the Skipped status after
// the challenges-runner merge.
func (d *definitionChallenge) skippedResult(start time.Time) *challenge.Result {
	now := time.Now()
	var skipReason string
	switch {
	case d.testCase == nil:
		skipReason = "declarative-only definition (no executable bank " +
			"case loaded) — no backend dispatcher for this Category"
	case d.platform != config.PlatformDesktop:
		skipReason = fmt.Sprintf(
			"bank case has no executable action for the %q platform "+
				"(needs an Android/UI/web topology backend)", d.platform)
	default:
		skipReason = "bank case has no desktop-executable `shell:` " +
			"action — convert prose steps to `action: \"shell: <cmd>\"`"
	}
	return &challenge.Result{
		ChallengeID:   d.def.ID,
		ChallengeName: d.def.Name,
		Status:        challenge.StatusSkipped,
		StartTime:     start,
		EndTime:       now,
		Duration:      now.Sub(start),
		RecordedActions: []string{
			"definition-loaded: id=" + string(d.def.ID),
			"definition-loaded: category=" + d.def.Category,
			definitionWrapperSkipSentinel + " " + skipReason,
		},
		Assertions: []challenge.AssertionResult{{
			Type:     "definition-loaded",
			Target:   "definition.ID",
			Expected: string(d.def.ID),
			Actual:   string(d.def.ID),
			Passed:   true,
			Message: "bank-loaded definition bridged to Challenge " +
				"interface; honestly skipped (no executable action)",
		}},
		Error: "",
	}
}

// truncateForRecord caps command output captured into RecordedActions
// so a noisy command does not bloat the report.
func truncateForRecord(s string) string {
	const max = 512
	s = strings.TrimRight(s, "\n")
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

// Cleanup is a no-op for declarative definitions.
func (d *definitionChallenge) Cleanup(ctx context.Context) error {
	return nil
}

// Compile-time interface assertion.
var _ challenge.Challenge = (*definitionChallenge)(nil)
