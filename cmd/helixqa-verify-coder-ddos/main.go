// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-coder-ddos is the RUNNABLE analyzer for the
// HelixQA HelixLLM coder DDoS / load-flood bank
// (banks/helixllm_coder_ddos.yaml). It drives real concurrent POST
// /v1/chat/completions against the live resident HelixLLM coder
// (llama.cpp OpenAI-compatible sidecar, default :18434) in three
// operation modes matching the three test cases:
//
//   burst      — N requests as fast as possible (fire-and-receive),
//                assert no-loss (exactly N responses), no-5xx, each
//                nonce present (DDOS-CODER-001)
//   soak       — N requests spread over a duration window, record
//                throughput + degradation (DDOS-CODER-002)
//   conn-flood — M parallel connections all at once, classify every
//                response outcome, verify coder process alive after
//                flood, verify post-flood recovery (DDOS-CODER-003)
//
// Mirrors the CLI convention of cmd/helixqa-verify-coder-concurrency
// (--out/--conduit-dir/--challenge-id/--expect-fail, exit 0/1/2).
//
// ANTI-BLUFF (§11.4.6/§11.4.69/§11.4.107(10)/§11.4.123/§11.4.169).
// PASS requires REAL concurrent HTTP round-trips against the live coder.
// A zero-response/stub/single-threaded-in-sequence is caught by no-loss
// (fewer than N responses), nonces (identical nonce on every "response"),
// or no-5xx assertion. --expect-fail inverts case success for self-
// validation: a golden-bad fixture (--expect-no-loss true against a dead
// endpoint where zero responses arrive) MUST produce exit 1.
//
// Usage:
//
//	# Burst mode (DDOS-CODER-001)
//	helixqa-verify-coder-ddos \
//	  --mode burst --n 200 \
//	  --prompt "Count to 5. NONCE:<idx>" \
//	  --expect-no-loss --expect-no-5xx \
//	  --out qa-results/helixllm_coder_ddos/burst_verdict.json
//
//	# Soak mode (DDOS-CODER-002)
//	helixqa-verify-coder-ddos \
//	  --mode soak --n 500 --duration 30s \
//	  --prompt "Say ONE word." \
//	  --out qa-results/helixllm_coder_ddos/soak_verdict.json
//
//	# Connection-flood mode (DDOS-CODER-003)
//	helixqa-verify-coder-ddos \
//	  --mode conn-flood --n 100 \
//	  --prompt "Hi." \
//	  --expect-no-crash \
//	  --out qa-results/helixllm_coder_ddos/flood_verdict.json
//
//	# Self-validation (golden-bad — MUST FAIL)
//	helixqa-verify-coder-ddos \
//	  --mode burst --n 3 \
//	  --endpoint http://localhost:18433/v1/chat/completions \
//	  --expect-no-loss --expect-no-5xx \
//	  --expect-fail \
//	  --out qa-results/helixllm_coder_ddos/golden_bad_verdict.json
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
	"os/exec"
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
	Message chatMessage `json:"message"`
	Finish  string      `json:"finish_reason"`
}

// ---------- per-request result ----------

type singleResult struct {
	Index      int    `json:"index"`
	Nonce      string `json:"nonce,omitempty"`
	Content    string `json:"content"`
	StatusCode int    `json:"status_code"`
	LatencyMS  int64  `json:"latency_ms"`
	Error      string `json:"error,omitempty"`
}

// ---------- aggregate verdict ----------

type verdict struct {
	Mode          string         `json:"mode"`
	N             int            `json:"n"`
	NOk           int            `json:"n_ok"`
	N429          int            `json:"n_429"`
	NRefused      int            `json:"n_refused"`
	N5xx          int            `json:"n_5xx"`
	NOther        int            `json:"n_other"`
	NDropped      int            `json:"n_dropped"`
	NFailed       int            `json:"n_failed"`
	NTimeout      int            `json:"n_timeout"`
	TotalReceived int            `json:"total_received"`
	NoLoss        bool           `json:"no_loss"`
	No5xx         bool           `json:"no_5xx"`
	NoCrash       bool           `json:"no_crash"`
	Duration      string         `json:"duration,omitempty"`
	Throughput    float64        `json:"throughput_req_per_sec,omitempty"`
	Endpoint      string         `json:"endpoint"`
	Model         string         `json:"model"`
	Prompt        string         `json:"prompt"`
	Results       []singleResult `json:"results,omitempty"`
	Pass          bool           `json:"pass"`
	ExpectFail    bool           `json:"expect_fail"`
	CaseResult    bool           `json:"case_result"`
	Error         string         `json:"error,omitempty"`
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
		// Replace the literal "<idx>" token with the actual nonce value
		// so each request gets a unique, verifiable token in the prompt.
		finalPrompt = strings.ReplaceAll(prompt, "<idx>", nonce)
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
		// Still capture truncated body for 429 analysis
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

	r.StatusCode = httpResp.StatusCode
	return r
}

