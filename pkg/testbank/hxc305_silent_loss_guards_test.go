// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// HXC-305 — hermetic guards for the silent-loss mechanisms that had
// NO test coverage at all.
//
// M1–M3 (round 6, BLOCKING B) shipped in round 4 and were verified
// only by a manual one-shot run. Round 5's independent review measured
// the consequence: each of the three could be DELETED and the whole
// package still passed.
//
//	M1  checkBankIDFloor: `return nil` as first statement   -> ok, EXIT=0
//	M2  the zero-case non-JSON twin abort -> `case false &&` -> ok, EXIT=0
//	M3  loadPlain's zero-case decline     -> `if false &&`   -> ok, EXIT=0
//
// M4–M5 (round 14) are the same class one layer out — not a mechanism
// that fails to DETECT a loss, but one that detects it and then throws
// the record away on the error path:
//
//	M4  loadDirVerbose's `abort` helper -> `return nil, err`  -> ok, EXIT=0
//	M5  LoadDir's twinLogger loop moved after the error return -> ok, EXIT=0
//
// A mechanism whose deletion no test notices is not a guard; it is
// commentary that happens to execute. These tests exist so that each
// mutation FAILS, and they are registered standing regression guards
// per §11.4.135 — the suite, not a human's memory, is what keeps the
// mechanisms alive.
//
// §11.4.115 polarity, and an HONEST BOUNDARY about it (§11.4.6).
// RED_MODE=1 flips each test into a defect-characterisation run;
// RED_MODE=0 (the standing default, so `go test ./...` stays green on
// the fixed artifact) is the regression guard. The polarity is NOT
// equally strong across the three, and that difference is stated
// rather than papered over:
//
//   - The FLOOR guard gets a TRUE §11.4.115 RED. The floor is opt-in
//     per directory, so a directory WITHOUT the file is literally the
//     pre-fix artifact — RED_MODE=1 removes the floor and asserts the
//     partial truncation is accepted SILENTLY, reproducing HXC-305 F2
//     exactly, on the same fixture the GREEN run catches it on.
//   - The two ZERO-CASE guards CANNOT get one. Both aborts are
//     unconditional in the loader, so no fixture can construct the
//     pre-fix loader; only editing the source could, which is the M2/M3
//     mutation itself and belongs in a mutation run, not in a committed
//     test. Their RED_MODE=1 therefore MEASURES the loss the abort
//     prevents (which ids exist only in the emptied file, and are
//     therefore unrecoverable from what survives on disk) rather than
//     claiming a reproduction it did not perform.
//   - The M4/M5 guards are in the same position as M2/M3: the discard
//     and the return-order are source properties, not fixture
//     properties. Their RED_MODE=1 measures what the discarded list or
//     the swallowed log line would have contained. Both instead carry a
//     POSITIVE CONTROL in the STANDING run — the same fixture minus the
//     aborting condition, asserted to produce the decline — so a zero
//     result on the aborting fixture is a measured absence rather than a
//     dead instrument.
//
// The kill assertions live in the STANDING (RED_MODE=0) run for all
// five, deliberately: a guard whose teeth are behind an opt-in env
// var would leave M1–M5 alive in every ordinary `go test` run, which
// is the exact hole being closed.

// hxc305RedModeOn reports whether the RED polarity is active.
// Deliberately named for this item rather than a bare redModeOn, so
// it cannot collide with another file's switch in this package later.
func hxc305RedModeOn() bool {
	v := strings.TrimSpace(os.Getenv("RED_MODE"))
	return v == "1" || strings.EqualFold(v, "true")
}

// hxc305YAMLBank renders a minimal, VALID yaml bank declaring one
// case per id. Every case carries a genuinely executable step, so
// these fixtures never depend on the executable-step comparison —
// the invariants under test fire before it.
func hxc305YAMLBank(name string, ids ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "version: \"1.0\"\nname: %q\ndescription: \"HXC-305 fixture\"\ntest_cases:\n", name)
	for _, id := range ids {
		fmt.Fprintf(&b, `  - id: %s
    name: "case %s"
    category: regression
    priority: high
    platforms: [web]
    steps:
      - name: "step"
        action: "http: GET /%s"
        expected: "200"
    expected_result: "ok"
`, id, id, id)
	}
	return b.String()
}

