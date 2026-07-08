// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-whisper is the RUNNABLE analyzer for the HelixQA
// CPU Whisper STT bank (banks/helixllm_whisper.yaml) — it drives a real
// POST /v1/audio/transcriptions (multipart) against the live HelixLLM
// faster-whisper (CTranslate2, int8, CPU) service (default :18437, boot
// mechanism reused from docs/qa/phase3_whisper_stt_20260707/harness per
// §11.4.74), applies the word-match + digit/number-word canonicalization
// runtime signature the phase-3 proof harness already established, and
// writes a machine-readable verdict artefact the bank's RequiredEvidence
// gates on. Mirrors the CLI convention of cmd/helixqa-verify-vision
// (--out/--conduit-dir/--challenge-id/--expect-fail, exit 0/1/2).
//
// ANTI-BLUFF (§11.4.6/§11.4.69/§11.4.107(10)/§11.4.123). The verdict
// requires the normalized transcript to contain EVERY --expect word
// (case-insensitive, punctuation-stripped, digit/number-word canonicalized —
// the documented, real Whisper text-normalization behavior per
// docs/qa/phase3_whisper_stt_20260707/RESULTS.md, never a weakened check).
// --expect-fail inverts which raw outcome counts as case success.
//
// Usage:
//
//	helixqa-verify-whisper \
//	  --wav fox.wav --expect "quick,brown,fox,lazy,dog" \
//	  --out qa-results/helixllm_whisper/fox_verdict.json \
//	  [--endpoint http://localhost:18437/v1/audio/transcriptions] \
//	  [--conduit-dir qa-results/helixllm_whisper/conduit]
//	  [--challenge-id STT-FOX-001] [--timeout 60s] [--expect-fail]
//
// Exit codes: 0 -> case_result==true; 1 -> case_result==false; 2 -> infra error.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/conduit"
)

const (
	exitPass  = 0
	exitFail  = 1
	exitInfra = 2
)

type sttResponse struct {
	Text            string  `json:"text"`
	Language        string  `json:"language"`
	NoSpeechProb    float64 `json:"no_speech_prob"`
	LanguageProb    float64 `json:"language_probability"`
}

