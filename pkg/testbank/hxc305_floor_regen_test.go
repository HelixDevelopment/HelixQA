// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// HXC-305 round 8, BLOCKING — hermetic guards for RegenerateBankIDFloor,
// the ONLY sanctioned way to rewrite the id floor and, by design, the
// one path that BYPASSES the guard the floor implements.
//
// Round 6 closed round-5's BLOCKING B for the three silent-loss
// mechanisms (see hxc305_silent_loss_guards_test.go) and, in the same
// round, shipped this mechanism verified only by a manual one-shot
// run — reintroducing the very defect class being closed: a
// regeneration path that could be DELETED, or quietly disarmed, with
// the whole package still green. Round 7 measured it: repo-wide,
// nothing referenced RegenerateBankIDFloor or cmdBanksRegenFloor, and
// all five ready-made mutations survived at exit 0.
//
// The four properties that make a regeneration safe, and the mutation
// each of these tests kills (§1.1, registered standing guards per
// §11.4.135):
//
//	R1  loader.go:872  loadDirVerbose(dir,false) -> true
//	                   "floor check OFF by construction" — a regeneration
//	                   is ONLY ever run while the floor is failing, so an
//	                   enforced scan makes the command unusable in the one
//	                   state it exists for (the round-6 BLOCKING A shape).
//	R2  loader.go:898  the empty-set refusal -> `if false &&`
//	                   writes the header-only, zero-id floor the old shell
//	                   pipeline produced — the exact BLOCKING A artifact,
//	                   which disarms the guard at exit 0.
//	R3  loader.go:957  temp+rename -> direct os.WriteFile
//	                   replacement stops being atomic; an interrupted or
//	                   refused write can leave a truncated floor, and the
//	                   floor is then written THROUGH rather than replaced.
//	R5  loader.go:940  drop `out.Removed` population
//	                   deletes the operator WARNING naming every id the
//	                   floor will stop protecting — measured as the ONLY
//	                   thing between an accidental truncation and a
//	                   permanently laundered baseline.
//
// HONEST BOUNDARY (§11.4.6) — R4 is NOT killed here, and this file does
// not pretend otherwise. R4 drops the Declined/ExcludedIDs union at
// loader.go:885-889. That union is currently a SEMANTIC NO-OP, because
// insertBank only ever excludes an id that some EARLIER bank already
// claimed in xref, and in any SUCCESSFULLY-RETURNED LoadDirResult the
// bank that claimed it is in banks — so `excluded` ⊆ loaded, and
// loaded∪excluded == loaded.
//
// The qualifier is load-bearing and not decoration. A bank can claim an
// id into xref and NOT be appended: the plain-vs-plain collision path
// returns a hard error at loader.go:1086-1089 after earlier cases of
// that same file already wrote xref entries. That does not weaken the
// subset relation, because the error propagates out of loadDirVerbose
// as `nil, err` (loader.go:638/643/647/653) — no LoadDirResult carrying
// the half-registered state is ever observable, so no caller, including
// RegenerateBankIDFloor, can see an excluded id outside the catalog by
// that route. Stated over returned results, not over intermediate
// loop state. Measured on this loader, not assumed:
// two twin/plain collision fixtures and the real banks/ corpus (50
// declined entries, 2 excluded ids) all yield excluded-not-loaded = ∅.
// No black-box fixture can therefore make R4 observable, and a test
// claiming to kill it would be the bluff this round exists to remove.
// TestHXC305_RegenFloorRecordsExcludedIDs instead pins the union
// CONTRACT and stands as a tripwire on the invariant that makes R4
// inert: the day an excluded id can be genuinely absent from the
// loaded catalog, that test fails and says the union has become
// load-bearing and needs a kill test of its own.
//
// §11.4.115 polarity, stated rather than assumed. Only R2 admits a
// TRUE RED-on-the-broken-artifact, and it gets one:
// TestHXC305_HeaderOnlyFloorDisarmsTheGuard reproduces the header-only
// floor the old pipeline wrote and asserts, at RED_MODE=1, that a
// truncation then passes SILENTLY — the round-6 BLOCKING A consequence,
// on the same fixture the RED_MODE=0 run catches it on. R1, R3 and R5
// cannot get one: their pre-fix artifact was a documented SHELL
// PIPELINE, not a code path this package ever had, so no fixture can
// construct it and only editing the source could — which is the
// mutation run itself, and does not belong in a committed test.
//
// All kill assertions live in the STANDING (RED_MODE=0) run, so an
// ordinary `go test ./...` is what keeps these mechanisms alive.
//
// Fixture helpers (hxc305YAMLBank / hxc305JSONBank / hxc305Write /
// hxc305RedModeOn) are shared with hxc305_silent_loss_guards_test.go.

