// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package challengegen implements HXC-1259: deterministic,
// rule-based generation of new HelixQA Challenges from completed
// test/Challenge OUTCOMES.
//
// It builds directly on the REAL Challenges-module types — it does
// NOT invent a parallel Challenge or outcome type:
//
//   - An OUTCOME is a digital.vasic.challenges/pkg/challenge.Result
//     (the canonical record a runner produces for one challenge run:
//     ChallengeID, ChallengeName, Status [passed/failed/skipped/...],
//     Duration, RecordedActions, Error). It is exactly "a pass/fail
//     result with evidence".
//
//   - A generated CHALLENGE is a challenge.Definition — the
//     declarative, behaviour-free Challenge record the bank loader
//     and registry already consume. A Definition that round-trips
//     through a BankFile passes the real bank.ValidateFile gate.
//
// The generator targets the GAPS revealed by a set of outcomes:
//
//   - a FAILED feature  -> a regression Challenge (re-prove it works)
//   - a FLAKY feature   -> a stability Challenge (prove it is
//     deterministic — the same ChallengeID seen both passed AND
//     failed across the outcome set is flaky)
//   - a SKIPPED feature -> a coverage Challenge (the path was never
//     actually exercised; close the coverage gap)
//
// All-pass input produces NO spurious challenges. Generation is
// deterministic: one challenge per distinct targeted feature, deduped
// by source ChallengeID, emitted in a stable (sorted) order, with no
// time.Now / rand in the logic under test.
package challengegen

import (
	"fmt"
	"sort"

	"digital.vasic.challenges/pkg/challenge"
)

// Kind enumerates the rule-based challenge kinds this generator
// produces. It is a thin classification label written into the
// generated Definition's Category and (prefix of its) ID so callers
// and tests can assert which gap a generated challenge targets.
type Kind string

const (
	// KindRegression targets a feature whose latest outcome FAILED.
	KindRegression Kind = "regression"

	// KindStability targets a feature whose outcomes were FLAKY
	// (the same ChallengeID seen both passed and failed).
	KindStability Kind = "stability"

	// KindCoverage targets a feature whose only outcomes were
	// SKIPPED — the path was never genuinely exercised.
	KindCoverage Kind = "coverage"
)

// idPrefix returns the deterministic Definition.ID prefix for a kind.
func (k Kind) idPrefix() string {
	switch k {
	case KindRegression:
		return "regen"
	case KindStability:
		return "stabgen"
	case KindCoverage:
		return "covgen"
	default:
		return "gen"
	}
}

// passedStatuses / failedStatuses classify a raw outcome Status into
// the two buckets the generator reasons about. Skipped, pending,
// running and the unknown remainder are handled explicitly by the
// caller (classifyFeature) and are intentionally NOT lumped into
// pass/fail.
func isPassed(status string) bool {
	return status == challenge.StatusPassed
}

// isFailed reports whether a status represents a genuine negative
// terminal outcome (a real defect signal), as opposed to a skip or a
// not-yet-run state. timed_out / stuck / error all mean "the feature
// did not demonstrably work for the end user" and therefore count as
// failures for regression-targeting purposes (Constitution §11.4 —
// absence of a positive result is not a pass).
func isFailed(status string) bool {
	switch status {
	case challenge.StatusFailed, challenge.StatusTimedOut,
		challenge.StatusStuck, challenge.StatusError:
		return true
	}
	return false
}

// featureClass is the per-feature verdict the generator derives by
// folding every outcome that shares a ChallengeID.
type featureClass int

const (
	classAllPass    featureClass = iota // every run passed -> nothing to do
	classRegression                     // latest run is a genuine failure
	classFlaky                          // saw BOTH a pass and a failure
	classCoverage                       // only ever skipped (never run)
	classUnknown                        // only non-terminal states (pending/running)
)

