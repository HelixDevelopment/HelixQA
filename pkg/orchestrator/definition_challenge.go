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
)

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

	// Honest-skip path: nothing this platform/wrapper can execute.
	return d.skippedResult(start), nil
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