// hxc305FloorBody reads a floor file whole, failing the test if it
// cannot be read — a floor that vanished is never an acceptable state
// for any assertion below.
func hxc305FloorBody(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, bankIDFloorFile))
	if err != nil {
		t.Fatalf("read floor in %s: %v", dir, err)
	}
	return string(data)
}

// hxc305FloorIDs returns the non-comment, non-blank lines of a floor —
// the ids it actually protects.
func hxc305FloorIDs(t *testing.T, dir string) []string {
	t.Helper()
	var ids []string
	for _, line := range strings.Split(hxc305FloorBody(t, dir), "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		ids = append(ids, s)
	}
	sort.Strings(ids)
	return ids
}

// hxc305WriteFloor writes a floor with the given header and ids, in
// the exact shape RegenerateBankIDFloor itself produces.
func hxc305WriteFloor(t *testing.T, dir, header string, ids ...string) {
	t.Helper()
	body := header + "\n"
	if len(ids) > 0 {
		body += strings.Join(ids, "\n") + "\n"
	}
	hxc305Write(t, filepath.Join(dir, bankIDFloorFile), body)
}

// hxc305NoTempResidue asserts no regeneration temp file survived.
// Cleanup, NOT the atomicity kill: a direct-write implementation
// leaves no temp either, so this assertion cannot distinguish them and
// is not claimed to.
func hxc305NoTempResidue(t *testing.T, dir string) {
	t.Helper()
	leftover, err := filepath.Glob(filepath.Join(dir, bankIDFloorFile+".regen-*"))
	if err != nil {
		t.Fatalf("glob temp residue in %s: %v", dir, err)
	}
	if len(leftover) > 0 {
		t.Errorf("regeneration left %d temp file(s) behind in %s: %v — an "+
			"abandoned temp floor is loose state in a bank directory",
			len(leftover), dir, leftover)
	}
}

// TestHXC305_RegenFloorScansWithFloorCheckOff is the standing guard for
// the "floor check OFF by construction" property (kills R1).
//
// A regeneration is BY DEFINITION run while the floor is failing —
// that failure is what sends the operator to the command. If the scan
// it performs enforced the floor, the command would abort on exactly
// the state it exists to resolve, and the operator's only remaining
// move would be to hand-edit or delete the floor: the guard removed
// rather than updated. The precondition below asserts the fixture
// really is in that failing state, so this test cannot pass by
// accident on a directory whose floor happens to be satisfied.
func TestHXC305_RegenFloorScansWithFloorCheckOff(t *testing.T) {
	dir := t.TempDir()
	hxc305Write(t, filepath.Join(dir, "alpha.yaml"), hxc305YAMLBank("alpha", "A1", "A2"))
	const header = "# fixture floor"
	hxc305WriteFloor(t, dir, header, "A1", "A2", "GONE1")

	// Instrument validity: the floor MUST genuinely be failing, or
	// this test proves nothing about bypassing it.
	if _, err := loadDirVerbose(dir, true); err == nil {
		t.Fatalf("fixture is not in the state a regeneration is run in: the "+
			"enforced scan of %s SUCCEEDED, so the floor is not failing and "+
			"the bypass under test is not exercised", dir)
	} else if !strings.Contains(err.Error(), "GONE1") {
		t.Fatalf("enforced scan failed for the wrong reason (want the floor "+
			"naming GONE1): %v", err)
	}

	res, err := RegenerateBankIDFloor(dir)
	if err != nil {
		t.Fatalf("R1 KILL: RegenerateBankIDFloor(%s) must succeed while the "+
			"floor is FAILING — that is the only state it is ever run in — but "+
			"it returned: %v. The scan must run with the floor check OFF "+
			"(loader.go: loadDirVerbose(dir, false)); enforcing it there makes "+
			"the command unusable and forces the operator to delete the guard "+
			"instead of updating it", dir, err)
	}

	if got, want := hxc305FloorIDs(t, dir), []string{"A1", "A2"}; !slices.Equal(got, want) {
		t.Errorf("regenerated floor ids = %v, want %v — the rewrite must record "+
			"the directory's live catalog", got, want)
	}
	if res.Total != 2 {
		t.Errorf("res.Total = %d, want 2", res.Total)
	}
	if !strings.HasPrefix(hxc305FloorBody(t, dir), header+"\n") {
		t.Errorf("existing header was not preserved verbatim; floor now starts:\n%s",
			hxc305FloorBody(t, dir))
	}

	// The regenerated floor must satisfy the enforced scan it feeds —
	// the round-trip that makes the bypass safe rather than merely
	// convenient.
	if _, err := loadDirVerbose(dir, true); err != nil {
		t.Errorf("enforced scan still fails after regeneration: %v — a "+
			"regeneration that does not clear the failure it was run for is "+
			"not a regeneration", err)
	}
	hxc305NoTempResidue(t, dir)
}

