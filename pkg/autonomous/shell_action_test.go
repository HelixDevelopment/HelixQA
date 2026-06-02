// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/testbank"
)

// TestPerformAction_Shell is the regression for the autonomous-executor
// `shell:` gap: before the fix, performAction had no case for
// ActionTypeShell, so every `shell:` step returned "Unknown action type:
// shell" and the structured executor filed a FALSE-NEGATIVE "Test Case
// Failed" finding — even though the same bank PASSES under `helixqa run`
// (whose executeDesktopShellSteps DOES run shell steps). That is a §107
// anti-bluff defect: a report claiming failure for a command it never ran.
//
// These cases prove the autonomous executor now genuinely runs the host
// command and scores it by the REAL exit code — and that the command's own
// assertion (a grep gate over stdout) actually bites.
func TestPerformAction_Shell(t *testing.T) {
	ste := &StructuredTestExecutor{}
	exec := &noopExecutor{} // the shell path does not use the UI executor
	ctx := context.Background()

	// The action really parses to ActionTypeShell.
	parseStep := testbank.TestStep{Action: "shell: exit 0"}
	at, _ := parseStep.ParseAction()
	require.Equal(t, testbank.ActionTypeShell, at)

	// 1. exit 0 → Success.
	res := ste.performAction(ctx, exec, testbank.TestStep{Action: "shell: exit 0"})
	require.True(t, res.Success, res.Message)

	// 2. non-zero exit → failure, naming the exit code.
	res = ste.performAction(ctx, exec, testbank.TestStep{Action: "shell: exit 7"})
	require.False(t, res.Success)
	require.Contains(t, res.Message, "exited 7")

	// 3. The real Herald pattern: a grep gate over stdout. Correct output
	// passes...
	res = ste.performAction(ctx, exec, testbank.TestStep{
		Action: `shell: printf '{"binary":"pherald"}' | grep -Eq '"binary":"pherald"'`,
	})
	require.True(t, res.Success, res.Message)

	// 4. ...and WRONG output fails (grep finds no match → exit 1). This is
	// the anti-bluff bite: a mislabeled binary forces a real FAIL.
	res = ste.performAction(ctx, exec, testbank.TestStep{
		Action: `shell: printf '{"binary":"WRONG"}' | grep -Eq '"binary":"pherald"'`,
	})
	require.False(t, res.Success)
}

// TestPerformAction_Swipe is the parity-audit regression: `swipe:` was a
// schema-recognized action with NO case in the switch → it fell through to
// "Unknown action type: swipe" (a false-negative). It now routes to the
// executor's Swipe like tap routes to Click.
func TestPerformAction_Swipe(t *testing.T) {
	ste := &StructuredTestExecutor{}
	res := ste.performAction(context.Background(), &noopExecutor{},
		testbank.TestStep{Action: "swipe: 0,0,10,10"})
	require.True(t, res.Success, res.Message)
	require.NotContains(t, res.Message, "Unknown action type")
}

// TestPerformAction_DescriptionSkips: a plain prose description is not
// executable; the autonomous executor must SKIP it (matching the bank-runner
// path) rather than FAIL it (the prior divergent false-negative verdict).
func TestPerformAction_DescriptionSkips(t *testing.T) {
	ste := &StructuredTestExecutor{}
	res := ste.performAction(context.Background(), &noopExecutor{},
		testbank.TestStep{Action: "This is a prose description of expected behaviour"})
	if res.Success || !res.Skipped {
		t.Fatalf("plain description should SKIP (parity with bank runner), got Success=%v Skipped=%v msg=%q",
			res.Success, res.Skipped, res.Message)
	}
}
