// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Command helixqa-verify-coder-memory is the RUNNABLE analyzer for the
// HelixQA HelixLLM coder memory-leak soak bank
// (banks/helixllm_coder_memory.yaml). It drives 200 sequential real
// POST /v1/chat/completions against the live resident HelixLLM coder
// (llama.cpp OpenAI-compatible sidecar, default :18434) and captures
// the coder process's RSS at regular intervals via `ps -o rss= -p <PID>`,
// then detects memory-leak patterns from the RSS curve:
//
//   - MONOTONIC-NO-LEAK  — final RSS <= initial RSS + leak-threshold
//     (default +15%). A sustained upward trend beyond this = leak.
//   - GC-POINT-STABILITY — RSS in the second half does not grow beyond
//     the midpoint value + threshold (proves GC / slab compaction is
//     keeping memory in check).
//   - STEADY-STATE       — the last N samples (default 5) are within
//     a narrow band of each other (proves RSS plateaus).
//
// These three dimensions are independently asserted via the corresponding
// flags so the bank's test cases combine only the subset they need.
//
// Mirrors the CLI convention of cmd/helixqa-verify-coder-concurrency
// (--out/--conduit-dir/--challenge-id/--expect-fail, exit 0/1/2).
//
// ANTI-BLUFF (§11.4.6/§11.4.69/§11.4.107(10)/§11.4.123). PASS requires
// (1) real sequential HTTP round-trips against the live coder, (2) real
// ps RSS readings of the live coder PID, (3) the three dimensional
// assertions actually tested against real sampled data.
// --expect-fail inverts case success.
//
// Usage:
//
//	helixqa-verify-coder-memory \
//	  --n 200 \
//	  --prompt "Count from 1 to 5." \
//	  --interval 10 \
//	  --monotonic --gc-stability --steady \
//	  --out qa-results/helixllm_coder_memory/soak_verdict.json \
//	  [--port 18434] \
//	  [--leak-pct 15] \
//	  [--conduit-dir qa-results/helixllm_coder_memory/conduit] \
//	  [--challenge-id MEMORY-MONO-001] [--timeout 120s] [--expect-fail]
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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

// ---------- RSS sample ----------

type rssSample struct {
	Index   int    `json:"index"`
	RSSKb   int64  `json:"rss_kb"`
	DeltaKb int64  `json:"delta_kb"`
	DeltaPct float64 `json:"delta_pct"`
}

// ---------- aggregate verdict ----------

type verdict struct {
	Mode               string      `json:"mode"`
	N                  int         `json:"n"`
	Interval           int         `json:"interval"`
	Endpoint           string      `json:"endpoint"`
	Model              string      `json:"model"`
	Prompt             string      `json:"prompt"`
	CoderPID           int         `json:"coder_pid"`
	InitialRSSKb       int64       `json:"initial_rss_kb"`
	FinalRSSKb         int64       `json:"final_rss_kb"`
	RSSPeakKb          int64       `json:"rss_peak_kb"`
	LeakPct            float64     `json:"leak_pct"`
	RSSSamples         []rssSample `json:"rss_samples"`
	MonotonicNoLeak    bool        `json:"monotonic_no_leak"`
	GCStability        bool        `json:"gc_stability"`
	SteadyState        bool        `json:"steady_state"`
	Pass               bool        `json:"pass"`
	ExpectFail         bool        `json:"expect_fail"`
	CaseResult         bool        `json:"case_result"`
	Error              string      `json:"error,omitempty"`
}

// ---------- helpers ----------

// findCoderPID finds the PID of the process listening on the given port.
// Uses `ss -tlnp`, `lsof -i`, and `fuser` in order of availability.
func findCoderPID(port int) (int, error) {
	// Strategy 1: ss -tlnp (fast, no extra deps)
	cmd := exec.Command("ss", "-tlnp", fmt.Sprintf("sport = :%d", port))
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			// ss output: LISTEN 0 4096 0.0.0.0:18434 0.0.0.0:* users:(("server",pid=12345,...))
			if !strings.Contains(line, fmt.Sprintf(":%d", port)) {
				continue
			}
			// Extract pid=N from the users() section
			pid := extractPID(line)
			if pid > 0 {
				return pid, nil
			}
		}
	}

	// Strategy 2: lsof -i :<port>
	cmd = exec.Command("lsof", "-t", "-i", fmt.Sprintf(":%d", port))
	out, err = cmd.Output()
	if err == nil {
		pidStr := strings.TrimSpace(string(out))
		if pidStr != "" {
			lines := strings.Split(pidStr, "\n")
			if len(lines) > 0 {
				pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
				if err == nil {
					return pid, nil
				}
			}
		}
	}

	// Strategy 3: fuser <port>/tcp
	cmd = exec.Command("fuser", fmt.Sprintf("%d/tcp", port))
	out, err = cmd.Output()
	if err == nil {
		pidStr := strings.TrimSpace(string(out))
		if pidStr != "" {
			// fuser outputs something like "12345" or "12345  "
			fields := strings.Fields(pidStr)
			if len(fields) > 0 {
				pid, err := strconv.Atoi(fields[0])
				if err == nil {
					return pid, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("cannot find PID listening on :%d (tried ss, lsof, fuser)", port)
}

// extractPID extracts the first pid=NNNN from a process line.
func extractPID(line string) int {
	// Look for pid=NNNN pattern
	idx := strings.Index(line, "pid=")
	if idx < 0 {
		return 0
	}
	rest := line[idx+4:]
	end := strings.IndexAny(rest, ",) \t")
	if end < 0 {
		end = len(rest)
	}
	pidStr := rest[:end]
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0
	}
	return pid
}

// readRSS reads the RSS (resident set size) in KB of the given PID.
func readRSS(pid int) (int64, error) {
	cmd := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(pid))
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ps: %w", err)
	}
	rssStr := strings.TrimSpace(string(out))
	if rssStr == "" {
		return 0, fmt.Errorf("empty RSS for pid %d", pid)
	}
	rss, err := strconv.ParseInt(rssStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse RSS %q: %w", rssStr, err)
	}
	return rss, nil
}

