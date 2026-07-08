// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-translate-nllb is the RUNNABLE analyzer for the
// HelixQA CPU-translation bank (banks/helixllm_translate_nllb.yaml) — it
// drives a real POST /translate against the live HelixLLM NLLB-200-CTranslate2
// shim (default :18436, boot mechanism reused from
// docs/qa/phase3_translation_nllb_20260707/harness per §11.4.74), applies the
// UNFAKEABLE keyword-substring + not-identity runtime signature the phase-3
// proof harness (docs/qa/phase3_translation_nllb_20260707/harness/main.go)
// already established, and writes a machine-readable verdict artefact the
// bank's RequiredEvidence gates on. Mirrors the CLI convention of
// cmd/helixqa-verify-vision (--out/--conduit-dir/--challenge-id/--expect-fail,
// exit 0/1/2) — a thin, project-agnostic dispatches_to analyzer (CONST-051(B),
// no HelixQA core-engine change).
//
// ANTI-BLUFF (§11.4.6/§11.4.69/§11.4.107(10)/§11.4.123). The verdict requires
// BOTH: (1) every --expect token present (case-insensitive substring) in the
// real forward translation, AND (2) the forward translation is NOT identical
// to the source (kills the echo/passthrough bluff the design doc's §2.5
// forbids). --expect-fail inverts which raw outcome counts as case success —
// the RAW pass is always recorded honestly (mirrors helixqa-verify-vision).
//
// Usage:
//
//	helixqa-verify-translate-nllb \
//	  --source "The house is blue." --source-lang eng_Latn --target-lang deu_Latn \
//	  --expect "haus,blau" \
//	  --out qa-results/helixllm_translate_nllb/en_de_verdict.json \
//	  [--endpoint http://localhost:18436/translate] \
//	  [--conduit-dir qa-results/helixllm_translate_nllb/conduit] \
//	  [--challenge-id TRA-UNDERSTAND-001] [--timeout 60s] [--expect-fail]
//
// Exit codes: 0 -> case_result==true; 1 -> case_result==false; 2 -> infra error.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
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

type translateRequest struct {
	Q      string `json:"q"`
	Source string `json:"source"`
	Target string `json:"target"`
}

type translateResponse struct {
	TranslatedText string `json:"translatedText"`
}

// verdict is the machine-readable artefact the ContentAssertingResolver
// gates on — embeds the raw request+response, never trust-the-analyzer-blindly.
type verdict struct {
	Source         string   `json:"source"`
	SourceLang     string   `json:"source_lang"`
	TargetLang     string   `json:"target_lang"`
	Endpoint       string   `json:"endpoint"`
	Forward        string   `json:"forward"`
	ExpectedFacts  []string `json:"expected_facts"`
	ForbiddenFacts []string `json:"forbidden_facts,omitempty"`
	MatchedFacts   int      `json:"matched_facts"`
	ExpectedCount  int      `json:"expected_count"`
	NotIdentity    bool     `json:"not_identity"`
	Hallucinated   bool     `json:"hallucinated"`
	Pass           bool     `json:"pass"`
	ExpectFail     bool     `json:"expect_fail"`
	CaseResult     bool     `json:"case_result"`
	LatencyMS      int64    `json:"latency_ms"`
	HTTPStatus     int      `json:"http_status"`
	Error          string   `json:"error,omitempty"`
}

func main() {
	os.Exit(run())
}

