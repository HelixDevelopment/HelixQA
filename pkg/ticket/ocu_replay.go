// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ticket

import (
	"fmt"
	"strings"

	automation "digital.vasic.helixqa/pkg/nexus/automation"
)

// BuildReplayScript emits a simple line-per-action text in the
// .ocu-replay DSL that the `helixqa replay <ticket>` subcommand can
// consume to reproduce a failure deterministically.
//
// Format: "<kind>:<field1>=<val1>:<field2>=<val2>"
//
// Each line encodes exactly one Action. Fields not relevant to a
// given ActionKind are omitted. String values that may contain spaces
// are quoted with Go's %q verb so the parser can round-trip them.
//
// Example output:
//
//	click:at=10,20
//	type:text="hello world"
//	scroll:at=100,200:dx=0:dy=-10
//	key:key=Return
//	drag:from=10,20:to=50,80
//	capture
//	analyze
//	record_clip:around=1713400000000000000:window=5000000000
//
// An empty actions slice produces an empty string (no trailing
// newline). The function never returns an error; callers store the
// result as an Evidence attachment with Kind=EvidenceKindReplayScript.
func BuildReplayScript(actions []automation.Action) string {
	if len(actions) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, a := range actions {
		sb.WriteString(string(a.Kind))
		switch a.Kind {
		case automation.ActionClick:
			fmt.Fprintf(&sb, ":at=%d,%d", a.At.X, a.At.Y)

		case automation.ActionType:
			fmt.Fprintf(&sb, ":text=%q", a.Text)

		case automation.ActionScroll:
			fmt.Fprintf(&sb, ":at=%d,%d:dx=%d:dy=%d",
				a.At.X, a.At.Y, a.DX, a.DY)

		case automation.ActionKey:
			fmt.Fprintf(&sb, ":key=%s", string(a.Key))

		case automation.ActionDrag:
			fmt.Fprintf(&sb, ":from=%d,%d:to=%d,%d",
				a.At.X, a.At.Y, a.To.X, a.To.Y)

		case automation.ActionCapture:
			// no extra fields

		case automation.ActionAnalyze:
			// no extra fields

		case automation.ActionRecordClip:
			fmt.Fprintf(&sb, ":around=%d:window=%d",
				a.ClipAround, a.ClipWindow)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