// hxc305JSONBank renders the same shape as a JSON twin.
func hxc305JSONBank(name string, ids ...string) string {
	var cases []string
	for _, id := range ids {
		cases = append(cases, fmt.Sprintf(`{"id":%q,"name":"case %s","category":"regression",`+
			`"priority":"high","platforms":["web"],"steps":[{"name":"step",`+
			`"action":"http: GET /%s","expected":"200"}],"expected_result":"ok"}`, id, id, id))
	}
	return fmt.Sprintf(`{"version":"1.0","name":%q,"description":"HXC-305 fixture",`+
		`"test_cases":[%s]}`, name, strings.Join(cases, ","))
}

// hxc305EmptyYAMLBank is a file that PARSES as a bank and declares
// nothing — the exact shape a truncated, emptied or half-written file
// takes, and the shape a stray non-bank document in a bank directory
// takes too.
const hxc305EmptyYAMLBank = "version: \"1.0\"\nname: \"emptied\"\ntest_cases: []\n"

func hxc305Write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

// TestHXC305_IDFloorCatchesPartialTruncation is the standing guard for
// checkBankIDFloor (kills M1).
//
// A PARTIAL truncation is the case zero-case detection structurally
// cannot see: the survivor still declares plenty of cases and parses
// as a perfectly well-formed bank. Only an expected id set can catch
// it, which is what the floor is.
//
// It also guards the round-6 secondary finding on the same path: the
// floor-failure return used to discard the declined list, so the CLI
// printed ZERO declined lines on precisely the run where they explain
// where content went.
func TestHXC305_IDFloorCatchesPartialTruncation(t *testing.T) {
	dir := t.TempDir()
	floor := filepath.Join(dir, bankIDFloorFile)
	alpha := filepath.Join(dir, "alpha.yaml")

	// A populated bank, a stray zero-case document (so the scan has a
	// declined entry to report), and a floor recording all three ids.
	hxc305Write(t, alpha, hxc305YAMLBank("Alpha", "FL-001", "FL-002", "FL-003"))
	hxc305Write(t, filepath.Join(dir, "stray.yaml"), hxc305EmptyYAMLBank)
	hxc305Write(t, floor, "# fixture floor\nFL-001\nFL-002\nFL-003\n")

	// Sanity: the fixture must load cleanly BEFORE the truncation, or
	// the test proves nothing about truncation (a guard that fires on
	// a fixture that never worked is a false positive machine).
	if _, err := LoadDirVerbose(dir); err != nil {
		t.Fatalf("fixture must load cleanly before truncation, got: %v", err)
	}

	// Truncate at a case boundary: FL-003 disappears, the file stays
	// valid YAML declaring 2 cases. This is HXC-305 F2 in miniature.
	hxc305Write(t, alpha, hxc305YAMLBank("Alpha", "FL-001", "FL-002"))

	if hxc305RedModeOn() {
		// TRUE pre-fix artifact: the floor is opt-in, so removing it
		// IS the loader as it behaved before this mechanism existed.
		if err := os.Remove(floor); err != nil {
			t.Fatalf("remove floor for RED baseline: %v", err)
		}
		res, err := LoadDirVerbose(dir)
		if err != nil {
			t.Fatalf("RED_MODE=1: the pre-fix artifact must accept the truncation "+
				"SILENTLY — that silence is the defect being reproduced — but the "+
				"scan errored: %v", err)
		}
		for _, bf := range res.Banks {
			for _, tc := range bf.TestCases {
				if tc.ID == "FL-003" {
					t.Fatal("RED_MODE=1: FL-003 must be GONE from the catalog; the " +
						"truncation did not take effect, so this run reproduces nothing")
				}
			}
		}
		t.Log("RED_MODE=1: partial truncation accepted at exit 0 with FL-003 " +
			"silently absent — HXC-305 F2 reproduced on the floor-less artifact")
		return
	}

	res, err := LoadDirVerbose(dir)
	if err == nil {
		t.Fatal("a PARTIAL truncation removed FL-003 and the scan SUCCEEDED. The id " +
			"floor is the only mechanism that can see this — zero-case detection " +
			"cannot, because the truncated file still declares 2 valid cases. " +
			"checkBankIDFloor is disabled, deleted, or no longer reached (HXC-305 M1)")
	}
	if !strings.Contains(err.Error(), "FL-003") {
		t.Errorf("the floor error must NAME the missing id — an operator needs to know "+
			"WHAT went missing, not merely that something did. Got: %v", err)
	}
	if !strings.Contains(err.Error(), bankIDFloorFile) {
		t.Errorf("the error must identify itself as the %s check so the operator knows "+
			"which mechanism fired and how to resolve it. Got: %v", bankIDFloorFile, err)
	}

	// Secondary: the declined list must survive the error return.
	if res == nil {
		t.Fatal("LoadDirVerbose returned a nil result alongside the floor error, so " +
			"every declined file is discarded on the single most diagnostic path " +
			"there is. The CLI prints the declined list it is handed, so this " +
			"renders it silent about which files were dropped exactly when that " +
			"explains the loss (HXC-305 round-6 BLOCKING A, secondary)")
	}
	var sawStray bool
	for _, d := range res.Declined {
		if strings.HasSuffix(d.Path, "stray.yaml") {
			sawStray = true
		}
	}
	if !sawStray {
		t.Errorf("the declined list returned with the floor error must still report "+
			"stray.yaml; got %d declined entr(ies): %+v", len(res.Declined), res.Declined)
	}
}

