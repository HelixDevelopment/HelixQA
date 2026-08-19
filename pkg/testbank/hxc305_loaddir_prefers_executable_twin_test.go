// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// HXC-305 — LoadDir silently discarded the JSON twin of a bank
// whenever a same-named YAML sibling existed (loader.go, historically
// ~lines 107-118), regardless of which of the two forms was actually
// executable. Filed from the HXC-243 investigation: HXC-243 fixed 125
// unasserted http: checks across 7 bank files, but those checks only
// ever run when the bank reaches an executor. A directory scan going
// through THIS package's LoadDir — `helixqa list --banks banks/`,
// `helixqa http --banks banks/`, and pkg/autonomous's
// StructuredTestExecutor — never reached them, because LoadDir always
// kept the legacy YAML prose twin and discarded the JSON form the
// 2026-04-29 bank-prose-to-http.py conversion produced (every affected
// JSON bank carries
// `metadata.bluff_audit_status: "converted-by-bank-prose-to-http.py"`).
// `helixqa run --banks banks/` is NOT one of the affected paths: its
// case source is pkg/orchestrator's loadBanks (digital.vasic.
// challenges/pkg/bank.Bank.LoadDir — a different module's loader,
// with its own, unrelated twin-handling gap) and loadExecutableCases
// (a hand-rolled os.ReadDir + testbank.LoadFile walk that never calls
// THIS LoadDir at all) — neither is touched by this fix.
//
// The ORIGINAL skip's intent (P9, docs/nexus/remaining-work.md,
// 2026-04-18) was to stop LoadDir's own cross-bank duplicate-id guard
// from firing on a YAML/JSON serialisation pair — both twins share
// the identical id set by construction, so loading both unconditionally
// would make LoadDir hard-fail the entire directory scan with a
// "duplicate test case id" error every time a converted pair exists.
// That intent was correct; the polarity was backwards — it always
// kept the non-executable prose form and always discarded whichever
// form actually ran, regardless of which was which.
//
// THE FIX is content-driven, not format-driven: LoadDir counts each
// twin's genuinely-executable steps (ParseAction() != description)
// and PREFERS the twin with STRICTLY more of them for any case shared
// by both, breaking ties toward the pre-existing default (keep YAML)
// so twin pairs that carry zero executable steps in EITHER form are
// unaffected. Critically, the preferred twin is NOT assumed to be a
// full superset of the other's case ids — some real banks convert
// only PART of their case set — so LoadDir MERGES: the preferred
// twin's cases are kept verbatim, and any case id present only in the
// OTHER twin is appended from it, so the loaded case-id set is always
// the UNION of both twins', never a subset of either (see loader.go's
// mergeTwinCases / LoadDirVerbose doc comment — this closes the HXC-
// 305 "B1" finding: an earlier draft of this fix compared executable-
// step counts alone and silently dropped 50 unconverted cases from
// one real bank, including several §11.4.135 regression guards).
// Measured against the full real banks/ corpus (50 twin pairs) before
// this fix shipped: JSON is never strictly worse than its YAML twin
// on executable-step count, confirming the preference rule is safe to
// apply directory-wide rather than gated on the bluff_audit_status
// marker.
//
// A twin's (possibly merged) case ids can also genuinely collide with
// an unrelated bank elsewhere in the directory — a real content-
// authoring defect, not a twin relationship. That must not abort the
// whole directory scan (HXC-305 "B3"): only the specific colliding
// case id is excluded and reported, regardless of whether the twin or
// the unrelated file happens to be processed first.
//
// The decline is never silent: LoadDir logs each declined file and
// why via the standard logger, and the new LoadDirVerbose returns the
// same information structurally so a caller (or a test) can assert on
// it without scraping log output.
func TestHXC305_LoadDirKeepsExecutableTwinOverProseTwin(t *testing.T) {
	dir := filepath.Join("..", "..", "banks")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("SKIP-OK: banks/ dir not present at %s: %v", dir, err)
	}
	banks, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("real banks/ dir must load cleanly: %v", err)
	}

	// One known http:-carrying case id per HXC-243-repaired bank, and
	// the name of its first http: step (see the corresponding
	// banks/<name>.json — verified 2026 by inspection).
	cases := []struct{ bank, id, step string }{
		{"admin-operations", "ADM-001", "Authenticate as admin"},
		{"entity-management", "ENT-001", "Send create entity request"},
		{"full-qa-android", "FQA-AND-150", "Trigger SMB reconnect"},
		{"full-qa-api", "FQA-API-001", "Send login request"},
		{"full-qa-cross-platform", "FQA-XP-001", "Login via API"},
		{"performance-validation", "PERF-004", "Navigate to directory with many files"},
		{"security-validation", "SEC-001", "Access protected endpoint without token"},
	}

	byID := map[string]*TestCase{}
	for _, bf := range banks {
		for i := range bf.TestCases {
			byID[bf.TestCases[i].ID] = &bf.TestCases[i]
		}
	}

	for _, c := range cases {
		tc, ok := byID[c.id]
		if !ok {
			t.Errorf("%s: case %s not found in any loaded bank — LoadDir "+
				"must load SOME twin of %s.{yaml,json}", c.bank, c.id, c.bank)
			continue
		}
		var step *TestStep
		for i := range tc.Steps {
			if tc.Steps[i].Name == c.step {
				step = &tc.Steps[i]
				break
			}
		}
		if step == nil {
			t.Errorf("%s: case %s has no step named %q", c.bank, c.id, c.step)
			continue
		}
		at, _ := step.ParseAction()

		if redModeOn() {
			// RED_MODE=1 reproduces the pre-fix baseline: LoadDir
			// always kept the YAML prose twin, so this step parsed
			// as ActionTypeDescription, never ActionTypeHTTP.
			// Captured once against the pre-fix loader.go as HXC-305
			// evidence; not expected to still hold once the fix has
			// landed (§11.4.115 — a baseline capture, not a standing
			// assertion).
			if at != ActionTypeDescription {
				t.Errorf("RED_MODE=1: %s step %q parsed as %v, want "+
					"ActionTypeDescription (the pre-fix defect: LoadDir "+
					"always keeps the legacy YAML prose twin)",
					c.bank, c.step, at)
			}
			continue
		}

		// GREEN: the standing guard. LoadDir must have kept the
		// executable JSON twin, so this known http:-carrying step
		// actually parses as ActionTypeHTTP.
		if at != ActionTypeHTTP {
			t.Errorf("%s: case %s step %q parsed as %v, want ActionTypeHTTP"+
				" — LoadDir kept the non-executable YAML twin instead of "+
				"the executable JSON twin (HXC-305 regression)",
				c.bank, c.id, c.step, at)
		}
	}
}

// TestHXC305_LoadDirVerboseReportsDeclinedTwinWithReason asserts that
// a twin decline is never silent: it is reported with a reason naming
// the file that superseded it. It also proves the pair never
// double-loads — LoadDirVerbose must return exactly one bank for the
// pair, never two (which would either corrupt downstream counts or
// trip the cross-bank duplicate-id guard).
//
// WHAT THIS TEST DOES NOT PROVE (§11.4.6). An earlier revision of
// this comment called it "the paired-mutation proof that the
// twin-preference decision is content-driven — the JSON twin wins
// because it has more executable steps, not merely because it is
// JSON". Both halves of that claim were false:
//
//   - It contains NO mutation, no §11.4.115 polarity switch and no
//     negative control. It is a single fixture with fixed
//     assertions.
//   - Its fixture cannot discriminate content from format at all. The
//     JSON twin has 1 executable step and the YAML twin has 0, so a
//     hypothetical "JSON always wins" loader satisfies it exactly as
//     well as the real content-driven one. A test that both the
//     correct and the incorrect implementation pass is not evidence
//     for either.
//
// Content-drivenness is established elsewhere, by tests whose
// outcomes actually depend on it: TestHXC305F1_SharedCaseBodyIsChosen
// PerCaseNotPerBank and TestHXC305M7_PerCaseOverrideTakesTheWHOLE
// SecondaryCase (a YAML body winning a shared case outright, which a
// format-driven loader cannot produce), and
// TestHXC305_LoadDirVerboseDeclinesJSONOnTieToo (a tie keeping the
// YAML side). Honest gap, tracked rather than papered over: no
// hermetic fixture in this file has the non-JSON twin winning the
// BANK-level comparison, so bank-level format-independence rests on
// the real-corpus guards rather than on a hermetic case.
func TestHXC305_LoadDirVerboseReportsDeclinedTwinWithReason(t *testing.T) {
	dir := t.TempDir()
	yamlContent := []byte(`name: planted-twin-bank
test_cases:
  - id: HXC305-PLANT-001
    name: "planted twin case"
    platforms: [api]
    steps:
      - name: "do the thing"
        action: "GET /whatever with admin token"
        expected: "200 OK"
`)
	jsonContent := []byte(`{
  "name": "planted-twin-bank",
  "test_cases": [
    {
      "id": "HXC305-PLANT-001",
      "name": "planted twin case",
      "platforms": ["api"],
      "steps": [
        {
          "name": "do the thing",
          "action": "http: GET /whatever",
          "expected": "200 OK",
          "expect_status": 200
        }
      ]
    }
  ]
}`)
	if err := os.WriteFile(filepath.Join(dir, "planted-twin.yaml"), yamlContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "planted-twin.json"), jsonContent, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := LoadDirVerbose(dir)
	if err != nil {
		t.Fatalf("LoadDirVerbose: %v", err)
	}

	if len(res.Banks) != 1 {
		t.Fatalf("loaded %d banks, want exactly 1 (twin must not double-load)",
			len(res.Banks))
	}
	if len(res.Banks[0].TestCases) != 1 {
		t.Fatalf("loaded bank has %d cases, want exactly 1",
			len(res.Banks[0].TestCases))
	}
	at, _ := res.Banks[0].TestCases[0].Steps[0].ParseAction()
	if at != ActionTypeHTTP {
		t.Errorf("loaded twin parsed step as %v, want ActionTypeHTTP "+
			"(the JSON twin must win: 1 executable step vs YAML's 0)", at)
	}

	if len(res.Declined) != 1 {
		t.Fatalf("Declined has %d entries, want exactly 1: %+v",
			len(res.Declined), res.Declined)
	}
	d := res.Declined[0]
	if !strings.HasSuffix(d.Path, "planted-twin.yaml") {
		t.Errorf("declined path = %q, want the YAML twin", d.Path)
	}
	if d.Reason == "" {
		t.Error("declined reason is empty — a decline must always be explained")
	}
	if !strings.Contains(d.Reason, "planted-twin.json") {
		t.Errorf("declined reason %q does not name the superseding file", d.Reason)
	}
}

