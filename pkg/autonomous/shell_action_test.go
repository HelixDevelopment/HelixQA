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
