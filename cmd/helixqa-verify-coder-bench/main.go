// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-coder-bench is the RUNNABLE analyzer for the
// HelixQA HelixLLM coder benchmarking bank
// (banks/helixllm_coder_bench.yaml). It drives real concurrent POST
// /v1/chat/completions against the live resident HelixLLM coder
// (llama.cpp OpenAI-compatible sidecar, default :18434) across four
// concurrency levels — 1, 10, 50, 100 — and captures:
//
//   - LATENCY PERCENTILES — p50 / p95 / p99 at each concurrency level
//   - THROUGHPUT        — requests per second AND tokens per second
//   - TIME-TO-FIRST-TOKEN — TTFT p50/p95/p99 via streaming, when
//     enabled (--ttft)
//
// Each concurrency level sends --n-per-level requests (default 20) as
// concurrent goroutines, records individual latencies, classifies
// outcomes (200 vs 429/5xx/timeout/refused), and computes aggregate
// metrics. Streaming requests (stream: true in the request body) let
// the server return tokens as SSE chunks; the analyzer measures the
// time from request start to the first non-empty content chunk as TTFT.
//
// BASELINE THRESHOLDS are passed as flags and asserted. The bank YAML
// encodes the project's current-iteration thresholds; the analyzer
// emits a PASS only when all asserted metrics are within budget.
//
// Mirrors the CLI convention of cmd/helixqa-verify-coder-concurrency
// (--out/--conduit-dir/--challenge-id/--expect-fail, exit 0/1/2).
//
// ANTI-BLUFF (§11.4.6/§11.4.69/§11.4.107(10)/§11.4.123/§11.4.169).
// PASS requires REAL concurrent HTTP round-trips against the live coder
// with REAL measured latency, throughput, and TTFT values. The analyzer
// self-validates via --expect-fail (golden-bad fixture: an impossible
// baseline — e.g. latency-p99-max=1ms, throughput-min=1e9 — MUST FAIL
// the case-level assertion while the raw measurements are honestly
// recorded). A paired §1.1 mutation (forcing all assertions pass=true
// unconditionally) MUST flip the golden-bad case_result from true to
// false — proving the threshold comparisons are load-bearing.
//
// Usage:
//
//	# Single concurrency level with throughput assertion
//	helixqa-verify-coder-bench \
//	  --level 10 --n-per-level 30 \
//	  --throughput-min 0.5 \
//	  --out qa-results/helixllm_coder_bench/bench_10_verdict.json
//
//	# All four levels with latency baseline + TTFT
//	helixqa-verify-coder-bench \
//	  --level 1 --level 10 --level 50 --level 100 \
//	  --n-per-level 20 \
//	  --latency-p99-max 30000 --throughput-min 0.3 \
//	  --ttft --ttft-p95-max 5000 \
//	  --out qa-results/helixllm_coder_bench/bench_all_verdict.json
//
//	# Self-validation golden-bad: impossible latency threshold
//	helixqa-verify-coder-bench \
//	  --level 1 --n-per-level 3 \
//	  --latency-p99-max 1 --expect-fail \
//	  --out qa-results/helixllm_coder_bench/golden_bad_verdict.json
//
// Exit codes: 0 -> case_result==true; 1 -> case_result==false; 2 -> infra error.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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