// TestHXC305_RealCorpusFloorIsPresentAndArmed is the corpus-side
// complement to the hermetic guard above, and it exists because
// checkBankIDFloor is OPT-IN: a directory with no floor file is not
// enforced, and the check returns nil. That is the right default for an
// arbitrary directory, but it means the guard protecting the real
// corpus is disarmed by the mere ABSENCE of one file — silently, at
// exit 0, with every other test in this package still green.
//
// The failure mode is not hypothetical and is not a code bug. The floor
// is a data file that has to be committed alongside the mechanism that
// reads it. Commit the loader and the guards, forget to `git add
// banks/.bank-id-floor.txt`, and every fresh clone and CI checkout runs
// the whole suite green with the entire id floor inert — the same
// silent-disarm class these tests exist to close, moved up one level to
// the commit boundary. Nothing else in this package looks at the real
// floor, so nothing else can notice.
//
// HONEST BOUNDARY (§11.4.6). This asserts the floor is PRESENT and
// ARMED in the tree the tests run against; it does not and cannot
// assert it is tracked by git (that would false-fail in an export or a
// tarball where no .git exists). Those differ only in a working tree
// where the file is present but unstaged — and a fresh clone, which is
// where the disarm actually bites, is precisely the case this catches.
func TestHXC305_RealCorpusFloorIsPresentAndArmed(t *testing.T) {
	dir := filepath.Join("..", "..", "banks")
	path := filepath.Join(dir, bankIDFloorFile)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the real bank corpus has NO readable id floor at %s (%v). "+
			"checkBankIDFloor treats a missing floor as 'not enforced for this "+
			"directory' and returns nil, so the entire id-floor guard over %s is "+
			"INERT — every truncation it exists to catch now passes at exit 0 "+
			"with this suite green. If the file was never committed, stage it; "+
			"if it was deliberately removed, that removal disarms a standing "+
			"guard and needs to be a visible, argued change, not an absence",
			path, err, dir)
	}

	var ids int
	for _, line := range strings.Split(string(data), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		ids++
	}
	if ids == 0 {
		t.Fatalf("the id floor at %s exists but records ZERO case ids, so it "+
			"protects nothing — a floor every id trivially satisfies is "+
			"indistinguishable from no floor at all. This is the header-only "+
			"artifact the old shell pipeline wrote (HXC-305 round-6 BLOCKING A); "+
			"regenerate it with `helixqa banks regen-floor --banks %s`, which "+
			"refuses to write an empty floor by construction", path, dir)
	}

	// Instrument validity: the floor must actually be satisfied by the
	// corpus it guards. A present, non-empty, but FAILING floor would
	// mean this test reports "armed" for a directory whose enforced scan
	// is broken — armed at something, but not at the truth.
	//
	// HONEST BOUNDARY (§11.4.6). That assertion is ONE-SIDED, and the
	// word "armed" in this test's name is correspondingly narrow. Because
	// checkBankIDFloor is a monotone LOWER BOUND, a wrong-but-satisfiable
	// floor passes it: truncating banks/.bank-id-floor.txt to its first 3
	// real ids leaves the enforced scan succeeding and this test green
	// (measured), with the remaining 3043 ids unprotected. "Armed" here
	// therefore means "present, and armed at >= 1 real id" — NOT "armed
	// over the corpus", which nothing in this file establishes. Nothing
	// stronger is asserted deliberately: any minimum-count threshold
	// would be an uncalibrated number rather than a derived one, and a
	// per-bank coverage assertion would reintroduce precisely the
	// add-tax that checkBankIDFloor's own honest boundary rejects (see
	// loader.go: "a monotone lower bound, plus a cheap and loud
	// regeneration"). The floor's CONTENT is instead kept honest by
	// RegenerateBankIDFloor being its only sanctioned writer, and by
	// that command's own guards in hxc305_floor_regen_test.go.
	if _, err := loadDirVerbose(dir, true); err != nil {
		t.Errorf("the enforced scan of %s fails against its own floor of %d id(s): "+
			"%v — either the corpus genuinely lost ids, or the floor is stale and "+
			"needs a deliberate regeneration recording what was removed",
			dir, ids, err)
	}
}