// TestHXC305_LoadDirVerboseDeclinesJSONOnTieToo proves the decline
// report fires even when the content-driven comparison ties (both
// twins carry zero executable steps) and the pre-existing default
// (keep YAML) is preserved. Silence is forbidden for ANY declined
// file, not only the ones where the preference flips — and this is
// the majority case in the real banks/ corpus.
//
// Measured, not estimated (§11.4.6): of the 50 twin pairs, 32 tie on
// the bank-level executable-step comparison and 18 are strict JSON
// wins; strict non-JSON wins number 0. An earlier revision of this
// comment said "~38-pair", which matches neither figure.
func TestHXC305_LoadDirVerboseDeclinesJSONOnTieToo(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`name: tied-twin-bank
test_cases:
  - id: HXC305-TIE-001
    name: "tied twin case"
    platforms: [api]
    steps:
      - name: "do the thing"
        action: "do the thing manually"
        expected: "it happens"
`)
	jsonContent := []byte(`{
  "name": "tied-twin-bank",
  "test_cases": [
    {
      "id": "HXC305-TIE-001",
      "name": "tied twin case",
      "platforms": ["api"],
      "steps": [
        {"name": "do the thing", "action": "do the thing manually", "expected": "it happens"}
      ]
    }
  ]
}`)
	if err := os.WriteFile(filepath.Join(dir, "tied-twin.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tied-twin.json"), jsonContent, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := LoadDirVerbose(dir)
	if err != nil {
		t.Fatalf("LoadDirVerbose: %v", err)
	}
	if len(res.Banks) != 1 {
		t.Fatalf("loaded %d banks, want exactly 1", len(res.Banks))
	}
	if len(res.Declined) != 1 || !strings.HasSuffix(res.Declined[0].Path, "tied-twin.json") {
		t.Fatalf("expected exactly 1 declined entry for the JSON twin, got %+v",
			res.Declined)
	}
	if res.Declined[0].Reason == "" {
		t.Error("declined reason is empty even on a tie — a decline must always be explained")
	}
}

// TestHXC305_LoadDirDoesNotDoubleLoadOrDuplicateErrorOnTwins answers
// the double-loading question directly: before this fix, naively
// removing the skip (loading BOTH twins unconditionally) would make
// LoadDir's own cross-bank duplicate-id guard fire on every twin pair
// in the real corpus, since YAML and JSON twins share the identical
// id set by construction — hard-failing the ENTIRE directory scan.
// This asserts the real banks/ directory (which has 50 twin pairs)
// still loads with zero duplicate-id errors after the fix.
func TestHXC305_LoadDirDoesNotDoubleLoadOrDuplicateErrorOnTwins(t *testing.T) {
	dir := filepath.Join("..", "..", "banks")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("SKIP-OK: banks/ dir not present at %s: %v", dir, err)
	}
	res, err := LoadDirVerbose(dir)
	if err != nil {
		t.Fatalf("real banks/ dir must load cleanly with no duplicate-id "+
			"error from twin pairs: %v", err)
	}
	if len(res.Declined) == 0 {
		t.Fatal("expected at least one declined twin in the real banks/ corpus")
	}
	seen := map[string]string{}
	for _, bf := range res.Banks {
		for _, tc := range bf.TestCases {
			if tc.ID == "" {
				continue
			}
			if prev, dup := seen[tc.ID]; dup {
				t.Errorf("duplicate id %q across banks (%q and %q) — a twin "+
					"pair double-loaded", tc.ID, prev, bf.Name)
				continue
			}
			seen[tc.ID] = bf.Name
		}
	}
}

// TestHXC305B1_PartialConversionMergesUnconvertedCasesIn is the
// standing regression guard for HXC-305 finding "B1": an earlier
// version of this fix compared ONLY executable-step counts between
// twins, so a twin that converts just PART of a bank's cases could
// still "win" outright (it has more executable steps) and silently
// drop every case it never converted — the exact real-corpus failure
// mode was atmosphere.json converting 129 of atmosphere.yaml's 179
// cases, losing 50 ids including several §11.4.135 regression guards.
//
// This fixture is the minimal reproduction: partial-conv.yaml has 3
// cases and 0 executable steps; partial-conv.json has ONLY 1 of those
// 3 cases (PC-001), converted to a real `http:` step. PC-001 alone
// gives the JSON twin strictly more executable steps than the YAML
// twin (1 > 0), so an executable-count-only comparison would keep
// ONLY PC-001 and silently lose PC-002/PC-003. The fix must keep all
// 3: PC-001 from the executable JSON twin, PC-002/PC-003 merged in
// verbatim from the YAML twin because JSON never converted them.
func TestHXC305B1_PartialConversionMergesUnconvertedCasesIn(t *testing.T) {
	dir := t.TempDir()
	yamlContent := []byte(`name: partial-conv-bank
test_cases:
  - id: PC-001
    name: "case one — the only one JSON converts"
    platforms: [api]
    steps:
      - name: "do it"
        action: "GET /one prose only"
        expected: "ok"
  - id: PC-002
    name: "case two — JSON never converts this one"
    platforms: [api]
    steps:
      - name: "do it"
        action: "GET /two prose only"
        expected: "ok"
  - id: PC-003
    name: "case three — JSON never converts this one either"
    platforms: [api]
    steps:
      - name: "do it"
        action: "GET /three prose only"
        expected: "ok"
`)
	jsonContent := []byte(`{
  "name": "partial-conv-bank",
  "test_cases": [
    {
      "id": "PC-001",
      "name": "case one — the only one JSON converts",
      "platforms": ["api"],
      "steps": [
        {"name": "do it", "action": "http: GET /one", "expected": "ok", "expect_status": 200}
      ]
    }
  ]
}`)
	if err := os.WriteFile(filepath.Join(dir, "partial-conv.yaml"), yamlContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "partial-conv.json"), jsonContent, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := LoadDirVerbose(dir)
	if err != nil {
		t.Fatalf("LoadDirVerbose: %v", err)
	}
	if len(res.Banks) != 1 {
		t.Fatalf("loaded %d banks, want exactly 1 (merged twin pair)", len(res.Banks))
	}

	byID := map[string]*TestCase{}
	for i := range res.Banks[0].TestCases {
		tc := &res.Banks[0].TestCases[i]
		byID[tc.ID] = tc
	}
	if len(byID) != 3 {
		t.Fatalf("loaded %d distinct case ids, want 3 (PC-001, PC-002, PC-003) — "+
			"a partial JSON conversion must not drop the cases it never converted",
			len(byID))
	}

	pc1, ok := byID["PC-001"]
	if !ok {
		t.Fatal("PC-001 missing entirely")
	}
	if at, _ := pc1.Steps[0].ParseAction(); at != ActionTypeHTTP {
		t.Errorf("PC-001 (present in both twins) should keep the executable JSON "+
			"version, parsed as %v, want ActionTypeHTTP", at)
	}

	for _, id := range []string{"PC-002", "PC-003"} {
		tc, ok := byID[id]
		if !ok {
			t.Errorf("%s missing — JSON's partial conversion silently dropped a "+
				"case it never converted (HXC-305 B1 regression)", id)
			continue
		}
		if at, _ := tc.Steps[0].ParseAction(); at != ActionTypeDescription {
			t.Errorf("%s should be the merged-in YAML-only prose version, "+
				"parsed as %v, want ActionTypeDescription", id, at)
		}
	}

	// The decline reason must name the merged-in ids explicitly
	// (§11.4.115 / the reviewer's remedy: "name the lost ids in the
	// reason" — with merging, nothing is lost, but the ids that had
	// to be pulled in from the non-preferred twin must still be named
	// for anyone auditing what happened).
	if len(res.Declined) != 1 {
		t.Fatalf("expected exactly 1 declined entry, got %d: %+v",
			len(res.Declined), res.Declined)
	}
	reason := res.Declined[0].Reason
	for _, want := range []string{"PC-002", "PC-003"} {
		if !strings.Contains(reason, want) {
			t.Errorf("declined reason does not name merged-in id %q: %q", want, reason)
		}
	}
}

// TestHXC305B1_LoadDirNeverLosesACaseIDPresentInAnyBankFile is the
// general, corpus-wide standing guard for finding "B1": it never
// trusts a specific fixture or a specific bank name to still exist —
// instead it independently loads EVERY *.yaml/*.yml/*.json file under
// the real banks/ directory via LoadFile (bypassing LoadDir's twin
// resolution entirely, mirroring the HXC-243 guard's own walk), and
// asserts every distinct case id that exists in ANY file is either
// (a) present in what LoadDirVerbose actually loaded, or (b) named as
// a genuinely excluded id in a Declined reason (the only legitimate
// way for content to be missing — a real cross-bank duplicate, HXC-
// 305 B3). Any id present in an independently-parsed file but absent
// from BOTH — silently vanished — fails this guard.
func TestHXC305B1_LoadDirNeverLosesACaseIDPresentInAnyBankFile(t *testing.T) {
	dir := filepath.Join("..", "..", "banks")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("SKIP-OK: banks/ dir not present at %s: %v", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read banks dir: %v", err)
	}

	// Every distinct case id that exists in ANY top-level bank file,
	// independent of LoadDir's twin resolution.
	everyID := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			continue
		}
		bf, err := LoadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("independently loading %s: %v", e.Name(), err)
		}
		for _, tc := range bf.TestCases {
			if tc.ID != "" {
				everyID[tc.ID] = true
			}
		}
	}
	if len(everyID) == 0 {
		t.Fatal("found 0 case ids scanning banks/ independently — fixture problem")
	}

	res, err := LoadDirVerbose(dir)
	if err != nil {
		t.Fatalf("LoadDirVerbose: %v", err)
	}
	loadedID := map[string]bool{}
	for _, bf := range res.Banks {
		for _, tc := range bf.TestCases {
			if tc.ID != "" {
				loadedID[tc.ID] = true
			}
		}
	}
	// An id may legitimately be missing from loadedID only if a
	// Declined entry explicitly names it as excluded (a genuine
	// cross-bank duplicate, HXC-305 B3) — never for any other reason.
	//
	// This reads the STRUCTURAL ExcludedIDs field rather than
	// substring-scraping the prose Reason (HXC-305 F3). Bank ids in
	// this corpus are not suffix-free — 144 of them end with another
	// corpus id, e.g. "AICHAT-SEC-001" ends with "SEC-001" and
	// "CLI-EXEC-001" ends with "EXEC-001" — so a Reason scrape for
	// `id+" (already claimed by"` can credit id Y as excluded merely
	// because some longer id X ending in Y was, and would then mask a
	// genuine loss of Y. Not exploitable at the time of writing (the
	// only excluded id is HXC029-CLI-PROBE, of which no other id is a
	// suffix), but the guard must not depend on that holding.
	excludedID := map[string]bool{}
	for _, d := range res.Declined {
		for _, id := range d.ExcludedIDs {
			excludedID[id] = true
		}
	}

	var lost []string
	for id := range everyID {
		if !loadedID[id] && !excludedID[id] {
			lost = append(lost, id)
		}
	}
	if len(lost) > 0 {
		sort.Strings(lost)
		t.Fatalf("%d case id(s) present in a bank FILE but absent from LoadDirVerbose's "+
			"loaded banks AND not named as an excluded duplicate — silently lost "+
			"(HXC-305 B1 regression): %s", len(lost), strings.Join(lost, ", "))
	}
}

