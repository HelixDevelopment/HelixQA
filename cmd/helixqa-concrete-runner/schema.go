// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Bank is the top-level concrete-action bank file.
type Bank struct {
	Version  string `yaml:"version"`
	Name     string `yaml:"name"`
	Platform string `yaml:"platform"` // "android" only for now
	Cases    []Case `yaml:"cases"`
}

// Case is a single test case — a sequence of concrete steps with
// a final assertion contract.
type Case struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Steps []Step `yaml:"steps"`
}

// Step is one concrete action against the device. Exactly ONE of the
// action fields must be set per step. The runner enforces this.
type Step struct {
	// Description is the human-readable label for the step. Included
	// in the result JSON for traceability.
	Description string `yaml:"description"`

	// Wait pauses the runner for the specified duration (e.g. "500ms",
	// "2s") before continuing.
	Wait string `yaml:"wait,omitempty"`

	// TapText taps the center of the first UI node whose `text` field
	// equals this string. Fails if no match.
	TapText string `yaml:"tap_text,omitempty"`

	// TapDesc taps the center of the first UI node whose
	// `content-desc` field equals this string. Fails if no match.
	TapDesc string `yaml:"tap_desc,omitempty"`

	// TapXY taps the absolute coordinates "x,y".
	TapXY string `yaml:"tap_xy,omitempty"`

	// TypeText sends keystrokes via `adb shell input text`.
	TypeText string `yaml:"type_text,omitempty"`

	// LaunchActivity starts an activity (form: "pkg/.MainActivity").
	LaunchActivity string `yaml:"launch_activity,omitempty"`

	// ForceStop kills the named package.
	ForceStop string `yaml:"force_stop,omitempty"`

	// AssertTextPresent fails the case if no UI node has this text
	// value within `wait_for` (default 5s).
	AssertTextPresent string `yaml:"assert_text_present,omitempty"`

	// AssertDescPresent — like AssertTextPresent but matches content-desc.
	AssertDescPresent string `yaml:"assert_desc_present,omitempty"`

	// AssertActivityCurrent fails unless dumpsys reports this activity
	// is the currently-focused one.
	AssertActivityCurrent string `yaml:"assert_activity_current,omitempty"`

	// WaitFor overrides the default 5s assertion timeout.
	WaitFor string `yaml:"wait_for,omitempty"`
}

func loadBank(path string) (*Bank, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var b Bank
	if err := yaml.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if b.Platform != "" && b.Platform != "android" {
		return nil, fmt.Errorf("unsupported platform %q (concrete-runner is android-only)", b.Platform)
	}
	if len(b.Cases) == 0 {
		return nil, fmt.Errorf("bank %s contains zero cases", path)
	}
	return &b, nil
}