// TestHXC305_RegenFloorRefusesEmptyIDSet is the standing guard for the
// empty-set refusal (kills R2).
//
// A zero-id floor is the precise artifact the old shell pipeline wrote
// at exit 0 (HXC-305 round-6 BLOCKING A): it protects nothing, so
// every future removal goes unnoticed while the file's presence still
// says the directory is guarded. Refusing outright is what makes that
// outcome unreachable rather than merely detectable, and the refusal
// must leave the previous floor untouched — a failed regeneration that
// still destroyed the old floor would disarm the guard just as
// thoroughly as writing an empty one.
func TestHXC305_RegenFloorRefusesEmptyIDSet(t *testing.T) {
	dir := t.TempDir()
	// A directory with a floor and no bank files at all: the scan
	// succeeds and yields nothing, which is exactly the shape an
	// operator hits after pointing the command at the wrong path, or
	// after a catastrophe emptied the directory.
	hxc305WriteFloor(t, dir, "# fixture floor", "B1", "B2")

	// Instrument validity: the SCAN itself must succeed, or the
	// refusal below would be indistinguishable from an ordinary scan
	// error and this test would not exercise the refusal at all.
	res, err := loadDirVerbose(dir, false)
	if err != nil {
		t.Fatalf("fixture invalid: unenforced scan of %s failed (%v); the "+
			"refusal under test is only reachable when the scan SUCCEEDS with "+
			"zero ids", dir, err)
	}
	if len(res.Banks) != 0 {
		t.Fatalf("fixture invalid: expected 0 banks, got %d", len(res.Banks))
	}

	before := hxc305FloorBody(t, dir)

	out, err := RegenerateBankIDFloor(dir)
	if err == nil {
		t.Fatalf("R2 KILL: RegenerateBankIDFloor(%s) returned nil error for a "+
			"scan that yielded 0 case ids (result %+v). Writing a floor that "+
			"protects nothing is never a legitimate outcome — it is the exact "+
			"header-only artifact the old `helixqa list` pipeline produced at "+
			"exit 0, and it disarms the guard while looking like it is armed",
			dir, out)
	}
	if !strings.Contains(err.Error(), "refusing to write an empty") {
		t.Errorf("refusal fired for the wrong reason — want the empty-floor "+
			"refusal, got: %v", err)
	}
	if !strings.Contains(err.Error(), bankIDFloorFile) {
		t.Errorf("refusal message does not name %s, so the operator cannot tell "+
			"which file was not written: %v", bankIDFloorFile, err)
	}

	if after := hxc305FloorBody(t, dir); after != before {
		t.Errorf("R2 KILL: the refused regeneration MODIFIED the existing floor.\n"+
			"before:\n%s\nafter:\n%s\nA refusal must leave the previous floor "+
			"byte-identical; destroying it fails just as open as writing an "+
			"empty one", before, after)
	}
	hxc305NoTempResidue(t, dir)
}