// TestHXC305B3_TwinExternalCollisionExcludesOnlyThatCaseRegardlessOfOrder
// is the standing regression guard for HXC-305 finding "B3": a twin
// pair's case id can genuinely collide with an id declared by an
// UNRELATED, non-twin bank elsewhere in the directory — a real
// content-authoring defect, not a twin relationship. An earlier
// version of this fix only made the TWIN side of such a collision
// tolerant; a plain file colliding with an id a twin had ALREADY
// claimed still hit the old whole-scan hard-fail, so the outcome
// depended on directory (alphabetical) processing order — proved
// hermetically here by running the identical fixture in both orders.
// Neither order may abort the scan; both must load every OTHER case
// normally and report only the single colliding id as excluded.
func TestHXC305B3_TwinExternalCollisionExcludesOnlyThatCaseRegardlessOfOrder(t *testing.T) {
	fixture := func(dir, twinBase, unrelatedName string) {
		yamlContent := []byte(`name: twin-bank
test_cases:
  - id: SHARED-001
    name: "shared case"
    platforms: [api]
    steps:
      - name: "do it"
        action: "GET /x prose only"
        expected: "ok"
`)
		jsonContent := []byte(`{
  "name": "twin-bank",
  "test_cases": [
    {
      "id": "SHARED-001",
      "name": "shared case",
      "platforms": ["api"],
      "steps": [
        {"name": "do it", "action": "http: GET /x", "expected": "ok", "expect_status": 200}
      ]
    }
  ]
}`)
		unrelatedContent := []byte(`name: unrelated-bank
test_cases:
  - id: SHARED-001
    name: "unrelated case declaring the SAME id"
    platforms: [api]
    steps:
      - name: "do something else"
        action: "GET /y prose"
        expected: "ok"
  - id: UNRELATED-002
    name: "an unrelated case with no collision at all"
    platforms: [api]
    steps:
      - name: "do it"
        action: "GET /z prose"
        expected: "ok"
`)
		if err := os.WriteFile(filepath.Join(dir, twinBase+".yaml"), yamlContent, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, twinBase+".json"), jsonContent, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, unrelatedName), unrelatedContent, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	orders := []struct {
		name, twinBase, unrelatedName string
	}{
		// "aaa-*" sorts before "zzz-*": the twin is processed first,
		// the unrelated plain file second.
		{"twin_first", "aaa-twin", "zzz-unrelated.yaml"},
		// "aaa-*" sorts before "zzz-*" here too, but now it is the
		// UNRELATED file that comes first and the twin second — the
		// reverse of the case above.
		{"unrelated_first", "zzz-twin", "aaa-unrelated.yaml"},
	}

	for _, o := range orders {
		o := o
		t.Run(o.name, func(t *testing.T) {
			dir := t.TempDir()
			fixture(dir, o.twinBase, o.unrelatedName)

			res, err := LoadDirVerbose(dir)
			if err != nil {
				t.Fatalf("LoadDirVerbose FAILED — a twin/unrelated id collision "+
					"must never abort the whole directory scan (HXC-305 B3 "+
					"regression), got: %v", err)
			}

			// Every OTHER case must still have loaded: the twin's
			// SHARED-001 (from whichever side wins the twin
			// comparison) and the unrelated bank's UNRELATED-002.
			ids := map[string]bool{}
			for _, bf := range res.Banks {
				for _, tc := range bf.TestCases {
					ids[tc.ID] = true
				}
			}
			if !ids["SHARED-001"] {
				t.Error("SHARED-001 missing entirely — the twin pair's own " +
					"content must survive even though ITS copy of the id lost " +
					"the cross-bank collision")
			}
			if !ids["UNRELATED-002"] {
				t.Error("UNRELATED-002 missing — a genuinely non-colliding case " +
					"in the same file as the colliding one must still load")
			}

			// Exactly one collision must be reported, naming the id
			// in BOTH forms: the human-readable Reason (the non-
			// silent-decline invariant) and the structural
			// ExcludedIDs field callers actually match on (HXC-305
			// F3 — Reason prose alone is not reliably matchable).
			foundCollision := false
			foundStructural := false
			for _, d := range res.Declined {
				// Match the CATEGORY in prose (a phrase, never an id)
				// and the id STRUCTURALLY below. Scraping Reason for
				// "SHARED-001" is exactly what DeclinedFile.ExcludedIDs
				// documents that callers and tests MUST NOT do: ids in
				// this corpus are not suffix-free, so a prose scrape
				// can credit an unrelated id and mask a real loss.
				// This test was itself doing it (HXC-305 round 6).
				if strings.Contains(d.Reason, "cross-bank duplicate") {
					foundCollision = true
				}
				for _, id := range d.ExcludedIDs {
					if id == "SHARED-001" {
						foundStructural = true
					}
				}
			}
			if !foundCollision {
				t.Errorf("no Declined entry names the SHARED-001 collision: %+v", res.Declined)
			}
			if !foundStructural {
				t.Errorf("no Declined entry reports SHARED-001 in ExcludedIDs — the "+
					"structural form must be populated, not just the prose reason: %+v",
					res.Declined)
			}
		})
	}
}

// caseExecCount reports how many of a loaded case's steps parse as a
// genuinely executable action. Deliberately re-implemented here from
// the public ParseAction rather than calling loader.go's unexported
// countCaseExecutableSteps: a guard that reuses the very helper the
// production decision depends on would pass even if that helper
// itself were mutated to lie.
func caseExecCount(tc *TestCase) int {
	n := 0
	for i := range tc.Steps {
		if at, _ := tc.Steps[i].ParseAction(); at != ActionTypeDescription {
			n++
		}
	}
	return n
}

// TestHXC305F1_SharedCaseBodyIsChosenPerCaseNotPerBank is the standing
// regression guard for HXC-305 finding "F1": an earlier version of
// this fix compared executable-step counts at WHOLE-BANK granularity
// and then kept the winning bank's body for EVERY shared case id.
// That is unsound, because a conversion pass can improve a bank
// overall while making an INDIVIDUAL case worse — de-converting an
// already-executable step back into prose. The bank-level winner then
// drags that per-case regression in with it, and the case silently
// stops executing: the HXC-305 defect class (a silently-preferred
// twin hiding executable steps) reintroduced in the opposite
// direction, invisible to every id-set-based guard because the id set
// is unchanged.
//
// The real-corpus instance this fixture models: atmosphere.json wins
// its bank comparison (49/395 executable steps vs atmosphere.yaml's
// 24/566) yet carries a strictly WORSE body for PA-001, PA-005 and
// VR-015 — each has one executable `adb_shell:` step in the YAML twin
// and none in the JSON twin, so all three stopped executing under
// bank granularity while 684 other cases were genuinely improved.
//
// This fixture is the minimal hermetic reproduction. The JSON twin
// wins the BANK comparison 3-to-1, but the shared case
// F1-YAML-BETTER is executable ONLY in the YAML twin. A correct
// loader keeps the JSON body for F1-JSON-BETTER and the YAML body for
// F1-YAML-BETTER — best available body per case, both directions at
// once, which no single whole-bank preference can produce.
func TestHXC305F1_SharedCaseBodyIsChosenPerCaseNotPerBank(t *testing.T) {
	dir := t.TempDir()
	// YAML bank total: 1 executable step (F1-YAML-BETTER's adb_shell).
	yamlContent := []byte(`name: per-case-pref-bank
test_cases:
  - id: F1-YAML-BETTER
    name: "shared case that only the YAML twin makes executable"
    platforms: [android]
    steps:
      - name: "run the device script"
        action: "adb_shell: sh /data/local/tmp/run.sh"
        expected: "exit 0"
  - id: F1-JSON-BETTER
    name: "shared case that only the JSON twin makes executable"
    platforms: [api]
    steps:
      - name: "call endpoint a"
        action: "GET /a described in prose"
        expected: "200"
      - name: "call endpoint b"
        action: "GET /b described in prose"
        expected: "200"
      - name: "call endpoint c"
        action: "GET /c described in prose"
        expected: "200"
`)
	// JSON bank total: 3 executable steps — so JSON WINS the
	// bank-level comparison 3 > 1 and becomes the primary, even
	// though its F1-YAML-BETTER body is the de-converted prose one.
	jsonContent := []byte(`{
  "name": "per-case-pref-bank",
  "test_cases": [
    {
      "id": "F1-YAML-BETTER",
      "name": "shared case that only the YAML twin makes executable",
      "platforms": ["android"],
      "steps": [
        {"name": "run the device script", "action": "Execute run.sh on the device", "expected": "exit 0"}
      ]
    },
    {
      "id": "F1-JSON-BETTER",
      "name": "shared case that only the JSON twin makes executable",
      "platforms": ["api"],
      "steps": [
        {"name": "call endpoint a", "action": "http: GET /a", "expected": "200", "expect_status": 200},
        {"name": "call endpoint b", "action": "http: GET /b", "expected": "200", "expect_status": 200},
        {"name": "call endpoint c", "action": "http: GET /c", "expected": "200", "expect_status": 200}
      ]
    }
  ]
}`)
	if err := os.WriteFile(filepath.Join(dir, "per-case-pref.yaml"), yamlContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "per-case-pref.json"), jsonContent, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := LoadDirVerbose(dir)
	if err != nil {
		t.Fatalf("LoadDirVerbose: %v", err)
	}
	if len(res.Banks) != 1 {
		t.Fatalf("loaded %d banks, want exactly 1 (merged twin pair)", len(res.Banks))
	}

	byID := map[string]*TestCase{}
	for i := range res.Banks[0].TestCases {
		tc := &res.Banks[0].TestCases[i]
		byID[tc.ID] = tc
	}
	if len(byID) != 2 {
		t.Fatalf("loaded %d distinct case ids, want 2", len(byID))
	}

	// The case the bank-level LOSER holds the better body for. This is
	// the assertion bank-granularity preference fails.
	yb, ok := byID["F1-YAML-BETTER"]
	if !ok {
		t.Fatal("F1-YAML-BETTER missing entirely")
	}

	if redModeOn() {
		// RED_MODE=1 reproduces the F1 defect as it presents on a
		// loader that decides twin preference at WHOLE-BANK
		// granularity: the JSON twin wins the bank comparison 3-to-1
		// and its de-converted prose body is kept for EVERY shared
		// case, so this step parses as ActionTypeDescription and the
		// case stops executing. Until now this finding had no in-source
		// RED reproduction at all — it was argued in prose and pinned
		// only by its GREEN assertion, which cannot show the defect is
		// real (§11.4.115: the bug-catcher and the regression-guard
		// must be one source with a polarity switch). Baseline capture,
		// NOT a standing assertion — expected to fail against the fixed
		// loader.
		if at, _ := yb.Steps[0].ParseAction(); at != ActionTypeDescription {
			t.Errorf("RED_MODE=1: F1-YAML-BETTER step parsed as %v, want "+
				"ActionTypeDescription (the pre-fix defect: a bank-granularity "+
				"twin preference keeps the bank-level winner's de-converted prose "+
				"body for a case the other twin makes executable)", at)
		}
		return
	}

	if at, _ := yb.Steps[0].ParseAction(); at != ActionTypeADBShell {
		t.Errorf("F1-YAML-BETTER step parsed as %v, want ActionTypeADBShell — the "+
			"loader kept the bank-level winner's de-converted prose body for a case "+
			"the other twin makes executable, so this case no longer runs at all "+
			"(HXC-305 F1 regression: twin preference must be per-case, not per-bank)",
			at)
	}
	if n := caseExecCount(yb); n != 1 {
		t.Errorf("F1-YAML-BETTER has %d executable step(s), want 1", n)
	}

	// The case the bank-level WINNER holds the better body for must
	// still resolve to the winner — the fix must not simply invert the
	// old bug.
	jb, ok := byID["F1-JSON-BETTER"]
	if !ok {
		t.Fatal("F1-JSON-BETTER missing entirely")
	}
	if at, _ := jb.Steps[0].ParseAction(); at != ActionTypeHTTP {
		t.Errorf("F1-JSON-BETTER step parsed as %v, want ActionTypeHTTP", at)
	}
	if n := caseExecCount(jb); n != 3 {
		t.Errorf("F1-JSON-BETTER has %d executable step(s), want 3 — per-case "+
			"preference must not regress the cases the primary genuinely improves", n)
	}

	// The decline must NAME the per-case override, not just report the
	// bank-level comparison: an auditor reading the reason has to be
	// able to see that content was taken from the declined file.
	if len(res.Declined) != 1 {
		t.Fatalf("expected exactly 1 declined entry, got %d: %+v",
			len(res.Declined), res.Declined)
	}
	if !strings.Contains(res.Declined[0].Reason, "F1-YAML-BETTER") {
		t.Errorf("declined reason does not name the per-case override F1-YAML-BETTER: %q",
			res.Declined[0].Reason)
	}
}

