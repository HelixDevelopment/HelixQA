// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	automation "digital.vasic.helixqa/pkg/nexus/automation"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// TestParseReplayScript_RoundTrip verifies that every ActionKind that
// BuildReplayScript serialises can be recovered identically by
// ParseReplayScript.
func TestParseReplayScript_RoundTrip(t *testing.T) {
	actions := []automation.Action{
		{
			Kind: automation.ActionClick,
			At:   contracts.Point{X: 10, Y: 20},
		},
		{
			Kind: automation.ActionType,
			Text: "hello world",
		},
		{
			Kind: automation.ActionScroll,
			At:   contracts.Point{X: 100, Y: 200},
			DX:   0,
			DY:   -10,
		},
		{
			Kind: automation.ActionKey,
			Key:  contracts.KeyEnter,
		},
		{
			Kind: automation.ActionDrag,
			At:   contracts.Point{X: 10, Y: 20},
			To:   contracts.Point{X: 50, Y: 80},
		},
		{
			Kind: automation.ActionCapture,
		},
		{
			Kind: automation.ActionAnalyze,
		},
		{
			Kind:       automation.ActionRecordClip,
			ClipAround: 1713400000000000000,
			ClipWindow: 5000000000,
		},
	}

	dsl := BuildReplayScript(actions)
	require.NotEmpty(t, dsl, "BuildReplayScript must produce non-empty output")

	got, warnings, err := ParseReplayScript([]byte(dsl))
	require.NoError(t, err)
	assert.Empty(t, warnings, "round-trip must produce no warnings")
	require.Len(t, got, len(actions),
		"parsed action count must match input count")

	for i, want := range actions {
		g := got[i]
		assert.Equal(t, want.Kind, g.Kind, "action[%d] Kind", i)
		switch want.Kind {
		case automation.ActionClick:
			assert.Equal(t, want.At, g.At, "action[%d] At", i)
		case automation.ActionType:
			assert.Equal(t, want.Text, g.Text, "action[%d] Text", i)
		case automation.ActionScroll:
			assert.Equal(t, want.At, g.At, "action[%d] At", i)
			assert.Equal(t, want.DX, g.DX, "action[%d] DX", i)
			assert.Equal(t, want.DY, g.DY, "action[%d] DY", i)
		case automation.ActionKey:
			assert.Equal(t, want.Key, g.Key, "action[%d] Key", i)
		case automation.ActionDrag:
			assert.Equal(t, want.At, g.At, "action[%d] At", i)
			assert.Equal(t, want.To, g.To, "action[%d] To", i)
		case automation.ActionCapture, automation.ActionAnalyze:
			// no extra fields to compare
		case automation.ActionRecordClip:
			assert.Equal(t, want.ClipAround, g.ClipAround, "action[%d] ClipAround", i)
			assert.Equal(t, want.ClipWindow, g.ClipWindow, "action[%d] ClipWindow", i)
		}
	}
}

// TestParseReplayScript_TextWithSpaces verifies that text values
// containing spaces survive the round-trip.
func TestParseReplayScript_TextWithSpaces(t *testing.T) {
	actions := []automation.Action{
		{Kind: automation.ActionType, Text: "admin@example.com"},
		{Kind: automation.ActionType, Text: "hello world with spaces"},
		{Kind: automation.ActionType, Text: "tab\there"},
	}
	dsl := BuildReplayScript(actions)
	got, warnings, err := ParseReplayScript([]byte(dsl))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.Len(t, got, len(actions))
	for i, want := range actions {
		assert.Equal(t, want.Text, got[i].Text, "action[%d] Text", i)
	}
}

// TestParseReplayScript_MalformedLines verifies that malformed lines
// produce warnings but do not abort parsing of subsequent valid lines.
func TestParseReplayScript_MalformedLines(t *testing.T) {
	input := strings.Join([]string{
		"click:at=10,20",
		"BOGUS_KIND:at=notapair",        // bad pair
		"scroll:at=nope,nope:dx=0:dy=0", // bad pair coords
		"key:key=enter",
	}, "\n")

	got, warnings, err := ParseReplayScript([]byte(input))
	require.NoError(t, err)
	assert.NotEmpty(t, warnings, "malformed lines must produce warnings")
	// Only click and key survive; the two bad lines are skipped.
	require.Len(t, got, 2)
	assert.Equal(t, automation.ActionClick, got[0].Kind)
	assert.Equal(t, automation.ActionKey, got[1].Kind)
}

// TestParseReplayScript_Empty verifies that empty input returns an
// empty slice without error.
func TestParseReplayScript_Empty(t *testing.T) {
	got, warnings, err := ParseReplayScript([]byte(""))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	assert.Empty(t, got)
}

// TestParseReplayScript_BlankAndCommentLines verifies that blank lines
// and lines beginning with '#' are silently ignored.
func TestParseReplayScript_BlankAndCommentLines(t *testing.T) {
	input := `
# this is a comment
click:at=5,10

# another comment
capture
`
	got, warnings, err := ParseReplayScript([]byte(input))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.Len(t, got, 2)
	assert.Equal(t, automation.ActionClick, got[0].Kind)
	assert.Equal(t, automation.ActionCapture, got[1].Kind)
}

// TestSplitKVSegments_QuotedColon verifies that a colon inside a
// double-quoted value is not treated as a separator.
func TestSplitKVSegments_QuotedColon(t *testing.T) {
	// text="hello:world" should be a single segment
	segs := splitKVSegments(`text="hello:world"`)
	require.Len(t, segs, 1)
	assert.Equal(t, `text="hello:world"`, segs[0])
}

// TestParsePair_Valid checks the happy path.
func TestParsePair_Valid(t *testing.T) {
	x, y, err := parsePair("42,99")
	require.NoError(t, err)
	assert.Equal(t, 42, x)
	assert.Equal(t, 99, y)
}

// TestParsePair_MissingComma verifies that missing comma returns an error.
func TestParsePair_MissingComma(t *testing.T) {
	_, _, err := parsePair("4299")
	assert.Error(t, err)
}

// TestParsePair_NonNumeric verifies that non-numeric coords return an error.
func TestParsePair_NonNumeric(t *testing.T) {
	_, _, err := parsePair("a,b")
	assert.Error(t, err)
}