// TestHXC305_HeaderOnlyFloorDisarmsTheGuard is the §11.4.115 RED for
// the refusal above: it reproduces the broken artifact on the pre-fix
// pipeline's own terms and shows what it costs.
//
// RED_MODE=1 writes the header-only floor the old pipeline produced,
// truncates a bank, and asserts the loss passes SILENTLY — HXC-305
// round-6 BLOCKING A, reproduced rather than described.
// RED_MODE=0 (the standing default) runs the identical fixture with a
// floor that records the ids, and asserts the same truncation is
// caught and the lost id named.
func TestHXC305_HeaderOnlyFloorDisarmsTheGuard(t *testing.T) {
	dir := t.TempDir()
	bank := filepath.Join(dir, "charlie.yaml")
	hxc305Write(t, bank, hxc305YAMLBank("charlie", "C1", "C2"))

	const header = "# HelixQA bank id floor — fixture"
	if hxc305RedModeOn() {
		// The broken artifact: header, no ids.
		hxc305WriteFloor(t, dir, header)
	} else {
		hxc305WriteFloor(t, dir, header, "C1", "C2")
	}

	// The truncation: charlie.yaml still parses and still declares a
	// case, so nothing but an expected id set can see that C2 is gone.
	hxc305Write(t, bank, hxc305YAMLBank("charlie", "C1"))

	_, err := loadDirVerbose(dir, true)

	if hxc305RedModeOn() {
		if err != nil {
			t.Fatalf("RED did not reproduce: a header-only floor was expected to "+
				"accept the truncation silently, but the scan failed: %v", err)
		}
		t.Logf("RED reproduced: with a header-only floor, C2 vanished from the " +
			"catalog at exit 0 and nothing reported it — the artifact the old " +
			"regeneration pipeline wrote, and the reason RegenerateBankIDFloor " +
			"refuses to write one")
		return
	}

	if err == nil {
		t.Fatalf("GREEN: the floor recorded C1 and C2, C2 was truncated away, and " +
			"the enforced scan still SUCCEEDED — the silent loss this guard exists " +
			"to catch")
	}
	if !strings.Contains(err.Error(), "C2") {
		t.Errorf("the scan failed but never named the missing id C2, so the "+
			"operator cannot tell what was lost: %v", err)
	}
}

// TestHXC305_RegenFloorReplacesFloorByRename is the standing guard for
// the atomic write (kills R3).
//
// The write is a temp file renamed over the target, so a regeneration
// that dies mid-write leaves the previous floor intact rather than a
// truncated one. The observable that distinguishes it from a direct
// write is that the floor is REPLACED, never written THROUGH: a rename
// installs a NEW file at the path, while a direct write truncates the
// existing one in place and keeps its identity. os.SameFile compares
// that identity, so the primary assertion below holds regardless of
// permissions, uid or filesystem.
//
// The read-only arm is a second, independent expression of the same
// property (renaming over a target needs write permission on the
// DIRECTORY; writing through it needs write permission on the FILE),
// and is skipped honestly where the host cannot enforce it.
func TestHXC305_RegenFloorReplacesFloorByRename(t *testing.T) {
	dir := t.TempDir()
	hxc305Write(t, filepath.Join(dir, "delta.yaml"), hxc305YAMLBank("delta", "D1", "D2"))
	hxc305WriteFloor(t, dir, "# fixture floor", "D1", "D2")
	floor := filepath.Join(dir, bankIDFloorFile)

	before, err := os.Stat(floor)
	if err != nil {
		t.Fatalf("stat floor before: %v", err)
	}

	if _, err := RegenerateBankIDFloor(dir); err != nil {
		t.Fatalf("regenerate: %v", err)
	}

	after, err := os.Stat(floor)
	if err != nil {
		t.Fatalf("stat floor after: %v", err)
	}
	if os.SameFile(before, after) {
		t.Errorf("R3 KILL: the floor at %s is the SAME file it was before the "+
			"regeneration, so it was written THROUGH rather than replaced. The "+
			"rewrite must go to a temp file renamed over the target (loader.go: "+
			"os.CreateTemp + os.Rename) — that is what makes an interrupted "+
			"regeneration leave the previous floor intact instead of a truncated "+
			"one", floor)
	}
	if got, want := hxc305FloorIDs(t, dir), []string{"D1", "D2"}; !slices.Equal(got, want) {
		t.Errorf("floor ids after replacement = %v, want %v", got, want)
	}
	hxc305NoTempResidue(t, dir)

	// Second arm: a read-only floor. Instrument validity (§11.4.3) —
	// confirm on THIS host and uid that a direct write to a read-only
	// file actually fails, or the arm proves nothing and is skipped
	// rather than reported as verified.
	probe := filepath.Join(t.TempDir(), "probe")
	hxc305Write(t, probe, "x")
	if err := os.Chmod(probe, 0o444); err != nil {
		t.Fatalf("chmod probe: %v", err)
	}
	if err := os.WriteFile(probe, []byte("y"), 0o644); err == nil {
		t.Log("SKIP-OK (§11.4.3): read-only arm not run — a direct write to a " +
			"mode-0444 file SUCCEEDS on this host (root, or a filesystem not " +
			"enforcing the mode), so it cannot discriminate. The primary " +
			"SameFile assertion above still holds")
		return
	}

	ro := t.TempDir()
	hxc305Write(t, filepath.Join(ro, "delta.yaml"), hxc305YAMLBank("delta", "D1", "D2"))
	hxc305WriteFloor(t, ro, "# fixture floor", "D1", "D2")
	if err := os.Chmod(filepath.Join(ro, bankIDFloorFile), 0o444); err != nil {
		t.Fatalf("chmod floor: %v", err)
	}
	if _, err := RegenerateBankIDFloor(ro); err != nil {
		t.Errorf("R3 KILL (read-only arm): RegenerateBankIDFloor(%s) failed "+
			"against a mode-0444 floor: %v. A replacement by rename does not "+
			"need write permission on the file it replaces; writing through it "+
			"does", ro, err)
	}
	hxc305NoTempResidue(t, ro)
}