// TestHXC305F1_LoadDirNeverLoadsAWorseBodyThanATwinOffers is the
// general, corpus-wide standing guard for finding "F1". It trusts no
// specific bank name or case id: it walks EVERY twin pair in the real
// banks/ directory, and for every case id the two twins share it
// computes the best executable-step count either twin offers, then
// asserts what LoadDirVerbose actually loaded is never worse than
// that best. Whole-bank preference violates this the moment any twin
// pair contains a single de-converted case; per-case preference
// cannot violate it by construction.
func TestHXC305F1_LoadDirNeverLoadsAWorseBodyThanATwinOffers(t *testing.T) {
	dir := filepath.Join("..", "..", "banks")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("SKIP-OK: banks/ dir not present at %s: %v", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read banks dir: %v", err)
	}

	// Group files by extension-stripped base name to find twin pairs,
	// independent of LoadDir's own grouping.
	byBase := map[string]map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			continue
		}
		base := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if byBase[base] == nil {
			byBase[base] = map[string]string{}
		}
		byBase[base][ext] = e.Name()
	}

	// bestExec[id] = the highest executable-step count any twin of a
	// pair offers for that case id.
	bestExec := map[string]int{}
	bestFrom := map[string]string{}
	pairs := 0
	for base, m := range byBase {
		jsonName := m[".json"]
		nonJSON := m[".yaml"]
		if nonJSON == "" {
			nonJSON = m[".yml"]
		}
		if jsonName == "" || nonJSON == "" {
			continue // not a twin pair
		}
		pairs++
		for _, name := range []string{jsonName, nonJSON} {
			bf, err := LoadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("independently loading %s (twin of base %q): %v", name, base, err)
			}
			for i := range bf.TestCases {
				tc := &bf.TestCases[i]
				if tc.ID == "" {
					continue
				}
				if n := caseExecCount(tc); n > bestExec[tc.ID] {
					bestExec[tc.ID] = n
					bestFrom[tc.ID] = name
				}
			}
		}
	}
	if pairs == 0 {
		t.Fatal("found 0 twin pairs in banks/ — fixture problem, this guard would be vacuous")
	}

	res, err := LoadDirVerbose(dir)
	if err != nil {
		t.Fatalf("LoadDirVerbose: %v", err)
	}
	loaded := map[string]*TestCase{}
	for _, bf := range res.Banks {
		for i := range bf.TestCases {
			tc := &bf.TestCases[i]
			if tc.ID != "" {
				loaded[tc.ID] = tc
			}
		}
	}

	var demoted []string
	for id, want := range bestExec {
		tc, ok := loaded[id]
		if !ok {
			continue // absence is the B1/B3 guards' business, not this one
		}
		if got := caseExecCount(tc); got < want {
			demoted = append(demoted, fmt.Sprintf(
				"%s: loaded body has %d executable step(s), but its twin %s offers %d",
				id, got, bestFrom[id], want))
		}
	}
	if len(demoted) > 0 {
		sort.Strings(demoted)
		t.Fatalf("%d case(s) loaded with a body strictly WORSE than the one an "+
			"available twin offers — a twin preference decided at bank granularity "+
			"demoted them, so they no longer execute (HXC-305 F1 regression):\n  %s",
			len(demoted), strings.Join(demoted, "\n  "))
	}
}

// TestHXC305F2_TwinParseFailureAsymmetry pins down what happens when
// exactly one twin of a pair fails to parse (HXC-305 finding "F2").
// An intermediate version of this fix quietly downgraded an
// unparseable NON-JSON twin from a hard scan failure to a log line,
// resolving the pair to its JSON sibling instead — which would let a
// truncated or malformed YAML bank silently drop every case the JSON
// form never converted. No test asserted the intent either way, so
// the change was invisible. The intent is now explicit, and
// deliberately ASYMMETRIC, because the two forms have never carried
// the same guarantee:
//
//   - the non-JSON twin has ALWAYS been required to parse, so its
//     failure still hard-fails the whole directory scan;
//   - the JSON twin was historically never even parsed when a
//     non-JSON sibling existed, so its failure must NOT newly
//     hard-fail a directory that always loaded before — the non-JSON
//     twin loads and the failure is REPORTED rather than swallowed.
func TestHXC305F2_TwinParseFailureAsymmetry(t *testing.T) {
	goodYAML := []byte(`name: parse-asym-bank
test_cases:
  - id: F2-CASE-001
    name: "a perfectly ordinary case"
    platforms: [api]
    steps:
      - name: "do it"
        action: "GET /x prose"
        expected: "ok"
`)
	goodJSON := []byte(`{
  "name": "parse-asym-bank",
  "test_cases": [
    {
      "id": "F2-CASE-001",
      "name": "a perfectly ordinary case",
      "platforms": ["api"],
      "steps": [
        {"name": "do it", "action": "http: GET /x", "expected": "ok", "expect_status": 200}
      ]
    }
  ]
}`)
	// Truncated mid-structure: valid-looking prefix, unparseable whole.
	brokenYAML := []byte("name: parse-asym-bank\ntest_cases:\n  - id: F2-CASE-001\n    name: \"x\n    steps: [[[\n")
	brokenJSON := []byte(`{"name": "parse-asym-bank", "test_cases": [{"id": `)

	t.Run("unparseable_non_json_twin_hard_fails_the_scan", func(t *testing.T) {
		// The fixture carries TWO base names on purpose. Round 16: with
		// only `parse-asym.*` present, nothing has been declined by the
		// time the scan aborts, so `res.Declined` is empty whether the
		// abort carries the list or discards it — the assertion below
		// would hold on both the fixed and the broken loader. The
		// lexically-earlier `a-decl.*` pair is resolved (and one side
		// declined) BEFORE the broken pair aborts, which is what makes
		// the discard observable at all.
		newFixture := func(t *testing.T, withBrokenPair bool) string {
			t.Helper()
			dir := t.TempDir()
			hxc305Write(t, filepath.Join(dir, "a-decl.yaml"), hxc305YAMLBank("a-decl", "F2-DECL-001"))
			hxc305Write(t, filepath.Join(dir, "a-decl.json"), hxc305JSONBank("a-decl", "F2-DECL-001"))
			if withBrokenPair {
				hxc305Write(t, filepath.Join(dir, "parse-asym.yaml"), string(brokenYAML))
				hxc305Write(t, filepath.Join(dir, "parse-asym.json"), string(goodJSON))
			}
			return dir
		}

		// POSITIVE CONTROL: without the broken pair the fixture must
		// load and report exactly the twin decline. Without it, an
		// empty declined list on the aborting run could not be told
		// apart from a fixture that declines nothing at all.
		ctrl, ctrlErr := LoadDirVerbose(newFixture(t, false))
		if ctrlErr != nil {
			t.Fatalf("positive control FAILED to load: %v — without it this "+
				"subtest measures nothing", ctrlErr)
		}
		if len(ctrl.Declined) != 1 {
			t.Fatalf("positive control: want exactly 1 declined twin, got %d (%+v)",
				len(ctrl.Declined), ctrl.Declined)
		}

		res, err := LoadDirVerbose(newFixture(t, true))
		if err == nil {
			t.Fatalf("LoadDirVerbose returned no error for an UNPARSEABLE non-JSON "+
				"twin — a corrupt or truncated YAML bank must never resolve quietly "+
				"to its JSON sibling, because every case the JSON form never "+
				"converted would vanish with no error at all (HXC-305 F2). Loaded: %+v",
				res)
		}
		if !strings.Contains(err.Error(), "parse-asym.yaml") {
			t.Errorf("error %q does not name the unparseable file", err)
		}

		// The abort must carry the declined list, like every other
		// abort in loadDirVerbose. Until round 16 this path returned a
		// bare nil, so a corrupt twin made the CLI silent about every
		// file already declined — measured on the real corpus as 50
		// declined lines collapsing to 0.
		if res == nil {
			t.Fatal("LoadDirVerbose returned a NIL result alongside the " +
				"unparseable-twin error, discarding every file already declined")
		}
		var sawEarlier bool
		for _, d := range res.Declined {
			if strings.HasSuffix(d.Path, "a-decl.json") {
				sawEarlier = true
			}
		}
		if !sawEarlier {
			t.Fatalf("the twin declined BEFORE the abort (a-decl.json) is absent "+
				"from the list returned alongside it. Declined entries were: %+v",
				res.Declined)
		}
	})

	t.Run("unparseable_json_twin_keeps_non_json_and_reports", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "parse-asym.yaml"), goodYAML, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "parse-asym.json"), brokenJSON, 0o644); err != nil {
			t.Fatal(err)
		}

		res, err := LoadDirVerbose(dir)
		if err != nil {
			t.Fatalf("LoadDirVerbose FAILED on an unparseable JSON twin: %v — this "+
				"directory loaded fine before the twin fix (the JSON sibling was "+
				"never parsed at all), so its failure must not newly abort the scan",
				err)
		}
		if len(res.Banks) != 1 || len(res.Banks[0].TestCases) != 1 {
			t.Fatalf("want exactly 1 bank with 1 case from the intact YAML twin, got %+v",
				res.Banks)
		}
		if got := res.Banks[0].TestCases[0].ID; got != "F2-CASE-001" {
			t.Errorf("loaded case id %q, want F2-CASE-001", got)
		}
		// Reported, never swallowed.
		if len(res.Declined) != 1 {
			t.Fatalf("want exactly 1 declined entry for the broken JSON twin, got %+v",
				res.Declined)
		}
		d := res.Declined[0]
		if !strings.HasSuffix(d.Path, "parse-asym.json") {
			t.Errorf("declined path = %q, want the JSON twin", d.Path)
		}
		if !strings.Contains(d.Reason, "failed to parse") {
			t.Errorf("declined reason %q does not say the twin failed to parse", d.Reason)
		}
	})
}