// TestHXC305_ZeroCaseNonJSONTwinAborts is the standing guard for the
// zero-case non-JSON twin invariant (kills M2).
//
// An emptied non-JSON twin is the ONE zero-case shape that must
// hard-fail rather than be reported: the pair resolves quietly to the
// JSON sibling, and every id the JSON form never carried vanishes
// with no error while the decline line claims "none lost" (measured
// on the real corpus: emptying banks/atmosphere.yaml loses exactly 50
// ids at exit 0).
func TestHXC305_ZeroCaseNonJSONTwinAborts(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "beta.json")
	yamlPath := filepath.Join(dir, "beta.yaml")

	// The JSON twin carries two shared ids; the YAML twin carries
	// those PLUS two of its own — the unconverted-prose residue that
	// exists nowhere else. Emptying the YAML is what puts ZC-Y01 and
	// ZC-Y02 at risk.
	hxc305Write(t, jsonPath, hxc305JSONBank("Beta", "ZC-001", "ZC-002"))
	hxc305Write(t, yamlPath, hxc305YAMLBank("Beta", "ZC-001", "ZC-002", "ZC-Y01", "ZC-Y02"))

	if _, err := LoadDirVerbose(dir); err != nil {
		t.Fatalf("fixture must load cleanly before the twin is emptied, got: %v", err)
	}

	hxc305Write(t, yamlPath, hxc305EmptyYAMLBank)

	if hxc305RedModeOn() {
		// HONEST BOUNDARY (§11.4.6): this does NOT reproduce the
		// pre-fix loader — the abort is unconditional and no fixture
		// can switch it off. It MEASURES the loss the abort prevents:
		// the ids that exist only in the emptied file and cannot be
		// recovered from the twin that would have survived it.
		emptied, err := LoadFile(yamlPath)
		if err != nil {
			t.Fatalf("RED_MODE=1: emptied twin must still PARSE — an unparseable file "+
				"is a different, already-covered defect: %v", err)
		}
		if len(emptied.TestCases) != 0 {
			t.Fatalf("RED_MODE=1: emptied twin must declare 0 cases, got %d",
				len(emptied.TestCases))
		}
		survivor, err := LoadFile(jsonPath)
		if err != nil {
			t.Fatalf("RED_MODE=1: surviving twin must parse: %v", err)
		}
		have := map[string]bool{}
		for _, tc := range survivor.TestCases {
			have[tc.ID] = true
		}
		var unrecoverable []string
		for _, id := range []string{"ZC-Y01", "ZC-Y02"} {
			if !have[id] {
				unrecoverable = append(unrecoverable, id)
			}
		}
		if len(unrecoverable) != 2 {
			t.Fatalf("RED_MODE=1: expected 2 ids recoverable from NEITHER twin, got %v",
				unrecoverable)
		}
		t.Logf("RED_MODE=1: %d id(s) exist only in the emptied twin and are "+
			"unrecoverable from the sibling that would survive the pair "+
			"resolution: %v — this is the loss the abort prevents",
			len(unrecoverable), unrecoverable)
		return
	}

	res, err := LoadDirVerbose(dir)
	if err == nil {
		// Surface the mutation's specific lie in the failure message:
		// the pair is resolved with a decline line claiming "none
		// lost" over two ids that just left the catalog. Only
		// reachable when the abort is gone, which is why it lives
		// here rather than after the guard.
		var lies []string
		var loaded int
		if res != nil {
			for _, bf := range res.Banks {
				loaded += len(bf.TestCases)
			}
			for _, d := range res.Declined {
				if strings.Contains(d.Reason, "none lost") {
					lies = append(lies, fmt.Sprintf("%s: %s", d.Path, d.Reason))
				}
			}
		}
		t.Fatalf("an emptied non-JSON twin was accepted at exit 0. The pair resolved "+
			"to the JSON sibling and ZC-Y01/ZC-Y02 left the catalog with no error "+
			"— the HXC-305 F2 shape this abort exists to refuse (M2). A zero-case "+
			"non-JSON twin must HARD-FAIL, not be reported as an ordinary decline. "+
			"Catalog loaded %d case(s) (4 were on disk before the twin was "+
			"emptied); decline lines claiming \"none lost\": %v",
			loaded, lies)
	}
	if !strings.Contains(err.Error(), "beta.yaml") {
		t.Errorf("the abort must name the emptied file so it can be fixed or deleted "+
			"deliberately. Got: %v", err)
	}
	if !strings.Contains(err.Error(), "0 test cases") {
		t.Errorf("the abort must state the fact it fired on — that the file declares "+
			"no cases. Got: %v", err)
	}
}