type verdict struct {
	WavPath       string   `json:"wav_path"`
	Endpoint      string   `json:"endpoint"`
	Transcript    string   `json:"transcript"`
	Normalized    string   `json:"normalized"`
	ExpectedWords []string `json:"expected_words"`
	MatchedWords  int      `json:"matched_words"`
	ExpectedCount int      `json:"expected_count"`
	Missing       []string `json:"missing,omitempty"`
	Language      string   `json:"language"`
	NoSpeechProb  float64  `json:"no_speech_prob"`
	Pass          bool     `json:"pass"`
	ExpectFail    bool     `json:"expect_fail"`
	CaseResult    bool     `json:"case_result"`
	LatencyMS     int64    `json:"latency_ms"`
	HTTPStatus    int      `json:"http_status"`
	Error         string   `json:"error,omitempty"`
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func normalize(s string) string {
	s = strings.ToLower(s)
	s = nonAlnum.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

// numberWordToDigit / digitToNumberWord / canonToken: the SAME documented
// real Whisper number-normalization equivalence as
// docs/qa/phase3_whisper_stt_20260707/harness/main.go (§11.4.120 — honest
// reconciliation of a real, evidenced engine behavior, never a weakening).
var numberWordToDigit = map[string]string{
	"zero": "0", "one": "1", "two": "2", "three": "3", "four": "4",
	"five": "5", "six": "6", "seven": "7", "eight": "8", "nine": "9", "ten": "10",
}
var digitToNumberWord = func() map[string]string {
	m := map[string]string{}
	for w, d := range numberWordToDigit {
		m[d] = w
	}
	return m
}()

func canonToken(tok string) string {
	if w, ok := digitToNumberWord[tok]; ok {
		return w
	}
	return tok
}

func main() {
	os.Exit(run())
}

func run() int {
	var (
		wavPath    = flag.String("wav", "", "path to the WAV fixture (required)")
		expectCSV  = flag.String("expect", "", "comma-separated required words (case-insensitive, canonicalized)")
		endpoint   = flag.String("endpoint", envOr("HELIXLLM_STT_ENDPOINT", "http://localhost:18437/v1/audio/transcriptions"), "Whisper STT /v1/audio/transcriptions endpoint")
		out        = flag.String("out", "", "path to write the verdict JSON (required)")
		conduitDir = flag.String("conduit-dir", "", "optional conduit JSONL event dir (§11.4.116)")
		challID    = flag.String("challenge-id", "", "challenge id for conduit events (defaults to --out basename)")
		timeout    = flag.Duration("timeout", 60*time.Second, "request timeout")
		expectFail = flag.Bool("expect-fail", false, "invert case-level exit code — for golden-bad self-validation fixtures")
	)
	flag.Parse()

	if *wavPath == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-whisper --wav <file.wav> --out <verdict.json> [--expect a,b]")
		return exitInfra
	}
	cid := *challID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_whisper", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}
	conduit.ChallengeStart(sink, cid, "stt")

	v := verdict{
		WavPath:       *wavPath,
		Endpoint:      *endpoint,
		ExpectedWords: splitCSV(*expectCSV),
	}
	v.ExpectedCount = len(v.ExpectedWords)

	wavBytes, err := os.ReadFile(*wavPath)
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("read wav: %v", err))
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", filepath.Base(*wavPath))
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("multipart create: %v", err))
	}
	if _, err := fw.Write(wavBytes); err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("multipart write: %v", err))
	}
	if err := mw.Close(); err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("multipart close: %v", err))
	}

	client := &http.Client{Timeout: *timeout}
	start := time.Now()
	httpReq, err := http.NewRequest(http.MethodPost, *endpoint, &body)
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("build request: %v", err))
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := client.Do(httpReq)
	latency := time.Since(start)
	v.LatencyMS = latency.Milliseconds()
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("http request: %v", err))
	}
	defer resp.Body.Close()
	v.HTTPStatus = resp.StatusCode
	respBytes, _ := io.ReadAll(resp.Body)

	var sr sttResponse
	if decErr := json.Unmarshal(respBytes, &sr); decErr != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("decode response: %v", decErr))
	}
	if resp.StatusCode != http.StatusOK {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("non-200 status=%d", resp.StatusCode))
	}
	v.Transcript = sr.Text
	v.Language = sr.Language
	v.NoSpeechProb = sr.NoSpeechProb
	v.Normalized = normalize(v.Transcript)

	// --- word-match with digit/number-word canonicalization (§11.4.6/§11.4.120) ---
	tokens := map[string]bool{}
	for _, t := range strings.Fields(v.Normalized) {
		tokens[canonToken(t)] = true
	}
	matched := 0
	for _, w := range v.ExpectedWords {
		if tokens[canonToken(strings.ToLower(w))] {
			matched++
		} else {
			v.Missing = append(v.Missing, w)
		}
	}
	v.MatchedWords = matched
	v.Pass = v.Normalized != "" && v.ExpectedCount > 0 && v.MatchedWords == v.ExpectedCount

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

	conduit.EvidenceCaptured(sink, cid, "stt_verdict_json", *out)

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, "")
		fmt.Printf("PASS: %s matched=%d/%d missing=%v expect_fail=%v raw_pass=%v transcript=%q\n",
			cid, v.MatchedWords, v.ExpectedCount, v.Missing, v.ExpectFail, v.Pass, v.Transcript)
		return exitPass
	}
	reason := fmt.Sprintf("matched=%d/%d missing=%v expect_fail=%v raw_pass=%v", v.MatchedWords, v.ExpectedCount, v.Missing, v.ExpectFail, v.Pass)
	conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, reason)
	fmt.Printf("FAIL: %s %s transcript=%q\n", cid, reason, v.Transcript)
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
