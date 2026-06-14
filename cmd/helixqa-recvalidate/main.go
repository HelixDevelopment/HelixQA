// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-recvalidate is a RUNNABLE validator entrypoint that drives a
// single recordingqa bank entry over a real mp4 via the Panoptic recvalidate
// video oracle + the recordingqa orchestrator, and prints the structured
// verdict + captured-evidence paths.
//
// It is the standalone, operator-invokable companion to the
// pkg/recordingqa/panopticoracle integration test (which does the same wiring
// behind `-tags=integration`). The main thread invokes it on a recorded session:
//
//	go run ./cmd/helixqa-recvalidate \
//	  --bank   banks/helixcode-ensemble-members.yaml \
//	  --case   HXC-ENS-002 \
//	  --mp4    /path/to/session_members.mp4 \
//	  --panoptic-dir ../panoptic \
//	  [--stderr-log /path/to/session_members.stderr.log] \
//	  [--frames-dir /tmp/frames] [--json-out /tmp/findings.json] [--json]
//
// PIPELINE (closing the "reuse HelixQA" loop, identical to the integration test):
//
//	bank YAML (metadata.recvalidate_options as DATA)
//	  → pkg/recordingqa.Spec
//	    → pkg/recordingqa.Validator (the orchestrator)
//	      → panopticoracle adapter
//	        → Panoptic recvalidate CLI (ffmpeg + tesseract OCR over the mp4)
//	          → JSON report → recordingqa verdict.
//
// EXIT CODES (machine-readable for the conductor):
//
//	0 → PASS   1 → FAIL   2 → SKIP (oracle could not run; honest §11.4.3)
//	3 → usage / load error (bad flags, bank/case missing)
//
// ANTI-BLUFF (§11.4.3 / §11.4.69 / §11.4.123). The verdict comes entirely from
// the recordingqa orchestrator + the real Panoptic OCR analysis — never a fake
// PASS. A 0-byte / missing mp4 FAILs (exit 1). An analyzer that cannot run
// (ffmpeg/tesseract/Panoptic absent) SKIPs (exit 2), never a fake PASS. The
// golden-good / golden-bad behaviour is proven by the package tests
// (pkg/recordingqa/recordingqa_test.go) which this entrypoint reuses verbatim:
// the same Validator that FAILs a bad recording in those tests FAILs it here.
//
// DECOUPLING (CONST-051(B) / §11.4.28). This entrypoint carries NO HelixCode /
// ATMOSphere knowledge: the bank, the case id, the mp4, the Panoptic invocation
// are all flags. The recvalidate options are read from the bank's
// metadata.recvalidate_options block as opaque consumer DATA.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/conduit"
	"digital.vasic.helixqa/pkg/recordingqa"
	"digital.vasic.helixqa/pkg/recordingqa/panopticoracle"
	"digital.vasic.helixqa/pkg/testbank"
)

const (
	exitPass  = 0
	exitFail  = 1
	exitSkip  = 2
	exitUsage = 3
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		bankPath    = flag.String("bank", "", "path to the recordingqa bank YAML (required)")
		caseID      = flag.String("case", "", "bank case id to run (required, e.g. HXC-ENS-002)")
		mp4         = flag.String("mp4", "", "path to the recorded session mp4 (required)")
		stderrLog   = flag.String("stderr-log", "", "path to the session's stderr log (optional; enables the log oracle)")
		panopticCmd = flag.String("panoptic-cmd", "", "argv to invoke Panoptic (default: 'go run .' in --panoptic-dir)")
		panopticDir = flag.String("panoptic-dir", "", "Panoptic checkout dir for `go run .` (default: sibling ../panoptic)")
		framesDir   = flag.String("frames-dir", "", "dir to keep extracted frames as evidence (default: a temp dir)")
		jsonOut     = flag.String("json-out", "", "path for the recvalidate JSON report (default: a temp file)")
		timeout     = flag.Duration("timeout", 9*time.Minute, "overall validation timeout")
		jsonMode    = flag.Bool("json", false, "emit a one-line JSON verdict instead of text")
	)
	flag.Parse()

	if *bankPath == "" || *caseID == "" || *mp4 == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-recvalidate --bank <yaml> --case <id> --mp4 <mp4> [--stderr-log <log>] [--panoptic-dir <dir>]")
		flag.PrintDefaults()
		return exitUsage
	}

	// --- load the bank + find the case ---
	bf, err := testbank.LoadFile(*bankPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load bank %s: %v\n", *bankPath, err)
		return exitUsage
	}
	entry := findCase(bf, *caseID)
	if entry == nil {
		fmt.Fprintf(os.Stderr, "case %s not found in %s\n", *caseID, *bankPath)
		return exitUsage
	}

	opts, err := extractRecvalidateOptions(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "case %s: %v\n", *caseID, err)
		return exitUsage
	}

	// --- resolve the Panoptic invocation (decoupled: injected, never hardcoded) ---
	var command []string
	var dir string
	if *panopticCmd != "" {
		command = strings.Fields(*panopticCmd)
	} else {
		command = []string{"go", "run", "."}
		dir = *panopticDir
		if dir == "" {
			// Default to the sibling Panoptic checkout under the same parent dir.
			wd, _ := os.Getwd()
			dir = filepath.Clean(filepath.Join(wd, "..", "panoptic"))
		}
	}

	// --- resolve evidence sinks (frames dir + json report) ---
	fdir := *framesDir
	jout := *jsonOut
	if fdir == "" || jout == "" {
		tmp, terr := os.MkdirTemp("", "helixqa-recvalidate-")
		if terr != nil {
			fmt.Fprintf(os.Stderr, "mktemp: %v\n", terr)
			return exitUsage
		}
		if fdir == "" {
			fdir = filepath.Join(tmp, "frames")
		}
		if jout == "" {
			jout = filepath.Join(tmp, "findings.json")
		}
	}

	oracle := panopticoracle.New(panopticoracle.Config{
		Command:      command,
		Dir:          dir,
		Model:        opts.expectedModel,
		FramesDir:    fdir,
		JSONOutPath:  jout,
		PreprocessVF: opts.preprocessVF,
	})

	// --- run the bank entry's options through the recordingqa orchestrator ---
	orch := &recordingqa.Validator{Video: oracle}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	res := orch.Validate(ctx, recordingqa.Spec{
		ChallengeID:        entry.EffectiveChallengeID(),
		RecordingPath:      *mp4,
		StderrLogPath:      *stderrLog,
		ExpectedReplies:    opts.expectedPrompts,
		ReplyMarkers:       opts.replyMarkers,
		ChromeLinePatterns: opts.chromePatterns,
		RequireLog:         *stderrLog != "",
	})

	emit(res, *jsonMode)

	switch res.Verdict {
	case conduit.VerdictPass:
		return exitPass
	case conduit.VerdictSkip:
		return exitSkip
	default:
		return exitFail
	}
}

