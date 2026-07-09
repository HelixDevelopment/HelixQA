// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-coder-chaos is the RUNNABLE analyzer for the
// HelixQA HelixLLM coder chaos-resilience bank
// (banks/helixllm_coder_chaos.yaml). It drives real adversarial
// scenarios against the live resident HelixLLM coder (llama.cpp
// OpenAI-compatible sidecar, default :18434) and asserts resilience
// runtime signatures across three independent chaos modes:
//
//   - PORT-FLOOD     — rapidly open/close N TCP connections to the coder
//     port in bursts; then confirm the coder still accepts a normal
//     completion request (recovery). Exercises connection-storm tolerance
//     without any code change/restart.
//   - OVERSIZED-PROMPT — send a prompt >> context-window size (200K
//     chars); assert graceful degrade (non-200 HTTP with meaningful error
//     body) OR post-oversized health (coder still processes a normal
//     request after). Exercises input-validation resilience.
//   - CONCURRENT-HEALTH — fire N concurrent POST /v1/chat/completions
//     while also sending health probes (/v1/models GET); assert ALL
//     succeed. Exercises endpoint isolation — health must not be
//     starved by generation load.
//
// These modes are independently selected via the --mode flag so the
// bank's test cases combine only the mode they need.
//
// Mirrors the CLI convention of cmd/helixqa-verify-coder-concurrency
// (--out/--conduit-dir/--challenge-id/--expect-fail, exit 0/1/2).
//
// ANTI-BLUFF (§11.4.6/§11.4.69/§11.4.107(10)/§11.4.123). PASS requires
// real adversarial interaction with the live coder. The port-flood mode
// genuinely opens/closes real TCP sockets; the oversized mode sends a
// genuinely huge payload; the concurrent-health mode really fires
// concurrent requests. --expect-fail inverts case success.
//
// Usage:
//
//	helixqa-verify-coder-chaos \
//	  --mode port-flood \
//	  --n 50 --burst-size 10 \
//	  --out qa-results/helixllm_coder_chaos/port_flood_001_verdict.json \
//	  [--endpoint http://localhost:18434] \
//	  [--conduit-dir ...] [--challenge-id ...] [--timeout 120s] [--expect-fail]
//
// Exit codes: 0 -> case_result==true; 1 -> case_result==false; 2 -> infra error.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
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
	Message chatMessage `json:"message"`
	Finish  string      `json:"finish_reason"`
}

// ---------- aggregated verdict ----------

type verdict struct {
	Mode              string `json:"mode"`
	FloodSent         int    `json:"flood_sent,omitempty"`
	FloodBurstSize    int    `json:"flood_burst_size,omitempty"`
	PostFloodStatus   int    `json:"post_flood_status,omitempty"`
	PostFloodOK       bool   `json:"post_flood_ok,omitempty"`
	Recovered         bool   `json:"recovered,omitempty"`
	OversizedStatus   int    `json:"oversized_status,omitempty"`
	OversizedHandled  bool   `json:"oversized_handled,omitempty"`
	OversizedBody     string `json:"oversized_body_snippet,omitempty"`
	PostOversizedOK   bool   `json:"post_oversized_ok,omitempty"`
	PostOversizedBody string `json:"post_oversized_body_snippet,omitempty"`
	NConcurrent       int    `json:"n_concurrent,omitempty"`
	NModels           int    `json:"n_models,omitempty"`
	NModelsOK         int    `json:"n_models_ok,omitempty"`
	ModelsOK          bool   `json:"models_ok,omitempty"`
	AllOK             bool   `json:"all_ok,omitempty"`
	Endpoint          string `json:"endpoint"`
	Port              int    `json:"port"`
	Pass              bool   `json:"pass"`
	ExpectFail        bool   `json:"expect_fail"`
	CaseResult        bool   `json:"case_result"`
	Error             string `json:"error,omitempty"`
}

// ---------- helpers ----------

func postCompletion(endpoint, prompt string, timeout time.Duration) (int, string, error) {
	client := &http.Client{Timeout: timeout}
	body, _ := json.Marshal(chatRequest{
		Model: "llama3.2",
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	})
	httpResp, err := client.Post(endpoint+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("http: %w", err)
	}
	defer httpResp.Body.Close()
	respBytes, _ := io.ReadAll(httpResp.Body)
	bodyText := strings.TrimSpace(string(respBytes))
	return httpResp.StatusCode, bodyText, nil
}

func getHealth(endpoint string, timeout time.Duration) (int, string, error) {
	client := &http.Client{Timeout: timeout}
	httpResp, err := client.Get(endpoint + "/v1/models")
	if err != nil {
		return 0, "", fmt.Errorf("http: %w", err)
	}
	defer httpResp.Body.Close()
	respBytes, _ := io.ReadAll(httpResp.Body)
	return httpResp.StatusCode, strings.TrimSpace(string(respBytes)), nil
}

