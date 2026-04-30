// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command qa-audio-probe checks reachability of the QA-reprocess
// containers (Whisper ASR on :7070 and Tesseract OCR on :7071)
// and prints a per-service PASS/FAIL/SKIP line.
//
// Use case: B4 (visual/audio coverage tests SKIP-with-reason when
// the host containers aren't running; previously hard to tell apart
// from "test broken" at a glance). This command is the single
// operator-facing answer to "is my QA stack healthy?".
//
// Exit codes (scriptable):
//
//	0 — all required services PASS
//	1 — at least one required service FAILed
//	2 — caller error (invalid flag combo)
//
// Output: text by default, JSON with --json (machine-parseable for
// CI integration).
//
// Constitution §11.4: every PASS records a SPECIFIC value from the
// /health response (model, version, langs) so a reviewer reading
// the log can verify the container is the expected build, not just
// that "something" answered.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/audio"
)

// ServiceResult is one row of the per-service probe table.
type ServiceResult struct {
	Service string `json:"service"`         // "whisper" / "tesseract"
	URL     string `json:"url"`             // base URL probed
	State   string `json:"state"`           // "PASS" / "FAIL" / "SKIP"
	Detail  string `json:"detail"`          // human-readable evidence
	Error   string `json:"error,omitempty"` // present iff State=FAIL
}

// ProbeReport is the full machine-parseable output.
type ProbeReport struct {
	Services []ServiceResult `json:"services"`
	Required []string        `json:"required"`
	AllOK    bool            `json:"all_ok"`
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

// run is split out for testability. argv is os.Args[1:]. Returns
// the exit code.
func run(ctx context.Context, argv []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("qa-audio-probe", flag.ContinueOnError)
	fs.SetOutput(errOut)

	whisperURL := fs.String("whisper-url", audio.DefaultWhisperBaseURL,
		"Whisper container base URL (no trailing slash).")
	tesseractURL := fs.String("tesseract-url", audio.DefaultTesseractBaseURL,
		"Tesseract container base URL (no trailing slash).")
	required := fs.String("require", "whisper,tesseract",
		"Comma-separated list of services that MUST PASS for exit 0. "+
			"Others are probed but only SKIP-counted on failure.")
	jsonOut := fs.Bool("json", false,
		"Emit machine-parseable JSON instead of human-readable text.")
	timeout := fs.Duration("timeout", 5*time.Second,
		"Per-service /health probe timeout.")

	if err := fs.Parse(argv); err != nil {
		// flag.ContinueOnError already wrote the usage line.
		return 2
	}

	requiredSet := splitCSV(*required)
	if len(requiredSet) == 0 {
		fmt.Fprintln(errOut, "qa-audio-probe: --require must list at least one service")
		return 2
	}
	for _, r := range requiredSet {
		if r != "whisper" && r != "tesseract" {
			fmt.Fprintf(errOut, "qa-audio-probe: --require has unknown service %q (valid: whisper, tesseract)\n", r)
			return 2
		}
	}

	probeCtx, cancel := context.WithTimeout(ctx, *timeout*2) // overall cap = 2× per-service
	defer cancel()

	// Probe both — emit results in deterministic order.
	results := []ServiceResult{
		probeWhisper(probeCtx, *whisperURL, *timeout, requiredSet),
		probeTesseract(probeCtx, *tesseractURL, *timeout, requiredSet),
	}

	report := ProbeReport{
		Services: results,
		Required: requiredSet,
		AllOK:    allRequiredPass(results, requiredSet),
	}

	if *jsonOut {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		emitText(out, report)
	}

	if report.AllOK {
		return 0
	}
	return 1
}

func probeWhisper(ctx context.Context, url string, perTimeout time.Duration, required []string) ServiceResult {
	res := ServiceResult{Service: "whisper", URL: url}
	probeCtx, cancel := context.WithTimeout(ctx, perTimeout)
	defer cancel()
	c := audio.NewWhisperClient(url)
	h, err := c.Health(probeCtx)
	if err != nil {
		if isRequired("whisper", required) {
			res.State = "FAIL"
		} else {
			res.State = "SKIP"
		}
		res.Error = err.Error()
		res.Detail = "container unreachable or unhealthy"
		return res
	}
	res.State = "PASS"
	res.Detail = fmt.Sprintf("backend=%s default_model=%s loaded=%v compute=%s",
		h.Backend, h.DefaultModel, h.LoadedModels, h.ComputeType)
	return res
}

func probeTesseract(ctx context.Context, url string, perTimeout time.Duration, required []string) ServiceResult {
	res := ServiceResult{Service: "tesseract", URL: url}
	probeCtx, cancel := context.WithTimeout(ctx, perTimeout)
	defer cancel()
	c := audio.NewTesseractClient(url)
	h, err := c.Health(probeCtx)
	if err != nil {
		if isRequired("tesseract", required) {
			res.State = "FAIL"
		} else {
			res.State = "SKIP"
		}
		res.Error = err.Error()
		res.Detail = "container unreachable or unhealthy"
		return res
	}
	res.State = "PASS"
	res.Detail = fmt.Sprintf("version=%s default_lang=%s default_psm=%d langs=%v",
		h.TesseractVersion, h.DefaultLang, h.DefaultPSM, h.Langs)
	return res
}

func emitText(out io.Writer, r ProbeReport) {
	fmt.Fprintln(out, "qa-audio-probe — QA-reprocess container health")
	fmt.Fprintln(out, "")
	for _, s := range r.Services {
		// Format: "<STATE>  <service>  @ <url>  — <detail>"
		// Right-pad STATE to 4 chars so columns align.
		fmt.Fprintf(out, "%-4s  %-9s @ %s  — %s\n",
			s.State, s.Service, s.URL, s.Detail)
		if s.Error != "" {
			fmt.Fprintf(out, "        error: %s\n", s.Error)
		}
	}
	fmt.Fprintln(out, "")
	if r.AllOK {
		fmt.Fprintf(out, "all required (%s) PASS\n", strings.Join(r.Required, ","))
	} else {
		fmt.Fprintf(out, "one or more required services FAILed (required: %s)\n",
			strings.Join(r.Required, ","))
	}
}

func allRequiredPass(results []ServiceResult, required []string) bool {
	pass := map[string]bool{}
	for _, r := range results {
		if r.State == "PASS" {
			pass[r.Service] = true
		}
	}
	for _, req := range required {
		if !pass[req] {
			return false
		}
	}
	return true
}

func isRequired(svc string, required []string) bool {
	for _, r := range required {
		if r == svc {
			return true
		}
	}
	return false
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	// De-duplicate so --require=whisper,whisper doesn't double-count.
	dedup := out[:0]
	var prev string
	for _, p := range out {
		if p != prev {
			dedup = append(dedup, p)
			prev = p
		}
	}
	return dedup
}