// TestHXC305_RegenFloorReportsRemovedIDs is the standing guard for the
// removal accounting (kills R5).
//
// A regeneration exists to record a REMOVAL, so the removed ids are
// the one fact that must reach the operator. Round 7 measured what
// happens without them: partially truncate a bank, `list` exits 1
// naming 20 ids, the operator follows the documented remedy,
// regen-floor exits 0, the floor drops from 3046 to 3026, and `list`
// passes forever after. The WARNING built from res.Removed is the only
// thing standing between an accidental truncation and a permanently
// laundered baseline — so it must not be silently deletable.
func TestHXC305_RegenFloorReportsRemovedIDs(t *testing.T) {
	dir := t.TempDir()
	hxc305Write(t, filepath.Join(dir, "echo.yaml"), hxc305YAMLBank("echo", "E1"))
	// A project-specific header, so header preservation is asserted on
	// content the default would not reproduce by coincidence.
	const header = "# project note: this directory is release-critical"
	hxc305WriteFloor(t, dir, header, "E1", "E2", "E3")

	res, err := RegenerateBankIDFloor(dir)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}

	if want := []string{"E2", "E3"}; !slices.Equal(res.Removed, want) {
		t.Fatalf("R5 KILL: res.Removed = %v, want %v. E2 and E3 were recorded by "+
			"the previous floor and are absent from the catalog, so this "+
			"regeneration stops protecting them. Without this list the command "+
			"prints no WARNING, and an accidental truncation is laundered into "+
			"the baseline at exit 0 — indistinguishable from a deliberate "+
			"removal", res.Removed, want)
	}
	if len(res.Added) != 0 {
		t.Errorf("res.Added = %v, want none — nothing was added", res.Added)
	}
	if res.Created {
		t.Errorf("res.Created = true for a directory that already had a floor")
	}
	if res.Total != 1 {
		t.Errorf("res.Total = %d, want 1", res.Total)
	}
	if got, want := hxc305FloorIDs(t, dir), []string{"E1"}; !slices.Equal(got, want) {
		t.Errorf("floor ids = %v, want %v", got, want)
	}
	if !strings.HasPrefix(hxc305FloorBody(t, dir), header+"\n") {
		t.Errorf("the project's own header was not preserved verbatim; floor "+
			"now starts:\n%s", hxc305FloorBody(t, dir))
	}
	hxc305NoTempResidue(t, dir)
}

