// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package scrcpy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// CommandRunner runs a command and returns its combined stdout. Matches the
// shape used in pkg/detector/detector.go — callers can reuse their existing
// runner instead of taking a new dependency.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ErrDeviceBlocked is returned by EnforceDevIgnore when the device matches the
// deny-list in .devignore. It is wrapped with the matching device model for
// clarity; callers can test with errors.Is.
var ErrDeviceBlocked = errors.New("scrcpy: device blocked by .devignore")

// LoadDevIgnore reads the .devignore file into a slice of non-empty, trimmed,
// comment-stripped lines. Lines beginning with `#` are skipped. Missing file
// returns an empty slice and a nil error — .devignore is optional but when
// present it is authoritative.
func LoadDevIgnore(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("scrcpy: open %s: %w", path, err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scrcpy: scan %s: %w", path, err)
	}
	return out, nil
}

// MatchModel reports whether model matches any entry in patterns using a
// case-insensitive substring match — identical semantics to the pre-commit
// `grep -qi` used in CLAUDE.md's sample check. Empty patterns never match.
func MatchModel(patterns []string, model string) bool {
	if model == "" {
		return false
	}
	ml := strings.ToLower(model)
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(ml, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// DeviceModel runs `adb -s <serial> shell getprop ro.product.model` via the
// supplied CommandRunner and returns the trimmed model string. An empty serial
// is allowed — adb will fall back to the default device on a single-device
// host — but ambiguous, so production callers should always pass a serial.
func DeviceModel(ctx context.Context, r CommandRunner, serial string) (string, error) {
	if r == nil {
		return "", errors.New("scrcpy: DeviceModel: nil runner")
	}
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "shell", "getprop", "ro.product.model")
	out, err := r.Run(ctx, "adb", args...)
	if err != nil {
		return "", fmt.Errorf("scrcpy: getprop ro.product.model: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// EnforceDevIgnore calls DeviceModel and aborts if the model matches any
// pattern in the .devignore file. It is the single gate every code path
// opening a scrcpy control socket MUST pass through.
//
// Returns ErrDeviceBlocked wrapped with the offending model on match, the
// underlying error on any failure to obtain the model, or nil on safe.
func EnforceDevIgnore(ctx context.Context, r CommandRunner, serial, devIgnorePath string) error {
	patterns, err := LoadDevIgnore(devIgnorePath)
	if err != nil {
		return fmt.Errorf("scrcpy: load .devignore: %w", err)
	}
	model, err := DeviceModel(ctx, r, serial)
	if err != nil {
		return err
	}
	if MatchModel(patterns, model) {
		return fmt.Errorf("%w: model=%q serial=%q", ErrDeviceBlocked, model, serial)
	}
	return nil
}
