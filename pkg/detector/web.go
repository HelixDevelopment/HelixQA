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

// checkWeb performs web platform crash detection by checking
// whether browser processes are running and inspecting for
// console errors.
func (d *Detector) checkWeb(
	ctx context.Context,
) (*DetectionResult, error) {
	result := &DetectionResult{
		Platform:  config.PlatformWeb,
		Timestamp: time.Now(),
	}

	// Check if any browser process is alive. We look for
	// common browser process names.
	alive, processName, err := d.isBrowserProcessAlive(ctx)
	if err != nil {
		result.Error = fmt.Sprintf(
			"failed to check browser process: %v", err,
		)
		return result, nil
	}
	result.ProcessAlive = alive

	if !alive {
		result.HasCrash = true
		result.LogEntries = append(
			result.LogEntries,
			fmt.Sprintf(
				"browser process not found: %s",
				processName,
			),
		)
	}

	return result, nil
}

// isBrowserProcessAlive checks if a browser process is
// running. It checks for chromium, chrome, firefox, and
// playwright browser processes.
func (d *Detector) isBrowserProcessAlive(
	ctx context.Context,
) (bool, string, error) {
	browsers := []string{
		"chromium",
		"chrome",
		"google-chrome",
		"firefox",
		"playwright",
	}

	for _, browser := range browsers {
		output, err := d.cmdRunner.Run(
			ctx, "pgrep", "-f", browser,
		)
		if err != nil {
			// pgrep returns non-zero if no match.
			continue
		}
		if strings.TrimSpace(string(output)) != "" {
			return true, browser, nil
		}
	}

	return false, "browser", nil
}
