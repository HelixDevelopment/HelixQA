// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package challengegen

import (
	"testing"
	"time"

	"digital.vasic.challenges/pkg/challenge"
)

// res is a small constructor for a realistic outcome record.
func res(id, name, status string) challenge.Result {
	return challenge.Result{
		ChallengeID:   challenge.ID(id),
		ChallengeName: name,
		Status:        status,
		StartTime:     time.Unix(1700000000, 0),
		EndTime:       time.Unix(1700000003, 0),
		Duration:      3 * time.Second,
	}
}

// byID indexes generated definitions for assertion convenience.
func byID(defs []challenge.Definition) map[challenge.ID]challenge.Definition {
	m := make(map[challenge.ID]challenge.Definition, len(defs))
	for _, d := range defs {
		m[d.ID] = d
	}
	return m
}

// --- core rules -----------------------------------------------------

func TestGenerateFromOutcomes_FailureBecomesRegression(t *testing.T) {
	failing := res("video-playback", "Video Playback", challenge.StatusFailed)
	failing.Error = "no frames decoded"

	got := GenerateFromOutcomes([]challenge.Result{
		res("audio-output", "Audio Output", challenge.StatusPassed),
		failing,
	})

	if len(got) != 1 {
		t.Fatalf("want exactly 1 generated challenge (regression), got %d: %+v", len(got), got)
	}
	d := got[0]
	if d.ID != "regen-video-playback" {
		t.Errorf("regression ID = %q, want regen-video-playback", d.ID)
	}
	if d.Category != string(KindRegression) {
		t.Errorf("regression Category = %q, want %q", d.Category, KindRegression)
	}
	if len(d.Dependencies) != 1 || d.Dependencies[0] != "video-playback" {
		t.Errorf("regression must depend on source feature, got %v", d.Dependencies)
	}
	// Evidence from the failing outcome must travel into the challenge.
	if len(d.Assertions) != 1 || d.Assertions[0].Message == "" {
		t.Fatalf("regression must carry an assertion w/ message, got %+v", d.Assertions)
	}
	if want := "no frames decoded"; !contains(d.Assertions[0].Message, want) {
		t.Errorf("regression assertion message %q must reference defect %q",
			d.Assertions[0].Message, want)
	}
}

func TestGenerateFromOutcomes_FlakyBecomesStability(t *testing.T) {
	got := GenerateFromOutcomes([]challenge.Result{
		res("login-flow", "Login Flow", challenge.StatusPassed),
		res("login-flow", "Login Flow", challenge.StatusFailed),
		res("login-flow", "Login Flow", challenge.StatusPassed),
	})

	if len(got) != 1 {
		t.Fatalf("want 1 stability challenge for a flaky feature, got %d: %+v", len(got), got)
	}
	d := got[0]
	if d.ID != "stabgen-login-flow" {
		t.Errorf("stability ID = %q, want stabgen-login-flow", d.ID)
	}
	if d.Category != string(KindStability) {
		t.Errorf("stability Category = %q, want %q", d.Category, KindStability)
	}
}

func TestGenerateFromOutcomes_SkippedBecomesCoverage(t *testing.T) {
	got := GenerateFromOutcomes([]challenge.Result{
		res("hdr-passthrough", "HDR Passthrough", challenge.StatusSkipped),
	})

	if len(got) != 1 {
		t.Fatalf("want 1 coverage challenge for a skip-only feature, got %d: %+v", len(got), got)
	}
	d := got[0]
	if d.ID != "covgen-hdr-passthrough" {
		t.Errorf("coverage ID = %q, want covgen-hdr-passthrough", d.ID)
	}
	if d.Category != string(KindCoverage) {
		t.Errorf("coverage Category = %q, want %q", d.Category, KindCoverage)
	}
}

func TestGenerateFromOutcomes_AllPassNoSpuriousChallenges(t *testing.T) {
	got := GenerateFromOutcomes([]challenge.Result{
		res("a", "A", challenge.StatusPassed),
		res("b", "B", challenge.StatusPassed),
		res("a", "A", challenge.StatusPassed), // repeat pass
	})
	if len(got) != 0 {
		t.Fatalf("all-pass input must produce ZERO challenges, got %d: %+v", len(got), got)
	}
}