func run() int {
	var (
		source     = flag.String("source", "", "source sentence to translate (required)")
		sourceLang = flag.String("source-lang", "eng_Latn", "FLORES-200 source language code")
		targetLang = flag.String("target-lang", "", "FLORES-200 target language code (required)")
		expectCSV  = flag.String("expect", "", "comma-separated required fact tokens (case-insensitive substring match)")
		forbidCSV  = flag.String("forbid", "", "comma-separated forbidden fact tokens")
		endpoint   = flag.String("endpoint", envOr("HELIXLLM_TRANSLATE_ENDPOINT", "http://localhost:18436/translate"), "NLLB shim /translate endpoint")
		out        = flag.String("out", "", "path to write the verdict JSON (required)")
		conduitDir = flag.String("conduit-dir", "", "optional conduit JSONL event dir (§11.4.116)")
		challID    = flag.String("challenge-id", "", "challenge id for conduit events (defaults to --out basename)")
		timeout    = flag.Duration("timeout", 60*time.Second, "request timeout")
		expectFail = flag.Bool("expect-fail", false, "invert case-level exit code — for golden-bad self-validation fixtures")
	)
	flag.Parse()

	if *source == "" || *targetLang == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-translate-nllb --source <text> --target-lang <code> --out <verdict.json> [--expect a,b] [--forbid c,d]")
		return exitInfra
	}
	cid := *challID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_translate_nllb", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}
	conduit.ChallengeStart(sink, cid, "translation")

	v := verdict{
		Source:         *source,
		SourceLang:     *sourceLang,
		TargetLang:     *targetLang,
		Endpoint:       *endpoint,
		ExpectedFacts:  splitCSV(*expectCSV),
		ForbiddenFacts: splitCSV(*forbidCSV),
	}
	v.ExpectedCount = len(v.ExpectedFacts)

	reqBody, _ := json.Marshal(translateRequest{Q: *source, Source: *sourceLang, Target: *targetLang})
	client := &http.Client{Timeout: *timeout}
	start := time.Now()
	httpReq, err := http.NewRequest(http.MethodPost, *endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("build request: %v", err))
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(httpReq)
	latency := time.Since(start)
	v.LatencyMS = latency.Milliseconds()
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("http request: %v", err))
	}
	defer resp.Body.Close()
	v.HTTPStatus = resp.StatusCode

	var tr translateResponse
	if decErr := json.NewDecoder(resp.Body).Decode(&tr); decErr != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("decode response: %v", decErr))
	}
	if resp.StatusCode != http.StatusOK {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("non-200 status=%d", resp.StatusCode))
	}
	v.Forward = tr.TranslatedText

	// --- strict fact-matching + not-identity (§11.4.6/§11.4.107(10)) ---
	fwdNorm := normalize(v.Forward)
	srcNorm := normalize(v.Source)
	v.NotIdentity = strings.TrimSpace(v.Forward) != "" && fwdNorm != srcNorm

	lowerFwd := strings.ToLower(v.Forward)
	matched := 0
	for _, f := range v.ExpectedFacts {
		if strings.Contains(lowerFwd, strings.ToLower(f)) {
			matched++
		}
	}
	v.MatchedFacts = matched

	hallucinated := false
	for _, f := range v.ForbiddenFacts {
		if f != "" && strings.Contains(lowerFwd, strings.ToLower(f)) {
			hallucinated = true
			break
		}
	}
	v.Hallucinated = hallucinated
	v.Pass = v.NotIdentity && v.ExpectedCount > 0 && v.MatchedFacts == v.ExpectedCount && !v.Hallucinated

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

	conduit.EvidenceCaptured(sink, cid, "translate_verdict_json", *out)

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, "")
		fmt.Printf("PASS: %s notIdentity=%v matched=%d/%d hallucinated=%v expect_fail=%v raw_pass=%v forward=%q\n",
			cid, v.NotIdentity, v.MatchedFacts, v.ExpectedCount, v.Hallucinated, v.ExpectFail, v.Pass, v.Forward)
		return exitPass
	}
	reason := fmt.Sprintf("notIdentity=%v matched=%d/%d hallucinated=%v expect_fail=%v raw_pass=%v", v.NotIdentity, v.MatchedFacts, v.ExpectedCount, v.Hallucinated, v.ExpectFail, v.Pass)
	conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, reason)
	fmt.Printf("FAIL: %s %s forward=%q\n", cid, reason, v.Forward)
	return exitFail
}

func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
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