// aggregate folds the outcomes for a single ChallengeID into the
// signals the classifier needs. Outcomes are assumed to be in their
// original observation order; "latest" therefore means last in the
// slice.
type aggregate struct {
	id         challenge.ID
	name       string
	sawPass    bool
	sawFail    bool
	sawSkip    bool
	sawOther   bool // pending / running / other non-terminal
	latestFail bool

	// failExample is a representative failing outcome (evidence),
	// held BY POINTER — challenge.Result carries an unexported
	// sync.Mutex (guarding concurrent RecordAction appends;
	// pkg/challenge/result.go) and copying a Result by value copies
	// that lock, which `go vet` correctly flags (HXC-140). Storing a
	// pointer shares the original outcome instead of copying it, so
	// aggregate itself never embeds a lock and can be freely copied
	// (see classify/buildDefinition/regressionAssertionMessage,
	// which all take aggregate by value).
	failExample *challenge.Result
}

// classify reduces an aggregate to a single verdict. The order of
// checks is meaningful and deterministic:
//
//  1. pass + fail together  -> flaky (stability gap), highest signal
//  2. latest run failed     -> regression gap
//  3. only ever skipped     -> coverage gap
//  4. at least one pass and nothing bad -> all-pass (no challenge)
//  5. anything else (only pending/running) -> unknown (no challenge)
func (a aggregate) classify() featureClass {
	if a.sawPass && a.sawFail {
		return classFlaky
	}
	if a.latestFail {
		return classRegression
	}
	if a.sawSkip && !a.sawPass && !a.sawFail && !a.sawOther {
		return classCoverage
	}
	if a.sawPass && !a.sawFail {
		return classAllPass
	}
	return classUnknown
}

// GenerateFromOutcomes is the public, deterministic, rule-based API.
//
// Given a set of completed test/Challenge OUTCOMES (challenge.Result
// records), it returns the set of new challenge.Definition records
// that target the gaps those outcomes reveal:
//
//   - one regression Challenge per distinct feature whose latest
//     outcome failed,
//   - one stability Challenge per distinct feature that was flaky,
//   - one coverage Challenge per distinct feature that was only ever
//     skipped.
//
// Guarantees:
//   - all-pass input -> empty (non-nil-or-nil) result, never a
//     spurious challenge;
//   - dedup: repeated outcomes for the same ChallengeID collapse to
//     at most one generated challenge;
//   - stable ordering: results are sorted by generated Definition.ID,
//     so the same input always yields byte-identical output;
//   - every returned Definition is VALID per the real bank validator
//     (non-empty unique ID + non-empty Name) — see Validate.
//
// Outcomes with an empty ChallengeID are ignored (they cannot be
// targeted) rather than producing a malformed challenge.
//
// Outcomes are taken by pointer ([]*challenge.Result), matching the
// convention the challenge package itself uses everywhere Result is
// handled (Execute, RecordAction, AllPassed, IsFinal all operate on
// *Result — see digital.vasic.challenges/pkg/challenge). Result embeds
// an unexported sync.Mutex (RecordAction's action-trace lock), so a
// []challenge.Result value slice forces every element into it to be
// copied by value at the call site, which `go vet` flags as an unsafe
// lock copy (HXC-140). Pointers avoid the copy entirely.
func GenerateFromOutcomes(outcomes []*challenge.Result) []challenge.Definition {
	// Fold outcomes by ChallengeID, preserving first-seen order so a
	// missing/duplicate observation never changes the verdict.
	order := make([]challenge.ID, 0, len(outcomes))
	aggs := make(map[challenge.ID]*aggregate, len(outcomes))

	for i := range outcomes {
		o := outcomes[i]
		if o == nil || o.ChallengeID == "" {
			continue
		}
		a, ok := aggs[o.ChallengeID]
		if !ok {
			a = &aggregate{id: o.ChallengeID, name: o.ChallengeName}
			aggs[o.ChallengeID] = a
			order = append(order, o.ChallengeID)
		}
		// A later outcome carrying a name fills in a missing one.
		if a.name == "" && o.ChallengeName != "" {
			a.name = o.ChallengeName
		}

		switch {
		case isPassed(o.Status):
			a.sawPass = true
			a.latestFail = false
		case isFailed(o.Status):
			a.sawFail = true
			a.latestFail = true
			a.failExample = o
		case o.Status == challenge.StatusSkipped:
			a.sawSkip = true
			// a skip does not change latestFail — a skip after a
			// pass is still "not currently failing".
		default:
			a.sawOther = true
		}
	}

	var out []challenge.Definition
	for _, id := range order {
		a := aggs[id]
		var kind Kind
		switch a.classify() {
		case classRegression:
			kind = KindRegression
		case classFlaky:
			kind = KindStability
		case classCoverage:
			kind = KindCoverage
		default:
			// classAllPass / classUnknown -> no challenge.
			continue
		}
		out = append(out, buildDefinition(kind, *a))
	}

	// Stable ordering: sort by the generated Definition.ID. Generated
	// IDs are unique by construction (kind-prefix + source ID), so the
	// sort is total and deterministic.
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})

	return out
}