func TestGenerateFromOutcomes_DedupRepeatedFailures(t *testing.T) {
	got := GenerateFromOutcomes([]challenge.Result{
		res("flash", "Flash", challenge.StatusFailed),
		res("flash", "Flash", challenge.StatusFailed),
		res("flash", "Flash", challenge.StatusFailed),
	})
	if len(got) != 1 {
		t.Fatalf("repeated failures of one feature must dedup to 1 challenge, got %d: %+v",
			len(got), got)
	}
	if got[0].ID != "regen-flash" {
		t.Errorf("deduped ID = %q, want regen-flash", got[0].ID)
	}
}

// --- ordering / determinism ----------------------------------------

func TestGenerateFromOutcomes_StableSortedOrder(t *testing.T) {
	in := []challenge.Result{
		res("zeta", "Zeta", challenge.StatusFailed),
		res("alpha", "Alpha", challenge.StatusFailed),
		res("mike", "Mike", challenge.StatusSkipped),
	}
	got := GenerateFromOutcomes(in)
	if len(got) != 3 {
		t.Fatalf("want 3 challenges, got %d", len(got))
	}
	want := []challenge.ID{"covgen-mike", "regen-alpha", "regen-zeta"}
	for i, w := range want {
		if got[i].ID != w {
			t.Errorf("order[%d] = %q, want %q (got order: %v)",
				i, got[i].ID, w, ids(got))
		}
	}
	// Determinism: re-running the same input yields identical IDs.
	got2 := GenerateFromOutcomes(in)
	for i := range got {
		if got[i].ID != got2[i].ID {
			t.Fatalf("non-deterministic output at [%d]: %q vs %q",
				i, got[i].ID, got2[i].ID)
		}
	}
}

// --- precedence + edge inputs --------------------------------------

func TestGenerateFromOutcomes_LatestFailureWins(t *testing.T) {
	// A pass followed by a later skip is NOT failing -> no challenge.
	got := GenerateFromOutcomes([]challenge.Result{
		res("x", "X", challenge.StatusPassed),
		res("x", "X", challenge.StatusSkipped),
	})
	if len(got) != 0 {
		t.Fatalf("pass-then-skip must not generate a challenge, got %+v", got)
	}
}

func TestGenerateFromOutcomes_EmptyIDIgnored(t *testing.T) {
	got := GenerateFromOutcomes([]challenge.Result{
		{ChallengeID: "", ChallengeName: "no id", Status: challenge.StatusFailed},
	})
	if len(got) != 0 {
		t.Fatalf("outcome with empty ChallengeID must be ignored, got %+v", got)
	}
}

func TestGenerateFromOutcomes_TimedOutAndErrorCountAsRegression(t *testing.T) {
	for _, st := range []string{
		challenge.StatusTimedOut, challenge.StatusStuck, challenge.StatusError,
	} {
		got := GenerateFromOutcomes([]challenge.Result{res("feat-"+st, "F", st)})
		if len(got) != 1 || got[0].Category != string(KindRegression) {
			t.Fatalf("status %q must yield a regression challenge, got %+v", st, got)
		}
	}
}

func TestGenerateFromOutcomes_EmptyInput(t *testing.T) {
	if got := GenerateFromOutcomes(nil); len(got) != 0 {
		t.Fatalf("nil input must yield no challenges, got %+v", got)
	}
}

// --- validity of generated challenges (real validator) -------------

func TestGenerateFromOutcomes_GeneratedAreValid(t *testing.T) {
	got := GenerateFromOutcomes([]challenge.Result{
		res("feat-fail", "Fail Feature", challenge.StatusFailed),
		res("feat-flaky", "Flaky Feature", challenge.StatusPassed),
		res("feat-flaky", "Flaky Feature", challenge.StatusFailed),
		res("feat-skip", "Skip Feature", challenge.StatusSkipped),
	})
	if len(got) != 3 {
		t.Fatalf("want 3 generated challenges, got %d: %v", len(got), ids(got))
	}
	// Round-trip through the REAL bank.ValidateFile validator.
	if err := ValidateGenerated(got); err != nil {
		t.Fatalf("generated challenges must be valid per the real validator: %v", err)
	}
	// Spot-check the invariants the validator enforces directly.
	m := byID(got)
	for id, d := range m {
		if string(d.ID) == "" {
			t.Errorf("generated challenge has empty ID")
		}
		if d.Name == "" {
			t.Errorf("generated challenge %s has empty Name", id)
		}
	}
}

// --- helpers --------------------------------------------------------

func ids(defs []challenge.Definition) []challenge.ID {
	out := make([]challenge.ID, len(defs))
	for i, d := range defs {
		out[i] = d.ID
	}
	return out
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