// TestHXC305_ZeroCaseStandaloneFileIsDeclined is the standing guard
// for loadPlain's zero-case decline (kills M3).
//
// Unlike the twin case this must NOT be fatal — a stray document in a
// bank directory must not newly break a directory that has always
// loaded, and there is no sibling here for lost content to hide
// behind. But it must be REPORTED: without the decline entry an
// emptied standalone bank is completely indistinguishable from a file
// that was never read at all.
func TestHXC305_ZeroCaseStandaloneFileIsDeclined(t *testing.T) {
	dir := t.TempDir()
	hxc305Write(t, filepath.Join(dir, "gamma.yaml"),
		hxc305YAMLBank("Gamma", "SA-001", "SA-002"))
	stray := filepath.Join(dir, "delta.yaml")
	hxc305Write(t, stray, hxc305EmptyYAMLBank)

	res, err := LoadDirVerbose(dir)
	if err != nil {
		t.Fatalf("a zero-case STANDALONE file must never be fatal — a stray document "+
			"in a bank directory cannot be allowed to break a directory that has "+
			"always loaded. Got: %v", err)
	}

	if hxc305RedModeOn() {
		// HONEST BOUNDARY (§11.4.6): as with the twin guard, this
		// measures rather than reproduces. It records what the file
		// looks like to every other mechanism — parses fine, declares
		// nothing, contributes nothing — i.e. exactly why it was
		// invisible before it was reported.
		bf, lerr := LoadFile(stray)
		if lerr != nil {
			t.Fatalf("RED_MODE=1: stray file must parse: %v", lerr)
		}
		if len(bf.TestCases) != 0 {
			t.Fatalf("RED_MODE=1: stray file must declare 0 cases, got %d",
				len(bf.TestCases))
		}
		t.Log("RED_MODE=1: delta.yaml parses successfully, declares 0 cases and " +
			"contributes nothing — absent a decline entry it is indistinguishable " +
			"from a file the scan never opened")
		return
	}

	var found *DeclinedFile
	for i := range res.Declined {
		if strings.HasSuffix(res.Declined[i].Path, "delta.yaml") {
			found = &res.Declined[i]
		}
	}
	if found == nil {
		t.Fatalf("delta.yaml parses, declares 0 test cases and contributes nothing, "+
			"yet the scan reported NO decline for it — making it indistinguishable "+
			"from a file that was never read. That is the silent-loss shape the "+
			"zero-case decline exists to close (M3). Declined entries were: %+v",
			res.Declined)
	}
	if !strings.Contains(found.Reason, "0 test cases") {
		t.Errorf("the decline must state that the file declares no cases; got %q",
			found.Reason)
	}

	// The populated sibling must be entirely unaffected: a guard that
	// also drops real content would be worse than the defect.
	var loaded int
	for _, bf := range res.Banks {
		loaded += len(bf.TestCases)
	}
	if loaded != 2 {
		t.Errorf("the stray file must not disturb the real bank: expected 2 loaded "+
			"case(s), got %d", loaded)
	}
}

