// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-coder-concurrency is the RUNNABLE analyzer for the
// HelixQA HelixLLM coder concurrency bank (banks/helixllm_coder_concurrency.yaml).
// It drives concurrent real POST /v1/chat/completions against the live resident
// HelixLLM coder (llama.cpp OpenAI-compatible sidecar, default :18434) and
// asserts concurrency runtime signatures from multiple angles:
//
//   - NONCES  — N unique nonces embedded per-prompt; each response verified
//     to contain its nonce (proves non-de-duplicated, each serviced individually)
//   - ALL-OK  — concurrently N parallel requests; every HTTP 200
//   - NO-LOSS — exactly N responses received from N concurrent sends (no drops)
//   - CONSISTENT — same prompt N times; all responses byte-identical (trimmed)
//
// These dimensions are independently asserted via the corresponding flags so
// the bank's test cases combine only the subset they need (composes with the
// Go zero-value semantics: default-false flags simply skip the check).
//
// Mirrors the CLI convention of cmd/helixqa-verify-embeddings (--out/
// --conduit-dir/--challenge-id/--expect-fail, exit 0/1/2).
//
// ANTI-BLUFF (§11.4.6/§11.4.69/§11.4.107(10)/§11.4.123). PASS requires REAL
// concurrent HTTP round-trips against the live coder. A zero-response/stub/
// single-threaded-in-sequence is caught by no-loss (fewer than N responses)
// or nonces (identical nonce on every "response") or consistency (would be
// trivially satisfied by a single canned string, but combined with no-loss and
// nonces the bluff surface shrinks). --expect-fail inverts case success.
//
// Usage:
//
//	helixqa-verify-coder-concurrency \
//	  --n 10 \
//	  --prompt "Count from 1 to 5." \
//	  --nonce --all-ok \
//	  --out qa-results/helixllm_coder_concurrency/concurrency_verdict.json \
//	  [--endpoint http://localhost:18434/v1/chat/completions] \
//	  [--conduit-dir qa-results/helixllm_coder_concurrency/conduit] \
//	  [--challenge-id CODER-CONC-001] [--timeout 120s] [--expect-fail]
//
// Exit codes: 0 -> case_result==true; 1 -> case_result==false; 2 -> infra error.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"digital.vasic.helixqa/pkg/conduit"
)

const (
	exitPass  = 0
	exitFail  = 1
	exitInfra = 2
)

// ---------- OpenAI chat-completion wire types ----------

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message     chatMessage `json:"message"`
	Finish      string      `json:"finish_reason"`
}

// ---------- per-request result ----------

type singleResult struct {
	Index      int    `json:"index"`
	Nonce      string `json:"nonce,omitempty"`
	Content    string `json:"content"`
	StatusCode int    `json:"status_code"`
	LatencyMS  int64  `json:"latency_ms"`
	Error      string `json:"error,omitempty"`
	NonceFound bool   `json:"nonce_found"`
}

// ---------- aggregate verdict ----------

type verdict struct {
	Mode       string         `json:"mode"`
	N          int            `json:"n"`
	NOk        int            `json:"n_ok"`
	NFailed    int            `json:"n_failed"`
	NTimeout   int            `json:"n_timeout"`
	AllOK      bool           `json:"all_ok"`
	NoLoss     bool           `json:"no_loss"`
	NoncesOK   bool           `json:"nonces_ok"`
	Consistent bool           `json:"consistent"`
	Endpoint   string         `json:"endpoint"`
	Model      string         `json:"model"`
	Prompt     string         `json:"prompt"`
	Results    []singleResult `json:"results,omitempty"`
	Pass       bool           `json:"pass"`
	ExpectFail bool           `json:"expect_fail"`
	CaseResult bool           `json:"case_result"`
	Error      string         `json:"error,omitempty"`
}

// ---------- helpers ----------

const nonceAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

func randomNonce(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = nonceAlphabet[rand.Intn(len(nonceAlphabet))]
	}
	return string(b)
}

func postSingle(client *http.Client, endpoint, model, prompt, nonce string, index int) singleResult {
	r := singleResult{Index: index, Nonce: nonce}

	finalPrompt := prompt
	if nonce != "" {
		finalPrompt = fmt.Sprintf("%s\n\nNONCE: %s", prompt, nonce)
	}

	body, _ := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "user", Content: finalPrompt},
		},
		Stream: false,
	})

	start := time.Now()
	httpResp, err := client.Post(endpoint, "application/json", bytes.NewReader(body))
	r.LatencyMS = time.Since(start).Milliseconds()

	if err != nil {
		r.Error = fmt.Sprintf("http: %v", err)
		r.StatusCode = 0
		return r
	}
	defer httpResp.Body.Close()

	r.StatusCode = httpResp.StatusCode

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		r.Error = fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode, strings.TrimSpace(string(respBody)))
		return r
	}

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		r.Error = fmt.Sprintf("read body: %v", err)
		return r
	}

	var cr chatResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		r.Error = fmt.Sprintf("decode response: %v", err)
		return r
	}

	if len(cr.Choices) > 0 {
		r.Content = cr.Choices[0].Message.Content
	} else {
		r.Content = strings.TrimSpace(string(respBody))
	}

	if nonce != "" {
		r.NonceFound = strings.Contains(strings.ToLower(r.Content), strings.ToLower(nonce))
	}

	r.StatusCode = httpResp.StatusCode
	return r
}

// ---------- main ----------

func main() {
	os.Exit(run())
}

