// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-vision is the RUNNABLE analyzer for HelixQA
// Bank B (`banks/helixllm_vision.yaml`) — it drives a real multimodal
// completion against a live HelixLLM vision (VLM) endpoint, writes a
// machine-readable verdict artefact, and exits with a PASS/FAIL code
// that the bank's `RequiredEvidence` content assertion (§1.2 of
// docs/research/07.2026/12_helixqa_testing/12_helixqa_testing.md)
// gates on.
//
// It is the "thin per-capability dispatches_to analyzer" that design
// document anticipates ("no core-engine change, per CONST-051(B)
// decoupling") — it reuses stdlib HTTP + JSON only, no HelixQA core
// change.
//
// ANTI-BLUFF (§11.4.6 / §11.4.69 / §11.4.107(10) / §11.4.123). The
// verdict is a STRICT fact-matching check over the model's genuine
// response text — never a lenient/partial match, never a hardcoded
// PASS. Every expected fact token (--expect) MUST appear in the
// response (case-insensitive substring) for pass=true; a forbidden
// token (--forbid) appearing anywhere makes hallucinated=true and
// forces pass=false regardless of matched_facts. This is the single
// analyzer used for BOTH the golden-good fixture (must PASS) and the
// golden-bad fixture (must FAIL, VIS-SELF-VALIDATE-001) — the discipline
// closing the "vision endpoint responds to /v1/models is not proof of
// correctness" gap (AB-15, capabilities plan §2.5).
//
// Usage:
//
//	helixqa-verify-vision \
//	  --image data/vision_gt/red_circle.png \
//	  --prompt "What shape and what color is the main object in this image?" \
//	  --expect "circle,red" \
//	  --out qa-results/helixllm_vision/understand_001_verdict.json \
//	  [--forbid "square,blue"] \
//	  [--endpoint http://localhost:18439/v1/chat/completions] \
//	  [--model vision] [--conduit-dir qa-results/helixllm_vision/conduit] \
//	  [--challenge-id VIS-UNDERSTAND-001] [--timeout 60s] [--max-tokens 150] \
//	  [--expect-fail]
//
// --expect-fail (golden-bad self-validation cases, mirrors the existing
// `panoptic-validate-recording --expect-fail` bank convention used by
// helixcode-ensemble-members.yaml HXC-ENS-003): inverts which raw outcome
// counts as case success. The RAW fact-match verdict is ALWAYS recorded
// honestly in the verdict JSON's "pass" field; "case_result" is the
// (possibly-inverted) field the bank's RequiredEvidence gates on.
//
// Exit codes (machine-readable for the Dispatcher, pkg/testbank/dispatch.go):
//
//	0 -> case_result==true (this case succeeded)
//	1 -> case_result==false (fact mismatch / hallucination / analyzer wrongly
//	     accepted a golden-bad fixture under --expect-fail)
//	2 -> infra/usage error (endpoint unreachable, bad flags, image unreadable)
//
// DECOUPLING (CONST-051(B) / §11.4.28). The endpoint URL, model id,
// image path, prompt, and expected/forbidden fact tokens are ALL
// flags/bank data — nothing about the consuming project (HelixLLM's
// vision port, the fixture corpus) is hardcoded in this tool beyond a
// documented default matching HelixLLM's on-demand-infra convention
// (:18439, §11.4.76).
package main

