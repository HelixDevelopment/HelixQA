// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package testbank

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"digital.vasic.helixqa/pkg/conduit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Self-validation of the content-asserting evidence ledger (§11.4.107(10)):
// a golden-GOOD artefact (content matches the assertion → satisfied) and a
// golden-BAD artefact (non-empty but WRONG content → NOT satisfied). This
// proves the enforcement itself is not a new bluff: the size-only blind
// spot (`echo stereo > codec.txt` passing) is closed.

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
}

func TestContentAssertingResolver_BackwardCompatPathOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "present.txt", "anything")
	writeFile(t, dir, "empty.txt", "")
	r := ContentAssertingResolver{BaseDir: dir}

	// Pure path token (no " | ") behaves exactly like GlobEvidenceResolver.
	got, err := r.Resolve("present.txt")
	require.NoError(t, err)
	assert.Len(t, got, 1)

	got, err = r.Resolve("empty.txt")
	require.NoError(t, err)
	assert.Empty(t, got, "0-byte artefact never satisfies")

	got, err = r.Resolve("missing.txt")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestContentAssertingResolver_DescriptiveParenStripped(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "codec.txt", "Multi-CH PCM")
	r := ContentAssertingResolver{BaseDir: dir}
	// Legacy descriptive bank token style: "<path> (human description)".
	got, err := r.Resolve("codec.txt (Arvus Codec-In-Use shows 5.1 / Multi-CH PCM)")
	require.NoError(t, err)
	assert.Len(t, got, 1, "descriptive parenthetical must not break globbing")
}

// GAP-3 CORE: a non-empty-but-WRONG artefact must FAIL the ledger.
func TestContentAssertingResolver_AudioCodec_GoodVsBad(t *testing.T) {
	dir := t.TempDir()
	r := ContentAssertingResolver{BaseDir: dir}
	// Bank declares: file must NOT be the empty placeholder N.E. and NOT
	// the downmixed `stereo`, AND must match a multichannel codec.
	tokGood := "codec.txt | not_match:^N\\.E\\.$|stereo | match:5\\.1|Multi-CH"

	// GOLDEN-GOOD: real multichannel reached the sink.
	writeFile(t, dir, "codec.txt", "Multi-CH PCM\n")
	got, err := r.Resolve(tokGood)
	require.NoError(t, err)
	assert.Len(t, got, 1, "golden-good multichannel codec MUST satisfy")

	// GOLDEN-BAD #1: the §FY silent bug — empty placeholder.
	writeFile(t, dir, "codec.txt", "N.E.\n")
	got, err = r.Resolve(tokGood)
	require.NoError(t, err)
	assert.Empty(t, got, "empty Codec-In-Use placeholder MUST FAIL (the §11.4.68 bluff)")

	// GOLDEN-BAD #2: stereo collapse despite a non-empty file.
	writeFile(t, dir, "codec.txt", "stereo\n")
	got, err = r.Resolve(tokGood)
	require.NoError(t, err)
	assert.Empty(t, got, "stereo collapse MUST FAIL even though file is non-empty")
}

func TestContentAssertingResolver_HwParamsChannels(t *testing.T) {
	dir := t.TempDir()
	r := ContentAssertingResolver{BaseDir: dir}
	tok := "hw_params.txt | min_int:channels:6"

	writeFile(t, dir, "hw_params.txt", "format: S16_LE\nchannels: 6\nrate: 48000\n")
	got, err := r.Resolve(tok)
	require.NoError(t, err)
	assert.Len(t, got, 1, "channels:6 satisfies channels>=6")

	// Stereo HAL open — channels:2 must FAIL the >=6 assertion.
	writeFile(t, dir, "hw_params.txt", "channels: 2\nrate: 48000\n")
	got, err = r.Resolve(tok)
	require.NoError(t, err)
	assert.Empty(t, got, "channels:2 MUST FAIL channels>=6 (downmix detection)")
}

