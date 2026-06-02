// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package detector

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/config"
)

// TestCheckDesktop_NoTargetConfigured documents the close-out⁷⁵
// contract change: when neither processName nor PID is configured,
// isDesktopProcessAlive returns true unconditionally — the absence
// of a configured target means "no desktop app under test", not
// "crash detected". The prior behavior (fallback to `pgrep -f java`)
// produced 23-of-23 false-positive crash reports against helixqa
// runs that didn't supply -desktop-process.
func TestCheckDesktop_NoTargetConfigured(t *testing.T) {
	d := New(
		config.PlatformDesktop,
		WithCommandRunner(newMockRunner()), // mock unused — no command should fire
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive,
		"no-target-configured must return ProcessAlive=true (no app under test ≠ crash)")
	assert.False(t, result.HasCrash,
		"no-target-configured must NOT flag crash — close-out⁷⁵ contract")
}

func TestCheckDesktop_ProcessDead_RequiresExplicitTarget(t *testing.T) {
	// close-out⁷⁵ update: a "dead process" reading now requires an
	// explicitly configured processName OR PID. Without either, there's
	// no target to be dead. This test exercises the explicit-name path.
	mock := newMockRunner()
	mock.On(
		"pgrep -f myapp",
		[]byte(""),
		fmt.Errorf("no process"),
	)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
		WithProcessName("myapp"),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.False(t, result.ProcessAlive)
	assert.True(t, result.HasCrash)
	assert.Contains(t, result.LogEntries[0],
		"desktop process not alive")
}

func TestCheckDesktop_ByProcessName(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"pgrep -f myapp",
		[]byte("5678"),
		nil,
	)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
		WithProcessName("myapp"),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive)
}

// guaranteedAbsentPID returns a PID that is guaranteed not to map
// to any live process. The classic value 2147483647 (math.MaxInt32)
// is out of any real PID range. This is the exact fixture that
// exposed the LVA-8 bluff: the old `exec kill -0 <pid>` path
// reported this PID as alive on macOS (the /bin/kill binary exits 0
// for it) while syscall.Kill correctly returns ESRCH.
const guaranteedAbsentPID = 2147483647

// TestCheckDesktop_ByPID_Alive exercises the real liveness probe
// against the test process's OWN PID, which is by definition alive.
// LVA-8: the by-PID path now calls syscall.Kill(pid, 0) directly —
// no command runner is involved, so the detector is created without
// a mock and the probe hits the real kernel.
func TestCheckDesktop_ByPID_Alive(t *testing.T) {
	d := New(
		config.PlatformDesktop,
		WithProcessPID(os.Getpid()),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive,
		"the running test process must read as alive")
	assert.False(t, result.HasCrash)
}

// TestCheckDesktop_ByPID_Dead is the LVA-8 regression test. A
// guaranteed-absent PID MUST read as dead → ProcessAlive=false →
// HasCrash=true. With the OLD `exec kill -0 <pid>` logic this test
// FAILED on macOS (the /bin/kill binary exits 0 for the absent PID,
// so ProcessAlive came back true and HasCrash false — the validator
// then reported a crashed step as passed). With the syscall.Kill
// fix it correctly reports the process dead.
//
// FALSIFIABILITY REHEARSAL: reverting checkProcessByPID to
// `d.cmdRunner.Run(ctx, "kill", "-0", pid)` (the historical bug)
// makes this test FAIL on macOS with:
//
//	Error: Should be false / Error: Should be true
//	Test: TestCheckDesktop_ByPID_Dead
//	ProcessAlive was true for a guaranteed-absent PID
//
// because the macOS /bin/kill binary exits 0 for PID 2147483647.
func TestCheckDesktop_ByPID_Dead(t *testing.T) {
	d := New(
		config.PlatformDesktop,
		WithProcessPID(guaranteedAbsentPID),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.False(t, result.ProcessAlive,
		"a guaranteed-absent PID must read as dead (LVA-8 regression)")
	assert.True(t, result.HasCrash,
		"a dead target process must flag a crash (LVA-8 regression)")
}

// TestCheckDesktop_ByPID_DeadOfReapedChild reinforces the regression
// with a process that was genuinely alive and then exited+reaped, so
// its PID is no longer live. This proves the probe distinguishes
// alive from dead for a PID that actually existed (not just an
// out-of-range sentinel).
func TestCheckDesktop_ByPID_DeadOfReapedChild(t *testing.T) {
	cmd := exec.Command("true")
	require.NoError(t, cmd.Start())
	pid := cmd.Process.Pid
	require.NoError(t, cmd.Wait()) // process exits and is reaped here

	d := New(
		config.PlatformDesktop,
		WithProcessPID(pid),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.False(t, result.ProcessAlive,
		"a reaped child PID must read as dead")
	assert.True(t, result.HasCrash)
}

func TestCheckDesktop_PIDTakesPrecedence(t *testing.T) {
	// PID is set to the live test process and a process name that, if
	// consulted, would hit the mock with NO canned response (the mock
	// returns an error for unknown commands). Because the by-PID path
	// takes precedence and uses syscall.Kill directly, the name path
	// (and its mock) must never be invoked — so ProcessAlive must be
	// true even though the mock would error on "pgrep -f should-not-use".
	mock := newMockRunner()

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
		WithProcessName("should-not-use"),
		WithProcessPID(os.Getpid()),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive,
		"by-PID path must short-circuit before the (unmockable) name path")
	assert.False(t, result.HasCrash)
}

// TestCheckDesktop_DefaultBehavior documents that the close-out⁷⁵
// contract no longer falls back to `pgrep -f java`. Default = no
// target = ProcessAlive=true. Use WithProcessName("java") explicitly
// if you actually want java-process detection.
func TestCheckDesktop_DefaultBehavior(t *testing.T) {
	mock := newMockRunner()
	// Mock is intentionally NOT configured for "pgrep -f java" — the
	// detector must NOT invoke any command when no target is set.

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.ProcessAlive,
		"default (no target) returns ProcessAlive=true per close-out⁷⁵")
}

func TestCheckDesktop_PlatformIsDesktop(t *testing.T) {
	mock := newMockRunner()
	mock.On("pgrep", []byte("12345"), nil)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.Equal(t, config.PlatformDesktop, result.Platform)
}

func TestCheckDesktop_CrashMessageContainsPID(t *testing.T) {
	d := New(
		config.PlatformDesktop,
		WithProcessPID(guaranteedAbsentPID),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.HasCrash)
	assert.Contains(t, result.LogEntries[0],
		fmt.Sprintf("PID %d", guaranteedAbsentPID))
}

func TestCheckDesktop_CrashMessageContainsName(t *testing.T) {
	mock := newMockRunner()
	mock.On(
		"pgrep -f myapp",
		[]byte(""),
		fmt.Errorf("not found"),
	)

	d := New(
		config.PlatformDesktop,
		WithCommandRunner(mock),
		WithProcessName("myapp"),
	)

	result, err := d.checkDesktop(context.Background())
	require.NoError(t, err)
	assert.True(t, result.HasCrash)
	assert.Contains(t, result.LogEntries[0], "myapp")
}
