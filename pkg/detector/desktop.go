// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package detector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/config"
)

// checkDesktop performs desktop platform crash detection by
// checking whether the JVM/application process is still
// running.
func (d *Detector) checkDesktop(
	ctx context.Context,
) (*DetectionResult, error) {
	result := &DetectionResult{
		Platform:  config.PlatformDesktop,
		Timestamp: time.Now(),
	}

	alive, err := d.isDesktopProcessAlive(ctx)
	if err != nil {
		result.Error = fmt.Sprintf(
			"failed to check desktop process: %v", err,
		)
		return result, nil
	}
	result.ProcessAlive = alive

	if !alive {
		result.HasCrash = true
		target := d.processName
		if d.processPID > 0 {
			target = fmt.Sprintf("PID %d", d.processPID)
		}
		result.LogEntries = append(
			result.LogEntries,
			fmt.Sprintf(
				"desktop process not alive: %s", target,
			),
		)
	}

	return result, nil
}

// isDesktopProcessAlive checks if the configured desktop
// process is running. If a PID is set, it checks by PID.
// Otherwise it checks by process name. If neither is configured,
// returns true unconditionally — per CONST-035 §11.4.3 the absence
// of a configured target means "no desktop app under test", not
// "crash detected on every step".
//
// close-out⁷⁵ fix: previously the default fallback was
// checkProcessByName("java") which spuriously flagged crashes on
// hosts that don't run java (or whose java is unrelated to any
// helixqa-launched app). This produced 23-of-23 false-positive
// crash reports against bank/all-formats.yaml runs that didn't
// configure -desktop-process — the canonical contract-bluff
// pattern per CONST-035 §11.4 ("absence-of-error PASS without
// runtime evidence" inverted: presence-of-process-elsewhere
// FAIL without runtime evidence the process WAS the one under
// test).
func (d *Detector) isDesktopProcessAlive(
	ctx context.Context,
) (bool, error) {
	if d.processPID > 0 {
		return d.checkProcessByPID(ctx, d.processPID)
	}
	if d.processName != "" {
		return d.checkProcessByName(ctx, d.processName)
	}
	// No process target configured → not under test → not a crash.
	return true, nil
}

// checkProcessByPID checks if a process with the given PID
// exists.
func (d *Detector) checkProcessByPID(
	ctx context.Context,
	pid int,
) (bool, error) {
	output, err := d.cmdRunner.Run(
		ctx, "kill", "-0",
		fmt.Sprintf("%d", pid),
	)
	if err != nil {
		// kill -0 returns non-zero if process doesn't exist.
		_ = output
		return false, nil
	}
	return true, nil
}

// checkProcessByName checks if a process with the given name
// is running.
func (d *Detector) checkProcessByName(
	ctx context.Context,
	name string,
) (bool, error) {
	output, err := d.cmdRunner.Run(
		ctx, "pgrep", "-f", name,
	)
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(output)) != "", nil
}