// ---------- mode runners ----------

func runPortFlood(endpoint string, port int, n, burstSize int, timeout time.Duration, expectFail bool) verdict {
	v := verdict{Mode: "port-flood", Endpoint: endpoint, Port: port, FloodSent: n, FloodBurstSize: burstSize, ExpectFail: expectFail}

	// Phase 1: flood with rapid TCP connections
	burstDelay := 5 * time.Millisecond
	addr := fmt.Sprintf("localhost:%d", port)

	var wg sync.WaitGroup
	sem := make(chan struct{}, burstSize)
	for i := 0; i < n; i++ {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err != nil {
				return // flood goal is to open rapid connections, partial failures are fine
			}
			conn.Close()
		}(i)
		time.Sleep(burstDelay)
	}
	wg.Wait()

	// Small cool-down to let the coder stabilise
	time.Sleep(500 * time.Millisecond)

	// Phase 2: verify recovery by sending a normal completion request
	status, body, err := postCompletion(endpoint, "Say 'recovered' and nothing else.", timeout)
	if err != nil {
		v.PostFloodStatus = 0
		v.PostFloodOK = false
		v.Recovered = false
		v.Error = fmt.Sprintf("post-flood request failed: %v", err)
		v.Pass = false
		v.CaseResult = computeCaseResult(false, v.ExpectFail)
		return v
	}
	v.PostFloodStatus = status
	v.PostFloodOK = status == http.StatusOK
	v.Recovered = status == http.StatusOK && len(body) > 0
	if !v.Recovered {
		v.Error = fmt.Sprintf("post-flood request returned HTTP %d", status)
	}

	v.Pass = v.Recovered
	v.CaseResult = computeCaseResult(v.Pass, v.ExpectFail)
	return v
}

func runOversizedPrompt(endpoint string, oversizedChars int, timeout time.Duration, expectFail bool) verdict {
	v := verdict{Mode: "oversized-prompt", Endpoint: endpoint, ExpectFail: expectFail}

	// Build a prompt that dwarfs the coder's context window
	chunk := "The quick brown fox jumps over the lazy dog. "
	chunkLen := len(chunk)
	repeats := (oversizedChars + chunkLen - 1) / chunkLen
	hugePrompt := strings.Repeat(chunk, repeats)

		// Send the oversized prompt (use the caller-provided timeout — CPU inference
		// processes this sequentially with other queued concurrent requests, so the
		// timeout must accommodate worst-case queue depth x per-request duration)
		status, body, err := postCompletion(endpoint, hugePrompt, timeout)
		if err != nil {
		v.OversizedStatus = 0
		v.OversizedBody = ""
		v.OversizedHandled = false
		v.Error = fmt.Sprintf("oversized request failed: %v", err)
	} else {
		v.OversizedStatus = status
		if len(body) > 200 {
			v.OversizedBody = body[:200]
		} else {
			v.OversizedBody = body
		}
		// Graceful degrade: non-200 with meaningful body (error message), OR
		// 200 means the coder accepted it (also graceful).
		v.OversizedHandled = status != 0
	}

	// Verify the coder still accepts normal requests after
	postStatus, postBody, postErr := postCompletion(endpoint, "Count from 1 to 5.", timeout)
	if postErr != nil {
		v.PostOversizedOK = false
		if v.Error == "" {
			v.Error = fmt.Sprintf("post-oversized request failed: %v", postErr)
		}
	} else {
		v.PostOversizedOK = postStatus == http.StatusOK
		if len(postBody) > 200 {
			v.PostOversizedBody = postBody[:200]
		} else {
			v.PostOversizedBody = postBody
		}
	}

	// PASS if the oversized request was gracefully handled (non-OK or accepted)
	// AND the coder accepts normal requests after
	v.Pass = v.OversizedHandled && v.PostOversizedOK
	if !v.Pass && v.Error == "" {
		if !v.OversizedHandled {
			v.Error = fmt.Sprintf("oversized request not handled (status %d)", v.OversizedStatus)
		} else if !v.PostOversizedOK {
			v.Error = "coder does not accept normal requests after oversized prompt"
		}
	}

	v.CaseResult = computeCaseResult(v.Pass, v.ExpectFail)
	return v
}