// TestHXC305_DeclinedListSurvivesEveryAbort is the standing guard for
// M4: an abort path that discards the accumulated declined list.
//
// Round 6 fixed exactly ONE abort (the id-floor failure) to return the
// declined list alongside its error and left the rest returning a bare
// nil, while three comments — manager.go's, loader.go's floor return,
// and both CLI sites — described the fixed behaviour as if it were
// general. Round 14 measured the gap on this fixture: a directory with
// a declined twin AND a cross-bank duplicate id produced ZERO declined
// lines, while the SAME fixture minus the duplicate produced one. The
// operator was told "duplicate test case id" and nothing about the
// twin already dropped or the content that went with it.
//
// The fixture is ordered deliberately: the twin pair sorts FIRST, so
// it is resolved and declined BEFORE the duplicate aborts the scan. A
// fixture whose duplicate sorted first would pass on the broken loader
// too — it would be asserting on an empty list either way.
//
// §11.4.115 polarity, HONEST BOUNDARY (§11.4.6): this guard gets the
// same treatment as M2/M3 above, not the floor guard's true RED. The
// aborts are unconditional in the loader, so no fixture can construct
// the pre-fix loader; only editing the source can, which is the M4
// mutation and belongs in a mutation run. RED_MODE=1 therefore
// MEASURES what the discarded list would have contained rather than
// claiming a reproduction it did not perform. The reproduction on the
// genuinely broken artifact was performed out-of-tree, before the fix,
// against a positive control (same fixture minus the duplicate).
func TestHXC305_DeclinedListSurvivesEveryAbort(t *testing.T) {
	// Round 16: this guard was named "EveryAbort" while exercising ONE
	// abort — the cross-bank duplicate id. That gap is exactly how the
	// eighth discarding abort (the unparseable non-JSON twin) survived
	// round 14's sweep and shipped: a guard named for a universal but
	// testing a single case reports green for every case it never
	// visits. It is now a table over EVERY abort a fixture can reach.
	//
	// The nine `return abort(...)` sites in loadDirVerbose fall into
	// six reachable classes (below) plus three that no fixture can
	// construct: the `insertTwin` error returns. insertTwin always
	// calls addBank with isTwin=true, and insertBank returns an error
	// ONLY on the !isTwin branch — a twin's colliding ids are excluded
	// and reported, never errored. Those three are defensive guards on
	// a call that cannot fail, so they are named here rather than
	// silently omitted (§11.4.6: an untestable path is declared, not
	// counted as covered).
	type abortCase struct {
		name string
		// seed adds ONLY the content that causes the abort. The base
		// fixture (a declined twin pair, lexically first) is identical
		// across every case, so the positive control below is a
		// control for all of them.
		seed    func(t *testing.T, dir string)
		wantErr string
	}

	base := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		// Twin pair, lexically first: resolved (and one side declined)
		// before anything downstream can abort. A fixture whose abort
		// sorted first would pass on the broken loader too — it would
		// be asserting on an empty list either way.
		hxc305Write(t, filepath.Join(dir, "apair.yaml"), hxc305YAMLBank("apair", "AB-T-001"))
		hxc305Write(t, filepath.Join(dir, "apair.json"), hxc305JSONBank("apair", "AB-T-001"))
		return dir
	}

	cases := []abortCase{
		{
			// loader.go: `return abort(nonJSONErr)` — the round-16 find.
			name: "unparseable_non_json_twin",
			seed: func(t *testing.T, dir string) {
				hxc305Write(t, filepath.Join(dir, "zbroken.yaml"),
					"name: zbroken\ntest_cases:\n  - id: ZB-001\n    name: \"x\n    steps: [[[\n")
				hxc305Write(t, filepath.Join(dir, "zbroken.json"), hxc305JSONBank("zbroken", "ZB-001"))
			},
			wantErr: "parse bank file",
		},
		{
			// loader.go: the zero-case non-JSON twin abort.
			name: "emptied_non_json_twin",
			seed: func(t *testing.T, dir string) {
				hxc305Write(t, filepath.Join(dir, "zempty.yaml"), hxc305EmptyYAMLBank)
				hxc305Write(t, filepath.Join(dir, "zempty.json"), hxc305JSONBank("zempty", "ZE-001"))
			},
			wantErr: "declares 0 test cases",
		},
		{
			// loader.go: loadPlain(nonJSONName) — a standalone YAML pair
			// sharing an id.
			name: "standalone_non_json_duplicate_id",
			seed: func(t *testing.T, dir string) {
				hxc305Write(t, filepath.Join(dir, "zdup-a.yaml"), hxc305YAMLBank("zdup-a", "AB-D-001"))
				hxc305Write(t, filepath.Join(dir, "zdup-b.yaml"), hxc305YAMLBank("zdup-b", "AB-D-001"))
			},
			wantErr: "duplicate test case id",
		},
		{
			// loader.go: loadPlain(jsonName) — the JSON-only arm of the
			// same switch, which the YAML case above never exercises.
			name: "standalone_json_duplicate_id",
			seed: func(t *testing.T, dir string) {
				hxc305Write(t, filepath.Join(dir, "zja.json"), hxc305JSONBank("zja", "AB-J-001"))
				hxc305Write(t, filepath.Join(dir, "zjb.json"), hxc305JSONBank("zjb", "AB-J-001"))
			},
			wantErr: "duplicate test case id",
		},
		{
			// loader.go: the extras loop — a third file sharing one base
			// name (.yaml + .yml), so the .yml is neither the JSON nor
			// the non-JSON twin and is loaded as an extra.
			name: "extra_extension_duplicate_id",
			seed: func(t *testing.T, dir string) {
				hxc305Write(t, filepath.Join(dir, "zextra.yaml"), hxc305YAMLBank("zextra", "AB-X-001"))
				hxc305Write(t, filepath.Join(dir, "zextra.yml"), hxc305YAMLBank("zextra", "AB-X-001"))
			},
			wantErr: "duplicate test case id",
		},
		{
			// loader.go: the id-floor failure — the ONE abort round 6
			// fixed, kept here so a regression on it fails with the rest.
			name: "id_floor_shortfall",
			seed: func(t *testing.T, dir string) {
				hxc305Write(t, filepath.Join(dir, bankIDFloorFile),
					"# fixture floor\nAB-T-001\nAB-GONE-001\n")
			},
			wantErr: bankIDFloorFile,
		},
	}

	// POSITIVE CONTROL, always run: the base fixture must load cleanly
	// and report exactly the twin decline. Without it, a zero-decline
	// result on an abort fixture could not be told apart from a fixture
	// that declines nothing at all — the measurement would have no
	// instrument behind it.
	ctrl, ctrlErr := LoadDirVerbose(base(t))
	if ctrlErr != nil {
		t.Fatalf("positive control FAILED to load: %v — the base fixture must be "+
			"loadable without any abort seed, or this test measures nothing", ctrlErr)
	}
	if len(ctrl.Declined) != 1 {
		t.Fatalf("positive control: want exactly 1 declined twin, got %d (%+v) — "+
			"the instrument does not detect a decline it is supposed to see",
			len(ctrl.Declined), ctrl.Declined)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := base(t)
			tc.seed(t, dir)

			res, err := LoadDirVerbose(dir)
			if err == nil {
				t.Fatalf("%s must abort the scan; it did not. Loaded: %+v", tc.name, res)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not identify the %s abort (want substring %q)",
					err, tc.name, tc.wantErr)
			}

			if hxc305RedModeOn() {
				t.Logf("RED_MODE=1: the aborting scan had already declined %q (%s). "+
					"A bare-nil return discards exactly that entry, so the operator "+
					"sees only %q and no record that the twin was dropped or what "+
					"content went with it — unrecoverable from the error text alone",
					ctrl.Declined[0].Path, ctrl.Declined[0].Reason, tc.wantErr)
				return
			}

			if res == nil {
				t.Fatalf("LoadDirVerbose returned a nil result alongside the %s "+
					"error, discarding every file already declined. The CLI prints "+
					"the declined list it is handed, so this renders it silent about "+
					"which twins were dropped on exactly the run where that explains "+
					"the loss", tc.name)
			}
			var sawTwin bool
			for _, d := range res.Declined {
				if strings.HasSuffix(d.Path, "apair.json") {
					sawTwin = true
				}
			}
			if !sawTwin {
				t.Fatalf("the declined twin apair.json is absent from the list "+
					"returned alongside the %s abort. Declined entries were: %+v",
					tc.name, res.Declined)
			}

			// The user-visible surface: the CLI does not call the package
			// function, it calls the Manager method and prints what it is
			// handed. Asserting only on the package function would leave
			// the path the operator actually sees unguarded.
			m := NewManager()
			declined, mErr := m.LoadDirVerbose(dir)
			if mErr == nil {
				t.Fatalf("Manager.LoadDirVerbose did not surface the %s abort", tc.name)
			}
			if len(declined) == 0 {
				t.Fatalf("Manager.LoadDirVerbose returned an EMPTY declined list "+
					"alongside the %s abort — this is the exact value the CLI loops "+
					"over, so the operator gets zero declined lines", tc.name)
			}
		})
	}
}

