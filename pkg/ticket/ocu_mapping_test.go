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

// --- FromAutomationResult ---

func TestFromAutomationResult_Empty(t *testing.T) {
	res := automation.Result{}
	ev := FromAutomationResult(res)
	assert.Empty(t, ev)
}

func TestFromAutomationResult_SingleRef(t *testing.T) {
	res := automation.Result{
		Success: true,
		Evidence: []automation.EvidenceRef{
			{Kind: EvidenceKindClip, Ref: "42 bytes"},
		},
	}
	ev := FromAutomationResult(res)
	require.Len(t, ev, 1)
	assert.Equal(t, EvidenceKindClip, ev[0].Kind)
	assert.Equal(t, "42 bytes", ev[0].Ref)
}

func TestFromAutomationResult_MultipleRefs(t *testing.T) {
	res := automation.Result{
		Success: true,
		Evidence: []automation.EvidenceRef{
			{Kind: "screenshot_before", Ref: "seq-1"},
			{Kind: EvidenceKindClip, Ref: "512 bytes"},
			{Kind: EvidenceKindHookTrace, Ref: "trace-0001"},
		},
	}
	ev := FromAutomationResult(res)
	require.Len(t, ev, 3)
	assert.Equal(t, "screenshot_before", ev[0].Kind)
	assert.Equal(t, "seq-1", ev[0].Ref)
	assert.Equal(t, EvidenceKindClip, ev[1].Kind)
	assert.Equal(t, EvidenceKindHookTrace, ev[2].Kind)
}

func TestFromAutomationResult_PreservesOrder(t *testing.T) {
	kinds := []string{
		EvidenceKindClip,
		EvidenceKindDiffOverlay,
		EvidenceKindOCRDump,
		EvidenceKindElementTree,
	}
	refs := make([]automation.EvidenceRef, len(kinds))
	for i, k := range kinds {
		refs[i] = automation.EvidenceRef{Kind: k, Ref: "r" + k}
	}
	ev := FromAutomationResult(automation.Result{Evidence: refs})
	require.Len(t, ev, len(kinds))
	for i, k := range kinds {
		assert.Equal(t, k, ev[i].Kind)
	}
}

// --- BuildReplayScript ---

func TestBuildReplayScript_Empty(t *testing.T) {
	script := BuildReplayScript(nil)
	assert.Empty(t, script)

	script2 := BuildReplayScript([]automation.Action{})
	assert.Empty(t, script2)
}

func TestBuildReplayScript_Click(t *testing.T) {
	actions := []automation.Action{
		{Kind: automation.ActionClick, At: contracts.Point{X: 10, Y: 20}},
	}
	script := BuildReplayScript(actions)
	assert.Equal(t, "click:at=10,20\n", script)
}

func TestBuildReplayScript_Type(t *testing.T) {
	actions := []automation.Action{
		{Kind: automation.ActionType, Text: "hello world"},
	}
	script := BuildReplayScript(actions)
	assert.Equal(t, "type:text=\"hello world\"\n", script)
}

func TestBuildReplayScript_Scroll(t *testing.T) {
	actions := []automation.Action{
		{
			Kind: automation.ActionScroll,
			At:   contracts.Point{X: 100, Y: 200},
			DX:   0,
			DY:   -10,
		},
	}
	script := BuildReplayScript(actions)
	assert.Equal(t, "scroll:at=100,200:dx=0:dy=-10\n", script)
}

func TestBuildReplayScript_Key(t *testing.T) {
	actions := []automation.Action{
		{Kind: automation.ActionKey, Key: contracts.KeyEnter},
	}
	script := BuildReplayScript(actions)
	assert.Equal(t, "key:key=enter\n", script)
}

func TestBuildReplayScript_Drag(t *testing.T) {
	actions := []automation.Action{
		{
			Kind: automation.ActionDrag,
			At:   contracts.Point{X: 10, Y: 20},
			To:   contracts.Point{X: 50, Y: 80},
		},
	}
	script := BuildReplayScript(actions)
	assert.Equal(t, "drag:from=10,20:to=50,80\n", script)
}

func TestBuildReplayScript_Capture(t *testing.T) {
	actions := []automation.Action{
		{Kind: automation.ActionCapture},
	}
	script := BuildReplayScript(actions)
	assert.Equal(t, "capture\n", script)
}

func TestBuildReplayScript_Analyze(t *testing.T) {
	actions := []automation.Action{
		{Kind: automation.ActionAnalyze},
	}
	script := BuildReplayScript(actions)
	assert.Equal(t, "analyze\n", script)
}

func TestBuildReplayScript_RecordClip(t *testing.T) {
	actions := []automation.Action{
		{
			Kind:       automation.ActionRecordClip,
			ClipAround: 1713400000000000000,
			ClipWindow: 5000000000,
		},
	}
	script := BuildReplayScript(actions)
	assert.Equal(
		t,
		"record_clip:around=1713400000000000000:window=5000000000\n",
		script,
	)
}

func TestBuildReplayScript_MultiAction(t *testing.T) {
	actions := []automation.Action{
		{Kind: automation.ActionCapture},
		{Kind: automation.ActionClick, At: contracts.Point{X: 5, Y: 5}},
		{Kind: automation.ActionType, Text: "admin"},
		{Kind: automation.ActionKey, Key: contracts.KeyEnter},
	}
	script := BuildReplayScript(actions)
	lines := strings.Split(strings.TrimSuffix(script, "\n"), "\n")
	require.Len(t, lines, 4)
	assert.Equal(t, "capture", lines[0])
	assert.Equal(t, "click:at=5,5", lines[1])
	assert.Equal(t, `type:text="admin"`, lines[2])
	assert.Equal(t, "key:key=enter", lines[3])
}

func TestBuildReplayScript_TypeWithSpecialChars(t *testing.T) {
	actions := []automation.Action{
		{Kind: automation.ActionType, Text: `say "hello\nworld"`},
	}
	script := BuildReplayScript(actions)
	// %q escapes backslashes and quotes; output must be round-trippable.
	assert.Contains(t, script, "type:text=")
	assert.NotContains(t, script, "\n\n") // newline only at end of line
}
