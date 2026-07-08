// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-tesseract is the RUNNABLE analyzer for the HelixQA
// CPU Tesseract OCR bank (banks/helixllm_tesseract.yaml) — it drives a real
// render->OCR round trip against the live HelixLLM Tesseract-5 (OEM 1/LSTM)
// shim (default :18438, boot mechanism reused from
// docs/qa/phase3_tesseract_ocr_20260707/harness per §11.4.74): POST
// /v1/render renders a KNOWN text string to a PNG (unfakeable — the shim
// controls the rendered pixels), then POST /v1/ocr recovers text from that
// SAME image via a *separate* call, so the only way this passes is if
// Tesseract genuinely reads pixels it was never told the answer to.
// Mirrors the CLI convention of cmd/helixqa-verify-vision
// (--out/--conduit-dir/--challenge-id/--expect-fail, exit 0/1/2).
//
// ANTI-BLUFF (§11.4.6/§11.4.69/§11.4.107(10)/§11.4.107(13)/§11.4.123). PASS
// requires BOTH: (1) every --expect token found in the recognized full_text
// (case-insensitive substring), AND (2) mean_conf >= --conf-floor (default
// 60 — calibrated from this capability's own observed data per the phase-3
// proof: good cluster ~95-96, worst bad-fixture ~12; §11.4.107(13) forbids
// a literature-sourced floor). --expect-fail inverts case success.
//
// Usage:
//
//	helixqa-verify-tesseract \
//	  --known-text "HELIX OCR 2026 quick brown fox" \
//	  --expect "helix,ocr,2026,quick,brown,fox" \
//	  --out qa-results/helixllm_tesseract/good1_verdict.json \
//	  [--endpoint http://localhost:18438] [--conf-floor 60] \
//	  [--conduit-dir qa-results/helixllm_tesseract/conduit]
//	  [--challenge-id OCR-GOOD-001] [--timeout 60s] [--expect-fail]
//
// Exit codes: 0 -> case_result==true; 1 -> case_result==false; 2 -> infra error.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/conduit"
)

const (
	exitPass  = 0
	exitFail  = 1
	exitInfra = 2
)

type renderRequest struct {
	Text      string `json:"text"`
	Mode      string `json:"mode"`
	PointSize int    `json:"pointsize"`
}

type ocrWord struct {
	Text string  `json:"text"`
	Conf float64 `json:"conf"`
}

type ocrResponse struct {
	Engine   string    `json:"engine"`
	Words    []ocrWord `json:"words"`
	FullText string    `json:"full_text"`
	MeanConf float64   `json:"mean_conf"`
}

type verdict struct {
	KnownText     string   `json:"known_text"`
	Mode          string   `json:"mode"`
	Endpoint      string   `json:"endpoint"`
	FullText      string   `json:"full_text"`
	MeanConf      float64  `json:"mean_conf"`
	ConfFloor     float64  `json:"conf_floor"`
	ExpectedFacts []string `json:"expected_facts"`
	MatchedFacts  int      `json:"matched_facts"`
	ExpectedCount int      `json:"expected_count"`
	Pass          bool     `json:"pass"`
	ExpectFail    bool     `json:"expect_fail"`
	CaseResult    bool     `json:"case_result"`
	LatencyMS     int64    `json:"latency_ms"`
	Error         string   `json:"error,omitempty"`
}

func main() {
	os.Exit(run())
}