// caseSkipMarkers reports how many of a case's steps carry an explicit
// honest-skip marker (§11.4.3 / §11.5: `_skip: true` plus the
// `_skip_reason` that justifies it). Deliberately counts a marker as
// present ONLY when the reason is non-empty too — a bare `_skip: true`
// with no justification is exactly the silent skip the covenant
// forbids, so it must not be able to satisfy a guard about honest
// skips surviving.
func caseSkipMarkers(tc *TestCase) int {
	n := 0
	for i := range tc.Steps {
		if tc.Steps[i].Skip && strings.TrimSpace(tc.Steps[i].SkipReason) != "" {
			n++
		}
	}
	return n
}

// TestHXC305F4_TiedSharedCaseKeepsPrimaryBodyAndItsSkipMarkers is the
// standing regression guard for HXC-305 finding "F4": the TIE arm of
// mergeTwinCases' per-case comparison.
//
// mergeTwinCases resolves a case id shared by both twins by comparing
// the two BODIES' executable-step counts, and keeps the primary's body
// unless the secondary's is STRICTLY better. Every guard written for
// findings F1/B1/B3 before this one compares counts in the `got < want`
// direction — "never load a body worse than one an available twin
// offers" — which a TIE leaves completely unmoved: when the counts are
// equal, taking either body satisfies every one of them. The tie rule
// was therefore load-bearing but entirely unguarded, and relaxing the
// comparison by a single character (`>` to `>=`) passed the whole
// pkg/testbank package.
//
// It is not a cosmetic distinction. Measured against the real banks/
// corpus at the time this guard was written, of 1897 case ids shared
// by both twins of a pair, 1210 are TIES on executable-step count and
// only 687 are strict wins — so the tie rule decides the body of the
// MAJORITY of shared cases. Flipping it to `>=` swaps 104 case bodies
// (the tied cases whose two bodies actually differ) and, critically,
// drops 204 honest-skip step markers across 77 cases — every `_skip:
// true` / `_skip_reason` pair in AICHAT-001..005, AICHAT-SEC-001 and
// their siblings — because the tie's secondary body carries the same
// executable steps WITHOUT the justification that says they must not
// run. Those markers are the §11.4.3 honest-skip record; losing them
// silently converts a reasoned skip into an unreasoned execution, and
// the tie rule is the only thing preserving them.
//
// This fixture is the minimal hermetic reproduction of that shape. The
// JSON twin wins the BANK-level comparison 3-to-1 and so becomes the
// primary; the shared case TIE-SKIP-001 has exactly ONE executable
// step in EACH twin — a tie — but only the primary's body carries the
// honest-skip marker. A correct loader keeps the primary's body
// verbatim, markers intact.
func TestHXC305F4_TiedSharedCaseKeepsPrimaryBodyAndItsSkipMarkers(t *testing.T) {
	dir := t.TempDir()
	// YAML twin — bank total 1 executable step. Its TIE-SKIP-001 body
	// has the SAME executable-step count as the JSON twin's (1), so
	// the per-case comparison is a genuine tie, but it carries NO
	// honest-skip marker and names its step differently, so "which
	// body was kept" is directly observable.
	yamlContent := []byte(`name: tie-pref-bank
test_cases:
  - id: TIE-SKIP-001
    name: "shared case whose two bodies tie on executable-step count"
    platforms: [android]
    steps:
      - name: "run the device script (unjustified variant)"
        action: "adb_shell: sh /data/local/tmp/run.sh"
        expected: "exit 0"
  - id: TIE-FILLER
    name: "case that only the JSON twin makes executable"
    platforms: [api]
    steps:
      - name: "call endpoint a"
        action: "GET /a described in prose"
        expected: "200"
      - name: "call endpoint b"
        action: "GET /b described in prose"
        expected: "200"
`)
	// JSON twin — bank total 3 executable steps, so it WINS the
	// bank-level comparison and becomes the primary. Its TIE-SKIP-001
	// body ties on executable-step count (1) but carries the honest
	// skip marker + reason.
	jsonContent := []byte(`{
  "name": "tie-pref-bank",
  "test_cases": [
    {
      "id": "TIE-SKIP-001",
      "name": "shared case whose two bodies tie on executable-step count",
      "platforms": ["android"],
      "steps": [
        {
          "name": "run the device script (justified variant)",
          "action": "adb_shell: sh /data/local/tmp/run.sh",
          "expected": "exit 0",
          "_skip": true,
          "_skip_reason": "no device attached in this topology; running it would report a confusing FAIL"
        }
      ]
    },
    {
      "id": "TIE-FILLER",
      "name": "case that only the JSON twin makes executable",
      "platforms": ["api"],
      "steps": [
        {"name": "call endpoint a", "action": "http: GET /a", "expected": "200", "expect_status": 200},
        {"name": "call endpoint b", "action": "http: GET /b", "expected": "200", "expect_status": 200}
      ]
    }
  ]
}`)
	if err := os.WriteFile(filepath.Join(dir, "tie-pref.yaml"), yamlContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tie-pref.json"), jsonContent, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := LoadDirVerbose(dir)
	if err != nil {
		t.Fatalf("LoadDirVerbose: %v", err)
	}
	if len(res.Banks) != 1 {
		t.Fatalf("loaded %d banks, want exactly 1 (merged twin pair)", len(res.Banks))
	}
	var tie *TestCase
	for i := range res.Banks[0].TestCases {
		if res.Banks[0].TestCases[i].ID == "TIE-SKIP-001" {
			tie = &res.Banks[0].TestCases[i]
		}
	}
	if tie == nil {
		t.Fatal("TIE-SKIP-001 missing entirely")
	}
	if len(tie.Steps) != 1 {
		t.Fatalf("TIE-SKIP-001 loaded with %d step(s), want 1", len(tie.Steps))
	}
	// Precondition: this fixture is only meaningful while the two
	// bodies genuinely TIE. If a future change to ParseAction made one
	// side count differently, the tie arm would stop being exercised
	// and this guard would silently become a strict-win guard.
	if n := caseExecCount(tie); n != 1 {
		t.Fatalf("TIE-SKIP-001 has %d executable step(s), want 1 — fixture no "+
			"longer models a tie, so this guard would be vacuous", n)
	}

	if redModeOn() {
		// RED_MODE=1 reproduces the defect this guard exists to catch,
		// as it presents on a loader whose tie arm has been relaxed to
		// `secondaryExec >= primaryExec`: the secondary's body wins the
		// tie and the honest-skip marker disappears. Captured against
		// that mutated loader as the §11.4.115 RED baseline; it is a
		// baseline capture, NOT a standing assertion, so it is expected
		// to fail against the fixed loader.
		if tie.Steps[0].Skip {
			t.Errorf("RED_MODE=1: TIE-SKIP-001 step still carries _skip — want the "+
				"tie resolved to the secondary's unjustified body (the defect: a "+
				"tie-break relaxed to >= silently drops the honest-skip marker). "+
				"step=%q", tie.Steps[0].Name)
		}
		return
	}

	// GREEN: the standing guard.
	if !tie.Steps[0].Skip || strings.TrimSpace(tie.Steps[0].SkipReason) == "" {
		t.Errorf("TIE-SKIP-001 lost its honest-skip marker: _skip=%v _skip_reason=%q "+
			"(step %q). Both twins offer the same executable-step count for this "+
			"case, so the tie MUST keep the primary's body — the only body carrying "+
			"the §11.4.3 justification that this step must not run. A tie-break "+
			"relaxed to >= takes the secondary's body instead and converts a "+
			"reasoned skip into an unreasoned execution (HXC-305 F4 regression)",
			tie.Steps[0].Skip, tie.Steps[0].SkipReason, tie.Steps[0].Name)
	}
	if n := caseSkipMarkers(tie); n != 1 {
		t.Errorf("TIE-SKIP-001 carries %d justified skip marker(s), want 1", n)
	}
	// The whole body — not just the marker — must come from the
	// primary, so the guard also catches a tie resolved the wrong way
	// in cases that carry no skip marker at all.
	if got, want := tie.Steps[0].Name, "run the device script (justified variant)"; got != want {
		t.Errorf("TIE-SKIP-001 step name = %q, want %q — the tie resolved to the "+
			"SECONDARY twin's body; ties must keep the primary's", got, want)
	}

	// A tie must never be REPORTED as a strict win. The decline reason
	// asserts, verbatim, that an overridden case's body has "strictly
	// more executable step(s)" — a statement that is simply false for a
	// tied case, and one no existing test constrained. Under `>=` the
	// real corpus emits 50 such lines where only 1 is true.
	for _, d := range res.Declined {
		if strings.Contains(d.Reason, "TIE-SKIP-001") &&
			strings.Contains(d.Reason, "strictly more executable") {
			t.Errorf("declined reason claims TIE-SKIP-001 was kept from the declined "+
				"twin for having \"strictly more executable step(s)\", but the two "+
				"bodies TIE at 1 each — the report states as fact something that is "+
				"false: %q", d.Reason)
		}
	}
}

// TestHXC305M7_PerCaseOverrideTakesTheWHOLESecondaryCase closes the
// "M7" latent gap: when the secondary's body wins a shared case,
// mergeTwinCases replaces the WHOLE case (`tc = *sec`), not just its
// steps. Nothing asserted that. Narrowing it to `tc.Steps = sec.Steps`
// — keeping the primary's name, tags, description and metadata while
// splicing in the secondary's steps — passed the entire pkg/testbank
// package, because every existing guard reasons about executable-step
// COUNTS, which that mutation leaves identical by construction.
//
// The result would be a case that exists in NEITHER twin: a chimera
// carrying one twin's steps under the other twin's description and
// tags. It has no effect on the current corpus (the three real
// secondary-win cases happen to carry matching metadata on both
// sides), which is exactly why it needs a hermetic guard — a latent
// seam with zero present-day symptom is the one that regresses
// unnoticed.
func TestHXC305M7_PerCaseOverrideTakesTheWHOLESecondaryCase(t *testing.T) {
	dir := t.TempDir()
	// The YAML twin holds the strictly-better body for M7-CASE (1
	// executable step vs 0) AND distinctive metadata the JSON twin
	// does not carry. The JSON twin wins the BANK comparison via
	// M7-FILLER, so YAML is the secondary and M7-CASE is a per-case
	// override — the exact path `tc = *sec` takes.
	yamlContent := []byte(`name: whole-case-bank
test_cases:
  - id: M7-CASE
    name: "name that lives ONLY in the secondary twin"
    description: "description that lives ONLY in the secondary twin"
    platforms: [android]
    tags: [secondary-only-tag]
    steps:
      - name: "run the device script"
        action: "adb_shell: sh /data/local/tmp/run.sh"
        expected: "exit 0"
  - id: M7-FILLER
    name: "filler"
    platforms: [api]
    steps:
      - name: "call endpoint a"
        action: "GET /a described in prose"
        expected: "200"
      - name: "call endpoint b"
        action: "GET /b described in prose"
        expected: "200"
`)
	jsonContent := []byte(`{
  "name": "whole-case-bank",
  "test_cases": [
    {
      "id": "M7-CASE",
      "name": "name from the PRIMARY twin",
      "description": "description from the PRIMARY twin",
      "platforms": ["android"],
      "tags": ["primary-only-tag"],
      "steps": [
        {"name": "run the device script", "action": "Execute run.sh on the device", "expected": "exit 0"}
      ]
    },
    {
      "id": "M7-FILLER",
      "name": "filler",
      "platforms": ["api"],
      "steps": [
        {"name": "call endpoint a", "action": "http: GET /a", "expected": "200", "expect_status": 200},
        {"name": "call endpoint b", "action": "http: GET /b", "expected": "200", "expect_status": 200}
      ]
    }
  ]
}`)
	if err := os.WriteFile(filepath.Join(dir, "whole-case.yaml"), yamlContent, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "whole-case.json"), jsonContent, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := LoadDirVerbose(dir)
	if err != nil {
		t.Fatalf("LoadDirVerbose: %v", err)
	}
	var got *TestCase
	for _, bf := range res.Banks {
		for i := range bf.TestCases {
			if bf.TestCases[i].ID == "M7-CASE" {
				got = &bf.TestCases[i]
			}
		}
	}
	if got == nil {
		t.Fatal("M7-CASE missing entirely")
	}
	// Precondition: the override must actually have fired, or the
	// guard proves nothing.
	if n := caseExecCount(got); n != 1 {
		t.Fatalf("M7-CASE loaded with %d executable step(s), want 1 — the per-case "+
			"override did not fire, so this guard is vacuous", n)
	}

	// The whole case must come from the secondary — steps AND the
	// metadata that travels with them.
	if got.Name != "name that lives ONLY in the secondary twin" {
		t.Errorf("M7-CASE name = %q, want the SECONDARY twin's name — the per-case "+
			"override must replace the whole case, not splice the secondary's steps "+
			"under the primary's metadata (that composite exists in neither twin)",
			got.Name)
	}
	if got.Description != "description that lives ONLY in the secondary twin" {
		t.Errorf("M7-CASE description = %q, want the SECONDARY twin's", got.Description)
	}
	hasSecondaryTag := false
	for _, tg := range got.Tags {
		if tg == "secondary-only-tag" {
			hasSecondaryTag = true
		}
		if tg == "primary-only-tag" {
			t.Errorf("M7-CASE carries the PRIMARY twin's tag %q alongside the "+
				"secondary's steps — a chimera case present in neither twin", tg)
		}
	}
	if !hasSecondaryTag {
		t.Errorf("M7-CASE tags = %v, want the secondary's [secondary-only-tag]", got.Tags)
	}
}

// TestHXC305F4_RealCorpusTiedSharedCasesKeepPrimaryBody is the
// general, corpus-wide standing guard for finding "F4". Where the
// hermetic fixture above pins one constructed tie, this walks EVERY
// twin pair in the real banks/ directory and asserts the tie rule
// directly: for every case id both twins share whose two bodies have
// EQUAL executable-step counts, the body LoadDirVerbose actually
// loaded must be the PRIMARY's, verbatim.
//
// It does not recompute which twin is primary. It reads that decision
// out of the loader's own Declined report — the declined path IS the
// secondary — so the guard cannot drift out of agreement with the
// production rule it is checking, and a change to the primary-choosing
// comparison is reflected here automatically rather than needing this
// test edited in lockstep (which is how a guard quietly stops guarding).
//
// The `got < want` guards for F1/B1 cannot cover this: a tie changes
// neither side of that inequality.
func TestHXC305F4_RealCorpusTiedSharedCasesKeepPrimaryBody(t *testing.T) {
	dir := filepath.Join("..", "..", "banks")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("SKIP-OK: banks/ dir not present at %s: %v", dir, err)
	}
	res, err := LoadDirVerbose(dir)
	if err != nil {
		t.Fatalf("LoadDirVerbose: %v", err)
	}
	loaded := map[string]*TestCase{}
	for _, bf := range res.Banks {
		for i := range bf.TestCases {
			tc := &bf.TestCases[i]
			if tc.ID != "" {
				loaded[tc.ID] = tc
			}
		}
	}
	// Ids legitimately absent — excluded as genuine cross-bank
	// duplicates (HXC-305 B3) — are not this guard's business.
	excluded := map[string]bool{}
	for _, d := range res.Declined {
		for _, id := range d.ExcludedIDs {
			excluded[id] = true
		}
	}

	var mismatched []string
	ties, pairs, skipMarkers := 0, 0, 0
	for _, d := range res.Declined {
		// Only twin-preference declines carry a primary; a zero-case or
		// unparseable file decline does not.
		if !strings.Contains(d.Reason, "superseded by ") {
			continue
		}
		secondaryPath := d.Path
		// The primary is the sibling sharing this base name.
		base := strings.TrimSuffix(secondaryPath, filepath.Ext(secondaryPath))
		var primaryPath string
		for _, ext := range []string{".json", ".yaml", ".yml"} {
			cand := base + ext
			if cand == secondaryPath {
				continue
			}
			if _, err := os.Stat(cand); err == nil {
				primaryPath = cand
				break
			}
		}
		if primaryPath == "" {
			t.Errorf("declined twin %s has no surviving sibling on disk — cannot "+
				"identify the primary (reason: %q)", secondaryPath, d.Reason)
			continue
		}
		if !strings.Contains(d.Reason, primaryPath) {
			t.Errorf("declined twin %s names a primary in its reason that is not its "+
				"sibling %s: %q", secondaryPath, primaryPath, d.Reason)
			continue
		}
		primaryBF, err := LoadFile(primaryPath)
		if err != nil {
			t.Fatalf("independently loading primary %s: %v", primaryPath, err)
		}
		secondaryBF, err := LoadFile(secondaryPath)
		if err != nil {
			t.Fatalf("independently loading secondary %s: %v", secondaryPath, err)
		}
		pairs++
		secByID := map[string]*TestCase{}
		for i := range secondaryBF.TestCases {
			if id := secondaryBF.TestCases[i].ID; id != "" {
				secByID[id] = &secondaryBF.TestCases[i]
			}
		}
		for i := range primaryBF.TestCases {
			p := &primaryBF.TestCases[i]
			if p.ID == "" || excluded[p.ID] {
				continue
			}
			s, shared := secByID[p.ID]
			if !shared {
				continue
			}
			if caseExecCount(p) != caseExecCount(s) {
				continue // a strict win — the F1 guards' business
			}
			ties++
			skipMarkers += caseSkipMarkers(p)
			got, ok := loaded[p.ID]
			if !ok {
				continue // absence is the B1/B3 guards' business
			}
			if !reflect.DeepEqual(*got, *p) {
				lostSkips := caseSkipMarkers(p) - caseSkipMarkers(got)
				detail := ""
				if lostSkips > 0 {
					detail = fmt.Sprintf(" and lost %d honest-skip marker(s)", lostSkips)
				}
				mismatched = append(mismatched, fmt.Sprintf(
					"%s (primary %s, secondary %s): both bodies have %d executable "+
						"step(s) — a tie — but the loaded body is not the primary's%s",
					p.ID, filepath.Base(primaryPath), filepath.Base(secondaryPath),
					caseExecCount(p), detail))
			}
		}
	}

	if pairs == 0 || ties == 0 {
		t.Fatalf("walked %d twin pair(s) and found %d tied shared case(s) — this "+
			"guard would be vacuous; the corpus or the decline reporting changed "+
			"shape", pairs, ties)
	}
	if skipMarkers == 0 {
		t.Errorf("found %d tied shared case(s) across %d twin pair(s) but NOT ONE "+
			"carries an honest-skip marker — the corpus lost the very property this "+
			"guard was written to protect, or the marker fields stopped parsing",
			ties, pairs)
	}
	if len(mismatched) > 0 {
		sort.Strings(mismatched)
		t.Fatalf("%d shared case(s) whose two twin bodies TIE on executable-step "+
			"count did not load the primary's body — a tie must keep the primary "+
			"(HXC-305 F4 regression; relaxing the comparison to >= swaps 104 bodies "+
			"and drops 204 honest-skip markers across 77 cases):\n  %s",
			len(mismatched), strings.Join(mismatched, "\n  "))
	}
	t.Logf("checked %d tied shared case(s) across %d twin pair(s), carrying %d "+
		"honest-skip marker(s)", ties, pairs, skipMarkers)
}