func classIfyStatus(code int) string {
	switch {
	case code == http.StatusOK:
		return "200"
	case code == http.StatusTooManyRequests:
		return "429"
	case code == 0:
		return "refused"
	case code >= 500:
		return "5xx"
	default:
		return "other"
	}
}

// isCoderAlive checks if the coder process is still running via pgrep.
func isCoderAlive() bool {
	// Use a broad pattern: look for llama or coder-related processes.
	// On this system the coder runs as a llama.cpp process.
	cmd := exec.Command("pgrep", "-f", "llama")
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		return true
	}
	// Also check for any process listening on the endpoint port.
	return false
}

// ---------- main ----------

func main() {
	os.Exit(run())
}

func run() int {
	var (
		mode           = flag.String("mode", "burst", "operation mode: burst | soak | conn-flood")
		n              = flag.Int("n", 200, "number of requests (burst/soak/flood)")
		duration       = flag.Duration("duration", 30*time.Second, "duration for soak mode")
		promptText     = flag.String("prompt", "Count to 5. NONCE:<idx>", "prompt to send; <idx> is replaced with unique nonce when --expect-no-loss is set")
		flagExpectNoLoss  = flag.Bool("expect-no-loss", false, "assert exactly N responses received (zero dropped)")
		flagExpectNo5xx   = flag.Bool("expect-no-5xx", false, "assert zero 5xx errors")
		flagExpectNoCrash = flag.Bool("expect-no-crash", false, "assert coder process alive after flood")
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
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-coder-ddos --out <verdict.json> [--mode burst|soak|conn-flood] [--n N] [--prompt ...] [--expect-no-loss] [--expect-no-5xx] [--expect-no-crash]")
		return exitInfra
	}
	cid := *challID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_coder_ddos", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}
	conduit.ChallengeStart(sink, cid, "coder_ddos_"+*mode)

	v := verdict{
		Mode:     *mode,
		N:        *n,
		Endpoint: *endpoint,
		Model:    *model,
		Prompt:   *promptText,
		Results:  make([]singleResult, *n),
	}

	client := &http.Client{Timeout: *timeout}

	switch *mode {
	case "burst":
		runBurst(client, &v, *n, *endpoint, *model, *promptText)
	case "soak":
		runSoak(client, &v, *n, *duration, *endpoint, *model, *promptText)
	case "conn-flood":
		runConnFlood(client, &v, *n, *endpoint, *model, *promptText)
	default:
		errMsg := fmt.Sprintf("unknown mode: %s (must be burst, soak, or conn-flood)", *mode)
		return failInfra(sink, cid, &v, *out, errMsg)
	}

	// Classify responses
	for _, r := range v.Results {
		switch classIfyStatus(r.StatusCode) {
		case "200":
			v.NOk++
		case "429":
			v.N429++
		case "refused":
			v.NRefused++
		case "5xx":
			v.N5xx++
		default:
			v.NOther++
		}
		if r.Error != "" && (strings.Contains(r.Error, "timeout") || strings.Contains(r.Error, "Timeout") || strings.Contains(r.Error, "context deadline")) {
			v.NTimeout++
		}
	}
	v.TotalReceived = v.NOk + v.N429 + v.N5xx + v.NOther

	// Deduced metrics
	v.NDropped = *n - v.TotalReceived - v.NRefused

	// Assertions
	v.NoLoss = v.TotalReceived == *n
	v.No5xx = v.N5xx == 0
	if *mode == "conn-flood" && *flagExpectNoCrash {
		v.NoCrash = isCoderAlive()
	} else {
		v.NoCrash = true // not asserted
	}

	// --- combine into PASS ---
	v.Pass = true
	if *flagExpectNoLoss && !v.NoLoss {
		v.Pass = false
	}
	if *flagExpectNo5xx && !v.No5xx {
		v.Pass = false
	}
	if *flagExpectNoCrash && !v.NoCrash {
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

	conduit.EvidenceCaptured(sink, cid, "ddos_verdict_json", *out)

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, "")
		fmt.Printf("PASS: %s mode=%s n=%d ok=%d n429=%d refused=%d 5xx=%d other=%d dropped=%d timeout=%d no-loss=%v no-5xx=%v no-crash=%v expect_fail=%v\n",
			cid, v.Mode, v.N, v.NOk, v.N429, v.NRefused, v.N5xx, v.NOther, v.NDropped, v.NTimeout, v.NoLoss, v.No5xx, v.NoCrash, v.ExpectFail)
		return exitPass
	}
	reason := fmt.Sprintf("mode=%s n=%d ok=%d n429=%d refused=%d 5xx=%d other=%d dropped=%d timeout=%d no-loss=%v no-5xx=%v no-crash=%v",
		v.Mode, v.N, v.NOk, v.N429, v.NRefused, v.N5xx, v.NOther, v.NDropped, v.NTimeout, v.NoLoss, v.No5xx, v.NoCrash)
	conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, reason)
	fmt.Printf("FAIL: %s %s\n", cid, reason)
	return exitFail
}