func run() int {
	var (
		knownText = flag.String("known-text", "", "known text string to render then OCR (required unless --image is given)")
		imagePath = flag.String("image", "", "pre-rendered image to OCR directly (skips the /v1/render step — for golden-bad blank/noise fixtures)")
		mode      = flag.String("mode", "label", "render mode: label|blank|noise")
		expectCSV = flag.String("expect", "", "comma-separated required tokens (case-insensitive substring match against full_text)")
		confFloor = flag.Float64("conf-floor", 60, "minimum mean_conf required for pass (calibrated from this capability's own data, §11.4.107(13))")
		endpoint  = flag.String("endpoint", envOr("HELIXLLM_OCR_ENDPOINT", "http://localhost:18438"), "Tesseract OCR shim base URL")
		out       = flag.String("out", "", "path to write the verdict JSON (required)")
		conduitDir = flag.String("conduit-dir", "", "optional conduit JSONL event dir (§11.4.116)")
		challID    = flag.String("challenge-id", "", "challenge id for conduit events (defaults to --out basename)")
		timeout    = flag.Duration("timeout", 60*time.Second, "request timeout")
		expectFail = flag.Bool("expect-fail", false, "invert case-level exit code — for golden-bad self-validation fixtures")
	)
	flag.Parse()

	if (*knownText == "" && *imagePath == "") || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-tesseract --known-text <str> --out <verdict.json> [--expect a,b] [--conf-floor 60] [--mode label|blank|noise]")
		return exitInfra
	}
	cid := *challID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_tesseract", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}
	conduit.ChallengeStart(sink, cid, "ocr")

	v := verdict{
		KnownText:     *knownText,
		Mode:          *mode,
		Endpoint:      *endpoint,
		ExpectedFacts: splitCSV(*expectCSV),
		ConfFloor:     *confFloor,
	}
	v.ExpectedCount = len(v.ExpectedFacts)

	client := &http.Client{Timeout: *timeout}
	start := time.Now()

	var imgBytes []byte
	if *imagePath != "" {
		b, err := os.ReadFile(*imagePath)
		if err != nil {
			return failInfra(sink, cid, &v, *out, fmt.Sprintf("read image: %v", err))
		}
		imgBytes = b
	} else {
		reqBody, _ := json.Marshal(renderRequest{Text: *knownText, Mode: *mode, PointSize: 48})
		rresp, err := client.Post(*endpoint+"/v1/render", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			return failInfra(sink, cid, &v, *out, fmt.Sprintf("POST /v1/render: %v", err))
		}
		defer rresp.Body.Close()
		b, _ := io.ReadAll(rresp.Body)
		if rresp.StatusCode != http.StatusOK {
			return failInfra(sink, cid, &v, *out, fmt.Sprintf("/v1/render non-200 status=%d", rresp.StatusCode))
		}
		imgBytes = b
	}

	oresp, err := client.Post(*endpoint+"/v1/ocr", "application/octet-stream", bytes.NewReader(imgBytes))
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("POST /v1/ocr: %v", err))
	}
	defer oresp.Body.Close()
	latency := time.Since(start)
	v.LatencyMS = latency.Milliseconds()
	ob, _ := io.ReadAll(oresp.Body)
	if oresp.StatusCode != http.StatusOK {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("/v1/ocr non-200 status=%d", oresp.StatusCode))
	}
	var or ocrResponse
	if err := json.Unmarshal(ob, &or); err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("decode /v1/ocr response: %v", err))
	}
	v.FullText = or.FullText
	v.MeanConf = or.MeanConf

	// --- token-set match + confidence floor (§11.4.6/§11.4.107(10)/(13)) ---
	lowerFT := strings.ToLower(v.FullText)
	matched := 0
	for _, f := range v.ExpectedFacts {
		if strings.Contains(lowerFT, strings.ToLower(f)) {
			matched++
		}
	}
	v.MatchedFacts = matched
	v.Pass = v.ExpectedCount > 0 && v.MatchedFacts == v.ExpectedCount && v.MeanConf >= v.ConfFloor

	v.ExpectFail = *expectFail
	if v.ExpectFail {
		v.CaseResult = !v.Pass
	} else {
		v.CaseResult = v.Pass
	}

	if writeErr := writeVerdict(*out, &v); writeErr != nil {
		fmt.Fprintf(os.Stderr, "ERROR: write verdict: %v\n", writeErr)
		return exitInfra
	}

	conduit.EvidenceCaptured(sink, cid, "ocr_verdict_json", *out)

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, "")
		fmt.Printf("PASS: %s matched=%d/%d mean_conf=%.2f expect_fail=%v raw_pass=%v full_text=%q\n",
			cid, v.MatchedFacts, v.ExpectedCount, v.MeanConf, v.ExpectFail, v.Pass, v.FullText)
		return exitPass
	}
	reason := fmt.Sprintf("matched=%d/%d mean_conf=%.2f expect_fail=%v raw_pass=%v", v.MatchedFacts, v.ExpectedCount, v.MeanConf, v.ExpectFail, v.Pass)
	conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, reason)
	fmt.Printf("FAIL: %s %s full_text=%q\n", cid, reason, v.FullText)
	return exitFail
}

func failInfra(sink conduit.Sink, cid string, v *verdict, out, reason string) int {
	v.Error = reason
	v.Pass = false
	if writeErr := writeVerdict(out, v); writeErr != nil {
		fmt.Fprintf(os.Stderr, "ERROR: write verdict (after infra error %q): %v\n", reason, writeErr)
	}
	conduit.Errorf(sink, cid, reason)
	conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, reason)
	fmt.Fprintf(os.Stderr, "INFRA-ERROR: %s: %s\n", cid, reason)
	return exitInfra
}

func writeVerdict(path string, v *verdict) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
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
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