func emit(res recordingqa.Result, jsonMode bool) {
	if jsonMode {
		// Minimal, dependency-free one-line JSON (no struct tags needed).
		fmt.Printf(`{"challenge_id":%q,"verdict":%q,"reason":%q,"matched_replies":%s,"evidence_paths":%s}`+"\n",
			res.ChallengeID, string(res.Verdict), res.Reason,
			jsonStrArr(res.Video.MatchedReplies), jsonStrArr(res.EvidencePaths))
		return
	}
	fmt.Printf("challenge_id:    %s\n", res.ChallengeID)
	fmt.Printf("verdict:         %s\n", res.Verdict)
	fmt.Printf("reason:          %s\n", res.Reason)
	fmt.Printf("video detail:    %s\n", res.Video.Detail)
	fmt.Printf("matched replies: %v\n", res.Video.MatchedReplies)
	if len(res.Video.MissingReplies) > 0 {
		fmt.Printf("missing replies: %v\n", res.Video.MissingReplies)
	}
	if len(res.Video.ErrorTextFound) > 0 {
		fmt.Printf("error text:      %v\n", res.Video.ErrorTextFound)
	}
	if len(res.Log.Matches) > 0 {
		fmt.Printf("log matches:     %v\n", res.Log.Matches)
	}
	fmt.Printf("evidence paths:  %v\n", res.EvidencePaths)
}

func jsonStrArr(ss []string) string {
	if len(ss) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range ss {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%q", s)
	}
	b.WriteByte(']')
	return b.String()
}

func findCase(bf *testbank.BankFile, id string) *testbank.TestCase {
	for i := range bf.TestCases {
		if bf.TestCases[i].ID == id {
			return &bf.TestCases[i]
		}
	}
	return nil
}

// bankOptions is the typed view of metadata.recvalidate_options — the same shape
// the panopticoracle integration test reads, so the bank's DATA drives the run.
type bankOptions struct {
	replyMarkers    []string
	chromePatterns  []string
	expectedReplies []string
	expectedPrompts []string
	expectedModel   string
	preprocessVF    string
}

// extractRecvalidateOptions reads metadata.recvalidate_options.{reply_markers,
// chrome_line_patterns,expected_replies,expected_prompts,expected_model_visible,
// preprocess_vf} from the bank entry. A case without the block, or without
// expected_prompts (the OCR-literal substrings that drive the real run), is a
// usage error — there is nothing to assert.
func extractRecvalidateOptions(tc *testbank.TestCase) (bankOptions, error) {
	raw, ok := tc.Metadata["recvalidate_options"]
	if !ok {
		return bankOptions{}, fmt.Errorf("no metadata.recvalidate_options block")
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return bankOptions{}, fmt.Errorf("metadata.recvalidate_options is not a map (got %T)", raw)
	}
	out := bankOptions{
		replyMarkers:    toStringSlice(m["reply_markers"]),
		chromePatterns:  toStringSlice(m["chrome_line_patterns"]),
		expectedReplies: toStringSlice(m["expected_replies"]),
		expectedPrompts: toStringSlice(m["expected_prompts"]),
	}
	if s, ok := m["expected_model_visible"].(string); ok {
		out.expectedModel = s
	}
	if s, ok := m["preprocess_vf"].(string); ok {
		out.preprocessVF = s
	}
	if len(out.expectedPrompts) == 0 {
		return bankOptions{}, fmt.Errorf("metadata.recvalidate_options has no expected_prompts (nothing to assert on the recording)")
	}
	return out, nil
}

func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
