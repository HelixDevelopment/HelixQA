// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package detector

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/config"
)

// checkAndroid performs Android-specific crash and ANR detection
// using ADB commands. It checks:
// 1. Whether the app is in the foreground
// 2. Whether the app process is alive
// 3. AndroidRuntime fatal errors in logcat
// 4. ANR events in logcat
// 5. Takes a screenshot as evidence
func (d *Detector) checkAndroid(
	ctx context.Context,
) (*DetectionResult, error) {
	result := &DetectionResult{
		Platform:  config.PlatformAndroid,
		Timestamp: time.Now(),
	}

	// CRITICAL: First check if app is in foreground
	inForeground, err := d.isAppInForeground(ctx)
	if err != nil {
		result.Error = fmt.Sprintf(
			"failed to check foreground: %v", err,
		)
		return result, nil
	}

	// If app is not in foreground, this is a critical failure
	// Tests cannot pass if the app isn't even running
	if !inForeground {
		result.HasCrash = true
		result.LogEntries = append(
			result.LogEntries,
			fmt.Sprintf("APP NOT IN FOREGROUND: %s is not the current activity", d.packageName),
			"TEST INVALID: Screenshots are of launcher/home, not the app",
		)
		// Take screenshot to prove what's actually showing
		screenshotPath, _ := d.takeAndroidScreenshot(ctx)
		result.ScreenshotPath = screenshotPath
		return result, nil
	}

	// 1. Check if app process is alive.
	alive, err := d.isAndroidProcessAlive(ctx)
	if err != nil {
		result.Error = fmt.Sprintf(
			"failed to check process: %v", err,
		)
		return result, nil
	}
	result.ProcessAlive = alive

	// 2. Check for crash in logcat.
	crashLogs, crashTrace, err := d.getAndroidCrashLogs(ctx)
	if err != nil {
		result.Error = fmt.Sprintf(
			"failed to read crash logs: %v", err,
		)
		return result, nil
	}
	if len(crashLogs) > 0 {
		result.HasCrash = true
		result.LogEntries = crashLogs
		result.StackTrace = crashTrace
	}

	// 3. Check for ANR.
	anrLogs, err := d.getAndroidANRLogs(ctx)
	if err != nil {
		result.Error = fmt.Sprintf(
			"failed to read ANR logs: %v", err,
		)
		return result, nil
	}
	if len(anrLogs) > 0 {
		result.HasANR = true
		result.LogEntries = append(
			result.LogEntries, anrLogs...,
		)
	}

	// If process is not alive and no crash was explicitly
	// found, flag as crash.
	if !alive && !result.HasCrash {
		result.HasCrash = true
		result.LogEntries = append(
			result.LogEntries,
			"process not alive (possible crash)",
		)
	}

	// 4. Take screenshot as evidence if crash or ANR detected.
	if result.HasCrash || result.HasANR {
		screenshotPath, screenshotErr := d.takeAndroidScreenshot(ctx)
		if screenshotErr == nil {
			result.ScreenshotPath = screenshotPath
		}
	}

	return result, nil
}

// isAndroidProcessAlive checks if the app process is running
// on the Android device.
func (d *Detector) isAndroidProcessAlive(
	ctx context.Context,
) (bool, error) {
	args := d.adbArgs("shell", "pidof", d.packageName)
	output, err := d.cmdRunner.Run(ctx, "adb", args...)
	if err != nil {
		// pidof returns non-zero exit if process not found.
		return false, nil
	}
	return strings.TrimSpace(string(output)) != "", nil
}

// getAndroidCrashLogs retrieves AndroidRuntime fatal errors
// from logcat.
func (d *Detector) getAndroidCrashLogs(
	ctx context.Context,
) ([]string, string, error) {
	args := d.adbArgs(
		"logcat", "-d", "-s", "AndroidRuntime:E",
	)
	output, err := d.cmdRunner.Run(ctx, "adb", args...)
	if err != nil {
		return nil, "", fmt.Errorf("logcat crash: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var crashLines []string
	var traceBuilder strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "FATAL") ||
			strings.Contains(trimmed, "Exception") ||
			strings.Contains(trimmed, d.packageName) {
			crashLines = append(crashLines, trimmed)
			traceBuilder.WriteString(trimmed)
			traceBuilder.WriteString("\n")
		}
	}

	return crashLines, traceBuilder.String(), nil
}

// getAndroidANRLogs checks logcat for ANR events related to
// the configured package.
func (d *Detector) getAndroidANRLogs(
	ctx context.Context,
) ([]string, error) {
	args := d.adbArgs("logcat", "-d")
	output, err := d.cmdRunner.Run(ctx, "adb", args...)
	if err != nil {
		return nil, fmt.Errorf("logcat ANR: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var anrLines []string

	target := fmt.Sprintf("ANR in %s", d.packageName)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, target) ||
			strings.Contains(trimmed, "ANR in") &&
				strings.Contains(trimmed, d.packageName) {
			anrLines = append(anrLines, trimmed)
		}
	}

	return anrLines, nil
}

// takeAndroidScreenshot captures a screenshot from the Android
// device and pulls it to the evidence directory.
func (d *Detector) takeAndroidScreenshot(
	ctx context.Context,
) (string, error) {
	remotePath := "/sdcard/helixqa-check.png"

	// Capture screenshot on device.
	captureArgs := d.adbArgs(
		"shell", "screencap", "-p", remotePath,
	)
	_, err := d.cmdRunner.Run(ctx, "adb", captureArgs...)
	if err != nil {
		return "", fmt.Errorf("screencap: %w", err)
	}

	// Pull to local evidence directory.
	localName := fmt.Sprintf(
		"crash-%d.png", time.Now().UnixMilli(),
	)
	localPath := filepath.Join(d.evidenceDir, localName)

	pullArgs := d.adbArgs("pull", remotePath, localPath)
	_, err = d.cmdRunner.Run(ctx, "adb", pullArgs...)
	if err != nil {
		return "", fmt.Errorf("pull screenshot: %w", err)
	}

	return localPath, nil
}

// isAppInForeground checks if the app is the current foreground activity
// by querying the activity manager's resumed activity.
func (d *Detector) isAppInForeground(ctx context.Context) (bool, error) {
	args := d.adbArgs("shell", "dumpsys", "activity", "activities")
	output, err := d.cmdRunner.Run(ctx, "adb", args...)
	if err != nil {
		return false, fmt.Errorf("dumpsys activity: %w", err)
	}

	// Parse output to find mResumedActivity
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "mResumedActivity") {
			// Check if our package is in the resumed activity
			return strings.Contains(line, d.packageName), nil
		}
	}
	return false, nil
}

// adbArgs prepends the -s device flag if a device is
// configured.
func (d *Detector) adbArgs(args ...string) []string {
	if d.device != "" {
		return append([]string{"-s", d.device}, args...)
	}
	return args
}