// TestHXC305_RegenFloorCreatesFirstFloor guards the first-floor path:
// a directory that has never carried a floor gets the default header,
// is reported as Created, and does NOT report every id as an addition.
func TestHXC305_RegenFloorCreatesFirstFloor(t *testing.T) {
	dir := t.TempDir()
	hxc305Write(t, filepath.Join(dir, "foxtrot.yaml"), hxc305YAMLBank("foxtrot", "F1", "F2"))

	res, err := RegenerateBankIDFloor(dir)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if !res.Created {
		t.Errorf("res.Created = false for a directory that had no floor")
	}
	if len(res.Added) != 0 {
		t.Errorf("res.Added = %v, want none — reporting every id as an addition "+
			"against a floor that did not exist is noise, not information",
			res.Added)
	}
	if len(res.Removed) != 0 {
		t.Errorf("res.Removed = %v, want none", res.Removed)
	}
	if got, want := hxc305FloorIDs(t, dir), []string{"F1", "F2"}; !slices.Equal(got, want) {
		t.Errorf("floor ids = %v, want %v", got, want)
	}
	if !strings.HasPrefix(hxc305FloorBody(t, dir), defaultBankIDFloorHeader) {
		t.Errorf("a first floor did not get the default header; floor starts:\n%s",
			hxc305FloorBody(t, dir))
	}
	if _, err := loadDirVerbose(dir, true); err != nil {
		t.Errorf("the floor just created does not satisfy the enforced scan: %v", err)
	}
	hxc305NoTempResidue(t, dir)
}

// TestHXC305_RegenFloorSuppressesAddedNoiseOnZeroIDFloor guards the
// condition the addition-noise suppression is keyed on.
//
// The suppression exists because reporting every id in the catalog as an
// "addition" is noise, not information. The state that produces that
// noise is a floor recording NO ids — and a floor file that is ABSENT is
// only one of the two ways to be in it. The other is a floor that exists
// but records zero ids: the header-only file the old shell pipeline
// wrote at exit 0, i.e. the round-6 BLOCKING A artifact this whole
// mechanism was built to make unreachable. Keying the suppression on
// `Created` (file absent) therefore covered the cheap case and missed
// the one that actually occurs, emitting a +3046 delta line on the real
// corpus.
//
// This test pins the general condition — no previously-recorded ids, by
// whatever route — so the suppression cannot silently narrow back to the
// file-existence test.
func TestHXC305_RegenFloorSuppressesAddedNoiseOnZeroIDFloor(t *testing.T) {
	dir := t.TempDir()
	hxc305Write(t, filepath.Join(dir, "alpha.yaml"), hxc305YAMLBank("alpha", "A1", "A2", "A3"))
	// The round-6 BLOCKING A shape: the file exists, so this is NOT the
	// Created path, but it protects nothing.
	hxc305WriteFloor(t, dir, "# header-only floor")

	res, err := RegenerateBankIDFloor(dir)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if res.Created {
		t.Fatalf("res.Created = true for a floor file that EXISTS — the fixture no " +
			"longer reproduces the header-only state this test is about")
	}
	if len(res.Added) != 0 {
		t.Errorf("res.Added = %v (%d ids), want none. The previous floor recorded no "+
			"ids, so every id in the catalog is trivially an 'addition' against it — "+
			"exactly the noise the suppression exists to remove, and on the real "+
			"corpus a %d-entry delta line. The suppression must key on 'the previous "+
			"floor recorded no ids', not on 'the floor file did not exist'",
			res.Added, len(res.Added), res.Total)
	}
	// The regeneration must still do its real job: the disarmed floor is
	// replaced by one that actually protects the catalog.
	if got, want := hxc305FloorIDs(t, dir), []string{"A1", "A2", "A3"}; !slices.Equal(got, want) {
		t.Errorf("floor ids = %v, want %v — suppressing the report must not suppress "+
			"the rearming", got, want)
	}
	if _, err := loadDirVerbose(dir, true); err != nil {
		t.Errorf("the floor just regenerated does not satisfy the enforced scan: %v", err)
	}
	hxc305NoTempResidue(t, dir)
}

