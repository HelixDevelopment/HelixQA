// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	automation "digital.vasic.helixqa/pkg/nexus/automation"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// sampleTicketMD produces a minimal ticket markdown containing a
// .ocu-replay fenced block with the given DSL content.
func sampleTicketMD(dsl string) string {
	return "# HELIX-001 — Sample ticket\n\n" +
		"## Replay script\n\n" +
		"```" + ".ocu-replay\n" +
		dsl + "\n" +
		"```\n\n" +
		"## Evidence\n\nSome evidence here.\n"
}

// TestExtractReplayDSL_Present verifies that a ticket with a
// .ocu-replay fenced block returns the correct DSL content.
func TestExtractReplayDSL_Present(t *testing.T) {
	dsl := "click:at=10,20\ntype:text=\"hello\"\ncapture"
	md := sampleTicketMD(dsl)
	got := extractReplayDSL([]byte(md))
	assert.Equal(t, dsl, got,
		"extracted DSL must match the content inside the fenced block")
}

// TestExtractReplayDSL_Absent verifies that a ticket without a
// .ocu-replay block returns an empty string.
func TestExtractReplayDSL_Absent(t *testing.T) {
	md := "# HELIX-002\n\nNo replay block here.\n\n```go\nfoo()\n```\n"
	got := extractReplayDSL([]byte(md))
	assert.Empty(t, got)
}

// TestExtractReplayDSL_MultipleActions verifies multi-line DSL is
// returned intact (trimmed of surrounding whitespace).
func TestExtractReplayDSL_MultipleActions(t *testing.T) {
	dsl := strings.Join([]string{
		"click:at=5,10",
		"key:key=enter",
		"analyze",
	}, "\n")
	md := sampleTicketMD(dsl)
	got := extractReplayDSL([]byte(md))
	assert.Equal(t, dsl, got)
}

// TestRunReplay_DryRun writes a temp ticket file and calls runReplay
// with --ticket pointing at it. Expects exit code 0 and the action
// descriptions printed to stdout.
func TestRunReplay_DryRun(t *testing.T) {
	dsl := "click:at=10,20\ncapture\nkey:key=enter"
	md := sampleTicketMD(dsl)

	tmp := filepath.Join(t.TempDir(), "HELIX-001.md")
	require.NoError(t, os.WriteFile(tmp, []byte(md), 0o600))

	var buf bytes.Buffer
	// Redirect os.Stdout temporarily.
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runReplay([]string{"--ticket", tmp})

	w.Close()
	os.Stdout = origStdout
	buf.ReadFrom(r)

	assert.Equal(t, 0, code, "dry-run must exit 0")
	out := buf.String()
	assert.Contains(t, out, "dry-run")
	assert.Contains(t, out, "click at (10, 20)")
	assert.Contains(t, out, "capture screenshot")
	assert.Contains(t, out, "key")
}

// TestRunReplay_MissingTicket verifies that a non-existent ticket path
// causes runReplay to return a non-zero exit code.
func TestRunReplay_MissingTicket(t *testing.T) {
	code := runReplay([]string{"--ticket", "/does/not/exist/ticket.md"})
	assert.NotEqual(t, 0, code,
		"missing ticket file must produce non-zero exit")
}

// TestRunReplay_NoTicketFlag verifies that omitting --ticket returns 2.
func TestRunReplay_NoTicketFlag(t *testing.T) {
	code := runReplay([]string{})
	assert.Equal(t, 2, code)
}

// TestRunReplay_NoReplayBlock verifies that a ticket with no
// .ocu-replay block returns exit code 1.
func TestRunReplay_NoReplayBlock(t *testing.T) {
	md := "# HELIX-003\n\nNo replay block.\n"
	tmp := filepath.Join(t.TempDir(), "HELIX-003.md")
	require.NoError(t, os.WriteFile(tmp, []byte(md), 0o600))

	code := runReplay([]string{"--ticket", tmp})
	assert.Equal(t, 1, code)
}

// TestDescribeAction covers all ActionKind branches of describeAction.
func TestDescribeAction(t *testing.T) {
	cases := []struct {
		action automation.Action
		want   string
	}{
		{
			automation.Action{Kind: automation.ActionClick, At: contracts.Point{X: 3, Y: 7}},
			"click at (3, 7)",
		},
		{
			automation.Action{Kind: automation.ActionType, Text: "hello"},
			`type "hello"`,
		},
		{
			automation.Action{Kind: automation.ActionScroll, At: contracts.Point{X: 1, Y: 2}, DX: 0, DY: -5},
			"scroll at (1, 2) dx=0 dy=-5",
		},
		{
			automation.Action{Kind: automation.ActionKey, Key: contracts.KeyEnter},
			`key "enter"`,
		},
		{
			automation.Action{Kind: automation.ActionDrag, At: contracts.Point{X: 0, Y: 0}, To: contracts.Point{X: 10, Y: 10}},
			"drag (0, 0) → (10, 10)",
		},
		{
			automation.Action{Kind: automation.ActionCapture},
			"capture screenshot",
		},
		{
			automation.Action{Kind: automation.ActionAnalyze},
			"analyze screenshot via vision pipeline",
		},
		{
			automation.Action{Kind: automation.ActionRecordClip, ClipAround: 123, ClipWindow: 456},
			"record clip around=123 window=456",
		},
	}
	for _, tc := range cases {
		t.Run(string(tc.action.Kind), func(t *testing.T) {
			assert.Equal(t, tc.want, describeAction(tc.action))
		})
	}
}

// TestDescribeAction_LongText verifies that text longer than 40 chars
// is truncated with an ellipsis.
func TestDescribeAction_LongText(t *testing.T) {
	a := automation.Action{
		Kind: automation.ActionType,
		Text: strings.Repeat("x", 50),
	}
	desc := describeAction(a)
	assert.True(t, strings.HasSuffix(desc, `..."`),
		"long text must be truncated with ellipsis inside the quotes")
}

// TestIndexBytes covers edge cases of indexBytes.
func TestIndexBytes(t *testing.T) {
	assert.Equal(t, 0, indexBytes([]byte("abc"), []byte("")))
	assert.Equal(t, -1, indexBytes([]byte("ab"), []byte("abc")))
	assert.Equal(t, 2, indexBytes([]byte("xyzabc"), []byte("zabc")))
	assert.Equal(t, -1, indexBytes([]byte("hello"), []byte("world")))
}
