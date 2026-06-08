// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"digital.vasic.helixqa/pkg/conduit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// --- Schema parse: the four new dispatch fields round-trip from YAML.

func TestSchema_DispatchFields_ParseFromYAML(t *testing.T) {
	const doc = `
version: "1.0"
name: dispatch-fixture
description: "Fixture exercising challenge_id / dispatches_to / domains / required_evidence"
test_cases:
  - id: atm999.audio.matrix
    name: "ATM-999 audio output matrix"
    category: functional
    priority: critical
    platforms: [android]
    challenge_id: atm999.audio.matrix
    dispatches_to: "device/rockchip/rk3588/tests/v999/test_atm999_audio.sh"
    domains: [audio, hdmi]
    required_evidence:
      - "arvus-codec-5-1.json"
      - "hw-params-channels.txt"
    requires_env: [ARVUS_HOST]
    steps:
      - name: dispatch
        action: "description: dispatched script is the body"
`
	var bf BankFile
	require.NoError(t, yaml.Unmarshal([]byte(doc), &bf))
	require.Len(t, bf.TestCases, 1)
	tc := bf.TestCases[0]

	assert.Equal(t, "atm999.audio.matrix", tc.ChallengeID)
	assert.Equal(t, "device/rockchip/rk3588/tests/v999/test_atm999_audio.sh", tc.DispatchesTo)
	assert.Equal(t, []string{"audio", "hdmi"}, tc.Domains)
	assert.Equal(t, []string{"arvus-codec-5-1.json", "hw-params-channels.txt"}, tc.RequiredEvidence)
	// Backward-compat: existing fields still parse.
	assert.Equal(t, []string{"ARVUS_HOST"}, tc.RequiresEnv)
	assert.Equal(t, "atm999.audio.matrix", tc.EffectiveChallengeID())
	assert.True(t, tc.HasDomain("AUDIO"), "HasDomain is case-insensitive")
	assert.False(t, tc.HasDomain("video"))
}

// TestSchema_DispatchFields_OmittedAreInert proves the fields are
// backward-compatible: a bank case that omits them parses fine and
// EffectiveChallengeID falls back to ID.
func TestSchema_DispatchFields_OmittedAreInert(t *testing.T) {
	const doc = `
version: "1.0"
name: legacy
test_cases:
  - id: legacy-case
    name: "legacy case without dispatch metadata"
    category: functional
`
	var bf BankFile
	require.NoError(t, yaml.Unmarshal([]byte(doc), &bf))
	require.Len(t, bf.TestCases, 1)
	tc := bf.TestCases[0]
	assert.Empty(t, tc.ChallengeID)
	assert.Empty(t, tc.DispatchesTo)
	assert.Empty(t, tc.Domains)
	assert.Empty(t, tc.RequiredEvidence)
	assert.Equal(t, "legacy-case", tc.EffectiveChallengeID())
}

// --- Evidence-ledger gate: present vs missing.

// writeFile creates a non-empty file under dir and returns its path.
func writeNonEmpty(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte("evidence-bytes"), 0o600))
	return p
}

func TestGlobEvidenceResolver_PresentAndMissing(t *testing.T) {
	dir := t.TempDir()
	writeNonEmpty(t, dir, "arvus-codec-5-1.json")
	// An empty file must NOT count as evidence (anti-bluff: 0-byte mp4).
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty.json"), nil, 0o600))

	r := GlobEvidenceResolver{BaseDir: dir}

	got, err := r.Resolve("arvus-codec-5-1.json")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, filepath.Join(dir, "arvus-codec-5-1.json"), got[0])

	// Missing token resolves to no paths.
	got, err = r.Resolve("does-not-exist.json")
	require.NoError(t, err)
	assert.Empty(t, got)

	// Empty file is rejected (present on disk but 0 bytes).
	got, err = r.Resolve("empty.json")
	require.NoError(t, err)
	assert.Empty(t, got, "0-byte artefact must not satisfy the evidence ledger")
}