// TestHXC305M1_ExecutableStepScalarRemainsSafeAndItsLossesArePinned
// records a DELIBERATE, evidence-backed decision about the scalar the
// twin preference is built on, and guards the precondition that makes
// it safe (HXC-305 finding "M1").
//
// THE CONCERN. The preference ranks bodies by a single scalar — how
// many steps parse as a genuinely-executable action — which says
// nothing about whether those steps ASSERT anything. A body of four
// bare `sleep:` steps with zero assertions therefore outranks a body
// of three steps each carrying `expected` + an explicit assertion, and
// the loader would keep the assertion-free one. That is demonstrable
// hermetically (see the sub-test below), and it is the honest weakness
// of a scalar heuristic.
//
// THE DECISION. The scalar is KEPT rather than replaced with an
// assertion-weighted rank, because the failure is not reachable in
// this corpus and the replacement is not obviously better: weighting
// assertions would change the resolution of all 687 strict-win cases
// on a judgment call, to fix zero observed defects. Instead the
// PRECONDITION that makes the scalar safe is asserted directly, so the
// decision expires automatically the moment it stops holding: in every
// strict-win pair the LOSING body has ZERO executable steps, meaning
// the choice is always "something executable" vs "pure prose", never
// "more but worse" vs "fewer but better". While that holds, no ranking
// among executable bodies can pick a worse one, because there is never
// more than one executable body to choose between.
//
// If a future bank ever offers two genuinely executable bodies for one
// case id with different counts, this guard fails and the scalar must
// be re-decided on that evidence rather than by assumption (§11.4.6).
//
// The metadata the winning body drops is pinned by name for the same
// reason — so the losses cannot silently grow.
//
// SCOPE LIMIT of this pin (§11.4.6, recorded round 8). The loop below
// RE-DERIVES which body the loader keeps — it mirrors mergeTwinCases'
// rule locally (`kept := p; if caseExecCount(s) > caseExecCount(p)`)
// rather than OBSERVING what LoadDirVerbose actually returned in
// res.Banks. A mutation of the tie arm in mergeTwinCases itself
// (`>` -> `>=`) therefore changes the loader and this pin identically,
// and the pin still exits 0: it cannot detect that class. That is a
// scope limit, not a coverage hole — the tie rule is guarded by the F4
// hermetic and F4 real-corpus tests, and both were confirmed to FAIL
// under that mutation. Anyone tightening this pin should switch it to
// observing res.Banks rather than re-deriving the rule.
func TestHXC305M1_ExecutableStepScalarRemainsSafeAndItsLossesArePinned(t *testing.T) {
	t.Run("scalar_can_prefer_an_assertion_free_body_hermetically", func(t *testing.T) {
		// The honest demonstration of the weakness. This is NOT a
		// defect assertion — it pins the KNOWN, accepted behaviour of
		// the scalar so that anyone changing the rank function sees
		// exactly what the current one does.
		dir := t.TempDir()
		// 4 executable steps, zero assertions.
		yamlContent := []byte(`name: scalar-bank
test_cases:
  - id: M1-CASE
    name: "assertion-free but numerous"
    platforms: [android]
    steps:
      - {name: "w1", action: "sleep: 100", expected: ""}
      - {name: "w2", action: "sleep: 100", expected: ""}
      - {name: "w3", action: "sleep: 100", expected: ""}
      - {name: "w4", action: "sleep: 100", expected: ""}
`)
		// 3 executable steps, every one asserted.
		jsonContent := []byte(`{
  "name": "scalar-bank",
  "test_cases": [
    {
      "id": "M1-CASE",
      "name": "fewer but fully asserted",
      "platforms": ["api"],
      "steps": [
        {"name": "a", "action": "http: GET /a", "expected": "200 ok", "expect_status": 200},
        {"name": "b", "action": "http: GET /b", "expected": "200 ok", "expect_status": 200},
        {"name": "c", "action": "http: GET /c", "expected": "200 ok", "expect_status": 200}
      ]
    }
  ]
}`)
		if err := os.WriteFile(filepath.Join(dir, "scalar.yaml"), yamlContent, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "scalar.json"), jsonContent, 0o644); err != nil {
			t.Fatal(err)
		}
		res, err := LoadDirVerbose(dir)
		if err != nil {
			t.Fatalf("LoadDirVerbose: %v", err)
		}
		var got *TestCase
		for _, bf := range res.Banks {
			for i := range bf.TestCases {
				if bf.TestCases[i].ID == "M1-CASE" {
					got = &bf.TestCases[i]
				}
			}
		}
		if got == nil {
			t.Fatal("M1-CASE missing entirely")
		}
		asserted := 0
		for i := range got.Steps {
			if got.Steps[i].ExpectStatus != 0 || got.Steps[i].ExpectJSONPath != "" ||
				got.Steps[i].ExpectBodyContains != "" {
				asserted++
			}
		}
		if len(got.Steps) != 4 || asserted != 0 {
			t.Fatalf("the scalar's known behaviour changed: M1-CASE loaded %d step(s) "+
				"with %d assertion(s); this fixture pins the CURRENT rank function "+
				"(4 bare executable steps outrank 3 asserted ones). If the rank was "+
				"deliberately changed to weight assertions, update this pin and "+
				"re-check the 687 strict-win cases it also re-decides",
				len(got.Steps), asserted)
		}
	})

	t.Run("strict_win_losers_are_always_pure_prose", func(t *testing.T) {
		dir := filepath.Join("..", "..", "banks")
		if _, err := os.Stat(dir); err != nil {
			t.Skipf("SKIP-OK: banks/ dir not present at %s: %v", dir, err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read banks dir: %v", err)
		}
		byBase := map[string]map[string]string{}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext != ".yaml" && ext != ".yml" && ext != ".json" {
				continue
			}
			b := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			if byBase[b] == nil {
				byBase[b] = map[string]string{}
			}
			byBase[b][ext] = e.Name()
		}

		var contested []string
		strict := 0
		for _, m := range byBase {
			jsonName, nonJSON := m[".json"], m[".yaml"]
			if nonJSON == "" {
				nonJSON = m[".yml"]
			}
			if jsonName == "" || nonJSON == "" {
				continue
			}
			jb, err1 := LoadFile(filepath.Join(dir, jsonName))
			nb, err2 := LoadFile(filepath.Join(dir, nonJSON))
			if err1 != nil || err2 != nil {
				continue
			}
			byID := map[string]*TestCase{}
			for i := range nb.TestCases {
				if id := nb.TestCases[i].ID; id != "" {
					byID[id] = &nb.TestCases[i]
				}
			}
			for i := range jb.TestCases {
				a := &jb.TestCases[i]
				b, shared := byID[a.ID]
				if !shared {
					continue
				}
				ae, be := caseExecCount(a), caseExecCount(b)
				if ae == be {
					continue
				}
				strict++
				loser := be
				if be > ae {
					loser = ae
				}
				if loser != 0 {
					contested = append(contested, fmt.Sprintf(
						"%s: %d vs %d executable step(s) — BOTH bodies executable",
						a.ID, ae, be))
				}
			}
		}
		if strict == 0 {
			t.Fatal("found 0 strict-win shared cases — this guard would be vacuous")
		}
		if len(contested) > 0 {
			sort.Strings(contested)
			t.Fatalf("%d shared case(s) now offer TWO genuinely executable bodies with "+
				"different step counts, so the executable-step scalar is now choosing "+
				"between them on count alone — the precondition that made a bare count "+
				"safe (the loser is always pure prose) no longer holds, and the rank "+
				"function must be re-decided on this evidence (HXC-305 M1):\n  %s",
				len(contested), strings.Join(contested, "\n  "))
		}
		t.Logf("%d strict-win shared case(s); every losing body has 0 executable steps", strict)
	})

	t.Run("pending_runtime_steps_never_outrank_genuinely_runnable_ones", func(t *testing.T) {
		// The scalar's second weakness, and unlike the first this one
		// IS reachable — it is currently costing the catalog one real
		// executable step.
		//
		// ActionTypePlaywright parses as "executable" and counts toward
		// the rank, but schema.go is explicit that such steps currently
		// SKIP with PLAYWRIGHT-RUNTIME-PENDING rather than run: the
		// runtime is not wired in yet. So the rank credits a body for
		// strength it does not have, and a body that cannot run a
		// single step today can tie — or beat — one that can.
		//
		// Most of that inflation is harmless and in fact desirable. Of
		// the 212 shared cases whose comparison changes when playwright
		// steps are excluded, 211 are a structured playwright body
		// beating a body with ZERO runnable steps (pure prose) — which
		// is exactly what HXC-305 wants: prose will NEVER run, a
		// playwright step will once the runtime lands.
		//
		// The 212th is a genuine defect, and it is the HXC-305 defect
		// class itself (a silently-preferred twin hiding an executable
		// step) surviving in the shipped catalog. It is invisible to
		// every other guard: F1's "never load a worse body" comparison
		// counts playwright as executable, so the two bodies look
		// EQUAL, and the F4 tie guard above then correctly keeps the
		// primary — the primary just happens to be the body that
		// cannot run.
		//
		// This guard therefore asserts the invariant that actually
		// matters — a body whose executable steps are all pending must
		// never outrank one with genuinely runnable steps — with the
		// single known instance pinned by name. The pin keeps the suite
		// honest rather than green-by-omission: any NEW instance fails
		// immediately, and deleting the pin is the acceptance test for
		// the fix.
		//
		// The fix is NOT applied here, deliberately. Ranking bodies
		// two-tier (genuinely-runnable steps first, pending steps only
		// as a tiebreak) resolves DC-001 correctly and leaves all 211
		// benign flips intact — but it does so by flipping the
		// BANK-LEVEL primary of atmosphere.{json,yaml}, the corpus's
		// largest bank (measured: atmosphere.yaml 24 runnable vs
		// atmosphere.json 15 runnable + 34 pending), which re-attributes
		// that bank's case order, name, version and metadata. That is a
		// corpus-wide change deserving its own reviewed commit and its
		// own before/after catalog diff, not a tail-end edit to a
		// finding about a single case.
		dir := filepath.Join("..", "..", "banks")
		if _, err := os.Stat(dir); err != nil {
			t.Skipf("SKIP-OK: banks/ dir not present at %s: %v", dir, err)
		}
		// Pair the twins from the loader's OWN report of what it
		// declined, never from a directory listing plus an assumption
		// about which side wins.
		//
		// This guard used to walk the directory and treat the .json
		// side as the primary unconditionally, which silently limited
		// it to the JSON-primary pairs. Measured on this corpus: 32
		// of the 50 twin pairs are YAML-primary and hold 552 of the
		// 1897 shared ids, so 29% of the population the guard claimed
		// to cover was never examined under the correct rule — a
		// planted DC-001-class defect in all-formats.{json,yaml} (a
		// YAML-primary pair) passed the whole suite (HXC-305 round-6
		// BLOCKING C).
		//
		// Declined names the loser and its Reason names the winner,
		// so the pairing is an OBSERVED loader output rather than a
		// re-derivation of the ranking — the same non-circular source
		// the sibling corpus guard above already uses.
		res, err := LoadDirVerbose(dir)
		if err != nil {
			t.Fatalf("LoadDirVerbose(%s): %v", dir, err)
		}
		type twinPair struct{ primary, secondary string }
		var pairs []twinPair
		for _, d := range res.Declined {
			const marker = "superseded by "
			at := strings.Index(d.Reason, marker)
			if at < 0 {
				// Not a twin-preference decline (zero-case, unparseable,
				// or an exclusion report) — it has no primary.
				continue
			}
			primary := d.Reason[at+len(marker):]
			if end := strings.Index(primary, " ("); end >= 0 {
				primary = primary[:end]
			}
			primary = strings.TrimSpace(primary)
			if primary == "" {
				t.Errorf("declined twin %s names no primary in its reason: %q",
					d.Path, d.Reason)
				continue
			}
			pairs = append(pairs, twinPair{primary: primary, secondary: d.Path})
		}
		if len(pairs) == 0 {
			t.Fatal("derived 0 twin pairs from the declined list — this guard " +
				"would be vacuous")
		}
		// The single KNOWN instance, pinned with its evidence.
		// atmosphere.{json,yaml} case DC-001 "Widevine L3 DRM
		// functional": the YAML twin runs
		//   adb_shell: sh /data/local/tmp/tests/test_widevine_drm.sh
		// while the JSON twin de-converts that step back to prose and
		// contributes instead
		//   playwright: assertVisible text=Widevine L3 security level
		// which cannot run. Both count 1 "executable" step, so the
		// comparison ties and the JSON primary's body is kept — the
		// runnable device-script invocation is dropped.
		knownPending := map[string]string{
			"DC-001": "atmosphere.{json,yaml}: JSON twin contributes a " +
				"PLAYWRIGHT-RUNTIME-PENDING step where the YAML twin has a runnable " +
				"adb_shell: test-script invocation; the counts tie at 1 so the " +
				"unrunnable body wins",
		}
		// runnable counts steps that execute TODAY; pending counts
		// executable-shaped steps waiting on an unwired runtime.
		rank := func(tc *TestCase) (runnable, pending int) {
			for i := range tc.Steps {
				at, _ := tc.Steps[i].ParseAction()
				switch {
				case at == ActionTypePlaywright:
					pending++
				case at != ActionTypeDescription:
					runnable++
				}
			}
			return runnable, pending
		}
		var regressions []string
		shared, pinned := 0, 0
		for _, tp := range pairs {
			pb, err1 := LoadFile(tp.primary)
			sb, err2 := LoadFile(tp.secondary)
			if err1 != nil || err2 != nil {
				continue
			}
			secByID := map[string]*TestCase{}
			for i := range sb.TestCases {
				if id := sb.TestCases[i].ID; id != "" {
					secByID[id] = &sb.TestCases[i]
				}
			}
			for i := range pb.TestCases {
				p := &pb.TestCases[i]
				s, ok := secByID[p.ID]
				if !ok {
					continue
				}
				shared++
				pRun, _ := rank(p)
				sRun, _ := rank(s)
				// The best runnable body either twin offers.
				bestRun := pRun
				if sRun > bestRun {
					bestRun = sRun
				}
				if bestRun == 0 {
					// Neither twin runs anything today; a playwright
					// body beating prose here is the desirable case.
					continue
				}
				// Which body the loader actually keeps, per
				// mergeTwinCases: the PRIMARY's, unless the secondary
				// has STRICTLY more executable steps. Keying this off
				// the real primary — rather than assuming JSON — is
				// the whole point of the Declined-derived pairing
				// above.
				kept := p
				if caseExecCount(s) > caseExecCount(p) {
					kept = s
				}
				keptRun, keptPending := rank(kept)
				if keptRun >= bestRun {
					continue
				}
				if _, known := knownPending[p.ID]; known {
					pinned++
					continue
				}
				regressions = append(regressions, fmt.Sprintf(
					"%s (primary %s, secondary %s): kept a body with %d runnable + "+
						"%d pending step(s) while a twin offers %d runnable",
					p.ID, tp.primary, tp.secondary, keptRun, keptPending, bestRun))
			}
		}
		if shared == 0 {
			t.Fatal("found 0 shared cases — this guard would be vacuous")
		}
		if pinned != len(knownPending) {
			t.Errorf("pinned %d known pending-runtime demotion(s), expected %d — if a "+
				"pin no longer reproduces it has been FIXED and must be removed from "+
				"knownPending (that removal is the acceptance test for the fix)",
				pinned, len(knownPending))
		}
		if len(regressions) > 0 {
			sort.Strings(regressions)
			t.Fatalf("%d shared case(s) load a body with FEWER genuinely-runnable "+
				"steps than a twin offers, because ActionTypePlaywright steps "+
				"(which SKIP with PLAYWRIGHT-RUNTIME-PENDING, see schema.go) are "+
				"counted as executable and inflate the rank. This is the HXC-305 "+
				"defect class itself — a preferred twin hiding an executable step "+
				"— and it is NOT covered by the F1 guards, which see the counts as "+
				"equal (HXC-305 M1):\n  %s",
				len(regressions), strings.Join(regressions, "\n  "))
		}
		t.Logf("%d shared case(s) checked; %d known pending-runtime demotion(s) "+
			"still reproducing: %v", shared, pinned, knownPending)
	})

	t.Run("metadata_dropped_by_the_preference_is_pinned", func(t *testing.T) {
		dir := filepath.Join("..", "..", "banks")
		if _, err := os.Stat(dir); err != nil {
			t.Skipf("SKIP-OK: banks/ dir not present at %s: %v", dir, err)
		}
		// Known, accepted tag losses: the body the preference keeps
		// does not carry these tags, and merging them would mean
		// synthesising a body neither twin actually declares. Pinned so
		// the set cannot grow unnoticed. Measured against banks/ at the
		// time this guard was written.
		want := map[string]string{
			"FQA-API-186": "null", // strict win
			"FQA-API-264": "404",  // strict win
			"FQA-XP-012":  "404",  // strict win
			"FQA-WEB-271": "404",  // tie
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read banks dir: %v", err)
		}
		byBase := map[string]map[string]string{}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext != ".yaml" && ext != ".yml" && ext != ".json" {
				continue
			}
			b := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			if byBase[b] == nil {
				byBase[b] = map[string]string{}
			}
			byBase[b][ext] = e.Name()
		}
		res, err := LoadDirVerbose(dir)
		if err != nil {
			t.Fatalf("LoadDirVerbose: %v", err)
		}
		loaded := map[string]*TestCase{}
		for _, bf := range res.Banks {
			for i := range bf.TestCases {
				if id := bf.TestCases[i].ID; id != "" {
					loaded[id] = &bf.TestCases[i]
				}
			}
		}
		got := map[string][]string{}
		for _, m := range byBase {
			jsonName, nonJSON := m[".json"], m[".yaml"]
			if nonJSON == "" {
				nonJSON = m[".yml"]
			}
			if jsonName == "" || nonJSON == "" {
				continue
			}
			for _, pair := range [][2]string{{jsonName, nonJSON}, {nonJSON, jsonName}} {
				bf, err := LoadFile(filepath.Join(dir, pair[0]))
				if err != nil {
					continue
				}
				for i := range bf.TestCases {
					src := &bf.TestCases[i]
					dst, ok := loaded[src.ID]
					if !ok || src.ID == "" {
						continue
					}
					have := map[string]bool{}
					for _, tg := range dst.Tags {
						have[tg] = true
					}
					for _, tg := range src.Tags {
						if !have[tg] {
							got[src.ID] = append(got[src.ID], tg)
						}
					}
				}
			}
		}
		for id, tags := range got {
			sort.Strings(tags)
			w, known := want[id]
			if !known {
				t.Errorf("%s: the loaded body drops tag(s) %v that a twin declares — a "+
					"NEW metadata loss the twin preference introduced. Either the corpus "+
					"changed or the preference did; pin it deliberately or fix it",
					id, tags)
				continue
			}
			if len(tags) != 1 || tags[0] != w {
				t.Errorf("%s: drops tag(s) %v, pinned as dropping exactly [%q]", id, tags, w)
			}
		}
		for id, w := range want {
			if _, found := got[id]; !found {
				t.Logf("pinned metadata loss %s (tag %q) no longer occurs — the pin can "+
					"be removed once that is deliberate", id, w)
			}
		}
	})
}

// redModeOn reports whether the RED polarity is active. Mirrors the
// convention already established in pkg/autonomous (§11.4.115): the
// STANDING default is GREEN; RED is opt-in via RED_MODE=1, which is
// how the pre-fix HXC-305 baseline was captured.
func redModeOn() bool {
	v := strings.TrimSpace(os.Getenv("RED_MODE"))
	return v == "1" || strings.EqualFold(v, "true")
}