// TestHXC305_RegenFloorRecordsExcludedIDs pins the accounting CONTRACT
// — a floor records every LOADED id plus every id reported as an
// excluded cross-bank duplicate, mirroring checkBankIDFloor exactly —
// and stands as the tripwire for the invariant that currently makes
// mutation R4 (dropping the Declined/ExcludedIDs union) inert.
//
// HONEST BOUNDARY (§11.4.6). This test does NOT kill R4, and must not
// be reported as doing so. insertBank only excludes an id that an
// earlier bank already claimed, and in any successfully-returned
// LoadDirResult that bank is in banks — so `excluded` ⊆ loaded and the
// union changes nothing. (The one path that claims an id without
// appending its bank, the plain-vs-plain collision at
// loader.go:1086-1089, aborts the scan: the caller gets `nil, err`, not
// a result, so the relation holds for every observable result.)
// Measured, not assumed:
// this fixture, its every-case-excluded variant, and the real banks/
// corpus (50 declined entries, 2 excluded ids) all yield
// excluded-not-loaded = ∅.
//
// The subset assertion below is therefore the useful half: if a future
// change lets an excluded id be genuinely absent from the catalog, the
// union at loader.go:885-889 becomes load-bearing, this test fails,
// and its message says a real kill test is now owed.
func TestHXC305_RegenFloorRecordsExcludedIDs(t *testing.T) {
	dir := t.TempDir()
	// A twin pair claims X first (directory order is by name), then an
	// unrelated plain bank declares X and Y: X is excluded as a genuine
	// cross-bank duplicate and reported, Y loads.
	hxc305Write(t, filepath.Join(dir, "aaa.yaml"), hxc305YAMLBank("aaa", "X"))
	hxc305Write(t, filepath.Join(dir, "aaa.json"), hxc305JSONBank("aaa", "X"))
	hxc305Write(t, filepath.Join(dir, "zzz.yaml"), hxc305YAMLBank("zzz", "X", "Y"))

	res, err := loadDirVerbose(dir, false)
	if err != nil {
		t.Fatalf("scan fixture: %v", err)
	}

	loaded := map[string]bool{}
	for _, bf := range res.Banks {
		for _, tc := range bf.TestCases {
			if tc.ID != "" {
				loaded[tc.ID] = true
			}
		}
	}
	var excluded []string
	for _, d := range res.Declined {
		excluded = append(excluded, d.ExcludedIDs...)
	}
	sort.Strings(excluded)
	if len(excluded) == 0 {
		t.Fatalf("no excluded cross-bank duplicate id was reported for a fixture "+
			"built to produce one (declined entries: %d). Either the fixture no "+
			"longer triggers the collision, or DeclinedFile.ExcludedIDs has "+
			"stopped being populated — and an unreported exclusion is exactly the "+
			"silent loss the floor exists to catch, since checkBankIDFloor treats "+
			"'excluded' as the one legitimate way for a case to be absent",
			len(res.Declined))
	}

	// The tripwire. While this holds, R4 is inert; when it stops
	// holding, the union becomes the only thing keeping an excluded id
	// in the floor.
	for _, id := range excluded {
		if !loaded[id] {
			t.Errorf("INVARIANT CHANGED: excluded id %q is absent from the loaded "+
				"catalog. The Declined/ExcludedIDs union in RegenerateBankIDFloor "+
				"(loader.go:885-889) is now LOAD-BEARING — dropping it would "+
				"silently strip %q from every regenerated floor — and it now owes "+
				"a §1.1 kill test of its own, which this file deliberately does "+
				"not claim to provide", id, id)
		}
	}

	if _, err := RegenerateBankIDFloor(dir); err != nil {
		t.Fatalf("regenerate: %v", err)
	}

	// Contract: every excluded id is recorded by the floor.
	got := hxc305FloorIDs(t, dir)
	recorded := map[string]bool{}
	for _, id := range got {
		recorded[id] = true
	}
	for _, id := range excluded {
		if !recorded[id] {
			t.Errorf("excluded id %q is missing from the regenerated floor %v — a "+
				"floor must record every id checkBankIDFloor would accept, or the "+
				"next scan reports a loss that never happened", id, got)
		}
	}
	for id := range loaded {
		if !recorded[id] {
			t.Errorf("loaded id %q is missing from the regenerated floor %v", id, got)
		}
	}

	// The regenerated floor must satisfy the enforced scan — the
	// round-trip that proves the two accountings agree in practice.
	if _, err := loadDirVerbose(dir, true); err != nil {
		t.Errorf("enforced scan rejects the floor just regenerated from the same "+
			"directory: %v — RegenerateBankIDFloor and checkBankIDFloor disagree "+
			"about what counts as present", err)
	}
	hxc305NoTempResidue(t, dir)
}