func TestDispatcher_EvidenceLedger_PassWhenAllPresent(t *testing.T) {
	dir := t.TempDir()
	writeNonEmpty(t, dir, "a.json")
	writeNonEmpty(t, dir, "b.json")

	d := &Dispatcher{Evidence: GlobEvidenceResolver{BaseDir: dir}}
	tc := &TestCase{
		ID:               "evidence-pass",
		RequiredEvidence: []string{"a.json", "b.json"},
	}
	res := d.Run(context.Background(), tc)
	assert.Equal(t, conduit.VerdictPass, res.Verdict)
	assert.Empty(t, res.MissingEvidence)
	assert.Len(t, res.EvidencePaths, 2)
}

func TestDispatcher_EvidenceLedger_FailWhenAnyMissing(t *testing.T) {
	dir := t.TempDir()
	writeNonEmpty(t, dir, "a.json")
	// b.json deliberately absent.

	d := &Dispatcher{Evidence: GlobEvidenceResolver{BaseDir: dir}}
	tc := &TestCase{
		ID:               "evidence-fail",
		RequiredEvidence: []string{"a.json", "b.json", "c.json"},
	}
	res := d.Run(context.Background(), tc)
	assert.Equal(t, conduit.VerdictFail, res.Verdict)
	assert.Equal(t, []string{"b.json", "c.json"}, res.MissingEvidence, "missing list is sorted")
	assert.Contains(t, res.Reason, "missing_required_evidence")
}

func TestDispatcher_EvidenceLedger_NoResolverFailsAll(t *testing.T) {
	// No EvidenceResolver injected → cannot satisfy any token.
	d := &Dispatcher{}
	tc := &TestCase{ID: "no-resolver", RequiredEvidence: []string{"x"}}
	res := d.Run(context.Background(), tc)
	assert.Equal(t, conduit.VerdictFail, res.Verdict)
	assert.Equal(t, []string{"x"}, res.MissingEvidence)
}

// --- Dispatch executor with a fake exec hook.

// fakeExec is a consumer-injected DeviceExec used only in tests.
type fakeExec struct {
	gotCommand string
	output     string
	exitCode   int
	err        error
}

func (f *fakeExec) Exec(_ context.Context, command string) (string, int, error) {
	f.gotCommand = command
	return f.output, f.exitCode, f.err
}

func TestDispatcher_RunsScript_PassOnZeroExit(t *testing.T) {
	dir := t.TempDir()
	writeNonEmpty(t, dir, "proof.json")

	fe := &fakeExec{output: "OK", exitCode: 0}
	d := &Dispatcher{Exec: fe, Evidence: GlobEvidenceResolver{BaseDir: dir}}
	tc := &TestCase{
		ID:               "dispatch-pass",
		DispatchesTo:     "tests/test_atm999.sh",
		RequiredEvidence: []string{"proof.json"},
	}
	res := d.Run(context.Background(), tc)
	assert.Equal(t, conduit.VerdictPass, res.Verdict)
	assert.Equal(t, "tests/test_atm999.sh", fe.gotCommand, "consumer-injected exec receives the DispatchesTo path verbatim")
	assert.Equal(t, 0, res.ExitCode)
	assert.Equal(t, "OK", res.Output)
}

func TestDispatcher_RunsScript_FailOnNonZeroExit(t *testing.T) {
	fe := &fakeExec{output: "boom", exitCode: 7}
	d := &Dispatcher{Exec: fe}
	tc := &TestCase{ID: "dispatch-fail", DispatchesTo: "tests/test_bad.sh"}
	res := d.Run(context.Background(), tc)
	assert.Equal(t, conduit.VerdictFail, res.Verdict)
	assert.Equal(t, 7, res.ExitCode)
	assert.Equal(t, "dispatch_exit_7", res.Reason)
}

func TestDispatcher_ScriptZeroExitButMissingEvidence_StillFails(t *testing.T) {
	// The whole point of the ledger: a green script does NOT excuse
	// absent evidence (§11.4.69 hole this gate closes).
	dir := t.TempDir() // empty: required evidence absent
	fe := &fakeExec{output: "OK", exitCode: 0}
	d := &Dispatcher{Exec: fe, Evidence: GlobEvidenceResolver{BaseDir: dir}}
	tc := &TestCase{
		ID:               "green-script-no-evidence",
		DispatchesTo:     "tests/test_atm999.sh",
		RequiredEvidence: []string{"arvus-codec.json"},
	}
	res := d.Run(context.Background(), tc)
	assert.Equal(t, conduit.VerdictFail, res.Verdict)
	assert.Equal(t, []string{"arvus-codec.json"}, res.MissingEvidence)
}

