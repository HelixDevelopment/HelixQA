// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-embeddings is the RUNNABLE analyzer for the
// HelixQA CPU embeddings bank (banks/helixllm_embeddings.yaml) — it drives
// a real POST /v1/embeddings against the live HelixLLM CPU embeddings
// service (HF Text Embeddings Inference, default :18435, boot mechanism
// reused from docs/qa/phase3_embeddings_20260706/harness per §11.4.74),
// applies the semantic-order cosine-margin runtime signature the phase-3
// proof harness already established, and writes a machine-readable verdict
// artefact the bank's RequiredEvidence gates on. Mirrors the CLI convention
// of cmd/helixqa-verify-vision (--out/--conduit-dir/--challenge-id/
// --expect-fail, exit 0/1/2).
//
// ANTI-BLUFF (§11.4.6/§11.4.69/§11.4.107(10)/§11.4.123). PASS requires: (1)
// all three vectors non-zero-norm and equal-dimension, (2) cos(A,A') >
// cos(A,U) by at least --margin-floor (default 0.15 — A/A' is a
// related/paraphrase pair, A/U is unrelated; semantic ordering is the
// unfakeable signature a zero-vector/shuffled/wrong-dim stub cannot
// produce). --expect-fail inverts case success.
//
// Usage:
//
//	helixqa-verify-embeddings \
//	  --text-a "The cat sat on the mat." \
//	  --text-a-prime "A feline rested on the rug." \
//	  --text-u "Quarterly revenue rose four percent." \
//	  --margin-floor 0.15 \
//	  --out qa-results/helixllm_embeddings/margin_verdict.json \
//	  [--endpoint http://localhost:18435/v1/embeddings] \
//	  [--conduit-dir qa-results/helixllm_embeddings/conduit]
//	  [--challenge-id EMB-MARGIN-001] [--timeout 60s] [--expect-fail]
//
// Exit codes: 0 -> case_result==true; 1 -> case_result==false; 2 -> infra error.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
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

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingItem struct {
	Embedding []float64 `json:"embedding"`
}

type embeddingResponse struct {
	Data  []embeddingItem `json:"data"`
	Model string          `json:"model"`
}

type verdict struct {
	TextA       string  `json:"text_a"`
	TextAPrime  string  `json:"text_a_prime"`
	TextU       string  `json:"text_u"`
	Endpoint    string  `json:"endpoint"`
	ModelServed string  `json:"model_served"`
	Dim         int     `json:"dim"`
	NormA       float64 `json:"norm_a"`
	NormAPrime  float64 `json:"norm_a_prime"`
	NormU       float64 `json:"norm_u"`
	CosAAPrime  float64 `json:"cos_a_aprime"`
	CosAU       float64 `json:"cos_a_u"`
	Margin      float64 `json:"margin"`
	MarginFloor float64 `json:"margin_floor"`
	Pass        bool    `json:"pass"`
	ExpectFail  bool    `json:"expect_fail"`
	CaseResult  bool    `json:"case_result"`
	LatencyMS   int64   `json:"latency_ms"`
	HTTPStatus  int     `json:"http_status"`
	Error       string  `json:"error,omitempty"`
}

func l2norm(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x * x
	}
	return math.Sqrt(s)
}

func cosine(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return math.NaN()
	}
	var dot float64
	for i := range a {
		dot += a[i] * b[i]
	}
	na, nb := l2norm(a), l2norm(b)
	if na == 0 || nb == 0 {
		return math.NaN()
	}
	return dot / (na * nb)
}

func main() {
	os.Exit(run())
}

func run() int {
	var (
		textA       = flag.String("text-a", "The cat sat on the mat.", "anchor sentence")
		textAPrime  = flag.String("text-a-prime", "A feline rested on the rug.", "related/paraphrase sentence")
		textU       = flag.String("text-u", "Quarterly revenue rose four percent.", "unrelated sentence")
		marginFloor = flag.Float64("margin-floor", 0.15, "minimum required cos(A,A')-cos(A,U) margin")
		endpoint    = flag.String("endpoint", envOr("HELIXLLM_EMBED_ENDPOINT", "http://localhost:18435/v1/embeddings"), "TEI /v1/embeddings endpoint")
		out         = flag.String("out", "", "path to write the verdict JSON (required)")
		conduitDir  = flag.String("conduit-dir", "", "optional conduit JSONL event dir (§11.4.116)")
		challID     = flag.String("challenge-id", "", "challenge id for conduit events (defaults to --out basename)")
		timeout     = flag.Duration("timeout", 60*time.Second, "request timeout")
		expectFail  = flag.Bool("expect-fail", false, "invert case-level exit code — for golden-bad self-validation fixtures")
	)
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-embeddings --out <verdict.json> [--text-a ...] [--text-a-prime ...] [--text-u ...] [--margin-floor 0.15]")
		return exitInfra
	}
	cid := *challID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_embeddings", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}
	conduit.ChallengeStart(sink, cid, "embeddings")

	v := verdict{
		TextA:       *textA,
		TextAPrime:  *textAPrime,
		TextU:       *textU,
		Endpoint:    *endpoint,
		MarginFloor: *marginFloor,
	}

	reqBody, _ := json.Marshal(embeddingRequest{Model: "helix-embed", Input: []string{*textA, *textAPrime, *textU}})
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

	var er embeddingResponse
	if decErr := json.NewDecoder(resp.Body).Decode(&er); decErr != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("decode response: %v", decErr))
	}
	if resp.StatusCode != http.StatusOK {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("non-200 status=%d", resp.StatusCode))
	}
	if len(er.Data) < 3 {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("need 3 vectors, got %d", len(er.Data)))
	}
	v.ModelServed = er.Model

	a, ap, u := er.Data[0].Embedding, er.Data[1].Embedding, er.Data[2].Embedding
	v.Dim = len(a)
	v.NormA = l2norm(a)
	v.NormAPrime = l2norm(ap)
	v.NormU = l2norm(u)
	v.CosAAPrime = cosine(a, ap)
	v.CosAU = cosine(a, u)
	v.Margin = v.CosAAPrime - v.CosAU

	// --- semantic-order cosine-margin signature (§11.4.6/§11.4.107(10)) ---
	dimOK := len(a) == len(ap) && len(ap) == len(u) && len(a) > 0
	normOK := v.NormA > 0 && v.NormAPrime > 0 && v.NormU > 0
	v.Pass = dimOK && normOK && !math.IsNaN(v.Margin) && v.Margin >= v.MarginFloor

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

	conduit.EvidenceCaptured(sink, cid, "embeddings_verdict_json", *out)

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, "")
		fmt.Printf("PASS: %s dim=%d cos(A,A')=%.4f cos(A,U)=%.4f margin=%.4f expect_fail=%v raw_pass=%v\n",
			cid, v.Dim, v.CosAAPrime, v.CosAU, v.Margin, v.ExpectFail, v.Pass)
		return exitPass
	}
	reason := fmt.Sprintf("dim=%d cos(A,A')=%.4f cos(A,U)=%.4f margin=%.4f expect_fail=%v raw_pass=%v", v.Dim, v.CosAAPrime, v.CosAU, v.Margin, v.ExpectFail, v.Pass)
	conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, reason)
	fmt.Printf("FAIL: %s %s\n", cid, reason)
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

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