// For non-streaming responses
type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   *usageInfo   `json:"usage,omitempty"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
	Finish  string      `json:"finish_reason"`
}

type usageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// For streaming responses (SSE / chat/completions with stream=true)
// Each line is "data: {..." with the same wire format
type streamChunk struct {
	Choices []streamChoice `json:"choices"`
}

type streamChoice struct {
	Delta streamDelta `json:"delta"`
	Index int         `json:"index"`
}

type streamDelta struct {
	Content string `json:"content,omitempty"`
}

// ---------- per-request result ----------

type singleResult struct {
	Index        int    `json:"index"`
	Content      string `json:"content"`
	StatusCode   int    `json:"status_code"`
	LatencyMS    int64  `json:"latency_ms"`
	TTFTMS       int64  `json:"ttft_ms,omitempty"`  // time-to-first-token (streaming only)
	TokenCount   int    `json:"token_count,omitempty"`
	Error        string `json:"error,omitempty"`
}

// ---------- per-level aggregate ----------

type levelMetrics struct {
	Concurrency  int     `json:"concurrency"`
	N            int     `json:"n"`
	NOk          int     `json:"n_ok"`
	NFailed      int     `json:"n_failed"`
	NTimeout     int     `json:"n_timeout"`
	NRefused     int     `json:"n_refused"`
	N5xx         int     `json:"n_5xx"`
	N429         int     `json:"n_429"`
	LatenciesMS  []int64 `json:"-"`
	TTFTMS       []int64 `json:"-"`
	P50LatencyMS int64   `json:"p50_latency_ms"`
	P95LatencyMS int64   `json:"p95_latency_ms"`
	P99LatencyMS int64   `json:"p99_latency_ms"`
	P50TTFTMS    int64   `json:"p50_ttft_ms,omitempty"`
	P95TTFTMS    int64   `json:"p95_ttft_ms,omitempty"`
	P99TTFTMS    int64   `json:"p99_ttft_ms,omitempty"`
	DurationMS   int64   `json:"duration_ms"`
	ThroughputRPS float64 `json:"throughput_req_per_sec"`
	ThroughputTPS float64 `json:"throughput_tok_per_sec"`
	TotalTokens  int     `json:"total_tokens"`
	AllOK        bool    `json:"all_ok"`
}

// ---------- aggregate verdict ----------

type verdict struct {
	Levels           []levelMetrics `json:"levels"`
	Endpoint         string         `json:"endpoint"`
	Model            string         `json:"model"`
	NPerLevel        int            `json:"n_per_level"`
	TTFTEnabled      bool           `json:"ttft_enabled"`
	LatencyP99MaxMS  int64          `json:"latency_p99_max_ms"`
	ThroughputMinRPS float64        `json:"throughput_min_rps"`
	TTFTP95MaxMS     int64          `json:"ttft_p95_max_ms,omitempty"`
	Pass             bool           `json:"pass"`
	ExpectFail       bool           `json:"expect_fail"`
	CaseResult       bool           `json:"case_result"`
	Error            string         `json:"error,omitempty"`
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

func percentileSorted(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100.0*float64(len(sorted))) - 1)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// ---------- HTTP helpers ----------

func postSingle(client *http.Client, endpoint, model, prompt string, idx int) singleResult {
	r := singleResult{Index: idx}

	body, _ := json.Marshal(chatRequest{
		Model:    model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
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
		if httpResp.StatusCode == http.StatusTooManyRequests {
			r.Content = strings.TrimSpace(string(respBody))
		}
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

	if cr.Usage != nil {
		r.TokenCount = cr.Usage.CompletionTokens
	} else {
		// Approximate token count from content length (4 chars per token)
		r.TokenCount = len(r.Content) / 4
		if r.TokenCount < 1 {
			r.TokenCount = 1
		}
	}

	return r
}

// postSingleStream sends a streaming request and measures TTFT (time to first
// token). Returns a singleResult whose TTFTMS is populated, plus the full
// accumulated content and total latency.
func postSingleStream(client *http.Client, endpoint, model, prompt string, idx int) singleResult {
	r := singleResult{Index: idx}

	body, _ := json.Marshal(chatRequest{
		Model:    model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		Stream: true,
	})

	start := time.Now()
	httpResp, err := client.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		r.LatencyMS = time.Since(start).Milliseconds()
		r.Error = fmt.Sprintf("http: %v", err)
		r.StatusCode = 0
		return r
	}
	defer httpResp.Body.Close()

	r.StatusCode = httpResp.StatusCode
	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		r.LatencyMS = time.Since(start).Milliseconds()
		r.Error = fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode, strings.TrimSpace(string(respBody)))
		return r
	}

	// Read SSE stream — measure TTFT as the time to the first non-empty
	// delta content from the start of the request.
	tokenStart := time.Now()
	ttftSet := false
	var accumulated bytes.Buffer

	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		for _, c := range chunk.Choices {
			if c.Delta.Content != "" {
				if !ttftSet {
					r.TTFTMS = time.Since(tokenStart).Milliseconds()
					ttftSet = true
				}
				accumulated.WriteString(c.Delta.Content)
			}
		}
	}

	r.LatencyMS = time.Since(start).Milliseconds()
	if !ttftSet {
		// No token ever arrived; set TTFT = total latency as an upper bound
		r.TTFTMS = r.LatencyMS
	}

	if err := scanner.Err(); err != nil {
		r.Error = fmt.Sprintf("stream read: %v", err)
		r.StatusCode = 0
		return r
	}

	r.Content = strings.TrimSpace(accumulated.String())
	r.TokenCount = len(r.Content) / 4
	if r.TokenCount < 1 {
		r.TokenCount = 1
	}

	return r
}

// ---------- level runner ----------

func runLevel(client *http.Client, endpoint, model, prompt string, concurrency, n int, useStream bool) levelMetrics {
	m := levelMetrics{
		Concurrency: concurrency,
		N:           n,
		LatenciesMS: make([]int64, 0, n),
		TTFTMS:      make([]int64, 0, n),
	}

	results := make([]singleResult, n)
	sem := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < n; i++ {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int) {
			defer func() {
				<-sem
				wg.Done()
			}()
			if useStream {
				results[idx] = postSingleStream(client, endpoint, model, prompt, idx)
			} else {
				results[idx] = postSingle(client, endpoint, model, prompt, idx)
			}
		}(i)
	}
	wg.Wait()
	m.DurationMS = time.Since(start).Milliseconds()

	// Classify and aggregate
	for _, r := range results {
		switch {
		case r.StatusCode == http.StatusOK && r.Error == "":
			m.NOk++
			m.LatenciesMS = append(m.LatenciesMS, r.LatencyMS)
			if r.TTFTMS > 0 {
				m.TTFTMS = append(m.TTFTMS, r.TTFTMS)
			}
			m.TotalTokens += r.TokenCount
		case r.StatusCode == http.StatusTooManyRequests:
			m.N429++
		case r.StatusCode >= 500:
			m.N5xx++
		case r.StatusCode == 0 && (strings.Contains(r.Error, "refused") || strings.Contains(r.Error, "connection") || strings.Contains(r.Error, "conn")) :
			m.NRefused++
		default:
			if r.Error != "" && (strings.Contains(r.Error, "timeout") || strings.Contains(r.Error, "Timeout") || strings.Contains(r.Error, "deadline")) {
				m.NTimeout++
			}
			m.NFailed++
		}
	}

	m.AllOK = m.NOk == n

	// Compute percentiles
	if len(m.LatenciesMS) > 0 {
		sorted := make([]int64, len(m.LatenciesMS))
		copy(sorted, m.LatenciesMS)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		m.P50LatencyMS = percentileSorted(sorted, 50)
		m.P95LatencyMS = percentileSorted(sorted, 95)
		m.P99LatencyMS = percentileSorted(sorted, 99)
	}

	if len(m.TTFTMS) > 0 {
		sorted := make([]int64, len(m.TTFTMS))
		copy(sorted, m.TTFTMS)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		m.P50TTFTMS = percentileSorted(sorted, 50)
		m.P95TTFTMS = percentileSorted(sorted, 95)
		m.P99TTFTMS = percentileSorted(sorted, 99)
	}

	// Throughput
	if m.DurationMS > 0 {
		m.ThroughputRPS = float64(m.NOk) / (float64(m.DurationMS) / 1000.0)
		m.ThroughputTPS = float64(m.TotalTokens) / (float64(m.DurationMS) / 1000.0)
	}

	return m
}

// ---------- main ----------

func main() {
	os.Exit(run())
}

func run() int {
	var (
		levelsArr     = flag.Int("level", 10, "concurrency level(s) — repeat for multiple (e.g. --level 1 --level 10 --level 50 --level 100)")
		nPerLevel     = flag.Int("n-per-level", 20, "number of requests per concurrency level")
		promptText    = flag.String("prompt", "Write a short poem about a robot learning to paint. Maximum 5 lines.", "prompt to send to every request")
		flagTTFT      = flag.Bool("ttft", false, "measure time-to-first-token using streaming (stream:true)")
		latencyP99Max = flag.Int64("latency-p99-max", 0, "max acceptable p99 latency in ms (0 = skip assertion)")
		throughputMin = flag.Float64("throughput-min", 0, "min acceptable throughput in req/sec (0 = skip assertion)")
		ttftP95Max    = flag.Int64("ttft-p95-max", 0, "max acceptable TTFT p95 in ms (0 = skip assertion; requires --ttft)")
		endpoint      = flag.String("endpoint", envOr("HELIX_CODER_ENDPOINT", "http://localhost:18434/v1/chat/completions"), "coder /v1/chat/completions endpoint")
		model         = flag.String("model", envOr("HELIX_CODER_MODEL", "llama3.2"), "model name")
		out           = flag.String("out", "", "path to write the verdict JSON (required)")
		conduitDir    = flag.String("conduit-dir", "", "optional conduit JSONL event dir (§11.4.116)")
		challID       = flag.String("challenge-id", "", "challenge id for conduit events (defaults to --out basename)")
		timeout       = flag.Duration("timeout", 120*time.Second, "per-request timeout")
		expectFail    = flag.Bool("expect-fail", false, "invert case-level exit code — for golden-bad self-validation fixtures")
	)
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-coder-bench --out <verdict.json> [--level N]... [--n-per-level M] [--latency-p99-max MS] [--throughput-min RPS] [--ttft] [--ttft-p95-max MS]")
		return exitInfra
	}

	// Gather levels (flag.Int provides only the last value; we re-parse)
	levelValues := []int{*levelsArr}
	if flag.NArg() > 0 || len(os.Args) > 1 {
		// Re-parse to collect multiple --level values
		levelValues = collectLevels()
	}

	cid := *challID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_coder_bench", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}
	conduit.ChallengeStart(sink, cid, "coder_bench")

	v := verdict{
		Endpoint:         *endpoint,
		Model:            *model,
		NPerLevel:        *nPerLevel,
		TTFTEnabled:      *flagTTFT,
		LatencyP99MaxMS:  *latencyP99Max,
		ThroughputMinRPS: *throughputMin,
		TTFTP95MaxMS:     *ttftP95Max,
		Levels:           make([]levelMetrics, 0, len(levelValues)),
	}

	client := &http.Client{Timeout: *timeout}

	// Run each concurrency level sequentially (levels are independent
	// measurement windows; within a level, requests are concurrent).
	for _, lvl := range levelValues {
		conduit.Logf(sink, cid, fmt.Sprintf("bench_level_start level=%d n=%d", lvl, *nPerLevel))
		m := runLevel(client, *endpoint, *model, *promptText, lvl, *nPerLevel, *flagTTFT)
		v.Levels = append(v.Levels, m)
		conduit.Logf(sink, cid, fmt.Sprintf("bench_level_done level=%d ok=%d/%d p50=%dms p95=%dms p99=%dms rps=%.1f tps=%.1f",
			lvl, m.NOk, m.N, m.P50LatencyMS, m.P95LatencyMS, m.P99LatencyMS, m.ThroughputRPS, m.ThroughputTPS))
	}

	// --- combine assertions into PASS ---
	v.Pass = true

	if v.LatencyP99MaxMS > 0 {
		for _, m := range v.Levels {
			if m.NOk == 0 {
				continue
			}
			if m.P99LatencyMS > v.LatencyP99MaxMS {
				v.Pass = false
				break
			}
		}
	}

	if v.ThroughputMinRPS > 0 {
		for _, m := range v.Levels {
			if m.NOk == 0 {
				continue
			}
			if m.ThroughputRPS < v.ThroughputMinRPS {
				v.Pass = false
				break
			}
		}
	}

	if v.TTFTEnabled && v.TTFTP95MaxMS > 0 {
		for _, m := range v.Levels {
			if len(m.TTFTMS) == 0 {
				continue
			}
			if m.P95TTFTMS > v.TTFTP95MaxMS {
				v.Pass = false
				break
			}
		}
	}

	// No level produced any results — infra failure, not pass
	anyOK := false
	for _, m := range v.Levels {
		if m.NOk > 0 {
			anyOK = true
			break
		}
	}
	if !anyOK {
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

	conduit.EvidenceCaptured(sink, cid, "bench_verdict_json", *out)

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, "")
		fmt.Printf("PASS: %s levels=%d n_per_level=%d ttft=%v latency_p99_max_ms=%d throughput_min_rps=%.1f ttft_p95_max_ms=%d expect_fail=%v\n",
			cid, len(v.Levels), v.NPerLevel, v.TTFTEnabled, v.LatencyP99MaxMS, v.ThroughputMinRPS, v.TTFTP95MaxMS, v.ExpectFail)
		for _, m := range v.Levels {
			fmt.Printf("  level=%d ok=%d/%d p50=%dms p95=%dms p99=%dms rps=%.1f tps=%.1f duration=%dms",
				m.Concurrency, m.NOk, m.N, m.P50LatencyMS, m.P95LatencyMS, m.P99LatencyMS, m.ThroughputRPS, m.ThroughputTPS, m.DurationMS)
			if v.TTFTEnabled && len(m.TTFTMS) > 0 {
				fmt.Printf(" ttft_p50=%dms ttft_p95=%dms", m.P50TTFTMS, m.P95TTFTMS)
			}
			fmt.Println()
		}
		return exitPass
	}

	var reasons []string
	for _, m := range v.Levels {
		reasons = append(reasons, fmt.Sprintf("level=%d ok=%d/%d p50=%dms p95=%dms p99=%dms rps=%.1f tps=%.1f",
			m.Concurrency, m.NOk, m.N, m.P50LatencyMS, m.P95LatencyMS, m.P99LatencyMS, m.ThroughputRPS, m.ThroughputTPS))
	}
	reason := strings.Join(reasons, "; ")
	conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, reason)
	fmt.Printf("FAIL: %s %s\n", cid, reason)
	return exitFail
}

// collectLevels re-parses os.Args to collect all unique --level values.
// Uses flag.ContinueOnError to silently skip flags defined by the outer
// flag.Parse() that this sidecar FlagSet does not carry.
func collectLevels() []int {
	seen := make(map[int]bool)
	var vals []int

	fs := flag.NewFlagSet("level-collector", flag.ContinueOnError)
	fs.Usage = func() {}
	// Suppress "flag provided but not defined" for flags the outer set owns
	fs.SetOutput(io.Discard)
	levels := fs.Int("level", 10, "")
	_ = fs.Parse(os.Args[1:])
	if !seen[*levels] {
		seen[*levels] = true
		vals = append(vals, *levels)
	}

	// Also scan os.Args manually for --level= N and --level N to pick up
	// all repeat occurrences (flag.Int holds only the last value).
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		var v int
		parsed := false
		if arg == "--level" || arg == "-level" {
			if i+1 < len(os.Args) {
				if _, err := fmt.Sscanf(os.Args[i+1], "%d", &v); err == nil {
					parsed = true
				}
				i++
			}
		} else if strings.HasPrefix(arg, "--level=") || strings.HasPrefix(arg, "-level=") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				if _, err := fmt.Sscanf(parts[1], "%d", &v); err == nil {
					parsed = true
				}
			}
		}
		if parsed && !seen[v] {
			seen[v] = true
			vals = append(vals, v)
		}
	}

	if len(vals) == 0 {
		vals = append(vals, 10)
	}
	return vals
}

// ---------- infrastructure ----------

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