// buildDefinition assembles a single generated Definition for a kind
// + aggregated feature. The shape is intentionally minimal but VALID:
// it always has a non-empty unique ID and a non-empty Name, an
// honest Description naming the source outcome, a Category equal to
// the Kind, and a Dependencies edge back to the source challenge so
// the generated challenge runs after the feature it targets.
func buildDefinition(kind Kind, a aggregate) challenge.Definition {
	srcName := a.name
	if srcName == "" {
		srcName = string(a.id)
	}

	def := challenge.Definition{
		ID:           challenge.ID(fmt.Sprintf("%s-%s", kind.idPrefix(), a.id)),
		Name:         fmt.Sprintf("%s: %s", kindTitle(kind), srcName),
		Category:     string(kind),
		Dependencies: []challenge.ID{a.id},
		Metrics:      []string{},
	}

	switch kind {
	case KindRegression:
		def.Description = fmt.Sprintf(
			"Auto-generated regression Challenge for feature %q "+
				"(source %s) whose latest outcome was a failure. "+
				"Re-prove the feature works end-to-end for the user.",
			srcName, a.id,
		)
		// Carry the failing outcome's error as the assertion the
		// regression must now satisfy (no_error / must-pass).
		def.Assertions = []challenge.AssertionDef{{
			Type:    "not_empty",
			Target:  "result",
			Message: regressionAssertionMessage(a),
		}}
	case KindStability:
		def.Description = fmt.Sprintf(
			"Auto-generated stability Challenge for feature %q "+
				"(source %s) which produced both passing and "+
				"failing outcomes (flaky). Prove the feature is "+
				"deterministic across repeated runs.",
			srcName, a.id,
		)
		def.Assertions = []challenge.AssertionDef{{
			Type:    "no_duplicates",
			Target:  "result",
			Message: "feature must produce a consistent outcome across runs",
		}}
	case KindCoverage:
		def.Description = fmt.Sprintf(
			"Auto-generated coverage Challenge for feature %q "+
				"(source %s) which was only ever skipped — the path "+
				"was never genuinely exercised. Close the coverage gap.",
			srcName, a.id,
		)
		def.Assertions = []challenge.AssertionDef{{
			Type:    "not_empty",
			Target:  "result",
			Message: "feature path must be exercised, not skipped",
		}}
	}

	return def
}

// regressionAssertionMessage embeds the failing outcome's evidence
// (its recorded error, if any) into the assertion message so the
// generated challenge is auditable back to the defect that motivated
// it (Constitution §11.4 — evidence travels with the artefact).
func regressionAssertionMessage(a aggregate) string {
	if a.failExample != nil && a.failExample.Error != "" {
		return fmt.Sprintf(
			"feature must no longer fail (was: %s)", a.failExample.Error,
		)
	}
	return "feature must pass (previously failing)"
}

// kindTitle renders a human-readable title fragment for a kind.
func kindTitle(k Kind) string {
	switch k {
	case KindRegression:
		return "Regression"
	case KindStability:
		return "Stability"
	case KindCoverage:
		return "Coverage"
	default:
		return "Generated"
	}
}
