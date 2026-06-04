// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-conduit-monitor is the conductor-facing consumer of
// the HelixQA real-time event channel (pkg/conduit). It tails the
// JSONL event stream a running HelixQA session appends to and prints
// a live, human-readable feed — letting an external orchestrator
// (operator or another AI agent) stay in sync with everything the
// session is doing: session start, phase transitions, per-Challenge
// steps, evidence captured, verdicts, LLM/Vision bridge calls, errors.
//
// It also supports a one-shot status snapshot read for an O(1)
// "where are we now" view without tailing.
//
// Usage:
//
//	# Live feed (follows the stream as it grows; stops at session_end):
//	helixqa-conduit-monitor -stream qa-results/conduit.events.jsonl
//
//	# Replay full history then follow:
//	helixqa-conduit-monitor -stream <path> -from-start
//
//	# Emit each event as raw JSONL (for machine consumption / piping):
//	helixqa-conduit-monitor -stream <path> -json
//
//	# One-shot status snapshot:
//	helixqa-conduit-monitor -status qa-results/conduit.status.json
//
// Exit code is non-zero when the session ends with a FAIL or
// OPERATOR-BLOCKED final verdict, so a conductor can gate on it.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"digital.vasic.helixqa/pkg/conduit"
)

func main() {
	var (
		streamPath = flag.String("stream", "", "path to the conduit JSONL event stream to tail")
		statusPath = flag.String("status", "", "path to a conduit status snapshot to print once, then exit")
		fromStart  = flag.Bool("from-start", false, "replay existing events from the beginning before following")
		jsonOut    = flag.Bool("json", false, "emit each event as raw JSONL instead of a human-readable line")
		follow     = flag.Bool("follow", true, "keep following after session_end (default: stop at session_end)")
		pollMS     = flag.Int("poll-ms", 200, "poll interval in milliseconds when at end of stream")
	)
	flag.Parse()

	if *statusPath != "" {
		os.Exit(printStatus(*statusPath))
	}
	if *streamPath == "" {
		fmt.Fprintln(os.Stderr, "error: -stream or -status is required")
		flag.Usage()
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	opts := []conduit.MonitorOption{conduit.WithPollInterval(time.Duration(*pollMS) * time.Millisecond)}
	if *fromStart {
		opts = append(opts, conduit.FromStart())
	}
	if !*follow {
		opts = append(opts, conduit.StopOnSessionEnd())
	}
	mon := conduit.NewMonitor(*streamPath, opts...)

	finalVerdict := conduit.VerdictUnknown
	err := mon.Tail(ctx, func(ev conduit.Event) error {
		if *jsonOut {
			line, _ := json.Marshal(ev)
			fmt.Println(string(line))
		} else {
			fmt.Println(render(ev))
		}
		if ev.Type == conduit.EventSessionEnd {
			finalVerdict = ev.Verdict
		}
		return nil
	})
	if err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "monitor: %v\n", err)
		os.Exit(1)
	}

	switch finalVerdict {
	case conduit.VerdictFail, conduit.VerdictOperatorBlocked:
		os.Exit(1)
	}
}

// render turns an event into a compact, aligned, human-readable line.
func render(ev conduit.Event) string {
	ts := ev.Time.Format("15:04:05.000")
	switch ev.Type {
	case conduit.EventSessionStart:
		return fmt.Sprintf("[%s] ▶ SESSION START  %s", ts, ev.Session)
	case conduit.EventSessionEnd:
		return fmt.Sprintf("[%s] ■ SESSION END    %s  verdict=%s  %s", ts, ev.Session, ev.Verdict, ev.Detail)
	case conduit.EventPhaseStart:
		return fmt.Sprintf("[%s]   ┌ phase start   %s", ts, ev.Phase)
	case conduit.EventPhaseComplete:
		return fmt.Sprintf("[%s]   └ phase done    %s  (%dms)", ts, ev.Phase, ev.DurationMS)
	case conduit.EventPhaseError:
		return fmt.Sprintf("[%s]   ✗ phase error   %s  %s", ts, ev.Phase, ev.Reason)
	case conduit.EventPhaseProgress:
		return fmt.Sprintf("[%s]     progress     %s  %.0f%%", ts, ev.Phase, ev.Progress*100)
	case conduit.EventChallengeStart:
		return fmt.Sprintf("[%s]     ▷ challenge   %s  [%s]", ts, ev.Challenge, ev.Platform)
	case conduit.EventChallengeStep:
		return fmt.Sprintf("[%s]       · step      %s — %s", ts, ev.Challenge, ev.Step)
	case conduit.EventChallengeVerdict:
		mark := verdictMark(ev.Verdict)
		reason := ""
		if ev.Reason != "" {
			reason = "  (" + ev.Reason + ")"
		}
		return fmt.Sprintf("[%s]     %s verdict     %s  %s%s", ts, mark, ev.Challenge, ev.Verdict, reason)
	case conduit.EventEvidenceCaptured:
		size := evidenceSize(ev.EvidencePath)
		return fmt.Sprintf("[%s]       ◆ evidence  %s  %s  (%s)", ts, ev.EvidenceKind, ev.EvidencePath, size)
	case conduit.EventLLMCall:
		return fmt.Sprintf("[%s]       ~ llm       %s  (%dms)  %v", ts, ev.Phase, ev.DurationMS, ev.Fields)
	case conduit.EventVisionCall:
		return fmt.Sprintf("[%s]       ~ vision    %s  (%dms)  %v", ts, ev.Phase, ev.DurationMS, ev.Fields)
	case conduit.EventError:
		return fmt.Sprintf("[%s]   ✗ ERROR        %s  %s", ts, ev.Phase, ev.Reason)
	case conduit.EventLog:
		return fmt.Sprintf("[%s]     log          %s", ts, ev.Detail)
	default:
		line, _ := json.Marshal(ev)
		return fmt.Sprintf("[%s] %s", ts, string(line))
	}
}

func verdictMark(v conduit.Verdict) string {
	switch v {
	case conduit.VerdictPass:
		return "✓"
	case conduit.VerdictFail:
		return "✗"
	case conduit.VerdictSkip:
		return "⊘"
	case conduit.VerdictOperatorBlocked:
		return "⚠"
	default:
		return "?"
	}
}

// evidenceSize reports the on-disk size of a captured-evidence
// artefact so the conductor can spot 0-byte (bluff) evidence at a
// glance — the §11.4.5 0-byte-mp4 pattern.
func evidenceSize(path string) string {
	if path == "" {
		return "no-path"
	}
	fi, err := os.Stat(path)
	if err != nil {
		return "MISSING"
	}
	if fi.Size() == 0 {
		return "0 bytes ⚠"
	}
	return humanSize(fi.Size())
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// printStatus reads and prints a status snapshot. Returns the process
// exit code.
func printStatus(path string) int {
	st, err := conduit.ReadStatus(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "status: %v\n", err)
		return 1
	}
	out, _ := json.MarshalIndent(st, "", "  ")
	fmt.Println(string(out))
	if st.FinalVerdict == conduit.VerdictFail || st.FinalVerdict == conduit.VerdictOperatorBlocked {
		return 1
	}
	return 0
}
