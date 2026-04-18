// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"testing"

	"github.com/stretchr/testify/assert"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

func TestActionKind_Constants(t *testing.T) {
	cases := []struct {
		kind ActionKind
		want string
	}{
		{ActionClick, "click"},
		{ActionType, "type"},
		{ActionScroll, "scroll"},
		{ActionKey, "key"},
		{ActionDrag, "drag"},
		{ActionCapture, "capture"},
		{ActionAnalyze, "analyze"},
		{ActionRecordClip, "record_clip"},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			assert.Equal(t, tc.want, string(tc.kind))
		})
	}
}

func TestAction_ZeroValue_Safe(t *testing.T) {
	// A zero-value Action must not panic on field access.
	var a Action
	assert.Equal(t, ActionKind(""), a.Kind)
	assert.Equal(t, contracts.Point{}, a.At)
	assert.Equal(t, contracts.Point{}, a.To)
	assert.Equal(t, "", a.Text)
	assert.Equal(t, contracts.KeyCode(""), a.Key)
	assert.Equal(t, contracts.MouseButton(0), a.Button)
	assert.Equal(t, 0, a.DX)
	assert.Equal(t, 0, a.DY)
	assert.Equal(t, int64(0), a.ClipAround)
	assert.Equal(t, int64(0), a.ClipWindow)
	assert.Equal(t, "", a.Expected)
}

func TestAction_FieldAssignment(t *testing.T) {
	a := Action{
		Kind:       ActionDrag,
		At:         contracts.Point{X: 10, Y: 20},
		To:         contracts.Point{X: 300, Y: 400},
		Button:     contracts.ClickRight,
		Expected:   "element moved",
		ClipAround: 1234567890,
		ClipWindow: 5000000000,
	}
	assert.Equal(t, ActionDrag, a.Kind)
	assert.Equal(t, 10, a.At.X)
	assert.Equal(t, 20, a.At.Y)
	assert.Equal(t, 300, a.To.X)
	assert.Equal(t, 400, a.To.Y)
	assert.Equal(t, contracts.ClickRight, a.Button)
	assert.Equal(t, "element moved", a.Expected)
	assert.Equal(t, int64(1234567890), a.ClipAround)
	assert.Equal(t, int64(5000000000), a.ClipWindow)
}

func TestAction_ScrollDeltas(t *testing.T) {
	a := Action{Kind: ActionScroll, At: contracts.Point{X: 50, Y: 50}, DX: -3, DY: 5}
	assert.Equal(t, -3, a.DX)
	assert.Equal(t, 5, a.DY)
}

func TestAction_KeyCode(t *testing.T) {
	a := Action{Kind: ActionKey, Key: contracts.KeyEnter}
	assert.Equal(t, contracts.KeyEnter, a.Key)
}