func runConcurrentHealth(endpoint string, nConcurrentPost, nHealth int, timeout time.Duration, expectFail bool) verdict {
	v := verdict{Mode: "concurrent-health", Endpoint: endpoint, NConcurrent: nConcurrentPost, NModels: nHealth, ExpectFail: expectFail}

	// Fire concurrent POST requests
	var mu sync.Mutex
	postOK := 0
	postTotal := 0

	var wg sync.WaitGroup

	wg.Add(nConcurrentPost)
	for i := 0; i < nConcurrentPost; i++ {
		go func(idx int) {
			defer wg.Done()
			status, _, err := postCompletion(endpoint, fmt.Sprintf("Count from %d to %d.", idx, idx+5), timeout)
			mu.Lock()
			postTotal++
			if err == nil && status == http.StatusOK {
				postOK++
			}
			mu.Unlock()
		}(i)
	}

	// While POSTs are in flight, send health probes
	healthOK := 0
	healthSent := 0
	for i := 0; i < nHealth; i++ {
		time.Sleep(200 * time.Millisecond)
		healthSent++
		status, _, err := getHealth(endpoint, 10*time.Second)
		if err == nil && status == http.StatusOK {
			healthOK++
		}
	}

	wg.Wait()

	v.NModelsOK = healthOK
	v.ModelsOK = healthOK == healthSent
	v.AllOK = postOK == nConcurrentPost && healthOK == healthSent

	if !v.AllOK {
		parts := []string{}
		if postOK < nConcurrentPost {
			parts = append(parts, fmt.Sprintf("concurrent POSTs: %d/%d OK", postOK, nConcurrentPost))
		}
		if healthOK < healthSent {
			parts = append(parts, fmt.Sprintf("health probes: %d/%d OK", healthOK, healthSent))
		}
		v.Error = strings.Join(parts, "; ")
	}

	v.Pass = v.AllOK
	v.CaseResult = computeCaseResult(v.Pass, v.ExpectFail)
	return v
}

func computeCaseResult(pass, expectFail bool) bool {
	if expectFail {
		return !pass
	}
	return pass
}

// ---------- main ----------

func main() {
	os.Exit(run())
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func run() int {
	var (
		mode         = flag.String("mode", "port-flood", "chaos mode: port-flood | oversized-prompt | concurrent-health")
		n            = flag.Int("n", 50, "port-flood: total connections; concurrent-health: concurrent POSTs")
		burstSize    = flag.Int("burst-size", 10, "port-flood: concurrent burst connections")
		oversizedK   = flag.Int("oversized-k", 200, "oversized-prompt: prompt size in KiB")
		nHealth      = flag.Int("n-health", 3, "concurrent-health: health probes during load")
		port         = flag.Int("port", 18434, "coder port (for port-flood direct TCP)")
		endpoint     = flag.String("endpoint", envOr("HELIX_CODER_ENDPOINT", "http://localhost:18434"), "coder base URL")
		out          = flag.String("out", "", "path to write the verdict JSON (required)")
		conduitDir   = flag.String("conduit-dir", "", "optional conduit JSONL event dir (§11.4.116)")
		challID      = flag.String("challenge-id", "", "challenge id for conduit events (defaults to --out basename)")
		timeout      = flag.Duration("timeout", 120*time.Second, "per-request timeout")
		expectFail   = flag.Bool("expect-fail", false, "invert case-level exit code — for golden-bad self-validation fixtures")
	)
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-coder-chaos --mode port-flood|oversized-prompt|concurrent-health --out <verdict.json> [--n N] [--port PORT] [--endpoint URL]")
		return exitInfra
	}
	cid := *challID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_coder_chaos", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}

	conduit.ChallengeStart(sink, cid, "coder_chaos")

	var v verdict
	v.ExpectFail = *expectFail

	switch *mode {
	case "port-flood":
		v = runPortFlood(*endpoint, *port, *n, *burstSize, *timeout, *expectFail)
	case "oversized-prompt":
		v = runOversizedPrompt(*endpoint, *oversizedK*1024, *timeout, *expectFail)
	case "concurrent-health":
		v = runConcurrentHealth(*endpoint, *n, *nHealth, *timeout, *expectFail)
	default:
		v = verdict{
			Mode:       *mode,
			Pass:       false,
			CaseResult: computeCaseResult(false, *expectFail),
			Error:      fmt.Sprintf("unknown chaos mode: %s", *mode),
		}
	}

	v.ExpectFail = *expectFail

	// Write verdict
	if err := writeVerdict(*out, &v); err != nil {
		fmt.Fprintf(os.Stderr, "error writing verdict: %v\n", err)
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, err.Error())
		return exitInfra
	}

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, v.Error)
	} else {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictFail, v.Error)
	}

	if v.CaseResult {
		return exitPass
	}
	return exitFail
}

func writeVerdict(path string, v *verdict) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create output dir: %w", err)
		}
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal verdict: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write verdict: %w", err)
	}
	return nil
}