// postSingle sends one chat completion request and returns the response body.
func postSingle(client *http.Client, endpoint, model, prompt string) error {
	body, _ := json.Marshal(chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	})
	httpResp, err := client.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		return fmt.Errorf("HTTP %d: %s", httpResp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// ---------- main ----------

func main() {
	os.Exit(run())
}

func run() int {
	var (
		n           = flag.Int("n", 200, "number of sequential requests")
		promptText  = flag.String("prompt", "Count from 1 to 5.", "prompt to send to every request")
		interval    = flag.Int("interval", 10, "sample RSS every N requests")
		flagMono    = flag.Bool("monotonic", false, "assert no monotonic leak (final RSS <= initial + leak-pct%)")
		flagGC      = flag.Bool("gc-stability", false, "assert GC-point stability (RSS second-half within threshold)")
		flagSteady  = flag.Bool("steady", false, "assert steady-state (last N samples within narrow band)")
		port        = flag.Int("port", 18434, "coder port")
		endpoint    = flag.String("endpoint", "", "coder /v1/chat/completions endpoint (derived from --port if empty)")
		model       = flag.String("model", envOr("HELIX_CODER_MODEL", "llama3.2"), "model name")
		leakPct     = flag.Float64("leak-pct", 15.0, "max allowed RSS growth from start, as percentage")
		steadyN     = flag.Int("steady-n", 5, "number of trailing samples for steady-state check")
		steadyBand = flag.Float64("steady-band", 5.0, "max allowed RSS variance in trailing samples, as percentage")
		out         = flag.String("out", "", "path to write the verdict JSON (required)")
		conduitDir  = flag.String("conduit-dir", "", "optional conduit JSONL event dir (§11.4.116)")
		challID     = flag.String("challenge-id", "", "challenge id for conduit events (defaults to --out basename)")
		timeout     = flag.Duration("timeout", 120*time.Second, "per-request timeout")
		expectFail  = flag.Bool("expect-fail", false, "invert case-level exit code — for golden-bad self-validation fixtures")
	)
	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: helixqa-verify-coder-memory --out <verdict.json> [--n 200] [--prompt ...] [--monotonic] [--gc-stability] [--steady]")
		return exitInfra
	}
	cid := *challID
	if cid == "" {
		cid = strings.TrimSuffix(filepath.Base(*out), filepath.Ext(*out))
	}

	// Derive endpoint from port if not specified
	ep := *endpoint
	if ep == "" {
		ep = fmt.Sprintf("http://localhost:%d/v1/chat/completions", *port)
	}

	var sink conduit.Sink = conduit.NopSink()
	if *conduitDir != "" {
		w, werr := conduit.NewWriter(conduit.Config{Session: "helixllm_coder_memory", Dir: *conduitDir})
		if werr == nil {
			sink = w
			defer w.Close()
		}
	}
	conduit.ChallengeStart(sink, cid, "coder_memory_soak")

	v := verdict{
		Mode:     "memory_soak",
		N:        *n,
		Interval: *interval,
		Endpoint: ep,
		Model:    *model,
		Prompt:   *promptText,
		LeakPct:  *leakPct,
	}

	// Find coder PID
	pid, err := findCoderPID(*port)
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("find coder PID: %v", err))
	}
	v.CoderPID = pid

	fmt.Fprintf(os.Stderr, "Detected coder PID %d on :%d\n", pid, *port)

	// Take initial RSS
	initialRSS, err := readRSS(pid)
	if err != nil {
		return failInfra(sink, cid, &v, *out, fmt.Sprintf("initial RSS: %v", err))
	}
	v.InitialRSSKb = initialRSS
	v.RSSPeakKb = initialRSS

	fmt.Fprintf(os.Stderr, "Initial RSS = %d KB\n", initialRSS)

	// Sequential requests with periodic RSS sampling
	client := &http.Client{Timeout: *timeout}
	v.RSSSamples = append(v.RSSSamples, rssSample{
		Index:   0,
		RSSKb:   initialRSS,
		DeltaKb: 0,
		DeltaPct: 0,
	})

	lastRSS := initialRSS

	for i := 1; i <= *n; i++ {
		if err := postSingle(client, ep, *model, *promptText); err != nil {
			return failInfra(sink, cid, &v, *out, fmt.Sprintf("request %d/%d: %v", i, *n, err))
		}

		// Sample RSS at interval
		if i%*interval == 0 {
			rss, err := readRSS(pid)
			if err != nil {
				return failInfra(sink, cid, &v, *out, fmt.Sprintf("RSS sample at %d/%d: %v", i, *n, err))
			}
			deltaKb := rss - lastRSS
			var deltaPct float64
			if lastRSS > 0 {
				deltaPct = float64(deltaKb) / float64(lastRSS) * 100.0
			}
			v.RSSSamples = append(v.RSSSamples, rssSample{
				Index:    i,
				RSSKb:    rss,
				DeltaKb:  deltaKb,
				DeltaPct: deltaPct,
			})
			lastRSS = rss
			if rss > v.RSSPeakKb {
				v.RSSPeakKb = rss
			}
			fmt.Fprintf(os.Stderr, "  [%d/%d] RSS = %d KB (delta=%d KB, %.1f%%)\n", i, *n, rss, deltaKb, deltaPct)
		}
	}

	// Final RSS (if not already captured at the last interval point)
	if *n%*interval != 0 {
		rss, err := readRSS(pid)
		if err != nil {
			return failInfra(sink, cid, &v, *out, fmt.Sprintf("final RSS: %v", err))
		}
		deltaKb := rss - lastRSS
		var deltaPct float64
		if lastRSS > 0 {
			deltaPct = float64(deltaKb) / float64(lastRSS) * 100.0
		}
		v.RSSSamples = append(v.RSSSamples, rssSample{
			Index:    *n,
			RSSKb:    rss,
			DeltaKb:  deltaKb,
			DeltaPct: deltaPct,
		})
		v.FinalRSSKb = rss
	} else {
		v.FinalRSSKb = lastRSS
	}

	// ---- Assertions ----

	// MONOTONIC-NO-LEAK: final RSS <= initial RSS + leak-pct%
	leakThresholdKb := int64(float64(v.InitialRSSKb) * (1.0 + *leakPct/100.0))
	v.MonotonicNoLeak = v.FinalRSSKb <= leakThresholdKb

	// GC-STABILITY: the second half of RSS samples does not grow beyond
	// the midpoint value + (leak-threshold / 2).
	samples := v.RSSSamples
	midIdx := len(samples) / 2
	midRSS := samples[midIdx].RSSKb
	gcThresholdKb := int64(float64(midRSS) * (1.0 + *leakPct/200.0)) // half the leak-pct
	allStable := true
	for i := midIdx; i < len(samples); i++ {
		if samples[i].RSSKb > gcThresholdKb {
			allStable = false
			break
		}
	}
	v.GCStability = allStable

	// STEADY-STATE: the last N trailing samples are within steady-band (%)
	// of each other.
	if len(samples) >= *steadyN {
		trailing := samples[len(samples)-*steadyN:]
		minRSS := trailing[0].RSSKb
		maxRSS := trailing[0].RSSKb
		for _, s := range trailing {
			if s.RSSKb < minRSS {
				minRSS = s.RSSKb
			}
			if s.RSSKb > maxRSS {
				maxRSS = s.RSSKb
			}
		}
		var varianceBand float64
		if minRSS > 0 {
			varianceBand = float64(maxRSS-minRSS) / float64(minRSS) * 100.0
		}
		v.SteadyState = varianceBand <= *steadyBand
	} else if len(samples) > 0 {
		// Fewer than steadyN samples: trivially steady
		v.SteadyState = true
	} else {
		v.SteadyState = false
	}

	// Combine into PASS
	v.Pass = true
	if *flagMono && !v.MonotonicNoLeak {
		v.Pass = false
	}
	if *flagGC && !v.GCStability {
		v.Pass = false
	}
	if *flagSteady && !v.SteadyState {
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

	conduit.EvidenceCaptured(sink, cid, "memory_soak_verdict_json", *out)

	if v.CaseResult {
		conduit.ChallengeVerdict(sink, cid, conduit.VerdictPass, "")
		fmt.Printf("PASS: %s pid=%d initial=%dkb peak=%dkb final=%dkb mono=%v gc=%v steady=%v expect_fail=%v\n",
			cid, v.CoderPID, v.InitialRSSKb, v.RSSPeakKb, v.FinalRSSKb,
			v.MonotonicNoLeak, v.GCStability, v.SteadyState, v.ExpectFail)
		return exitPass
	}
	reason := fmt.Sprintf("pid=%d initial=%dkb peak=%dkb final=%dkb mono=%v gc=%v steady=%v",
		v.CoderPID, v.InitialRSSKb, v.RSSPeakKb, v.FinalRSSKb,
		v.MonotonicNoLeak, v.GCStability, v.SteadyState)
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
