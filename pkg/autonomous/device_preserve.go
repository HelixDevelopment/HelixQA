// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// devicePreservedSettings captures the Android settings that a
// HelixQA QA session must NEVER leave mutated on a device. This
// addresses a 2026-04-21 user-reported regression: after two
// consecutive sessions on different Android TV devices, both devices
// were left with system font_scale=2.0 (default is 1.0). No HelixQA
// source code explicitly sets font_scale, but the LLM-driven
// curiosity phase navigates via DPAD and can land inside
// Settings → Accessibility → Font Size by accident. To make this
// impossible by CONSTRUCTION, we snapshot every sensitive system
// setting at session start and restore it verbatim at session end.
//
// This is a DEFENSE-IN-DEPTH measure — the LLM should never be
// navigating into device settings in the first place, but when it
// does, the operator must not be left with a polluted device.
type devicePreservedSettings struct {
	device string

	// Captured values (empty string = not set / unknown, leave
	// untouched on restore).
	systemFontScale    string
	secureAccessFont   string // accessibility large text (deprecated but still honored)
	systemScreenOffTO  string
	systemScreenBright string
	systemBrightMode   string // screen_brightness_mode: 0 manual, 1 auto
	globalAutoRotate   string
}

// captureDeviceSettings reads the system/secure/global keys we
// consider sensitive. Missing keys are recorded as empty strings so
// they're left alone on restore.
func captureDeviceSettings(ctx context.Context, device string) (*devicePreservedSettings, error) {
	if device == "" {
		return nil, fmt.Errorf("captureDeviceSettings: empty device")
	}
	dps := &devicePreservedSettings{device: device}

	capture := func(ns, key string) string {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(c, "adb",
			"-s", device, "shell", "settings", "get", ns, key,
		).Output()
		if err != nil {
			return ""
		}
		val := strings.TrimSpace(string(out))
		if val == "null" {
			return ""
		}
		return val
	}

	dps.systemFontScale = capture("system", "font_scale")
	dps.secureAccessFont = capture("secure", "accessibility_font_scaling_has_been_changed")
	dps.systemScreenOffTO = capture("system", "screen_off_timeout")
	dps.systemScreenBright = capture("system", "screen_brightness")
	dps.systemBrightMode = capture("system", "screen_brightness_mode")
	dps.globalAutoRotate = capture("system", "accelerometer_rotation")

	return dps, nil
}

// restore writes each previously-captured non-empty setting back to
// the device. Restoration failures are logged but non-fatal — the
// goal is to clean up as much as possible.
func (dps *devicePreservedSettings) restore(ctx context.Context) {
	if dps == nil || dps.device == "" {
		return
	}

	restoreKey := func(ns, key, want string) {
		if want == "" {
			return
		}
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		// Read current; only write if it differs, so we don't
		// touch devices the session never polluted.
		cur, err := exec.CommandContext(c, "adb",
			"-s", dps.device, "shell", "settings", "get", ns, key,
		).Output()
		if err == nil {
			if strings.TrimSpace(string(cur)) == want {
				return
			}
		}
		if out, err := exec.CommandContext(c, "adb",
			"-s", dps.device, "shell", "settings", "put", ns, key, want,
		).CombinedOutput(); err != nil {
			fmt.Printf(
				"  [device-preserve] restore %s/%s on %s failed: %v (%s)\n",
				ns, key, dps.device, err, strings.TrimSpace(string(out)),
			)
		} else {
			fmt.Printf(
				"  [device-preserve] restored %s/%s=%s on %s\n",
				ns, key, want, dps.device,
			)
		}
	}

	restoreKey("system", "font_scale", dps.systemFontScale)
	restoreKey("secure", "accessibility_font_scaling_has_been_changed", dps.secureAccessFont)
	restoreKey("system", "screen_off_timeout", dps.systemScreenOffTO)
	restoreKey("system", "screen_brightness", dps.systemScreenBright)
	restoreKey("system", "screen_brightness_mode", dps.systemBrightMode)
	restoreKey("system", "accelerometer_rotation", dps.globalAutoRotate)
}