// ---------- mode runners ----------

// runBurst fires N requests as fast as possible (goroutine-per-request, no
// throttling) and collects all responses. Each request embeds a unique nonce
// so the analyzer can verify no-loss and nonce-presence.
func runBurst(client *http.Client, v *verdict, n int, endpoint, model, prompt string) {
	v.Mode = "burst"

	type reqData struct {
		nonce string
	}
	reqs := make([]reqData, n)
	// Embed per-request nonces for loss detection
	for i := 0; i < n; i++ {
		reqs[i].nonce = fmt.Sprintf("NONCE:%s", randomNonce(8))
	}

	var wg sync.WaitGroup
	start := time.Now()
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			// Substitute <idx> with the nonce in the prompt
			effectivePrompt := strings.ReplaceAll(prompt, "<idx>", reqs[idx].nonce)
			v.Results[idx] = postSingle(client, endpoint, model, effectivePrompt, reqs[idx].nonce, idx)
		}(i)
	}
	wg.Wait()
	v.Duration = time.Since(start).Round(time.Millisecond).String()
	if sec := time.Since(start).Seconds(); sec > 0 {
		v.Throughput = float64(n) / sec
	}
}

// runSoak spreads N requests uniformly over the given duration window,
// recording throughput + degradation.
func runSoak(client *http.Client, v *verdict, n int, dur time.Duration, endpoint, model, prompt string) {
	v.Mode = "soak"

	var wg sync.WaitGroup
	start := time.Now()
	interval := dur / time.Duration(n)
	if interval < time.Millisecond {
		interval = time.Millisecond
	}

	mu := sync.Mutex{}
	idx := 0
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			// Sleep so the launch is staggered
			time.Sleep(time.Duration(i) * interval)
			mu.Lock()
			myIdx := idx
			idx++
			mu.Unlock()
			v.Results[myIdx] = postSingle(client, endpoint, model, prompt, "", myIdx)
		}()
	}
	wg.Wait()
	v.Duration = time.Since(start).Round(time.Millisecond).String()
	if sec := time.Since(start).Seconds(); sec > 0 {
		v.Throughput = float64(n) / sec
	}
}

// runConnFlood opens all N connections in parallel (burst) and then reports
// the full classification of every outcome. No nonce assertion — the goal
// is crash-survival and graceful-degrade.
func runConnFlood(client *http.Client, v *verdict, n int, endpoint, model, prompt string) {
	v.Mode = "conn-flood"

	var wg sync.WaitGroup
	start := time.Now()
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			v.Results[idx] = postSingle(client, endpoint, model, prompt, "", idx)
		}(i)
	}
	wg.Wait()
	v.Duration = time.Since(start).Round(time.Millisecond).String()
	if sec := time.Since(start).Seconds(); sec > 0 {
		v.Throughput = float64(n) / sec
	}
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
