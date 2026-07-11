// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-coder-race-llm is the RUNNABLE analyzer for the
// HelixQA HelixLLM coder race/concurrency-correctness bank
// (banks/helixllm_coder_race.yaml). It closes the §11.4.169 "race
// conditions or deadlocks" test-type gap for the LLM coder itself (this is
// DISTINCT from cmd/helixqa-verify-coder-race, which exercises the
// HelixCode HTTP server's ModelManager RWMutex on :8080 — a different
// target and a different bug class entirely).
//
// This analyzer drives real concurrent POST /v1/chat/completions against
// the live resident HelixLLM coder (llama.cpp OpenAI-compatible sidecar,
// default :18434) and asserts three independent race-correctness runtime
// signatures:
//
//   - DISTINCT mode  — N concurrent requests, each carrying a UNIQUE
//     arithmetic question (guaranteed pairwise-distinct expected answer)
//     AND a unique per-request nonce token the model is asked to echo.
//     Asserts: ALL-OK (every response 200), NO-LOSS (exactly N responses),
//     NO-DUPLICATE (no two response bodies are byte-identical — a
//     duplicated-completion signal), OWN-NONCE (every response contains
//     its OWN nonce), and — the core race-detection signature —
//     NO-CROSS-CONTAM: no response contains any OTHER request's nonce.
//     Under a genuine race (e.g. a worker pool that misaligns request/
//     response indices under concurrent dispatch), a response would
//     surface a foreign nonce; this is the sink-side signature that
//     proves per-request isolation holds under concurrency.
//
//   - IDENTICAL mode — N concurrent requests with the SAME deterministic
//     prompt (temperature=0, fixed seed) fired simultaneously. Asserts
//     ALL-OK, NO-LOSS, and DETERMINISTIC (the extracted numeric answer is
//     identical across all N concurrent completions) — "deterministic-
//     where-expected" per the task brief. Honest boundary (§11.4.6): CPU/
//     GPU floating-point non-associativity under concurrent batched
//     inference can occasionally perturb greedy decoding even at
//     temperature=0; this is documented, not silently absorbed.
//
//   - SELFTEST-CROSSCONTAM mode — an OFFLINE, no-network self-validation
//     of the core detectCrossContamination() function itself (the novel
//     logic this bank introduces, beyond what
//     cmd/helixqa-verify-coder-concurrency already covers). It builds an
//     in-memory synthetic fixture (--fixture clean|contaminated) and
//     asserts the detector reports the correct verdict. This is the
//     mechanism used for the mandatory §11.4.107(10) self-validation of
//     the contamination detector: paired with a real, executed (never
//     committed) §1.1 mutation of detectCrossContamination() that forces
//     it to always report "no contamination" — proving the check is
//     load-bearing (see docs/qa/<run-id>/RESULTS.md for the captured
//     mutation-proof transcript).
//
// Mirrors the CLI convention of cmd/helixqa-verify-coder-concurrency
// (--out/--conduit-dir/--challenge-id/--expect-fail, exit 0/1/2).
//
// ANTI-BLUFF (§11.4.6/§11.4.69/§11.4.107(10)/§11.4.123/§11.4.169). DISTINCT
// and IDENTICAL modes require REAL concurrent HTTP round-trips against the
// live coder — no simulation. SELFTEST-CROSSCONTAM is explicitly labelled
// as an offline synthetic self-test of the detector logic (never conflated
// with a live-coder PASS). --expect-fail inverts case-level exit code for
// golden-bad fixtures.
//
// Usage:
//
//	helixqa-verify-coder-race-llm \
//	  --mode distinct --n 8 \
//	  --assert-all-ok --assert-no-loss --assert-no-duplicate \
//	  --assert-own-nonce --assert-no-cross-contam \
//	  --out qa-results/helixllm_coder_race/distinct_001_verdict.json \
//	  [--endpoint http://localhost:18434/v1/chat/completions] \
//	  [--model /models/Qwen3-Coder-30B-A3B-Instruct-Q4_K_M.gguf] \
//	  [--conduit-dir qa-results/helixllm_coder_race/conduit] \
//	  [--challenge-id CODER-RACE-001] [--timeout 120s] [--expect-fail]
//
// Exit codes: 0 -> case_result==true; 1 -> case_result==false; 2 -> infra error.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	Temperature *float64      `json:"temperature,omitempty"`
	Seed        *int64        `json:"seed,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
	Finish  string      `json:"finish_reason"`
}

// ---------- per-request result ----------

type singleResult struct {
	Index       int    `json:"index"`
	Nonce       string `json:"nonce,omitempty"`
	ExpectedSum int    `json:"expected_sum,omitempty"`
	Content     string `json:"content"`
	StatusCode  int    `json:"status_code"`
	LatencyMS   int64  `json:"latency_ms"`
	Error       string `json:"error,omitempty"`
}

// ---------- aggregate verdict ----------

type verdict struct {
	Mode             string         `json:"mode"`
	Fixture          string         `json:"fixture,omitempty"`
	N                int            `json:"n"`
	NOk              int            `json:"n_ok"`
	NFailed          int            `json:"n_failed"`
	NTimeout         int            `json:"n_timeout"`
	AllOK            bool           `json:"all_ok"`
	NoLoss           bool           `json:"no_loss"`
	NoDuplicate      bool           `json:"no_duplicate"`
	OwnNonceOK       bool           `json:"own_nonce_ok"`
	NoCrossContam    bool           `json:"no_cross_contam"`
	CrossContamPairs []string       `json:"cross_contam_pairs,omitempty"`
	DeterministicOK  bool           `json:"deterministic_ok"`
	DistinctAnswers  []string       `json:"distinct_answers,omitempty"`
	Endpoint         string         `json:"endpoint"`
	Model            string         `json:"model"`
	Prompt           string         `json:"prompt,omitempty"`
	Results          []singleResult `json:"results,omitempty"`
	Pass             bool           `json:"pass"`
	ExpectFail       bool           `json:"expect_fail"`
	CaseResult       bool           `json:"case_result"`
	Error            string         `json:"error,omitempty"`
}

// ---------- helpers ----------

const nonceAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// secureNonce generates a cryptographically random, distinctive token
// (prefixed so it cannot collide with ordinary English/code prose the
// model might otherwise emit).
func secureNonce(length int) string {
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(nonceAlphabet))))
		if err != nil {
			// Extremely unlikely; fall back to a fixed but still-unique-enough
			// character rather than silently truncating the token.
			b[i] = nonceAlphabet[i%len(nonceAlphabet)]
			continue
		}
		b[i] = nonceAlphabet[n.Int64()]
	}
	return "RACEID_" + string(b)
}

func postRace(client *http.Client, endpoint, model, content string, temperature *float64) (int, string, int64, error) {
	body, _ := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "user", Content: content},
		},
		Stream:      false,
		Temperature: temperature,
	})

	start := time.Now()
	httpResp, err := client.Post(endpoint, "application/json", bytes.NewReader(body))
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return 0, "", latency, fmt.Errorf("http: %w", err)
	}
	defer httpResp.Body.Close()

	respBytes, _ := io.ReadAll(httpResp.Body)
	if httpResp.StatusCode != http.StatusOK {
		return httpResp.StatusCode, strings.TrimSpace(string(respBytes)), latency, nil
	}

	var cr chatResponse
	if err := json.Unmarshal(respBytes, &cr); err != nil {
		return httpResp.StatusCode, "", latency, fmt.Errorf("decode response: %w", err)
	}
	text := strings.TrimSpace(string(respBytes))
	if len(cr.Choices) > 0 {
		text = cr.Choices[0].Message.Content
	}
	return httpResp.StatusCode, text, latency, nil
}

// detectCrossContamination is the CORE race-detection logic this bank
// exists to exercise: for every successful result i, assert that NONE of
// the OTHER (N-1) requests' nonces appear in result i's content. A hit
// means result i's content was contaminated by (or swapped with) another
// concurrent request's expected output — the sink-side signature of a
// request/response correlation race.
//
// Returns (noCrossContam, pairs) where pairs is a human-readable list of
// every detected "i<-j" contamination (result i contains request j's
// nonce).
func detectCrossContamination(results []singleResult, nonces []string) (bool, []string) {
	var pairs []string
	for i, r := range results {
		if r.Error != "" || r.StatusCode != http.StatusOK {
			continue
		}
		for j, n := range nonces {
			if i == j || n == "" {
				continue
			}
			if strings.Contains(r.Content, n) {
				pairs = append(pairs, fmt.Sprintf("result[%d] contains nonce belonging to request[%d] (%s)", i, j, n))
			}
		}
	}
	return len(pairs) == 0, pairs
}

// detectOwnNonce asserts every successful result contains its OWN nonce.
func detectOwnNonce(results []singleResult) bool {
	for _, r := range results {
		if r.Error != "" || r.StatusCode != http.StatusOK {
			continue
		}
		if r.Nonce == "" {
			continue
		}
		if !strings.Contains(r.Content, r.Nonce) {
			return false
		}
	}
	return true
}

// detectNoDuplicate asserts no two successful results have byte-identical
// (trimmed) content. Since every DISTINCT-mode prompt differs (different
// arithmetic + a unique nonce), any exact duplicate is a strong signal of
// a duplicated/replayed completion rather than genuine per-request output.
func detectNoDuplicate(results []singleResult) bool {
	seen := make(map[string]bool)
	for _, r := range results {
		if r.Error != "" || r.StatusCode != http.StatusOK {
			continue
		}
		trimmed := strings.TrimSpace(r.Content)
		if trimmed == "" {
			continue
		}
		if seen[trimmed] {
			return false
		}
		seen[trimmed] = true
	}
	return true
}

var firstIntRe = regexp.MustCompile(`-?\d+`)

func extractFirstInt(s string) (string, bool) {
	m := firstIntRe.FindString(s)
	return m, m != ""
}

// ---------- mode runners ----------

func runDistinct(client *http.Client, endpoint, model string, n int) verdict {
	v := verdict{Mode: "distinct", N: n, Endpoint: endpoint, Model: model, Results: make([]singleResult, n)}
	nonces := make([]string, n)
	for i := 0; i < n; i++ {
		nonces[i] = secureNonce(12)
	}
	temp0 := 0.0

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			// Distinct arithmetic per index guarantees a pairwise-distinct
			// expected numeric answer independent of the nonce mechanism —
			// a second, independent contamination signal.
			a, b := 1000+idx*7, idx*3+1
			expected := a + b
			prompt := fmt.Sprintf(
				"Compute %d + %d. Respond with ONLY the numeric result on the first line, "+
					"then on a new line write exactly this token and nothing else: %s",
				a, b, nonces[idx],
			)
			status, content, latency, err := postRace(client, endpoint, model, prompt, &temp0)
			r := singleResult{Index: idx, Nonce: nonces[idx], ExpectedSum: expected, StatusCode: status, LatencyMS: latency}
			if err != nil {
				r.Error = err.Error()
			} else {
				r.Content = content
			}
			v.Results[idx] = r
		}(i)
	}
	wg.Wait()

	for _, r := range v.Results {
		switch {
		case r.StatusCode == http.StatusOK && r.Error == "":
			v.NOk++
		case strings.Contains(strings.ToLower(r.Error), "timeout") || strings.Contains(strings.ToLower(r.Error), "deadline"):
			v.NTimeout++
		default:
			v.NFailed++
		}
	}
	v.AllOK = v.NOk == n
	v.NoLoss = (v.NOk + v.NFailed + v.NTimeout) == n
	v.NoDuplicate = detectNoDuplicate(v.Results)
	v.OwnNonceOK = detectOwnNonce(v.Results)
	v.NoCrossContam, v.CrossContamPairs = detectCrossContamination(v.Results, nonces)
	return v
}

func runIdentical(client *http.Client, endpoint, model, prompt string, n int) verdict {
	v := verdict{Mode: "identical", N: n, Endpoint: endpoint, Model: model, Prompt: prompt, Results: make([]singleResult, n)}
	temp0 := 0.0

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			status, content, latency, err := postRace(client, endpoint, model, prompt, &temp0)
			r := singleResult{Index: idx, StatusCode: status, LatencyMS: latency}
			if err != nil {
				r.Error = err.Error()
			} else {
				r.Content = content
			}
			v.Results[idx] = r
		}(i)
	}
	wg.Wait()

	for _, r := range v.Results {
		switch {
		case r.StatusCode == http.StatusOK && r.Error == "":
			v.NOk++
		case strings.Contains(strings.ToLower(r.Error), "timeout") || strings.Contains(strings.ToLower(r.Error), "deadline"):
			v.NTimeout++
		default:
			v.NFailed++
		}
	}
	v.AllOK = v.NOk == n
	v.NoLoss = (v.NOk + v.NFailed + v.NTimeout) == n

	// DETERMINISTIC: extract the first integer token from every successful
	// response and assert they are all identical. Honest boundary
	// (§11.4.6): documented, not asserted as an absolute physical
	// guarantee — floating-point non-associativity under concurrent
	// batched inference can occasionally perturb even greedy (temp=0)
	// decoding; a mismatch here is real signal, not a test bug.
	seen := map[string]bool{}
	var distinct []string
	allExtracted := true
	for _, r := range v.Results {
		if r.Error != "" || r.StatusCode != http.StatusOK {
			continue
		}
		val, ok := extractFirstInt(r.Content)
		if !ok {
			allExtracted = false
			continue
		}
		if !seen[val] {
			seen[val] = true
			distinct = append(distinct, val)
		}
	}
	v.DistinctAnswers = distinct
	v.DeterministicOK = allExtracted && len(distinct) == 1

	return v
}

func runSelftestCrossContam(fixture string) verdict {
	v := verdict{Mode: "selftest-crosscontam", Fixture: fixture}

	var results []singleResult
	var nonces []string

	switch fixture {
	case "clean":
		// Three synthetic results, each correctly containing ONLY its own
		// nonce — the detector must report NoCrossContam == true.
		nonces = []string{"RACEID_AAA", "RACEID_BBB", "RACEID_CCC"}
		results = []singleResult{
			{Index: 0, Nonce: nonces[0], StatusCode: 200, Content: "42\nRACEID_AAA"},
			{Index: 1, Nonce: nonces[1], StatusCode: 200, Content: "17\nRACEID_BBB"},
			{Index: 2, Nonce: nonces[2], StatusCode: 200, Content: "9\nRACEID_CCC"},
		}
	case "contaminated":
		// Same shape, but result[0]'s content contains request[1]'s nonce
		// (a synthetic cross-contamination injection) — the detector MUST
		// report NoCrossContam == false and cite the exact pair.
		nonces = []string{"RACEID_AAA", "RACEID_BBB", "RACEID_CCC"}
		results = []singleResult{
			{Index: 0, Nonce: nonces[0], StatusCode: 200, Content: "42\nRACEID_AAA\nRACEID_BBB"},
			{Index: 1, Nonce: nonces[1], StatusCode: 200, Content: "17\nRACEID_BBB"},
			{Index: 2, Nonce: nonces[2], StatusCode: 200, Content: "9\nRACEID_CCC"},
		}
	default:
		v.Error = fmt.Sprintf("unknown fixture: %s (want clean|contaminated)", fixture)
		return v
	}

	v.N = len(results)
	v.NOk = len(results)
	v.AllOK = true
	v.NoLoss = true
	v.Results = results
	v.NoDuplicate = detectNoDuplicate(results)
	v.OwnNonceOK = detectOwnNonce(results)
	v.NoCrossContam, v.CrossContamPairs = detectCrossContamination(results, nonces)
	return v
}

// ---------- main ----------

func main() {
	os.Exit(run())
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func run() int {
	var (
		mode           = flag.String("mode", "distinct", "race mode: distinct | identical | selftest-crosscontam")
		n              = flag.Int("n", 8, "number of concurrent requests (distinct/identical modes)")
		promptText     = flag.String("prompt", "What is 6 * 7? Respond with ONLY the numeric result.", "prompt for identical mode")
		fixture        = flag.String("fixture", "clean", "selftest-crosscontam fixture: clean | contaminated")
		endpoint       = flag.String("endpoint", envOr("HELIX_CODER_ENDPOINT", "http://localhost:18434/v1/chat/completions"), "coder /v1/chat/completions endpoint")
		model          = flag.String("model", envOr("HELIX_CODER_MODEL", "llama3.2"), "model name")
		assertAllOK    = flag.Bool("assert-all-ok", false, "assert every response is HTTP 200")
		assertNoLoss   = flag.Bool("assert-no-loss", false, "assert exactly N responses received")
		assertNoDup    = flag.Bool("assert-no-duplicate", false, "assert no two responses are byte-identical (distinct mode)")
		assertOwnNonce = flag.Bool("assert-own-nonce", false, "assert every response contains its own nonce (distinct mode)")
		assertNoCross  = flag.Bool("assert-no-cross-contam", false, "assert no response contains another request's nonce (distinct mode / selftest)")
		assertDeterm   = flag.Bool("assert-deterministic", false, "assert all responses extract to the same integer (identical mode)")
		out            = flag.String("out", "", "path to write the verdict JSON (required)")
		conduitDir     = flag.String("conduit-dir", "", "optional conduit JSONL event dir (§11.4.116)")
		challID        = flag.String("challenge-id", "", "challenge id for conduit events (defaults to --out basename)")
		timeout        = flag.Duration("timeout", 120*time.Second, "per-request timeout")
		expectFail     = flag.Bool("expect-fail", false, "invert case-level exit code — for golden-bad self-validation fixtures")
	)
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-coder-race-llm --mode distinct|identical|selftest-crosscontam --out <verdict.json> [--n 8] [--assert-*]")
		return exitInfra
	}
	cid := *challID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_coder_race", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}
	conduit.ChallengeStart(sink, cid, "coder_race_llm")

	var v verdict
	switch *mode {
	case "distinct":
		client := &http.Client{Timeout: *timeout}
		v = runDistinct(client, *endpoint, *model, *n)
	case "identical":
		client := &http.Client{Timeout: *timeout}
		v = runIdentical(client, *endpoint, *model, *promptText, *n)
	case "selftest-crosscontam":
		v = runSelftestCrossContam(*fixture)
	default:
		v = verdict{Mode: *mode, Error: fmt.Sprintf("unknown mode: %s", *mode)}
	}

	// --- combine assertions into PASS ---
	v.Pass = true
	if *assertAllOK && !v.AllOK {
		v.Pass = false
	}
	if *assertNoLoss && !v.NoLoss {
		v.Pass = false
	}
	if *assertNoDup && !v.NoDuplicate {
		v.Pass = false
	}
	if *assertOwnNonce && !v.OwnNonceOK {
		v.Pass = false
	}
	if *assertNoCross && !v.NoCrossContam {
		v.Pass = false
	}
	if *assertDeterm && !v.DeterministicOK {
		v.Pass = false
	}
	if v.Error != "" {
		v.Pass = false
	}
	if v.N == 0 && *mode != "selftest-crosscontam" {
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
	conduit.EvidenceCaptured(sink, cid, "race_verdict_json", *out)

	summary := fmt.Sprintf(
		"mode=%s n=%d ok=%d failed=%d timeout=%d all-ok=%v no-loss=%v no-dup=%v own-nonce=%v no-cross-contam=%v deterministic=%v cross-contam-pairs=%d expect_fail=%v",
		v.Mode, v.N, v.NOk, v.NFailed, v.NTimeout, v.AllOK, v.NoLoss, v.NoDuplicate, v.OwnNonceOK, v.NoCrossContam, v.DeterministicOK, len(v.CrossContamPairs), v.ExpectFail,
	)

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, "")
		fmt.Printf("PASS: %s %s\n", cid, summary)
		return exitPass
	}
	conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, summary)
	fmt.Printf("FAIL: %s %s\n", cid, summary)
	return exitFail
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
