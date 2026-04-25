// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	automation "digital.vasic.helixqa/pkg/nexus/automation"
	"digital.vasic.helixqa/pkg/ticket"
)

// runReplay implements the `helixqa replay` subcommand.
//
// It reads a ticket file, extracts the .ocu-replay fenced code block,
// parses the DSL into []automation.Action, and either prints a dry-run
// plan (default) or executes via a live engine stub (--execute).
//
// Exit codes:
//
//	0  success (dry-run print or execute complete)
//	1  runtime error (cannot read file, no DSL block, parse error)
//	2  usage error (missing required flag)
func runReplay(args []string) int {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	ticketPath := fs.String("ticket", "",
		"path to the ticket markdown file (required)")
	execute := fs.Bool("execute", false,
		"actually run the replay; default is dry-run (print only)")

	if err := fs.Parse(args); err != nil {
		// ContinueOnError: flag already printed the error.
		return 2
	}
	if *ticketPath == "" {
		fmt.Fprintln(os.Stderr,
			"helixqa replay: --ticket <path> is required")
		fs.Usage()
		return 2
	}

	data, err := os.ReadFile(*ticketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"helixqa replay: read ticket: %v\n", err)
		return 1
	}

	dsl := extractReplayDSL(data)
	if dsl == "" {
		fmt.Fprintln(os.Stderr,
			"helixqa replay: no ```.ocu-replay block found in ticket")
		return 1
	}

	actions, warnings, err := ticket.ParseReplayScript([]byte(dsl))
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"helixqa replay: parse: %v\n", err)
		return 1
	}
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "helixqa replay: WARN: %s\n", w)
	}

	if *execute {
		return replayExecute(actions, os.Stdout)
	}
	return replayDryRun(actions, os.Stdout)
}

// extractReplayDSL extracts the content of the first ```.ocu-replay
// fenced code block from a markdown ticket. Returns an empty string
// when no such block is found.
func extractReplayDSL(md []byte) string {
	const fence = "```"
	const label = fence + ".ocu-replay"
	start := indexBytes(md, []byte(label))
	if start < 0 {
		return ""
	}
	// Advance past the opening fence line (to the next newline).
	afterFence := md[start+len(label):]
	// Skip the rest of the opening fence line.
	nlIdx := indexBytes(afterFence, []byte{'\n'})
	if nlIdx >= 0 {
		afterFence = afterFence[nlIdx+1:]
	}
	// Find the closing ``` fence.
	end := indexBytes(afterFence, []byte(fence))
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(string(afterFence[:end]))
}

// indexBytes returns the first index of needle in haystack, or -1.
// It is a self-contained replacement for bytes.Index that avoids an
// additional import in this lightweight CLI file.
func indexBytes(haystack, needle []byte) int {
	n, h := len(needle), len(haystack)
	if n == 0 {
		return 0
	}
	if h < n {
		return -1
	}
outer:
	for i := 0; i <= h-n; i++ {
		for j := 0; j < n; j++ {
			if haystack[i+j] != needle[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}

// replayDryRun prints each action that would be executed and returns 0.
func replayDryRun(actions []automation.Action, out io.Writer) int {
	fmt.Fprintln(out, "=== helixqa replay (dry-run) ===")
	if len(actions) == 0 {
		fmt.Fprintln(out, "  (no actions)")
		return 0
	}
	for i, a := range actions {
		fmt.Fprintf(out, "  [%d] %s\n", i+1, describeAction(a))
	}
	fmt.Fprintf(out,
		"\nWould execute %d action(s). Pass --execute to run for real.\n",
		len(actions))
	return 0
}

// replayExecute is the --execute path. In P*.7 scope this is a
// documented stub: it prints the action plan and returns 0. Full engine
// wiring (capture.Open + interact.Open + automation.Engine.Perform)
// lands in P*.7.1 and requires a running OCU stack.
func replayExecute(actions []automation.Action, out io.Writer) int {
	fmt.Fprintln(out, "=== helixqa replay (execute — P*.7 stub) ===")
	fmt.Fprintln(out,
		"NOTE: --execute currently prints the action plan and exits.")
	fmt.Fprintln(out,
		"Full engine wiring lands in P*.7.1 (requires a running Engine).")
	fmt.Fprintln(out)
	return replayDryRun(actions, out)
}

// describeAction returns a human-readable one-liner for an Action,
// including the key fields relevant to each ActionKind.
func describeAction(a automation.Action) string {
	switch a.Kind {
	case automation.ActionClick:
		return fmt.Sprintf("click at (%d, %d)", a.At.X, a.At.Y)
	case automation.ActionType:
		text := a.Text
		if len(text) > 40 {
			text = text[:37] + "..."
		}
		return fmt.Sprintf("type %q", text)
	case automation.ActionScroll:
		return fmt.Sprintf("scroll at (%d, %d) dx=%d dy=%d",
			a.At.X, a.At.Y, a.DX, a.DY)
	case automation.ActionKey:
		return fmt.Sprintf("key %q", string(a.Key))
	case automation.ActionDrag:
		return fmt.Sprintf("drag (%d, %d) → (%d, %d)",
			a.At.X, a.At.Y, a.To.X, a.To.Y)
	case automation.ActionCapture:
		return "capture screenshot"
	case automation.ActionAnalyze:
		return "analyze screenshot via vision pipeline"
	case automation.ActionRecordClip:
		return fmt.Sprintf("record clip around=%d window=%d",
			a.ClipAround, a.ClipWindow)
	default:
		return string(a.Kind)
	}
}