// TestHXC305_LoadDirLogsDeclinedEvenWhenTheScanAborts is the standing
// guard for M5: package LoadDir returning before its twinLogger loop.
//
// LoadDir's whole reason to exist over LoadDirVerbose is that a caller
// which ignores the declined list still gets a non-silent record via
// the logger — its doc comment says the call "is never silent about
// what it skipped". Round 14 measured it logging ZERO bytes on a
// directory with a declined twin and a FAILING id floor, while the
// same directory with a satisfied floor logged the decline: silent on
// exactly the run where the decline explains where content went.
// pkg/autonomous.StructuredTestExecutor only ever calls LoadDir, so
// this is a real consumer's only view of the scan.
//
// Manager.LoadDir logs before returning the error too; this asserts
// the package-level function does the same. Both orderings are NEW in
// this change — at 0634b1b8 Manager.LoadDir did not log at all, so
// neither is an inherited convention the other is being aligned to.
//
// §11.4.115 polarity, HONEST BOUNDARY (§11.4.6): as with M4, the
// pre-fix ordering is a source property, not a fixture property, so
// RED_MODE=1 MEASURES the loss (what the logger would have swallowed)
// rather than reproducing it. The paired §1.1 mutation restores the
// early return and this guard FAILs.
func TestHXC305_LoadDirLogsDeclinedEvenWhenTheScanAborts(t *testing.T) {
	newFixture := func(t *testing.T, floorHolds bool) string {
		t.Helper()
		dir := t.TempDir()
		hxc305Write(t, filepath.Join(dir, "logged.yaml"), hxc305YAMLBank("logged", "LD-001"))
		hxc305Write(t, filepath.Join(dir, "logged.json"), hxc305JSONBank("logged", "LD-001"))
		floor := "LD-001\n"
		if !floorHolds {
			// An id the directory cannot supply: the floor fails, the
			// scan aborts, and the decline is what explains it.
			floor = "LD-001\nLD-GONE-001\n"
		}
		hxc305Write(t, filepath.Join(dir, bankIDFloorFile), floor)
		return dir
	}

	capture := func(t *testing.T, dir string) (string, error) {
		t.Helper()
		var buf bytes.Buffer
		prev := twinLogger
		twinLogger = log.New(&buf, "", 0)
		defer func() { twinLogger = prev }()
		_, err := LoadDir(dir)
		return buf.String(), err
	}

	// POSITIVE CONTROL, always run: with the floor satisfied the same
	// directory must log its decline. A zero-byte capture below is
	// only evidence of silence if the instrument can record sound.
	ctrlLog, ctrlErr := capture(t, newFixture(t, true))
	if ctrlErr != nil {
		t.Fatalf("positive control FAILED to load: %v", ctrlErr)
	}
	if !strings.Contains(ctrlLog, "logged.json") {
		t.Fatalf("positive control logged nothing naming the declined twin: %q — "+
			"the logger capture does not work, so the aborting case proves nothing",
			ctrlLog)
	}

	logged, err := capture(t, newFixture(t, false))
	if err == nil {
		t.Fatal("a floor naming an absent id must abort the scan; it did not")
	}

	if hxc305RedModeOn() {
		t.Logf("RED_MODE=1: the aborting scan declined a twin that the control run "+
			"logs as %q. Returning before the logger loop swallows that line, so a "+
			"LoadDir-only caller sees the floor error and no record of the decline",
			strings.TrimSpace(ctrlLog))
		return
	}

	if logged == "" {
		t.Fatal("LoadDir logged NOTHING on an aborted scan. Its doc comment " +
			"promises the call is never silent about what it skipped; returning " +
			"the error before the twinLogger loop makes it silent precisely when " +
			"the decline explains the failure (M5)")
	}
	if !strings.Contains(logged, "logged.json") {
		t.Errorf("the logged output does not name the declined twin: %q", logged)
	}
}
