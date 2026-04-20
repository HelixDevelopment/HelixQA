// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package bridges provides Go bridge packages for launching and
// interacting with open-source QA tools via subprocess, and a
// registry for discovering which tools are available on the host.
package bridges

import (
	"context"
	"os/exec"
	"strings"
)

// CommandRunner abstracts command execution for testing.
type CommandRunner interface {
	// Run executes a command and returns its combined output.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ToolStatus captures the availability and metadata of a single
// external QA tool.
type ToolStatus struct {
	// Name is the tool's canonical binary name (e.g. "scrcpy").
	Name string `json:"name"`

	// Available reports whether the tool was found on the host.
	Available bool `json:"available"`

	// Path is the resolved absolute path to the binary, or empty
	// if the tool was not found.
	Path string `json:"path,omitempty"`

	// Version is the version string reported by the tool, or
	// empty if the version could not be determined.
	Version string `json:"version,omitempty"`

	// Kind distinguishes external tools from HelixQA's own sidecars.
	Kind ToolKind `json:"kind"`
}

// NativeTools returns the subset of statuses that are HelixQA-native sidecars.
func NativeTools(all []ToolStatus) []ToolStatus {
	out := make([]ToolStatus, 0, len(all))
	for _, s := range all {
		if s.Kind == KindHelixQANative {
			out = append(out, s)
		}
	}
	return out
}

// ExternalTools returns the subset that are external third-party binaries.
func ExternalTools(all []ToolStatus) []ToolStatus {
	out := make([]ToolStatus, 0, len(all))
	for _, s := range all {
		if s.Kind == KindExternal {
			out = append(out, s)
		}
	}
	return out
}

// ToolKind distinguishes external third-party binaries from HelixQA's own
// sidecar binaries shipped alongside the Go host. Consumers use this to
// decide whether a missing tool is a packaging bug ("we ship it") or a
// host-setup requirement ("operator must install it").
type ToolKind int

const (
	// KindExternal is a third-party tool installed on the host (scrcpy,
	// adb, ffmpeg, etc.).
	KindExternal ToolKind = iota
	// KindHelixQANative is a sidecar binary shipped by the HelixQA release
	// (helixqa-capture-linux, helixqa-kmsgrab, helixqa-input, ...).
	KindHelixQANative
)

func (k ToolKind) String() string {
	switch k {
	case KindHelixQANative:
		return "helixqa-native"
	default:
		return "external"
	}
}

// toolProbe describes how to discover a single tool.
type toolProbe struct {
	// name is the binary name to look up.
	name string

	// versionArgs are the arguments passed to the binary to
	// obtain a version string. If nil, version detection is
	// skipped.
	versionArgs []string

	// kind distinguishes external tools from HelixQA sidecars.
	kind ToolKind
}

// toolProbes is the ordered list of tools that DiscoverTools checks. External
// third-party tools come first (existing set), followed by HelixQA's own
// sidecar binaries added in OpenClawing4 Phase 1+ (see
// docs/openclawing/OpenClawing4.md §6.1).
var toolProbes = []toolProbe{
	// External tools (pre-existing).
	{name: "scrcpy", versionArgs: []string{"--version"}},
	{name: "appium", versionArgs: []string{"--version"}},
	{name: "allure", versionArgs: []string{"--version"}},
	{name: "perfetto", versionArgs: []string{"--version"}},
	{name: "maestro", versionArgs: []string{"--version"}},
	{name: "ffmpeg", versionArgs: []string{"-version"}},
	{name: "adb", versionArgs: []string{"version"}},
	{name: "npx", versionArgs: []string{"--version"}},
	{name: "xdotool", versionArgs: []string{"version"}},

	// HelixQA-native sidecars (OpenClawing4 Phase 1+).
	{name: "helixqa-capture-linux", versionArgs: []string{"--health"}, kind: KindHelixQANative},
	{name: "helixqa-kmsgrab", versionArgs: []string{"--health"}, kind: KindHelixQANative},
	{name: "helixqa-input", versionArgs: []string{"--health"}, kind: KindHelixQANative},
	{name: "helixqa-frida", versionArgs: []string{"--health"}, kind: KindHelixQANative},
	{name: "helixqa-axtree-darwin", versionArgs: []string{"--health"}, kind: KindHelixQANative},
	{name: "helixqa-capture-darwin", versionArgs: []string{"--health"}, kind: KindHelixQANative},
	{name: "helixqa-capture-win", versionArgs: []string{"--health"}, kind: KindHelixQANative},
	{name: "helixqa-omniparser", versionArgs: []string{"--health"}, kind: KindHelixQANative},
	{name: "helixqa-langgraph", versionArgs: []string{"--health"}, kind: KindHelixQANative},
	{name: "helixqa-browser-use", versionArgs: []string{"--health"}, kind: KindHelixQANative},
	{name: "qa-vision-infer", versionArgs: []string{"--health"}, kind: KindHelixQANative},
	{name: "qa-video-decode", versionArgs: []string{"--health"}, kind: KindHelixQANative},
	{name: "qa-vulkan-compute", versionArgs: []string{"--health"}, kind: KindHelixQANative},
}

// DiscoverTools checks which QA tools are installed and reachable on
// the host. It uses runner to probe each tool's version; the PATH
// lookup is done via exec.LookPath (not the runner) so that
// availability is based on the real filesystem even in tests.
func DiscoverTools(runner CommandRunner) []ToolStatus {
	ctx := context.Background()
	statuses := make([]ToolStatus, 0, len(toolProbes))

	for _, probe := range toolProbes {
		status := ToolStatus{Name: probe.name, Kind: probe.kind}

		path, err := exec.LookPath(probe.name)
		if err != nil {
			// Tool not found in PATH.
			statuses = append(statuses, status)
			continue
		}

		status.Available = true
		status.Path = path

		if len(probe.versionArgs) > 0 {
			status.Version = probeVersion(
				ctx, runner, probe.name, probe.versionArgs,
			)
		}

		statuses = append(statuses, status)
	}

	return statuses
}

// probeVersion runs the tool with its version arguments and returns
// the first non-empty trimmed line of the output. Returns an empty
// string on error.
func probeVersion(
	ctx context.Context,
	runner CommandRunner,
	name string,
	args []string,
) string {
	output, err := runner.Run(ctx, name, args...)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(output), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