// GAP-3 + which-display + ΔE2000 wiring: the video verdict JSON must say
// the content is LIVE, on the EXPECTED display, with ΔE2000 passing.
func TestContentAssertingResolver_VideoVerdict_JSON(t *testing.T) {
	dir := t.TempDir()
	r := ContentAssertingResolver{BaseDir: dir}
	tok := "video_verdict.json | json:video_live==true | json:route==secondary | json:checks.delta_e2000==true"

	// GOLDEN-GOOD: live, on the secondary display, colour-faithful.
	writeFile(t, dir, "video_verdict.json",
		`{"video_live":true,"route":"secondary","checks":{"delta_e2000":true}}`)
	got, err := r.Resolve(tok)
	require.NoError(t, err)
	assert.Len(t, got, 1, "live+secondary+ΔE2000-pass MUST satisfy")

	// GOLDEN-BAD #1: a frozen/stale frame — video_live false.
	writeFile(t, dir, "video_verdict.json",
		`{"video_live":false,"route":"secondary","checks":{"delta_e2000":true}}`)
	got, err = r.Resolve(tok)
	require.NoError(t, err)
	assert.Empty(t, got, "frozen frame (video_live:false) MUST FAIL")

	// GOLDEN-BAD #2: live but on the WRONG display (Bug #13 / route=NONE class).
	writeFile(t, dir, "video_verdict.json",
		`{"video_live":true,"route":"primary","checks":{"delta_e2000":true}}`)
	got, err = r.Resolve(tok)
	require.NoError(t, err)
	assert.Empty(t, got, "video on the WRONG display MUST FAIL (route assertion)")

	// GOLDEN-BAD #3: live, right display, but PALE/desaturated render (ΔE2000 fail).
	writeFile(t, dir, "video_verdict.json",
		`{"video_live":true,"route":"secondary","checks":{"delta_e2000":false}}`)
	got, err = r.Resolve(tok)
	require.NoError(t, err)
	assert.Empty(t, got, "ΔE2000 colour-fidelity failure MUST FAIL the PASS")
}

func TestContentAssertingResolver_JSON_NumericThresholds(t *testing.T) {
	dir := t.TempDir()
	r := ContentAssertingResolver{BaseDir: dir}

	// Captured-audio RMS / channel JSON (GAP A leg).
	writeFile(t, dir, "audio_capture.json",
		`{"rms_dbfs":-12.3,"channels":6,"silent":false}`)
	tok := "audio_capture.json | json:silent==false | json:channels>=6 | json:rms_dbfs>-40"
	got, err := r.Resolve(tok)
	require.NoError(t, err)
	assert.Len(t, got, 1, "captured WAV: not silent, 6ch, above floor MUST satisfy")

	// Silent capture (the screenrecord-has-no-audio class) MUST FAIL.
	writeFile(t, dir, "audio_capture.json",
		`{"rms_dbfs":-90.0,"channels":6,"silent":true}`)
	got, err = r.Resolve(tok)
	require.NoError(t, err)
	assert.Empty(t, got, "silent captured WAV MUST FAIL even if channels match")
}

func TestContentAssertingResolver_MalformedAssertionSurfacesError(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "x.txt", "data")
	r := ContentAssertingResolver{BaseDir: dir}
	_, err := r.Resolve("x.txt | bogus_assertion_kind:1")
	assert.Error(t, err, "an unknown assertion is the bank author's bug — must not silently pass")
}

// End-to-end through the Dispatcher: a Challenge whose dispatched script
// exits 0 but whose evidence content is WRONG must still FAIL (the ledger
// is the second gate, the §11.4.69 hole this resolver closes).
func TestDispatcher_ContentLedger_FailsOnWrongContent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "codec.txt", "stereo\n") // wrong: downmixed
	fe := DeviceExecFunc(func(ctx context.Context, cmd string) (string, int, error) {
		return "ran", 0, nil // script "passes"
	})
	d := &Dispatcher{
		Exec:     fe,
		Evidence: ContentAssertingResolver{BaseDir: dir},
	}
	tc := &TestCase{
		ID:               "atm.audio.multichannel",
		DispatchesTo:     "test_multichannel.sh",
		RequiredEvidence: []string{"codec.txt | not_match:^stereo$ | match:5\\.1|Multi-CH"},
	}
	res := d.Run(context.Background(), tc)
	assert.Equal(t, conduit.VerdictFail, res.Verdict,
		"script exit 0 but stereo-collapse content MUST FAIL the Challenge (no PASS-bluff)")
	assert.NotEmpty(t, res.MissingEvidence)

	// Now make the content correct → PASS.
	writeFile(t, dir, "codec.txt", "Multi-CH PCM\n")
	res = d.Run(context.Background(), tc)
	assert.Equal(t, conduit.VerdictPass, res.Verdict,
		"correct multichannel content → PASS")
}