import (
	"bytes"
	"encoding/base64"
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

// chatContentPart mirrors the OpenAI-compatible multimodal content-part
// shape (pkg/api/openai.go ContentPart on the HelixLLM side / the
// llama.cpp multimodal server's own OpenAI-compatible wire format).
type chatContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type chatMessage struct {
	Role    string            `json:"role"`
	Content []chatContentPart `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
	Messages    []chatMessage `json:"messages"`
}

type chatChoice struct {
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
	Message      struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
}

type chatResponse struct {
	ID      string       `json:"id"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// verdict is the machine-readable artefact the ContentAssertingResolver
// gates on (embeds the raw response + ground-truth + metric per Part 4
// item 2 of the design doc — auditable, never trust-the-analyzer-blindly).
type verdict struct {
	Image          string   `json:"image"`
	Prompt         string   `json:"prompt"`
	Endpoint       string   `json:"endpoint"`
	ModelRequested string   `json:"model_requested"`
	ModelServed    string   `json:"model_served"`
	ExpectedFacts  []string `json:"expected_facts"`
	ForbiddenFacts []string `json:"forbidden_facts,omitempty"`
	Response       string   `json:"response"`
	MatchedFacts   int      `json:"matched_facts"`
	ExpectedCount  int      `json:"expected_count"`
	Hallucinated   bool     `json:"hallucinated"`
	Pass           bool     `json:"pass"`
	ExpectFail     bool     `json:"expect_fail"`
	CaseResult     bool     `json:"case_result"`
	LatencyMS      int64    `json:"latency_ms"`
	HTTPStatus     int      `json:"http_status"`
	PromptTokens   int      `json:"prompt_tokens"`
	CompletionToks int      `json:"completion_tokens"`
	Error          string   `json:"error,omitempty"`
}

func main() {
	os.Exit(run())
}

func run() int {
	var (
		image       = flag.String("image", "", "path to the ground-truth image (required)")
		prompt      = flag.String("prompt", "", "prompt to send alongside the image (required)")
		expectCSV   = flag.String("expect", "", "comma-separated required fact tokens (case-insensitive substring match)")
		forbidCSV   = flag.String("forbid", "", "comma-separated forbidden fact tokens (presence forces hallucinated=true)")
		endpoint    = flag.String("endpoint", envOr("HELIXLLM_VISION_ENDPOINT", "http://localhost:18439/v1/chat/completions"), "vision chat/completions endpoint")
		model       = flag.String("model", envOr("HELIXLLM_VISION_MODEL", "vision"), "model id to request")
		out         = flag.String("out", "", "path to write the verdict JSON (required)")
		conduitDir  = flag.String("conduit-dir", "", "optional conduit JSONL event dir (§11.4.116)")
		challengeID = flag.String("challenge-id", "", "challenge id for conduit events (defaults to --out basename)")
		timeout     = flag.Duration("timeout", 60*time.Second, "request timeout")
		maxTokens   = flag.Int("max-tokens", 150, "max_tokens for the completion request")
		expectFail  = flag.Bool("expect-fail", false, "invert the case-level exit code — for golden-bad self-validation fixtures (mirrors the panoptic-validate-recording --expect-fail bank convention: the RAW fact-match verdict (\"pass\") is still recorded honestly; \"case_result\" is what the bank's RequiredEvidence gates on)")
	)
	flag.Parse()

	if *image == "" || *prompt == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-vision --image <png> --prompt <text> --out <verdict.json> [--expect a,b] [--forbid c,d] [--endpoint url] [--model id]")
		return exitInfra
	}
	cid := *challengeID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_vision", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}
	conduit.ChallengeStart(sink, cid, "vision")

	v := verdict{
		Image:          *image,
		Prompt:         *prompt,
		Endpoint:       *endpoint,
		ModelRequested: *model,
		ExpectedFacts:  splitCSV(*expectCSV),
		ForbiddenFacts: splitCSV(*forbidCSV),
	}
	v.ExpectedCount = len(v.ExpectedFacts)

	imgBytes, err := os.ReadFile(*image)
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("read image: %v", err))
	}
	b64 := base64.StdEncoding.EncodeToString(imgBytes)

	reqBody := chatRequest{
		Model:       *model,
		Temperature: 0,
		MaxTokens:   *maxTokens,
		Messages: []chatMessage{
			{
				Role: "user",
				Content: []chatContentPart{
					{Type: "text", Text: *prompt},
					{Type: "image_url", ImageURL: &imageURL{URL: "data:image/png;base64," + b64}},
				},
			},
		},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("marshal request: %v", err))
	}

	client := &http.Client{Timeout: *timeout}
	start := time.Now()
	httpReq, err := http.NewRequest(http.MethodPost, *endpoint, bytes.NewReader(payload))
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

	var cr chatResponse
	if decErr := json.NewDecoder(resp.Body).Decode(&cr); decErr != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("decode response: %v", decErr))
	}
	if resp.StatusCode != http.StatusOK || len(cr.Choices) == 0 {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("non-200 or empty choices (status=%d)", resp.StatusCode))
	}

	v.ModelServed = cr.Model
	v.Response = cr.Choices[0].Message.Content
	v.PromptTokens = cr.Usage.PromptTokens
	v.CompletionToks = cr.Usage.CompletionTokens

	// --- strict fact-matching (§11.4.6 / §11.4.107(10)) ---
	lowerResp := strings.ToLower(v.Response)
	matched := 0
	for _, f := range v.ExpectedFacts {
		if strings.Contains(lowerResp, strings.ToLower(f)) {
			matched++
		}
	}
	v.MatchedFacts = matched

	hallucinated := false
	for _, f := range v.ForbiddenFacts {
		if f != "" && strings.Contains(lowerResp, strings.ToLower(f)) {
			hallucinated = true
			break
		}
	}
	v.Hallucinated = hallucinated
	v.Pass = v.ExpectedCount > 0 && v.MatchedFacts == v.ExpectedCount && !v.Hallucinated

	// --expect-fail (golden-bad self-validation, §11.4.107(10)): the RAW
	// fact-match verdict ("pass") is recorded honestly and NEVER flipped —
	// what inverts is which raw outcome counts as this CASE succeeding.
	// A golden-bad fixture's case_result is true iff the analyzer correctly
	// REJECTED it (pass==false); if a broken/mutated analyzer wrongly
	// accepts it (pass==true), case_result is false and the case FAILs —
	// exactly the discriminator VIS-SELF-VALIDATE-001 requires.
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

	conduit.VisionCall(sink, cid, latency, map[string]any{
		"model_served":      v.ModelServed,
		"prompt_tokens":     v.PromptTokens,
		"completion_tokens": v.CompletionToks,
		"latency_ms":        v.LatencyMS,
	})
	conduit.EvidenceCaptured(sink, cid, "vision_verdict_json", *out)

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, "")
		fmt.Printf("PASS: %s matched=%d/%d hallucinated=%v expect_fail=%v raw_pass=%v response=%q\n", cid, v.MatchedFacts, v.ExpectedCount, v.Hallucinated, v.ExpectFail, v.Pass, v.Response)
		return exitPass
	}
	reason := fmt.Sprintf("matched=%d/%d hallucinated=%v expect_fail=%v raw_pass=%v", v.MatchedFacts, v.ExpectedCount, v.Hallucinated, v.ExpectFail, v.Pass)
	conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, reason)
	fmt.Printf("FAIL: %s %s response=%q\n", cid, reason, v.Response)
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