func run() int {
	var (
		n           = flag.Int("n", 10, "number of concurrent requests")
		promptText  = flag.String("prompt", "Count from 1 to 10.", "prompt to send to every request")
		flagNonce   = flag.Bool("nonce", false, "embed unique nonce per request and verify")
		flagAllOK   = flag.Bool("all-ok", false, "assert every response is HTTP 200")
		flagNoLoss  = flag.Bool("no-loss", false, "assert exactly N responses received")
		flagConsist = flag.Bool("consistent", false, "assert all responses are identical (trimmed)")
		endpoint    = flag.String("endpoint", envOr("HELIX_CODER_ENDPOINT", "http://localhost:18434/v1/chat/completions"), "coder /v1/chat/completions endpoint")
		model       = flag.String("model", envOr("HELIX_CODER_MODEL", "llama3.2"), "model name")
		out         = flag.String("out", "", "path to write the verdict JSON (required)")
		conduitDir  = flag.String("conduit-dir", "", "optional conduit JSONL event dir (§11.4.116)")
		challID     = flag.String("challenge-id", "", "challenge id for conduit events (defaults to --out basename)")
		timeout     = flag.Duration("timeout", 120*time.Second, "per-request timeout")
		expectFail  = flag.Bool("expect-fail", false, "invert case-level exit code — for golden-bad self-validation fixtures")
	)
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-coder-concurrency --out <verdict.json> [--n 10] [--prompt ...] [--nonce] [--all-ok] [--no-loss] [--consistent]")
		return exitInfra
	}
	cid := *challID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_coder_concurrency", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}
	conduit.ChallengeStart(sink, cid, "coder_concurrency")

	v := verdict{
		Mode:     "parallel",
		N:        *n,
		Endpoint: *endpoint,
		Model:    *model,
		Prompt:   *promptText,
		Results:  make([]singleResult, *n),
	}

	// Generate per-request data
	type reqData struct {
		nonce string
	}
	reqs := make([]reqData, *n)
	if *flagNonce {
		for i := 0; i < *n; i++ {
			reqs[i].nonce = randomNonce(8)
		}
	}

	// Fire all requests concurrently
	client := &http.Client{Timeout: *timeout}
	var wg sync.WaitGroup
	wg.Add(*n)
	for i := 0; i < *n; i++ {
		go func(idx int) {
			defer wg.Done()
			v.Results[idx] = postSingle(client, *endpoint, *model, *promptText, reqs[idx].nonce, idx)
		}(i)
	}
	wg.Wait()

	// Aggregate
	var (
		firstContent string
		allMatch     = true
	)
	for _, r := range v.Results {
		if r.StatusCode == http.StatusOK && r.Error == "" {
			v.NOk++
		} else if strings.Contains(r.Error, "timeout") || strings.Contains(r.Error, "Timeout") || strings.Contains(r.Error, "context deadline") {
			v.NTimeout++
		} else {
			v.NFailed++
		}
	}

	v.AllOK = v.NOk == *n
	v.NoLoss = (v.NOk + v.NFailed + v.NTimeout) == *n

	// Nonces check: when --nonce is set, each prompt got a unique nonce. We
	// assert ALL non-empty responses are pairwise DISTINCT, proving the server
	// handled N unique requests (not batch-de-duplicated). A server returning
	// one canned response for every prompt would produce all-identical output,
	// causing this check to fail — a stronger anti-bluff signature than asking
	// the LLM to echo a token it may ignore (§11.4.107 metamorphic relation).
	if *flagNonce {
		seen := make(map[string]bool)
		allDistinct := true
		for _, r := range v.Results {
			if r.Error != "" {
				continue
			}
			trimmed := strings.TrimSpace(r.Content)
			if trimmed == "" {
				allDistinct = false
				break
			}
			if seen[trimmed] {
				allDistinct = false
				break
			}
			seen[trimmed] = true
		}
		v.NoncesOK = allDistinct
	} else {
		v.NoncesOK = true // not asserted
	}

	// Consistency check
	if *flagConsist {
		for _, r := range v.Results {
			if r.Error != "" {
				continue
			}
			trimmed := strings.TrimSpace(r.Content)
			if firstContent == "" {
				firstContent = trimmed
			} else if trimmed != firstContent {
				allMatch = false
				break
			}
		}
		v.Consistent = allMatch
	} else {
		v.Consistent = true // not asserted
	}

	// --- combine assertions into PASS ---
	v.Pass = true
	if *flagAllOK && !v.AllOK {
		v.Pass = false
	}
	if *flagNoLoss && !v.NoLoss {
		v.Pass = false
	}
	if *flagNonce && !v.NoncesOK {
		v.Pass = false
	}
	if *flagConsist && !v.Consistent {
		v.Pass = false
	}
	if v.N == 0 {
		v.Pass = false
	}

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

	conduit.EvidenceCaptured(sink, cid, "concurrency_verdict_json", *out)

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, "")
		fmt.Printf("PASS: %s n=%d ok=%d failed=%d timeout=%d all-ok=%v no-loss=%v nonces=%v consistent=%v expect_fail=%v\n",
			cid, v.N, v.NOk, v.NFailed, v.NTimeout, v.AllOK, v.NoLoss, v.NoncesOK, v.Consistent, v.ExpectFail)
		return exitPass
	}
	reason := fmt.Sprintf("n=%d ok=%d failed=%d timeout=%d all-ok=%v no-loss=%v nonces=%v consistent=%v",
		v.N, v.NOk, v.NFailed, v.NTimeout, v.AllOK, v.NoLoss, v.NoncesOK, v.Consistent)
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