func TestDispatcher_ExecError_Fails(t *testing.T) {
	fe := &fakeExec{err: errors.New("spawn failed")}
	d := &Dispatcher{Exec: fe}
	tc := &TestCase{ID: "exec-err", DispatchesTo: "tests/x.sh"}
	res := d.Run(context.Background(), tc)
	assert.Equal(t, conduit.VerdictFail, res.Verdict)
	assert.Contains(t, res.Reason, "dispatch_exec_error")
}

func TestDispatcher_NoExecInjected_SkipsHonestly(t *testing.T) {
	d := &Dispatcher{} // no Exec
	tc := &TestCase{ID: "no-exec", DispatchesTo: "tests/x.sh"}
	res := d.Run(context.Background(), tc)
	assert.Equal(t, conduit.VerdictSkip, res.Verdict)
	assert.Equal(t, "no_device_exec_injected", res.Reason)
}

func TestDispatcher_NoDispatchMetadata_SkipsToOrdinaryExecutor(t *testing.T) {
	d := &Dispatcher{}
	tc := &TestCase{ID: "plain"} // no DispatchesTo, no RequiredEvidence
	res := d.Run(context.Background(), tc)
	assert.Equal(t, conduit.VerdictSkip, res.Verdict)
	assert.Equal(t, "no_dispatch_metadata", res.Reason)
}

// --- Conduit integration: a verdict + evidence events reach the sink.

// recordSink is a conduit.Sink that records emitted events for assertion.
type recordSink struct{ events []conduit.Event }

func (r *recordSink) Emit(ev conduit.Event) { r.events = append(r.events, ev) }
func (r *recordSink) Err() error            { return nil }
func (r *recordSink) Close() error          { return nil }

func TestDispatcher_EmitsConduitVerdictAndEvidence(t *testing.T) {
	dir := t.TempDir()
	writeNonEmpty(t, dir, "p.json")
	sink := &recordSink{}
	fe := &fakeExec{exitCode: 0}
	d := &Dispatcher{
		Exec:     fe,
		Evidence: GlobEvidenceResolver{BaseDir: dir},
		Conduit:  sink,
	}
	tc := &TestCase{
		ID:               "cond",
		ChallengeID:      "atm999.audio",
		DispatchesTo:     "tests/x.sh",
		RequiredEvidence: []string{"p.json"},
	}
	res := d.Run(context.Background(), tc)
	require.Equal(t, conduit.VerdictPass, res.Verdict)

	var sawStart, sawEvidence, sawVerdict bool
	for _, ev := range sink.events {
		switch ev.Type {
		case conduit.EventChallengeStart:
			sawStart = true
			assert.Equal(t, "atm999.audio", ev.Challenge)
		case conduit.EventEvidenceCaptured:
			sawEvidence = true
			assert.NotEmpty(t, ev.EvidencePath)
		case conduit.EventChallengeVerdict:
			sawVerdict = true
			assert.Equal(t, conduit.VerdictPass, ev.Verdict)
		}
	}
	assert.True(t, sawStart, "challenge_start emitted")
	assert.True(t, sawEvidence, "evidence_captured emitted with a real path")
	assert.True(t, sawVerdict, "challenge_verdict emitted")
}

// --- Domain filter for domain-scoped dispatch.

func TestFilterByDomain(t *testing.T) {
	cases := []TestCase{
		{ID: "a", Domains: []string{"audio"}},
		{ID: "v", Domains: []string{"video"}},
		{ID: "av", Domains: []string{"audio", "video"}},
		{ID: "none"},
	}
	audio := FilterByDomain(cases, "AUDIO")
	ids := func(cs []TestCase) []string {
		out := make([]string, len(cs))
		for i := range cs {
			out[i] = cs[i].ID
		}
		return out
	}
	assert.Equal(t, []string{"a", "av"}, ids(audio))
	assert.Equal(t, 4, len(FilterByDomain(cases, "")), "empty domain returns all")
}
